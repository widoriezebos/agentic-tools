package plain

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

func gatesPath(install string) string { return filepath.Join(Dir(install), "gates.jsonl") }

func (s ProveSeams) resultsPath(install string) string {
	if s.Gate {
		return gatesPath(install)
	}
	return resultsPath(install)
}

func (s ProveSeams) checkBudget(install, checkout, commit string) error {
	if s.Gate {
		return nil
	}
	return checkProofBudget(install, checkout, commit, s)
}

// LastGate is the newest cheap check, never evidence for a push.
func LastGate(install string) (Result, bool, error) {
	results, err := readLines[Result](gatesPath(install))
	if err != nil || len(results) == 0 {
		return Result{}, false, err
	}
	return results[len(results)-1], true, nil
}

// gateBaseline requires a green cheap check of the merge's first parent.
// Its result and repeat belong to that parent's tree, independently of HEAD.
func gateBaseline(seams ProveSeams, install, checkout, command string, running Running, output io.Writer) (Result, bool, scopeDecision) {
	decision := scopeDecision{scopeRecord: scopeRecord{Scope: "gate"}}
	baseline := Result{Result: Red, Commit: running.Commit, Tree: running.Tree, Attempt: running.Attempt, Log: running.Log,
		At: seams.now().Format(time.RFC3339Nano), Scope: "gate", Cause: &Cause{Kind: "unclassified", Evidence: running.Log}}
	parents, err := seams.git(checkout, "show", "-s", "--format=%P", running.Commit)
	if err != nil || len(strings.Fields(parents)) != 2 {
		baseline.Reason = "the cheap check needs HEAD to be a merge with two parents"
		return baseline, false, decision
	}
	parent := strings.Fields(parents)[0]
	batch, checkErr := CheckBatch(install, checkout, parent, "", true, seams)
	if checkErr != nil {
		baseline.Reason = checkErr.Error()
		return baseline, false, decision
	}
	if batch != nil {
		baseline.BatchID, baseline.BatchMembers = batch.ID, batch.Members
	}
	tree, err := seams.git(checkout, "rev-parse", "--verify", parent+"^{tree}")
	if err != nil {
		baseline.Reason = "the tree before the merge could not be read: " + err.Error()
		return baseline, false, decision
	}
	decision.Base = tree
	previous, found, err := seams.checkBound(install, tree)
	if err != nil {
		if found {
			return previous, false, decision
		}
		baseline.Reason = err.Error()
		return baseline, false, decision
	}
	if found && previous.Result == Green {
		return previous, true, decision
	}
	baseRun := Running{Person: running.Person, Gate: true, Commit: parent, Tree: tree, Attempt: running.Attempt + "-baseline",
		Log: filepath.Join(Dir(install), "proofs", running.Attempt+"-baseline.log")}
	baseRun.BatchID, baseRun.BatchMembers = running.BatchID, running.BatchMembers
	baseline.Person = running.Person
	baseline.Commit, baseline.Tree, baseline.Attempt, baseline.Log = parent, tree, baseRun.Attempt, baseRun.Log
	baseline.Cause.Evidence = baseRun.Log
	baseline.Goals, err = goalsInCommit(install, checkout, parent, seams.git)
	if err != nil {
		baseline.Reason = err.Error()
		return baseline, false, decision
	}
	if found {
		previous.Repeat = "started"
		err = withLock(install, func() error { return appendLine(gatesPath(install), previous) })
	}
	if err == nil {
		err = os.MkdirAll(filepath.Dir(baseRun.Log), 0o755)
	}
	var file *os.File
	if err == nil {
		file, err = os.Create(baseRun.Log)
	}
	if err != nil {
		baseline.Reason = "the baseline log could not be made: " + err.Error()
		baseline.Cause.Kind, baseline.Cause.Name = "environment", "lost-process"
		if !found {
			baseline.Repeat = "allowed"
		}
		return baseline, false, decision
	}
	defer file.Close()
	// The tree before the merge is gated with its own committed
	// declaration, never HEAD's: a goal that changes proof.cheap must not
	// make main's baseline run a command main does not declare.
	baseCommand := command
	if seams.CommandForCommit != nil {
		declared, err := seams.CommandForCommit(parent)
		if err != nil {
			baseline.Reason = "the tree before the merge declares no cheap check it can run: " + err.Error()
			return baseline, false, decision
		}
		baseCommand = declared
	}
	seams.gateBaseline = true
	baseDecision := decision
	baseline.Result, baseline.Cause = Green, nil
	baseline = proveInWorktree(seams, install, checkout, baseCommand, baseRun, &baseDecision, file, &proofOutput{output: io.Discard}, baseline, previous)
	baseline.At = seams.now().Format(time.RFC3339Nano)
	baseline = baseDecision.describe(baseline, &proofOutput{})
	if baseline.Result != Green {
		return baseline, false, decision
	}
	if err := withLock(install, func() error {
		if err := appendLine(gatesPath(install), baseline); err != nil {
			return err
		}
		return closeProofAdmissionStopsLocked(install, baseRun, seams.now())
	}); err != nil {
		baseline.Result, baseline.Reason, baseline.Cause = Red, err.Error(), &Cause{Kind: "unclassified", Evidence: baseRun.Log}
		return baseline, false, decision
	}
	return baseline, true, decision
}

func replayGate(seams ProveSeams, install, checkout, command string, running Running, result, previous Result) Result {
	result.Cause = &Cause{Kind: "unclassified", Tests: failingTests(result.Failed), Evidence: result.Log}
	main, err := checkoutGit(checkout, seams).main()
	if err != nil {
		return result
	}
	trees := []replayTree{{Running: Running{Commit: running.Commit}, Main: running.Commit == main}}
	if !seams.gateBaseline {
		parents, err := seams.git(checkout, "show", "-s", "--format=%P", running.Commit)
		fields := strings.Fields(parents)
		if err != nil || len(fields) != 2 {
			return result
		}
		index := slices.IndexFunc(result.Goals, func(g GoalSHA) bool { return g.SHA == fields[1] })
		if index < 0 {
			return result
		}
		trees = []replayTree{{Running: Running{Commit: fields[0]}, Main: fields[0] == main},
			{Running: Running{Commit: running.Commit}, Goal: result.Goals[index]}}
	}
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0o755); err != nil {
		result.Reason += fmt.Sprintf("; the isolated checks' logs could not be made: %v", err)
		return result
	}
	return classifyReplay(seams, install, checkout, command, running, result, previous, trees)
}
