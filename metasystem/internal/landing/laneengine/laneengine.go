// Package laneengine owns the landing lane's engine identity (lane runtime
// design §2, K5). It answers three questions, and nothing else does:
//
//   - RequireSelf: is the executable this process runs the lane's enrolled
//     engine? Every kernel verb asks it first and refuses on anything but
//     yes, so a hand-rebuilt bin/metasystem, or a seat's engine, can never
//     act as the lane's kernel.
//   - RequirePolicyEngine: was a proof judged by the enrolled engine? A
//     publication needs the proof's PolicyEngineDigest to be the enrolled
//     one; its CandidateEngineDigest (the engine the proven tree builds) is
//     free, so an engine change is proven by its own candidate.
//   - Advance: the one way the lane's engine changes, `landing engine
//     advance`: only between batches, only with custody settled and the lane
//     not paused, and only to the engine built from landed origin/main.
//
// Call contract for the kernel verbs built later (begin, prove, publish,
// return, validate; units K-b, K-c, K-d, K-f): a verb reaches its own code
// only after RequireSelf admitted this process against the lane's
// installation. In cmd/metasystem that is by construction: a landing command
// whose action is a kernel action is declared through laneKernelCommand,
// which runs RequireSelf before the verb and hands the verb the admitted
// Identity; TestEveryLaneKernelVerbChecksItsEngineFirst fails for any kernel
// action declared otherwise. publish (K-c) calls RequirePolicyEngine with the
// admitted Identity's enrollment and the proof result's two digests before it
// mints a tuple token.
//
// Residual risk (design §2, D6): code running under the lane's own OS account
// can replace the executable and the enrollment together; that is not
// prevented here.
package laneengine

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// The engine identity's refusal codes (the refusal register names their
// sites).
const (
	CodeNotEnrolled          = "LANE_ENGINE_NOT_ENROLLED"
	CodeEnrollmentUnknown    = "LANE_ENGINE_ENROLLMENT_UNKNOWN"
	CodePolicyNotEnrolled    = "LANE_POLICY_ENGINE_NOT_ENROLLED"
	CodeAdvancePaused        = "LANE_ENGINE_ADVANCE_PAUSED"
	CodeAdvanceBatchInFlight = "LANE_ENGINE_ADVANCE_BATCH_IN_FLIGHT"
	CodeAdvanceCustodyLive   = "LANE_ENGINE_ADVANCE_CUSTODY_LIVE"
	CodeAdvanceNotLanded     = "LANE_ENGINE_ADVANCE_NOT_LANDED"
	CodeAdvanceRunnerDown    = "LANE_ENGINE_ADVANCE_RUNNER_DOWN"
)

// Refusal is a refusal a person or the landing agent reads: Message is line
// 1, the plain situation; Argv is line 2, the one command; Detail is for
// --verbose only.
type Refusal struct {
	Code, Message string
	Argv          []string
	Detail        string
}

func (r *Refusal) Error() string {
	if r.Detail == "" {
		return r.Code + ": " + r.Message
	}
	return r.Code + ": " + r.Message + " (" + r.Detail + ")"
}

// Identity is a kernel verb's admission: the lane installation it acts for,
// that installation's enrollment, and the digest of the executable this
// process runs, which RequireSelf found equal to the enrolled one.
type Identity struct {
	Installation string
	Enrolled     steward.InstallIdentity
	Running      string
}

// runningExecutable is the file whose bytes this process runs. On Linux the
// kernel names the running image itself (/proc/self/exe), so a path replaced
// after exec cannot pass for the running engine. Elsewhere (macOS) it is the
// path the process was started from: bytes replaced at that path after exec
// are what is hashed, so there the guarantee is the path's content at the
// check, not the running image's (a same-account residual risk, D6).
func runningExecutable() (string, error) {
	if info, err := os.Stat("/proc/self/exe"); err == nil && info.Mode().IsRegular() {
		return "/proc/self/exe", nil
	}
	return os.Executable()
}

// fileDigest is a file's digest in the enrollment's spelling, sha256:<hex>.
func fileDigest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return fmt.Sprintf("sha256:%x", hash.Sum(nil)), nil
}

// RunningDigest is the digest of the executable this process runs.
func RunningDigest() (string, error) {
	path, err := runningExecutable()
	if err != nil {
		return "", err
	}
	return fileDigest(path)
}

