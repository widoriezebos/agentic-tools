package goadapter

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

func TestGoAdapterTestStepsWholeChangedPackages(t *testing.T) {
	t.Parallel()
	closure := adapter.Closure{Changed: []string{"./cmd/tool", "./internal/lib/", "."}, Dependents: []string{"./dependent"}}
	steps := (Adapter{}).TestSteps(closure)
	want := []adapter.GateStep{
		{Name: "package-1", Args: []string{"go", "test", "-count=1", "-timeout", "30m", "./cmd/tool/"}},
		{Name: "package-2", Args: []string{"go", "test", "-count=1", "-timeout", "30m", "./internal/lib/"}},
		{Name: "package-3", Args: []string{"go", "test", "-count=1", "-timeout", "30m", "./"}},
	}
	if !reflect.DeepEqual(steps, want) || len((Adapter{}).TestSteps(adapter.Closure{})) != 0 {
		t.Fatalf("steps=%+v want=%+v", steps, want)
	}
}

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
