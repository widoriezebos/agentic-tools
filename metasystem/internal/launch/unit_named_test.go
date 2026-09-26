package launch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

func rewriteUnitPlan(t *testing.T, path string, change func(*UnitPlan)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan UnitPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	change(&plan)
	data, _ = json.Marshal(plan)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func unitRunDirectories(t *testing.T, root string) []string {
	t.Helper()
	entries, _ := os.ReadDir(root)
	var runs []string
	for _, entry := range entries {
		if idPattern.MatchString(entry.Name()) {
			runs = append(runs, entry.Name())
		}
	}
	return runs
}

func requireLaunchedOnce(t *testing.T, fixture unitFixture, want int) {
	t.Helper()
	seen := map[string]bool{}
	for _, id := range fixture.starter.ids {
		if seen[id] {
			t.Fatalf("launch started twice: %s in %v", id, fixture.starter.ids)
		}
		seen[id] = true
	}
	if len(fixture.starter.ids) != want {
		t.Fatalf("launches=%v, want %d", fixture.starter.ids, want)
	}
}

func TestNamedUnitRetrySharesTheRunAndTheLaunch(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.State != "awaiting-judgement" || first.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	moved := filepath.Join(t.TempDir(), "another-name.json")
	data, _ := os.ReadFile(fixture.plan)
	os.WriteFile(moved, data, 0o600)
	for _, plan := range []string{fixture.plan, moved} {
		again, err := fixture.runner.AdvanceNamed(plan)
		if err != nil || again.Record.ID != first.Record.ID || again.Record.State != "awaiting-judgement" || len(again.Record.Rounds) != 1 {
			t.Fatalf("plan=%s result=%+v err=%v", plan, again, err)
		}
	}
	if !slices.Equal(fixture.starter.order, []string{"build", "proof", "read"}) {
		t.Fatalf("order=%v", fixture.starter.order)
	}
	requireLaunchedOnce(t, fixture, 3)
	if runs := unitRunDirectories(t, fixture.runner.Root); len(runs) != 1 {
		t.Fatalf("runs=%v", runs)
	}
}

func TestNamedUnitConcurrentRepeatLaunchesOnce(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	var wait sync.WaitGroup
	results := make([]UnitResult, 6)
	errs := make([]error, len(results))
	for index := range results {
		wait.Add(1)
		go func() {
			defer wait.Done()
			results[index], errs[index] = fixture.runner.AdvanceNamed(fixture.plan)
		}()
	}
	wait.Wait()
	run := ""
	for index, err := range errs {
		if err != nil {
			if !strings.Contains(err.Error(), "UNIT_RUN_BUSY") || !strings.Contains(err.Error(), "repeat the same command") {
				t.Fatalf("err=%v", err)
			}
			continue
		}
		if run != "" && results[index].Record.ID != run {
			t.Fatalf("two runs: %s and %s", run, results[index].Record.ID)
		}
		run = results[index].Record.ID
	}
	if run == "" {
		t.Fatal("no caller advanced the unit")
	}
	requireLaunchedOnce(t, fixture, 3)
	if runs := unitRunDirectories(t, fixture.runner.Root); !slices.Equal(runs, []string{run}) {
		t.Fatalf("runs=%v", runs)
	}
}

func TestNamedUnitBusyCallerNamesTheRunAndLaunchesNothing(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	plan, _ := ReadUnitPlan(fixture.plan)
	_, key, _ := namedUnitIdentity(plan)
	lock, err := fixture.runner.namedLock(key, plan)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	_, err = fixture.runner.AdvanceNamed(fixture.plan)
	if err == nil || !strings.Contains(err.Error(), "UNIT_RUN_BUSY") || !strings.Contains(err.Error(), "run="+first.Record.ID) {
		t.Fatalf("err=%v", err)
	}
	requireLaunchedOnce(t, fixture, 3)
}

func TestNamedUnitChangedInputRefusesWithThePriorRun(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		change func(t *testing.T, fixture unitFixture)
	}{
		{"brief", func(t *testing.T, fixture unitFixture) {
			plan, _ := ReadUnitPlan(fixture.plan)
			os.WriteFile(plan.Build.Brief, []byte("Declared size: 2 changed lines\nand more\n"), 0o600)
		}},
		{"input", func(t *testing.T, fixture unitFixture) {
			os.WriteFile(filepath.Join(filepath.Dir(fixture.plan), "input.md"), []byte("changed\n"), 0o600)
		}},
		{"proof-argv", func(t *testing.T, fixture unitFixture) {
			rewriteUnitPlan(t, fixture.plan, func(plan *UnitPlan) { plan.Proof[0].Argv = []string{"false"} })
		}},
		{"proof-env", func(t *testing.T, fixture unitFixture) {
			rewriteUnitPlan(t, fixture.plan, func(plan *UnitPlan) { plan.Proof[0].Env = []string{"A=C"} })
		}},
		{"base", func(t *testing.T, fixture unitFixture) {
			rewriteUnitPlan(t, fixture.plan, func(plan *UnitPlan) { plan.Base = "other" })
		}},
		{"build-model", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.BuildModel = "another-model" }},
		{"build-runtime", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.BuildRuntime = "codex" }},
		{"read-runtime", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.ReadRuntime = "codex" }},
		{"build-window", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.BuildWindow = 400000 }},
		{"read-window", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.ReadWindow = 400000 }},
		{"build-lines-cap", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.BuildLinesCap = 3 }},
		{"read-split-lines", func(t *testing.T, fixture unitFixture) { fixture.manager.Settings.ReadSplitLines = 7 }},
	} {
		t.Run(row.name, func(t *testing.T) {
			fixture := newUnitFixture(t, "", "branch", "branch", "round")
			input := filepath.Join(filepath.Dir(fixture.plan), "input.md")
			os.WriteFile(input, []byte("input\n"), 0o600)
			rewriteUnitPlan(t, fixture.plan, func(plan *UnitPlan) { plan.Build.Inputs = []string{input} })
			first, err := fixture.runner.AdvanceNamed(fixture.plan)
			if err != nil {
				t.Fatal(err)
			}
			row.change(t, fixture)
			_, err = fixture.runner.AdvanceNamed(fixture.plan)
			if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "run="+first.Record.ID) ||
				!strings.Contains(err.Error(), "follow-up") || !strings.Contains(err.Error(), "another unit name") {
				t.Fatalf("err=%v", err)
			}
			requireLaunchedOnce(t, fixture, 3)
		})
	}
}

