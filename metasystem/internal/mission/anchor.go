package mission

import (
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
	"time"
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

// clearIndexLock waits briefly for a live lock holder, then removes a dead
// holder's leftover .git/index.lock. Waiting on a corpse's lock is how a run
// can hang forever; git's own advice is to remove a stale one.
func clearIndexLock(repo string) {
	lock := filepath.Join(repo, ".git", "index.lock")
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(lock); os.IsNotExist(err) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	info, err := os.Stat(lock)
	if err != nil {
		return
	}
	if age := time.Since(info.ModTime()).Seconds(); age >= 4 {
		fmt.Fprintf(os.Stderr, "removing stale index.lock (age %.0fs, holder presumed dead)\n", age)
		_ = os.Remove(lock)
	}
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
	text := string(data)
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

// anchorWrite is the printing-free anchor used by reconciliation's
// retry-safe heal as well as the public verb; it returns the commit sha.
func anchorWrite(statePath, repo, ledgerPath string) (string, error) {
	state, err := readStateDoc(statePath)
	if err != nil {
		return "", err
	}
	if err := validate(state); err != nil {
		return "", err
	}
	cycles, err := ledgerCycleCount(ledgerPath)
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
	lHash, err := ledgerHash(ledgerPath)
	if err != nil {
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
	blob, err := gitOutput(repo, "hash-object", "-w", "--no-filters", "--", resolvePath(ledgerPath))
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
	identity := []string{
		"GIT_AUTHOR_NAME=metasystem", "GIT_AUTHOR_EMAIL=metasystem@example.invalid",
		"GIT_COMMITTER_NAME=metasystem", "GIT_COMMITTER_EMAIL=metasystem@example.invalid",
	}
	commitArgs := []string{"commit-tree", strings.TrimSpace(tree), "-m", subject + "\n\n" + body}
	oldValue := strings.TrimSpace(parent)
	if oldValue != "" {
		commitArgs = append(commitArgs, "-p", oldValue)
	}
	commit, err := gitEnvOutput(repo, identity, commitArgs...)
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

// verifyStateAnchor checks that the current ledger extends the anchored ledger
// truth (used when reconciling a ledger that has grown since the anchor).
func verifyStateAnchor(repo string, state map[string]any, ledgerPath string) error {
	_, _, err := anchoredLedgerPrefix(repo, state, ledgerPath)
	return err
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

// AnchoredLedgerTruth exposes the authenticated anchored-ledger
// comparison to the wall's in-turn guard (slice-5 round-4): the anchored
// bytes come from the runner-owned anchor ref with every cross-check —
// state hash, cycle count, path, sha — applied before anything is
// trusted, never from whatever commit last touched a path.
func AnchoredLedgerTruth(repo string, state map[string]any, ledgerPath string) (anchored, current string, err error) {
	return anchoredLedgerPrefix(repo, state, ledgerPath)
}

func mustBranch(state map[string]any) string {
	b, _ := state["branch"].(string)
	return b
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
		if errors.Is(err, ErrLegacyState) {
			return 3, err
		}
		return reconcileCorruptState(statePath, raw)
	}
	cycles, err := ledgerCycleCount(ledgerPath)
	if err != nil {
		return 3, parkIntegrity(statePath, raw, nil)
	}
	ledger, _ := raw["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	switch {
	case int64(cycles) < stateCycles:
		return 3, parkIntegrity(statePath, raw, nil)
	case int64(cycles) > stateCycles+1:
		// The reserve/append machinery books at most ONE cycle between
		// state writes, so a ledger more than one block ahead is not a
		// crash window — it is divergence (slice-5 round-5).
		return 3, parkIntegrity(statePath, raw, nil)
	case int64(cycles) > stateCycles:
		if err := verifyStateAnchor(repo, raw, ledgerPath); err != nil {
			return 3, parkIntegrity(statePath, raw, nil)
		}
		// The single tolerated block must be the RESERVED, OPEN cycle
		// (round-6): the fence counters prove the runner reserved it and
		// the open-turn marker proves a turn was in flight for exactly
		// that cycle — contiguity alone proves a number, not authorship.
		if err := verifyReservedGap(repo, raw, int64(cycles)); err != nil {
			return 3, parkIntegrity(statePath, raw, nil)
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
		// hash and anchor in agreement, not park for integrity.
		if _, err := anchorWrite(statePath, repo, ledgerPath); err != nil {
			return 3, parkIntegrity(statePath, finalized, nil)
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
			if anchorLagHealable(repo, raw, ledgerPath) {
				if _, aerr := anchorWrite(statePath, repo, ledgerPath); aerr == nil {
					return 0, nil
				}
			}
			return 3, parkIntegrity(statePath, raw, nil)
		}
	}
	return 0, nil
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
func anchorLagHealable(repo string, state map[string]any, ledgerPath string) bool {
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
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	if anchor["Mission-Cycle"] != strconv.FormatInt(stateCycles, 10) {
		return false
	}
	lHash, err := ledgerHash(ledgerPath)
	if err != nil || anchor["Mission-Ledger-SHA256"] != lHash {
		return false
	}
	// The tip must BE a complete anchor, not merely claim the right
	// trailers (round-7): its recorded path is this mission's ledger and
	// its committed blob hashes to the declared sha — which the live
	// check above then ties to the present bytes.
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil || anchor["Mission-Ledger-Path"] != ledgerRel {
		return false
	}
	blob, code := gitTry(repo, "show", anchor["commit"]+":"+ledgerRel)
	if code != 0 || sha256Hex(blob) != anchor["Mission-Ledger-SHA256"] {
		return false
	}
	return true
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
