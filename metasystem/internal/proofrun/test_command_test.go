package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestVersionIdentityHelper(t *testing.T) {
	mode := os.Getenv("GO_WANT_VERSION_IDENTITY_HELPER")
	if mode == "" {
		return
	}
	if mode == "hang" {
		time.Sleep(10 * time.Second)
		return
	}
	file, err := os.OpenFile(os.Getenv("VERSION_IDENTITY_MARKER"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("started\n"); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestPreparedToolIdentityStartsVersionHelperOnceAndHonorsBound(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	marker := filepath.Join(root, "version-starts")
	environment := mergeTestEnvironment(os.Environ(), map[string]string{
		"GO_WANT_VERSION_IDENTITY_HELPER": "count",
		"VERSION_IDENTITY_MARKER":         marker,
	})
	group := testpolicy.Group{ID: "command", Adapter: "command", CWD: ".",
		Argv: []string{executable, "-test.run=^TestVersionIdentityHelper$"},
		Tools: []testpolicy.Tool{{ID: "fixture", Executable: executable,
			VersionArgs: []string{"-test.run=^TestVersionIdentityHelper$"}}}}
	_, executableDigests, argv, _, _, _, _, launches, err := plannedToolIdentities(context.Background(), root, environment, group, root, nil)
	if err != nil || launches != 1 {
		t.Fatalf("prepared identity starts=%d err=%v", launches, err)
	}
	if err := verifyPreparedExecutables(context.Background(), root, environment, group, argv, executableDigests); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(marker)
	if err != nil || string(data) != "started\n" {
		t.Fatalf("version helper was repeated after preparation: %q err=%v", data, err)
	}

	hangingEnvironment := mergeTestEnvironment(os.Environ(), map[string]string{"GO_WANT_VERSION_IDENTITY_HELPER": "hang"})
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, _, started, err := identifyTool(ctx, root, hangingEnvironment, group.Tools[0])
	if !started || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("hanging version helper started=%v err=%v", started, err)
	}
}

func TestGoDiscoveryCatalogIsLoadedOncePerModuleEnvironment(t *testing.T) {
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/discovery-cache\n\ngo 1.23\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "sample", "sample.go"), []byte("package sample\nfunc Value() int { return 1 }\n"), 0o644)
	writeTestFile(t, filepath.Join(root, "sample", "sample_test.go"), []byte("package sample\nimport \"testing\"\nfunc TestOne(t *testing.T) {}\nfunc TestTwo(t *testing.T) {}\n"), 0o644)
	realGo, err := exec.LookPath("go")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(root, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "go-starts")
	wrapper := filepath.Join(bin, "go")
	body := "#!/bin/sh\nprintf '%s\\n' \"$*\" >>\"$GO_DISCOVERY_MARKER\"\nexec " + fmt.Sprintf("%q", realGo) + " \"$@\"\n"
	writeTestFile(t, wrapper, []byte(body), 0o755)
	environment := mergeTestEnvironment(os.Environ(), map[string]string{"PATH": bin, "GO_DISCOVERY_MARKER": marker, "GOFLAGS": "-mod=readonly"})
	cache := &goDiscoveryCache{catalogs: map[string]goPackageCatalog{}}
	first := testpolicy.Group{ID: "first", Adapter: "go", CWD: ".", Packages: []string{"./sample"}, Tests: json.RawMessage(`["TestOne"]`)}
	second := testpolicy.Group{ID: "second", Adapter: "go", CWD: ".", Packages: []string{"./sample"}, Tests: json.RawMessage(`["TestTwo"]`)}
	_, _, _, firstStarted, err := goArgumentsCached(context.Background(), first, root, environment, cache)
	if err != nil || !firstStarted {
		t.Fatalf("first discovery started=%v err=%v", firstStarted, err)
	}
	_, _, _, secondStarted, err := goArgumentsCached(context.Background(), second, root, environment, cache)
	if err != nil || secondStarted {
		t.Fatalf("cached discovery started=%v err=%v", secondStarted, err)
	}
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(data), "list -json -deps -test ./..."); got != 1 {
		t.Fatalf("Go discovery started %d times, log=%q", got, data)
	}
}

