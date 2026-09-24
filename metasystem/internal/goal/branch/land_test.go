package branch_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

type landFixture struct {
	*branchFixture
	units     []string
	tip       string
	projected string
	receipt   string
}

func newLandFixture(t *testing.T) landFixture {
	t.Helper()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "seed receipt register")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	git(t, f.root, "config", "user.name", "Ambient Seat")
	git(t, f.root, "config", "user.email", "ambient@example.invalid")
	git(t, f.root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")

	stage(t, f, "metasystem/plans/goal-a.md", "approved goal plan\n")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "land-plan", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	var units []string
	for index, item := range []struct{ unit, path, body string }{
		{"u1", "metasystem/one.go", "one\n"},
		{"u2", "metasystem/two.go", "two\n"},
		{"u3", "metasystem/three.go", "three\n"},
	} {
		commit := commitUnit(t, f, item.unit, item.path, item.body)
		units = append(units, commit)
		readUnit(t, f, item.unit, commit)
		if index == 1 {
			stage(t, f, "metasystem/plans/between.md", "folded into u3\n")
			if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
				GoalID: "goal-a", OpID: "land-between", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
				t.Fatal(err)
			}
		}
	}
	stage(t, f, "metasystem/plans/tail.md", "tail fold\n")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "land-tail", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(f, "land-push")); err != nil {
		t.Fatal(err)
	}
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, receipt, projected, "attempt-deep")
	return landFixture{branchFixture: f, units: units, tip: tip, projected: projected, receipt: receipt}
}

func writeLandingReceipt(t *testing.T, path, tree, attempt string) {
	t.Helper()
	body := fmt.Sprintf(`{"schemaVersion":3,"tree":%q,"exitStatus":0,"time":"2026-09-17T10:00:00Z","proof":{"attemptId":%q}}`, tree, attempt)
	if err := os.WriteFile(path, []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func landRequest(t *testing.T, f landFixture, out string) branch.LandRequest {
	t.Helper()
	return branch.LandRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base, BranchTip: f.tip,
		GoalID: "goal-a", Out: out, TestReceipt: f.receipt, Last: true, LandingReady: true,
		GoalPage: "the whole goal is ready", ApprovedBy: "human:Wido", Seat: "seat-a", CheckClaim: claimAllowed}
}

func requireLandCode(t *testing.T, err error, code string) {
	t.Helper()
	var refusal *branch.OpError
	if !errors.As(err, &refusal) || refusal.Code != code {
		t.Fatalf("refusal = %v, want %s", err, code)
	}
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("%s exists after refusal: %v", path, err)
	}
}

func TestGLENestedInstallationLandingProjection(t *testing.T) {
	t.Parallel()
	f := newLandFixture(t)
	request := landRequest(t, f, filepath.Join(t.TempDir(), "prepared"))
	request.Repo = filepath.Join(f.root, "metasystem")
	result, err := branch.PrepareLanding(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate == "" || result.Landing == "" {
		t.Fatalf("incomplete landing result: %+v", result)
	}
}

func TestGLEProjectRootLandingProjection(t *testing.T) {
	t.Parallel()
	f := newLandFixture(t)
	request := landRequest(t, f, filepath.Join(t.TempDir(), "prepared"))
	if request.Repo != f.root {
		t.Fatalf("project-root caller changed: %s", request.Repo)
	}
	result, err := branch.PrepareLanding(request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidate == "" || result.Landing == "" {
		t.Fatalf("incomplete landing result: %+v", result)
	}
}

func TestBranchLandingSyncMergesTestingContractBySurface(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeBranchContract(t, f.root, branchContractFixture())
	write(t, f.root, "metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "testing contract base")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	git(t, f.root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")

	stageBranchContract(t, f.root, branchContractWithAddition(branchContractFixture(), "goal", 1100))
	unit, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Unit: "testing", OpID: "testing-unit", Kind: branch.Unit, CheckClaim: claimAllowed})
	if err != nil {
		t.Fatal(err)
	}
	readUnit(t, f, "testing", unit)
	branchTip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")

	git(t, f.root, "switch", "--quiet", "--detach", f.base)
	stageBranchContract(t, f.root, branchContractWithAddition(branchContractFixture(), "main", 1200))
	git(t, f.root, "commit", "-qm", "main testing group")
	endpointTip := git(t, f.root, "rev-parse", "HEAD")
	git(t, f.root, "push", "-q", "origin", endpointTip+":main")
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, receipt, strings.Repeat("0", 40), "discover-sync")
	request := branch.LandRequest{Repo: f.root, Remote: "origin", EndpointTip: endpointTip, BranchTip: branchTip,
		GoalID: "goal-a", Out: filepath.Join(t.TempDir(), "discover"), TestReceipt: receipt, Last: true, LandingReady: true,
		GoalPage: "the whole goal is ready", ApprovedBy: "human:Wido", Seat: "seat-a", CheckClaim: claimAllowed}
	_, discoveryErr := branch.PrepareLanding(request)
	const marker = "candidate workspace "
	markerAt := strings.LastIndex(fmt.Sprint(discoveryErr), marker)
	if markerAt < 0 {
		t.Fatalf("sync candidate discovery = %v", discoveryErr)
	}
	candidate := strings.TrimSpace(fmt.Sprint(discoveryErr)[markerAt+len(marker):])
	writeLandingReceipt(t, receipt, candidate, "branch-sync")
	request.Out = filepath.Join(t.TempDir(), "prepared")
	result, err := branch.PrepareLanding(request)
	if err != nil {
		t.Fatal(err)
	}
	merged := decodeBranchContract(t, git(t, f.root, "show", result.Candidate+":metasystem/testing.json"))
	if got := branchContractIDs(merged.Groups); !reflect.DeepEqual(got, []string{"base-group", "main-group", "goal-group"}) {
		t.Fatalf("synced groups = %v", got)
	}
	if got := branchContractSurface(t, merged, "residual").Standard; !reflect.DeepEqual(got, []string{"base-group", "main-group", "goal-group"}) {
		t.Fatalf("synced residual groups = %v", got)
	}
}

func branchContractFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			branchContractSurfaceFixture("base", "base-group"),
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"base-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups: []testpolicy.Group{branchContractGroup("base-group", 1000)}, Always: testpolicy.Always{Canary: []string{}, Standard: []string{}},
		Unknown: []string{"base-group"}, Cadence: []string{}}
}

func branchContractWithAddition(contract testpolicy.Contract, name string, target int64) testpolicy.Contract {
	id := name + "-group"
	contract.Groups = append(contract.Groups, branchContractGroup(id, target))
	contract.Surfaces = append(contract.Surfaces, branchContractSurfaceFixture(name, id))
	for index := range contract.Surfaces {
		if contract.Surfaces[index].ID == contract.Fallback {
			contract.Surfaces[index].Standard = append(contract.Surfaces[index].Standard, id)
		}
	}
	contract.Unknown = append(contract.Unknown, id)
	return contract
}

func branchContractGroup(id string, target int64) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{},
		Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: target,
		Packages: []string{"./example"}, Tests: json.RawMessage(`["TestExample"]`)}
}

func branchContractSurfaceFixture(id, group string) testpolicy.Surface {
	return testpolicy.Surface{ID: id, Paths: []string{id + "/**"}, DependsOn: []string{}, Standard: []string{group}, Deep: []string{}, Critical: []string{}}
}

func writeBranchContract(t *testing.T, root string, contract testpolicy.Contract) {
	t.Helper()
	data, err := contractmerge.Render(contract)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, "metasystem/testing.json", string(data))
}

func stageBranchContract(t *testing.T, root string, contract testpolicy.Contract) {
	t.Helper()
	writeBranchContract(t, root, contract)
	git(t, root, "add", "metasystem/testing.json")
}

