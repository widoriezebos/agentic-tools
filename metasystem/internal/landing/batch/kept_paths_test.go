package batch

import (
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// These tests hold the batch code the landing lane's kernel keeps (join,
// change membership, survivor reassembly and the landing branch's
// transport) directly, without the deleted batch owner that used to reach
// it through its pass (design r10 §5, unit D).

// TestChangeJoinsTheOpenBatchAsOneCarriedMember (U11b): a change joins the
// open batch that holds a goal member, as one joined member with the asker's
// seat and no claim revisions and its own tree as its carried admission; a
// repeat changes nothing; the batch's proofs are charged to the goal member.
func TestChangeJoinsTheOpenBatchAsOneCarriedMember(t *testing.T) {
	t.Parallel()
	base := testCommit(101)
	store := NewStore(t.TempDir(), nil)
	strictReassembly(t, &store,
		expectedAssembly(base, []string{"goal-a", "change:abcdef012345"}, []string{"chain-a", "change:abcdef012345"}, []string{testCommit(103), testCommit(104)}),
		expectedAssembly(base, []string{"change:abcdef012345"}, []string{"change:abcdef012345"}, []string{testCommit(105)}),
	)
	must(t, store.Create(openBatchWithGoalMember(base)))
	join := ChangeJoin{Unit: changeMemberUnit(), BaseTree: base, NewID: "01j5x00000000000000000ba99", Actor: "m1e+human", At: ten.Add(time.Minute)}
	record, err := JoinChange(store, join)
	must(t, err)
	if record.BatchID != testBatchID || len(record.Units) != 2 || !slices.Equal(record.PrefixTrees, []string{testCommit(103), testCommit(104)}) || record.TipTree != testCommit(104) {
		t.Fatalf("joined record=%+v", record)
	}
	unit := record.Units[1]
	if unit.GoalID != "change:abcdef012345" || unit.Chain != unit.GoalID || unit.State != UnitJoined || !unit.IsChange() ||
		unit.Claim != (Claim{Machine: "m1e", Lineage: "human"}) || unit.Admission == nil || unit.Admission.Tree != testCommit(105) || unit.Admission.Status != AdmissionCarried {
		t.Fatalf("change member=%+v admission=%+v", unit, unit.Admission)
	}
	again, err := JoinChange(store, join)
	must(t, err)
	if len(again.History) != len(record.History) || len(again.Units) != 2 {
		t.Fatalf("repeat changed the record: %+v", again)
	}
	if charge, ok := ChargeMember(joinedUnits(again.Units)); !ok || charge.GoalID != "goal-a" {
		t.Fatalf("charge member=%+v ok=%v", charge, ok)
	}
	if charge := ChargeUnit(again.Units); charge.GoalID != "goal-a" {
		t.Fatalf("charge unit=%+v", charge)
	}
	// A batch of changes alone is charged to its last change; no units, to
	// nobody.
	if _, ok := ChargeMember([]Unit{unit}); ok {
		t.Fatal("a change was taken as a goal member")
	}
	if charge := ChargeUnit([]Unit{unit}); charge.GoalID != unit.GoalID {
		t.Fatalf("change-only charge unit=%+v", charge)
	}
	if charge := ChargeUnit(nil); charge.GoalID != "" {
		t.Fatalf("empty charge unit=%+v", charge)
	}
}

// TestChangeOpensABatchWhenNoneIsOpen: with no open batch the change opens
// one on the join's base under the join's id; an incomplete change member is
// refused before anything is read.
func TestChangeOpensABatchWhenNoneIsOpen(t *testing.T) {
	t.Parallel()
	base := testCommit(201)
	store := NewStore(t.TempDir(), nil)
	strictReassembly(t, &store,
		expectedAssembly(base, []string{"change:abcdef012345"}, []string{"change:abcdef012345"}, []string{testCommit(202)}),
		expectedAssembly(base, []string{"change:abcdef012345"}, []string{"change:abcdef012345"}, []string{testCommit(202)}),
	)
	record, err := JoinChange(store, ChangeJoin{Unit: changeMemberUnit(), BaseTree: base, NewID: testBatchID, Actor: "m1e+human", At: ten})
	must(t, err)
	stored := load(t, store)
	if record.BatchID != testBatchID || stored.State != StateOpen || stored.BaseTree != base || stored.TipTree != testCommit(202) ||
		len(stored.Units) != 1 || stored.Units[0].Admission == nil || stored.Units[0].Admission.Tree != testCommit(202) {
		t.Fatalf("opened record=%+v stored=%+v", record, stored)
	}
	incomplete := changeMemberUnit()
	incomplete.Claim.Lineage = ""
	if _, err := JoinChange(store, ChangeJoin{Unit: incomplete, BaseTree: base, NewID: "other", Actor: "m1e+human", At: ten}); err == nil ||
		!strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("incomplete change join=%v", err)
	}
}