func TestNamedUnitWorktreesAreSeparateUnits(t *testing.T) {
	t.Parallel()
	first := newUnitFixture(t, "", "branch", "branch", "round")
	second := newUnitFixture(t, "", "branch", "branch", "round")
	second.runner.Root = first.runner.Root
	one, err := first.runner.AdvanceNamed(first.plan)
	if err != nil {
		t.Fatal(err)
	}
	two, err := second.runner.AdvanceNamed(second.plan)
	if err != nil || two.Record.ID == one.Record.ID || two.Record.Worktree != second.worktree || two.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("one=%+v two=%+v err=%v", one.Record, two.Record, err)
	}
	requireLaunchedOnce(t, first, 3)
	requireLaunchedOnce(t, second, 3)
	entries, _ := filepath.Glob(filepath.Join(first.runner.Root, ".named", "*.json"))
	if len(entries) != 2 {
		t.Fatalf("entries=%v", entries)
	}
}

func TestNamedUnitRecoversAnInterruptedFirstCallOnce(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		events []string
		crash  func(UnitRunRecord) bool
		after  func(t *testing.T, fixture unitFixture, run string)
	}{
		// Killed after the reservation, before the run record was written.
		{"reserved", []string{"branch", "branch", "branch", "round"}, func(UnitRunRecord) bool { return true },
			func(t *testing.T, fixture unitFixture, run string) { os.RemoveAll(fixture.runner.runDir(run)) }},
		// Killed after the run record, before the entry said so.
		{"recorded", []string{"branch", "branch", "round"}, func(UnitRunRecord) bool { return true }, nil},
		// Killed after the build launch was marked starting.
		{"launching", []string{"branch", "branch", "branch", "round"}, func(record UnitRunRecord) bool {
			return slices.ContainsFunc(record.Rounds[0].Steps, func(step UnitStep) bool { return step.State == StepStarting })
		}, nil},
	} {
		t.Run(row.name, func(t *testing.T) {
			fixture := newUnitFixture(t, "", row.events...)
			fixture.runner.AfterWrite = func(record UnitRunRecord) error {
				if row.crash(record) {
					return errors.New("killed")
				}
				return nil
			}
			if _, err := fixture.runner.AdvanceNamed(fixture.plan); err == nil || err.Error() != "killed" {
				t.Fatalf("err=%v", err)
			}
			fixture.runner.AfterWrite = nil
			runs := unitRunDirectories(t, fixture.runner.Root)
			if len(runs) != 1 {
				t.Fatalf("runs=%v", runs)
			}
			if row.after != nil {
				row.after(t, fixture, runs[0])
			}
			result, err := fixture.runner.AdvanceNamed(fixture.plan)
			if err != nil || result.Record.ID != runs[0] || result.Record.State != "awaiting-judgement" || result.Record.Rounds[0].Outcome != "green" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			requireLaunchedOnce(t, fixture, 3)
			if again, err := fixture.runner.AdvanceNamed(fixture.plan); err != nil || again.Record.ID != runs[0] {
				t.Fatalf("again=%+v err=%v", again, err)
			}
			requireLaunchedOnce(t, fixture, 3)
		})
	}
}

