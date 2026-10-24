package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestUnitBuildResultJudgement(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		launch State
		result string
		state  UnitStepState
		reason string
	}{
		{"partial", Completed, "PARTIAL", StepFailed, "build-partial"},
		{"collection_failed_done", Failed, "DONE", StepFailed, "declared-output: missing report"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newUnitFixture(t, "", []string{}...)
			directory := filepath.Join(f.runner.Root, "result-unit", "round-1")
			if err := os.MkdirAll(directory, 0700); err != nil {
				t.Fatal(err)
			}
			stateDir, err := f.manager.Store.StateDir("builder")
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(stateDir, 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(stateDir, "last-message.txt"), []byte("RESULT: "+test.result+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			code := 0
			launch := Record{ID: "builder", Kind: "build", State: test.launch, ExitCode: &code}
			step := UnitStep{Name: "build", LaunchID: launch.ID, State: StepPassed}
			if test.launch == Failed {
				launch.Reason, step.State, step.Reason = test.reason, StepFailed, test.reason
			}
			if err := f.manager.Store.Create(launch); err != nil {
				t.Fatal(err)
			}
			record := UnitRunRecord{ID: "result-unit", Unit: "U", Goal: "goal", Worktree: f.worktree, State: "running", Rounds: []UnitRound{{Number: 1, Directory: directory, Steps: []UnitStep{step}}}}
			if err := f.runner.judgeBuildResult(&record, &record.Rounds[0], 0); err != nil {
				t.Fatal(err)
			}
			judged := record.Rounds[0].Steps[0]
			if judged.State != test.state || judged.Reason != test.reason {
				t.Fatalf("result judgement=%+v, want state=%s reason=%s", judged, test.state, test.reason)
			}
		})
	}
}

func TestUnitBuildResultBlock(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, message, tail, verdict, reason, open string
		state                                      State
		passed                                     bool
	}{
		{name: "done", message: "RESULT: DONE\n", verdict: "DONE", state: Completed, passed: true},
		{name: "partial", message: "RESULT: PARTIAL\nUNIT: U\nOPEN: Decide the file scope.\nKeep the proof deadline.\n", verdict: "PARTIAL", reason: "build-partial", open: "OPEN: Decide the file scope.\nKeep the proof deadline.\n", state: Completed},
		{name: "failed", message: "RESULT: FAILED\nOPEN:\n", verdict: "FAILED", reason: "build-failed", state: Completed},
		{name: "missing", message: "The build finished.\n", reason: "no-result-block", state: Completed, passed: true},
		{name: "first_word_and_fence", message: "```text\n`RESULT: pArTiAl`\nOPEN: Decide.\n```\nOutside the result block.\nRESULT: DONE\n", verdict: "PARTIAL", reason: "build-partial", open: "OPEN: Decide.\n", state: Completed},
		{name: "word_boundary", message: "RESULT: DONEish\nRESULT: FAILED\n", verdict: "FAILED", reason: "build-failed", state: Completed},
		{name: "retained_tail", tail: "RESULT: partial | OPEN: Decide.", verdict: "PARTIAL", reason: "build-partial", open: "OPEN: Decide.\n", state: Completed},
		{name: "collection_failed_done", message: "RESULT: DONE\n", reason: "declared-output: missing report", state: Failed},
		{name: "collection_failed_missing", message: "The build finished.\n", reason: "declared-output: missing report", state: Failed},
		{name: "cancelled_done", message: "RESULT: DONE\n", reason: "cancelled", state: Cancelled},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			events := []string{"branch", "before", "branch"}
			if test.passed {
				events = append(events, "round")
			} else {
				events = append(events, "failed")
			}
			f := newUnitFixture(t, "", events...)
			f.starter.holdKind = "build"
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped {
				t.Fatalf("start=%+v err=%v", first, err)
			}
			id := first.Record.Rounds[0].Steps[0].LaunchID
			stateDir, err := f.manager.Store.StateDir(id)
			if err != nil {
				t.Fatal(err)
			}
			if test.message != "" {
				if err := os.WriteFile(filepath.Join(stateDir, "last-message.txt"), []byte(test.message), 0600); err != nil {
					t.Fatal(err)
				}
			}
			_, err = f.manager.Store.Update(id, func(r *Record) error {
				code := 0
				r.State, r.ExitCode, r.Measurement.ResultTail = test.state, &code, test.tail
				if test.state != Completed {
					r.Reason = test.reason
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			f.starter.holdKind = ""
			got, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
			if err != nil {
				t.Fatal(err)
			}
			round := got.Record.Rounds[0]
			step := round.Steps[0]
			wantState := StepFailed
			if test.passed {
				wantState = StepPassed
			}
			if step.State != wantState || step.Reason != test.reason || got.Record.State != "awaiting-judgement" {
				t.Fatalf("build=%+v run=%s", step, got.Record.State)
			}
			launch, err := f.manager.Store.Read(id)
			if err != nil || launch.State != test.state || launch.Measurement.Verdict != test.verdict {
				t.Fatalf("launch=%+v err=%v", launch, err)
			}
			if !test.passed {
				outcome := "build-failed"
				if test.reason == "build-partial" {
					outcome = test.reason
				}
				if round.Outcome != outcome || len(round.Steps) != 3 || len(f.starter.ids) != 1 || round.Stop == nil {
					t.Fatalf("failed round=%+v launches=%v", round, f.starter.ids)
				}
				for _, skipped := range round.Steps[1:] {
					if skipped.State != StepSkipped {
						t.Fatalf("ran after failed build: %+v", skipped)
					}
				}
				if test.verdict == "PARTIAL" || test.verdict == "FAILED" {
					reported := "a partial result"
					if test.verdict == "FAILED" {
						reported = "a failed result"
					}
					if got.BuildResultSummary() != "U: the builder reported "+reported {
						t.Fatalf("summary=%q", got.BuildResultSummary())
					}
				} else if got.BuildResultSummary() != "" {
					t.Fatalf("launch failure reported a builder verdict: %s", got.BuildResultSummary())
				}
			}
			open, err := os.ReadFile(filepath.Join(round.Directory, "open.md"))
			if test.open == "" {
				if !os.IsNotExist(err) {
					t.Fatalf("empty OPEN wrote a file: %q %v", open, err)
				}
			} else if err != nil || string(open) != test.open {
				t.Fatalf("OPEN=%q err=%v", open, err)
			}
			retained := readOutputs(f.manager, round)
			if test.open != "" && !strings.Contains(strings.Join(retained, "\n"), filepath.Join(round.Directory, "open.md")) {
				t.Fatalf("revision inputs lost OPEN: %v", retained)
			}
		})
	}
}

