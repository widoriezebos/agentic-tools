package branch_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
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

func TestGoalLandingPreparationSeries(t *testing.T) {
	f := newLandFixture(t)

	existingOut := filepath.Join(t.TempDir(), "existing")
	if err := os.Mkdir(existingOut, 0o755); err != nil {
		t.Fatal(err)
	}
	beforeLanding := git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a")
	if _, err := branch.PrepareLanding(landRequest(t, f, existingOut)); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("existing output refusal = %v", err)
	}
	afterLanding := git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a")
	if beforeLanding != afterLanding {
		t.Fatalf("existing output moved landing ref: before=%q after=%q", beforeLanding, afterLanding)
	}

	partialOut := filepath.Join(t.TempDir(), "partial")
	partial := landRequest(t, f, partialOut)
	partial.Last, partial.Through = false, f.units[1]
	partial.GoalPage = "- Next step: goal/goal-a last unit u2 commit " + f.units[1] + " is read clean\n"
	_, err := branch.PrepareLanding(partial)
	requireLandCode(t, err, branch.LandPartialCode)
	requireAbsent(t, partialOut)

	throughOut := filepath.Join(t.TempDir(), "through")
	through := partial
	through.Out = throughOut
	through.GoalPage = "- Next step: land through " + f.units[1] + "\n"
	discoveryReceipt := filepath.Join(t.TempDir(), "through-discovery.json")
	writeLandingReceipt(t, discoveryReceipt, strings.Repeat("0", 40), "discover-through")
	through.TestReceipt = discoveryReceipt
	_, discoveryErr := branch.PrepareLanding(through)
	const candidateMarker = "candidate workspace "
	markerAt := strings.LastIndex(fmt.Sprint(discoveryErr), candidateMarker)
	if markerAt < 0 {
		t.Fatalf("partial candidate discovery = %v", discoveryErr)
	}
	throughProjected := strings.TrimSpace(fmt.Sprint(discoveryErr)[markerAt+len(candidateMarker):])
	throughReceipt := filepath.Join(t.TempDir(), "through-receipt.json")
	writeLandingReceipt(t, throughReceipt, throughProjected, "attempt-through")
	through.TestReceipt = throughReceipt
	throughResult, err := branch.PrepareLanding(through)
	if err != nil {
		t.Fatal(err)
	}
	throughCommits := strings.Fields(git(t, f.root, "rev-list", "--reverse", f.base+".."+throughResult.Landing))
	if len(throughCommits) != 2 {
		t.Fatalf("partial landing commits = %v", throughCommits)
	}
	for _, commit := range throughCommits {
		if message := git(t, f.root, "show", "-s", "--format=%B", commit); strings.Contains(message, "Goal-Last:") {
			t.Fatalf("partial landing commit carries Goal-Last:\n%s", message)
		}
	}

	intermediate := git(t, f.root, "rev-parse", f.units[0]+"^{tree}")
	intermediate, err = landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), intermediate)
	if err != nil {
		t.Fatal(err)
	}
	wrongReceipt := filepath.Join(t.TempDir(), "receipt.json")
	writeLandingReceipt(t, wrongReceipt, intermediate, "attempt-interim")
	wrongOut := filepath.Join(t.TempDir(), "wrong")
	wrong := landRequest(t, f, wrongOut)
	wrong.TestReceipt = wrongReceipt
	_, err = branch.PrepareLanding(wrong)
	requireLandCode(t, err, branch.LandUnprovenCode)
	requireAbsent(t, wrongOut)

	unboundOut := filepath.Join(t.TempDir(), "unbound")
	unbound := landRequest(t, f, unboundOut)
	unbound.ApprovedBy = "human:Nobody"
	_, err = branch.PrepareLanding(unbound)
	requireLandCode(t, err, branch.LandAuthorUnboundCode)
	requireAbsent(t, unboundOut)

	out := filepath.Join(t.TempDir(), "prepared")
	result, err := branch.PrepareLanding(landRequest(t, f, out))
	if err != nil {
		t.Fatal(err)
	}
	if result.Endpoint != f.base || result.Attempt != "attempt-deep" || result.LastUnit != "u3" ||
		result.ProofNumber != 1 || git(t, f.origin, "rev-parse", "refs/heads/landing/goal-a") != result.Landing ||
		git(t, f.root, "rev-parse", result.Landing+"^{tree}") != result.Candidate {
		t.Fatalf("landing result = %+v", result)
	}
	commits := strings.Fields(git(t, f.root, "rev-list", "--reverse", f.base+".."+result.Landing))
	if len(commits) != 3 {
		t.Fatalf("landing commits = %v", commits)
	}
	for index, commit := range commits {
		message := git(t, f.root, "show", "-s", "--format=%B", commit)
		want := []string{"Goal-Unit: goal-a/u", "Goal-Digest: ", "Goal-Source: " + f.units[index], "Landed-By: seat-a"}
		for _, fragment := range want {
			if !strings.Contains(message, fragment) {
				t.Fatalf("message %d lacks %q:\n%s", index, fragment, message)
			}
		}
		if got := strings.Count(message, "Goal-Last: goal-a"); got != map[bool]int{true: 1, false: 0}[index == 2] {
			t.Fatalf("Goal-Last count on commit %d = %d", index, got)
		}
		identity := git(t, f.root, "show", "-s", "--format=%an <%ae>|%cn <%ce>", commit)
		if identity != "Wido Approver <wido@example.invalid>|Wido Approver <wido@example.invalid>" {
			t.Fatalf("landing identity = %s", identity)
		}
	}
	lastMessage := git(t, f.root, "show", "-s", "--format=%B", commits[2])
	if !strings.Contains(lastMessage, "Goal-Fold: metasystem/plans/tail.md") ||
		!strings.Contains(lastMessage, "Goal-Fold: metasystem/plans/between.md") {
		t.Fatalf("last commit does not carry its plan folds:\n%s", lastMessage)
	}
	receipts := git(t, f.root, "show", result.Landing+":metasystem/memory/receipts.log")
	if strings.Count(receipts, "|goal=goal-a|") != 3 || strings.Count(receipts, "|last_unit=u3|") != 3 ||
		strings.Count(receipts, "|proof=attempt-deep|") != 3 {
		t.Fatalf("landing receipt rows:\n%s", receipts)
	}
	rows := strings.Split(strings.TrimSpace(receipts), "\n")
	if len(rows) != 4 {
		t.Fatalf("receipt row count = %d: %v", len(rows), rows)
	}
	for index, row := range rows[1:] {
		fields := strings.Split(row, "|")
		if len(fields) < 17 {
			t.Fatalf("receipt row %d is incomplete: %s", index, row)
		}
		epoch, parseErr := strconv.ParseInt(fields[0], 10, 64)
		stamp, stampErr := time.Parse("2006-01-02T15:04:05Z", fields[1])
		if parseErr != nil || stampErr != nil || epoch != stamp.Unix() || fields[1] != "2026-09-17T10:00:00Z" {
			t.Fatalf("receipt row %d time = %q|%q parse=%v/%v", index, fields[0], fields[1], parseErr, stampErr)
		}
		for _, field := range []string{"skills=none", "verify=clean", "corrections=0", "stop_loss=no", "delegate=none", "built_by=coordinator"} {
			if !strings.Contains("|"+row+"|", "|"+field+"|") {
				t.Fatalf("receipt row %d lacks %s: %s", index, field, row)
			}
		}
		oneRow := filepath.Join(t.TempDir(), fmt.Sprintf("receipt-%d.log", index))
		if err := os.WriteFile(oneRow, []byte(row+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		stats := receipt.Stats(receipt.Options{File: oneRow, All: true})
		if stats.Code != 0 || !strings.Contains(strings.Join(stats.Out, " "), "receipts=1") ||
			!strings.Contains(strings.Join(stats.Out, " "), "span_days=0.0") {
			t.Fatalf("receipt reader rejected row %d: %+v", index, stats)
		}
	}
	draft, err := os.ReadFile(filepath.Join(out, "record-draft"))
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"n=1", "endpoint=" + f.base, "candidate=" + result.Candidate,
		"landing=" + result.Landing, "attempt=attempt-deep"} {
		if !strings.Contains(string(draft), value) {
			t.Fatalf("record draft lacks %q: %s", value, draft)
		}
	}

	stage(t, f.branchFixture, "metasystem/plans/tail-two.md", "new candidate\n")
	if _, err := branch.CommitStaged(branch.CommitRequest{Repo: f.root, Remote: "origin", EndpointTip: f.base,
		GoalID: "goal-a", OpID: "land-tail-two", Kind: branch.Plan, CheckClaim: claimAllowed}); err != nil {
		t.Fatal(err)
	}
	if _, err := branch.Push(pushRequest(f.branchFixture, "land-push-two")); err != nil {
		t.Fatal(err)
	}
	f.tip = git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	f.projected, err = landing.ProjectWorkspaceTree(filepath.Join(f.root, "metasystem"), git(t, f.root, "rev-parse", f.tip+"^{tree}"))
	if err != nil {
		t.Fatal(err)
	}
	writeLandingReceipt(t, f.receipt, f.projected, "attempt-two")
	secondOut := filepath.Join(t.TempDir(), "second")
	second, err := branch.PrepareLanding(landRequest(t, f, secondOut))
	if err != nil || second.Landing == result.Landing || git(t, f.origin, "rev-parse", "refs/heads/landing/goal-a") != second.Landing {
		t.Fatalf("replacement landing = %+v err=%v", second, err)
	}

	movedOut := filepath.Join(t.TempDir(), "moved")
	moved := landRequest(t, f, movedOut)
	intruder := git(t, f.root, "commit-tree", f.base+"^{tree}", "-p", f.base, "-m", "landing intruder")
	moved.Hooks.AfterLandingRead = func() error {
		git(t, f.root, "push", "-q", "--force", "origin", intruder+":refs/heads/landing/goal-a")
		return nil
	}
	_, err = branch.PrepareLanding(moved)
	requireLandCode(t, err, branch.LandBranchMovedCode)
	requireAbsent(t, movedOut)
}

