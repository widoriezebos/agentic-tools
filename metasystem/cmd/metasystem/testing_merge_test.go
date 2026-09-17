package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

func TestTestingMergeDriverUsesGitArgumentOrderAndPrintsUsage(t *testing.T) {
	root := t.TempDir()
	base := testingMergeFixture()
	ours, theirs := testingMergeClone(t, base), testingMergeClone(t, base)
	ours.Groups[0].Tests = json.RawMessage(`["TestBase","TestOurs"]`)
	theirs.Groups[0].Tests = json.RawMessage(`["TestBase","TestTheirs"]`)
	paths := []string{filepath.Join(root, "base.json"), filepath.Join(root, "ours.json"), filepath.Join(root, "theirs.json")}
	for i, contract := range []testpolicy.Contract{base, ours, theirs} {
		data, err := contractmerge.Render(contract)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[i], data, 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if code := runTestingMergeDriver(paths); code != 0 {
		t.Fatalf("merge driver exit = %d", code)
	}
	merged, err := testpolicy.Load(paths[1])
	if err != nil {
		t.Fatal(err)
	}
	_, names, err := testpolicy.GoTests(merged.Groups[0])
	if err != nil || !reflect.DeepEqual(names, []string{"TestBase", "TestOurs", "TestTheirs"}) {
		t.Fatalf("driver wrote tests=%v err=%v", names, err)
	}
	if info, err := os.Stat(paths[1]); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("driver did not preserve ours mode: info=%v err=%v", info, err)
	}

	stderr := captureTestingStderr(t, func() { runTestingMergeDriver(nil) })
	for _, want := range []string{"BASE OURS THEIRS", "%O %A %B", "metasystem/testing.json merge=metasystem-testing", "git config merge.metasystem-testing.driver"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("usage missing %q:\n%s", want, stderr)
		}
	}
}

func TestTestingMergeVerbRoutesAndPreservesOutputOnRefusal(t *testing.T) {
	root := t.TempDir()
	base := testingMergeFixture()
	base.Groups = append(base.Groups, base.Groups[0])
	base.Groups[1].ID = "spare"
	ours, theirs := testingMergeClone(t, base), testingMergeClone(t, base)
	ours.Groups = ours.Groups[:1]
	theirs.Surfaces[0].Standard = append(theirs.Surfaces[0].Standard, "spare")
	paths := []string{filepath.Join(root, "base.json"), filepath.Join(root, "ours.json"), filepath.Join(root, "theirs.json")}
	for i, contract := range []testpolicy.Contract{base, ours, theirs} {
		data, err := contractmerge.Render(contract)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[i], data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	out := filepath.Join(root, "out.json")
	marker := []byte("unchanged on refusal\n")
	if err := os.WriteFile(out, marker, 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"testing", "merge", "--base", paths[0], "--ours", paths[1], "--theirs", paths[2], "--out", out}
	if code := dispatch(args); code != 1 {
		t.Fatalf("invalid merge dispatch exit = %d", code)
	}
	if got, err := os.ReadFile(out); err != nil || !reflect.DeepEqual(got, marker) {
		t.Fatalf("refused merge changed output: got=%q err=%v", got, err)
	}

	clean := testingMergeFixture()
	for i := range paths {
		data, err := contractmerge.Render(clean)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(paths[i], data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if code := dispatch(args); code != 0 {
		t.Fatalf("valid merge dispatch exit = %d", code)
	}
	if _, err := testpolicy.Load(out); err != nil {
		t.Fatalf("merge output did not load: %v", err)
	}
}

func TestTestingAddTestsWritesCanonicalContract(t *testing.T) {
	root := t.TempDir()
	packageDir := filepath.Join(root, "internal", "example")
	if err := os.MkdirAll(packageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(packageDir, "example_test.go"), []byte("package example\nfunc TestBase(t any) {}\nfunc TestAdded(t any) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	contract := testingMergeFixture()
	contract.Groups[0].Packages = []string{"internal/example"}
	path := filepath.Join(root, "testing.json")
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	args := []string{"--file", path, "--group", "app-group", "--tests", "TestAdded,TestAdded"}
	if code := runTestingAddTests(args); code != 0 {
		t.Fatalf("add-tests exit = %d", code)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if code := runTestingAddTests(args); code != 0 {
		t.Fatalf("second add-tests exit = %d", code)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("idempotent add-tests changed canonical bytes")
	}
	loaded, err := testpolicy.Decode(second)
	if err != nil {
		t.Fatal(err)
	}
	_, names, _ := testpolicy.GoTests(loaded.Groups[0])
	if !reflect.DeepEqual(names, []string{"TestAdded", "TestBase"}) {
		t.Fatalf("written tests = %v", names)
	}
}

func testingMergeFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			{ID: "app", Paths: []string{"app/**"}, DependsOn: []string{}, Standard: []string{"app-group"}, Deep: []string{}, Critical: []string{}},
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"app-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups: []testpolicy.Group{{ID: "app-group", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{}, Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"internal/example"}, Tests: json.RawMessage(`["TestBase"]`)}},
		Always: testpolicy.Always{Canary: []string{}, Standard: []string{}}, Unknown: []string{"app-group"}, Cadence: []string{}}
}

func testingMergeClone(t *testing.T, contract testpolicy.Contract) testpolicy.Contract {
	t.Helper()
	data, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	var result testpolicy.Contract
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func captureTestingStderr(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stderr
	os.Stderr = writer
	run()
	os.Stderr = old
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return string(data)
}