func TestNamedUnitAwaitingJudgementRepeatsWithoutABuild(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	fixture.starter.failKind = "proof"
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.State != "awaiting-judgement" || first.Record.Rounds[0].Outcome != "proof-red" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	for range 2 {
		again, err := fixture.runner.AdvanceNamed(fixture.plan)
		if err != nil || again.Record.ID != first.Record.ID || again.Record.State != "awaiting-judgement" || again.Record.Rounds[0].Outcome != "proof-red" {
			t.Fatalf("result=%+v err=%v", again, err)
		}
	}
	if !slices.Equal(fixture.starter.order, []string{"build", "proof"}) {
		t.Fatalf("order=%v", fixture.starter.order)
	}
}

func TestNamedUnitFollowUpStillUsesResume(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	follow := filepath.Join(t.TempDir(), "follow")
	os.WriteFile(follow, []byte("Declared size: 1 changed lines\n"), 0o600)
	second, err := fixture.runner.Advance(UnitRequest{Resume: first.Record.ID, FollowUp: follow})
	if err != nil || len(second.Record.Rounds) != 2 || second.Record.State != "awaiting-judgement" {
		t.Fatalf("result=%+v err=%v", second, err)
	}
	again, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || again.Record.ID != first.Record.ID || len(again.Record.Rounds) != 2 {
		t.Fatalf("result=%+v err=%v", again, err)
	}
	requireLaunchedOnce(t, fixture, 6)
}

