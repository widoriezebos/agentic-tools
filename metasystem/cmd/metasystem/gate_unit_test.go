package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch/goadapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

// scriptedGoClosure is the Go adapter with its closure scripted, so the gate's
// steps and reds come from the real Go adapter without a module on disk.
type scriptedGoClosure struct {
	goadapter.Adapter
	closure func(root, base, tree string) (adapter.Closure, error)
}

func (s scriptedGoClosure) Closure(root, base, tree string) (adapter.Closure, error) {
	return s.closure(root, base, tree)
}

func withAdapter(language adapter.Adapter) func(string) (adapter.Adapter, error) {
	return func(string) (adapter.Adapter, error) { return language, nil }
}

func TestGateUnitGreenOutputAndAggregatedSteps(t *testing.T) {
	t.Parallel()
	selection := adapter.Closure{
		Tree:       "candidate-tree",
		Module:     "example.invalid/unit",
		Changed:    []string{"./base"},
		Dependents: []string{"./cmd/metasystem"},
	}
	var calls []adapter.GateStep
	dependencies := gateUnitDependencies{
		adapter: withAdapter(scriptedGoClosure{closure: func(root, base, tree string) (adapter.Closure, error) {
			if base != "base-tree" || tree != "HEAD" || root == "" {
				t.Fatalf("selection inputs root=%q base=%q tree=%q", root, base, tree)
			}
			return selection, nil
		}}),
		runStep: func(root, tree string, step adapter.GateStep) adapter.GateStepResult {
			if root == "" || tree != selection.Tree {
				t.Fatalf("runner inputs root=%q tree=%q", root, tree)
			}
			calls = append(calls, step)
			return adapter.GateStepResult{}
		},
	}
	var stdout, stderr bytes.Buffer
	code := runGateUnitWith([]string{"--root", t.TempDir(), "--base", "base-tree"}, dependencies, &stdout, &stderr)
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("gate unit code=%d stderr=%q", code, stderr.String())
	}
	wantOutput := "CHANGED PACKAGES 1\nCHANGED ./base\nDEPENDENT PACKAGES 1\nDEPENDENT ./cmd/metasystem\nGATE-UNIT GREEN 2 packages\n"
	if stdout.String() != wantOutput {
		t.Fatalf("gate unit output = %q, want %q", stdout.String(), wantOutput)
	}
	if len(calls) != 2 ||
		!slices.Equal(calls[0].Args, []string{"go", "test", "-trimpath", "-count=1", "-timeout", "40m", "./base", "./cmd/metasystem"}) ||
		!slices.Equal(calls[1].Args, []string{"go", "test", "-trimpath", "-count=1", "-timeout", "40m", "-tags", "batchtest", "./cmd/metasystem"}) {
		t.Fatalf("gate unit steps = %v", calls)
	}
}

func TestGateUnitRedOutputAndExitCode(t *testing.T) {
	t.Parallel()
	selection := adapter.Closure{
		Tree:       "candidate-tree",
		Module:     "example.invalid/unit",
		Changed:    []string{"./base"},
		Dependents: []string{"./cmd/metasystem"},
	}
	dependencies := gateUnitDependencies{
		adapter: withAdapter(scriptedGoClosure{closure: func(string, string, string) (adapter.Closure, error) { return selection, nil }}),
		runStep: func(_ string, _ string, step adapter.GateStep) adapter.GateStepResult {
			if strings.Contains(step.Name, "batchtest") {
				return adapter.GateStepResult{ExitCode: 1, Detail: "--- FAIL: TestBatchOnly (0.00s)\nFAIL\nFAIL\texample.invalid/unit/cmd/metasystem\t0.01s\n"}
			}
			return adapter.GateStepResult{ExitCode: 1, Detail: "--- FAIL: TestBase (0.00s)\nFAIL\nFAIL\texample.invalid/unit/base\t0.01s\n--- FAIL: TestCommand (0.00s)\nFAIL\nFAIL\texample.invalid/unit/cmd/metasystem\t0.01s\n"}
		},
	}
	var stdout, stderr bytes.Buffer
	code := runGateUnitWith([]string{"--root", t.TempDir(), "--base", "base-tree", "--tree", "tip-tree"}, dependencies, &stdout, &stderr)
	if code == 0 || stderr.Len() != 0 {
		t.Fatalf("gate unit code=%d stderr=%q", code, stderr.String())
	}
	for _, line := range []string{
		"RED ./base TestBase\n",
		"RED ./cmd/metasystem TestCommand\n",
		"RED ./cmd/metasystem TestBatchOnly\n",
		"GATE-UNIT RED 3 reds\n",
	} {
		if !strings.Contains(stdout.String(), line) {
			t.Errorf("gate unit output %q does not contain %q", stdout.String(), line)
		}
	}
}

// TestLaneDecisionsWithAFakeAdapter drives the lane's unit gate with the fake,
// non-Go adapter: the closure, the steps and the reds all come from the fake,
// whose classnames differ from its unit names, and no Go tool is planned.
func TestLaneDecisionsWithAFakeAdapter(t *testing.T) {
	t.Parallel()
	fake := fakeadapter.New()
	var ran [][]string
	dependencies := gateUnitDependencies{
		adapter: withAdapter(fake),
		runStep: func(_, tree string, step adapter.GateStep) adapter.GateStepResult {
			if tree != "tip-tree" {
				t.Fatalf("step tree=%q", tree)
			}
			ran = append(ran, step.Args)
			if slices.Equal(step.Args, []string{fakeadapter.Tool, "test", "payments"}) {
				return adapter.GateStepResult{ExitCode: 1, Detail: "FAILED com.example.PaymentTest#rejectsInvalidInput\n"}
			}
			return adapter.GateStepResult{}
		},
	}
	var stdout, stderr bytes.Buffer
	code := runGateUnitWith([]string{"--root", t.TempDir(), "--base", "base-tree", "--tree", "tip-tree"}, dependencies, &stdout, &stderr)
	want := "CHANGED PACKAGES 1\nCHANGED payments\nDEPENDENT PACKAGES 1\nDEPENDENT ledger\n" +
		"RED payments rejectsInvalidInput\nGATE-UNIT RED 1 reds\n"
	if code != 1 || stderr.Len() != 0 || stdout.String() != want {
		t.Fatalf("fake gate code=%d stdout=%q stderr=%q want %q", code, stdout.String(), stderr.String(), want)
	}
	if !slices.EqualFunc(ran, [][]string{{fakeadapter.Tool, "test", "payments"}, {fakeadapter.Tool, "test", "ledger"}}, slices.Equal[[]string]) {
		t.Fatalf("fake gate ran %v", ran)
	}
	if !slices.Equal(*fake.Calls, []string{"Closure", "UnitGateSteps", "GateReds", "OwnerUnit"}) {
		t.Fatalf("fake adapter calls=%v", *fake.Calls)
	}
	for _, line := range strings.Split(stdout.String(), "\n") {
		if strings.Contains(line, "com.example") {
			t.Fatalf("the gate reported a classname as a unit: %q", line)
		}
	}
}

func TestGateUnitRefusesAnAdapterWithoutAUnitGate(t *testing.T) {
	t.Parallel()
	language, err := adapter.Resolve(testpolicy.Group{Adapter: "section"})
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := runGateUnitWith([]string{"--root", t.TempDir(), "--base", "base-tree"}, gateUnitDependencies{
		adapter: withAdapter(language),
		runStep: func(string, string, adapter.GateStep) adapter.GateStepResult {
			t.Fatal("a step ran without a unit gate")
			return adapter.GateStepResult{}
		},
	}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "has no unit gate") {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
}
