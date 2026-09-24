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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func TestNewTestResultKeepsDigestOnlyIdentityForLegacyPolicyProbe(t *testing.T) {
	t.Parallel()
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

func TestRunTestPlanFinalizesOperationalErrorResult(t *testing.T) {
	t.Parallel()
	for _, schema := range []int{testpolicy.SchemaVersion, testpolicy.ExecutionContractSchemaVersion} {
		t.Run(fmt.Sprintf("schema-%d", schema), func(t *testing.T) {
			root := t.TempDir()
			progressDirectory := filepath.Join(root, "artifacts", "progress")
			progressPath := filepath.Join(progressDirectory, "events.jsonl")
			greenProgressDirectory := filepath.Join(root, "artifacts", "green-progress")
			greenProgressPath := filepath.Join(greenProgressDirectory, "events.jsonl")
			if err := os.MkdirAll(progressDirectory, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(greenProgressDirectory, 0o755); err != nil {
				t.Fatal(err)
			}
			redScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
mkdir -p reports-red
printf '%%s\n' '<testsuite><testcase classname="fixture" name="red"><failure message="red"/></testcase></testsuite>' > reports-red/tests.xml
printf 'RAW-FAILED-OUTPUT\n' >&2
mv %s %s
printf 'closed\n' > %s
exit 23
`, strconv.Quote(progressDirectory), strconv.Quote(progressDirectory+"-closed"), strconv.Quote(progressDirectory))
			greenScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
mkdir -p reports-green
printf '%%s\n' '<testsuite><testcase classname="fixture" name="green"/></testsuite>' > reports-green/tests.xml
printf 'RAW-PASSED-OUTPUT\n' >&2
mv %s %s
printf 'closed\n' > %s
`, strconv.Quote(greenProgressDirectory), strconv.Quote(greenProgressDirectory+"-closed"), strconv.Quote(greenProgressDirectory))
			tree := strings.Repeat("3", 40)
			snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
				"source.txt":       testSnapshotFile("source\n", 0o644),
				"scripts/red.sh":   testSnapshotFile(redScript, 0o755),
				"scripts/green.sh": testSnapshotFile(greenScript, 0o755),
				"scripts/later.sh": testSnapshotScript("later", "passed", 0),
			}, 4)

			group := func(id string) testpolicy.Group {
				return testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"source.txt", "scripts/" + id + ".sh"},
					Outputs: []string{"reports-" + id}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 1000,
					Argv: []string{"bash", "scripts/" + id + ".sh"}, Reports: []string{"reports-" + id}, Format: "junit-xml",
					ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}}
			}
			groups := []testpolicy.Group{group("red"), group("later"), group("green")}
			if schema == testpolicy.ExecutionContractSchemaVersion {
				for index := range groups {
					groups[index].Phase, groups[index].EnvironmentMode = "acceptance", "inherit"
				}
			}
			contract := testpolicy.Contract{SchemaVersion: schema,
				ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
				Surfaces:    []testpolicy.Surface{{ID: "app", Paths: []string{"source.txt", "scripts/**"}, Standard: []string{"red", "later", "green"}}},
				Groups:      groups, Always: testpolicy.Always{Canary: []string{"red"}, Standard: []string{"later", "green"}}, Unknown: []string{"red"}}
			if err := contract.Validate(); err != nil {
				t.Fatal(err)
			}
			plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard,
				ExecutedMode: testpolicy.ModeStandard, RequiredGroups: []string{"red", "later"}, SelectedGroups: []string{"red", "later"},
				Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"red"}}, {ID: "standard", Groups: []string{"later"}}}}
			request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", Contract: contract, Plan: plan,
				AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "logs"), ProgressPath: progressPath,
				CandidateEngineDigest: strings.Repeat("a", 64), Concurrency: 2}
			if schema == testpolicy.ExecutionContractSchemaVersion {
				request.Workers, request.AdmissionMaximum = 2, 0
			}
			identities, prepared, launches, err := PrepareGroupExecutionIdentities(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			request.ComponentIdentities, request.PreparedGroups, request.PreparationLaunches = identities, prepared, launches
			result, status, runErr := RunTestPlan(context.Background(), request)
			if runErr == nil || !strings.Contains(runErr.Error(), "record testing group red end") || status == 0 {
				t.Fatalf("operational error was not retained: status=%d err=%v", status, runErr)
			}
			if err := ValidateTestResult(result); err != nil {
				t.Fatalf("operational result is not valid: %v\n%+v", err, result)
			}
			if result.Delivery.Sufficient || result.LaunchCounts.Test != 1 || len(result.Groups) != 2 || result.Groups[0].Status != "failed" ||
				result.Groups[1].Status != "not-run" || result.Groups[1].NativeLaunched || result.Groups[1].ExecutionIdentity != identities["later"] {
				t.Fatalf("operational result lost evidence or invented coverage: counts=%+v delivery=%+v groups=%+v", result.LaunchCounts, result.Delivery, result.Groups)
			}
			if len(result.Groups[0].Observed) != 1 || result.Groups[0].Observed[0].Name != "red" || result.Groups[0].Observed[0].Status != "failed" {
				t.Fatalf("native JUnit failure join was lost: %+v", result.Groups[0].Observed)
			}
			logged, err := os.ReadFile(result.Groups[0].LogPath)
			if err != nil || !strings.Contains(string(logged), "RAW-FAILED-OUTPUT") {
				t.Fatalf("raw failed output was lost: err=%v output=%q", err, logged)
			}

			greenPlan := plan
			greenPlan.RequiredGroups, greenPlan.SelectedGroups = []string{"green"}, []string{"green"}
			greenPlan.Stages = []testpolicy.Stage{{ID: "canary", Groups: []string{"green"}}}
			greenRequest := request
			greenRequest.Plan, greenRequest.ProgressPath = greenPlan, greenProgressPath
			greenIdentities, greenPrepared, greenLaunches, err := PrepareGroupExecutionIdentities(context.Background(), greenRequest)
			if err != nil {
				t.Fatal(err)
			}
			greenRequest.ComponentIdentities, greenRequest.PreparedGroups, greenRequest.PreparationLaunches = greenIdentities, greenPrepared, greenLaunches
			greenResult, greenStatus, greenErr := RunTestPlan(context.Background(), greenRequest)
			if greenErr == nil || !strings.Contains(greenErr.Error(), "record testing group green end") || greenStatus == 0 {
				t.Fatalf("all-pass operational error was not retained: status=%d err=%v", greenStatus, greenErr)
			}
			if err := ValidateTestResult(greenResult); err != nil {
				t.Fatalf("all-pass operational result is not valid: %v\n%+v", err, greenResult)
			}
			if greenResult.Delivery.Sufficient || len(greenResult.Delivery.FailingGroups) != 0 || len(greenResult.Delivery.MissingGroups) != 0 ||
				len(greenResult.Uncertainty) != 1 || len(greenResult.Delivery.Discrepancies) != 1 || len(greenResult.Groups) != 1 ||
				greenResult.Groups[0].Status != "passed" || !greenResult.Groups[0].NativeLaunched || greenResult.Groups[0].ExecutionIdentity != greenIdentities["green"] ||
				len(greenResult.Groups[0].Observed) != 1 || greenResult.Groups[0].Observed[0].Status != "passed" || greenResult.LaunchCounts.Test != 1 {
				t.Fatalf("all-pass operational result lost evidence or invented failure: delivery=%+v uncertainty=%v counts=%+v groups=%+v",
					greenResult.Delivery, greenResult.Uncertainty, greenResult.LaunchCounts, greenResult.Groups)
			}
		})
	}
}

