package proofrun

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNewTestResultKeepsDigestOnlyIdentityForLegacyPolicyProbe(t *testing.T) {
	digestOnly := NewTestResult(TestRunRequest{CandidateEngineDigest: strings.Repeat("a", 64)})
	if digestOnly.CandidateEngineIdentityVersion != candidateEngineDigestIdentityVersion || digestOnly.CandidateEngineBuildIdentity != "" {
		t.Fatalf("digest-only candidate identity was not projected as version one: %+v", digestOnly)
	}
	fieldBearing := NewTestResult(TestRunRequest{CandidateEngineDigest: strings.Repeat("a", 64),
		CandidateEngineBuildIdentity: strings.Repeat("b", 40)})
	if fieldBearing.CandidateEngineIdentityVersion != CandidateEngineIdentitySchemaVersion ||
		fieldBearing.CandidateEngineBuildIdentity == "" {
		t.Fatalf("field-bearing candidate identity was not projected as version two: %+v", fieldBearing)
	}
}

func TestSectionEnginePreparationPreservesBytesAndNestedSelector(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "vendor", "engine with spaces")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "engine")
	data := []byte("#!/bin/sh\nexit 0\n")
	if err := os.WriteFile(source, data, 0o500); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSectionEngine(cwd, source, strings.Repeat("0", 64)); err == nil {
		t.Fatal("tampered executable was declared ready")
	}
	engine, err := prepareSectionEngine(cwd, source, digestBytes(data))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := os.ReadFile(engine)
	if err != nil || string(actual) != string(data) {
		t.Fatalf("prepared bytes changed: %v", err)
	}
	if _, err := prepareSectionEngine(cwd, source, digestBytes(data)); err == nil {
		t.Fatal("candidate-owned executable was overwritten")
	}
	if after, _ := os.ReadFile(engine); string(after) != string(data) {
		t.Fatal("refused preparation changed candidate bytes")
	}
	group := testpolicy.Group{ID: "section/fixture", Adapter: "section", CWD: "vendor/engine with spaces", Section: "fixture"}
	argv, _, _, _, err := groupArguments(context.Background(), group, root, cwd, nil, nil)
	if err != nil || len(argv) != 4 || argv[1] != "scripts/agents/validate-section-selector.sh" || argv[3] != "fixture" {
		t.Fatalf("prepared selector retained a temporary absolute root: %v, %v", argv, err)
	}
	foreign := t.TempDir()
	if err := os.Symlink(foreign, filepath.Join(root, "bin")); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareSectionEngine(root, source, digestBytes(data)); err == nil {
		t.Fatal("section engine followed a foreign bin symlink")
	}
}

func TestMetaSystemStewardConsumerReceivesCandidateEngine(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "metasystem", "go.mod"), []byte("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n"), 0o644)
	writeTestResultFile(t, filepath.Join(root, "metasystem", "internal", "steward", "runner_test.go"), []byte(`package steward

import (
	"os"
	"testing"
)

func TestDetachedStewardReadsCandidateEngine(t *testing.T) {
	engine := os.Getenv("METASYSTEM_BIN")
	data, err := os.ReadFile(engine)
	if err != nil || string(data) != "#!/bin/sh\nexit 0\n" {
		t.Fatalf("candidate engine path=%q data=%q err=%v", engine, data, err)
	}
}
`), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	engineData := []byte("#!/bin/sh\nexit 0\n")
	engine := filepath.Join(t.TempDir(), "metasystem")
	writeTestResultFile(t, engine, engineData, 0o500)
	group := testpolicy.Group{ID: "arbitrary-go-group", Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs: []string{"metasystem/go.mod", "metasystem/internal/steward/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"steward-engine"}, Platforms: []string{"any"}, TargetMS: 5000, Packages: []string{"internal/steward"}, Tests: []byte(`"all"`)}
	result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: tree,
		CandidateEngine: engine, CandidateEngineDigest: digestBytes(engineData), Environment: append(gittree.ScrubbedEnviron(), "GOFLAGS=-buildvcs=false"), LogRoot: filepath.Join(root, "logs")}, group)
	if result.Status != "passed" || !result.CollectionComplete || result.ExecutionIdentity == "" {
		t.Fatalf("detached steward result=%+v", result)
	}
}