func canonical(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}

// Enrollment reads the lane installation's steward enrollment, authenticated
// by the steward's own rules (owner, mode, repository).
func Enrollment(installation string) (steward.InstallIdentity, error) {
	top := canonical(installation)
	return steward.VerifyIdentity(steward.RepoIdentityPath(top), top)
}

// RequireSelf is the check every kernel verb makes before anything else: the
// executable this process runs must be the lane installation's enrolled
// engine, by content. Anything it cannot establish refuses (fail closed).
// checkout is the lane checkout, named in a person's fix; retry is the
// verb's words after "metasystem", run again by the enrolled engine.
func RequireSelf(checkout, installation string, retry []string) (Identity, error) {
	top := canonical(installation)
	enrolled, err := Enrollment(top)
	if err != nil {
		return Identity{}, &Refusal{Code: CodeEnrollmentUnknown,
			Message: "the landing lane's enrolled engine can't be read, so nothing was done",
			Argv:    []string{"metasystem", "system", "start", "--repo", checkout},
			Detail:  "a person at a terminal no agent started arms the lane checkout with metasystem system start: " + err.Error()}
	}
	running, err := RunningDigest()
	if err != nil {
		return Identity{}, &Refusal{Code: CodeNotEnrolled,
			Message: "this engine's own bytes can't be read, so it can't act for the landing lane; nothing was done",
			Argv:    append([]string{enrolled.InstallPath}, retry...),
			Detail:  "hash the running executable: " + err.Error()}
	}
	if enrolled.InstallDigest != "" && running == enrolled.InstallDigest {
		return Identity{Installation: top, Enrolled: enrolled, Running: running}, nil
	}
	detail := fmt.Sprintf("running %s; enrolled engine number %d is %s at %s", running, enrolled.Generation, enrolled.InstallDigest, enrolled.InstallPath)
	if installed, err := fileDigest(enrolled.InstallPath); err == nil && installed == enrolled.InstallDigest {
		return Identity{}, &Refusal{Code: CodeNotEnrolled,
			Message: "this metasystem isn't the landing lane's enrolled engine, so nothing was done",
			Argv:    append([]string{enrolled.InstallPath}, retry...), Detail: detail}
	}
	// The enrolled path no longer holds the enrolled bytes: it was rebuilt
	// outside landing engine advance. The enrolled pin, when it survives,
	// is the enrolled engine and moves the lane to landed main.
	refusal := &Refusal{Code: CodeNotEnrolled,
		Message: "the landing lane's engine was rebuilt outside metasystem landing engine advance, so nothing was done",
		Detail:  detail}
	pin := steward.EnrolledExecutionPath(top, enrolled)
	if pinned, err := fileDigest(pin); err == nil && pinned == enrolled.InstallDigest {
		refusal.Argv = []string{pin, "landing", "engine", "advance"}
		return Identity{}, refusal
	}
	refusal.Argv = []string{"metasystem", "system", "start", "--repo", checkout}
	refusal.Detail += "; no enrolled bytes survive, so a person at a terminal no agent started arms the checkout again"
	return Identity{}, refusal
}

// ProofEngines are the two engine digests a proof result records.
type ProofEngines struct {
	// Policy is the engine that chose and judged the tests.
	Policy string
	// Candidate is the engine the proven tree builds; it is free.
	Candidate string
}

func bareDigest(digest string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(digest), "sha256:"))
}

// RequirePolicyEngine is publish's engine check: the proof was judged by the
// enrolled engine. The candidate engine is deliberately not consulted.
// reprove is line 2, the proof to run again on the enrolled engine.
func RequirePolicyEngine(enrolled steward.InstallIdentity, proof ProofEngines, reprove []string) error {
	want, got := bareDigest(enrolled.InstallDigest), bareDigest(proof.Policy)
	if len(want) == sha256.Size*2 && got == want {
		return nil
	}
	return &Refusal{Code: CodePolicyNotEnrolled,
		Message: "the tests were judged by an engine that isn't the landing lane's enrolled one, so nothing was published",
		Argv:    reprove,
		Detail:  fmt.Sprintf("judging engine %q; enrolled engine number %d is %q", proof.Policy, enrolled.Generation, enrolled.InstallDigest)}
}
