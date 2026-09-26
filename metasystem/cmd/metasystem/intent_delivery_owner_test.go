package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// wholeOwnerLanding is one claimed, approved, land-ready goal with a single
// reader-record unit and a plan fold on its pushed goal branch, in temporary
// repositories with a temporary bare upstream.
type wholeOwnerLanding struct {
	goalRoot, mainRoot, upstream, base, unit, branchTip string
	receipts                                            []string
}

func newWholeOwnerLanding(t *testing.T) *wholeOwnerLanding {
	t.Helper()
	root, upstream, _ := goalBranchCLIFixtureBelow(t, "m1", "metasystem")
	f := &wholeOwnerLanding{goalRoot: root, upstream: upstream, mainRoot: goalBranchHolderRoot(root)}
	// The public commands resolve a self-hosted checkout by its template
	// marker beside the installation (stateroot templateMode).
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(f.mainRoot), "development", "metasystem-design.md"), []byte("# fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	pagePath := filepath.Join(root, "plans", "goals", "standing-validation.md")
	pageData, err := os.ReadFile(pagePath)
	if err != nil {
		t.Fatal(err)
	}
	file, problems := goal.ParseFile(pageData)
	if len(problems) != 0 {
		t.Fatalf("parse goal page: %v", problems)
	}
	landReadyOpid := goal.Opid("01ARZ3NDEKTSV4RRFFQ69G5FB0", "mac-cli", "m1")
	file.Revision++
	file.Landing = &goal.LandingRecord{At: "2026-09-17T09:00:00Z", Opid: landReadyOpid}
	file.History = append(file.History, goal.HistoryLine{At: file.Landing.At, Opid: landReadyOpid,
		Verb: "land-ready", Actor: "mac-cli+m1", Targets: []string{file.Id}, Keep: -1})
	writeTestingFixtureFile(t, pagePath, goal.RenderFile(file), 0o644)
	writeTestingFixtureFile(t, filepath.Join(root, "memory", "receipts.log"),
		[]byte("1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/goals/standing-validation.md", "memory/receipts.log")
	goalSyncMutationGit(t, root, "commit", "-qm", "mark fixture land ready")
	f.base = goalSyncMutationGit(t, root, "rev-parse", "HEAD")
	goalSyncMutationGit(t, root, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, root, "update-ref", goal.LocalLedgerBranch, f.base)
	goalSyncMutationGit(t, root, "update-ref", goal.AcceptedRef, f.base)
	claim := func() error { return nil }
	writeTestingFixtureFile(t, filepath.Join(root, "owned.go"), []byte("package fixture\n\nconst Landed = 1\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "owned.go")
	f.unit, err = branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", Unit: "u1", OpID: "owner-unit", Kind: branch.Unit, CheckClaim: claim})
	if err != nil {
		t.Fatal(err)
	}
	digest, err := branch.UnitDigest(root, f.unit)
	if err != nil {
		t.Fatal(err)
	}
	readerRecord := "metasystem/records/misc/owner-read.md"
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(root), filepath.FromSlash(readerRecord)), []byte(f.unit+" "+digest+"\n"), 0o644)
	unitTree := goalSyncMutationGit(t, root, "rev-parse", f.unit+"^{tree}")
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", Unit: "u1", OpID: "owner-read", ReaderRecord: readerRecord,
		GateRunID: "fast-clean", GateTree: unitTree, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(root, "plans", "fold.md"), []byte("folded plan\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "plans/fold.md")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", OpID: "owner-fold", Kind: branch.Plan, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", OpID: "owner-push", CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	f.branchTip = goalSyncMutationGit(t, root, "rev-parse", "refs/heads/goal/standing-validation")
	return f
}

// land runs the public land command from the main installation, through
// the production owners. Only the landing proof's execution is replaced: it
// returns a green schema-3 receipt of exactly the tree the owner asked to
// prove, so receipt parsing and candidate matching stay the owners' own.
func (f *wholeOwnerLanding) land(t *testing.T) (int, intentResult) {
	t.Helper()
	owners := defaultIntentOwners()
	delivery := defaultIntentDeliveryOwners()
	delivery.process = func(process intentProcess) intentProcessResult {
		want := []string{"landing", "test-receipt", "--root", f.mainRoot, "--tree", flagValue(process.argv, "--tree"), "--mode", "auto", "--goal", "standing-validation"}
		if len(process.argv) < 2 || !slices.Equal(process.argv[1:], want) {
			t.Fatalf("unexpected owner subprocess %v", process.argv)
		}
		tree := flagValue(process.argv, "--tree")
		f.receipts = append(f.receipts, tree)
		receipt, _ := json.Marshal(map[string]any{"schemaVersion": 3, "tree": tree, "exitStatus": 0,
			"time": "2026-09-17T10:00:00Z", "proof": map[string]any{"attemptId": "whole-owner-proof"}})
		return intentProcessResult{stdout: receipt}
	}
	owners.delivery = delivery
	command, ok := findIntentCommand("land")
	if !ok {
		t.Fatal("land command missing")
	}
	var stdout, stderr bytes.Buffer
	code := runIntentIn(command, []string{"standing-validation", "--repo", f.mainRoot, "--json"}, &stdout, &stderr, f.mainRoot, owners)
	var result intentResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("land printed no result: %v; stdout=%q stderr=%q", err, stdout.String(), stderr.String())
	}
	return code, result
}

func (f *wholeOwnerLanding) remote(t *testing.T, ref string) string {
	t.Helper()
	line := goalSyncMutationGit(t, f.mainRoot, "ls-remote", "--refs", "upstream", ref)
	tip, _, _ := strings.Cut(line, "\t")
	return tip
}

func (f *wholeOwnerLanding) retained(t *testing.T, result intentResult) (string, branch.PreparedLanding) {
	t.Helper()
	data, _ := result.Data.(map[string]any)
	dir, _ := data["retained"].(string)
	prepared, err := branch.ReadPreparedLanding(filepath.Join(dir, "prepared"))
	if dir == "" || err != nil {
		t.Fatalf("no retained prepared landing in %+v: %v", result, err)
	}
	return dir, prepared
}

// assertLanded checks the physical landing the owners published: main moved
// to the prepared landing, its series verifies against the attested unit,
// it names its source and carries the code and fold, and the goal stays open.
func (f *wholeOwnerLanding) assertLanded(t *testing.T, prepared branch.PreparedLanding) {
	t.Helper()
	if main := f.remote(t, "refs/heads/main"); main != prepared.Landing || main == f.base {
		t.Fatalf("upstream main = %s, prepared landing %s, base %s", main, prepared.Landing, f.base)
	}
	goalSyncMutationGit(t, f.mainRoot, "fetch", "-q", "upstream", "main")
	series, err := branch.VerifyLandedSeries(f.mainRoot, prepared.Landing)
	if err != nil || len(series) == 0 {
		t.Fatalf("landed series = %+v, %v", series, err)
	}
	for _, entry := range series {
		if entry.Actual != entry.Expected {
			t.Fatalf("landed series entry %+v differs", entry)
		}
	}
	message := goalSyncMutationGit(t, f.mainRoot, "log", "-1", "--format=%B", prepared.Landing)
	if !strings.Contains(message, "Goal-Source: "+f.unit) || !strings.Contains(message, "Goal-Last: standing-validation") {
		t.Fatalf("landing message lacks its source or last marker:\n%s", message)
	}
	if code := goalSyncMutationGit(t, f.mainRoot, "show", prepared.Landing+":metasystem/owned.go"); !strings.Contains(code, "const Landed = 1") {
		t.Fatalf("landed code = %q", code)
	}
	if fold := goalSyncMutationGit(t, f.mainRoot, "show", prepared.Landing+":metasystem/plans/fold.md"); fold != "folded plan" {
		t.Fatalf("landed fold = %q", fold)
	}
	endpoint, err := goalBranchEndpoint(f.mainRoot)
	if err != nil {
		t.Fatal(err)
	}
	projection, err := goal.Project(endpoint, true, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if file := projection.Tree.Live["standing-validation"]; file == nil || file.State == goal.StateDone {
		t.Fatalf("landing concluded the goal: %+v", file)
	}
}

// TestIntentLandWholeOwnerGitAdapter drives the public land command through
// the unchanged hand-landing owners: candidate composition, land-prep,
// land-push and sweep. Physical Git is the claim here: the composed
// candidate tree, the receipt's tree identity, the atomic main and
// landing-ref publication and the goal-ref sweep must agree across those
// owners. Only the expensive proof run is a fake effect.
func TestIntentLandWholeOwnerGitAdapter(t *testing.T) {
	t.Parallel()
	t.Run("lands once and repeats unchanged", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		code, result := f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || result.Data.(map[string]any)["route"] != "hand" || len(f.receipts) != 1 {
			t.Fatalf("land = %d %+v; receipts %v", code, result, f.receipts)
		}
		if candidate := result.Data.(map[string]any)["candidate"]; candidate != f.receipts[0] {
			t.Fatalf("receipt proved %s, candidate-only composition was %v", f.receipts[0], candidate)
		}
		dir, prepared := f.retained(t, result)
		f.assertLanded(t, prepared)
		if refs := goalSyncMutationGit(t, f.mainRoot, "ls-remote", "--refs", "upstream", "refs/heads/goal/standing-validation", "refs/heads/landing/standing-validation"); refs != "" {
			t.Fatalf("the sweep left branches: %s", refs)
		}
		before, _ := os.Stat(filepath.Join(dir, "prepared", "trunk"))
		code, again := f.land(t)
		if code != 0 || again.Outcome != intentUnchanged || len(f.receipts) != 1 || f.remote(t, "refs/heads/main") != prepared.Landing {
			t.Fatalf("repeat = %d %+v; receipts %v", code, again, f.receipts)
		}
		if after, _ := os.Stat(filepath.Join(dir, "prepared", "trunk")); !after.ModTime().Equal(before.ModTime()) {
			t.Fatal("the repeat regenerated the prepared landing")
		}
	})

	t.Run("refused atomic publication retries the prepared landing", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		goalSyncMutationGit(t, f.upstream, "config", "receive.denyDeletes", "true")
		code, result := f.land(t)
		if code == 0 || result.Outcome != intentPartial || len(f.receipts) != 1 {
			t.Fatalf("refused publication = %d %+v", code, result)
		}
		_, prepared := f.retained(t, result)
		if f.remote(t, "refs/heads/main") != f.base || f.remote(t, "refs/heads/landing/standing-validation") != prepared.Landing {
			t.Fatal("a refused atomic publication moved main or lost the landing ref")
		}
		goalSyncMutationGit(t, f.upstream, "config", "receive.denyDeletes", "false")
		code, result = f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || len(f.receipts) != 1 {
			t.Fatalf("publication retry = %d %+v; receipts %v", code, result, f.receipts)
		}
		f.assertLanded(t, prepared)
	})

	t.Run("a failed sweep resumes without a second proof", func(t *testing.T) {
		f := newWholeOwnerLanding(t)
		lock := filepath.Join(f.upstream, "refs", "heads", "goal", "standing-validation.lock")
		writeTestingFixtureFile(t, lock, []byte("held\n"), 0o644)
		code, result := f.land(t)
		if code == 0 || result.Outcome != intentPartial || len(f.receipts) != 1 {
			t.Fatalf("sweep failure = %d %+v", code, result)
		}
		dir, prepared := f.retained(t, result)
		var landed intentLanded
		encoded, err := os.ReadFile(filepath.Join(dir, "landed.json"))
		if err != nil || json.Unmarshal(encoded, &landed) != nil || landed.Landing != prepared.Landing || landed.Swept {
			t.Fatalf("retained landing = %+v %v", landed, err)
		}
		if f.remote(t, "refs/heads/main") != prepared.Landing || f.remote(t, "refs/heads/landing/standing-validation") != "" ||
			f.remote(t, "refs/heads/goal/standing-validation") != f.branchTip {
			t.Fatal("publication must have landed while the goal ref stays for the sweep")
		}
		if err := os.Remove(lock); err != nil {
			t.Fatal(err)
		}
		code, result = f.land(t)
		if code != 0 || result.Outcome != intentConfirmed || len(f.receipts) != 1 || f.remote(t, "refs/heads/goal/standing-validation") != "" {
			t.Fatalf("resumed sweep = %d %+v; receipts %v", code, result, f.receipts)
		}
		f.assertLanded(t, prepared)
	})
}

