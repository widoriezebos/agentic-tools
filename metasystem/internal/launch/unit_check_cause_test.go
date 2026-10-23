package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func writeCheckObservation(t *testing.T, round, directory, parent, start string, outcomes ...string) {
	t.Helper()
	observation := CheckExecution{ExecutionID: strings.TrimPrefix(directory, "builder-"), Role: "builder"}
	for _, outcome := range outcomes {
		observation.Steps = append(observation.Steps, processmeasure.Step{Parent: parent, Start: start, Outcome: outcome})
	}
	data, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(round, directory)
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "observation.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestUnitBuildCheckDeadlineCause(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, parent, start, missing string
		outcomes                     []string
		deadline                     bool
		newer                        bool
	}{
		{name: "deadline", outcomes: []string{"deadline", "passed"}, deadline: true},
		{name: "newer_passed", outcomes: []string{"passed", "passed"}, newer: true},
		{name: "cheap_failed", outcomes: []string{"failed", "deadline"}},
		{name: "audits_failed", outcomes: []string{"deadline", "failed"}},
		{name: "earlier_launch", parent: "another-launch", outcomes: []string{"deadline", "passed"}},
		{name: "unreadable", missing: "corrupt", outcomes: []string{"deadline", "passed"}},
		{name: "missing_check", missing: "check-missing", outcomes: []string{"failed", "passed"}, newer: true},
		{name: "missing_builder_check", missing: "builder-check-missing", outcomes: []string{"failed", "passed"}, newer: true},
		{name: "unordered", start: "unusable", outcomes: []string{"failed", "passed"}, newer: true},
		{name: "nonterminal", outcomes: []string{"deadline", ""}},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			f := newUnitFixture(t, "", "branch", "before", "branch")
			f.starter.holdKind = "build"
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped {
				t.Fatalf("start=%+v err=%v", first, err)
			}
			round := first.Record.Rounds[0]
			id := round.Steps[0].LaunchID
			parent, start := id, "2026-10-23T09:00:00Z"
			if row.parent != "" {
				parent = row.parent
			}
			if row.newer {
				writeCheckObservation(t, round.Directory, "builder-check-old", id, start, "deadline", "passed")
				start = "2026-10-23T10:00:00Z"
			}
			if row.start != "" {
				start = row.start
			}
			directory := "builder-check-current"
			if row.missing != "" && row.missing != "corrupt" {
				directory = row.missing
			}
			writeCheckObservation(t, round.Directory, directory, parent, start, row.outcomes...)
			if row.missing != "" {
				path := filepath.Join(round.Directory, row.missing)
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
				if row.missing == "corrupt" {
					if err := os.WriteFile(filepath.Join(path, "observation.json"), []byte("{"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					// The newer failed attempt has a directory but no readable observation.
					if err := os.Remove(filepath.Join(round.Directory, directory, "observation.json")); err != nil {
						t.Fatal(err)
					}
				}
			}
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
			step := stopped.Steps[0]
			cause, handoff := "unclassified", "stopped unclassified"
			if row.deadline {
				cause, handoff = "environment", "stopped deadline"
			}
			if result.Record.State != "awaiting-judgement" || step.Cause != cause || stopped.Cause != cause || step.Deadline != row.deadline || step.Reason != "build-partial" || stopped.Outcome != "build-partial" || stopped.Stop == nil || stopped.Stop.Handoff != handoff || stopped.Stop.Class != "build: build-partial" || len(f.starter.ids) != 1 {
				t.Fatalf("round=%+v step=%+v launches=%v", stopped, step, f.starter.ids)
			}
		})
	}
}

func TestUnitProofCheckDeadlineCause(t *testing.T) {
	t.Parallel()
	for _, resumed := range []bool{false, true} {
		t.Run(map[bool]string{false: "proof_loop", true: "resumed_failure"}[resumed], func(t *testing.T) {
			t.Parallel()
			f, git := attributionFixture(t, false, "", true)
			f.runner.Git = resultAttributionGit{git}
			plan, err := ReadUnitPlan(f.plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.Read = UnitReadPlan{Brief: filepath.Join(filepath.Dir(f.worktree), "read.md"), Inputs: []string{}, Outputs: []string{}, Model: "read-model"}
			if err := writeUnitJSON(f.plan, plan, filepath.Dir(f.plan)); err != nil {
				t.Fatal(err)
			}
			f.starter.holdKind = "proof"
			first, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil || !first.Capped {
				t.Fatalf("start=%+v err=%v", first, err)
			}
			record := first.Record
			step := &record.Rounds[0].Steps[1]
			writeCheckObservation(t, record.Rounds[0].Directory, "check-proof", step.LaunchID, "2026-10-23T09:00:00Z", "deadline", "passed")
			if _, err := f.manager.Store.Update(step.LaunchID, func(r *Record) error { code := 1; r.State, r.ExitCode, r.Reason = Failed, &code, "exit-1"; return nil }); err != nil {
				t.Fatal(err)
			}
			if resumed {
				step.State, step.Cause, step.Reason = StepFailed, "unclassified", "exit-1"
				if err := f.runner.save(record); err != nil {
					t.Fatal(err)
				}
			}
			f.starter.holdKind = ""
			result, err := f.runner.Continue(UnitRequest{Resume: record.ID})
			if err != nil {
				t.Fatal(err)
			}
			stopped := result.Record.Rounds[0]
			step = &stopped.Steps[1]
			if result.Record.State != "awaiting-judgement" || step.Cause != "environment" || stopped.Cause != "environment" || !step.Deadline || step.Reason != "the frozen check stopped at proof.deadline" || step.Comparison != nil || stopped.Stop == nil || stopped.Stop.Handoff != "stopped deadline" || len(f.starter.ids) != 2 || git.exports != 0 {
				t.Fatalf("round=%+v step=%+v launches=%v", stopped, step, f.starter.ids)
			}
		})
	}
}
