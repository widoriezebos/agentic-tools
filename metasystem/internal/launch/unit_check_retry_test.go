package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// unitCheckDeadlineFixture is a round whose unit check failed at
// proof.deadline in its first launch, held before the run continues once
// for each resume.
func unitCheckDeadlineFixture(t *testing.T, resumes ...bool) (unitFixture, *attributionGit, UnitRunRecord) {
	t.Helper()
	f, git := attributionFixture(t, false, "", resumes...)
	f.runner.Git = resultAttributionGit{git}
	plan, err := ReadUnitPlan(f.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan.Proof[0].Name = "unit-check"
	if err := writeUnitJSON(f.plan, plan, filepath.Dir(f.plan)); err != nil {
		t.Fatal(err)
	}
	f.starter.holdKind = "proof"
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("start=%+v err=%v", first, err)
	}
	record := first.Record
	step := record.Rounds[0].Steps[1]
	writeCheckObservation(t, record.Rounds[0].Directory, "check-first", step.LaunchID, "2026-10-23T09:00:00Z", "deadline", "passed")
	if _, err := f.manager.Store.Update(step.LaunchID, func(r *Record) error { code := 1; r.State, r.ExitCode, r.Reason = Failed, &code, "exit-1"; return nil }); err != nil {
		t.Fatal(err)
	}
	return f, git, record
}

func TestUnitCheckDeadlineRetriesOnce(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"deadline", "passes", "cheap_failed"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			f, git, record := unitCheckDeadlineFixture(t, true, true)
			result, err := f.runner.Continue(UnitRequest{Resume: record.ID})
			if err != nil || !result.Capped {
				t.Fatalf("retry=%+v err=%v", result, err)
			}
			round := result.Record.Rounds[0]
			step := round.Steps[1]
			original := step.LaunchIDs[0]
			if step.Name != "proof:unit-check" || !slices.Equal(step.LaunchIDs, []string{original, original + "-retry"}) || step.RetryBy != "machine" || step.RetryReason != "one retry after the check reached proof.deadline" || step.RetryLaunch != "" || step.Comparison != nil || round.Stop != nil || git.exports != 0 {
				t.Fatalf("first deadline: round=%+v step=%+v launches=%v", round, step, f.starter.ids)
			}
			retry := step.LaunchIDs[1]
			switch row {
			case "deadline":
				writeCheckObservation(t, round.Directory, "check-retry", retry, "2026-10-23T09:30:00Z", "deadline", "passed")
			case "cheap_failed":
				writeCheckObservation(t, round.Directory, "check-retry", retry, "2026-10-23T09:30:00Z", "failed", "passed")
			}
			state, code := Failed, 1
			if row == "passes" {
				state, code = Completed, 0
			}
			if _, err := f.manager.Store.Update(retry, func(r *Record) error { r.State, r.ExitCode = state, &code; return nil }); err != nil {
				t.Fatal(err)
			}
			f.starter.holdKind = ""
			result, err = f.runner.Continue(UnitRequest{Resume: record.ID})
			if err != nil {
				t.Fatal(err)
			}
			round = result.Record.Rounds[0]
			step = round.Steps[1]
			proofs := 0
			for _, id := range f.starter.ids {
				if strings.HasPrefix(id, original) && !strings.HasSuffix(id, "-base") {
					proofs++
				}
			}
			if proofs != 2 || len(step.LaunchIDs) != 2 {
				t.Fatalf("%s: a third check launch: %v", row, f.starter.ids)
			}
			switch row {
			case "deadline":
				if step.State != StepFailed || step.Cause != "environment" || !step.Deadline || round.Cause != "environment" || round.Stop == nil || round.Stop.Handoff != "stopped deadline" || step.Comparison != nil {
					t.Fatalf("second deadline did not stop for a person: round=%+v step=%+v", round, step)
				}
			case "passes":
				if step.State != StepPassed || round.Outcome == "proof-red" || round.Stop != nil {
					t.Fatalf("a passing retry did not let the round go on: round=%+v step=%+v", round, step)
				}
			case "cheap_failed":
				if step.State != StepFailed || step.Deadline || step.Comparison == nil {
					t.Fatalf("a red retry was not compared as unclassified: round=%+v step=%+v", round, step)
				}
			}
		})
	}
}

func TestUnitCheckRetryThatEndsAtOnceIsJudged(t *testing.T) {
	t.Parallel()
	f, _, record := unitCheckDeadlineFixture(t, true)
	start := f.starter.onStart
	f.starter.onStart = func(launch Record) error {
		if launch.Kind == "proof" && strings.HasSuffix(launch.ID, "-retry") {
			writeCheckObservation(t, record.Rounds[0].Directory, "check-retry", launch.ID, "2026-10-23T09:30:00Z", "deadline", "passed")
		}
		return start(launch)
	}
	f.starter.holdKind = ""
	result, err := f.runner.Continue(UnitRequest{Resume: record.ID})
	if err != nil {
		t.Fatal(err)
	}
	round := result.Record.Rounds[0]
	step := round.Steps[1]
	if len(step.LaunchIDs) != 2 || step.Cause != "environment" || round.Cause != "environment" || round.Stop == nil || round.Stop.Handoff != "stopped deadline" || step.Comparison != nil {
		t.Fatalf("a retry that reached the deadline at once was not judged: round=%+v step=%+v", round, step)
	}
}