func TestCommandDetectsOmittedExpectedTestsAfterFirstFailure(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	report := `<testsuite><testcase classname="suite" name="first"><failure message="red"/></testcase></testsuite>`
	if err := os.WriteFile(filepath.Join(root, "reports", "tests.xml"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{Reports: []string{"reports"}, ExpectedTests: []testpolicy.ExpectedTest{
		{Report: "reports/tests.xml", Classname: "suite", Name: "first"},
		{Report: "reports/tests.xml", Classname: "suite", Name: "second"},
	}}
	observed, missing, unexpected, complete, _, err := parseJUnit(root, group)
	if err != nil || complete || len(observed) != 1 || observed[0].Status != "failed" || len(missing) != 1 || missing[0].Name != "second" || len(unexpected) != 0 {
		t.Fatalf("junit collection observed=%+v missing=%+v unexpected=%+v complete=%v err=%v", observed, missing, unexpected, complete, err)
	}
}

func TestSectionPrerequisiteIsBlockedNotPassed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stage.tsv")
	if err := os.WriteFile(path, []byte("section\tneeds-engine\tgated\t0\tengine prerequisite failed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, _, blocked, complete := parseSectionResult(path, "needs-engine")
	if status != "unavailable" || complete || len(blocked) != 1 || blocked[0].Reason != "engine prerequisite failed" {
		t.Fatalf("blocked section status=%s complete=%v blocked=%+v", status, complete, blocked)
	}
}

func TestSectionPreservesNativeExitStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stage.tsv")
	if err := os.WriteFile(path, []byte("section\texact-status\tfail\t23\tfirst failure\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	status, exit, _, complete := parseSectionResult(path, "exact-status")
	if status != "failed" || exit != 23 || !complete {
		t.Fatalf("section result status=%s exit=%d complete=%v", status, exit, complete)
	}
}

func TestGoCollectionKeepsSubtestsAndDetectsMissingTerminalEvents(t *testing.T) {
	completeOutput := []byte("" +
		`{"Action":"run","Package":"example/app","Test":"TestApplication"}` + "\n" +
		`{"Action":"run","Package":"example/app","Test":"TestApplication/variation"}` + "\n" +
		`{"Action":"pass","Package":"example/app","Test":"TestApplication/variation"}` + "\n" +
		`{"Action":"pass","Package":"example/app","Test":"TestApplication"}` + "\n")
	observed, missing, unexpected, complete := parseGoJSON(completeOutput, []NativeTestIdentity{{Name: "TestApplication", Status: "expected"}})
	if !complete || len(observed) != 2 || len(missing) != 0 || len(unexpected) != 0 {
		t.Fatalf("complete subtest collection observed=%+v missing=%+v unexpected=%+v complete=%v", observed, missing, unexpected, complete)
	}
	partial := []byte(`{"Action":"run","Package":"example/app","Test":"TestApplication"}` + "\n")
	_, missing, _, complete = parseGoJSON(partial, []NativeTestIdentity{{Name: "TestApplication", Status: "expected"}})
	if complete || len(missing) == 0 || missing[0].Status != "missing-terminal" {
		t.Fatalf("partial Go report claimed complete: missing=%+v complete=%v", missing, complete)
	}
}

func TestGoDiscoveryBindsDeclaredNamesToActualPackages(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/application\n\ngo 1.22\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range []string{"first", "second"} {
		directory := filepath.Join(root, pkg)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(directory, pkg+".go"), []byte("package "+pkg+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		source := "package " + pkg + "\nimport \"testing\"\nfunc TestShared(t *testing.T) {}\n"
		if err := os.WriteFile(filepath.Join(directory, pkg+"_test.go"), []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	group := testpolicy.Group{Adapter: "go", Packages: []string{"first", "second"}, Tests: []byte(`["TestShared","TestMissing"]`)}
	_, expected, discovery, started, err := goArguments(context.Background(), group, root, os.Environ())
	if err != nil || len(expected) != 3 || expected[0].Name != "TestMissing" || expected[0].Classname != "" ||
		expected[1].Classname != "example.invalid/application/first" || expected[2].Classname != "example.invalid/application/second" {
		t.Fatalf("Go discovery expected=%+v started=%v inputs=%v err=%v", expected, started, discovery.Inputs, err)
	}
}

func TestGoDiscoveryBindsDependencyConsumerEmbedAndTestdataClosure(t *testing.T) {
	root := t.TempDir()
	write := func(relative, body string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.invalid/closure\n\ngo 1.22\n")
	write("provider/provider.go", "package provider\nimport _ \"embed\"\n//go:embed embedded.txt\nvar Embedded string\n")
	write("provider/embedded.txt", "embedded\n")
	write("target/target.go", "package target\nimport \"example.invalid/closure/provider\"\nvar Value = provider.Embedded\n")
	write("target/target_test.go", "package target\nimport (\"testing\"; _ \"example.invalid/closure/testdep\")\nfunc TestTarget(t *testing.T) {}\n")
	write("target/testdata/case.txt", "fixture\n")
	write("testdep/testdep.go", "package testdep\n")
	write("consumer/consumer.go", "package consumer\nimport _ \"example.invalid/closure/target\"\n")
	group := testpolicy.Group{Adapter: "go", Packages: []string{"target"}, Tests: json.RawMessage(`"all"`)}
	_, _, discovery, started, err := goArguments(context.Background(), group, root, os.Environ())
	if err != nil || !started {
		t.Fatalf("Go closure discovery started=%v err=%v", started, err)
	}
	for _, required := range []string{
		"provider/provider.go", "provider/embedded.txt", "target/target.go", "target/target_test.go",
		"target/testdata/case.txt", "testdep/testdep.go", "consumer/consumer.go",
	} {
		found := false
		for _, observed := range discovery.Inputs {
			found = found || observed == required
		}
		if !found {
			t.Fatalf("Go discovery omitted %s from dependency/consumer closure: %v", required, discovery.Inputs)
		}
	}

	group.Coverage = true
	_, _, withPolicy, _, err := groupArguments(context.Background(), group, root, root, os.Environ(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, policyPath := range coverageBaselineInputs() {
		found := false
		for _, observed := range withPolicy.Inputs {
			found = found || observed == policyPath
		}
		if !found {
			t.Fatalf("coverage policy input %s is absent from group identity: %v", policyPath, withPolicy.Inputs)
		}
	}
}

func TestExternalInputContentAndAbsenceEnterGroupIdentity(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("source\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	externalRoot := t.TempDir()
	externalPath := filepath.Join(externalRoot, "tool.conf")
	if err := os.WriteFile(externalPath, []byte("first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{Inputs: []string{"source.txt"}, ExternalInputs: []testpolicy.ExternalInput{{ID: "tool-config", Path: "${TOOL_HOME}/tool.conf"}}}
	environment := []string{"TOOL_HOME=" + externalRoot}
	first, err := digestGroupInputs(root, group, environment)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(externalPath, []byte("second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := digestGroupInputs(root, group, environment)
	if err != nil || second == first {
		t.Fatalf("external content did not change the group identity: first=%s second=%s err=%v", first, second, err)
	}
	if err := os.Remove(externalPath); err != nil {
		t.Fatal(err)
	}
	absent, err := digestGroupInputs(root, group, environment)
	if err != nil || absent == second {
		t.Fatalf("declared absent external input lacks a stable marker: second=%s absent=%s err=%v", second, absent, err)
	}
	if _, err := digestGroupInputs(root, group, nil); err == nil {
		t.Fatal("unset external input locator was accepted")
	}
}

func TestNativeDiscoveryRejectsDeclaredMissingGoTestWithoutCompiling(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.invalid/app\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app_test.go"), []byte("package app\nimport \"testing\"\nfunc TestPresent(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("package app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "app", Adapter: "go", CWD: ".", Packages: []string{"."}, Tests: json.RawMessage(`["TestMissing"]`)}
	if err := CheckNativeDiscovery(root, root, testpolicy.Contract{Groups: []testpolicy.Group{group}}); err == nil || !strings.Contains(err.Error(), "TestMissing") {
		t.Fatalf("missing declared Go test passed metadata discovery: %v", err)
	}
}

func TestJUnitFailureCanStillBeACompleteFailureCollection(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	report := `<testsuite><testcase classname="suite" name="first"><failure message="red"/></testcase><testcase classname="suite" name="second"/></testsuite>`
	if err := os.WriteFile(filepath.Join(root, "reports", "tests.xml"), []byte(report), 0o600); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{Reports: []string{"reports"}, ExpectedTests: []testpolicy.ExpectedTest{
		{Report: "reports/tests.xml", Classname: "suite", Name: "first"},
		{Report: "reports/tests.xml", Classname: "suite", Name: "second"},
	}}
	observed, missing, unexpected, complete, _, err := parseJUnit(root, group)
	if err != nil || !complete || len(observed) != 2 || len(missing) != 0 || len(unexpected) != 0 || observed[0].Status != "failed" {
		t.Fatalf("complete failing JUnit collection observed=%+v missing=%+v unexpected=%+v complete=%v err=%v", observed, missing, unexpected, complete, err)
	}
}