func TestUnitContinueKeepsPassedBuildWithLatePartialResult(t *testing.T) {
	t.Parallel()
	for _, initial := range []string{"missing", "done"} {
		t.Run(initial, func(t *testing.T) {
			t.Parallel()
			f := newUnitFixture(t, "", "branch", "round", "branch", "branch")
			f.starter.holdKind = "proof"
			f.starter.onBuild = func(r Record) error {
				if initial == "missing" {
					return nil
				}
				directory, err := f.manager.Store.StateDir(r.ID)
				if err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(directory, "last-message.txt"), []byte("RESULT: DONE\n"), 0600)
			}
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped || first.Step != "proof:check" {
				t.Fatalf("start=%+v err=%v", first, err)
			}
			round := first.Record.Rounds[0]
			if len(round.Steps) != 2 || round.Steps[0].State != StepPassed || round.Steps[1].State != StepRunning {
				t.Fatalf("build did not pass before the proof started: %+v", round.Steps)
			}
			build := round.Steps[0]
			builder, err := f.manager.Store.Read(build.LaunchID)
			if err != nil {
				t.Fatal(err)
			}
			if initial == "missing" && build.Reason != "no-result-block" || initial == "done" && builder.Measurement.Verdict != "DONE" {
				t.Fatalf("build judgement was not recorded: step=%+v launch=%+v", build, builder)
			}
			directory, err := f.manager.Store.StateDir(build.LaunchID)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "last-message.txt"), []byte("RESULT: PARTIAL\nOPEN: Late message.\n"), 0600); err != nil {
				t.Fatal(err)
			}
			// A new runner must recover the judgement from the saved run.
			resumed := &UnitRunner{Manager: f.manager, Root: f.runner.Root, Git: f.runner.Git}
			held, err := resumed.Continue(UnitRequest{Resume: first.Record.ID})
			if err != nil || !held.Capped || held.Record.State != "running" || held.Step != "proof:check" || held.Launch != first.Launch {
				t.Fatalf("continuation orphaned the running proof: %+v err=%v", held, err)
			}
			round = held.Record.Rounds[0]
			if len(round.Steps) != 2 || round.Steps[0].State != StepPassed || round.Steps[0].Reason != build.Reason || round.Steps[1].LaunchID != first.Launch || round.Outcome != "" || len(f.starter.ids) != 2 {
				t.Fatalf("continuation rewrote the build or duplicated steps: %+v launches=%v", round, f.starter.ids)
			}
			proof, err := f.manager.Store.Read(first.Launch)
			if err != nil || proof.State != Running {
				t.Fatalf("running proof was lost: %+v err=%v", proof, err)
			}
			if _, err := f.manager.Store.Update(first.Launch, func(r *Record) error {
				code := 0
				r.State, r.ExitCode = Completed, &code
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			f.starter.holdKind = ""
			finished, err := resumed.Continue(UnitRequest{Resume: first.Record.ID})
			if err != nil || finished.Record.State != "awaiting-judgement" {
				t.Fatalf("finish=%+v err=%v", finished, err)
			}
			round = finished.Record.Rounds[0]
			if round.Outcome != "green" || len(round.Steps) != 3 || round.Steps[0].State != StepPassed || round.Steps[1].State != StepPassed || round.Steps[1].LaunchID != first.Launch || round.Steps[2].Name != "read" || round.Steps[2].State != StepPassed || len(f.starter.ids) != 3 {
				t.Fatalf("proof did not finish with exactly one read: %+v launches=%v", round, f.starter.ids)
			}
			builderAfter, err := f.manager.Store.Read(build.LaunchID)
			if err != nil || builderAfter.Measurement.Verdict != builder.Measurement.Verdict {
				t.Fatalf("continuation changed the recorded builder verdict: %+v err=%v", builderAfter, err)
			}
			if _, err := os.Stat(filepath.Join(round.Directory, "open.md")); !os.IsNotExist(err) {
				t.Fatalf("late PARTIAL message was judged: %v", err)
			}
		})
	}
}