func TestSectionEnginePreparationPreservesBytesAndNestedSelector(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	cwd := filepath.Join(root, "vendor", "engine with spaces")
	if err := os.MkdirAll(cwd, 0o700); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "engine")
	data := []byte("#!/bin/sh\nexit 0\n")
	if err := testexec.WriteFile(source, data, 0o500); err != nil {
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
	stewardTest := `package steward

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
`
	tree := verdictFixtureTree(t, "steward")
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"metasystem/go.mod":                          testSnapshotFile("module github.com/widoriezebos/agentic-tools/metasystem\n\ngo 1.22\n", 0o644),
		"metasystem/internal/steward/runner_test.go": testSnapshotFile(stewardTest, 0o644),
	}, 1)
	engineData := []byte("#!/bin/sh\nexit 0\n")
	engine := filepath.Join(t.TempDir(), "metasystem")
	writeTestResultFile(t, engine, engineData, 0o500)
	group := testpolicy.Group{ID: "arbitrary-go-group", Kind: "unit", Adapter: "go", CWD: "metasystem",
		Inputs: []string{"metasystem/go.mod", "metasystem/internal/steward/**"}, Tools: []testpolicy.Tool{{ID: "go", Executable: "go", VersionArgs: []string{"version"}}},
		Obligations: []string{"steward-engine"}, Platforms: []string{"any"}, TargetMS: 5000, Packages: []string{"internal/steward"}, Tests: []byte(`"all"`)}
	result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, InstallationPrefix: "metasystem", CandidateTree: tree, openCandidate: snapshot.open,
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
	tree := strings.Repeat("4", 40)
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"scripts/first.sh":  testSnapshotScript("first", "failed", 23),
		"scripts/second.sh": testSnapshotScript("second", "failed", 24),
		"scripts/later.sh":  testSnapshotScript("later", "passed", 0),
		"scripts/deep.sh":   testSnapshotScript("deep", "failed", 25),
	}, 4)
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
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD", Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress})
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
			tree := strings.Repeat("5", 40)
			snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
				".gitignore": testSnapshotFile("artifacts/\n", 0o644),
				"source.txt": testSnapshotFile("source\n", 0o644),
			}, 1)
			script := fmt.Sprintf("set -eu; d=artifacts/agents/suite-failures/nested; mkdir -p \"$d\"; printf 'marker-exact\\n' >\"$d/marker.txt\"; printf 'pid-exact\\n' >\"$d/pid\"; printf 'birth-exact\\n' >\"$d/birth\"; pwd >\"$d/candidate-root\"; exit %d", specimen.exit)
			group := testpolicy.Group{ID: "detached-evidence", Kind: "static", Adapter: "command", CWD: ".",
				Inputs: []string{"source.txt"}, Outputs: []string{"artifacts/agents/suite-failures"}, Platforms: []string{"any"}, TargetMS: 1000,
				Argv: []string{"bash", "-c", script}, Format: "exit-status"}
			result := runTestGroup(context.Background(), TestRunRequest{ControlRoot: root, ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open,
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

func TestExitStatusUnitCommandRecordsNativeResultWithoutTestcaseCensus(t *testing.T) {
	root := t.TempDir()
	tree := strings.Repeat("6", 40)
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"source.txt": testSnapshotFile("source\n", 0o644),
	}, 2)

	for _, specimen := range []struct {
		name, status string
		exit         int
		complete     bool
	}{
		{name: "success", status: "passed", exit: 0, complete: true},
		{name: "failure", status: "failed", exit: 23, complete: false},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			output := "native-" + specimen.name + "\n"
			logOutput := output
			if specimen.exit != 0 {
				logOutput += "TEST-VERDICT application-tests status=failed exit=23 reason=process exit 23 with no failing test in the evidence (collection incomplete)\n"
			}
			group := testpolicy.Group{ID: "application-tests", Kind: "unit", Adapter: "command", CWD: ".",
				Phase: "acceptance", EnvironmentMode: "inherit", Inputs: []string{"source.txt"}, Platforms: []string{"any"}, TargetMS: 1000,
				Argv: []string{"sh", "-c", fmt.Sprintf("printf %s; exit %d", strconv.Quote(output), specimen.exit)}, Format: "exit-status"}
			result := runTestGroup(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open,
				LogRoot: filepath.Join(root, "logs", specimen.name)}, group)
			logged, err := os.ReadFile(result.LogPath)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != specimen.status || !result.NativeLaunched || result.NativeExitStatus == nil || *result.NativeExitStatus != specimen.exit ||
				result.CollectionComplete != specimen.complete || string(logged) != logOutput || result.LogDigest != digestBytes(logged) {
				t.Fatalf("native command result=%+v log=%q", result, logged)
			}
			if len(result.Expected) != 0 || len(result.Observed) != 0 || len(result.Missing) != 0 || len(result.Unexpected) != 0 {
				t.Fatalf("exit-status command fabricated a testcase census: %+v", result)
			}
		})
	}
}