// TestFindOrCreateOpenPrefersTheBatchOnTheBase: an open batch on the join's
// base is chosen over a newer one on another base; with none open a batch
// is created on the base, and a repeat finds it.
func TestFindOrCreateOpenPrefersTheBatchOnTheBase(t *testing.T) {
	t.Parallel()
	store := NewStore(t.TempDir(), nil)
	created, err := FindOrCreateOpen(store, testCommit(301), "01j5x00000000000000000b301", "lane", ten)
	must(t, err)
	if created.BatchID != "01j5x00000000000000000b301" || created.State != StateOpen || created.TipTree != testCommit(301) {
		t.Fatalf("created=%+v", created)
	}
	again, err := FindOrCreateOpen(store, testCommit(302), "01j5x00000000000000000b302", "lane", ten)
	must(t, err)
	if again.BatchID != created.BatchID {
		t.Fatalf("an open batch on another base was not joined: %+v", again)
	}
	records, err := store.Records()
	must(t, err)
	if len(records) != 1 {
		t.Fatalf("records=%d", len(records))
	}
}

// TestHoldReasonNamesOnlyAHoldOrARefusedProof: a batch whose last word is a
// hold or a refused proof admission says why; any other last word says
// nothing.
func TestHoldReasonNamesOnlyAHoldOrARefusedProof(t *testing.T) {
	t.Parallel()
	for verb, want := range map[string]string{"hold": "held because", "prove-refused": "held because", "join": ""} {
		record := Record{History: []HistoryEntry{{Verb: "open"}, {Verb: verb, Detail: "held because"}}}
		if got := HoldReason(record); got != want {
			t.Errorf("%s: HoldReason=%q want %q", verb, got, want)
		}
	}
	if got := HoldReason(Record{}); got != "" {
		t.Fatalf("no history: %q", got)
	}
}

func reassemblyUnits() (Claim, Unit, Unit) {
	claim := Claim{Machine: "landing", Lineage: "landing-lane", Epoch: 1, Revision: 1, AccountingRevision: 1}
	unitA := Unit{GoalID: "goal-a", Chain: testCommit(1006), BranchTip: testCommit(1006), Claim: claim, State: UnitJoined,
		ChangedPaths: []string{"metasystem/a-1.txt"}, Builds: []BranchBuild{{Commit: testCommit(1008)}}}
	unitB := Unit{GoalID: "goal-b", Chain: testCommit(1007), BranchTip: testCommit(1007), Claim: claim, State: UnitJoined,
		ChangedPaths: []string{"metasystem/b-1.txt"}, Builds: []BranchBuild{{Commit: testCommit(1009)}}}
	return claim, unitA, unitB
}