func TestNamedUnitCorruptOrStaleEntryFailsWithoutOverwrite(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, want string
		spoil      func(t *testing.T, fixture unitFixture, entry, run string)
	}{
		{"unreadable", "UNIT_NAMED_ENTRY_CORRUPT", func(t *testing.T, fixture unitFixture, entry, run string) {
			os.WriteFile(entry, []byte("{not json"), 0o600)
		}},
		{"other-unit", "UNIT_NAMED_ENTRY_CORRUPT", func(t *testing.T, fixture unitFixture, entry, run string) {
			data, _ := os.ReadFile(entry)
			os.WriteFile(entry, []byte(strings.Replace(string(data), `"unit": "U"`, `"unit": "V"`, 1)), 0o600)
		}},
		{"run-gone", "UNIT_NAMED_RUN_MISSING", func(t *testing.T, fixture unitFixture, entry, run string) {
			os.RemoveAll(fixture.runner.runDir(run))
		}},
		{"run-of-another-goal", "UNIT_NAMED_ENTRY_CORRUPT", func(t *testing.T, fixture unitFixture, entry, run string) {
			path := filepath.Join(fixture.runner.runDir(run), "run.json")
			data, _ := os.ReadFile(path)
			os.WriteFile(path, []byte(strings.Replace(string(data), `"goal": "goal"`, `"goal": "other"`, 1)), 0o600)
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			fixture := newUnitFixture(t, "", "branch", "branch", "round")
			first, err := fixture.runner.AdvanceNamed(fixture.plan)
			if err != nil {
				t.Fatal(err)
			}
			entries, _ := filepath.Glob(filepath.Join(fixture.runner.Root, ".named", "*.json"))
			if len(entries) != 1 {
				t.Fatalf("entries=%v", entries)
			}
			row.spoil(t, fixture, entries[0], first.Record.ID)
			before, _ := os.ReadFile(entries[0])
			_, err = fixture.runner.AdvanceNamed(fixture.plan)
			if err == nil || !strings.Contains(err.Error(), row.want) {
				t.Fatalf("err=%v", err)
			}
			after, _ := os.ReadFile(entries[0])
			if string(before) != string(after) {
				t.Fatalf("entry rewritten: %s", after)
			}
			requireLaunchedOnce(t, fixture, 3)
			if runs := unitRunDirectories(t, fixture.runner.Root); len(runs) > 1 {
				t.Fatalf("runs=%v", runs)
			}
		})
	}
}

func TestLegacyPlanAdvanceKeepsItsBehaviour(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "round", "branch", "round")
	first, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil {
		t.Fatal(err)
	}
	second, err := fixture.runner.Advance(UnitRequest{Plan: fixture.plan})
	if err != nil || second.Record.ID == first.Record.ID {
		t.Fatalf("first=%s second=%+v err=%v", first.Record.ID, second, err)
	}
	if _, err := os.Stat(filepath.Join(fixture.runner.Root, ".named")); !os.IsNotExist(err) {
		t.Fatalf("legacy plan wrote a named entry: %v", err)
	}
}

func TestNamedUnitWaitCapIsNotPartOfTheUnit(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil {
		t.Fatal(err)
	}
	fixture.manager.Settings.WaitCapSeconds = 5
	again, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || again.Record.ID != first.Record.ID {
		t.Fatalf("result=%+v err=%v", again, err)
	}
	requireLaunchedOnce(t, fixture, 3)
}

// hookGit runs a change once, on the first goal-branch check, which comes
// after the plan is read and before its run record is written.
type hookGit struct {
	inner  GitRunner
	once   sync.Once
	change func()
}

func (git *hookGit) Run(directory string, environment []string, args ...string) ([]byte, error) {
	if len(args) > 0 && args[0] == "symbolic-ref" {
		git.once.Do(git.change)
	}
	return git.inner.Run(directory, environment, args...)
}

func proofArgv(t *testing.T, fixture unitFixture, run string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(fixture.runner.runDir(run), "round-1", "proof-check.json"))
	if err != nil {
		t.Fatal(err)
	}
	var brief PlainBrief
	if err := json.Unmarshal(data, &brief); err != nil {
		t.Fatal(err)
	}
	return brief.Argv
}

func TestNamedUnitRunsThePlanItReservedWhenThePlanIsRewritten(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch", "round")
	fixture.runner.Git = &hookGit{inner: fixture.git, change: func() {
		rewriteUnitPlan(t, fixture.plan, func(plan *UnitPlan) { plan.Proof[0].Argv = []string{"false"}; plan.Read.Model = "other-model" })
	}}
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || first.Record.Rounds[0].Outcome != "green" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	if argv := proofArgv(t, fixture, first.Record.ID); !slices.Equal(argv, []string{"true"}) {
		t.Fatalf("proof ran %v, not the reserved plan", argv)
	}
	if model := stepNamed(t, first.Record.Rounds[0], "read").Model; model != "read-model" {
		t.Fatalf("read model=%s", model)
	}
	retained, err := readUnitPlan(first.Record.Plan, first.Record.PlanDirectory)
	if err != nil || !slices.Equal(retained.Proof[0].Argv, []string{"true"}) {
		t.Fatalf("retained=%+v err=%v", retained.Proof, err)
	}
	_, err = fixture.runner.AdvanceNamed(fixture.plan)
	if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "run="+first.Record.ID) {
		t.Fatalf("err=%v", err)
	}
	requireLaunchedOnce(t, fixture, 3)
}

