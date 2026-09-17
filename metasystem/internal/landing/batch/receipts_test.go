package batch

import (
	"errors"
	"slices"
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
	err = DiagnoseRed(store, testBatchID, "owner", prefixRed.Groups, prefixRed.GoalID, time.Unix(4, 0), RedSeams{Run: func(DiagnosticRequest) (DiagnosticResult, error) {
		return DiagnosticResult{AttemptID: "base-green"}, nil
	}})
	must(t, err)
	record := load(t, store)
	if record.Units[0].State != UnitReturnPending || record.Units[1].State != UnitJoined || record.State != StateOpen {
		t.Fatalf("prefix red classification=%+v", record)
	}
}