// TestReturningAMemberRebuildsTheLeasedLandingBranch: a member returned
// from a batch whose landing branch is published leaves the survivors
// composed on the base and the branch rebuilt under the old tip's lease, in
// one record update.
func TestReturningAMemberRebuildsTheLeasedLandingBranch(t *testing.T) {
	t.Parallel()
	base, prefixA, prefixB, oldTip, newTip := testCommit(1001), testCommit(1002), testCommit(1003), testCommit(1004), testCommit(1005)
	_, unitA, unitB := reassemblyUnits()
	store := NewStore(t.TempDir(), nil)
	strictReassembly(t, &store,
		expectedAssembly(base, []string{"goal-b"}, []string{unitB.Chain}, []string{testCommit(1010)}),
		expectedReassembly{kind: "rebuild", batchID: testBatchID, base: base, leaseTip: oldTip, actor: "lane",
			goals: []string{"goal-b"}, chains: []string{unitB.Chain}, tip: newTip})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: prefixB, State: StateOpen, Units: []Unit{unitA, unitB},
		Landing: &LandingProgress{Base: base, BranchTip: oldTip}, batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{prefixA, prefixB}}}))
	must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "lane", time.Unix(2, 0), []ReturnDecision{{GoalID: "goal-a", Outcome: UnitEjected, Reason: "EJECTED: red"}}))
	updated := load(t, store)
	if updated.State != StateOpen || updated.TipTree != testCommit(1010) || !slices.Equal(updated.PrefixTrees, []string{testCommit(1010)}) ||
		updated.Landing == nil || updated.Landing.BranchTip != newTip || updated.Landing.Base != base ||
		updated.Units[0].State != UnitReturnPending || updated.Units[0].Outcome != UnitEjected || updated.Units[0].Failure != "EJECTED: red" ||
		updated.Units[1].State != UnitJoined || updated.History[len(updated.History)-1].To != StateOpen {
		t.Fatalf("record=%+v", updated)
	}
}

// TestReassembleSurvivorsKeepsChainOnlyAndMixedBatches: survivors of chains
// alone delete the published landing branch; survivors with a branch member
// rebuild it, in original join order.
func TestReassembleSurvivorsKeepsChainOnlyAndMixedBatches(t *testing.T) {
	t.Parallel()
	for _, mixed := range []bool{false, true} {
		name := "chain only"
		if mixed {
			name = "chain and branch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, prefixA, prefixOne, oldTip := testCommit(1101), testCommit(1102), testCommit(1103), testCommit(1104)
			prefixTwo, survivorOne, survivorTwo, newTip := testCommit(1105), testCommit(1106), testCommit(1107), testCommit(1108)
			claim, unitA, unitB := reassemblyUnits()
			chainOne := Unit{GoalID: "goal-chain-one", Chain: "chain-one", Claim: claim, State: UnitJoined}
			chainTwo := Unit{GoalID: "goal-chain-two", Chain: "chain-two", Claim: claim, State: UnitJoined}
			units := []Unit{unitA, chainOne, chainTwo}
			expected := []expectedReassembly{
				expectedAssembly(base, []string{"goal-chain-one", "goal-chain-two"}, []string{"chain-one", "chain-two"}, []string{survivorOne, survivorTwo}),
				{kind: "delete", batchID: testBatchID, leaseTip: oldTip},
			}
			if mixed {
				units = []Unit{unitA, chainOne, unitB}
				expected = []expectedReassembly{
					expectedAssembly(base, []string{"goal-chain-one", "goal-b"}, []string{"chain-one", unitB.Chain}, []string{survivorOne, survivorTwo}),
					{kind: "rebuild", batchID: testBatchID, base: base, leaseTip: oldTip, actor: "lane",
						goals: []string{"goal-chain-one", "goal-b"}, chains: []string{"chain-one", unitB.Chain}, tip: newTip},
				}
			}
			store := NewStore(t.TempDir(), nil)
			strictReassembly(t, &store, expected...)
			must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: prefixTwo, State: StateOpen, Units: units,
				Landing:           &LandingProgress{Base: base, BranchTip: oldTip},
				batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{prefixA, prefixOne, prefixTwo}}}))
			must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "fixture ejection", "lane", time.Unix(2, 0)))
			must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "lane", time.Unix(3, 0), nil))
			updated := load(t, store)
			if updated.State != StateOpen || updated.TipTree != survivorTwo || !slices.Equal(updated.PrefixTrees, []string{survivorOne, survivorTwo}) ||
				updated.Units[0].State != UnitReturnPending || updated.Units[1].State != UnitJoined {
				t.Fatalf("reassembled record=%+v", updated)
			}
			if mixed && (updated.Landing == nil || updated.Landing.BranchTip != newTip) {
				t.Fatalf("mixed branch=%+v", updated.Landing)
			}
			if !mixed && updated.Landing != nil {
				t.Fatalf("chain survivors kept a landing branch: %+v", updated.Landing)
			}
		})
	}
}