func TestUnitContinueFinishesRecordedProofFailure(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"exit-1", "launch-deadline"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			f, git := attributionFixture(t, false, "", true)
			f.runner.Git = resultAttributionGit{git}
			plan, err := ReadUnitPlan(f.plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.Read = UnitReadPlan{Brief: filepath.Join(filepath.Dir(f.worktree), "read.md"), Inputs: []string{}, Outputs: []string{}, Model: "read-model"}
			data, err := json.Marshal(plan)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(f.plan, data, 0600); err != nil {
				t.Fatal(err)
			}
			start := f.starter.onStart
			f.starter.onStart = func(r Record) error {
				if err := start(r); err != nil {
					return err
				}
				if strings.HasSuffix(r.ID, "-base") {
					f.starter.holdKind = "proof"
				}
				return nil
			}
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped {
				t.Fatalf("first=%+v err=%v", first, err)
			}
			if summary := first.AttributionSummary(f.manager.Now().Add(3 * time.Minute)); summary != "U: comparing the red with the base (3 min)" {
				t.Fatalf("attribution summary=%q", summary)
			}
			record := first.Record
			round := &record.Rounds[0]
			if round.Outcome != "" || round.Steps[1].State != StepFailed || round.Steps[1].Cause != "unclassified" {
				t.Fatalf("failure not pending: %+v", round)
			}
			round.Steps[1].Reason = reason
			if err := f.runner.save(record); err != nil {
				t.Fatal(err)
			}
			comparison := round.Steps[1].Comparison
			sleep := f.manager.Sleep
			completed := false
			f.manager.Sleep = func(duration time.Duration) {
				sleep(duration)
				_, updateErr := f.manager.Store.Update(comparison.LaunchID, func(r *Record) error { code := 0; r.State, r.ExitCode = Completed, &code; return nil })
				if updateErr != nil {
					t.Fatal(updateErr)
				}
				completed = true
			}
			got, err := f.runner.Continue(UnitRequest{Resume: record.ID})
			if err != nil {
				t.Fatal(err)
			}
			finished := got.Record.Rounds[0]
			if !completed || got.Record.State != "awaiting-judgement" || finished.Outcome != "proof-red" || finished.Cause != "own" || !finished.Steps[1].Comparison.Verified || finished.Stop == nil || git.exports != 1 || len(f.starter.ids) != 3 {
				t.Fatalf("continued=%+v exports=%d launches=%v", got, git.exports, f.starter.ids)
			}

			if finished.Steps[1].Reason != reason || finished.Stop.Loop != "unit-build" || finished.Stop.Cause.Kind != "own" {
				t.Fatalf("lost failure evidence: %+v", finished)
			}
			if len(finished.Steps) != 3 || finished.Steps[2].Name != "read" || finished.Steps[2].State != StepSkipped || finished.Steps[2].Reason != "proof-red" {
				t.Fatalf("read not skipped for red proof: %+v", finished.Steps)
			}
			counted, machinery := countedRounds(got.Record)
			if counted != 1 || machinery != 0 {
				t.Fatalf("counted=%d machinery=%d, want one own correction", counted, machinery)
			}
			got.Record.CountedCap = 1
			if err := f.runner.countedCap(got.Record); !IsCode(err, "UNIT_ROUND_CAP") {
				t.Fatalf("own failure escaped correction cap: %v", err)
			}
		})
	}
}

