package batch

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"
)

func prefixReceiptBed(t *testing.T) (assemblyBed, Store) {
	t.Helper()
	bed := assemblyFixture(t)
	prefixes, err := assembleUnits(bed.root, bed.base, bed.record.Units)
	must(t, err)
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, prefixes, prefixes[len(prefixes)-1]
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip-attempt", SelectedGroups: []string{"same", "different"}}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

func TestPrefixReceiptsReuseByIdentity(t *testing.T) {
	bed, store := prefixReceiptBed(t)
	var goal, tree string
	var groups []string
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(gotGoal, gotTree string, gotGroups []string) (PrefixRunResult, error) {
		goal, tree, groups = gotGoal, gotTree, gotGroups
		return PrefixRunResult{AttemptID: "prefix-attempt", Executed: []string{"different"}, Reused: map[string]string{"same": "tip-attempt"}}, nil
	}})
	must(t, err)
	receipt := load(t, store).Receipts["goal-a"]
	if goal != "goal-a" || tree != bed.record.PrefixTrees[0] || !slices.Equal(groups, []string{"same", "different"}) ||
		receipt.Tree != bed.record.PrefixTrees[0] || receipt.AttemptID != "prefix-attempt" || !slices.Equal(receipt.Executed, []string{"different"}) || receipt.Reused["same"] != "tip-attempt" {
		t.Fatalf("request=%s/%s/%v receipt=%+v", goal, tree, groups, receipt)
	}

	_, reusedStore := prefixReceiptBed(t)
	must(t, ComposePrefixReceipts(reusedStore, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{Reused: map[string]string{"same": "tip-attempt", "different": "tip-attempt"}}, nil
	}}))
	fullyReused := load(t, reusedStore).Receipts["goal-a"]
	if fullyReused.AttemptID != "" || len(fullyReused.Executed) != 0 {
		t.Fatalf("fully reused prefix created an attempt: %+v", fullyReused)
	}
}

func TestPrefixRevisionMoveReturnsForRevision(t *testing.T) {
	_, store := prefixReceiptBed(t)
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{}, &PrefixRevisionRefusal{Reason: "GOAL_REVISION_MOVED: goal-a changed after seal"}
	}})
	if err != nil {
		t.Fatal(err)
	}
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitEjected {
		t.Fatalf("revision movement was not returned as a revision refusal: %+v", record.Units[0])
	}
}

func TestPrefixRedEjectsItsUnit(t *testing.T) {
	_, store := prefixReceiptBed(t)
	red := []RedGroup{{ID: "different", InputManifest: []string{"a.go"}}}
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{AttemptID: "prefix-red", Red: red}, nil
	}})
	var prefixRed *PrefixRedError
	if !errors.As(err, &prefixRed) || prefixRed.GoalID != "goal-a" {
		t.Fatalf("prefix red=%v", err)
	}
	stored := load(t, store)
	if len(stored.Proof.RedGroups) != 1 || stored.Proof.RedGroups[0].ID != red[0].ID ||
		!slices.Equal(stored.Proof.RedGroups[0].InputManifest, red[0].InputManifest) || stored.Proof.Failure != "goal-a" || stored.Proof.PrefixGoal != "goal-a" {
		t.Fatalf("stored prefix diagnosis inputs=%+v", stored.Proof)
	}
	err = DiagnoseRed(store, testBatchID, "owner", stored.Proof.RedGroups, stored.Proof.PrefixGoal, time.Unix(4, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}})
	must(t, err)
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitJoined || record.State != StateOpen {
		t.Fatalf("prefix red classification=%+v", record)
	}
}

func TestPrefixRedPersistenceFailureIsReturned(t *testing.T) {
	_, store := prefixReceiptBed(t)
	store.seams.flock = func(int, int) error { return errors.New("prefix record unavailable") }
	err := ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{Red: []RedGroup{{ID: "different", InputManifest: []string{"a.go"}}}}, nil
	}})
	var prefixRed *PrefixRedError
	if err == nil || errors.As(err, &prefixRed) || !strings.Contains(err.Error(), "persist prefix red") || !strings.Contains(err.Error(), "prefix record unavailable") {
		t.Fatalf("prefix persistence failure=%T %v", err, err)
	}
}

func TestBatchMemberRecordReceiptAndNextCarryIdentity(t *testing.T) {
	bed := assemblyFixture(t)
	member := BranchMember{GoalID: "goal-a", Tip: "branch-tip", Builds: []BranchBuild{
		{Units: []string{"8", "9"}, Commit: "commit-89"},
		{Units: []string{"10a", "10b"}, Commit: "commit-10"},
	}}
	bed.record.Units[0] = BindBranchMember(bed.record.Units[0], member)
	bed.record.State = StateLanding
	bed.record.PrefixTrees = []string{"prefix-a", "prefix-b"}
	bed.record.TipTree = "prefix-b"
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip", SelectedGroups: []string{"group"}}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	must(t, ComposePrefixReceipts(store, testBatchID, "owner", time.Unix(3, 0), PrefixReceiptSeams{Execute: func(string, string, []string) (PrefixRunResult, error) {
		return PrefixRunResult{AttemptID: "prefix"}, nil
	}}))
	record := load(t, store)
	receipt := record.Receipts["goal-a"]
	if !slices.Equal(record.Units[0].CommitIDs, []string{"commit-89", "commit-10"}) || record.Units[0].LastUnit != "10b" ||
		!slices.Equal(receipt.CommitIDs, record.Units[0].CommitIDs) || receipt.LastUnit != "10b" || !slices.Equal(receipt.Units, []string{"8", "9", "10a", "10b"}) {
		t.Fatalf("record=%+v receipt=%+v", record.Units[0], receipt)
	}
	must(t, RequestReturn(store, testBatchID, "goal-a", UnitEjected, "attempt=red groups=group", "owner", time.Unix(4, 0)))
	var next string
	must(t, ReturnUnits(store, testBatchID, "tree", "owner", time.Unix(5, 0), ReturnSeams{
		Read: func(string, string, string) (ReturnLedgerGoal, error) {
			return ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}, nil
		},
		Target:  func(Unit) ReturnTarget { return ReturnTarget{State: ReturnTargetDead} },
		Release: func(_ string, value string) error { next = value; return nil },
	}))
	if !strings.Contains(next, "goal-a through 10b") || !strings.Contains(next, "attempt=red groups=group") {
		t.Fatalf("Next=%q", next)
	}
}
