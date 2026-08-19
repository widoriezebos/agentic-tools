package mission

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// The mission anchor is a local git commit that binds the mission's state hash
// and ledger bytes into history, so a recovered checkout can prove its ledger
// truth. This file owns writing that anchor, verifying it, and reconciling a
// state against its ledger and anchor. Git is the source of truth here, so
// these operations invoke it directly.

// gitOutput runs a git command and returns its stdout, failing on a nonzero
// exit.
func gitOutput(repo string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// Bounded like every other external call (B4): a git that never
	// returns must not hang the caller.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " ")); err != nil {
		return stdout.String(), stateErr("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// gitTry runs a git command and returns its stdout and exit code without
// treating a nonzero exit as an error.
func gitTry(repo string, args ...string) (string, int) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	var stdout strings.Builder
	cmd.Stdout = &stdout
	// Bounded like every other external call (B4); a timeout is a failure
	// answer, not an exit code.
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " "))
	if err == nil {
		return stdout.String(), 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return stdout.String(), exit.ExitCode()
	}
	return stdout.String(), 1
}

var classLineRe = regexp.MustCompile(`(?m)^- Classification:[ \t]*`)

// ledgerCycleCount returns the number of cycles a ledger records, requiring the
// headings to be contiguous from 1 and each cycle block to carry exactly one
// classification line.
func ledgerCycleCount(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, stateErr("cannot read mission ledger: %v", err)
	}
	return cycleCountOf(string(data))
}

func cycleCountOf(text string) (int, error) {
	headings := headingRe.FindAllStringSubmatch(text, -1)
	for i, h := range headings {
		n, _ := strconv.Atoi(h[1])
		if n != i+1 {
			return 0, stateErr("mission ledger cycle headings are not contiguous")
		}
	}
	blocks := headingRe.Split(text, -1)
	for i, block := range blocks[1:] {
		if len(classLineRe.FindAllString(block, -1)) != 1 {
			return 0, stateErr("mission ledger Cycle %d lacks exactly one classification", i+1)
		}
	}
	return len(headings), nil
}

func ledgerHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", stateErr("cannot read mission ledger: %v", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// relUnderRepo returns path relative to repo in posix form, erroring when path
// is outside the repository.
func relUnderRepo(path, repo string) (string, error) {
	pathAbs := resolvePath(path)
	repoAbs := resolvePath(repo)
	rel, err := filepath.Rel(repoAbs, pathAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", stateErr("mission ledger is outside the repository")
	}
	return filepath.ToSlash(rel), nil
}

func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

var trailerRe = regexp.MustCompile(`(?m)^(Mission-[A-Za-z0-9-]+): (.+)$`)

// latestAnchor returns the trailers (plus the commit sha) of the most recent
// anchor commit for a mission.
// stateAnchorRef is the runner-owned ref carrying a mission's state
// anchors (slice-5 round-4): anchors OFF the mission branch keep the
// branch tree free of force-tracked bookkeeping — delegate worktrees
// inherit no artifacts files, the wall's identity space and the raw
// commit trees agree, and a host commit on the branch can never become
// the guard's baseline.
func stateAnchorRef(mission string) string {
	return "refs/metasystem/missions/" + mission + "/state-anchors"
}

// ErrNoAnchor names the one tolerable absence: a mission that has never
// anchored (fresh unit beds); every OTHER anchor failure is disagreement.
var ErrNoAnchor = stateErr("mission state has no local anchor commit")

func latestAnchor(repo, mission string) (map[string]string, error) {
	// Absence must be PROVEN, not inferred from a nonzero exit (round-6:
	// rev-parse returns 128 for a missing ref AND for a broken
	// repository): for-each-ref succeeds in any healthy repository and
	// prints nothing when the ref does not exist.
	probe, err := gitOutput(repo, "for-each-ref", "--format=%(refname)", stateAnchorRef(mission))
	if err != nil {
		return nil, stateErr("mission anchor ref probe failed: %v", err)
	}
	if strings.TrimSpace(probe) == "" {
		return nil, ErrNoAnchor
	}
	// ONLY the ref tip is the anchor (round-7): scanning past a
	// malformed tip to an older matching commit would let a forged child
	// commit demote the real anchor instead of parking.
	output, err := gitOutput(repo, "log", "-1", "--format=%H%x1f%B", stateAnchorRef(mission))
	if err != nil {
		return nil, stateErr("mission anchor ref is unreadable: %v", err)
	}
	commit, message, _ := strings.Cut(output, "\x1f")
	trailers := map[string]string{}
	for _, m := range trailerRe.FindAllStringSubmatch(message, -1) {
		trailers[m[1]] = m[2]
	}
	if trailers["Mission-Id"] != mission {
		return nil, stateErr("mission anchor ref tip does not name %s", mission)
	}
	for _, key := range []string{"Mission-State-Hash", "Mission-Ledger-SHA256", "Mission-Ledger-Path", "Mission-Cycle"} {
		if trailers[key] == "" {
			return nil, stateErr("mission anchor ref tip lacks the %s trailer", key)
		}
	}
	trailers["commit"] = strings.TrimSpace(commit)
	return trailers, nil
}

