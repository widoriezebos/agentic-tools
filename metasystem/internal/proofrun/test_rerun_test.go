package proofrun

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestFailThenPassRerunIsAFindingNeverAGreen(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), "flipped")
	source := `package rerunflip

import (
	"os"
	"testing"
)

func TestFlipsOnSecondRun(t *testing.T) {
	marker := os.Getenv("RERUN_MARKER")
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		if err := os.WriteFile(marker, []byte("seen"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Fatal("first run")
	} else if err != nil {
		t.Fatal(err)
	}
}
`
	result := runRerunFixture(t, "rerun-flip", "rerunflip", source, map[string]string{"RERUN_MARKER": marker})
	if len(result.Reruns) != 1 {
		t.Fatalf("fail-then-pass reruns=%+v, want one finding", result.Reruns)
	}
	if result.Status != "failed" {
		t.Errorf("fail-then-pass result status=%q, want failed", result.Status)
	}
	finding := result.Reruns[0]
	validation := validSupervisorStatusResult(result.Status, "fixture failed")
	validation.WorkerPolicyVersion = TestWorkerPolicyVersion
	validation.Workers = 1
	validation.Groups[0].Reruns = append([]RerunFinding(nil), result.Reruns...)
	zero := 0
	validation.Groups[0].NativeLaunched = true
	validation.Groups[0].NativeExitStatus = &zero
	validation.Groups[0].CollectionComplete = true
	validation.RecomputeDelivery()
	if err := ValidateTestResult(validation); err != nil {
		t.Fatalf("group with rerun finding was rejected: %v", err)
	}
	wantPackage := "github.com/widoriezebos/agentic-tools/metasystem/internal/rerunflip"
	if finding.Package != wantPackage || finding.Test != "TestFlipsOnSecondRun" || finding.First != "failed" || finding.Second != "passed" {
		t.Fatalf("rerun finding=%+v", finding)
	}
	wantLoad := LoadSample{Sample: hostload.Sample{At: finding.FailedLoad.At, Load1m: 7.5, Load5m: 6.5, Load15m: 5.5, Cores: 8, Available: true},
		OverlappingHost: 1, OverlapKnown: true}
	if !reflect.DeepEqual(finding.FailedLoad, wantLoad) || !finding.FailedLoad.Loaded() {
		t.Fatalf("failed load=%+v, want %+v and loaded", finding.FailedLoad, wantLoad)
	}
	if finding.RerunLoad.At == "" || !finding.RerunLoad.Loaded() || finding.At != finding.RerunLoad.At {
		t.Fatalf("rerun timing/load is incomplete: %+v", finding)
	}
	if !strings.Contains(result.NotRunReason, "fail-then-pass under rerun: "+wantPackage+".TestFlipsOnSecondRun") {
		t.Fatalf("group reason omits fail-then-pass: %q", result.NotRunReason)
	}
	rerunLog, err := os.ReadFile(finding.LogPath)
	if err != nil || !strings.Contains(string(rerunLog), `"Action":"pass"`) || !strings.Contains(string(rerunLog), `"Test":"TestFlipsOnSecondRun"`) {
		t.Fatalf("rerun log does not contain the passing terminal event: err=%v\n%s", err, rerunLog)
	}
	groupLog, err := os.ReadFile(result.LogPath)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(groupLog), "\n"), "\n")
	if len(lines) < 2 || !strings.HasPrefix(lines[len(lines)-2], "TEST-RERUN rerun-flip "+wantPackage+".TestFlipsOnSecondRun first=failed second=passed") ||
		!strings.HasPrefix(lines[len(lines)-1], "TEST-VERDICT rerun-flip status=failed") {
		t.Fatalf("group log does not end with rerun then verdict:\n%s", groupLog)
	}
	wantDigest := fmt.Sprintf("%x", sha256.Sum256(groupLog))
	if result.LogDigest != wantDigest {
		t.Fatalf("group log digest=%q, want %q", result.LogDigest, wantDigest)
	}

	validation.Groups[0].Status = "passed"
	validation.Groups[0].NativeLaunched = true
	validation.Groups[0].NativeExitStatus = &zero
	validation.Groups[0].CollectionComplete = true
	validation.RecomputeDelivery()
	if err := ValidateTestResult(validation); err == nil || !strings.Contains(err.Error(), "rerun") {
		t.Fatalf("passed group with rerun finding was accepted: %v", err)
	}
}

func TestFailThenFailStaysRedAndNamesBothRuns(t *testing.T) {
	t.Parallel()
	result := runRerunFixture(t, "rerun-red", "rerunred", `package rerunred

import "testing"

func TestAlwaysFails(t *testing.T) { t.Fatal("red") }
`, nil)
	if result.Status != "failed" || len(result.Reruns) != 1 {
		t.Fatalf("fail-then-fail result status=%q reruns=%+v", result.Status, result.Reruns)
	}
	finding := result.Reruns[0]
	if finding.Package != "github.com/widoriezebos/agentic-tools/metasystem/internal/rerunred" || finding.Test != "TestAlwaysFails" ||
		finding.First != "failed" || finding.Second != "failed" || finding.FailedLoad.At == "" || finding.RerunLoad.At == "" ||
		!finding.FailedLoad.Loaded() || !finding.RerunLoad.Loaded() {
		t.Fatalf("fail-then-fail finding=%+v", finding)
	}
	if strings.Contains(result.NotRunReason, "fail-then-pass") {
		t.Fatalf("fail-then-fail reason claims a pass: %q", result.NotRunReason)
	}
}