// TestReassemblyReturnsTheSurvivorThatNoLongerApplies: a survivor whose
// composition conflicts after another member left is returned too, naming
// what left; when nobody is left the batch dissolves and its published
// branch is deleted.
func TestReassemblyReturnsTheSurvivorThatNoLongerApplies(t *testing.T) {
	t.Parallel()
	base, oldTip := testCommit(1201), testCommit(1202)
	_, unitA, unitB := reassemblyUnits()
	conflict := &assemblyConflict{GoalID: "goal-b", Paths: []string{"metasystem/b-1.txt"}, Cause: errors.New("BATCH_JOIN_CONFLICT: b-1")}
	store := NewStore(t.TempDir(), nil)
	strictReassembly(t, &store,
		expectedReassembly{kind: "assemble", base: base, goals: []string{"goal-b"}, chains: []string{unitB.Chain}, err: conflict},
		expectedReassembly{kind: "delete", batchID: testBatchID, leaseTip: oldTip})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: testCommit(1203), State: StateOpen, Units: []Unit{unitA, unitB},
		Landing: &LandingProgress{Base: base, BranchTip: oldTip}, batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(1204), testCommit(1203)}}}))
	must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "lane", time.Unix(2, 0), []ReturnDecision{{GoalID: "goal-a", Outcome: UnitEjected, Reason: "EJECTED: red"}}))
	updated := load(t, store)
	if updated.State != StateDissolved || updated.TipTree != base || updated.PrefixTrees != nil || updated.Landing != nil ||
		updated.Units[1].State != UnitReturnPending || !strings.Contains(updated.Units[1].Failure, "cannot apply after returning goal-a") {
		t.Fatalf("record=%+v", updated)
	}
}

// TestReassemblyHoldsWhatItCannotClassify: a survivor composition that
// fails without a member to blame, or a landing branch that cannot be
// rebuilt, holds the batch unclassified with the reason and the next step,
// never guessing a member to return.
func TestReassemblyHoldsWhatItCannotClassify(t *testing.T) {
	t.Parallel()
	base, oldTip := testCommit(1301), testCommit(1302)
	_, unitA, unitB := reassemblyUnits()
	for name, expected := range map[string][]expectedReassembly{
		"unclassified composition": {{kind: "assemble", base: base, goals: []string{"goal-b"}, chains: []string{unitB.Chain}, err: errors.New("git apply: broken")}},
		"composition of nobody":    {{kind: "assemble", base: base, goals: []string{"goal-b"}, chains: []string{unitB.Chain}, err: &assemblyConflict{GoalID: "goal-z", Cause: errors.New("conflict")}}},
		"branch rebuild refused": {
			expectedAssembly(base, []string{"goal-b"}, []string{unitB.Chain}, []string{testCommit(1303)}),
			{kind: "rebuild", batchID: testBatchID, base: base, leaseTip: oldTip, actor: "lane", goals: []string{"goal-b"}, chains: []string{unitB.Chain}, err: errors.New("lease refused")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			store := NewStore(t.TempDir(), nil)
			strictReassembly(t, &store, expected...)
			must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, TipTree: testCommit(1304), State: StateOpen, Units: []Unit{unitA, unitB},
				Landing: &LandingProgress{Base: base, BranchTip: oldTip}, batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(1305), testCommit(1304)}}}))
			must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "lane", time.Unix(2, 0), []ReturnDecision{{GoalID: "goal-a", Outcome: UnitEjected, Reason: "EJECTED: red"}}))
			updated := load(t, store)
			if updated.State != StateHeldUnclassified || updated.Proof == nil || updated.Proof.Status != "held-unclassified" ||
				!strings.Contains(updated.Proof.Failure, "inspect ordered member composition") || updated.Units[1].State != UnitJoined {
				t.Fatalf("record=%+v proof=%+v", updated, updated.Proof)
			}
		})
	}
}