// Anchor writes the mission's anchor commit and prints the resulting sha.
func Anchor(statePath, repo, ledgerPath string) error {
	commit, err := anchorWrite(statePath, repo, ledgerPath)
	if err != nil {
		return err
	}
	fmt.Println(commit)
	return nil
}

// AnchorNamed is the runner's in-process anchor (slice-6 successor
// finding 4: the CLI verb is now human-reserved, so the runner no longer
// shells out): the git author pins to the acting identity, and the
// optional expectations pin the anchored position to bytes the caller
// VERIFIED — a state hash and a ledger sha computed at its own recheck
// (successor finding 2: the anchor must bind what was ruled on, not
// whatever a reread finds).
func AnchorNamed(statePath, repo, ledgerPath, identity, expectStateHash, expectLedgerSHA string) error {
	_, err := anchorWritePinned(statePath, repo, ledgerPath, identity, expectStateHash, expectLedgerSHA)
	return err
}

// anchorWrite is the printing-free anchor used by reconciliation's
// retry-safe heal as well as the public verb; it returns the commit sha.
func anchorWrite(statePath, repo, ledgerPath string) (string, error) {
	return anchorWritePinned(statePath, repo, ledgerPath, "", "", "")
}

func anchorWritePinned(statePath, repo, ledgerPath, identity, expectStateHash, expectLedgerSHA string) (string, error) {
	// The STATE LOCK spans the whole publication (slice-6 successor
	// round-2 finding 1): every state write takes this lock, so the
	// position read here is still the live position when the ref
	// updates — a concurrent writer can never make a fresh anchor name
	// an already-superseded state (the anchor moving BACKWARD).
	lock, err := lockFile(statePath)
	if err != nil {
		return "", err
	}
	defer lock.release()
	return anchorWriteHeld(statePath, repo, ledgerPath, identity, expectStateHash, expectLedgerSHA)
}

