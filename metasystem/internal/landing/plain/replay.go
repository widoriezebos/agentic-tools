package plain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// classifyRed applies the first-match cause table. The caller supplies the
// isolated replay over its subject's trees; every repeat uses its existing
// allowance, including a registered flake's repeat in the current worktree.
func classifyRed(seams ProveSeams, install, checkout, command, dir string, running Running, decision scopeDecision, output io.Writer, observed *proofOutput, result, previous Result, report checkReport, runErr error, replay func(Result, Result) Result) Result {
	result.Result, result.Reason, result.Load = Red, runErr.Error(), report.load
	result.Cause = &Cause{Kind: "unclassified", Evidence: running.Log}
	var exit *exec.ExitError
	var launch *exec.Error
	var path *os.PathError
	if report.kind == "not-run" || errors.As(runErr, &exit) && !exit.Exited() || errors.As(runErr, &launch) || errors.As(runErr, &path) {
		result.Cause.Kind, result.Cause.Name = "environment", "lost-process"
		result.CountedFull = false
	}
	result = decision.describe(result, observed)
	if result.Cause.Kind == "environment" && previous.Result == "" {
		result.Repeat = "allowed"
	}
	if report.kind != "complete" {
		return result
	}
	if result.Cause.Kind == "environment" {
		return result
	}
	result.Failed = report.failed
	result.Cause.Tests = failingTests(result.Failed)
	known := len(result.Failed) > 0
	if seams.Judge != nil {
		judged, err := seams.Judge(checkout, running.Commit, result.Failed)
		known = known && err == nil
		for i := range result.Failed {
			unit := &result.Failed[i]
			j, found := judged[unit.Unit]
			unit.Surfaces = j.Surfaces
			known = known && found && !j.Affected && j.Known && len(unit.Tests) > 0
		}
	} else {
		known = false
	}
	if !known || previous.Result != "" {
		return replay(result, previous)
	}
	result.Repeat = "started"
	result.Cause.Kind = "flake"
	result.Cause.Name = strings.Join(result.Cause.Tests, ", ")
	if err := withLock(install, func() error {
		if err := recordProofStop(install, result); err != nil {
			return err
		}
		return appendLine(seams.resultsPath(install), result)
	}); err != nil {
		result.Reason = "the repeat could not be recorded: " + err.Error()
		return result
	}
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0o755); err != nil {
		result.Reason = "the repeat's log folder could not be made: " + err.Error()
		return result
	}
	repeats := make([]Running, len(result.Failed))
	var failures []string
	for i, unit := range result.Failed {
		repeat := running
		repeat.Attempt = fmt.Sprintf("%s-repeat-%d", running.Attempt, i+1)
		repeat.Log = filepath.Join(Dir(install), "proofs", repeat.Attempt+".log")
		repeats[i] = repeat
		file, err := os.OpenFile(repeat.Log, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err == nil {
			_, err = runCheck(seams, dir, command, repeat, unit.Unit, decision, file, &proofOutput{output: io.Discard})
			if info, statErr := file.Stat(); statErr == nil {
				_, copyErr := io.Copy(output, io.NewSectionReader(file, 0, info.Size()))
				err = errors.Join(err, copyErr)
			}
			err = errors.Join(err, file.Close())
		}
		if err != nil {
			failures = append(failures, unit.Unit+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		result.Reason = strings.Join(failures, "; ")
		return replay(result, result)
	}
	green := result
	green.Result, green.Repeat, green.Failed, green.Load, green.Reason, green.Cause = Green, "", nil, 0, "", nil
	return recordFlakes(seams, result, green, "alone", repeats)
}

// replayTree is a prefix of the check's subject, oldest first. Main is true
// only for origin/main; Goal identifies the merge that ends a later prefix.
type replayTree struct {
	Running
	Main bool
	Goal GoalSHA
}

func replayBatch(seams ProveSeams, install, checkout, command string, running Running, result, previous Result) Result {
	result.Cause = &Cause{Kind: "unclassified", Tests: failingTests(result.Failed), Evidence: result.Log}
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0o755); err != nil {
		return result
	}
	main, err := checkoutGit(checkout, seams).main()
	if err != nil {
		return result
	}
	merges := ""
	if !running.Trunk {
		merges, err = seams.git(checkout, "log", "--first-parent", "--merges", "--reverse", "--format=%H %P", "origin/main.."+running.Commit)
	}
	if running.Trunk {
		main = running.Commit
	}
	if err != nil {
		return result
	}
	trees := []replayTree{{Running: Running{Commit: main}, Main: true}}
	for _, line := range strings.Split(strings.TrimSpace(merges), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if len(fields) != 3 || fields[1] != trees[len(trees)-1].Commit {
			return result
		}
		index := slices.IndexFunc(result.Goals, func(g GoalSHA) bool { return g.SHA == fields[2] })
		if index < 0 {
			return result
		}
		trees = append(trees, replayTree{Running: Running{Commit: fields[0]}, Goal: result.Goals[index]})
	}
	return classifyReplay(seams, install, checkout, command, running, result, previous, trees)
}

// classifyReplay attributes only a complete isolated failure after a complete
// isolated green. It neither turns the full red green nor spends a repeat.
func classifyReplay(seams ProveSeams, install, checkout, command string, running Running, result, previous Result, trees []replayTree) Result {
	for i, prefix := range trees {
		if !running.Trunk {
			if _, err := CheckBatch(install, checkout, prefix.Commit, "", true, seams); err != nil {
				result.Reason += "; " + err.Error()
				return result
			}
		}
		prefix.BatchID, prefix.BatchMembers = running.BatchID, running.BatchMembers
		prefix.Attempt = fmt.Sprintf("%s-replay-%d", running.Attempt, i+1)
		var err error
		prefix.Tree, err = seams.git(checkout, "rev-parse", "--verify", prefix.Commit+"^{tree}")
		if err != nil {
			return result
		}
		tree := filepath.Join(proofTrees(install), prefix.Attempt)
		if _, err := seams.git(checkout, "worktree", "add", "--detach", tree, prefix.Commit); err != nil {
			return result
		}
		dir := tree
		if rel, err := filepath.Rel(checkout, install); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			dir = filepath.Join(tree, rel)
		}
		failed, complete := false, true
		var mainChecks []Result
		recordingMain := i == 0 && prefix.Main && prefix.Tree != running.Tree && seams.RecordMain != nil
		for j, unit := range result.Failed {
			prefix.Log = filepath.Join(Dir(install), "proofs", fmt.Sprintf("%s-%d.log", prefix.Attempt, j+1))
			file, err := os.Create(prefix.Log)
			if err != nil {
				complete = false
				result.Reason += "; isolated check of " + unit.Unit + " on " + prefix.Commit + " did not complete"
				break
			}
			result.Cause.Evidence = prefix.Log
			report, runErr := runCheck(seams, dir, command, prefix.Running, unit.Unit, scopeDecision{scopeRecord: scopeRecord{Scope: "full"}}, file, &proofOutput{output: io.Discard})
			closeErr := file.Close()
			var exit *exec.ExitError
			if report.kind != "complete" || closeErr != nil || runErr != nil && (!slices.ContainsFunc(report.failed, func(f FailedUnit) bool { return f.Unit == unit.Unit }) || errors.As(runErr, &exit) && !exit.Exited()) {
				complete = false
				result.Reason += "; isolated check of " + unit.Unit + " on " + prefix.Commit + " did not complete"
				break
			}
			if runErr != nil {
				failed = true
				result.Cause.Evidence = prefix.Log
				if recordingMain {
					units := slices.DeleteFunc(slices.Clone(report.failed), func(f FailedUnit) bool { return f.Unit != unit.Unit })
					mainChecks = append(mainChecks, Result{Result: Red, Commit: prefix.Commit, Tree: prefix.Tree,
						Attempt: prefix.Attempt, Log: prefix.Log, At: result.At, Failed: units})
					continue
				}
				break
			}
		}
		_, removeErr := seams.git(checkout, "worktree", "remove", "--force", tree)
		if i == 0 && prefix.Main && failed {
			result.Cause.Kind = "main"
			result.Cause.Name = mainFailureIdentity(result.Failed)
			if len(mainChecks) > 0 {
				result.Cause.Evidence = mainChecks[0].Log
				result = recordMainFailures(seams, result, mainChecks)
			}
		}
		if !complete || removeErr != nil {
			return result
		}
		if failed {
			if i > 0 && prefix.Goal.Goal != "" {
				result.Cause.Kind, result.Cause.Goal, result.Cause.SHA = "own", prefix.Goal.Goal, prefix.Goal.SHA
			}
			return result
		}
	}
	if len(trees) > 0 && len(result.Failed) > 0 && previous.Result == "" {
		result.Repeat = "allowed"
	}
	return result
}