func TestDiagnosticRerunExportsReservedWorkerShare(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeTestResultFile(t, filepath.Join(root, "go.mod"), []byte("module example.invalid/rerunleaf\n\ngo 1.27\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg.go"), []byte("package pkg\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "pkg", "pkg_test.go"), []byte(`package pkg
import ("os"; "testing")
func TestPass(t *testing.T) {
 if workers := os.Getenv("METASYSTEM_TEST_WORKERS"); workers != "1" { t.Fatalf("diagnostic worker allowance=%q, want 1", workers) }
}
`), 0o644)
	request := TestRunRequest{Workers: 2, LogRoot: filepath.Join(root, "logs"),
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false")}
	if err := os.MkdirAll(request.LogRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	group := testpolicy.Group{ID: "performance-rerun", Kind: "performance", Adapter: "go"}
	environment := groupTestEnvironment(request, group)
	if !strings.Contains(strings.Join(environment, "\n"), TestWorkersEnvironment+"=2") {
		t.Fatalf("performance group did not retain the aggregate allowance: %v", environment)
	}
	finding := runFailedTestAgain(t.Context(), request, group, root, environment, supervisorLimits{}, time.Second,
		NativeTestIdentity{Classname: "example.invalid/rerunleaf/pkg", Name: "TestPass"}, 1, LoadSample{})
	if finding.First != "failed" || finding.Second != "passed" {
		t.Fatalf("diagnostic rerun did not export its reserved one-worker share: %+v", finding)
	}
}

func TestRerunCapAndNonTestFailuresAreNotRerun(t *testing.T) {
	t.Parallel()
	result := runRerunFixture(t, "rerun-cap", "reruncap", `package reruncap

import "testing"

func TestA(t *testing.T) { t.Fatal("a") }
func TestB(t *testing.T) { t.Fatal("b") }
func TestC(t *testing.T) { t.Fatal("c") }
func TestD(t *testing.T) { t.Fatal("d") }
func TestE(t *testing.T) { t.Fatal("e") }
func TestF(t *testing.T) { t.Fatal("f") }
func TestG(t *testing.T) { t.Fatal("g") }
`, nil)
	if result.Status != "failed" || len(result.Reruns) != 5 || result.RerunNote != "cap 5: 2 failed tests not rerun" {
		t.Fatalf("capped reruns status=%q count=%d note=%q rows=%+v", result.Status, len(result.Reruns), result.RerunNote, result.Reruns)
	}
	for index, name := range []string{"TestA", "TestB", "TestC", "TestD", "TestE"} {
		if result.Reruns[index].Test != name || result.Reruns[index].Second != "failed" {
			t.Fatalf("rerun %d=%+v, want %s failed", index, result.Reruns[index], name)
		}
	}

	broken := runRerunFixtureWithFiles(t, "rerun-compile", "reruncap", map[string]string{
		"broken.go":      "package reruncap\n\nvar Broken = doesNotExist\n",
		"broken_test.go": "package reruncap\n\nimport \"testing\"\n\nfunc TestNeverRuns(t *testing.T) {}\n",
	}, nil)
	if (broken.Status != "failed" && broken.Status != "invalid") || len(broken.Reruns) != 0 || broken.RerunNote != "" {
		t.Fatalf("non-test compile failure was rerun: status=%q reruns=%+v note=%q reason=%q", broken.Status, broken.Reruns, broken.RerunNote, broken.NotRunReason)
	}
}

func runRerunFixture(t *testing.T, groupID, packageName, testSource string, environment map[string]string) GroupResult {
	t.Helper()
	return runRerunFixtureWithFiles(t, groupID, packageName, map[string]string{packageName + "_test.go": testSource}, environment)
}

func runRerunFixtureWithFiles(t *testing.T, groupID, packageName string, files map[string]string, environment map[string]string) GroupResult {
	t.Helper()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "metasystem", "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n"), 0o644)
	for name, source := range files {
		writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", packageName, name), []byte(source), 0o644)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	group := testpolicy.Group{ID: groupID, Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs:      []string{"metasystem/go.mod", "metasystem/internal/" + packageName + "/**"},
		Tools:       []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"rerun"}, Platforms: []string{"any"}, TargetMS: 60000,
		Packages: []string{"internal/" + packageName}, Tests: []byte(`"all"`), Env: environment}
	return runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: tree,
		Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), LogRoot: filepath.Join(root, "logs"),
		loadOptions: []loadSampleOption{withScriptedLoad(hostload.Sample{Load1m: 7.5, Load5m: 6.5, Load15m: 5.5, Cores: 8, Available: true}, 1, true)}}, group)
}
