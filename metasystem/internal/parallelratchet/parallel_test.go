package parallelratchet

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestReadParallelRatchetReadsTheCanonicalBaselineAndRefusesAMalformedOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	path := filepath.Join(root, "testing-parallel-ratchet.json")
	writeParallelFixture(t, path, "{\n  \"packages\": {\n    \"example.test/a\": 1,\n    \"example.test/z\": 2\n  },\n  \"exempt\": [\n"+
		"    {\"package\":\"example.test/a\",\"test\":\"TestSerial\",\"reason\":\"owns a singleton\"}\n  ]\n}\n")
	decoded, err := ReadParallelRatchet(path)
	if err != nil {
		t.Fatal(err)
	}
	want := ParallelRatchet{
		Packages: map[string]int{"example.test/z": 2, "example.test/a": 1},
		Exempt:   []ParallelExemption{{Package: "example.test/a", Test: "TestSerial", Reason: "owns a singleton"}},
	}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("read = %#v, want %#v", decoded, want)
	}
	for name, content := range map[string]string{
		"unknown field":  `{"packages":{},"extra":1}`,
		"no packages":    `{"exempt":[]}`,
		"negative count": `{"packages":{"example.test/a":-1}}`,
		"blank package":  `{"packages":{" ":1}}`,
		"two values":     `{"packages":{}} {}`,
	} {
		writeParallelFixture(t, path, content)
		if _, err := ReadParallelRatchet(path); err == nil {
			t.Fatalf("%s was accepted", name)
		}
	}
	if _, err := ReadParallelRatchet(filepath.Join(root, "absent.json")); err == nil {
		t.Fatal("an absent baseline was accepted")
	}
}
