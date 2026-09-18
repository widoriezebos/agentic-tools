package batch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractmerge"
)

type goalBranchBed struct {
	root, base string
}

func branchGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), output, err)
	}
	return strings.TrimSpace(string(output))
}

func branchWrite(t *testing.T, root, path, body string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(path))
	must(t, os.MkdirAll(filepath.Dir(abs), 0o755))
	must(t, os.WriteFile(abs, []byte(body), 0o644))
}

func branchJSON(t *testing.T, root, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	must(t, err)
	branchWrite(t, root, path, string(append(data, '\n')))
}

func newGoalBranchBed(t *testing.T) goalBranchBed {
	t.Helper()
	root := t.TempDir()
	branchGit(t, root, "init", "-q", "-b", "main")
	branchGit(t, root, "config", "user.name", "Batch Fixture")
	branchGit(t, root, "config", "user.email", "batch@example.invalid")
	for _, path := range []string{"metasystem/a-1.txt", "metasystem/a-2.txt", "metasystem/a-3.txt", "metasystem/b-1.txt"} {
		branchWrite(t, root, path, "base\n")
	}
	branchWrite(t, root, "metasystem/gone/value.go", "package gone\n")
	branchGit(t, root, "add", ".")
	branchGit(t, root, "commit", "-qm", "base")
	return goalBranchBed{root: root, base: branchGit(t, root, "rev-parse", "HEAD")}
}

func branchCriticRead(t *testing.T, bed goalBranchBed, goalID, unit, commit, job string, readerRecord bool) string {
	t.Helper()
	subject, present, err := dispatch.ComputeReadSubject(dispatch.ReadSubjectRequest{RepoRoot: bed.root, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("subject present=%v err=%v", present, err)
	}
	request := goalbranch.CommitReadRequest{
		Repo: bed.root, Remote: bed.root, EndpointTip: bed.base, GoalID: goalID, Unit: unit, OpID: "read-" + job,
		CheckClaim: func() error { return nil }, GateRunID: "fast-" + job, GateTree: subject.Tree,
	}
	if readerRecord {
		digest, err := goalbranch.UnitDigest(bed.root, commit)
		must(t, err)
		path := "metasystem/records/misc/" + job + ".md"
		branchWrite(t, bed.root, path, "Read "+commit+" with unit digest "+digest+".\n")
		request.ReaderRecord = path
	} else {
		root := map[string]any{
			"jobId": job, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
			"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
			"closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"},
		}
		branchJSON(t, bed.root, "artifacts/agents/jobs/"+job+".json", root)
		branchJSON(t, bed.root, "artifacts/agents/"+job+"/rounds/1/subject.json", subject)
		branchJSON(t, bed.root, "artifacts/agents/"+job+"/rounds/1/return.json", map[string]any{"jobId": job, "round": 1, "reviewedTree": subject.Tree})
		request.RootJob = job
	}
	tip, _, err := goalbranch.CommitRead(request)
	if err != nil {
		t.Fatal(err)
	}
	return tip
}

func buildGoalBranch(t *testing.T, bed goalBranchBed, goalID string, units []string, readerAt int) string {
	t.Helper()
	branchGit(t, bed.root, "switch", "--quiet", "--detach", bed.base)
	var tip string
	for index, unit := range units {
		path := "metasystem/" + strings.TrimPrefix(goalID, "goal-") + "-" + unit + ".txt"
		branchWrite(t, bed.root, path, goalID+"/"+unit+"\n")
		branchGit(t, bed.root, "add", "--", path)
		commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{
			Repo: bed.root, Remote: bed.root, EndpointTip: bed.base, GoalID: goalID, Unit: unit, OpID: "build-" + goalID + "-" + unit,
			Kind: goalbranch.Unit, CheckClaim: func() error { return nil },
		})
		if err != nil {
			t.Fatal(err)
		}
		tip = branchCriticRead(t, bed, goalID, unit, commit, "critic-"+goalID+"-"+unit, index == readerAt)
	}
	return tip
}