func TestUnitCheckRetryBooksTheFirstDeadlineAsEnvironment(t *testing.T) {
	t.Parallel()
	f, _, record := unitCheckDeadlineFixture(t, true)
	var collected []string
	f.runner.CollectLaunch = func(_ UnitRunRecord, execution Record, cause string) error {
		if execution.Kind == "proof" && cause == "environment" {
			collected = append(collected, execution.ID)
		}
		return nil
	}
	result, err := f.runner.Continue(UnitRequest{Resume: record.ID})
	if err != nil || !result.Capped {
		t.Fatalf("retry=%+v err=%v", result, err)
	}
	original := result.Record.Rounds[0].Steps[1].LaunchIDs[0]
	if !slices.Contains(collected, original) {
		t.Fatalf("the first deadline was not collected as environment: %v", collected)
	}
}

func TestUnitBuildDeadlineIsNotRetriedByTheMachine(t *testing.T) {
	t.Parallel()
	f := newUnitFixture(t, "", "branch", "before", "branch", "failed")
	f.starter.holdKind = "build"
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("start=%+v err=%v", first, err)
	}
	round := first.Record.Rounds[0]
	id := round.Steps[0].LaunchID
	writeCheckObservation(t, round.Directory, "builder-check-current", id, "2026-10-23T09:00:00Z", "deadline", "passed")
	state, err := f.manager.Store.StateDir(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: PARTIAL\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.manager.Store.Update(id, func(r *Record) error { code := 0; r.State, r.ExitCode = Completed, &code; return nil }); err != nil {
		t.Fatal(err)
	}
	f.starter.holdKind = ""
	result, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	stopped := result.Record.Rounds[0]
	if step := stopped.Steps[0]; len(f.starter.ids) != 1 || step.RetryBy != "" || len(step.LaunchIDs) != 1 || !step.Deadline || stopped.Stop == nil || stopped.Stop.Handoff != "stopped deadline" {
		t.Fatalf("a build deadline was retried by the machine: round=%+v launches=%v", stopped, f.starter.ids)
	}
	// A continuation of the held round starts no builder turn either.
	if _, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID}); len(f.starter.ids) != 1 {
		t.Fatalf("a later continuation retried the build deadline: %v %v", f.starter.ids, err)
	}
}

func TestUnitReadPromptNamesTheSeverities(t *testing.T) {
	t.Parallel()
	record := Record{Kind: "read", AdapterData: map[string]json.RawMessage{}}
	setStrings(record.AdapterData, "declaredOutputs", []string{"report.md", "return.json"})
	prompt := string(appendReadPacket(nil, record))
	if !strings.Contains(prompt, "Each finding requires severity (critical, high, medium or low),") {
		t.Fatalf("the read prompt does not name the severities the collector accepts: %s", prompt)
	}
}

func TestRetriedFailedBuildIsCountedAfresh(t *testing.T) {
	t.Parallel()
	f, repo := newGitUnitFixture(t)
	turn := "first\n"
	f.starter.onBuild = func(record Record) error {
		return os.WriteFile(filepath.Join(record.WorkingDirectory, "built"), []byte(turn), 0600)
	}
	f.starter.holdKind = "build"
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("start=%+v err=%v", first, err)
	}
	round := first.Record.Rounds[0]
	id := round.Steps[0].LaunchID
	writeCheckObservation(t, round.Directory, "builder-check-current", id, "2026-10-23T09:00:00Z", "deadline", "passed")
	state, err := f.manager.Store.StateDir(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(state, "last-message.txt"), []byte("RESULT: PARTIAL\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.manager.Store.Update(id, func(r *Record) error { code := 0; r.State, r.ExitCode = Completed, &code; return nil }); err != nil {
		t.Fatal(err)
	}
	f.starter.holdKind = ""
	failed, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	round = failed.Record.Rounds[0]
	if round.Cause != "environment" || round.BuildLines != 1 || round.DeclaredLines == 0 || filepath.Dir(round.GapMessage) != state {
		t.Fatalf("the failed build was not counted: %+v", round)
	}
	if _, err := os.Stat(filepath.Join(round.Directory, "worktree.diff")); !os.IsNotExist(err) {
		t.Fatalf("the failed build wrote the round's frozen diff: %v", err)
	}
	if _, err := f.runner.RetryFailedStep(first.Record.ID, "Wido", "retry the builder turn", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	turn = "one\ntwo\nthree\n"
	retried, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	round = retried.Record.Rounds[0]
	diff, err := os.ReadFile(filepath.Join(round.Directory, "worktree.diff"))
	if err != nil || !strings.Contains(string(diff), "+three") || round.BuildLines != 3 || round.Outcome == "build-gap" || round.GapMessage != "" {
		t.Fatalf("the retried build kept the failed turn's count: lines=%d declared=%d outcome=%s diff=%s err=%v (repo %s)", round.BuildLines, round.DeclaredLines, round.Outcome, diff, err, repo)
	}
}