func TestMetaSystemStewardEngineIdentityIsScopedToItsSelectedPackage(t *testing.T) {
	for _, specimen := range []struct {
		name, installation, cwd, module, adapter string
		packages                                 []string
		consumes                                 bool
	}{
		{"nested engine", "metasystem", "metasystem", "github.com/widoriezebos/agentic-tools/metasystem", "go", []string{"internal/gaterun", "internal/steward"}, true},
		{"root engine", "", ".", "github.com/widoriezebos/agentic-tools/metasystem", "go", []string{"./internal/steward"}, true},
		{"vendored engine", "vendor/agent-runtime", "vendor/agent-runtime", "github.com/widoriezebos/agentic-tools/metasystem", "go", []string{"internal/steward"}, true},
		{"foreign module with same layout", "metasystem", "metasystem", "example.test/application", "go", []string{"internal/steward"}, false},
		{"unrelated package", "metasystem", "metasystem", "github.com/widoriezebos/agentic-tools/metasystem", "go", []string{"internal/gaterun"}, false},
		{"other working directory", "metasystem", "other", "github.com/widoriezebos/agentic-tools/metasystem", "go", []string{"internal/steward"}, false},
		{"other adapter", "metasystem", "metasystem", "github.com/widoriezebos/agentic-tools/metasystem", "command", []string{"internal/steward"}, false},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			cwd := filepath.Join(t.TempDir(), specimen.cwd)
			writeTestResultFile(t, filepath.Join(cwd, "go.mod"), []byte("module "+specimen.module+"\n"), 0o644)
			group := testpolicy.Group{ID: "arbitrary-group", Adapter: specimen.adapter, CWD: specimen.cwd, Packages: specimen.packages}
			request := TestRunRequest{InstallationPrefix: specimen.installation, CandidateEngineDigest: strings.Repeat("a", 64)}
			first := groupExecutionIdentity(request, group, cwd, "inputs", "environment", nil, nil)
			request.CandidateEngineDigest = strings.Repeat("b", 64)
			second := groupExecutionIdentity(request, group, cwd, "inputs", "environment", nil, nil)
			if (first != second) != specimen.consumes {
				t.Fatalf("engine identity dependency=%v, want %v", first != second, specimen.consumes)
			}
		})
	}
}

