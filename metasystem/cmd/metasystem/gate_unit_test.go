package main

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

func TestGateUnitGreenOutputAndAggregatedSteps(t *testing.T) {
	t.Parallel()
	selection := batch.UnitPackages{
		Tree:       "candidate-tree",
		ModulePath: "example.invalid/unit",
		Changed:    []string{"./base"},
		Dependents: []string{"./cmd/metasystem"},
	}
	var calls []batch.GateStep
	dependencies := gateUnitDependencies{
		selectPackages: func(root, base, tree string) (batch.UnitPackages, error) {
			if base != "base-tree" || tree != "HEAD" || root == "" {
				t.Fatalf("selection inputs root=%q base=%q tree=%q", root, base, tree)
			}
			return selection, nil
		},
		runStep: func(root, tree string, step batch.GateStep) batch.GateStepResult {
			if root == "" || tree != selection.Tree {
				t.Fatalf("runner inputs root=%q tree=%q", root, tree)
			}
			calls = append(calls, step)
			return batch.GateStepResult{}
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
		!slices.Equal(calls[0].Args, []string{"go", "test", "-count=1", "-timeout", "40m", "./base", "./cmd/metasystem"}) ||
		!slices.Equal(calls[1].Args, []string{"go", "test", "-count=1", "-timeout", "40m", "-tags", "batchtest", "./cmd/metasystem"}) {
		t.Fatalf("gate unit steps = %v", calls)
	}
}

func TestGateUnitRedOutputAndExitCode(t *testing.T) {
	t.Parallel()
	selection := batch.UnitPackages{
		Tree:       "candidate-tree",
		ModulePath: "example.invalid/unit",
		Changed:    []string{"./base"},
		Dependents: []string{"./cmd/metasystem"},
	}
	dependencies := gateUnitDependencies{
		selectPackages: func(string, string, string) (batch.UnitPackages, error) { return selection, nil },
		runStep: func(_ string, _ string, step batch.GateStep) batch.GateStepResult {
			if strings.Contains(step.Name, "batchtest") {
				return batch.GateStepResult{ExitCode: 1, Detail: "--- FAIL: TestBatchOnly (0.00s)\nFAIL\nFAIL\texample.invalid/unit/cmd/metasystem\t0.01s\n"}
			}
			return batch.GateStepResult{ExitCode: 1, Detail: "--- FAIL: TestBase (0.00s)\nFAIL\nFAIL\texample.invalid/unit/base\t0.01s\n--- FAIL: TestCommand (0.00s)\nFAIL\nFAIL\texample.invalid/unit/cmd/metasystem\t0.01s\n"}
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
