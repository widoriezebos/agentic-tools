package batch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
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
