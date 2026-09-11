package proofrun

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// Groups of one stage run side by side under the request's concurrency cap:
// the stage's wall time is its longest group, the results keep plan order,
// and the progress file still records one start and one end per group.
func TestStageRunsIndependentGroupsSideBySide(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	ids := []string{"alpha", "beta", "gamma"}
	groups := []testpolicy.Group{}
	for _, id := range ids {
		body := "<testsuite><testcase classname=\"fixture\" name=\"" + id + "\"></testcase></testsuite>"
		script := "#!/usr/bin/env bash\nset -euo pipefail\nsleep 2\nmkdir -p reports-" + id + "\nprintf '%s\\n' " + strconv.Quote(body) + " > reports-" + id + "/tests.xml\n"
		path := filepath.Join(root, "scripts", id+".sh")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
		groups = append(groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-" + id},
			Tools: []testpolicy.Tool{}, Obligations: []string{id + "-obligation"}, Platforms: []string{"any"}, TargetMS: 10000,
			Argv: []string{"bash", "scripts/" + id + ".sh"}, Reports: []string{"reports-" + id}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}})
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"scripts/**"}, Standard: ids, Critical: []string{"alpha-obligation"}}}, Groups: groups,
		Always: testpolicy.Always{Canary: []string{"alpha"}}, Unknown: []string{"alpha"}, Cadence: ids}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: ids, SelectedGroups: ids, Stages: []testpolicy.Stage{{ID: "standard", Groups: ids}}}
	progress := filepath.Join(root, "artifacts", "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{filepath.Join(root, "artifacts", "launcher.log")}}); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress, Concurrency: 3})
	wall := time.Since(started)
	if err != nil || status != 0 {
		t.Fatalf("concurrent stage: status=%d err=%v result=%+v", status, err, result.Groups)
	}
	if wall >= 5*time.Second {
		t.Fatalf("three two-second groups under a cap of three took %s; they did not run side by side", wall)
	}
	if len(result.Groups) != 3 {
		t.Fatalf("expected three results, got %+v", result.Groups)
	}
	for index, id := range ids {
		if result.Groups[index].ID != id || result.Groups[index].Status != "passed" {
			t.Fatalf("results left plan order or failed: %+v", result.Groups)
		}
	}
	if result.LaunchCounts.Test != 3 || result.ChildDurationMS < 6000 {
		t.Fatalf("launch counts or child durations lost under the pool: counts=%+v child=%d", result.LaunchCounts, result.ChildDurationMS)
	}
	data, err := os.ReadFile(progress)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		starts := strings.Count(string(data), `"section":"`+id+`","event":"start"`)
		ends := strings.Count(string(data), `"section":"`+id+`","event":"end"`)
		if starts != 1 || ends != 1 {
			t.Fatalf("progress for %s has %d starts and %d ends:\n%s", id, starts, ends, data)
		}
	}
}

// A cap of one keeps the old serial behaviour exactly.
func TestStageWithCapOfOneRunsSerially(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultScript(t, root, "one", "passed", 0)
	writeTestResultScript(t, root, "two", "passed", 0)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	groups := []testpolicy.Group{}
	for _, id := range []string{"one", "two"} {
		groups = append(groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-" + id},
			Tools: []testpolicy.Tool{}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 10000,
			Argv: []string{"bash", "scripts/" + id + ".sh"}, Reports: []string{"reports-" + id}, Format: "junit-xml",
			ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}})
	}
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"scripts/**"}, Standard: []string{"one", "two"}, Critical: []string{"one"}}}, Groups: groups,
		Always: testpolicy.Always{Canary: []string{"one"}}, Unknown: []string{"one"}, Cadence: []string{"one", "two"}}
	plan := testpolicy.Plan{Purpose: testpolicy.PurposeDelivery, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeStandard, ExecutedMode: testpolicy.ModeStandard,
		RequiredGroups: []string{"one", "two"}, SelectedGroups: []string{"one", "two"}, Stages: []testpolicy.Stage{{ID: "standard", Groups: []string{"one", "two"}}}}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), Concurrency: 1})
	if err != nil || status != 0 || len(result.Groups) != 2 || result.Groups[0].ID != "one" || result.Groups[1].ID != "two" {
		t.Fatalf("serial stage: status=%d err=%v result=%+v", status, err, result.Groups)
	}
}
