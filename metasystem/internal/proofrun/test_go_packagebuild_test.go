package proofrun

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestGoWholePackageWithoutTestsRequiresNativeBuildTerminal(t *testing.T) {
	root := t.TempDir()
	goodSnapshot := newTestSnapshotFactory(t, root, strings.Repeat("d", 40), map[string]testSnapshotEntry{
		"go.mod":         testSnapshotFile("module example.invalid/packagebuild\n\ngo 1.27\n", 0o644),
		"plain/plain.go": testSnapshotFile("package plain\nconst Value = 1\n", 0o644),
	}, 1)
	goodTree := goodSnapshot.tree
	group := testpolicy.Group{ID: "plain-build", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "plain/**"},
		Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"plain"}, Tests: []byte(`"all"`)}
	if err := CheckNativeDiscovery(context.Background(), root, root, testpolicy.Contract{SchemaVersion: testpolicy.SchemaVersion, Groups: []testpolicy.Group{group}}, os.Environ()); err == nil || !strings.Contains(err.Error(), "found no tests") {
		t.Fatalf("schema-1 no-test package discovery = %v; want legacy refusal", err)
	}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: goodTree, Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-mod=readonly -buildvcs=false"), LogRoot: filepath.Join(root, "logs")}
	request.openCandidate = goodSnapshot.open
	request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	good := runTestGroup(context.Background(), request, group)
	if good.Status != "passed" || !good.CollectionComplete || len(good.Expected) != 1 || good.Expected[0].Name != goPackageBuildIdentity ||
		len(good.Observed) != 1 || good.Observed[0].Status != "passed" || good.NativeExitStatus == nil || *good.NativeExitStatus != 0 {
		t.Fatalf("valid no-test package did not produce build evidence: %+v", good)
	}
	badSnapshot := newTestSnapshotFactory(t, root, strings.Repeat("e", 40), map[string]testSnapshotEntry{
		"go.mod":         testSnapshotFile("module example.invalid/packagebuild\n\ngo 1.27\n", 0o644),
		"plain/plain.go": testSnapshotFile("package plain\nvar Value int = \"compile failure\"\n", 0o644),
	}, 1)
	request.CandidateTree = badSnapshot.tree
	request.openCandidate = badSnapshot.open
	bad := runTestGroup(context.Background(), request, group)
	if bad.Status == "passed" || bad.NativeExitStatus == nil || *bad.NativeExitStatus == 0 {
		t.Fatalf("compile failure claimed a successful no-test package: %+v", bad)
	}
}

func TestGoWholePackageMissingOneOfTwoNativeTerminalsIsIncomplete(t *testing.T) {
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/twopackages\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "a", "a.go"), []byte("package a\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "b", "b.go"), []byte("package b\n"), 0o644)
	group := testpolicy.Group{Adapter: "go", Packages: []string{"a", "b"}, Tests: []byte(`"all"`)}
	argv, expected, _, _, err := goArgumentsForSchema(context.Background(), group, root, os.Environ(), testpolicy.ExecutionContractSchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != 2 || expected[0].Name != goPackageBuildIdentity || expected[1].Name != goPackageBuildIdentity {
		t.Fatalf("expected both native package-build identities, got %v", expected)
	}
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native package build: %v: %s", err, output)
	}
	_, missing, _, complete := parseGoJSON(output, expected)
	if !complete || len(missing) != 0 {
		t.Fatalf("complete native package output rejected: missing=%v output=%s", missing, output)
	}
	var truncated []string
	for _, line := range strings.Split(string(output), "\n") {
		if strings.Contains(line, `"Action":"skip"`) && strings.Contains(line, `"Package":"example.invalid/twopackages/b"`) {
			continue
		}
		truncated = append(truncated, line)
	}
	_, missing, _, complete = parseGoJSON([]byte(strings.Join(truncated, "\n")), expected)
	if complete || len(missing) == 0 {
		t.Fatalf("missing package-b terminal claimed complete: missing=%v", missing)
	}
}

func TestGoWholePackageExamplesRemainNativeTests(t *testing.T) {
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/examples\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "example_test.go"), []byte("package pkg\nimport \"fmt\"\nfunc Example() { fmt.Println(\"ok\") // Output: ok\n}\n"), 0o644)
	group := testpolicy.Group{Adapter: "go", Packages: []string{"pkg"}, Tests: []byte(`"all"`)}
	argv, expected, _, _, err := goArguments(context.Background(), group, root, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	if len(expected) != 1 || expected[0].Name != "Example" {
		t.Fatalf("example inventory = %v", expected)
	}
	command := exec.Command(argv[0], argv[1:]...)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native example: %v: %s", err, output)
	}
	observed, missing, unexpected, complete := parseGoJSON(output, expected)
	if !complete || len(missing) != 0 || len(unexpected) != 0 || len(observed) != 1 || observed[0].Status != "passed" {
		t.Fatalf("example evidence incomplete: observed=%v missing=%v unexpected=%v output=%s", observed, missing, unexpected, output)
	}
}

