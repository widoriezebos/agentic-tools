package mission

import (
	"crypto/sha256"
	"encoding/hex"
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
	err := cmd.Run()
	if err == nil {
		return stdout.String(), 0
	}
	if exit, ok := err.(*exec.ExitError); ok {
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
func latestAnchor(repo, mission string) (map[string]string, error) {
	output, err := gitOutput(repo, "log", "--format=%H%x1f%B%x1e")
	if err != nil {
		return nil, err
	}
	for _, raw := range strings.Split(output, "\x1e") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		commit, message, _ := strings.Cut(raw, "\x1f")
		trailers := map[string]string{}
		for _, m := range trailerRe.FindAllStringSubmatch(message, -1) {
			trailers[m[1]] = m[2]
		}
		if trailers["Mission-Id"] == mission {
			trailers["commit"] = strings.TrimSpace(commit)
			return trailers, nil
		}
	}
	return nil, stateErr("mission state has no local anchor commit")
}

// Anchor writes the mission's anchor commit and prints the resulting sha.
func Anchor(statePath, repo, ledgerPath string) error {
	state, err := readStateDoc(statePath)
	if err != nil {
		return err
	}
	if err := validate(state); err != nil {
		return err
	}
	cycles, err := ledgerCycleCount(ledgerPath)
	if err != nil {
		return err
	}
	ledger, _ := state["ledger"].(map[string]any)
	stateCycles, _ := intValue(ledger["cycles"])
	if int64(cycles) != stateCycles {
		return stateErr("anchor refused: ledger is truth and its cycle count disagrees with state")
	}
	branch, err := gitOutput(repo, "branch", "--show-current")
	if err != nil {
		return err
	}
	stateBranch, _ := state["branch"].(string)
	if strings.TrimSpace(branch) != stateBranch {
		return stateErr("anchor refused: current branch is not the mission branch")
	}
	if _, code := gitTry(repo, "diff", "--cached", "--quiet"); code != 0 {
		return stateErr("anchor refused: staged changes would be swept into the local anchor commit")
	}
	ledgerRel, err := relUnderRepo(ledgerPath, repo)
	if err != nil {
		return stateErr("anchor refused: mission ledger is outside the repository")
	}
	clearIndexLock(repo)
	if _, err := gitOutput(repo, "add", "-f", "--", ledgerRel); err != nil {
		return err
	}
	missionID, _ := state["missionId"].(string)
	integrity, _ := state["integrity"].(map[string]any)
	stateHashValue, _ := integrity["hash"].(string)
	lHash, err := ledgerHash(ledgerPath)
	if err != nil {
		return err
	}
	subject := fmt.Sprintf("mission(%s): anchor cycle %d", missionID, cycles)
	body := fmt.Sprintf("Mission-Id: %s\nMission-State-Hash: %s\nMission-Ledger-SHA256: %s\nMission-Ledger-Path: %s\nMission-Cycle: %d",
		missionID, stateHashValue, lHash, ledgerRel, cycles)

	// The anchor is a lease-holder mutation. Where the target carries the
	// pre-commit guard, a raw commit is refused, so route through the commit
	// wrapper that establishes the holder token; a target without the guard has
	// no wrapper requirement and a raw commit is correct.
	guard := filepath.Join(repo, ".git", "hooks", "pre-commit")
	wrapper := filepath.Join(repo, "scripts", "agents", "commit.sh")
	var commitCommand []string
	if fileExists(guard) && fileExists(wrapper) {
		commitCommand = []string{wrapper, "--allow-empty", "-m", subject, "-m", body}
	} else {
		// The anchor is a machine bookkeeping commit: it carries its own
		// identity, exactly like the fixture repositories' commits, instead
		// of borrowing whatever ambient git config the host happens to have
		// — a pristine host has none, and the first Linux acceptance run
		// proved this path failed there (go-production-grade Phase 1).
		commitCommand = []string{"git", "-C", repo,
			"-c", "user.name=metasystem", "-c", "user.email=metasystem@example.invalid",
			"commit", "--allow-empty", "-m", subject, "-m", body}
	}
	for attempt := 0; attempt < 6; attempt++ {
		cmd := exec.Command(commitCommand[0], commitCommand[1:]...)
		var stdout, stderr strings.Builder
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if cmd.Run() == nil {
			break
		}
		if !strings.Contains(stderr.String(), "index.lock") || attempt == 5 {
			out := strings.TrimSpace(stderr.String())
			if out == "" {
				out = strings.TrimSpace(stdout.String())
			}
			return stateErr("anchor commit failed: %s", out)
		}
		clearIndexLock(repo)
	}
	head, err := gitOutput(repo, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	fmt.Println(strings.TrimSpace(head))
	return nil
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
	if _, code := gitTry(repo, "merge-base", "--is-ancestor", anchor["commit"], mustBranch(state)); code != 0 {
		return stateErr("mission anchor commit is not on the mission branch")
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
	if _, code := gitTry(repo, "merge-base", "--is-ancestor", anchor["commit"], mustBranch(state)); code != 0 {
		return "", "", stateErr("mission anchor commit is not on the mission branch")
	}
	return anchored, string(data), nil
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
	case int64(cycles) > stateCycles:
		if err := verifyStateAnchor(repo, raw, ledgerPath); err != nil {
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
		return 0, atomicWriteJSON(statePath, finalized)
	default:
		if err := verifyAnchor(repo, raw, ledgerPath); err != nil {
			// One named, checkable exception (plans/stop-loss-core.md): a
			// stagnation-parked mission whose unanchored ledger suffix is
			// solely vocal stop-loss reset lines is replayable state — a
			// crash between the reset append and its anchor — not divergence.
			if stopLossResetForgivable(statePath, repo, raw, ledgerPath) {
				return 0, nil
			}
			return 3, parkIntegrity(statePath, raw, nil)
		}
	}
	return 0, nil
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
