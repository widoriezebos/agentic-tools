package proofrun

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// stopFixture runs a four-group plan (canary: first, second, third; deep:
// deep) whose second group fails, one group at a time so the plan order is
// the launch order, and returns the result by group.
func stopFixture(t *testing.T, purpose testpolicy.Purpose) (TestResult, int, map[string]GroupResult) {
	t.Helper()
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultScript(t, root, "first", "passed", 0)
	writeTestResultScript(t, root, "second", "failed", 24)
	writeTestResultScript(t, root, "third", "passed", 0)
	writeTestResultScript(t, root, "deep", "passed", 0)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	contract, plan := stopFixtureContract(purpose)
	progress := filepath.Join(root, "artifacts", "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{filepath.Join(root, "artifacts", "launcher.log")}}); err != nil {
		t.Fatal(err)
	}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress, Concurrency: 1})
	if err != nil {
		t.Fatalf("plan run failed: %v", err)
	}
	byID := map[string]GroupResult{}
	for _, group := range result.Groups {
		byID[group.ID] = group
	}
	return result, status, byID
}

func stopFixtureContract(purpose testpolicy.Purpose) (testpolicy.Contract, testpolicy.Plan) {
	groups := []testpolicy.Group{}
	for _, id := range []string{"first", "second", "third", "deep"} {
		groups = append(groups, testpolicy.Group{ID: id, Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"scripts/**"}, Outputs: []string{"reports-" + id}, Tools: []testpolicy.Tool{}, Obligations: []string{id}, Platforms: []string{"any"}, TargetMS: 1000,
			Argv: []string{"bash", "scripts/" + id + ".sh"}, Reports: []string{"reports-" + id}, Format: "junit-xml", ExpectedTests: []testpolicy.ExpectedTest{{Report: "reports-" + id + "/tests.xml", Classname: "fixture", Name: id}}})
	}
	contract := testpolicy.Contract{SchemaVersion: 1, ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{{ID: "app", Paths: []string{"scripts/**"}, Standard: []string{"first", "second", "third"}, Deep: []string{"deep"}, Critical: []string{"first"}}}, Groups: groups,
		Always: testpolicy.Always{Canary: []string{"first", "second", "third"}}, Unknown: []string{"first"}, Cadence: []string{"deep"}}
	plan := testpolicy.Plan{Purpose: purpose, RequestedMode: testpolicy.ModeAuto, RequiredMode: testpolicy.ModeDeep, ExecutedMode: testpolicy.ModeDeep,
		RequiredGroups: []string{"deep", "first", "second", "third"}, SelectedGroups: []string{"deep", "first", "second", "third"},
		Stages: []testpolicy.Stage{{ID: "canary", Groups: []string{"first", "second", "third"}}, {ID: "deep", Groups: []string{"deep"}}}}
	return contract, plan
}

func TestDeliveryAttemptStopsLaunchingAtTheFirstFailedGroup(t *testing.T) {
	result, status, byID := stopFixture(t, testpolicy.PurposeDelivery)
	if status != 24 || result.LaunchCounts.Test != 2 || result.Delivery.Sufficient {
		t.Fatalf("delivery attempt did not stop at its first failure: status=%d counts=%+v delivery=%+v", status, result.LaunchCounts, result.Delivery)
	}
	if byID["first"].Status != "passed" || !byID["first"].CollectionComplete || byID["second"].Status != "failed" || !byID["second"].CollectionComplete {
		t.Fatalf("the groups that ran lost their evidence: %+v", result.Groups)
	}
	for _, id := range []string{"third", "deep"} {
		group := byID[id]
		if group.Status != "not-run" || group.NotRunReason != haltReason("second") || group.NativeLaunched || group.StartedAt != "" {
			t.Fatalf("group %s after the first failure classified as %+v", id, group)
		}
	}
	if strings.Join(result.Delivery.MissingGroups, ",") != "deep,third" || strings.Join(result.Delivery.FailingGroups, ",") != "second" {
		t.Fatalf("delivery judgment does not name the stopped groups: %+v", result.Delivery)
	}
	if len(result.Groups) != 4 {
		t.Fatalf("result does not carry every selected group: %+v", result.Groups)
	}
}