// anchorWriteHeld is the body of the anchor for callers that ALREADY
// hold the state lock — reconciliation's heal branches, which lock for
// their whole pass (flock self-deadlocks on a second acquisition).
func anchorWriteHeld(statePath, repo, ledgerPath, identity, expectStateHash, expectLedgerSHA string) (string, error) {
	state, err := readStateDoc(statePath)
	if err != nil {
		return "", err
	}
	if err := validate(state); err != nil {
		return "", err
	}
	// ONE read of the ledger under ITS lock, held through the ref
	// publication (successor round-10 finding 4): the hash, the cycle
	// count, and the stored blob all derive from the SAME bytes, and no
	// ledger writer can move them while the anchor publishes.
	ledgerLock, err := lockFile(ledgerPath)
	if err != nil {
		return "", err
	}
	defer ledgerLock.release()
	ledgerBytes, err := os.ReadFile(ledgerPath)
	if err != nil {
		return "", stateErr("cannot read mission ledger: %v", err)
	}
	// FULL parseability gates the anchor (slice-6 successor round-12
	// finding 1): an anchor binds bytes as the mission's operational
	// baseline, and a baseline the ledger machinery cannot read is a
	// mission nothing can drive. Every lawful writer keeps the ledger
	// parseable, so only tampered bytes ever refuse here.
	if _, _, _, perr := parseLedgerText(string(ledgerBytes)); perr != nil {
		return "", stateErr("anchor refused: mission ledger is unparsable: %v", perr)
	}
	cycles, err := cycleCountOf(string(ledgerBytes))
	if err != nil {
		return "", err
	}
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	if int64(cycles) != stateCycles {
		return "", stateErr("anchor refused: ledger is truth and its cycle count disagrees with state")
	}
	branch, err := gitOutput(repo, "branch", "--show-current")
	if err != nil {
		return "", err
	}
	stateBranch, _ := state["branch"].(string)
	if strings.TrimSpace(branch) != stateBranch {
		return "", stateErr("anchor refused: current branch is not the mission branch")
	}
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil {
		return "", stateErr("anchor refused: mission ledger is outside the repository")
	}
	missionID, _ := state["missionId"].(string)
	integrity, _ := state["integrity"].(map[string]any)
	stateHashValue, _ := integrity["hash"].(string)
	lHash := sha256Hex(string(ledgerBytes))
	if expectStateHash != "" && expectStateHash != stateHashValue {
		return "", stateErr("anchor refused: the state moved past the verified position")
	}
	if expectLedgerSHA != "" && expectLedgerSHA != lHash {
		return "", stateErr("anchor refused: the ledger moved past the verified bytes")
	}
	// Anchor cadence (slice-6 round-3 finding 1): the anchored LEDGER
	// truth moves only alongside a state-hash change. Every lawful
	// anchor follows a state write (new hash) or re-anchors an identical
	// position (heal retries); an anchor that would re-bind the SAME
	// state hash to DIFFERENT ledger bytes is the mid-turn laundering
	// shape — a host rewriting the ledger and re-anchoring to move the
	// guard's baseline — and refuses.
	if tip, err := latestAnchor(repo, missionID); err == nil {
		if tip["Mission-State-Hash"] == stateHashValue && tip["Mission-Ledger-SHA256"] != lHash {
			return "", stateErr("anchor refused: ledger bytes changed without a state write")
		}
	} else if !errors.Is(err, ErrNoAnchor) {
		return "", err
	}
	subject := fmt.Sprintf("mission(%s): anchor cycle %d", missionID, cycles)
	body := fmt.Sprintf("Mission-Id: %s\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: %s\nMission-Cycle: %d",
		missionID, stateHashValue, lHash, ledgerRel, cycles)

	// The anchor commit is built with plumbing onto the runner-owned ref:
	// the ledger blob rides in the commit's own tree (the reconciliation
	// prefix check reads it back), the mission branch and the real index
	// stay untouched, and the ref advances with compare-and-swap so two
	// racing anchors cannot silently drop one another.
	blob, err := gitStdinOutput(repo, ledgerBytes, "hash-object", "-w", "--no-filters", "--stdin")
	if err != nil {
		return "", stateErr("anchor cannot store the ledger blob: %v", err)
	}
	indexDir, err := os.MkdirTemp("", "metasystem-anchor.")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(indexDir)
	indexEnv := "GIT_INDEX_FILE=" + filepath.Join(indexDir, "index")
	if _, err := gitEnvOutput(repo, []string{indexEnv}, "read-tree", "--empty"); err != nil {
		return "", stateErr("anchor cannot build its tree: %v", err)
	}
	if _, err := gitEnvOutput(repo, []string{indexEnv}, "update-index", "--add",
		"--cacheinfo", "100644,"+strings.TrimSpace(blob)+","+ledgerRel); err != nil {
		return "", stateErr("anchor cannot build its tree: %v", err)
	}
	tree, err := gitEnvOutput(repo, []string{indexEnv}, "write-tree")
	if err != nil {
		return "", stateErr("anchor cannot build its tree: %v", err)
	}
	ref := stateAnchorRef(missionID)
	parent, _ := gitTry(repo, "rev-parse", "--verify", ref)
	author := "metasystem"
	if identity != "" {
		author = identity
	}
	identityEnv := []string{
		"GIT_AUTHOR_NAME=" + author, "GIT_AUTHOR_EMAIL=" + author + "@example.invalid",
		"GIT_COMMITTER_NAME=metasystem", "GIT_COMMITTER_EMAIL=metasystem@example.invalid",
	}
	commitArgs := []string{"commit-tree", strings.TrimSpace(tree), "-m", subject + "\n\n" + body}
	oldValue := strings.TrimSpace(parent)
	if oldValue != "" {
		commitArgs = append(commitArgs, "-p", oldValue)
	}
	commit, err := gitEnvOutput(repo, identityEnv, commitArgs...)
	if err != nil {
		return "", stateErr("anchor commit failed: %v", err)
	}
	updateArgs := []string{"update-ref", ref, strings.TrimSpace(commit)}
	if oldValue != "" {
		updateArgs = append(updateArgs, oldValue)
	} else {
		updateArgs = append(updateArgs, "")
	}
	if _, err := gitOutput(repo, updateArgs...); err != nil {
		return "", stateErr("anchor ref update failed: %v", err)
	}
	return strings.TrimSpace(commit), nil
}

// gitStdinOutput runs git feeding the given bytes on stdin — the anchor
// stores exactly the bytes it hashed, never a second file read.
func gitStdinOutput(repo string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " ")); err != nil {
		return "", stateErr("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// gitEnvOutput runs git with extra environment, capturing stdout —
// bounded like every other external call (B4).
func gitEnvOutput(repo string, extraEnv []string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	limit := boundedexec.Timeout(filepath.Join(repo, "metasystem.conf"), boundedexec.Local)
	if err := boundedexec.Run(cmd, limit, "git "+strings.Join(args, " ")); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), detail)
	}
	return stdout.String(), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// verifyAnchor checks the latest anchor's trailers, that it is on the mission