func TestBatchBranchReaderCertifiesThreeBuildMember(t *testing.T) {
	bed := newGoalBranchBed(t)
	tip := buildGoalBranch(t, bed, "goal-a", []string{"1", "2", "3"}, -1)
	member, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tip, GoalID: "goal-a", Last: true})
	if err != nil || len(member.Builds) != 3 {
		t.Fatalf("member=%+v err=%v", member, err)
	}
	prefixes, err := AssembleBranchMembers(bed.root, branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}"), []BranchMember{member})
	if err != nil || len(prefixes) != 1 {
		t.Fatalf("prefixes=%v err=%v", prefixes, err)
	}
	for _, unit := range []string{"1", "2", "3"} {
		if got := branchGit(t, bed.root, "show", prefixes[0]+":metasystem/a-"+unit+".txt"); got != "goal-a/"+unit {
			t.Fatalf("unit %s bytes=%q", unit, got)
		}
	}
}

func TestAssembleUnitsKeepsBranchAndChainMembersInJoinOrder(t *testing.T) {
	bed := newGoalBranchBed(t)
	branchTip := buildGoalBranch(t, bed, "goal-a", []string{"1"}, -1)
	member, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: branchTip, GoalID: "goal-a", Last: true})
	if err != nil {
		t.Fatal(err)
	}
	branchGit(t, bed.root, "switch", "-q", "--detach", bed.base)
	branchWrite(t, bed.root, "metasystem/b-1.txt", "chain\n")
	branchGit(t, bed.root, "add", "metasystem/b-1.txt")
	branchGit(t, bed.root, "commit", "-qm", "chain fixture")
	chainCommit := branchGit(t, bed.root, "rev-parse", "HEAD")
	patch := branchGit(t, bed.root, "diff", "--binary", "--full-index", bed.base, chainCommit)
	branchWrite(t, bed.root, "artifacts/agents/landing-batches/chains/chain-a/diff.patch", patch+"\n")
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	branchUnit := BindBranchMember(Unit{GoalID: "goal-a"}, member)
	chainUnit := Unit{GoalID: "goal-chain", Chain: "chain-a"}
	for _, test := range []struct {
		name  string
		units []Unit
	}{
		{name: "branch then chain", units: []Unit{branchUnit, chainUnit}},
		{name: "chain then branch", units: []Unit{chainUnit, branchUnit}},
	} {
		t.Run(test.name, func(t *testing.T) {
			prefixes, err := assembleUnits(bed.root, baseTree, test.units)
			if err != nil || len(prefixes) != 2 {
				t.Fatalf("prefixes=%v err=%v", prefixes, err)
			}
			if got := branchGit(t, bed.root, "show", prefixes[1]+":metasystem/a-1.txt"); got != "goal-a/1" {
				t.Fatalf("branch bytes=%q", got)
			}
			if got := branchGit(t, bed.root, "show", prefixes[1]+":metasystem/b-1.txt"); got != "chain" {
				t.Fatalf("chain bytes=%q", got)
			}
		})
	}
}

func TestBatchBranchReaderRefusesReaderRecord(t *testing.T) {
	bed := newGoalBranchBed(t)
	tip := buildGoalBranch(t, bed, "goal-a", []string{"1", "2", "3"}, 1)
	_, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tip, GoalID: "goal-a", Last: true})
	if err == nil || !strings.HasPrefix(err.Error(), "BATCH_JOIN_UNREAD:") {
		t.Fatalf("reader-record join=%v", err)
	}
}

func TestBatchBranchMembersCheckEachGoalTransition(t *testing.T) {
	bed := newGoalBranchBed(t)
	tipA := buildGoalBranch(t, bed, "goal-a", []string{"1", "2", "3"}, -1)
	memberA, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipA, GoalID: "goal-a", Last: true})
	must(t, err)
	tipB := buildGoalBranch(t, bed, "goal-b", []string{"1"}, -1)
	memberB, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipB, GoalID: "goal-b", Last: true})
	must(t, err)
	prefixes, err := AssembleBranchMembers(bed.root, branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}"), []BranchMember{memberA, memberB})
	if err != nil || len(prefixes) != 2 || branchGit(t, bed.root, "show", prefixes[1]+":metasystem/a-3.txt") != "goal-a/3" || branchGit(t, bed.root, "show", prefixes[1]+":metasystem/b-1.txt") != "goal-b/1" {
		t.Fatalf("two-goal prefixes=%v err=%v", prefixes, err)
	}
}