func decodeBranchContract(t *testing.T, data string) testpolicy.Contract {
	t.Helper()
	contract, err := testpolicy.Decode([]byte(data))
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

func branchContractIDs(groups []testpolicy.Group) []string {
	ids := make([]string, len(groups))
	for i := range groups {
		ids[i] = groups[i].ID
	}
	return ids
}

func branchContractSurface(t *testing.T, contract testpolicy.Contract, id string) testpolicy.Surface {
	t.Helper()
	for _, surface := range contract.Surfaces {
		if surface.ID == id {
			return surface
		}
	}
	t.Fatalf("surface %s is absent", id)
	return testpolicy.Surface{}
}

func TestGoalLandingKeepsBuildUnitList(t *testing.T) {
	t.Parallel()
	f := newBranchFixture(t)
	write(t, f.root, "metasystem/memory/receipts.log", "1|1970-01-01T00:00:00Z|RECEIPT|type=seed|outcome=shipped\n")
	git(t, f.root, "add", ".")
	git(t, f.root, "commit", "-qm", "seed receipt register")
	git(t, f.root, "push", "-q", "origin", "HEAD:main")
	f.base = git(t, f.root, "rev-parse", "HEAD")
	git(t, f.root, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	stage(t, f, "metasystem/multi.go", "multi\n")
	unit, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", Units: []string{"5", "6"}, OpID: "landing-multi", Kind: branch.Unit, CheckClaim: claimAllowed})
	if err != nil {
		t.Fatal(err)
	}
	readUnit(t, f, "5+6", unit)
	if _, err := branch.Push(pushRequest(f, "landing-multi-push")); err != nil {
		t.Fatal(err)
	}
	tip := git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	receipt := filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, receipt, projected, "attempt-multi")
	fixture := landFixture{branchFixture: f, units: []string{unit}, tip: tip, projected: projected, receipt: receipt}
	result, err := branch.PrepareLanding(landRequest(t, fixture, filepath.Join(t.TempDir(), "prepared")))
	if err != nil {
		t.Fatal(err)
	}
	message := git(t, f.root, "show", "-s", "--format=%B", result.Landing)
	verified, verifyErr := branch.VerifyLanded(f.root, result.Landing)
	if result.LastUnit != "6" || !strings.Contains(message, "Goal-Unit: goal-a/5+6") || verifyErr != nil || verified.Units != "5+6" {
		t.Fatalf("multi-unit landing=%+v message=%q verified=%+v err=%v", result, message, verified, verifyErr)
	}
}

func TestGoalLandingPreimageRetryAndCanaryFence(t *testing.T) {
	t.Parallel()
	t.Run("endpoint preimage", func(t *testing.T) {
		t.Parallel()
		f := newLandFixture(t)
		other := filepath.Join(t.TempDir(), "endpoint")
		git(t, filepath.Dir(other), "clone", "-q", f.origin, other)
		git(t, other, "config", "user.name", "endpoint")
		git(t, other, "config", "user.email", "endpoint@example.invalid")
		write(t, other, "metasystem/one.go", "endpoint owns this path\n")
		git(t, other, "add", ".")
		git(t, other, "commit", "-qm", "endpoint move")
		git(t, other, "push", "-q", "origin", "HEAD:main")
		git(t, f.root, "fetch", "-q", "origin", "main")
		out := filepath.Join(t.TempDir(), "preimage")
		req := landRequest(t, f, out)
		req.EndpointTip = git(t, f.root, "rev-parse", "origin/main")
		_, err := branch.PrepareLanding(req)
		requireLandCode(t, err, branch.UnitRereadCode)
		requireAbsent(t, out)
	})

	t.Run("recorded red candidate", func(t *testing.T) {
		t.Parallel()
		f := newLandFixture(t)
		firstOut := filepath.Join(t.TempDir(), "first")
		first, err := branch.PrepareLanding(landRequest(t, f, firstOut))
		if err != nil {
			t.Fatal(err)
		}
		proof := branch.LandingProof{Number: 1, Endpoint: first.Endpoint, Candidate: first.Candidate,
			Landing: first.Landing, Attempt: first.Attempt, Verdict: "red", Groups: []string{"deep"}}
		write(t, f.root, "metasystem/records/misc/goal-a-landing.md", branch.RenderLandingProof(proof)+"\n")
		git(t, f.root, "add", "metasystem/records/misc/goal-a-landing.md")
		if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", OpID: "red-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(pushRequest(f.branchFixture, "red-record-push")); err != nil {
			t.Fatal(err)
		}
		f.tip = git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
		projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", f.tip+"^{tree}"))
		if err != nil {
			t.Fatal(err)
		}
		writeLandingReceipt(t, f.receipt, projected, "attempt-red-rerun")
		out := filepath.Join(t.TempDir(), "retry")
		_, err = branch.PrepareLanding(landRequest(t, f, out))
		requireLandCode(t, err, branch.LandRetryCode)
		requireAbsent(t, out)
	})

	t.Run("red proof needs clean canary", func(t *testing.T) {
		t.Parallel()
		f := newLandFixture(t)
		proof := branch.LandingProof{Number: 1, Endpoint: f.base, Candidate: f.base, Landing: f.base,
			Attempt: "old", Verdict: "red", Groups: []string{"deep"}}
		write(t, f.root, "metasystem/records/misc/goal-a-landing.md", branch.RenderLandingProof(proof)+"\n")
		git(t, f.root, "add", "metasystem/records/misc/goal-a-landing.md")
		if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", OpID: "unchecked-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(pushRequest(f.branchFixture, "unchecked-push")); err != nil {
			t.Fatal(err)
		}
		f.tip = git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
		projected, err := landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", f.tip+"^{tree}"))
		if err != nil {
			t.Fatal(err)
		}
		writeLandingReceipt(t, f.receipt, projected, "attempt-after-red")
		uncheckedOut := filepath.Join(t.TempDir(), "unchecked")
		_, err = branch.PrepareLanding(landRequest(t, f, uncheckedOut))
		requireLandCode(t, err, branch.LandUncheckedCode)
		requireAbsent(t, uncheckedOut)

		proof.CanaryRun, proof.CanaryTip, proof.Fix = "canary-clean", f.tip, f.units[2]
		write(t, f.root, "metasystem/records/misc/goal-a-landing.md", branch.RenderLandingProof(proof)+"\n")
		git(t, f.root, "add", "metasystem/records/misc/goal-a-landing.md")
		if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
			GoalID: "goal-a", OpID: "checked-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
			t.Fatal(err)
		}
		if _, err := branch.Push(pushRequest(f.branchFixture, "checked-push")); err != nil {
			t.Fatal(err)
		}
		f.tip = git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
		projected, err = landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", f.tip+"^{tree}"))
		if err != nil {
			t.Fatal(err)
		}
		writeLandingReceipt(t, f.receipt, projected, "attempt-after-canary")
		checkedOut := filepath.Join(t.TempDir(), "checked")
		result, err := branch.PrepareLanding(landRequest(t, f, checkedOut))
		if err != nil || result.ProofNumber != 2 {
			t.Fatalf("checked landing = %+v err=%v", result, err)
		}
	})
}