// branch, and that it carries the exact current ledger bytes.
func verifyAnchor(repo string, state map[string]any, ledgerPath string) error {
	missionID, _ := state["missionId"].(string)
	anchor, err := latestAnchor(repo, missionID)
	if err != nil {
		return err
	}
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil {
		return err
	}
	integrity, _ := state["integrity"].(map[string]any)
	stateHashValue, _ := integrity["hash"].(string)
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	lHash, err := ledgerHash(ledgerPath)
	if err != nil {
		return err
	}
	expected := map[string]string{
		"Mission-State-Hash":    stateHashValue,
		"Mission-Ledger-SHA256": lHash,
		"Mission-Ledger-Path":   ledgerRel,
		"Mission-Cycle":         strconv.FormatInt(stateCycles, 10),
	}
	for key, value := range expected {
		if anchor[key] != value {
			return stateErr("mission anchor disagrees at %s", key)
		}
	}
	if _, code := gitTry(repo, "merge-base", "--is-ancestor", anchor["commit"], stateAnchorRef(missionID)); code != 0 {
		return stateErr("mission anchor commit is not on the mission's anchor ref")
	}
	anchored, code := gitTry(repo, "show", anchor["commit"]+":"+ledgerRel)
	if code != 0 || sha256Hex(anchored) != anchor["Mission-Ledger-SHA256"] {
		return stateErr("mission anchor commit does not contain the declared ledger bytes")
	}
	return nil
}

// anchoredLedgerPrefix verifies every anchor claim except that the ledger is
// byte-identical: the anchor's state hash, cycle, path, and branch ancestry
// hold, and the current ledger extends the anchored bytes. It returns the
// anchored prefix and the current ledger bytes so a caller can judge the
// unanchored suffix.
func anchoredLedgerPrefix(repo string, state map[string]any, ledgerPath string) (anchored, current string, err error) {
	missionID, _ := state["missionId"].(string)
	anchor, err := latestAnchor(repo, missionID)
	if err != nil {
		return "", "", err
	}
	integrity, _ := state["integrity"].(map[string]any)
	stateHashValue, _ := integrity["hash"].(string)
	if anchor["Mission-State-Hash"] != stateHashValue {
		return "", "", stateErr("mission anchor disagrees at Mission-State-Hash")
	}
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	if anchor["Mission-Cycle"] != strconv.FormatInt(stateCycles, 10) {
		return "", "", stateErr("mission anchor disagrees at Mission-Cycle")
	}
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil {
		return "", "", err
	}
	if anchor["Mission-Ledger-Path"] != ledgerRel {
		return "", "", stateErr("mission anchor disagrees at Mission-Ledger-Path")
	}
	anchored, code := gitTry(repo, "show", anchor["commit"]+":"+ledgerRel)
	if code != 0 {
		return "", "", stateErr("mission anchor commit does not contain the prior ledger")
	}
	if sha256Hex(anchored) != anchor["Mission-Ledger-SHA256"] {
		return "", "", stateErr("mission anchor prior ledger hash is invalid")
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return "", "", stateErr("cannot read mission ledger: %v", err)
	}
	if !strings.HasPrefix(string(data), anchored) {
		return "", "", stateErr("mission ledger does not extend the anchored ledger truth")
	}
	if _, code := gitTry(repo, "merge-base", "--is-ancestor", anchor["commit"], stateAnchorRef(missionID)); code != 0 {
		return "", "", stateErr("mission anchor commit is not on the mission's anchor ref")
	}
	return anchored, string(data), nil
}

// AnchoredLedgerSHA returns the ledger sha the mission's anchor tip
// records — the PIN ORIGIN for anchors of positions whose ledger was
// verified at entry and untouched since (slice-6 successor round-11
// findings 2-3): a pin must certify the position the caller VERIFIED,
// never whatever bytes a post-write reread finds.
func AnchoredLedgerSHA(repo, missionID string) (string, error) {
	anchor, err := latestAnchor(repo, missionID)
	if err != nil {
		return "", err
	}
	return anchor["Mission-Ledger-SHA256"], nil
}