func TestStageCollectsIndependentFailuresAndNativePrerequisiteResults(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultScript(t, root, "first", "failed", 23)
	writeTestResultScript(t, root, "second", "failed", 24)
	writeTestResultScript(t, root, "later", "passed", 0)
	writeTestResultScript(t, root, "deep", "failed", 25)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	groups := []testpolicy.Group{}
	for _, fixture := range []struct {
		id     string
		status int
	}{{"first", 23}, {"second", 24}, {"later", 0}, {"deep", 25}} {
		groups = append(groups, testpolicy.Group{ID: fixture.id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-" + fixture.id}, Tools: []testpolicy.Tool{}, Obligations: []string{fixture.id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"bash", "scripts/" + fixture.id + ".sh"}, Reports: []string{"reports-" + fixture.id}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + fixture.id + "/tests.xml", Classname: "fixture", Name: fixture.id}}})
	}
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"scripts/**"}, Standard: []string{"first", "second", "later"}, Deep: []string{"deep"}, Critical: []string{"first"}}}, Groups: groups,
		Always: testpolicy.Always{Canary: []string{"first", "second", "later"}}, Unknown: []string{"first"}, Cadence: []string{"deep"}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeCadence, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeDeep, ExecutedMode: testpolicy.ModeDeep,
		RequiredGroups: []string{"deep", "first", "later", "second"}, SelectedGroups: []string{"deep", "first", "later", "second"}, Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"first", "second", "later"}}, {ID: "deep", Groups: []string{"deep"}}}}
	progress := filepath.Join(root, "artifacts", "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{filepath.Join(root, "artifacts", "launcher.log")}}); err != nil {
		t.Fatal(err)
	}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress})
	if err != nil || status != 23 || result.LaunchCounts.Test != 4 || result.Delivery.Sufficient {
		t.Fatalf("stage result status=%d counts=%+v delivery=%+v err=%v", status, result.LaunchCounts, result.Delivery, err)
	}
	byID := map[string]GroupResult{}
	for _, group := range result.Groups {
		byID[group.ID] = group
	}
	if byID["first"].Status != "failed" || byID["second"].Status != "failed" || byID["later"].Status != "passed" || byID["deep"].Status != "failed" || !byID["deep"].CollectionComplete {
		t.Fatalf("independent cross-stage collection results=%+v", result.Groups)
	}
	for id, group := range byID {
		if group.ProgressRule != "cpu-budget/none+zero-window/30m" || group.LongestSilentSeconds < 0 || group.LongestZeroCPUSeconds < 0 {
			t.Fatalf("group %s omitted progress supervision record: %+v", id, group)
		}
		logged, readErr := os.ReadFile(group.LogPath)
		if readErr != nil || digestBytes(logged) != group.LogDigest {
			t.Fatalf("group %s tee log differs from parsed buffer: err=%v", id, readErr)
		}
	}
	run, err := ReadLatestProgressRun(progress)
	if err != nil {
		t.Fatal(err)
	}
	if err := AssertSectionProgress(run, "testing", []string{"first", "second", "later", "deep"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestSchemaTwoGroupPreservesNestedSuiteFailureEvidenceBeforeCandidateCleanup(t *testing.T) {
	for _, specimen := range []struct {
		name string
		exit int
		want string
	}{{name: "successful enclosing group", exit: 0, want: "passed"}, {name: "failed enclosing group", exit: 7, want: "failed"}} {
		t.Run(specimen.name, func(t *testing.T) {
			root := t.TempDir()
			runTestResultGit(t, root, "init", "-q", "-b", "main")
			runTestResultGit(t, root, "config", "user.name", "fixture")
			runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
			writeTestResultFile(t, filepath.Join(root, ".gitignore"), []byte("artifacts/\n"), 0o644)
			writeTestResultFile(t, filepath.Join(root, "source.txt"), []byte("source\n"), 0o644)
			runTestResultGit(t, root, "add", ".")
			runTestResultGit(t, root, "commit", "-qm", "fixture")
			tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
			script := fmt.Sprintf("set -eu; d=artifacts/agents/suite-failures/nested; mkdir -p \"$d\"; printf 'marker-exact\\n' >\"$d/marker.txt\"; printf 'pid-exact\\n' >\"$d/pid\"; printf 'birth-exact\\n' >\"$d/birth\"; pwd >\"$d/candidate-root\"; exit %d", specimen.exit)
			group := testpolicy.Group{ID: "detached-evidence", Kind: "static", Adapter: "command", CWD: ".",
				Inputs: []string{"source.txt"}, Outputs: []string{"artifacts/agents/suite-failures"}, Platforms: []string{"any"}, TargetMS: 1000,
				Argv: []string{"bash", "-c", script}, Format: "exit-status"}
			result := runTestGroup(context.Background(), TestRunRequest{ControlRoot: root, ProjectRoot: root, CandidateTree: tree,
				LogRoot: filepath.Join(root, "artifacts", "test-logs"), EvidenceTimeoutMS: 5000, EvidenceMaxBytes: 1024 * 1024}, group)
			if result.Status != specimen.want {
				t.Fatalf("group status=%s reason=%s", result.Status, result.NotRunReason)
			}
			preserved := map[string]string{}
			err := filepath.WalkDir(filepath.Join(root, "artifacts", "agents", "suite-failures"), func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.Type().IsRegular() {
					data, readErr := os.ReadFile(path)
					if readErr != nil {
						return readErr
					}
					preserved[entry.Name()] = string(data)
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			for name, want := range map[string]string{"marker.txt": "marker-exact\n", "pid": "pid-exact\n", "birth": "birth-exact\n"} {
				if preserved[name] != want {
					t.Fatalf("preserved %s=%q, want %q", name, preserved[name], want)
				}
			}
			candidate := strings.TrimSpace(preserved["candidate-root"])
			if candidate == "" {
				t.Fatal("preserved evidence did not identify its real detached candidate")
			}
			if _, err := os.Stat(candidate); !os.IsNotExist(err) {
				t.Fatalf("real detached candidate still exists after preservation: %s (%v)", candidate, err)
			}
		})
	}
}

func TestCommandApplicationRunsWithoutGoAndSelectedGoRefuses(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultFile(t, filepath.Join(root, "source.txt"), []byte("source\n"), 0o644)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	sh, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	pathOnly := filepath.Join(root, "path")
	if err := os.Mkdir(pathOnly, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sh, filepath.Join(pathOnly, "sh")); err != nil {
		t.Fatal(err)
	}
	commandGroup := testpolicy.Group{ID: "command", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"}, Outputs: []string{"reports"},
		Tools: []testpolicy.Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"-c", "printf shell-v1"}}}, Obligations: []string{"output"}, Platforms: []string{"any"}, TargetMS: 1000,
		Argv:    []string{"sh", "-c", `printf '%s\n' '<testsuite><testcase classname="app" name="smoke"/></testsuite>' > reports/result.xml`},
		Reports: []string{"reports"}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports/result.xml", Classname: "app", Name: "smoke"}}}
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"source.txt"}, Standard: []string{"command"}, Critical: []string{"output"}}}, Groups: []testpolicy.Group{commandGroup},
		Always: testpolicy.Always{Canary: []string{"command"}}, Unknown: []string{"command"}, Cadence: []string{"command"}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"command"}, SelectedGroups: []string{"command"}, Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"command"}}}}
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "base", PolicyBaseCommit: "base", Contract: contract, Plan: plan,
		AttemptID: "attempt", Environment: []string{"PATH=" + pathOnly, "HOME=" + root}, LogRoot: filepath.Join(root, "artifacts", "command-logs")}
	result, status, err := RunTestPlan(context.Background(), request)
	if err != nil || status != 0 || !result.Delivery.Sufficient || result.LaunchCounts.Test != 1 || result.LaunchCounts.Other != 1 || result.Groups[0].Status != "passed" {
		t.Fatalf("command application result=%+v status=%d err=%v", result, status, err)
	}

	goGroup := commandGroup
	goGroup.ID, goGroup.Adapter, goGroup.Argv, goGroup.Reports, goGroup.ExpectedTests, goGroup.Format = "go", "go", nil, nil, nil, ""
	goGroup.Tools = []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}}
	goGroup.Packages, goGroup.Tests = []string{"."}, []byte(`"all"`)
	request.Contract.Groups = []testpolicy.Group{goGroup}
	request.Plan.RequiredGroups, request.Plan.SelectedGroups = []string{"go"}, []string{"go"}
	request.Plan.Stages = []testpolicy.Stage{{ID: "canary", Groups: []string{"go"}}}
	identities, identityErr := GroupExecutionIdentities(context.Background(), request)
	if identityErr != nil || identities["go"] == "" {
		t.Fatalf("missing Go tool prevented component admission: identities=%v err=%v", identities, identityErr)
	}
	_, request.PreparedGroups, request.PreparationLaunches, identityErr = PrepareGroupExecutionIdentities(context.Background(), request)
	if identityErr != nil || request.PreparedGroups["go"].Unavailable == "" {
		t.Fatalf("missing Go was not retained during preparation: groups=%+v err=%v", request.PreparedGroups, identityErr)
	}
	result, status, err = RunTestPlan(context.Background(), request)
	if err != nil || status == 0 || result.Groups[0].NotRunReason != request.PreparedGroups["go"].Unavailable ||
		!strings.Contains(result.Groups[0].NotRunReason, "go") || strings.Contains(result.Groups[0].NotRunReason, "empty command") || result.Groups[0].Status != "unavailable" || result.Groups[0].NativeLaunched ||
		result.LaunchCounts.Test != 0 || result.Groups[0].ExecutionIdentity != identities["go"] {
		t.Fatalf("selected Go group did not honestly refuse a no-Go environment: result=%+v status=%d err=%v", result, status, err)
	}
}

