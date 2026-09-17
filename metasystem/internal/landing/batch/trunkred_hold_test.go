package batch

import (
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type countingLedgerOwner struct {
	calls int
}

func (owner *countingLedgerOwner) Record(string, TrunkRed) ([]EntryRef, error) {
	owner.calls++
	return nil, nil
}

func (owner *countingLedgerOwner) Clear(string, EntryRef, Green) error {
	owner.calls++
	return nil
}

func (owner *countingLedgerOwner) Open() ([]OpenEntry, error) {
	owner.calls++
	return nil, nil
}

func trunkRedHoldUnit(machine string, revision uint64) Unit {
	return Unit{
		GoalID: "goal-" + machine,
		Chain:  "chain-" + machine,
		Claim: Claim{
			Machine:            machine,
			Lineage:            "lineage-" + machine,
			Epoch:              1,
			Revision:           revision,
			AccountingRevision: revision,
		},
		State: UnitJoined,
	}
}

func TestTrunkRedHoldIsDurableAndCallsNoOwner(t *testing.T) {
	store := NewStore(t.TempDir(), scriptedProber{})
	units := []Unit{trunkRedHoldUnit("m1c", 1), trunkRedHoldUnit("m1b", 2)}
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateProving, Units: units}))
	owner := &countingLedgerOwner{}
	store = store.WithLedgerOwner(owner)
	lockCalls := 0
	lockHeld := false
	store.seams.flock = func(_ int, operation int) error {
		lockCalls++
		if operation == unix.LOCK_UN {
			if !lockHeld {
				t.Fatal("batch lock was released while it was not held")
			}
			lockHeld = false
			return nil
		}
		if lockHeld {
			t.Fatal("batch lock was acquired twice")
		}
		lockHeld = true
		return nil
	}
	at := time.Date(2030, 2, 3, 4, 5, 6, 7, time.UTC)
	red := TrunkRed{AttemptID: "attempt-1", Groups: []RedGroup{{ID: "fast"}, {ID: "deep"}}}
	must(t, store.HoldTrunkRed(testBatchID, red, "op-1", at, "m1b+landing-m1b"))
	record := load(t, store)
	if record.State != StateHeldTrunkRed || record.TrunkRed == nil {
		t.Fatalf("state=%q trunk red=%v", record.State, record.TrunkRed)
	}
	hold := record.TrunkRed
	if hold.Opid != "op-1" || len(hold.Opids) != 1 || hold.Opids[0] != "op-1" || len(hold.Entries) != 0 || hold.RecordedAt != "" {
		t.Fatalf("hold=%+v", hold)
	}
	if hold.Red.BatchID != testBatchID || !hold.Red.SeenAt.Equal(at) {
		t.Fatalf("red batch=%q seen=%s", hold.Red.BatchID, hold.Red.SeenAt)
	}
	if len(record.History) != 1 || record.History[0].Verb != "trunk-red-hold" || record.History[0].Detail != "attempt=attempt-1 groups=fast,deep opid=op-1" {
		t.Fatalf("history=%+v", record.History)
	}
	if lockCalls != 2 || lockHeld {
		t.Fatalf("batch lock calls=%d held=%v", lockCalls, lockHeld)
	}
	if owner.calls != 0 {
		t.Fatalf("ledger owner calls=%d, want zero", owner.calls)
	}
}

func TestTrunkRedHoldTakesJoinersAndRefusesOtherStates(t *testing.T) {
	at := time.Date(2031, 3, 4, 5, 6, 7, 8, time.UTC)
	units := []Unit{trunkRedHoldUnit("m1c", 1), trunkRedHoldUnit("m1b", 2)}
	for _, state := range []string{StateProving, StateDiagnosing} {
		t.Run("accepts "+state, func(t *testing.T) {
			store := NewStore(t.TempDir(), scriptedProber{})
			must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: state, Units: units}))
			red := TrunkRed{AttemptID: "attempt", Groups: []RedGroup{{ID: "fast"}}, Joiners: []Claim{units[1].Claim, units[0].Claim}}
			must(t, store.HoldTrunkRed(testBatchID, red, "op", at, "actor"))
			if got := load(t, store).TrunkRed.Red.OwnerMachine(); got != "m1c" {
				t.Fatalf("owner machine=%q, want m1c", got)
			}
		})
	}
	cases := []struct {
		name, state, opid, reason string
		red                       TrunkRed
		units                     []Unit
		hold                      *TrunkRedHold
	}{
		{"open state", StateOpen, "op", "state open", TrunkRed{Groups: []RedGroup{{ID: "fast"}}}, units, nil},
		{"landed state", StateLanded, "op", "state landed", TrunkRed{Groups: []RedGroup{{ID: "fast"}}}, units, nil},
		{"already held state", StateHeldTrunkRed, "op", "state held-trunk-red", TrunkRed{Groups: []RedGroup{{ID: "fast"}}}, units, &TrunkRedHold{Opid: "prior", Opids: []string{"prior"}}},
		{"empty opid", StateProving, "", "empty opid", TrunkRed{Groups: []RedGroup{{ID: "fast"}}}, units, nil},
		{"no groups", StateProving, "op", "without red groups", TrunkRed{}, units, nil},
		{"no units", StateProving, "op", "without units", TrunkRed{Groups: []RedGroup{{ID: "fast"}}}, nil, nil},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			store := NewStore(t.TempDir(), scriptedProber{})
			record := Record{Schema: 1, BatchID: testBatchID, State: test.state, Units: test.units, TrunkRed: test.hold}
			must(t, store.Create(record))
			path, err := store.recordPath(testBatchID)
			must(t, err)
			before, err := os.ReadFile(path)
			must(t, err)
			err = store.HoldTrunkRed(testBatchID, test.red, test.opid, at, "actor")
			if err == nil || !strings.Contains(err.Error(), "batch "+testBatchID) || !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("error=%v, want batch and reason %q", err, test.reason)
			}
			after, readErr := os.ReadFile(path)
			must(t, readErr)
			if string(after) != string(before) {
				t.Fatal("refusal changed the batch record")
			}
		})
	}
	validHold := &TrunkRedHold{Opid: "op", Opids: []string{"op"}}
	validationCases := []struct {
		name string
		hold *TrunkRedHold
	}{
		{"missing hold", nil},
		{"empty active opid", &TrunkRedHold{Opids: []string{"op"}}},
		{"empty opid history", &TrunkRedHold{Opid: "op"}},
		{"active opid differs from last", &TrunkRedHold{Opid: "op", Opids: []string{"other"}}},
	}
	for _, test := range validationCases {
		t.Run("validation refuses "+test.name, func(t *testing.T) {
			record := Record{Schema: 1, BatchID: testBatchID, State: StateHeldTrunkRed, TrunkRed: test.hold}
			if err := validateRecord(record); err == nil {
				t.Fatal("invalid held trunk-red record was accepted")
			}
		})
	}
	if err := validateRecord(Record{Schema: 1, BatchID: testBatchID, State: StateHeldTrunkRed, TrunkRed: validHold}); err != nil {
		t.Fatalf("valid held trunk-red record was refused: %v", err)
	}
}