func TestGoalLandingRetryIdentitySurvivesASecondClone(t *testing.T) {
	t.Parallel()
	f := newLandFixture(t)
	first, err := branch.PrepareLanding(landRequest(t, f, filepath.Join(t.TempDir(), "first")))
	if err != nil {
		t.Fatal(err)
	}
	if first.RetryIdentity == "" {
		t.Fatal("prepared landing has no persisted retry identity")
	}
	git(t, f.root, "push", "-q", "origin", ":refs/heads/landing/goal-a")
	proof := branch.LandingProof{Number: 1, Endpoint: first.Endpoint, Candidate: first.Candidate, Landing: first.Landing,
		RetryIdentity: first.RetryIdentity, Attempt: first.Attempt, Verdict: "red", Groups: []string{"deep"}}
	write(t, f.root, "metasystem/records/misc/goal-a-landing.md", branch.RenderLandingProof(proof)+"\n")
	git(t, f.root, "add", "metasystem/records/misc/goal-a-landing.md")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "clone-red-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(f.branchFixture, "clone-red-push")); err != nil {
		t.Fatal(err)
	}

	clone := filepath.Join(t.TempDir(), "clone-b")
	git(t, filepath.Dir(clone), "clone", "-q", "--no-local", f.origin, clone)
	git(t, clone, "config", "user.name", "Clone B")
	git(t, clone, "config", "user.email", "clone-b@example.invalid")
	git(t, clone, "config", "goal.human.Wido", "Wido Approver <wido@example.invalid>")
	freshBase := &branchFixture{root: clone, origin: f.origin, base: f.base}
	fresh := landFixture{branchFixture: freshBase, units: append([]string(nil), f.units...), tip: git(t, clone, "rev-parse", "origin/goal/goal-a")}
	fresh.projected, err = landing.ProjectWorkspaceTree(filepath.Join(clone, "metasystem"), git(t, clone, "rev-parse", fresh.tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	fresh.receipt = filepath.Join(t.TempDir(), "retry.json")
	writeLandingReceipt(t, fresh.receipt, fresh.projected, "clone-b-retry")
	_, err = branch.PrepareLanding(landRequest(t, fresh, filepath.Join(t.TempDir(), "retry")))
	requireLandCode(t, err, branch.LandRetryCode)

	fix := commitUnit(t, freshBase, "land-fix-1", "metasystem/fix.go", "fixed\n")
	readUnit(t, freshBase, "land-fix-1", fix)
	if _, err := branch.Push(pushRequest(freshBase, "clone-b-fix-push")); err != nil {
		t.Fatal(err)
	}
	fresh.tip = git(t, clone, "rev-parse", "refs/heads/goal/goal-a")
	proof.CanaryRun, proof.CanaryTip, proof.Fix = "clone-b-canary", fresh.tip, fix
	write(t, clone, "metasystem/records/misc/goal-a-landing.md", branch.RenderLandingProof(proof)+"\n")
	git(t, clone, "add", "metasystem/records/misc/goal-a-landing.md")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: clone, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "clone-b-canary-record", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(freshBase, "clone-b-canary-push")); err != nil {
		t.Fatal(err)
	}
	fresh.tip = git(t, clone, "rev-parse", "refs/heads/goal/goal-a")
	fresh.projected, err = landing.ProjectWorkspaceTree(filepath.Join(clone, "metasystem"), git(t, clone, "rev-parse", fresh.tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	writeLandingReceipt(t, fresh.receipt, fresh.projected, "clone-b-fixed")
	result, err := branch.PrepareLanding(landRequest(t, fresh, filepath.Join(t.TempDir(), "fixed")))
	if err != nil || result.Landing == first.Landing {
		t.Fatalf("second clone fixed landing=%+v err=%v", result, err)
	}
}
