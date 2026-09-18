package parallelratchet

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanParallelTestsDetectsFirstStatementAcrossFormsAndBuildTags(t *testing.T) {
	t.Parallel()
	root := parallelFixtureModule(t)
	writeParallelFixture(t, filepath.Join(root, "plain_test.go"), `package fixture
import "testing"
func TestParallelFirst(t *testing.T) { t.Parallel(); t.Log("okay") }
func TestParallelLater(t *testing.T) { t.Helper(); t.Parallel() }
func TestSubtestOnly(t *testing.T) { t.Run("child", func(t *testing.T) { t.Parallel() }) }
func TestTableDriven(t *testing.T) { t.Parallel(); for _, item := range []int{1, 2} { t.Run("case", func(t *testing.T) { _ = item }) } }
func TestMain(m *testing.M) {}
`)
	writeParallelFixture(t, filepath.Join(root, "tagged_test.go"), `//go:build fixturetag

package fixture
import "testing"
func TestBuildTagged(t *testing.T) { t.Parallel() }
`)

	inventory, err := ScanParallelTests(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"example.test/fixture"}; !reflect.DeepEqual(inventory.Packages, want) {
		t.Fatalf("packages = %v, want %v", inventory.Packages, want)
	}
	got := map[string]bool{}
	for _, test := range inventory.Tests {
		got[test.Test] = test.Parallel
		if test.File == "tagged_test.go" && test.Line != 5 {
			t.Errorf("tagged test line = %d, want 5", test.Line)
		}
	}
	want := map[string]bool{
		"TestParallelFirst": true,
		"TestParallelLater": false,
		"TestSubtestOnly":   false,
		"TestTableDriven":   true,
		"TestBuildTagged":   true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parallel detection = %#v, want %#v", got, want)
	}
}

func TestCheckParallelRatchetUsesPackageCeilingsAndReasonedExemptions(t *testing.T) {
	t.Parallel()
	inventory := ParallelInventory{
		Packages: []string{"example.test/fixture", "example.test/new"},
		Tests: []ParallelTest{
			{Package: "example.test/fixture", Test: "TestExempt", File: "fixture_test.go", Line: 3},
			{Package: "example.test/fixture", Test: "TestEmptyReason", File: "fixture_test.go", Line: 7},
			{Package: "example.test/fixture", Test: "TestParallel", File: "fixture_test.go", Line: 11, Parallel: true},
			{Package: "example.test/new", Test: "TestAbsentPackage", File: "new_test.go", Line: 5},
		},
	}
	ratchet := ParallelRatchet{
		Packages: map[string]int{"example.test/fixture": 0},
		Exempt: []ParallelExemption{
			{Package: "example.test/fixture", Test: "TestExempt", Reason: "owns process-wide state"},
			{Package: "example.test/fixture", Test: "TestEmptyReason"},
		},
	}
	counts, violations := CheckParallelRatchet(ratchet, inventory)
	if counts["example.test/fixture"] != 1 || counts["example.test/new"] != 1 {
		t.Fatalf("serial counts = %#v, want one in each package", counts)
	}
	if len(violations) != 2 {
		t.Fatalf("violations = %#v, want two", violations)
	}
	if violations[0].Test != "TestEmptyReason" || violations[0].Recorded != 0 || violations[0].Actual != 1 {
		t.Fatalf("first violation = %#v", violations[0])
	}
	if violations[1].Package != "example.test/new" || violations[1].Test != "TestAbsentPackage" || violations[1].File != "new_test.go" || violations[1].Line != 5 {
		t.Fatalf("absent-package violation = %#v", violations[1])
	}
}

func TestLowerParallelRatchetDropsCountsAndRefusesRaises(t *testing.T) {
	t.Parallel()
	baseline := ParallelRatchet{
		Packages: map[string]int{"example.test/gone": 2, "example.test/lower": 3, "example.test/steady": 1},
		Exempt:   []ParallelExemption{{Package: "example.test/lower", Test: "TestReason", Reason: "documented"}},
	}
	inventory := ParallelInventory{
		Packages: []string{"example.test/lower", "example.test/new", "example.test/steady"},
		Tests: []ParallelTest{
			{Package: "example.test/lower", Test: "TestSerial"},
			{Package: "example.test/lower", Test: "TestReason"},
			{Package: "example.test/new", Test: "TestParallel", Parallel: true},
			{Package: "example.test/steady", Test: "TestSerial"},
		},
	}
	updated, drops, violations := LowerParallelRatchet(baseline, inventory)
	if len(violations) != 0 {
		t.Fatalf("unexpected violations: %#v", violations)
	}
	wantCounts := map[string]int{"example.test/gone": 0, "example.test/lower": 1, "example.test/new": 0, "example.test/steady": 1}
	if !reflect.DeepEqual(updated.Packages, wantCounts) {
		t.Fatalf("updated counts = %#v, want %#v", updated.Packages, wantCounts)
	}
	wantDrops := []ParallelDrop{{Package: "example.test/gone", From: 2, To: 0}, {Package: "example.test/lower", From: 3, To: 1}}
	if !reflect.DeepEqual(drops, wantDrops) {
		t.Fatalf("drops = %#v, want %#v", drops, wantDrops)
	}

	raiseBaseline := ParallelRatchet{Packages: map[string]int{"example.test/steady": 0}}
	refused, refusedDrops, raiseViolations := LowerParallelRatchet(raiseBaseline, inventory)
	if len(raiseViolations) != 3 || refusedDrops != nil || !reflect.DeepEqual(refused, raiseBaseline) {
		t.Fatalf("raise result = %#v, %#v, %#v", refused, refusedDrops, raiseViolations)
	}
}

func TestParallelRatchetRoundTripsCanonicalListObjects(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "testing-parallel-ratchet.json")
	ratchet := ParallelRatchet{
		Packages: map[string]int{"example.test/z": 2, "example.test/a": 1},
		Exempt:   []ParallelExemption{{Package: "example.test/a", Test: "TestSerial", Reason: "owns a singleton"}},
	}
	if err := WriteParallelRatchet(path, root, ratchet); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `    {"package":"example.test/a","test":"TestSerial","reason":"owns a singleton"}`) {
		t.Fatalf("exemption is not one object on one line:\n%s", data)
	}
	if strings.Index(string(data), "example.test/a") > strings.Index(string(data), "example.test/z") {
		t.Fatalf("package keys are not sorted:\n%s", data)
	}
	decoded, err := ReadParallelRatchet(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(decoded, ratchet) {
		t.Fatalf("round trip = %#v, want %#v", decoded, ratchet)
	}
}

func TestGoGateFastModeRunsParallelRatchetBesideDependencyRatchet(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "scripts", "agents", "go-gate.sh"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(data)
	dependency := strings.Index(source, "audit dependency-ratchet --root")
	parallel := strings.Index(source, "audit parallel-ratchet --root")
	registration := strings.Index(source, "# A STANDALONE go-gate run registers itself")
	if dependency < 0 || parallel < dependency || registration < parallel {
		t.Fatalf("fast gate does not run the parallel ratchet beside the dependency ratchet")
	}
}

func parallelFixtureModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeParallelFixture(t, filepath.Join(root, "go.mod"), "module example.test/fixture\n\ngo 1.27\n")
	return root
}

func writeParallelFixture(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