// Recording an incident preserves the proof's verdict and repeat allowance.
func recordMainFailures(seams ProveSeams, result Result, checks []Result) Result {
	if seams.RecordMain != nil {
		if err := seams.RecordMain(checks); err != nil {
			result.Reason += "; main's failed tests could not be recorded: " + err.Error()
		} else if len(checks) > 0 && result.Cause != nil {
			result.Cause.Kind, result.Cause.Name = "main", mainFailureIdentity(checks[0].Failed)
		}
	}
	return result
}

// Main incidents are keyed by the failing unit and, when known, its test.
func mainFailureIdentity(failed []FailedUnit) string {
	if len(failed) == 0 {
		return ""
	}
	identity := "red:" + failed[0].Unit
	if len(failed[0].Tests) > 0 {
		identity += ":" + failed[0].Tests[0]
	}
	return identity
}

// ProofBudget refuses a third completed full check in the open batch loop.
type ProofBudget struct{}

func (*ProofBudget) Error() string {
	return "this batch used two full checks; goals hold; ask a person to run: metasystem landing run"
}

func subsetGoals(goals, first []GoalSHA) bool {
	return len(goals) > 0 && !slices.ContainsFunc(goals, func(g GoalSHA) bool {
		return !slices.ContainsFunc(first, func(f GoalSHA) bool { return f.Goal == g.Goal })
	})
}