func TestSectionMismatchKeepsNativeStatusAndLaterIndependentResult(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	script := `#!/usr/bin/env bash
set -u
[[ -x "$PWD/bin/metasystem" && -z "${METASYSTEM_BIN:-}" ]] || exit 26
section=${2:-}
case "$section" in
  mismatch)
    printf 'section\tmismatch\tfail\t24\treported status\n' >"$METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT"
    exit 23
    ;;
  later)
    printf 'section\tlater\tpass\t0\n' >"$METASYSTEM_ENUMERATION_STAGE_RESULTS_OUT"
    exit 0
    ;;
  *) exit 2 ;;
esac
`
	writeTestResultFile(t, filepath.Join(root, "scripts", "agents", "validate-section-selector.sh"), []byte(script), 0o755)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "section fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	groups := []testpolicy.Group{
		{ID: "mismatch", Kind: "integration", Adapter: "section", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-a"}, Obligations: []string{"mismatch"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "mismatch"},
		{ID: "later", Kind: "integration", Adapter: "section", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-b"}, Obligations: []string{"later"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "later"},
	}
	contract := testpolicy.Contract{SchemaVersion: 1, Groups: groups}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeCadence, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"later", "mismatch"}, SelectedGroups: []string{"later", "mismatch"},
		Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"mismatch", "later"}}}}
	engineData := []byte("#!/usr/bin/env bash\nexit 0\n")
	engine := filepath.Join(t.TempDir(), "metasystem")
	writeTestResultFile(t, engine, engineData, 0o500)
	t.Setenv("METASYSTEM_BIN", filepath.Join(t.TempDir(), "ambient-engine"))
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD",
		PolicyBaseCommit: "HEAD", Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "logs"),
		CandidateEngine: engine, CandidateEngineDigest: digestBytes(engineData)}
	var prepareErr error
	_, request.PreparedGroups, request.PreparationLaunches, prepareErr = PrepareGroupExecutionIdentities(context.Background(), request)
	if prepareErr != nil {
		t.Fatal(prepareErr)
	}
	result, status, err := RunTestPlan(context.Background(), request)
	if err != nil || status != 23 || len(result.Groups) != 2 || result.Groups[0].Status != "invalid" ||
		result.Groups[0].NativeExitStatus == nil || *result.Groups[0].NativeExitStatus != 23 ||
		!strings.Contains(result.Groups[0].NotRunReason, "disagrees") || result.Groups[1].Status != "passed" {
		t.Fatalf("section mismatch result=%+v status=%d err=%v", result, status, err)
	}
}

