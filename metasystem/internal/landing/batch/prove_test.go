package batch

import (
	"strings"
	"testing"
	"time"
)

func TestBatchDiagnosticRefusalHoldsUnclassified(t *testing.T) {
	const id = "01j5x00000000000000000ba12"
	store := NewStore(t.TempDir(), nil)
	claim := Claim{Machine: "seat", Lineage: "lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}
	record := Record{Schema: 1, BatchID: id, State: StateDiagnosing, Proof: &Proof{Status: "failed"},
		Units: []Unit{{GoalID: "goal-k", Chain: "chain-k", Claim: claim, State: UnitJoined}}}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	if err := HoldUnclassified(store, id, "owner", "BATCH_MEMBER_BUDGET_REFUSED", "Gk Next: raise proof budget", time.Unix(9, 0)); err != nil {
		t.Fatal(err)
	}
	held, err := store.Load(id)
	if err != nil || held.State != StateHeldUnclassified || held.Proof == nil || held.Proof.Status != "held-unclassified" ||
		!strings.Contains(held.Proof.Failure, "Gk Next") || held.Units[0].State != UnitJoined {
		t.Fatalf("held=%+v err=%v", held, err)
	}
}