// AnchoredLedgerTruth exposes the authenticated anchored-ledger
// comparison to the wall's in-turn guard (slice-5 round-4): the anchored
// bytes come from the runner-owned anchor ref with every cross-check —
// state hash, cycle count, path, sha — applied before anything is
// trusted, never from whatever commit last touched a path.
func AnchoredLedgerTruth(repo string, state map[string]any, ledgerPath string) (anchored, current string, err error) {
	return anchoredLedgerPrefix(repo, state, ledgerPath)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// parkIntegrity writes a state parked for state-integrity, advancing the chain.
func parkIntegrity(statePath string, state map[string]any, recoveryOf any) error {
	parked, _ := deepCopyDoc(state).(map[string]any)
	parked["status"] = "parked"
	parked["parkReason"] = "state-integrity"
	parked["gatePassed"] = false
	finalized, err := finalizeNext(parked, state, recoveryOf)
	if err != nil {
		return err
	}
	return atomicWriteJSON(statePath, finalized)
}

// Reconcile validates a state against its ledger and anchor, parking it for
// human reconciliation when they disagree. It returns 0 when reconciled, or 3
// when the mission was parked.
func Reconcile(statePath, repo, ledgerPath string) (int, error) {
	lock, err := lockFile(statePath)
	if err != nil {
		return 1, err
	}
	defer lock.release()

	raw, err := readStateDoc(statePath)
	if err != nil {
		return 1, err
	}
	if err := validate(raw); err != nil {
		// A pre-wall state is NOT corruption: the named refusal reaches the
		// operator verbatim, no corrupt-state file is written, and no
		// recovery is attempted — the remedy is re-provisioning.
		if errors.Is(err, ErrLegacyState) || errors.Is(err, ErrPreWallBaseline) {
			return 3, err
		}
		return reconcileCorruptState(statePath, raw)
	}
	cycles, err := ledgerCycleCount(ledgerPath)
	if err != nil {
		return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
	}
	ledger, _ := raw["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	switch {
	case int64(cycles) < stateCycles:
		return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
	case int64(cycles) > stateCycles+1:
		// The reserve/append machinery books at most ONE cycle between
		// state writes, so a ledger more than one block ahead is not a
		// crash window — it is divergence (slice-5 round-5).
		return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
	case int64(cycles) > stateCycles:
		// The unanchored suffix must be EXACTLY the one appended block
		// (slice-6 successor finding 1): "extends the anchored bytes"
		// alone would let a crash bless host bytes smuggled BETWEEN the
		// anchored truth and the runner's block — the suffix must open
		// with the block heading, nothing before it.
		anchored, current, err := anchoredLedgerPrefix(repo, raw, ledgerPath)
		if err != nil {
			return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
		}
		suffix := strings.TrimLeft(current[len(anchored):], "\n")
		if !strings.HasPrefix(suffix, fmt.Sprintf("### Cycle %d", cycles)) {
			return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
		}
		// BYTE-PRECISE (round-8 finding 3): the whole current file must
		// hash to exactly what the crashed append wrote — the pending
		// stamp beside the ledger. A single byte added by anyone else
		// after that write refuses.
		if !pendingStampMatches(ledgerPath, int64(cycles), current) {
			return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
		}
		// The single tolerated block must be the RESERVED cycle
		// (round-6): the fence counters prove the runner reserved it,
		// and either the open-turn marker names it as the turn in
		// flight, or the block is the reserve/append heal's own
		// lost-turn shape (slice-6 successor finding 3) — contiguity
		// alone proves a number, not authorship.
		if err := verifyReservedGap(repo, raw, int64(cycles)); err != nil {
			if !lostTurnBlock(suffix) {
				return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
			}
			fencesPath := filepath.Join(repo, "artifacts", "agents", "missions",
				func() string { m, _ := raw["missionId"].(string); return m }(), "fences.json")
			fences, ferr := readStateDoc(fencesPath)
			if ferr != nil {
				return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
			}
			if reserved, ok := intValue(fences["cycles"]); !ok || reserved < int64(cycles) {
				return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
			}
		}
		proposed, _ := deepCopyDoc(raw).(map[string]any)
		pledger, _ := proposed["ledger"].(map[string]any)
		pledger["cycles"] = cycles
		pfences, _ := proposed["fences"].(map[string]any)
		fenceCycles, _ := intValue(pfences["cycles"])
		if int64(cycles) > fenceCycles {
			pfences["cycles"] = cycles
		}
		finalized, err := finalizeNext(proposed, raw, nil)
		if err != nil {
			return 3, parkIntegrity(statePath, raw, nil)
		}
		if err := atomicWriteJSON(statePath, finalized); err != nil {
			return 1, err
		}
		// Retry safety (slice-5 round-5): the healed state anchors NOW —
		// a crash after this write but before any later anchor must find
		// hash and anchor in agreement, not park for integrity. A CAS
		// loss against a RACING anchor of the same position converges
		// instead of parking (slice-6 successor finding 5).
		// The anchor pins to the EXACT bytes the stamp check verified
		// (round-9 finding 1): a byte landing between the check and the
		// anchor's own read refuses instead of being anchored.
		healedIntegrity, _ := finalized["integrity"].(map[string]any)
		healedHash, _ := healedIntegrity["hash"].(string)
		if _, err := anchorWriteHeld(statePath, repo, ledgerPath, "", healedHash, sha256Hex(current)); err != nil {
			if verifyAnchor(repo, finalized, ledgerPath) == nil {
				return 0, nil
			}
			// The healed write is durable and the pending stamp still
			// proves the block, so THIS shape satisfies the one-block
			// retry heal on the next reconcile (round-16 finding 4) — a
			// park here would make the anchor a grandparent. Refuse
			// WITHOUT writing, naming the repairable cause.
			return 3, stateErr("mission heal anchor failed: %v; repair the repository condition and reconcile again", err)
		}
		return 0, nil
	default:
		if err := verifyAnchor(repo, raw, ledgerPath); err != nil {
			// One named, checkable exception (docs/design/stop-loss-core.md): a
			// stagnation-parked mission whose unanchored ledger suffix is
			// solely vocal stop-loss reset lines is replayable state — a
			// crash between the reset append and its anchor — not divergence.
			if stopLossResetForgivable(statePath, repo, raw, ledgerPath) {
				return 0, nil
			}
			// The heal-crash window (round-6): a crash between the heal's
			// state write and its anchor leaves the anchor exactly ONE
			// state-step behind with identical ledger truth. That precise
			// shape re-anchors and passes; anything looser still parks.
			if verifiedLedgerSHA, healable := anchorLagHealable(repo, raw, ledgerPath); healable {
				lagIntegrity, _ := raw["integrity"].(map[string]any)
				lagHash, _ := lagIntegrity["hash"].(string)
				// Same pin discipline as the ledger-ahead heal (round-9
				// finding 1): the pin is the sha of the EXACT bytes the
				// healability check verified — never a second read.
				aerr := func() error {
					_, err := anchorWriteHeld(statePath, repo, ledgerPath, "", lagHash, verifiedLedgerSHA)
					return err
				}()
				if aerr == nil {
					return 0, nil
				}
				if verifyAnchor(repo, raw, ledgerPath) == nil {
					return 0, nil
				}
				// A HEALABLE position whose publication failed is
				// retryable (round-17 finding 1): the shape re-heals on
				// the next reconcile once the repository condition —
				// e.g. the checked-out branch — is repaired. A park
				// here would make the anchor a grandparent.
				return 3, stateErr("mission heal anchor failed: %v; repair the repository condition and reconcile again", aerr)
			}
			return parkUnlessRecoverable(statePath, repo, raw, ledgerPath)
		}
	}
	return 0, nil
}

// pendingStampMatches verifies the ledger's complete current bytes are
// exactly the last append's post-write bytes: the pending-block stamp
// records {cycle, sha256(full file)} at every ledger write (round-8
// finding 3). Absent or mismatched stamps fail closed.
func pendingStampMatches(ledgerPath string, cycle int64, current string) bool {
	doc, err := readStateDoc(filepath.Join(filepath.Dir(ledgerPath), "pending-block.json"))
	if err != nil {
		return false
	}
	recorded, ok := intValue(doc["cycle"])
	if !ok || recorded != cycle {
		return false
	}
	sha, _ := doc["ledgerSha256"].(string)
	return sha != "" && sha == sha256Hex(current)
}

// parkUnlessRecoverable is every reconciliation park's LAST gate
// (slice-6 successor rounds 14-15): ONE LAWFUL STEP with a disputed
// ledger — the state write landed, its authenticated anchor never did —
// is recoverable by byte restoration, and writing a state-integrity
// park would make the anchor a grandparent and destroy that recovery.
// The shape refuses WITHOUT writing, naming the repair; every other
// divergence parks exactly as before.
func parkUnlessRecoverable(statePath, repo string, state map[string]any, ledgerPath string) (int, error) {
	if oneStepDisputedLedger(repo, state, ledgerPath) {
		return 3, stateErr("mission ledger disputes its anchored truth one step behind the state; restore the ledger bytes to the anchor ref's copy, then reconcile again")
	}
	return 3, parkIntegrity(statePath, state, nil)
}

// oneStepDisputedLedger reports the one recoverable divergence shape:
// the anchor tip is exactly the state's immediate predecessor (the
// write landed, its anchor never did) while the live ledger bytes
// disagree with the tip's recorded truth.
func oneStepDisputedLedger(repo string, state map[string]any, ledgerPath string) bool {
	missionID, _ := state["missionId"].(string)
	anchor, err := latestAnchor(repo, missionID)
	if err != nil {
		return false
	}
	integrity, _ := state["integrity"].(map[string]any)
	previousHash, _ := integrity["previousHash"].(string)
	if previousHash == "" || anchor["Mission-State-Hash"] != previousHash {
		return false
	}
	// The tip must BE a complete, authenticated anchor (successor
	// round-15 finding 2): its recorded path is this mission's ledger
	// and its committed blob hashes to the declared sha — the repair
	// this refusal prescribes must actually exist. A forged or
	// blob-less tip is NOT recoverable and keeps the established
	// state-integrity park. A DELETED ledger cannot path-resolve
	// itself (round-17: the deleted-live-ledger shape is precisely a
	// dispute), so its parent directory anchors the resolution.
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil {
		dirRel, derr := relUnderRepo(filepath.Dir(ledgerPath), repo)
		if derr != nil {
			return false
		}
		ledgerRel = filepath.Join(dirRel, filepath.Base(ledgerPath))
	}
	if anchor["Mission-Ledger-Path"] != ledgerRel {
		return false
	}
	blob, code := gitTry(repo, "show", anchor["commit"]+":"+ledgerRel)
	if code != 0 || sha256Hex(blob) != anchor["Mission-Ledger-SHA256"] {
		return false
	}
	// The CYCLE relationship authenticates too (round-16 finding 3):
	// the blob must contain exactly the cycle count the trailer claims,
	// and restoring it must yield a position the exact-match heal can
	// bridge — the trailer cycle equals the state's own count. A tip a
	// cycle behind (or a forged trailer) is NOT prescribed as repair.
	anchorCycle, err := strconv.ParseInt(anchor["Mission-Cycle"], 10, 64)
	if err != nil {
		return false
	}
	blobCount, cerr := cycleCountOf(blob)
	if cerr != nil || int64(blobCount) != anchorCycle {
		return false
	}
	ledgerDoc, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledgerDoc["cycles"])
	if anchorCycle != stateCycles {
		return false
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		// An unreadable or deleted live ledger IS the dispute in its
		// starkest form (round-16 finding 2): the authenticated blob
		// exists and restoring it heals — parking would destroy that.
		return true
	}
	return sha256Hex(string(data)) != anchor["Mission-Ledger-SHA256"]
}