func TestUnitContinueRecordedDeadlineAllowsRetry(t *testing.T) {
	t.Parallel()
	f := newUnitFixture(t, "", "branch", "round", "branch")
	f.starter.holdKind = "proof"
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if err != nil || !first.Capped {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	record := first.Record
	record.Rounds[0].Cause = "deadline"
	step := &record.Rounds[0].Steps[1]
	step.State, step.Cause, step.Reason, step.Deadline = StepFailed, "deadline", "launch-deadline", true
	if _, err := f.manager.Store.Update(step.LaunchID, func(r *Record) error {
		r.State, r.Cause, r.Reason = Failed, "deadline", "launch-deadline"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.runner.save(record); err != nil {
		t.Fatal(err)
	}
	got, err := f.runner.Continue(UnitRequest{Resume: record.ID})
	if err != nil {
		t.Fatal(err)
	}
	round := got.Record.Rounds[0]
	if round.Outcome != "proof-red" || round.Cause != "deadline" || round.Stop == nil || round.Stop.Loop != "unit-build" || round.Steps[1].Reason != "launch-deadline" {
		t.Fatalf("deadline hold=%+v", round)
	}
	impacts := 0
	retried, err := f.runner.RetryFailedStep(record.ID, "person", "retry after the deadline", func() error { impacts++; return nil })
	if err != nil {
		t.Fatal(err)
	}
	round = retried.Record.Rounds[0]
	if impacts != 1 || retried.Record.State != "running" || round.Stop != nil || round.Outcome != "" || round.Cause != "" || round.Steps[0].State != StepPassed || round.Steps[1].State != StepPending || round.Steps[1].RetryBy != "person" || round.Steps[1].Deadline || round.Steps[2].State != StepPending || len(f.starter.ids) != 2 {
		t.Fatalf("retry=%+v impacts=%d launches=%v", retried, impacts, f.starter.ids)
	}
}

func TestUnitContinueGreenFlakeKeepsAdvancing(t *testing.T) {
	t.Parallel()
	counter := filepath.Join(t.TempDir(), "counter")
	script := fmt.Sprintf("if [ -f '%s' ]; then exit 0; fi\ntouch '%s'\nexit 1\n", counter, counter)
	f, git := attributionFixture(t, false, script, true)
	f.runner.Git = resultAttributionGit{git}
	f.runner.KnownFlake = func(Record) (bool, error) {
		return false, coded("BUDGET_REFUSED", "", fmt.Errorf("comparison interrupted"))
	}
	first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
	if !IsCode(err, "BUDGET_REFUSED") || first.Record.Rounds[0].Steps[1].State != StepFailed {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	f.runner.KnownFlake = func(Record) (bool, error) { return true, nil }
	got, err := f.runner.Continue(UnitRequest{Resume: first.Record.ID})
	if err != nil {
		t.Fatal(err)
	}
	round := got.Record.Rounds[0]
	if got.Record.State != "awaiting-judgement" || round.Outcome != "green" || round.Cause != "" || round.Steps[1].State != StepPassed || !round.Steps[1].FlakeRepeat || git.exports != 0 || len(f.starter.ids) != 3 {
		t.Fatalf("green retry stopped: %+v exports=%d launches=%v", got, git.exports, f.starter.ids)
	}
}

// The continuation resolves the same temporary worktree through its canonical path.
type resultAttributionGit struct{ *attributionGit }

func (git resultAttributionGit) Run(dir string, env []string, args ...string) ([]byte, error) {
	canonical, err := filepath.EvalSymlinks(git.fixture.worktree)
	if err == nil && dir == canonical {
		dir = git.fixture.worktree
	}
	return git.attributionGit.Run(dir, env, args...)
}