func TestGoSchema2WholePackageNativeSkipsRemainVisibleAndAccepted(t *testing.T) {
	root := t.TempDir()
	snapshot := newTestSnapshotFactory(t, root, strings.Repeat("f", 40), map[string]testSnapshotEntry{
		"go.mod":         testSnapshotFile("module example.invalid/native-skips\n\ngo 1.27\n", 0o644),
		"mixed/mixed.go": testSnapshotFile("package mixed\n", 0o644),
		"mixed/mixed_test.go": testSnapshotFile(`package mixed
import "testing"
func TestPass(t *testing.T) {}
func TestSkip(t *testing.T) { t.Skip("deliberate") }
`, 0o644),
		"onlyskip/onlyskip.go": testSnapshotFile("package onlyskip\n", 0o644),
		"onlyskip/onlyskip_test.go": testSnapshotFile(`package onlyskip
import "testing"
func TestSkip(t *testing.T) { t.Skip("deliberate") }
`, 0o644),
	}, 4)
	tree := snapshot.tree
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-mod=readonly -buildvcs=false"), LogRoot: filepath.Join(root, "logs")}
	request.openCandidate = snapshot.open
	group := testpolicy.Group{ID: "native-skips", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "mixed/**", "onlyskip/**"},
		Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}, Platforms: []string{"any"}, TargetMS: 1000,
		Packages: []string{"mixed"}, Tests: []byte(`"all"`)}
	request.Contract.SchemaVersion = testpolicy.ExecutionContractSchemaVersion
	mixed := runTestGroup(context.Background(), request, group)
	if mixed.Status != "passed" || !mixed.CollectionComplete || mixed.NativeExitStatus == nil || *mixed.NativeExitStatus != 0 || len(mixed.Observed) != 2 {
		t.Fatalf("schema-2 mixed native verdict: %+v", mixed)
	}
	statuses := map[string]string{}
	for _, observed := range mixed.Observed {
		statuses[observed.Name] = observed.Status
	}
	if statuses["TestPass"] != "passed" || statuses["TestSkip"] != "skipped" {
		t.Fatalf("mixed native observations = %v", mixed.Observed)
	}
	group.Packages = []string{"onlyskip"}
	onlySkipped := runTestGroup(context.Background(), request, group)
	if onlySkipped.Status != "passed" || !onlySkipped.CollectionComplete || len(onlySkipped.Observed) != 1 || onlySkipped.Observed[0].Status != "skipped" ||
		onlySkipped.NativeExitStatus == nil || *onlySkipped.NativeExitStatus != 0 {
		t.Fatalf("schema-2 all-skipped native verdict: %+v", onlySkipped)
	}
	group.Packages, group.Tests = []string{"mixed"}, []byte(`["TestSkip"]`)
	named := runTestGroup(context.Background(), request, group)
	if named.Status != "failed" || !named.CollectionComplete || len(named.Observed) != 1 || named.Observed[0].Status != "skipped" {
		t.Fatalf("schema-2 named skip unexpectedly accepted: %+v", named)
	}
	request.Contract.SchemaVersion = testpolicy.SchemaVersion
	group.Tests = []byte(`"all"`)
	legacy := runTestGroup(context.Background(), request, group)
	if legacy.Status != "failed" || !legacy.CollectionComplete || legacy.ExecutionIdentity == mixed.ExecutionIdentity {
		t.Fatalf("schema-1 skip or verdict identity unexpectedly accepted: %+v", legacy)
	}
}

func TestGoDocumentationOnlyExampleDoesNotClaimNativeTest(t *testing.T) {
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/documentation-example\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "docs", "docs.go"), []byte("package docs\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "docs", "docs_test.go"), []byte(`package docs
import "fmt"
func Example() { fmt.Println("documentation only") }
`), 0o644)
	writeTestResultFile(t, filepath.Join(root, "empty", "empty.go"), []byte("package empty\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "empty", "empty_test.go"), []byte(`package empty
func Example() { // Output:
}
`), 0o644)
	for _, check := range []struct {
		pkg, expected string
	}{{"docs", goPackageBuildIdentity}, {"empty", "Example"}} {
		group := testpolicy.Group{Adapter: "go", Packages: []string{check.pkg}, Tests: []byte(`"all"`)}
		argv, expected, _, _, err := goArgumentsForSchema(context.Background(), group, root, os.Environ(), testpolicy.ExecutionContractSchemaVersion)
		if err != nil || len(expected) != 1 || expected[0].Name != check.expected {
			t.Fatalf("%s native inventory = %v, err %v", check.pkg, expected, err)
		}
		command := exec.Command(argv[0], argv[1:]...)
		command.Dir = root
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("%s native execution: %v: %s", check.pkg, err, output)
		}
		observed, missing, unexpected, complete := parseGoJSON(output, expected)
		if !complete || len(missing) != 0 || len(unexpected) != 0 || len(observed) != 1 || observed[0].Status != "passed" {
			t.Fatalf("%s native evidence: observed=%v missing=%v unexpected=%v complete=%v output=%s", check.pkg, observed, missing, unexpected, complete, output)
		}
	}
}
