package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func TestALaterCheckSectionKeepsItsFailedTests(t *testing.T) {
	t.Parallel()
	const named = "--- FAIL: TestAudit (0.01s)\nFAIL\nFAIL\texample.test/m/pkg/unit\t0.01s\n"
	for _, scenario := range []struct {
		name, output, outcome string
		both, fallback, older bool
	}{
		{name: "audit flake", output: named, outcome: "failed"},
		{name: "same package", output: "--- FAIL: TestRed (0.01s)\n" + named, outcome: "failed", both: true},
		{name: "build failure", output: "FAIL\texample.test/m/pkg/unit [build failed]\n", outcome: "failed", fallback: true},
		{name: "mixed build failure", output: named + "FAIL\texample.test/m/other [build failed]\n", outcome: "failed", fallback: true},
		{name: "crash after named failure", output: "--- FAIL: TestAudit (0.01s)\nfatal error: stack overflow\n\ngoroutine 1 [running]:\nFAIL\texample.test/m/pkg/unit\t1.5s\nFAIL\n", outcome: "failed", fallback: true},
		{name: "outside module", output: strings.ReplaceAll(named, "example.test/m/", "example.test/else/"), outcome: "failed", fallback: true},
		{name: "deadline", output: named, outcome: "deadline", fallback: true},
		{name: "cancelled", output: named, outcome: "cancelled", fallback: true},
		{name: "passed text ignored", output: named, outcome: "passed", fallback: true},
		{name: "newest only", output: "FAIL\texample.test/m/pkg/unit [build failed]\n", outcome: "failed", fallback: true, older: true},
		{name: "missing result", outcome: "failed", fallback: true},
		{name: "unreadable result", outcome: "failed", fallback: true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			initial, repeated := "LANDING-CHECKED\\t0\\n", "LANDING-CHECKED\\t0\\n"
			cheap := "passed"
			if scenario.both || (scenario.fallback && scenario.name != "crash after named failure") {
				cheap = "failed"
				initial = "LANDING-FAILED\\tpkg/unit\\tTestRed\\nLANDING-CHECKED\\t1\\n"
			}
			if scenario.outcome == "passed" {
				// A passed section contributes nothing, even when its output looks red.
				initial = "LANDING-CHECKED\\t0\\n"
			}
			if scenario.both {
				repeated = "LANDING-FAILED\\tpkg/unit\\tTestRed TestAudit\\nLANDING-CHECKED\\t1\\n"
			}
			script := fmt.Sprintf("if [ -n \"$LANDING_ONLY\" ] && [ -f red ]; then printf '%s'; exit %d; fi\nif [ -f red ]; then printf '%s'; exit 1; fi\nprintf 'LANDING-CHECKED\\t0\\n'\n", repeated, map[bool]int{true: 1}[scenario.both], initial)
			f, git := attributionFixture(t, false, script)
			if err := os.WriteFile(filepath.Join(f.worktree, "go.mod"), []byte("module example.test/m\n"), 0600); err != nil {
				t.Fatal(err)
			}
			plan, err := ReadUnitPlan(f.plan)
			if err != nil {
				t.Fatal(err)
			}
			plan.Check = &UnitCheck{Cheap: "check", Audits: "audits", Minutes: 1, Directory: f.worktree, Environment: []string{}}
			if err := writeUnitJSON(f.plan, plan, f.runner.Root); err != nil {
				t.Fatal(err)
			}
			readUnits := f.runner.RedUnits
			f.runner.RedUnits = func(r Record) ([]loopstop.RedUnit, bool, error) {
				units, complete, err := readUnits(r)
				if scenario.both && len(units) > 0 {
					units[0].Registered = []string{"flake:6373c392abcdef00"}
				}
				return units, complete, err
			}
			start := f.starter.onStart
			f.starter.onStart = func(r Record) error {
				if err := start(r); err != nil {
					return err
				}
				if r.Kind != "proof" || strings.HasSuffix(r.ID, "-alone") || strings.HasSuffix(r.ID, "-base") {
					return nil
				}
				round := filepath.Join(f.runner.Root, strings.Split(r.ID, "-r1-")[0], "round-1")
				write := func(id, output string, at time.Time) error {
					dir := filepath.Join(round, id)
					observation := CheckExecution{ExecutionID: id, Steps: []processmeasure.Step{
						{ID: r.ID + ":cheap", Parent: r.ID, Start: at.Format(time.RFC3339Nano), Outcome: cheap, Terminal: true},
						{ID: r.ID + ":audits", Parent: r.ID, Outcome: scenario.outcome, Terminal: true},
					}}
					if err := writeUnitJSON(filepath.Join(dir, "observation.json"), observation, f.runner.Root); err != nil {
						return err
					}
					if scenario.name == "missing result" {
						return nil
					}
					if scenario.name == "unreadable result" {
						return os.WriteFile(filepath.Join(dir, "result.json"), []byte("{"), 0600)
					}
					return writeUnitJSON(filepath.Join(dir, "result.json"), struct {
						Exits []CheckExit `json:"exits"`
					}{[]CheckExit{{Name: "cheap", Output: named}, {Name: "audits", Exit: 1, Output: output}}}, f.runner.Root)
				}
				if scenario.older {
					if err := write("check-older", named, f.manager.Now().Add(-time.Minute)); err != nil {
						return err
					}
				}
				return write("check-sections", scenario.output, f.manager.Now())
			}
			f.runner.BeforeModelLaunch = func(record UnitRunRecord, spec StartSpec) error {
				if strings.HasSuffix(spec.ID, "-alone") && git.exports != 0 {
					t.Fatal("base comparison started before rerun alone")
				}
				return nil
			}
			result, err := f.runner.Advance(UnitRequest{Plan: f.plan})
			if err != nil {
				t.Fatal(err)
			}
			step := result.Record.Rounds[0].Steps[1]
			if !scenario.fallback {
				tests := "TestAudit"
				if scenario.both {
					tests = "TestRed TestAudit"
				}
				want := []string{"LANDING_ONLY=pkg/unit", "LANDING_ONLY_TESTS=" + tests}
				if step.Alone == nil || !slices.Equal(step.Alone.Selectors, want) {
					t.Fatalf("failed tests were not merged into one rerun: %+v", step)
				}
				if !scenario.both {
					if step.State != StepPassed || step.Comparison != nil || git.exports != 0 || !slices.Equal(step.Flakes, []string{"pkg/unit TestAudit"}) {
						t.Fatalf("audit did not pass after a flake rerun: %+v", step)
					}
					return
				}
				if step.State != StepFailed || step.Cause != "own" || !strings.Contains(step.Red, "TestRed in pkg/unit, registered as flake:6373c392") || strings.Contains(step.Red, "TestAudit in pkg/unit, registered") {
					t.Fatalf("merged failure lost its attribution: %+v", step)
				}
			} else if step.State != StepFailed || step.Cause != "own" || step.Comparison == nil || len(step.Comparison.Selectors) != 0 || strings.Contains(step.Red, "TestAudit") {
				t.Fatalf("incomplete section was narrowed or called a flake: %+v", step)
			}
			if scenario.fallback && scenario.outcome != "passed" && !strings.Contains(step.Red, "section/audits") {
				t.Fatalf("unnamed audits failure lost: %+v", step)
			}
			if scenario.name == "crash after named failure" && len(step.Flakes) != 0 {
				t.Fatalf("crashed audits section recorded a flake: %+v", step)
			}
			var base PlainBrief
			data, err := os.ReadFile(filepath.Join(result.Record.Rounds[0].Directory, "base-1.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &base); err != nil {
				t.Fatal(err)
			}
			if scenario.both && (step.Comparison == nil || !slices.Equal(step.Comparison.Selectors, step.Alone.Selectors) || !slices.Equal(base.Argv, plan.Proof[0].Argv)) {
				t.Fatalf("base did not use the alone brief: %+v base=%+v", step, base)
			}
			for _, selector := range []string{"LANDING_ONLY=pkg/unit", "LANDING_ONLY_TESTS=TestRed TestAudit"} {
				if slices.Contains(base.Env, selector) != scenario.both {
					t.Fatalf("base selectors=%v", base.Env)
				}
			}
		})
	}
}