func TestBatchCompositionMergesTestingContractBySurface(t *testing.T) {
	bed, memberA, memberB := testingContractMembers(t, "member-a", 1100, "member-b", 1200)
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	prefixes, err := AssembleBranchMembers(bed.root, baseTree, []BranchMember{memberA, memberB})
	if err != nil {
		t.Fatal(err)
	}
	merged := decodeBatchContract(t, branchGit(t, bed.root, "show", prefixes[1]+":metasystem/testing.json"))
	if got := batchContractIDs(merged.Groups); !reflect.DeepEqual(got, []string{"base-group", "member-a-group", "member-b-group"}) {
		t.Fatalf("composed groups = %v", got)
	}
	if got := batchContractSurface(t, merged, "residual").Standard; !reflect.DeepEqual(got, []string{"base-group", "member-a-group", "member-b-group"}) {
		t.Fatalf("composed residual groups = %v", got)
	}

}

func TestBatchCompositionRefusesNamedTestingContractConflictCleanly(t *testing.T) {
	bed, memberA, memberB := testingContractMembers(t, "shared", 1100, "shared", 1200)
	t.Setenv("METASYSTEM_CONTRACT_DRIVER_HELPER", "1")
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	_, err := AssembleBranchMembers(bed.root, baseTree, []BranchMember{memberA, memberB})
	if err == nil || !strings.Contains(err.Error(), "BATCH_JOIN_REREAD:") ||
		!strings.Contains(err.Error(), `TESTING_MERGE_CONFLICT: group "shared-group" field targetMs`) {
		t.Fatalf("named testing contract conflict = %v", err)
	}
	if status := branchGit(t, bed.root, "status", "--porcelain=v1", "--untracked-files=all"); status != "" {
		t.Fatalf("refused composition left repository changes: %q", status)
	}
}

func testingContractMembers(t *testing.T, first string, firstTarget int64, second string, secondTarget int64) (goalBranchBed, BranchMember, BranchMember) {
	t.Helper()
	root := t.TempDir()
	branchGit(t, root, "init", "-q", "-b", "main")
	branchGit(t, root, "config", "user.name", "Batch Fixture")
	branchGit(t, root, "config", "user.email", "batch@example.invalid")
	branchWrite(t, root, ".gitattributes", "metasystem/testing.json merge=metasystem-testing\n")
	writeBatchContract(t, root, batchContractFixture())
	branchGit(t, root, "add", ".")
	branchGit(t, root, "commit", "-qm", "base")
	base := branchGit(t, root, "rev-parse", "HEAD")
	member := func(goalID, name string, target int64) BranchMember {
		branchGit(t, root, "reset", "-q", "--hard", base)
		writeBatchContract(t, root, batchContractWithAddition(batchContractFixture(), name, target))
		branchGit(t, root, "add", "metasystem/testing.json")
		branchGit(t, root, "commit", "-qm", goalID)
		commit := branchGit(t, root, "rev-parse", "HEAD")
		digest, err := goalbranch.UnitDigest(root, commit)
		must(t, err)
		return BranchMember{GoalID: goalID, Tip: commit, Builds: []BranchBuild{{Units: []string{"testing"}, Commit: commit, Digest: digest}}}
	}
	memberA := member("goal-a", first, firstTarget)
	memberB := member("goal-b", second, secondTarget)
	return goalBranchBed{root: root, base: base}, memberA, memberB
}

func batchContractFixture() testpolicy.Contract {
	return testpolicy.Contract{SchemaVersion: 1,
		ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Fallback:    "residual",
		Surfaces: []testpolicy.Surface{
			batchContractSurfaceFixture("base", "base-group"),
			{ID: "residual", Paths: []string{}, DependsOn: []string{}, Standard: []string{"base-group"}, Deep: []string{}, Critical: []string{}},
		},
		Groups: []testpolicy.Group{batchContractGroup("base-group", 1000)}, Always: testpolicy.Always{Canary: []string{}, Standard: []string{}},
		Unknown: []string{"base-group"}, Cadence: []string{}}
}

func batchContractWithAddition(contract testpolicy.Contract, name string, target int64) testpolicy.Contract {
	id := name + "-group"
	contract.Groups = append(contract.Groups, batchContractGroup(id, target))
	contract.Surfaces = append(contract.Surfaces, batchContractSurfaceFixture(name, id))
	for index := range contract.Surfaces {
		if contract.Surfaces[index].ID == contract.Fallback {
			contract.Surfaces[index].Standard = append(contract.Surfaces[index].Standard, id)
		}
	}
	contract.Unknown = append(contract.Unknown, id)
	return contract
}