// landingRemote gives a goal-branch bed an origin whose main is the bed's
// base, as the lane's checkout sees it.
func landingRemote(t *testing.T, bed goalBranchBed) string {
	t.Helper()
	origin := filepath.Join(t.TempDir(), "origin.git")
	branchGit(t, filepath.Dir(origin), "init", "-q", "--bare", origin)
	branchGit(t, bed.root, "remote", "add", "origin", origin)
	branchGit(t, bed.root, "push", "-q", "origin", bed.base+":refs/heads/main")
	return origin
}

// changeOnBase is a change member's commit made on the bed's base: one
// file written, with the asker's trailers.
func changeOnBase(t *testing.T, bed goalBranchBed, path, body string) Unit {
	t.Helper()
	branchGit(t, bed.root, "switch", "--quiet", "--detach", bed.base)
	branchWrite(t, bed.root, path, body)
	branchGit(t, bed.root, "add", path)
	branchGit(t, bed.root, "-c", "user.name=Asker", "-c", "user.email=asker@example.invalid", "commit", "-qm", "record: notes\n\nMachine: m1e+human")
	commit := branchGit(t, bed.root, "rev-parse", "HEAD")
	unit := NewChangeUnit(ChangeMember{Commit: commit, Parent: bed.base, AskedBy: "m1e+human", Subject: "record: notes"},
		bed.root, "m1e", "human", []string{path}, nil)
	unit.State = UnitJoined
	return unit
}