func TestCadenceCatchClassesRequireTerminalCompleteGroups(t *testing.T) {
	result := TestResult{RequiredGroups: testpolicy.CadenceCatchGroupIDs()}
	for _, id := range result.RequiredGroups {
		result.Groups = append(result.Groups, GroupResult{ID: id, Status: "passed", CollectionComplete: true})
	}
	if err := RequireResultGroups(result, testpolicy.CadenceCatchGroupIDs()); err != nil {
		t.Fatal(err)
	}
	result.Groups[0].CollectionComplete = false
	if err := RequireResultGroups(result, testpolicy.CadenceCatchGroupIDs()); err == nil || !strings.Contains(err.Error(), result.Groups[0].ID) {
		t.Fatalf("incomplete cadence group was accepted: %v", err)
	}
}

func TestReusedTestResultComposesAcrossCandidateChangeAndExactRecoveryUsesExecutionIdentity(t *testing.T) {
	groupIdentity := strings.Repeat("7", 64)
	oldTree := strings.Repeat("b", 40)
	currentTree := strings.Repeat("c", 40)
	startedAt := "2026-09-09T09:00:00Z"
	endedAt := "2026-09-09T09:00:01Z"
	source := componentAttemptResult("original-attempt", "application", groupIdentity, "passed")
	source.CandidateTree = oldTree
	source.StartedAt = startedAt
	source.EndedAt = endedAt
	source.Groups[0].StartedAt = startedAt
	source.Groups[0].EndedAt = endedAt
	source.Groups[0].DurationMS = 1000
	attempt := Attempt{AttemptID: "original-attempt", GoalID: "goal-a", AccountingRevision: 2, StartedAt: startedAt,
		Terminal: &AttemptTerminal{Result: TerminalSuccess, ExitStatus: 0, At: endedAt}, TestResult: &source,
		PendingTestGroups:    map[string]string{"application": groupIdentity},
		DeliveryReceiptBytes: []byte(`{"schemaVersion":2,"marker":"original"}`)}
	template := source
	template.AttemptID = ""
	template.CandidateTree = currentTree
	template.BaseCommit = "current-base"
	template.PolicyBaseCommit = "current-policy-base"
	template.PlanDigest = strings.Repeat("9", 64)
	template.Groups = nil
	contract := testpolicy.Contract{Groups: []testpolicy.Group{{ID: "application", Kind: "unit", Inputs: []string{"source"}}}}

	composed := ReusedTestResult(template, []Attempt{attempt}, map[string]string{"application": groupIdentity}, contract, "goal-a", 2)
	if !composed.Delivery.Sufficient || composed.CandidateTree != currentTree || composed.AttemptID != "" || len(composed.Groups) != 1 ||
		composed.Groups[0].Status != "reused" || composed.Groups[0].ReuseAttempt != attempt.AttemptID ||
		composed.Groups[0].StartedAt != startedAt || composed.Groups[0].EndedAt != endedAt || composed.Groups[0].DurationMS != 1000 {
		t.Fatalf("current-candidate component projection lost retained evidence: %+v", composed)
	}
	if exact, ok := ExactReusableTestResult(template, []Attempt{attempt}, map[string]string{"application": groupIdentity}, "goal-a", 2); !ok || exact.AttemptID != attempt.AttemptID || exact.CandidateTree != oldTree {
		t.Fatalf("identity-equivalent candidate did not recover the original exact result: ok=%v result=%+v", ok, exact)
	}
	if exact, ok := ExactReusableTestResult(template, []Attempt{attempt}, map[string]string{"application": strings.Repeat("8", 64)}, "goal-a", 2); ok {
		t.Fatalf("different group execution identity recovered an old exact result: %+v", exact)
	}
	exactTemplate := source
	exactTemplate.AttemptID = ""
	if exact, ok := ExactReusableTestResult(exactTemplate, []Attempt{attempt}, map[string]string{"application": groupIdentity}, "goal-a", 2); !ok || exact.AttemptID != attempt.AttemptID || exact.EndedAt != endedAt {
		t.Fatalf("same-candidate exact recovery lost the original result: ok=%v result=%+v", ok, exact)
	}
	differentEngine := exactTemplate
	differentEngine.CandidateEngineDigest = strings.Repeat("e", 64)
	if exact, ok := ExactReusableTestResult(differentEngine, []Attempt{attempt}, map[string]string{"application": groupIdentity}, "goal-a", 2); ok {
		t.Fatalf("different candidate engine recovered an old exact result: %+v", exact)
	}

	for _, mismatch := range []struct {
		name   string
		change func(*TestResult)
	}{
		{name: "candidate contract", change: func(result *TestResult) { result.ContractDigest = strings.Repeat("d", 64) }},
		{name: "base contract", change: func(result *TestResult) { result.BaseContractDigest = strings.Repeat("d", 64) }},
		{name: "policy engine", change: func(result *TestResult) { result.PolicyEngineDigest = strings.Repeat("d", 64) }},
		{name: "behavior policy", change: func(result *TestResult) { result.BehaviorPolicyDigest = strings.Repeat("d", 64) }},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			changed := template
			mismatch.change(&changed)
			projection := ReusedTestResult(changed, []Attempt{attempt}, map[string]string{"application": groupIdentity}, contract, "goal-a", 2)
			if projection.Delivery.Sufficient || len(projection.Groups) != 1 || projection.Groups[0].Status != "not-run" {
				t.Fatalf("mismatched policy binding reused evidence: %+v", projection)
			}
		})
	}
}