// lostTurnBlock reports whether a ledger suffix is the reserve/append
// heal's own booking: a no-progress block observed as a lost turn or a
// drain stall — the only blocks that lawfully exist without an open-turn
// marker (their state write crashed).
func lostTurnBlock(suffix string) bool {
	if !strings.Contains(suffix, "- Classification: no-progress;") {
		return false
	}
	return strings.Contains(suffix, "observed=unmeasurable:turn-lost") ||
		strings.Contains(suffix, "observed=unmeasurable:drain-stalled")
}

// verifyReservedGap proves the ledger-ahead block is the runner's own: the
// mission fences reserved exactly that cycle, and the open-turn marker
// names it as the turn in flight.
func verifyReservedGap(repo string, state map[string]any, appended int64) error {
	missionID, _ := state["missionId"].(string)
	fencesPath := filepath.Join(repo, "artifacts", "agents", "missions", missionID, "fences.json")
	fences, err := readStateDoc(fencesPath)
	if err != nil {
		return stateErr("ledger-ahead heal cannot read the mission fences: %v", err)
	}
	reserved, ok := intValue(fences["cycles"])
	if !ok || reserved < appended {
		return stateErr("ledger-ahead block %d was never reserved in the mission fences", appended)
	}
	openTurn, ok := state["openTurn"].(map[string]any)
	if !ok {
		return stateErr("ledger-ahead block %d has no open turn to answer for it", appended)
	}
	if cycle, ok := intValue(openTurn["cycle"]); !ok || cycle != appended {
		return stateErr("ledger-ahead block %d does not match the open turn's cycle", appended)
	}
	return nil
}