// TestLandingBranchRebuildCarriesChangesAndSurvivesItsOwnRepeat drives the
// kept landing-branch transport against real repositories: goal builds and
// a change replay as distinct commits in join order (the change's with its
// Landing-Change trailer and the asker's identity); a survivor rebuild
// replaces the branch under the old tip's lease; a rebuild whose lease is
// stale adopts a published branch of the same tree and refuses one of
// another; deleting the branch is safe to repeat; and a branch deleted is
// published again from nothing.
func TestLandingBranchRebuildCarriesChangesAndSurvivesItsOwnRepeat(t *testing.T) {
	bed := newGoalBranchBed(t)
	tipA := buildGoalBranch(t, bed, "goal-a", []string{"1"}, -1)
	memberA, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipA, GoalID: "goal-a", Last: true})
	must(t, err)
	tipB := buildGoalBranch(t, bed, "goal-b", []string{"1"}, -1)
	memberB, err := ReadGoalBranch(BranchReadRequest{Repo: bed.root, EndpointTip: bed.base, BranchTip: tipB, GoalID: "goal-b", Last: true})
	must(t, err)
	change := changeOnBase(t, bed, "metasystem/records/notes.md", "notes\n")
	remote := landingRemote(t, bed)
	claim := Claim{Machine: "landing", Lineage: "landing-lane", Epoch: 1, Revision: 1, AccountingRevision: 1}
	unitA := BindBranchMember(Unit{GoalID: "goal-a", Chain: tipA, Claim: claim, State: UnitJoined, AuthorName: "Approver A", AuthorEmail: "a@example.invalid"}, memberA)
	unitB := BindBranchMember(Unit{GoalID: "goal-b", Chain: tipB, Claim: claim, State: UnitJoined, AuthorName: "Approver B", AuthorEmail: "b@example.invalid"}, memberB)
	baseTree := branchGit(t, bed.root, "rev-parse", bed.base+"^{tree}")
	ref := "refs/heads/landing/" + testBatchID
	t.Setenv("GIT_COMMITTER_DATE", "2001-01-01T00:00:00Z")

	oldTip, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, "", "lane:landing", []Unit{unitA, change})
	must(t, err)
	if got := branchGit(t, remote, "rev-parse", ref); got != oldTip {
		t.Fatalf("published tip=%s want %s", got, oldTip)
	}
	message := branchGit(t, bed.root, "show", "-s", "--format=%B", oldTip)
	if !strings.Contains(message, "Machine: m1e+human\nLanding-Change: "+change.GoalID) ||
		branchGit(t, bed.root, "show", "-s", "--format=%an|%ae", oldTip) != "Asker|asker@example.invalid" ||
		branchGit(t, bed.root, "show", oldTip+":metasystem/records/notes.md") != "notes" ||
		branchGit(t, bed.root, "show", oldTip+":metasystem/a-1.txt") != "goal-a/1" {
		t.Fatalf("change replay %s:\n%s", oldTip, message)
	}
	if branchGit(t, bed.root, "show", "-s", "--format=%an", oldTip+"^") != "Approver A" {
		t.Fatal("the goal member's build was not the change's parent")
	}

	survivorTip, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, oldTip, "lane:landing", []Unit{unitB, change})
	must(t, err)
	if got := branchGit(t, remote, "rev-parse", ref); got != survivorTip || survivorTip == oldTip ||
		branchGit(t, remote, "show", survivorTip+":metasystem/a-1.txt") != "base" || branchGit(t, remote, "show", survivorTip+":metasystem/b-1.txt") != "goal-b/1" {
		t.Fatalf("survivor tip=%s remote=%s", survivorTip, got)
	}

	// A process that died after publishing still names the old tip: the
	// stale lease adopts the published branch of the same tree.
	t.Setenv("GIT_COMMITTER_DATE", "2001-01-01T00:00:01Z")
	adopted, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, oldTip, "lane:landing", []Unit{unitB, change})
	must(t, err)
	if adopted != survivorTip {
		t.Fatalf("adopted=%s want the published %s", adopted, survivorTip)
	}
	if _, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, oldTip, "lane:landing", []Unit{unitB}); err == nil || !IsStaleEndpointLease(err) {
		t.Fatalf("a stale lease over another tree=%v", err)
	}
	if got := branchGit(t, remote, "rev-parse", ref); got != survivorTip {
		t.Fatalf("a refused rebuild moved the branch to %s", got)
	}

	must(t, DeleteLandingBranch(bed.root, testBatchID, survivorTip))
	if err := exec.Command("git", "--git-dir", remote, "show-ref", "--verify", "--quiet", ref).Run(); err == nil {
		t.Fatalf("branch %s survived its deletion", ref)
	}
	must(t, DeleteLandingBranch(bed.root, testBatchID, survivorTip))
	must(t, DeleteLandingBranch(bed.root, testBatchID, ""))
	republished, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, survivorTip, "lane:landing", []Unit{unitB})
	must(t, err)
	if got := branchGit(t, remote, "rev-parse", ref); got != republished {
		t.Fatalf("republished tip=%s want %s", got, republished)
	}

	if _, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, republished, "lane:landing", nil); err == nil || !strings.Contains(err.Error(), "no surviving builds") {
		t.Fatalf("no survivors=%v", err)
	}
	if _, err := RebuildLandingBranch(bed.root, testBatchID, testCommit(9), republished, "lane:landing", []Unit{unitB}); err == nil ||
		!strings.Contains(err.Error(), codeLandTrunkMoved) {
		t.Fatalf("a base main never had=%v", err)
	}
	conflicting := changeOnBase(t, bed, "metasystem/a-1.txt", "the asker's\n")
	var conflict *assemblyConflict
	if _, err := RebuildLandingBranch(bed.root, testBatchID, baseTree, republished, "lane:landing", []Unit{unitA, conflicting}); !errors.As(err, &conflict) ||
		conflict.GoalID != conflicting.GoalID || !slices.Equal(conflict.Paths, []string{"metasystem/a-1.txt"}) {
		t.Fatalf("a change that does not apply on the series=%v", err)
	}
}

// TestCleanupLandingBranchLeavesNoBranchAndRepeats: after the push the
// checkout is detached at the pushed tip and the local landing branch is
// gone; a repeat succeeds; a tip the checkout cannot reach is refused with
// its code.
func TestCleanupLandingBranchLeavesNoBranchAndRepeats(t *testing.T) {
	t.Parallel()
	bed := newGoalBranchBed(t)
	branchGit(t, bed.root, "switch", "-q", "-c", "landing/"+testBatchID)
	must(t, CleanupLandingBranch(bed.root, testBatchID, bed.base))
	if head := branchGit(t, bed.root, "rev-parse", "HEAD"); head != bed.base {
		t.Fatalf("head=%s", head)
	}
	if err := exec.Command("git", "-C", bed.root, "show-ref", "--verify", "--quiet", "refs/heads/landing/"+testBatchID).Run(); err == nil {
		t.Fatal("the local landing branch survived its cleanup")
	}
	must(t, CleanupLandingBranch(bed.root, testBatchID, bed.base))
	if err := CleanupLandingBranch(bed.root, testBatchID, testCommit(7)); err == nil || !strings.Contains(err.Error(), "BATCH_LAND_CLEANUP_REFUSED") {
		t.Fatalf("an unreachable tip=%v", err)
	}
}

