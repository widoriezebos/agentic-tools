package batch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var evidenceNow = time.Date(2026, 9, 30, 20, 0, 0, 0, time.UTC)

const evidenceBatch = "01j5x00000000000000000kd11"

// evidenceStore holds one hand-off of two joined members and no proof
// attempt at all.
func evidenceStore(t *testing.T) Store {
	t.Helper()
	store := NewStore(t.TempDir(), nil)
	var units []Unit
	for _, member := range []string{"m-red", "m-person"} {
		units = append(units, Unit{GoalID: member, Chain: "chain-" + member, SeatRoot: "/seat", State: UnitJoined,
			Claim: Claim{Machine: "seat", Lineage: "seat-lineage", Epoch: 1, Revision: 2, AccountingRevision: 2}})
	}
	must(t, store.Create(Record{Schema: 1, BatchID: evidenceBatch, State: StateOpen, Units: units}))
	return store
}

// The landing agent decides which member broke the batch: its return needs
// only the member and its reason, never a recorded proof that places the
// red on the member. A person's return needs the proven person. The
// return then stays the existing settlement: return-pending until the
// member's custody is handed back, and a repeat changes nothing.
func TestReturnNeedsOnlyMemberAndReason(t *testing.T) {
	t.Parallel()
	store := evidenceStore(t)
	var refusal *ReturnRefusal
	if _, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionRed, "", " ", "lane:test", evidenceNow); !errors.As(err, &refusal) || refusal.Code != CodeReturnEvidenceMissing {
		t.Fatalf("a return without a reason = %v; want %s", err, CodeReturnEvidenceMissing)
	}
	if _, err := RequestTypedReturn(store, evidenceBatch, "stranger", DispositionRed, "", "red", "lane:test", evidenceNow); !errors.As(err, &refusal) || refusal.Code != CodeReturnMemberAbsent {
		t.Fatalf("a return of a non-member = %v; want %s", err, CodeReturnMemberAbsent)
	}
	evidence, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionRed, "", "app-standard fails since it joined", "lane:test", evidenceNow)
	if err != nil || evidence == "" {
		t.Fatalf("the agent's return with a reason = %q, %v; want it asked for", evidence, err)
	}
	record, err := store.Load(evidenceBatch)
	must(t, err)
	unit := record.Units[0]
	if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || unit.Disposition != DispositionRed || !strings.Contains(unit.Failure, "app-standard fails since it joined") {
		t.Fatalf("the returned member = %+v; want it return-pending as red with its reason", unit)
	}
	if again, err := RequestTypedReturn(store, evidenceBatch, "m-red", DispositionRed, "", "app-standard fails since it joined", "lane:test", evidenceNow); err != nil || again != evidence {
		t.Fatalf("a repeated return = %q, %v; want the same evidence", again, err)
	}
	if _, err := RequestTypedReturn(store, evidenceBatch, "m-person", DispositionPerson, "", "the design changes", "lane:test", evidenceNow); !errors.As(err, &refusal) || refusal.Code != CodeReturnEvidenceMissing {
		t.Fatalf("a person's return without a person = %v; want %s", err, CodeReturnEvidenceMissing)
	}
	if evidence, err := RequestTypedReturn(store, evidenceBatch, "m-person", DispositionPerson, "Wido", "", "Wido", evidenceNow); err != nil || evidence != "person Wido" {
		t.Fatalf("a person's return = %q, %v; want it on the person's word", evidence, err)
	}
}