func TestBranchLandingSyncMergesTestingContractBySurface(t *testing.T) {
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

	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
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
	last := len(contract.Surfaces) - 1
	contract.Surfaces = append(contract.Surfaces[:last], branchContractSurfaceFixture(name, id), contract.Surfaces[last])
	contract.Surfaces[len(contract.Surfaces)-1].Standard = append(contract.Surfaces[len(contract.Surfaces)-1].Standard, id)
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
	t.Run("endpoint preimage", func(t *testing.T) {
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

func TestGoalLandingLastRefusesUnitBeyondPrefix(t *testing.T) {
	f := newLandFixture(t)
	commitUnit(t, f.branchFixture, "u4", "metasystem/four.go", "four\n")
	if _, err := branch.Push(pushRequest(f.branchFixture, "land-unread-u4")); err != nil {
		t.Fatal(err)
	}
	f.tip = git(t, f.root, "rev-parse", "refs/heads/goal/goal-a")
	out := filepath.Join(t.TempDir(), "last")
	_, err := branch.PrepareLanding(landRequest(t, f, out))
	requireLandCode(t, err, branch.LandPartialCode)
	requireAbsent(t, out)
	if remote := git(t, f.root, "ls-remote", "--refs", "origin", "refs/heads/landing/goal-a"); remote != "" {
		t.Fatalf("unread suffix published landing ref: %s", remote)
	}
}