// The push is classified from git's --porcelain ref status lines on stdout
// (flag, refs, summary), never from its human text on stderr.
func TestEndpointPushErrorClassifiesOnlyStaleInfoAsLease(t *testing.T) {
	t.Parallel()
	stale := classifyEndpointPushError([]byte("To origin\n!\trefs/heads/main:refs/heads/main\t[rejected] (stale info)\nDone\n"), errors.New("push failed"))
	if !IsStaleEndpointLease(stale) || stale.Error() != "push failed" || !errors.Is(stale, errors.Unwrap(stale)) {
		t.Fatalf("stale classification=%T %v", stale, stale)
	}
	hook := classifyEndpointPushError([]byte("To origin\n!\trefs/heads/main:refs/heads/main\t[remote rejected] (pre-receive hook declined)\nDone\n"), errors.New("push failed"))
	var push *EndpointPushError
	if IsStaleEndpointLease(hook) || !errors.As(hook, &push) || !push.RemoteRejected {
		t.Fatalf("hook classification=%T %v", hook, hook)
	}
	words := classifyEndpointPushError(nil, errors.New("push failed: ! [rejected] main -> main (stale info)"))
	if IsStaleEndpointLease(words) || !errors.As(words, &push) || push.RemoteRejected {
		t.Fatalf("words outside the porcelain classified as %T %v", words, words)
	}
}

// TestProvingWaitLastsUntilTheStateMoves: a batch waits for the host's
// proving flock from its last proving-wait entry until a state change
// follows it.
func TestProvingWaitLastsUntilTheStateMoves(t *testing.T) {
	t.Parallel()
	waiting := Record{History: []HistoryEntry{{Verb: "open", To: StateOpen}, {Verb: ProvingWaitVerb, From: StateSealed, To: StateSealed, Detail: "host proving"},
		{Verb: "note", From: StateSealed, To: StateSealed}}}
	if entry, ok := ProvingWait(waiting); !ok || entry.Detail != "host proving" {
		t.Fatalf("waiting=%+v %v", entry, ok)
	}
	moved := waiting
	moved.History = append(slices.Clone(waiting.History), HistoryEntry{Verb: "prove", From: StateSealed, To: StateProving})
	if _, ok := ProvingWait(moved); ok {
		t.Fatal("a batch whose state moved still waits")
	}
	if _, ok := ProvingWait(Record{History: []HistoryEntry{{Verb: "open", From: "", To: StateOpen}}}); ok {
		t.Fatal("a batch that never waited waits")
	}
}

// TestHelmHeldSeatNamesTheFirstSeatAtTheHelm: a batch is held whole by the
// first member whose seat is at the helm; no active seat, or no way to ask,
// holds nothing.
func TestHelmHeldSeatNamesTheFirstSeatAtTheHelm(t *testing.T) {
	t.Parallel()
	record := Record{Units: []Unit{{GoalID: "goal-a"}, {GoalID: "goal-b", SeatRoot: "/seats/m1b"}, {GoalID: "goal-c", SeatRoot: "/seats/m1c"}}}
	active := func(root string) bool { return root != "/seats/m1b" }
	if seat, held := HelmHeldSeat(record, active); !held || seat != "/seats/m1c" {
		t.Fatalf("held=%v seat=%q", held, seat)
	}
	if _, held := HelmHeldSeat(record, nil); held {
		t.Fatal("no helm check held the batch")
	}
	if _, held := HelmHeldSeat(record, func(string) bool { return false }); held {
		t.Fatal("no seat at the helm held the batch")
	}
}
