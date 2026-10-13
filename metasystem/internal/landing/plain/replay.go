package plain

import (
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// classifyRed applies the first-match cause table. The caller supplies the
// isolated replay over its subject's trees; every repeat uses its existing
// allowance, including a registered flake's repeat in the current worktree.
func classifyRed(seams ProveSeams, install, checkout, command, dir string, running Running, decision scopeDecision, output io.Writer, observed *proofOutput, result, previous Result, report checkReport, runErr error, replay func(Result, Result) Result) Result {
	result = observeRed(seams, install, running, decision, observed, result, previous, report, runErr)
	if result.executionErr != nil || result.Result == "cancelled" {
		return result
	}
	return continueRed(seams, install, checkout, command, dir, running, decision, output, result, previous, replay)
}

// allowEnvironmentRepeat grants one retry when a tree's first check could not
// be proved because of its environment. A later failure spends that allowance.
func (r *Result) allowEnvironmentRepeat(previous Result) {
	if r.Cause != nil && r.Cause.Kind == "environment" && previous.Result == "" {
		r.Repeat = "allowed"
	}
}

func observeRed(seams ProveSeams, install string, running Running, decision scopeDecision, observed *proofOutput, result, previous Result, report checkReport, runErr error) Result {
	result.LoadStart, result.LoadEnd = report.loadStart, report.loadEnd
	result = timedResult(result, running.Since, seams.now())
	result.Result, result.Reason, result.Load = Red, runErr.Error(), report.load
	if result.Load == 0 {
		if sample := hostload.Read(seams.now()); sample.Available {
			result.Load = sample.Load1m
		}
	}
	result.Cause = &Cause{Kind: "unclassified", Evidence: running.Log}
	var exit *exec.ExitError
	var launch *exec.Error
	var path *os.PathError
	if report.kind == "not-run" || errors.As(runErr, &exit) && !exit.Exited() || errors.As(runErr, &launch) || errors.As(runErr, &path) {
		result.Cause.Kind, result.Cause.Name = "environment", "lost-process"
		result.CountedFull = false
	}
	result = decision.describe(result, observed)
	result.allowEnvironmentRepeat(previous)
	result.Failed = report.failed
	if slices.ContainsFunc(result.Failed, func(unit FailedUnit) bool { return unit.Unit == "fast-static-build" }) {
		result.Reason = "fast-static-build: " + result.Reason
	}
	result.Cause.Tests = failingTests(result.Failed)
	result.Person, result.Executions = running.Person, running.Executions
	result.ClassificationOf = running.ClassificationOf
	result.ClassificationPending = true
	if err := withLock(install, func() error {
		cancel, err := readCancel(install, running)
		if err != nil {
			return err
		}
		if cancel != nil {
			result = *cancel
			return nil
		}
		if err := appendLine(seams.resultsPath(install), result, seams); err != nil {
			return err
		}
		return redContinuationLocked(install, result, seams)
	}); err != nil {
		result.Reason += "; " + err.Error()
		result.executionErr = err
		return result
	}
	return result
}

// continueRed uses the recorded report; it never repeats the original full admission.
func continueRed(seams ProveSeams, install, checkout, command, dir string, running Running, decision scopeDecision, output io.Writer, result, previous Result, replay func(Result, Result) Result) Result {
	result.ClassificationPending = false
	// Only a flake repeat (it carries FlakeRepeats) is held here; a whole-check,
	// environment or person re-proof is also "started" and keeps the lane's attribution.
	if result.Repeat == "started" && len(result.FlakeRepeats) > 0 {
		if !result.RepeatComplete {
			return result
		}
		green := result
		green.Result, green.Failed, green.Cause = Green, nil, nil
		return recordFlakes(seams, result, green, "alone", result.FlakeRepeats)
	}
	if result.Cause.Kind == "environment" || len(result.Failed) == 0 {
		return result
	}
	if running.Trunk {
		if !slices.ContainsFunc(result.Failed, func(unit FailedUnit) bool { return len(unit.Tests) > 0 }) {
			return replay(result, previous)
		}
		repeated := repeatRed(seams, install, command, dir, running, decision, result, true)
		if repeated.Result == Red && repeated.Cause.Kind == "unclassified" {
			return replay(repeated, previous)
		}
		return repeated
	}
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
	repeated := repeatRed(seams, install, command, dir, running, decision, result, false)
	if len(repeated.FlakeRepeats) == 0 {
		return replay(repeated, previous)
	}
	return repeated
}

// repeatRed spends one repeat in the same worktree before a red is attributed.
// Main checks bind the allowance to the attempt; batch checks bind it to the tree.
func repeatRed(seams ProveSeams, install, command, dir string, running Running, decision scopeDecision, result Result, main bool) Result {
	ordinary, ordinaryRepeat := *result.Cause, result.Repeat
	var retained *Result
	result.FlakeRepeats = make([]Running, len(result.Failed))
	for i := range result.Failed {
		repeat := running
		repeat.Attempt = fmt.Sprintf("%s-repeat-%d", running.Attempt, i+1)
		repeat.Log = filepath.Join(Dir(install), "proofs", repeat.Attempt+".log")
		result.FlakeRepeats[i] = repeat
	}
	result.Repeat = "started"
	result.Cause.Kind = "flake"
	result.Cause.Name = strings.Join(result.Cause.Tests, ", ")
	if err := withLock(install, func() error {
		history, err := readLines[Result](seams.resultsPath(install))
		if err != nil {
			return err
		}
		for i := len(history) - 1; i >= 0; i-- {
			prior := history[i]
			if main && prior.Attempt == result.Attempt && (prior.Repeat == "started" || prior.FlakePublished) {
				retained = &prior
				return &NoRepeat{}
			}
			if !main && prior.Repeat == "started" && prior.Tree == result.Tree {
				return &NoRepeat{}
			}
		}
		if err := recordProofStop(install, result, seams); err != nil {
			return err
		}
		return appendLine(seams.resultsPath(install), result, seams)
	}); err != nil {
		var refused *NoRepeat
		if errors.As(err, &refused) {
			if retained != nil {
				if retained.RepeatComplete && !retained.FlakePublished {
					green := *retained
					green.Result, green.Cause, green.Failed = Green, nil, nil
					return recordFlakes(seams, *retained, green, "alone", retained.FlakeRepeats)
				}
				return *retained
			}
			// The tree's one repeat is spent: the red keeps its ordinary cause.
			result.FlakeRepeats, result.Repeat = nil, ordinaryRepeat
			*result.Cause = ordinary
			return result
		}
		result.Reason = "the repeat could not be recorded: " + err.Error()
		return result
	}
	if err := os.MkdirAll(filepath.Join(Dir(install), "proofs"), 0o755); err != nil {
		result.Reason = "the repeat's log folder could not be made: " + err.Error()
		return result
	}
	repeats := result.FlakeRepeats
	contract, _ := testpolicy.Load(filepath.Join(dir, "testing.json"))
	var failures []string
	uncertain := false
	for i, unit := range result.Failed {
		repeat := repeats[i]
		repeat.Since = seams.now().UTC().Format(time.RFC3339)
		var report checkReport
		file, err := os.OpenFile(repeat.Log, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
		if err == nil {
			tests, _ := namedUnitTests(unit, contract)
			report, err = runCheck(seams, dir, command, repeat, unit.Unit, decision, file, &proofOutput{output: io.Discard}, tests...)
			if err == nil && (report.kind != "complete" || len(report.failed) != 0) {
				err = fmt.Errorf("the repeat has no complete passing report")
			}

			err = errors.Join(err, file.Close())
		}
		repeat.Minutes = elapsedMinutes(repeat.Since, seams.now().UTC().Format(time.RFC3339))
		repeats[i] = repeat
		if err != nil {
			result.Cause.Kind = "unclassified"
			var exit *exec.ExitError
			if report.kind != "complete" || main && (len(report.failed) == 0 || errors.As(err, &exit) && !exit.Exited()) {
				uncertain = true
				result.Cause.Kind, result.Cause.Name = "environment", "lost-process"
			}
			failures = append(failures, unit.Unit+": "+err.Error())
		}
	}
	if len(failures) > 0 {
		if uncertain {
			result.Cause.Kind, result.Cause.Name = "environment", "lost-process"
		}
		result.Reason = strings.Join(failures, "; ")
		result = recordFlakes(seams, result, result, "alone", repeats)
		if main {
			if err := withLock(install, func() error { return appendLine(seams.resultsPath(install), result, seams) }); err != nil {
				result.Cause.Kind = "flake"
				result.Reason += "; " + err.Error()
			}
		}
		return result
	}
	result.RepeatComplete = true
	if err := withLock(install, func() error { return appendLine(seams.resultsPath(install), result, seams) }); err != nil {
		result.Reason = "the repeat's outcome could not be recorded: " + err.Error()
		return result
	}
	green := result
	green.Result, green.Repeat, green.Failed, green.Load, green.Reason, green.Cause = Green, "", nil, 0, "", nil
	green = recordFlakes(seams, result, green, "alone", repeats)
	if main {
		if err := withLock(install, func() error { return appendLine(seams.resultsPath(install), green) }); err != nil {
			result.Reason = "the repeat publication could not be recorded: " + err.Error()
			return result
		}
	}
	return green
}

// namedUnitTests selects only top-level Go tests in package units. Groups keep
// their whole command, and an invalid identity keeps every test in its unit.
func namedUnitTests(unit FailedUnit, contract testpolicy.Contract) ([]string, bool) {
	if len(unit.Tests) == 0 || !strings.Contains(unit.Unit, "/") || strings.HasPrefix(unit.Unit, "section/") || slices.ContainsFunc(contract.Groups, func(group testpolicy.Group) bool { return group.ID == unit.Unit }) {
		return nil, false
	}
	if slices.ContainsFunc(unit.Tests, func(name string) bool {
		return !token.IsIdentifier(name) || !strings.HasPrefix(name, "Test") || name == "TestMain"
	}) {
		return nil, true
	}
	return unit.Tests, false
}

// replayTree is a prefix of the check's subject, oldest first. Main is true
// for the batch's main base; Goal identifies the merge ending a later prefix.
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
	main, merges := running.Commit, ""
	var err error
	if !running.Trunk {
		fetchedMain, readErr := checkoutGit(checkout, seams).main()
		if readErr != nil {
			return result
		}
		if running.Commit != fetchedMain {
			main, err = replayBase(checkout, running.Commit, fetchedMain, seams)
			if err != nil {
				return result
			}
			if fix, fixErr := currentFix(install, seams); fixErr == nil && fix != nil && fix.Commit == running.Commit {
				batch, batchErr := ReadBatch(install)
				if batchErr != nil || batch == nil {
					return result
				}
				main = batch.Base
			}
			merges, err = seams.git(checkout, "log", "--first-parent", "--merges", "--reverse", "--format=%H %P", main+".."+running.Commit)
		}
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
	if fix, err := currentFix(install, seams); err == nil && fix != nil && fix.Commit == running.Commit {
		for i := range trees {
			if trees[i].Goal.Goal == fix.Goal {
				trees[i].Commit = fix.Commit
				trees = trees[:i+1]
				break
			}
		}
	}
	return classifyReplay(seams, install, checkout, command, running, result, previous, trees)
}

// replayBase finds the first main commit on the batch's first-parent chain.
func replayBase(checkout, tip, main string, seams ProveSeams) (string, error) {
	seen := map[string]bool{}
	for commit := tip; commit != "" && !seen[commit]; {
		seen[commit] = true
		if commit == main {
			return commit, nil
		}
		inside, err := checkoutGit(checkout, seams).contains(main, commit)
		if err != nil {
			return "", err
		}
		if inside {
			return commit, nil
		}
		parents, err := seams.git(checkout, "show", "-s", "--format=%P", commit)
		fields := strings.Fields(parents)
		if err != nil || len(fields) == 0 {
			return "", fmt.Errorf("cannot read the batch's main base at %s: %v", commit, err)
		}
		commit = fields[0]
	}
	return "", fmt.Errorf("the batch's first-parent chain does not reach main")
}

// classifyReplay attributes only a complete isolated failure after a complete
// isolated green. It neither turns the full red green nor spends a repeat.
func classifyReplay(seams ProveSeams, install, checkout, command string, running Running, result, previous Result, trees []replayTree) Result {
	result.Replay = ""
	for i, prefix := range trees {
		if !running.Trunk && result.ClassificationPerson == nil {
			if _, err := CheckBatch(install, checkout, prefix.Commit, "", true, seams); err != nil {
				result.Reason += "; " + err.Error()
				return result
			}
		}
		if prefix.Commit == running.Commit {
			result.Cause.Evidence = result.Log
			if prefix.Main {
				result.Cause.Kind, result.Cause.Name = "main", mainFailureIdentity(result.Failed)
			} else if prefix.Goal.Goal != "" {
				result.Cause.Kind, result.Cause.Goal, result.Cause.SHA = "own", prefix.Goal.Goal, prefix.Goal.SHA
			}
			return result
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
		units := make([]string, len(result.Failed))
		var tests, forms, wholeForms []string
		named := false
		contract, _ := testpolicy.Load(filepath.Join(dir, "testing.json"))
		for j, unit := range result.Failed {
			units[j] = unit.Unit
			wholeForm := "replay: whole unit " + unit.Unit
			wholeForms = append(wholeForms, wholeForm)
			names, invalid := namedUnitTests(unit, contract)
			if len(names) == 0 {
				if invalid {
					wholeForm = "named replay invalid test identity: whole unit; " + wholeForm
				}
				forms = append(forms, wholeForm)
				continue
			}
			named = true
			tests = append(tests, names...)
			forms = append(forms, fmt.Sprintf("replay: %d named tests of %s", len(names), unit.Unit))
			// Multiple units keep each package's names in its own selector.
			if len(result.Failed) > 1 {
				units[j] += "=" + strings.Join(names, ",")
			}
		}
		prefix.Log = filepath.Join(Dir(install), "proofs", prefix.Attempt+".log")
		file, err := os.Create(prefix.Log)
		if err != nil {
			complete = false
		} else {
			form := strings.Join(forms, "; ")
			result.Cause.Evidence = prefix.Log
			prefix.Since = seams.now().UTC().Format(time.RFC3339)
			run := func(selectors, names []string) (checkReport, bool) {
				report, runErr := runCheck(seams, dir, command, prefix.Running, strings.Join(selectors, " "), scopeDecision{scopeRecord: scopeRecord{Scope: "full"}}, file, &proofOutput{output: io.Discard}, names...)
				var exit *exec.ExitError
				return report, report.kind == "complete" && (runErr == nil || len(report.failed) > 0 && (!errors.As(runErr, &exit) || exit.Exited()))
			}
			report, done := run(units, tests)
			if named && (!done || len(report.failed) == 0) {
				reason := "named replay incomplete: whole unit"
				if done {
					reason = "named replay green on prefix: whole-unit replay ran"
					if prefix.Main {
						reason = "named replay green on main: whole-unit replay ran"
					}
				}
				form = reason + "; " + strings.Join(wholeForms, "; ")
				fmt.Fprintln(file, form)
				for j, unit := range result.Failed {
					units[j] = unit.Unit
				}
				report, done = run(units, nil)
			}
			closeErr := file.Close()
			complete = done && closeErr == nil
			failed = complete && len(report.failed) > 0
			if result.Replay != "" {
				result.Replay += "; "
			}
			result.Replay += form
			if complete && failed && prefix.Main {
				// Isolated checks retain their batch context but cannot authorize a full green.
				check := timedResult(Result{Result: Red, Commit: prefix.Commit, Tree: prefix.Tree, Scope: "scoped",
					Goals: result.Goals, BatchID: running.BatchID, BatchMembers: running.BatchMembers,
					Attempt: prefix.Attempt, Log: prefix.Log, Failed: report.failed, Load: report.load, LoadStart: report.loadStart, LoadEnd: report.loadEnd,
					Cause: &Cause{Kind: "unclassified", Tests: failingTests(report.failed), Evidence: prefix.Log}}, prefix.Since, seams.now())
				if check.Load == 0 {
					if sample := hostload.Read(seams.now()); sample.Available {
						check.Load = sample.Load1m
					}
				}
				if slices.ContainsFunc(report.failed, func(unit FailedUnit) bool { return len(unit.Tests) > 0 }) {
					prefix.Trunk = true
					check = repeatRed(seams, install, command, dir, prefix.Running, scopeDecision{scopeRecord: scopeRecord{Scope: "full"}}, check, true)
					failed = check.Result == Red
					complete = !failed || check.Cause.Kind == "unclassified"
				}
				if !failed {
					result.Flakes = append(result.Flakes, check.Flakes...)
					result.Reason += "; " + check.Reason
				} else if complete && recordingMain {
					mainChecks = append(mainChecks, check)
				}
			}
			if prefix.Main && failed {
				result.Cause.Name = mainFailureIdentity(report.failed)
			}
		}
		if !complete {
			result.Reason += "; isolated check of " + strings.Join(units, " ") + " on " + prefix.Commit + " did not complete"
		}

		_, removeErr := seams.git(checkout, "worktree", "remove", "--force", tree)
		if i == 0 && prefix.Main && failed && complete {
			result.Cause.Kind = "main"
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

// ProofBudget holds an automatic full check after two completed red attempts.
type ProofBudget struct{}

func (*ProofBudget) Error() string {
	return "two full checks went red on this base; a person decides\nmetasystem landing prove, or metasystem landing give-back GOAL --reason TEXT; " + humanauthority.PersonActRemedy("metasystem landing prove")
}

func subsetGoals(goals, first []GoalSHA) bool {
	return len(goals) > 0 && !slices.ContainsFunc(goals, func(g GoalSHA) bool {
		return !slices.ContainsFunc(first, func(f GoalSHA) bool { return f.Goal == g.Goal })
	})
}

// checkProofBudget runs under the lane lock, before a start or repeat marker.
func checkProofBudget(install, checkout, commit string, seams ProveSeams) error {
	results, skipped, err := countedLines[Result](resultsPath(install))
	if err != nil {
		return err
	}
	if skipped != 0 {
		return fmt.Errorf("the full-check history contains unreadable lines; a person may request one fresh check")
	}
	batch, err := ReadBatch(install)
	if err != nil {
		return err
	}
	var first []GoalSHA
	base := ""
	firstRed := false
	attempts := map[string]bool{}
	for _, r := range results {
		if batch != nil && r.BatchBase != batch.Base {
			continue
		}
		if r.LoopClosed {
			first, firstRed, attempts = nil, false, map[string]bool{}
			continue
		}
		if r.Trunk || !r.CountedFull || len(r.Goals) == 0 || r.BatchBase == "" {
			continue
		}
		if !subsetGoals(r.Goals, first) || r.BatchBase != base {
			first, firstRed, attempts = nil, false, map[string]bool{}
		}
		if first == nil || !firstRed && r.Result == Red {
			first = r.Goals
		}
		base = r.BatchBase
		firstRed = firstRed || r.Result == Red
		if first != nil {
			attempts[r.Attempt] = r.Result == Red
		}
	}
	reds := 0
	for _, red := range attempts {
		if red {
			reds++
		}
	}
	if reds < 2 {
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

func closeProofLoop(install string) error {
	last, ok, err := LastResult(install)
	if err != nil || !ok || last.LoopClosed {
		return err
	}
	last.LoopClosed = true
	return appendLine(resultsPath(install), last)
}