func TestRunTestPlanRefusesForgedComponentReuseWithoutTerminalOuterOwner(t *testing.T) {
	zero := 0
	group := testpolicy.Group{ID: "guard", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt"},
		Obligations: []string{"guard"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"true"}, Format: "exit-status"}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
		ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"guard"}, SelectedGroups: []string{"guard"},
		Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"guard"}}}}
	forged := GroupResult{ID: "guard", Kind: "unit", Obligations: []string{"guard"}, InputDigest: strings.Repeat("a", 64),
		InputManifest: []string{"source.txt"}, ExecutionIdentity: strings.Repeat("b", 64), CWD: ".", Status: "passed",
		NativeLaunched: true, NativeExitStatus: &zero, CollectionComplete: true, ReuseAttempt: "invented-success",
		ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ControlRoot: t.TempDir(), ProjectRoot: t.TempDir(),
		CandidateTree: strings.Repeat("c", 40), BaseCommit: "base", PolicyBaseCommit: "base", Contract: testpolicy.Contract{SchemaVersion: 1, Groups: []testpolicy.Group{group}},
		Plan: plan, AttemptID: "live", Reused: map[string]GroupResult{"guard": forged}})
	if err != nil || status == 0 || result.Delivery.Sufficient || len(result.Groups) != 1 || result.Groups[0].Status != "invalid" ||
		!strings.Contains(result.Groups[0].NotRunReason, "outer attempt") {
		t.Fatalf("forged reuse result=%+v status=%d err=%v", result, status, err)
	}
}

func writeTestResultFile(t *testing.T, path string, data []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, mode); err != nil {
		t.Fatal(err)
	}
}

func writeTestResultScript(t *testing.T, root, name, status string, exit int) {
	t.Helper()
	body := "<testsuite><testcase classname=\"fixture\" name=\"" + name + "\">"
	if status == "failed" {
		body += "<failure message=\"red\"/>"
	}
	body += "</testcase></testsuite>"
	script := "#!/usr/bin/env bash\nset -euo pipefail\nmkdir -p reports-" + name + "\nprintf '%s\\n' " + strconv.Quote(body) + " > reports-" + name + "/tests.xml\nexit " + strconv.Itoa(exit) + "\n"
	path := filepath.Join(root, "scripts", name+".sh")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runTestResultGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	data, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, data)
	}
	return strings.TrimSpace(string(data))
}