func TestNamedUnitRefusesATamperedRetainedPlanBeforeItsPendingStep(t *testing.T) {
	t.Parallel()
	fixture := newUnitFixture(t, "", "branch", "branch")
	fixture.starter.holdKind = "build"
	first, err := fixture.runner.AdvanceNamed(fixture.plan)
	if err != nil || !first.Capped || first.Step != "build" {
		t.Fatalf("result=%+v err=%v", first, err)
	}
	rewriteUnitPlan(t, first.Record.Plan, func(plan *UnitPlan) { plan.Proof[0].Argv = []string{"false"} })
	fixture.manager.Store.Update(first.Launch, func(record *Record) error { record.State = Completed; return nil })
	fixture.starter.holdKind = ""
	entries, _ := filepath.Glob(filepath.Join(fixture.runner.Root, ".named", "*.json"))
	before, _ := os.ReadFile(entries[0])
	_, err = fixture.runner.AdvanceNamed(fixture.plan)
	if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "run="+first.Record.ID) {
		t.Fatalf("err=%v", err)
	}
	if after, _ := os.ReadFile(entries[0]); string(after) != string(before) {
		t.Fatalf("reservation rewritten: %s", after)
	}
	if !slices.Equal(fixture.starter.order, []string{"build"}) {
		t.Fatalf("order=%v", fixture.starter.order)
	}
	if _, err := os.Stat(filepath.Join(fixture.runner.runDir(first.Record.ID), "round-1", "proof-check.json")); !os.IsNotExist(err) {
		t.Fatalf("proof brief written: %v", err)
	}
}

func TestNamedUnitChangeBeforeAPendingLaunchLaunchesNothingUntilRestored(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name            string
		change, restore func(fixture unitFixture)
	}{
		{"read-runtime", func(fixture unitFixture) { fixture.manager.Settings.ReadRuntime = "codex" },
			func(fixture unitFixture) { fixture.manager.Settings.ReadRuntime = DefaultSettings().ReadRuntime }},
		{"read-brief", func(fixture unitFixture) {
			plan, _ := ReadUnitPlan(fixture.plan)
			os.WriteFile(plan.Read.Brief, []byte("read something else\n"), 0o600)
		}, func(fixture unitFixture) {
			plan, _ := ReadUnitPlan(fixture.plan)
			os.WriteFile(plan.Read.Brief, []byte("read it\n"), 0o600)
		}},
	} {
		t.Run(row.name, func(t *testing.T) {
			fixture := newUnitFixture(t, "", "branch", "branch", "round", "branch")
			fixture.starter.onStart = func(record Record) error {
				if record.Kind == "proof" {
					row.change(fixture)
				}
				return nil
			}
			_, err := fixture.runner.AdvanceNamed(fixture.plan)
			if err == nil || !strings.Contains(err.Error(), "UNIT_NAMED_INPUT_CHANGED") || !strings.Contains(err.Error(), "nothing more was launched") {
				t.Fatalf("err=%v", err)
			}
			if !slices.Equal(fixture.starter.order, []string{"build", "proof"}) {
				t.Fatalf("order=%v", fixture.starter.order)
			}
			fixture.starter.onStart = nil
			row.restore(fixture)
			result, err := fixture.runner.AdvanceNamed(fixture.plan)
			if err != nil || result.Record.Rounds[0].Outcome != "green" {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			requireLaunchedOnce(t, fixture, 3)
		})
	}
}