func TestCadenceAndDiagnosticAttemptsKeepContinueAndCollect(t *testing.T) {
	for _, purpose := range []testpolicy.Purpose{testpolicy.PurposeCadence, testpolicy.PurposeDiagnostic} {
		t.Run(string(purpose), func(t *testing.T) {
			result, status, byID := stopFixture(t, purpose)
			if status != 24 || result.LaunchCounts.Test != 4 {
				t.Fatalf("%s attempt stopped early: status=%d counts=%+v", purpose, status, result.LaunchCounts)
			}
			if byID["third"].Status != "passed" || byID["deep"].Status != "passed" || byID["second"].Status != "failed" {
				t.Fatalf("%s attempt did not collect every group: %+v", purpose, result.Groups)
			}
		})
	}
}

func TestDeliveryRetryReusesTheFailedPredecessorsPassedGroups(t *testing.T) {
	controlRoot, baseIdentity := proofAttemptFixture(t, "testing")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	identities := map[string]string{"first": strings.Repeat("1", 64), "second": strings.Repeat("2", 64), "third": strings.Repeat("3", 64)}
	identity := BindIdentityInputs(baseIdentity, []string{"group:first:" + identities["first"], "group:second:" + identities["second"], "group:third:" + identities["third"], "plan:one"})
	request := AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: controlRoot, GoalID: "goal-a", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 2, Identity: identity, Launcher: launcher, Now: now, ComponentIdentities: identities}
	attempt, _, err := ReserveLocked(request)
	if err != nil {
		t.Fatal(err)
	}
	// The predecessor: first passed, second failed, third never launched
	// because the attempt stopped at second.
	failed := componentAttemptResult(attempt.AttemptID, "first", identities["first"], "passed")
	second := componentAttemptResult(attempt.AttemptID, "second", identities["second"], "failed").Groups[0]
	third := GroupResult{ID: "third", Kind: "unit", InputManifest: []string{"source"}, ExecutionIdentity: identities["third"],
		Status: "not-run", NotRunReason: haltReason("second"), ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
	failed.Groups = append(failed.Groups, second, third)
	failed.SelectedGroups, failed.RequiredGroups = []string{"first", "second", "third"}, []string{"first", "second", "third"}
	failed.RecomputeDelivery()
	if _, err := FinalizeAttemptWithTestResultLocked(controlRoot, attempt.AttemptID, TerminalFailed, 24, "red", nil, &failed, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	attempts, err := ReadAttempts(controlRoot)
	if err != nil {
		t.Fatal(err)
	}
	contract := testpolicy.Contract{Groups: []testpolicy.Group{
		{ID: "first", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"first"}},
		{ID: "second", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"second"}},
		{ID: "third", Kind: "unit", Inputs: []string{"source"}, Obligations: []string{"third"}}}}
	template := componentAttemptResult("", "first", identities["first"], "passed")
	template.Groups = nil
	template.SelectedGroups, template.RequiredGroups = []string{"first", "second", "third"}, []string{"first", "second", "third"}
	projection := ReusedTestResult(template, attempts, identities, contract, "goal-a", 2)
	byID := map[string]GroupResult{}
	for _, group := range projection.Groups {
		byID[group.ID] = group
	}
	if byID["first"].Status != "reused" || byID["first"].ReuseAttempt != attempt.AttemptID {
		t.Fatalf("the predecessor's passed group was not reused for the delivery retry: %+v", projection.Groups)
	}
	if byID["second"].Status != "not-run" || byID["second"].NotRunReason != "missing-proof" || byID["third"].Status != "not-run" || projection.Delivery.Sufficient {
		t.Fatalf("the failed or unrun groups were reused: %+v", projection.Groups)
	}
	if err := validateRetainedGroupReuse(controlRoot, "first", byID["first"], testpolicy.PurposeDelivery); err != nil {
		t.Fatalf("the runner refused the delivery retry's reuse: %v", err)
	}
	if err := validateRetainedGroupReuse(controlRoot, "first", byID["first"], testpolicy.PurposeCadence); err == nil {
		t.Fatal("a cadence attempt reused a group from a failed predecessor")
	}
	cadence := template
	cadence.Purpose = testpolicy.PurposeCadence
	for _, group := range ReusedTestResult(cadence, attempts, identities, contract, "goal-a", 2).Groups {
		if group.Status == "reused" {
			t.Fatalf("a cadence template reused %s from a failed predecessor", group.ID)
		}
	}

	// The retry itself: the fixed second script and the unrun third run; the
	// reused first does not launch.
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	writeTestResultScript(t, root, "first", "passed", 0)
	writeTestResultScript(t, root, "second", "passed", 0)
	writeTestResultScript(t, root, "third", "passed", 0)
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	retryContract, retryPlan := stopFixtureContract(testpolicy.PurposeDelivery)
	retryContract.Groups = retryContract.Groups[:3]
	retryContract.Surfaces[0].Deep = nil
	retryContract.Cadence = nil
	retryPlan.RequiredGroups, retryPlan.SelectedGroups = []string{"first", "second", "third"}, []string{"first", "second", "third"}
	retryPlan.Stages = []testpolicy.Stage{{ID: "canary", Groups: []string{"first", "second", "third"}}}
	progress := filepath.Join(root, "artifacts", "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{filepath.Join(root, "artifacts", "launcher.log")}}); err != nil {
		t.Fatal(err)
	}
	retry, status, err := RunTestPlan(context.Background(), TestRunRequest{ControlRoot: controlRoot, ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: retryContract, Plan: retryPlan, AttemptID: "retry", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress, Concurrency: 1,
		Reused: map[string]GroupResult{"first": byID["first"]}, ComponentIdentities: identities})
	if err != nil || status != 0 || !retry.Delivery.Sufficient || retry.LaunchCounts.Test != 2 || retry.LaunchCounts.ReusedTest != 1 {
		t.Fatalf("retry did not reuse the pass and rerun the rest: status=%d counts=%+v delivery=%+v err=%v", status, retry.LaunchCounts, retry.Delivery, err)
	}
	for _, group := range retry.Groups {
		switch group.ID {
		case "first":
			if group.Status != "reused" || group.ReuseAttempt != attempt.AttemptID {
				t.Fatalf("first was not reused on the retry: %+v", group)
			}
		default:
			if group.Status != "passed" || !group.NativeLaunched {
				t.Fatalf("%s did not run on the retry: %+v", group.ID, group)
			}
		}
	}
}

