package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestReverseDependentsIncludeDirectTransitiveTestAndTaggedImports(t *testing.T) {
	t.Parallel()
	root, tree := dependencyModuleTree(t)

	got, err := ReverseDependents(root, tree, []string{"./base"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}
	if !slices.Equal(got, want) {
		t.Fatalf("reverse dependents = %v, want %v", got, want)
	}
}

func TestWorkingUnitSelectionIncludesTrackedAndUntrackedPackages(t *testing.T) {
	t.Parallel()
	root, _ := dependencyModuleTree(t)
	if err := os.WriteFile(filepath.Join(root, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	newPath := filepath.Join(root, "fresh", "fresh.go")
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("package fresh\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ignoredPath := filepath.Join(root, "ignored", "ignored.go")
	if err := os.MkdirAll(filepath.Dir(ignoredPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ignoredPath, []byte("package ignored\nimport _ \"example.invalid/unitgate/base\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection, err := SelectWorkingUnitPackages(root, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Changed, []string{"./base", "./fresh"}) {
		t.Fatalf("working changed packages = %v", selection.Changed)
	}
	wantDependents := []string{"./cmd/metasystem", "./direct", "./tagged", "./testonly", "./transitive"}
	if !slices.Equal(selection.Dependents, wantDependents) {
		t.Fatalf("working dependents = %v, want %v", selection.Dependents, wantDependents)
	}
}

func TestWorkingUnitSelectionAcceptsNestedModuleTreeBase(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	moduleRoot := filepath.Join(top, "metasystem")
	for path, content := range map[string]string{
		"outside.txt":                    "outside\n",
		"metasystem/go.mod":              "module example.invalid/nested\n\ngo 1.27\n",
		"metasystem/base/base.go":        "package base\n",
		"metasystem/direct/direct.go":    "package direct\nimport _ \"example.invalid/nested/base\"\n",
		"metasystem/unrelated/value.go":  "package unrelated\n",
		"metasystem/unrelated/second.go": "package unrelated\n",
	} {
		absolute := filepath.Join(top, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bedGit(t, top, "init", "-q", "-b", "main")
	bedGit(t, top, "config", "user.name", "Fixture")
	bedGit(t, top, "config", "user.email", "fixture@example.invalid")
	bedGit(t, top, "add", ".")
	bedGit(t, top, "commit", "-qm", "fixture")
	baseTree := bedGit(t, top, "rev-parse", "HEAD:metasystem")
	if err := os.WriteFile(filepath.Join(moduleRoot, "base", "base.go"), []byte("package base\nconst Changed = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	selection, err := SelectWorkingUnitPackages(moduleRoot, baseTree)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(selection.Changed, []string{"./base"}) || !slices.Equal(selection.Dependents, []string{"./direct"}) {
		t.Fatalf("nested module selection changed=%v dependents=%v", selection.Changed, selection.Dependents)
	}
}

func TestBatchJoinGateStepsChangedThenSortedDependentsAndBatchTests(t *testing.T) {
	t.Parallel()
	root, tree := dependencyModuleTree(t)
	unit := Unit{}
	var calls []GateStep
	err := runJoinGateAt(root, tree, gatePatch("base/base.go"), nil, &unit, func(_ string, step GateStep) GateStepResult {
		calls = append(calls, step)
		return GateStepResult{RunID: fmt.Sprintf("gate-%d", len(calls))}
	})
	if err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"bash", "scripts/agents/go-gate.sh", "--fast"},
		{"go", "test", "-count=1", "-timeout", "900s", "./base"},
		{"go", "test", "-count=1", "-timeout", "40m", "./cmd/metasystem"},
		{"go", "test", "-count=1", "-timeout", "40m", "./direct"},
		{"go", "test", "-count=1", "-timeout", "40m", "./tagged"},
		{"go", "test", "-count=1", "-timeout", "40m", "./testonly"},
		{"go", "test", "-count=1", "-timeout", "40m", "./transitive"},
		{"go", "test", "-count=1", "-timeout", "40m", "-tags", "batchtest", "./cmd/metasystem"},
	}
	if len(calls) != len(want) {
		t.Fatalf("join gate steps = %v, want %d steps", calls, len(want))
	}
	for index := range want {
		if !slices.Equal(calls[index].Args, want[index]) {
			t.Errorf("join gate step %d = %v, want %v", index, calls[index].Args, want[index])
		}
	}
}

func TestBatchJoinGateRedNamesFailingDependentTests(t *testing.T) {
	t.Parallel()
	root, tree := dependencyModuleTree(t)
	unit := Unit{}
	err := runJoinGateAt(root, tree, gatePatch("base/base.go"), nil, &unit, func(_ string, step GateStep) GateStepResult {
		result := GateStepResult{RunID: "green"}
		if step.Name == "dependent package ./direct" {
			result.ExitCode = 1
			result.Detail = "--- FAIL: TestDirectContract (0.00s)\nFAIL\n"
		}
		return result
	})
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_GATE_RED") ||
		!strings.Contains(err.Error(), "./direct") || !strings.Contains(err.Error(), "TestDirectContract") {
		t.Fatalf("dependent test refusal = %v", err)
	}
}

func dependencyModuleTree(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module example.invalid/unitgate\n\ngo 1.27\n",
		".gitignore":             "ignored/\n",
		"base/base.go":           "package base\n",
		"direct/direct.go":       "package direct\nimport _ \"example.invalid/unitgate/base\"\n",
		"transitive/value.go":    "package transitive\nimport _ \"example.invalid/unitgate/direct\"\n",
		"testonly/value.go":      "package testonly\n",
		"testonly/value_test.go": "package testonly\nimport _ \"example.invalid/unitgate/base\"\n",
		"tagged/tagged.go":       "//go:build unitgate_never\n\npackage tagged\nimport _ \"example.invalid/unitgate/base\"\n",
		"none/none.go":           "package none\n",
		"cmd/metasystem/main.go": "package main\nimport _ \"example.invalid/unitgate/transitive\"\nfunc main() {}\n",
	}
	for path, content := range files {
		absolute := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(absolute, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	bedGit(t, root, "init", "-q", "-b", "main")
	bedGit(t, root, "config", "user.name", "Fixture")
	bedGit(t, root, "config", "user.email", "fixture@example.invalid")
	bedGit(t, root, "add", ".")
	bedGit(t, root, "commit", "-qm", "fixture")
	return root, bedGit(t, root, "rev-parse", "HEAD^{tree}")
}