// anchorLagHealable reports the exact heal-crash shape: the latest anchor
// binds the state's PREVIOUS hash while cycle count and the ledger bytes'
// hash agree with the present truth.
func anchorLagHealable(repo string, state map[string]any, ledgerPath string) (string, bool) {
	missionID, _ := state["missionId"].(string)
	anchor, err := latestAnchor(repo, missionID)
	if err != nil {
		return "", false
	}
	integrity, _ := state["integrity"].(map[string]any)
	previousHash, _ := integrity["previousHash"].(string)
	if previousHash == "" || anchor["Mission-State-Hash"] != previousHash {
		return "", false
	}
	// The anchor either matches the state's cycle count exactly (a
	// ledger-untouched write) or lags it by the ONE block that write
	// booked (slice-6 successor finding 3) — the byte checks below pin
	// which shape this is.
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	anchorCycle, aerr := strconv.ParseInt(anchor["Mission-Cycle"], 10, 64)
	if aerr != nil || (anchorCycle != stateCycles && anchorCycle != stateCycles-1) {
		return "", false
	}
	// The tip must BE a complete anchor, not merely claim the right
	// trailers (round-7): its recorded path is this mission's ledger and
	// its committed blob hashes to the declared sha — which the live
	// check below then ties to the present bytes.
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil || anchor["Mission-Ledger-Path"] != ledgerRel {
		return "", false
	}
	blob, code := gitTry(repo, "show", anchor["commit"]+":"+ledgerRel)
	if code != 0 || sha256Hex(blob) != anchor["Mission-Ledger-SHA256"] {
		return "", false
	}
	data, err := os.ReadFile(ledgerPath)
	if err != nil {
		return "", false
	}
	current := string(data)
	if sha256Hex(current) == anchor["Mission-Ledger-SHA256"] {
		if anchorCycle == stateCycles {
			return sha256Hex(current), true
		}
		return "", false
	}
	// One step further (slice-6 successor finding 3): the state write
	// that also BOOKED one ledger block, crashed before its anchor. The
	// current bytes must be the anchored bytes plus exactly that block —
	// clean append, correct count, counted by the state itself.
	if !strings.HasPrefix(current, blob) {
		return "", false
	}
	suffix := strings.TrimLeft(current[len(blob):], "\n")
	count, cerr := cycleCountOf(current)
	if cerr != nil {
		return "", false
	}
	if int64(count) != anchorCycle+1 || stateCycles != int64(count) {
		return "", false
	}
	if !pendingStampMatches(ledgerPath, int64(count), current) {
		return "", false
	}
	if !strings.HasPrefix(suffix, fmt.Sprintf("### Cycle %d", count)) {
		return "", false
	}
	return sha256Hex(current), true
}