func TestDeliveryAttemptStopsAtAnInvalidReuse(t *testing.T) {
	root := t.TempDir()
	runTestResultGit(t, root, "init", "-q", "-b", "main")
	runTestResultGit(t, root, "config", "user.name", "fixture")
	runTestResultGit(t, root, "config", "user.email", "fixture@example.invalid")
	for _, id := range []string{"first", "second", "third", "deep"} {
		writeTestResultScript(t, root, id, "passed", 0)
	}
	runTestResultGit(t, root, "add", ".")
	runTestResultGit(t, root, "commit", "-qm", "fixture")
	tree := runTestResultGit(t, root, "rev-parse", "HEAD^{tree}")
	contract, plan := stopFixtureContract(testpolicy.PurposeDelivery)
	progress := filepath.Join(root, "artifacts", "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{filepath.Join(root, "artifacts", "launcher.log")}}); err != nil {
		t.Fatal(err)
	}
	// A reuse naming an attempt the control root never retained is stale or
	// forged: the runner marks it invalid, and a delivery attempt stops there.
	stale := GroupResult{ID: "second", Kind: "unit", ExecutionIdentity: strings.Repeat("2", 64), ReuseAttempt: "proof-never-retained",
		Status: "reused", CollectionComplete: true, ToolIdentities: map[string]string{}, ReportDigests: map[string]string{}}
	result, status, err := RunTestPlan(context.Background(), TestRunRequest{ControlRoot: t.TempDir(), ProjectRoot: root, CandidateTree: tree, BaseCommit: "HEAD", PolicyBaseCommit: "HEAD",
		Contract: contract, Plan: plan, AttemptID: "attempt", LogRoot: filepath.Join(root, "artifacts", "test-logs"), ProgressPath: progress, Concurrency: 1,
		Reused: map[string]GroupResult{"second": stale}})
	if err != nil || status != 1 || result.LaunchCounts.Test != 0 || result.Delivery.Sufficient {
		t.Fatalf("an invalid reuse did not stop the delivery attempt: status=%d counts=%+v delivery=%+v err=%v", status, result.LaunchCounts, result.Delivery, err)
	}
	byID := map[string]GroupResult{}
	for _, group := range result.Groups {
		byID[group.ID] = group
	}
	if byID["second"].Status != "invalid" {
		t.Fatalf("stale reuse was not judged invalid: %+v", byID["second"])
	}
	for _, id := range []string{"first", "third", "deep"} {
		if byID[id].Status != "not-run" || byID[id].NotRunReason != haltReason("second") {
			t.Fatalf("group %s after the invalid reuse classified as %+v", id, byID[id])
		}
	}
}