func batchContractGroup(id string, target int64) testpolicy.Group {
	return testpolicy.Group{ID: id, Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod"}, Outputs: []string{},
		Tools: []testpolicy.Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: target,
		Packages: []string{"./example"}, Tests: json.RawMessage(`["TestExample"]`)}
}

func batchContractSurfaceFixture(id, group string) testpolicy.Surface {
	return testpolicy.Surface{ID: id, Paths: []string{id + "/**"}, DependsOn: []string{}, Standard: []string{group}, Deep: []string{}, Critical: []string{}}
}

func writeBatchContract(t *testing.T, root string, contract testpolicy.Contract) {
	t.Helper()
	data, err := contractmerge.Render(contract)
	must(t, err)
	branchWrite(t, root, "metasystem/testing.json", string(data))
}

func decodeBatchContract(t *testing.T, data string) testpolicy.Contract {
	t.Helper()
	contract, err := testpolicy.Decode([]byte(data))
	must(t, err)
	return contract
}

func batchContractIDs(groups []testpolicy.Group) []string {
	ids := make([]string, len(groups))
	for i := range groups {
		ids[i] = groups[i].ID
	}
	return ids
}

func batchContractSurface(t *testing.T, contract testpolicy.Contract, id string) testpolicy.Surface {
	t.Helper()
	for _, surface := range contract.Surfaces {
		if surface.ID == id {
			return surface
		}
	}
	t.Fatalf("surface %s is absent", id)
	return testpolicy.Surface{}
}

func TestBatchBranchMemberRefusesMovedTrunk(t *testing.T) {
	bed := newGoalBranchBed(t)
	tip := buildGoalBranch(t, bed, "goal-a", []string{"1", "2", "3"}, -1)
	member, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tip, GoalID: "goal-a", Last: true})
	must(t, err)
	branchGit(t, bed.root, "switch", "--quiet", "--detach", bed.base)
	must(t, os.Chmod(filepath.Join(bed.root, "metasystem/a-1.txt"), 0o755))
	branchGit(t, bed.root, "add", "metasystem/a-1.txt")
	branchGit(t, bed.root, "commit", "-qm", "moved trunk")
	moved := branchGit(t, bed.root, "rev-parse", "HEAD^{tree}")
	_, err = AssembleBranchMembers(bed.root, moved, []BranchMember{member})
	if err == nil || !strings.HasPrefix(err.Error(), "BATCH_JOIN_REREAD:") {
		t.Fatalf("moved trunk join=%v", err)
	}
}

func TestBatchSealBranchDeletionUsesParentPackage(t *testing.T) {
	bed := newGoalBranchBed(t)
	branchGit(t, bed.root, "switch", "--quiet", "--detach", bed.base)
	must(t, os.Remove(filepath.Join(bed.root, "metasystem", "gone", "value.go")))
	branchGit(t, bed.root, "add", "-u", "metasystem/gone/value.go")
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{
		Repo: bed.root, Remote: bed.root, EndpointTip: bed.base, GoalID: "goal-a", Unit: "delete-package",
		OpID: "build-goal-a-delete-package", Kind: goalbranch.Unit, CheckClaim: func() error { return nil },
	})
	must(t, err)
	tip := branchCriticRead(t, bed, "goal-a", "delete-package", commit, "critic-goal-a-delete-package", false)
	member, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tip, GoalID: "goal-a", Last: true})
	must(t, err)
	unit := BindBranchMember(Unit{GoalID: "goal-a", ChangedPaths: []string{"metasystem/gone/value.go"}}, member)
	sealedTree, err := AssembleBranchMembers(bed.root, branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}"), []BranchMember{member})
	must(t, err)
	var packages []string
	err = runSealGate(bed.root, sealedTree[0], []Unit{unit}, func(_ string, step gateStep) gateStepResult {
		if strings.HasPrefix(step.Name, "package ") {
			packages = append(packages, strings.TrimPrefix(step.Name, "package "))
		}
		if step.Name == "package ./gone" {
			return gateStepResult{RunID: "missing-package", ExitCode: 1}
		}
		return gateStepResult{RunID: "seal-green"}
	})
	if err != nil || !slices.Contains(packages, "./...") || slices.Contains(packages, "./gone") {
		t.Fatalf("seal deletion gate=%v packages=%v", err, packages)
	}
}