// stopLossResetForgivable is the exact reconciliation tolerance: (a) the
// mission state is parked with the stagnation stop-loss reason, and (b) the
// ledger's unanchored suffix consists solely of `Stop-loss reset:` lines each
// naming an ask that exists on disk as a stagnation stop-loss ask. Anything
// else parks on disagreement as today.
func stopLossResetForgivable(statePath, repo string, state map[string]any, ledgerPath string) bool {
	if state["status"] != "parked" || state["parkReason"] != "stop-loss" {
		return false
	}
	anchored, current, err := anchoredLedgerPrefix(repo, state, ledgerPath)
	if err != nil {
		return false
	}
	suffix := current[len(anchored):]
	asksDir := filepath.Join(filepath.Dir(statePath), "asks")
	sawReset := false
	for _, line := range strings.Split(suffix, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		m := resetLineRe.FindStringSubmatch(line)
		if m == nil || !isStagnationStopLossAsk(filepath.Join(asksDir, m[1]+".json"), m[1]) {
			return false
		}
		sawReset = true
	}
	return sawReset
}

// isStagnationStopLossAsk reports whether the file holds the named ask and
// that ask is a stagnation stop-loss ask — the only kind whose answer may
// have written a reset line.
func isStagnationStopLossAsk(path, askID string) bool {
	ask, err := readJSONObjectFile(path)
	if err != nil {
		return false
	}
	return ask["askId"] == askID && ask["reasonClass"] == "stop-loss" &&
		ask["stopLossKind"] == StopLossKindStagnation
}

// reconcileCorruptState preserves the corrupt bytes as evidence and starts a
// visibly recovered chain, leaving the mission parked.
func reconcileCorruptState(statePath string, raw map[string]any) (int, error) {
	data, err := os.ReadFile(statePath)
	if err != nil {
		return 1, err
	}
	corruptHash := sha256Hex(string(data))
	evidence := filepath.Join(filepath.Dir(statePath), "state.corrupt."+corruptHash+".json")
	if !fileExists(evidence) {
		_ = os.WriteFile(evidence, data, 0o644)
	}
	// A corrupt state that carries WALL HISTORY — acceptance entries or
	// taint entries — is never re-rooted (slice-4 critique R2-1): the
	// re-root builds a fresh genesis with no transition validation, so an
	// erased or rewritten acceptance entry would launder into a valid
	// chain and the consumption index would forget it. The evidence is
	// preserved above; the human repairs or re-provisions.
	if turnLog, _ := raw["turnLog"].([]any); true {
		for _, item := range turnLog {
			entry, _ := item.(map[string]any)
			if entry == nil {
				continue
			}
			_, hasWall := entry["wall"]
			_, hasConsumed := entry["consumedAuthorizations"]
			if hasWall || hasConsumed {
				return 3, stateErr("mission state is corrupt and carries wall acceptance history; automatic recovery refused — evidence preserved at %s", filepath.Base(evidence))
			}
		}
	}
	if taint, _ := raw["workspaceTaint"].(map[string]any); taint != nil {
		if entries, _ := taint["entries"].([]any); len(entries) > 0 {
			return 3, stateErr("mission state is corrupt and carries taint history; automatic recovery refused — evidence preserved at %s", filepath.Base(evidence))
		}
	}
	integrity, _ := raw["integrity"].(map[string]any)
	if integrity == nil {
		integrity = map[string]any{}
		raw["integrity"] = integrity
	}
	hash, _ := integrity["hash"].(string)
	if !hashRe.MatchString(hash) {
		integrity["hash"] = strings.Repeat("0", 64)
	}
	history, _ := integrity["history"].([]any)
	if history == nil {
		history = []any{}
	}
	integrity["history"] = history
	integrity["sequence"] = len(history) - 1
	if len(history) > 1 {
		prev, _ := history[len(history)-2].(map[string]any)
		integrity["previousHash"] = prev["hash"]
	} else {
		integrity["previousHash"] = nil
	}
	raw["status"] = "parked"
	raw["parkReason"] = "state-integrity"
	raw["gatePassed"] = false
	finalized, err := finalizeNext(raw, nil, corruptHash)
	if err != nil {
		return 1, err
	}
	return 3, atomicWriteJSON(statePath, finalized)
}

// VerifyStateWithAnchor validates a state's shape and, given a repo and ledger,
// its anchor, returning the sequence and hash.
func VerifyStateWithAnchor(statePath, repo, ledgerPath string) (int64, string, error) {
	state, err := readStateDoc(statePath)
	if err != nil {
		return 0, "", err
	}
	if err := validate(state); err != nil {
		return 0, "", err
	}
	if err := verifyAnchor(repo, state, ledgerPath); err != nil {
		return 0, "", err
	}
	integrity, _ := state["integrity"].(map[string]any)
	seq, _ := intValue(integrity["sequence"])
	h, _ := integrity["hash"].(string)
	return seq, h, nil
}