func TestCommandApplicationRunsWithoutGoAndSelectedGoRefuses(t *testing.T) {
	root := t.TempDir()
	tree := verdictFixtureTree(t, "command")
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"source.txt": testSnapshotFile("source\n", 0o644),
	}, 4)
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
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open, BaseCommit: "base", PolicyBaseCommit: "base", Contract: contract, Plan: plan,
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
	tree := verdictFixtureTree(t, "section")
	snapshot := newTestSnapshotFactory(t, root, tree, map[string]testSnapshotEntry{
		"scripts/agents/validate-section-selector.sh": testSnapshotFile(script, 0o755),
	}, 3)
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
	request := TestRunRequest{ProjectRoot: root, CandidateTree: tree, openCandidate: snapshot.open, BaseCommit: "HEAD",
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
	t.Parallel()
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
	t.Parallel()
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

	composed := ReusedTestResult(template, []Attempt{attempt}, map[string]string{"application": groupIdentity}, contract)
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
		{name: "judge key", change: func(result *TestResult) { result.JudgeKey = "judge/v1:changed" }},
		{name: "behavior policy", change: func(result *TestResult) { result.BehaviorPolicyDigest = strings.Repeat("d", 64) }},
	} {
		t.Run(mismatch.name, func(t *testing.T) {
			changed := template
			mismatch.change(&changed)
			projection := ReusedTestResult(changed, []Attempt{attempt}, map[string]string{"application": groupIdentity}, contract)
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
	if err := testexec.WriteFile(path, data, mode); err != nil {
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
	if err := testexec.WriteFile(path, []byte(script), 0o755); err != nil {
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