func TestBatchBranchAllowsOnlyOneMemberPerGoal(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, scriptedProber{})
	claim := Claim{Machine: "seat", Lineage: "goal-a", Epoch: 1, Revision: 1, AccountingRevision: 1}
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateOpen, Units: []Unit{{GoalID: "goal-a", Chain: "branch-tip-a", Claim: claim, State: UnitJoined}}}))
	err := checkMembership(store, testBatchID, "goal-a", "branch-tip-b")
	if err == nil || !strings.HasPrefix(err.Error(), "BATCH_GOAL_ELSEWHERE:") {
		t.Fatalf("second member=%v", err)
	}
}

func TestBatchGoalEjectionRebuildsLeasedLandingBranch(t *testing.T) {
	bed := newGoalBranchBed(t)
	tipA := buildGoalBranch(t, bed, "goal-a", []string{"1", "2"}, -1)
	memberA, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipA, GoalID: "goal-a", Last: true})
	must(t, err)
	tipB := buildGoalBranch(t, bed, "goal-b", []string{"1"}, -1)
	memberB, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipB, GoalID: "goal-b", Last: true})
	must(t, err)
	origin := filepath.Join(t.TempDir(), "origin.git")
	branchGit(t, filepath.Dir(origin), "init", "-q", "--bare", origin)
	branchGit(t, bed.root, "remote", "add", "origin", origin)
	branchGit(t, bed.root, "push", "-q", "origin", "main")
	claim := Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}
	unitA := BindBranchMember(Unit{GoalID: "goal-a", Chain: tipA, Claim: claim, State: UnitJoined, ChangedPaths: []string{"metasystem/a-1.txt"}, AuthorName: "Approver A", AuthorEmail: "a@example.invalid"}, memberA)
	unitB := BindBranchMember(Unit{GoalID: "goal-b", Chain: tipB, Claim: claim, State: UnitJoined, ChangedPaths: []string{"metasystem/b-1.txt"}, AuthorName: "Approver B", AuthorEmail: "b@example.invalid"}, memberB)
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	oldTip, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, "", "owner", []Unit{unitA, unitB})
	must(t, err)
	prefixes, err := assembleUnits(bed.root, baseTree, []Unit{unitA, unitB})
	must(t, err)
	record := Record{Schema: 1, BatchID: testBatchID, TipTree: prefixes[len(prefixes)-1], State: StateDiagnosing, Units: []Unit{unitA, unitB},
		Proof: &Proof{Status: "failed", AttemptID: "tip-red"}, Landing: &LandingProgress{Base: baseTree, BranchTip: oldTip},
		batchRecordFields: batchRecordFields{BaseTree: baseTree, PrefixTrees: prefixes}}
	store := NewStore(bed.root, nil)
	must(t, store.Create(record))
	must(t, DiagnoseRed(store, testBatchID, "owner", []RedGroup{{ID: "group", InputManifest: []string{"metasystem/a-1.txt"}}}, "", time.Unix(2, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}}))
	updated := load(t, store)
	newTip := branchGit(t, origin, "rev-parse", "refs/heads/landing/"+testBatchID)
	if newTip == oldTip || updated.Landing == nil || updated.Landing.BranchTip != newTip || updated.Units[0].State != UnitReturnPending ||
		!slices.Equal([]string{updated.Units[1].GoalID}, []string{"goal-b"}) {
		t.Fatalf("old=%s new=%s record=%+v", oldTip, newTip, updated)
	}
	if got := branchGit(t, origin, "show", newTip+":metasystem/a-1.txt"); got != "base" {
		t.Fatalf("ejected goal-a bytes remain on rebuilt landing branch: %q", got)
	}
	if got := branchGit(t, origin, "show", newTip+":metasystem/b-1.txt"); got != "goal-b/1" {
		t.Fatalf("survivor bytes=%q", got)
	}
	must(t, DeleteLandingBranch(bed.root, testBatchID, newTip))
	republished, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, newTip, "owner", []Unit{unitB})
	must(t, err)
	if got := branchGit(t, origin, "rev-parse", "refs/heads/landing/"+testBatchID); got != republished {
		t.Fatalf("republished tip=%s, want %s", got, republished)
	}
}