// batchAdmissionLanding gives the whole-owner fixture's goal branch a
// critic-root read, so the goal branch is admissible to the landing batch.
// The critic chain is a synthetic already-closed fixture record, as in
// addBatchBranchUnit; public close is proved separately.
func newBatchAdmissionLanding(t *testing.T) (*wholeOwnerLanding, string) {
	t.Helper()
	root, upstream, _ := goalBranchCLIFixtureBelow(t, "m1", "metasystem")
	f := &wholeOwnerLanding{goalRoot: root, upstream: upstream, mainRoot: goalBranchHolderRoot(root)}
	writeTestingFixtureFile(t, filepath.Join(filepath.Dir(f.mainRoot), "development", "metasystem-design.md"), []byte("# fixture\n"), 0o644)
	goalSyncMutationGit(t, root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	// The batch owners find a nested installation by its module file
	// (batch.ModuleRoot); the endpoint carries one before the goal branch.
	writeTestingFixtureFile(t, filepath.Join(f.mainRoot, "go.mod"), []byte("module fixture\n"), 0o644)
	// The batch author owner reads the approver's identity, and the protected
	// test gate its committed testing contract, from metasystem.conf.
	conf := filepath.Join(f.mainRoot, "metasystem.conf")
	confBytes, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, conf, append(confBytes, []byte("\ngoal.human.wido=Wido Approver <wido@example.invalid>\ntesting.contract=contracts/fixture.json\n")...), 0o644)
	contract, err := json.Marshal(testpolicy.Contract{
		SchemaVersion: 1,
		ProjectRisk:   testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:      []testpolicy.Surface{{ID: "fixture", Paths: []string{"metasystem/**"}, Standard: []string{"go-fixture"}, Critical: []string{"go-fixture"}}},
		Groups: []testpolicy.Group{{ID: "go-fixture", Kind: "unit", Adapter: "go", CWD: "metasystem", Inputs: []string{"metasystem/**"},
			Obligations: []string{"go-fixture"}, Platforms: []string{"any"}, TargetMS: 1, Packages: []string{"./..."}, Tests: json.RawMessage(`"all"`)}},
		Always: testpolicy.Always{Canary: []string{"go-fixture"}}, Unknown: []string{"go-fixture"}, Cadence: []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	writeTestingFixtureFile(t, filepath.Join(f.mainRoot, "contracts", "fixture.json"), contract, 0o644)
	goalSyncMutationGit(t, f.mainRoot, "add", "go.mod", "metasystem.conf", "contracts/fixture.json")
	goalSyncMutationGit(t, f.mainRoot, "commit", "-qm", "module marker")
	f.base = goalSyncMutationGit(t, f.mainRoot, "rev-parse", "HEAD")
	goalSyncMutationGit(t, f.mainRoot, "push", "-q", "upstream", "HEAD:main")
	goalSyncMutationGit(t, f.mainRoot, "update-ref", goal.LocalLedgerBranch, f.base)
	goalSyncMutationGit(t, f.mainRoot, "update-ref", goal.AcceptedRef, f.base)
	goalSyncMutationGit(t, root, "reset", "-q", "--hard", f.base)
	// The batch member reader and the landing checkout fetch "origin".
	goalSyncMutationGit(t, f.mainRoot, "remote", "add", "origin", upstream)
	claim := func() error { return nil }
	writeTestingFixtureFile(t, filepath.Join(root, "owned.go"), []byte("package fixture\n\nconst Batched = 1\n"), 0o644)
	goalSyncMutationGit(t, root, "add", "owned.go")
	unit, err := branch.CommitStaged(branch.CommitRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", Unit: "u1", OpID: "batch-unit", Kind: branch.Unit, CheckClaim: claim})
	if err != nil {
		t.Fatal(err)
	}
	f.unit = unit
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: root, Role: "code-critic", Reviews: "commit:" + unit})
	if err != nil || !present {
		t.Fatalf("read subject present=%v err=%v", present, err)
	}
	job := "critic-u1"
	writeBatchBranchJSON(t, filepath.Join(root, "artifacts", "agents", "jobs", job+".json"), map[string]any{
		"jobId": job, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"}})
	writeBatchBranchJSON(t, filepath.Join(root, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	writeBatchBranchJSON(t, filepath.Join(root, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
	if _, _, err := branch.CommitRead(branch.CommitReadRequest{Repo: root, Remote: "upstream", EndpointTip: f.base, GoalID: "standing-validation",
		Unit: "u1", OpID: "batch-read", RootJob: job, GateRunID: "fast-u1", GateTree: subject.Tree, CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(branch.PushRequest{Repo: root, Remote: "upstream", EndpointTip: f.base,
		GoalID: "standing-validation", OpID: "batch-push", CheckClaim: claim}); err != nil {
		t.Fatal(err)
	}
	f.branchTip = goalSyncMutationGit(t, root, "rev-parse", "refs/heads/goal/standing-validation")
	landingRoot := filepath.Join(t.TempDir(), "landing")
	goalSyncMutationGit(t, filepath.Dir(landingRoot), "clone", "-q", upstream, landingRoot)
	return f, landingRoot
}

// TestIntentLandBatchAdmissionGitAdapter drives public land into the actual
// executeBatchJoin: the member reader, transport, unit assembly, protected
// test gate, the batch store and PublishJoinWithAdmission. Fake effects are
// the admission's proof run, the claim handover and the batch owner's start;
// the cost forecast is not in this claim (costForecast=nil, a real branch).
func TestIntentLandBatchAdmissionGitAdapter(t *testing.T) {
	t.Parallel()
	f, landingRoot := newBatchAdmissionLanding(t)
	deps := productionBatchJoinDependencies()
	deps.costForecast = nil
	var handovers, admissions, ensures int
	deps.handover = func(batchJoinRequest, string, batch.Claim) error { handovers++; return nil }
	deps.admissionRun = func(_ string, _ string, unit batch.Unit) (batch.JoinAdmission, error) {
		admissions++
		if unit.State != batch.UnitJoining || unit.Admission == nil || unit.Admission.Status != "handed-over" {
			t.Fatalf("admission ran before handover: %+v", unit)
		}
		return batch.JoinAdmission{Tree: unit.Admission.Tree, Status: "verified", AttemptID: "batch-admission"}, nil
	}
	deps.plan = func(string, string, string) (testpolicy.Plan, error) {
		return testpolicy.Plan{SelectedGroups: []string{"go-fixture"}, RequiredGroups: []string{"go-fixture"}}, nil
	}
	deps.ensure = func(string) error { ensures++; return nil }
	// The claim/budget binding is a fixture fact: the goal's real projected
	// file under this fixture claim, not the stop-authority owner.
	deps.binding = func(root, goalID string, at time.Time) (dispatchcore.GoalBinding, error) {
		endpoint, err := goalBranchEndpoint(root)
		if err != nil {
			return dispatchcore.GoalBinding{}, err
		}
		projection, err := goal.Project(endpoint, true, at)
		if err != nil {
			return dispatchcore.GoalBinding{}, err
		}
		file := projection.Tree.Live[goalID]
		if file == nil || file.Claimed == nil {
			return dispatchcore.GoalBinding{}, fmt.Errorf("fixture goal %s is not claimed", goalID)
		}
		binding := dispatchcore.GoalBinding{GoalID: goalID, Revision: file.Revision, Machine: "mac-cli", Lineage: "m1", File: file}
		binding.Capability.ClaimEpoch = 1
		return binding, nil
	}
	owners := defaultIntentOwners()
	delivery := defaultIntentDeliveryOwners()
	delivery.batchRoot = func(string, time.Time) (string, bool, error) { return landingRoot, true, nil }
	delivery.batchJoin = func(request batchJoinRequest) (batch.Record, error) { return executeBatchJoin(request, deps) }
	delivery.process = func(process intentProcess) intentProcessResult {
		t.Fatalf("the batch route ran a subprocess %v", process.argv)
		return intentProcessResult{}
	}
	owners.delivery = delivery
	command, _ := findIntentCommand("land")
	run := func() intentResult {
		var stdout, stderr bytes.Buffer
		runIntentIn(command, []string{"standing-validation", "--repo", f.mainRoot, "--json"}, &stdout, &stderr, f.mainRoot, owners)
		var result intentResult
		if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
			t.Fatalf("land printed no result: %v; %q %q", err, stdout.String(), stderr.String())
		}
		return result
	}
	result := run()
	data, _ := result.Data.(map[string]any)
	if result.Outcome != intentInProgress || data["route"] != "batch" || data["joinedNow"] != true || handovers != 1 || admissions != 1 || ensures != 1 {
		t.Fatalf("batch land = %+v; handovers %d admissions %d ensures %d", result, handovers, admissions, ensures)
	}
	id, _ := data["batchId"].(string)
	record, err := batch.NewStore(landingRoot, identity.KernelProber{}).Load(id)
	if err != nil || len(record.Units) != 1 {
		t.Fatalf("stored batch %s = %+v, %v", id, record, err)
	}
	unit := record.Units[0]
	if unit.State != batch.UnitJoined || unit.Admission == nil || unit.Admission.Status != "verified" || unit.Admission.AttemptID != "batch-admission" ||
		!unit.GoalLast || unit.BranchTip != f.branchTip || len(unit.Builds) != 1 || unit.Builds[0].Commit != f.unit || unit.Approver != "Wido" {
		t.Fatalf("the admitted member does not bind this goal branch: %+v admission %+v", unit, unit.Admission)
	}
	again := run()
	if again.Outcome != intentInProgress || again.Data.(map[string]any)["joinedNow"] != false || again.Data.(map[string]any)["batchId"] != id ||
		handovers != 1 || admissions != 1 {
		t.Fatalf("a repeat reads the stored membership and never joins again: %+v", again)
	}
}
