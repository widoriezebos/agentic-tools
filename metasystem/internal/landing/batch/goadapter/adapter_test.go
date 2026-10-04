package goadapter

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestGoAdapterIsBoundToTheGoGroups(t *testing.T) {
	t.Parallel()
	bound, err := adapter.Resolve(testpolicy.Group{Adapter: Name})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := bound.(Adapter); !ok {
		t.Fatalf("go groups bind %T", bound)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/detect\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if detected, err := adapter.Detect(root); err != nil || detected != (Adapter{}) {
		t.Fatalf("detect=%T err=%v", detected, err)
	}
	if _, err := adapter.Detect(t.TempDir()); err == nil {
		t.Fatal("a root without a module was detected as Go")
	}
}

func TestClosureNamesUnownedPaths(t *testing.T) {
	t.Parallel()
	root, base := dependencyModuleTree(t)
	base["plans/x.md"] = "base plan\n"
	for _, tc := range []struct {
		name    string
		edit    map[string]string
		changed []string
		full    bool
	}{
		{"code and plan", map[string]string{"base/base.go": "package base\nconst Changed = true\n", "plans/x.md": "changed plan\n"}, []string{"./base"}, false},
		{"plan only", map[string]string{"plans/x.md": "changed plan\n"}, nil, false},
		{"go.mod", map[string]string{"go.mod": base["go.mod"] + "// changed\n", "plans/x.md": "changed plan\n"}, []string{"./..."}, true},
		{"go.sum", map[string]string{"go.sum": "new checksum\n", "plans/x.md": "changed plan\n"}, []string{"./..."}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clonePackageFiles(base)
			for path, content := range tc.edit {
				candidate[path] = content
			}
			fixture := newPackageTreeFixture(t, root, base, candidate, "")
			closure, err := closureWithWorkspaceSnapshot(fixture.workspace(), fixture.baseTree, fixture.candidateTree, fixture.openSnapshot)
			if err != nil {
				t.Fatal(err)
			}
			if closure.Tree != fixture.candidateTree || closure.Module != "example.invalid/unitgate" || !slices.Equal(closure.Changed, tc.changed) || !slices.Equal(closure.Unowned, []string{"plans/x.md"}) {
				t.Fatalf("closure = %+v", closure)
			}
			var dependents []string
			if tc.name == "code and plan" {
				dependents = gateConsumerPackages().Dependents
			}
			if !slices.Equal(closure.Dependents, dependents) || closure.Contains("plans/x.md") || slices.Contains(closure.Units(), "plans/x.md") {
				t.Fatalf("dependents or unit membership = %+v; units=%v", closure, closure.Units())
			}
			if tc.full && !closure.Contains("./...") {
				t.Fatalf("manifest did not select every package: %+v", closure)
			}
		})
	}
}

func TestClosureFindsANestedModule(t *testing.T) {
	t.Parallel()
	top := t.TempDir()
	moduleRoot := filepath.Join(top, "metasystem")
	if err := os.MkdirAll(moduleRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{
		"go.mod":           "module example.invalid/nested\n\ngo 1.27\n",
		"base/base.go":     "package base\n",
		"direct/direct.go": "package direct\nimport _ \"example.invalid/nested/base\"\n",
		"none/none.go":     "package none\n",
		"plans/x.md":       "base plan\n",
	}
	if err := os.WriteFile(filepath.Join(moduleRoot, "go.mod"), []byte(base["go.mod"]), 0o644); err != nil {
		t.Fatal(err)
	}
	candidate := clonePackageFiles(base)
	candidate["base/base.go"] += "const Changed = true\n"
	candidate["plans/x.md"] = "changed plan\n"
	fixture := newPackageTreeFixture(t, moduleRoot, base, candidate, "metasystem/")
	workspace := fixture.workspace()
	workspace.Dir = top
	closure, err := closureWithWorkspaceSnapshot(workspace, fixture.topTree, fixture.candidateTopTree, fixture.openSnapshot)
	if err != nil {
		t.Fatal(err)
	}
	if closure.Tree != fixture.candidateTree || closure.Module != "example.invalid/nested" || !slices.Equal(closure.Changed, []string{"./base"}) || !slices.Equal(closure.Dependents, []string{"./direct"}) || !slices.Equal(closure.Unowned, []string{"metasystem/plans/x.md"}) {
		t.Fatalf("nested closure = %+v", closure)
	}
	missing := t.TempDir()
	if _, err := (Adapter{}).Closure(missing, "base", "candidate"); err == nil || !strings.Contains(err.Error(), missing) {
		t.Fatalf("root without module returned %v", err)
	}
}

func TestGoOwnerUnitAndIdentityFromTestJSONPackages(t *testing.T) {
	t.Parallel()
	closure := adapter.Closure{Module: "example.invalid/mod", Changed: []string{"./cmd/tool"}}
	failure := adapter.Failure{Report: "go-test-json", Classname: "example.invalid/mod/cmd/tool", Name: "TestX"}
	if unit, ok := (Adapter{}).OwnerUnit(failure, closure); !ok || unit != "./cmd/tool" {
		t.Fatalf("owner unit=%q ok=%v", unit, ok)
	}
	if unit, ok := (Adapter{}).OwnerUnit(adapter.Failure{Classname: "example.invalid/other", Name: "TestX"}, closure); ok {
		t.Fatalf("a foreign package was owned by %q", unit)
	}
	if identity, ok := (Adapter{}).Identity(failure); !ok || identity.Classname != failure.Classname || identity.Name != "TestX" {
		t.Fatalf("identity=%+v ok=%v", identity, ok)
	}
	build := failure
	build.Name = packageBuildIdentity
	if _, ok := (Adapter{}).Identity(build); ok {
		t.Fatal("the package build terminal was accepted as a test identity")
	}
}