// checkProofBudget runs under the lane lock, before a start or repeat marker.
func checkProofBudget(install, checkout, commit string, seams ProveSeams) error {
	results, err := Results(install)
	if err != nil {
		return err
	}
	var first []GoalSHA
	firstRed := false
	attempts := map[string]bool{}
	for _, r := range results {
		if r.LoopClosed {
			first, firstRed, attempts = nil, false, map[string]bool{}
			continue
		}
		if r.Trunk || !r.CountedFull || len(r.Goals) == 0 {
			continue
		}
		if !subsetGoals(r.Goals, first) {
			first, firstRed, attempts = nil, false, map[string]bool{}
		}
		if first == nil || !firstRed && r.Result == Red {
			first = r.Goals
		}
		firstRed = firstRed || r.Result == Red
		if first != nil {
			attempts[r.Attempt] = true
		}
	}
	if len(attempts) < 2 {
		return nil
	}
	goals, err := goalsInCommit(install, checkout, commit, seams.git)
	if err != nil {
		return err
	}
	if subsetGoals(goals, first) {
		return &ProofBudget{}
	}
	return nil
}

// CloseProofLoop closes the batch budget while preserving its newest proof.
func CloseProofLoop(install string) error {
	return withLock(install, func() error {
		if err := closeProofLoop(install); err != nil {
			return err
		}
		return closeStopsLocked(install, "", "landing run", time.Now())
	})
}

func closeProofLoop(install string) error {
	last, ok, err := LastResult(install)
	if err != nil || !ok || last.LoopClosed {
		return err
	}
	last.LoopClosed = true
	return appendLine(resultsPath(install), last)
}
