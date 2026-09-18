package batch

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type recordOwner func(string, TrunkRed) ([]EntryRef, error)

func (owner recordOwner) Record(opid string, red TrunkRed) ([]EntryRef, error) {
	return owner(opid, red)
}
func (recordOwner) Clear(string, EntryRef, Green) error { return nil }
func (recordOwner) Open() ([]OpenEntry, error)          { return nil, nil }

func heldTrunkRedStore(t *testing.T, owner LedgerOwner) Store {
	t.Helper()
	store := NewStore(t.TempDir(), scriptedProber{})
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateProving, Units: []Unit{trunkRedHoldUnit("m1b", 1)}}))
	must(t, store.HoldTrunkRed(testBatchID, TrunkRed{AttemptID: "attempt", Groups: []RedGroup{{ID: "fast"}}}, "op-1", time.Date(2032, 1, 2, 3, 4, 5, 6, time.UTC), "actor"))
	if owner != nil {
		store = store.WithLedgerOwner(owner)
	}
	return store
}

func recordBytes(t *testing.T, store Store) []byte {
	t.Helper()
	path, err := store.recordPath(testBatchID)
	must(t, err)
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

func ensureTrunkRedRecorded(t *testing.T, store Store, mint func() (string, error), at time.Time, actor string) TrunkRedRecordOutcome {
	t.Helper()
	outcome, err := store.EnsureTrunkRedRecorded(testBatchID, mint, at, actor)
	must(t, err)
	return outcome
}

func TestTrunkRedHoldIsDurableBeforeAnyLedgerCall(t *testing.T) {
	var store Store
	lockHeld := false
	owner := recordOwner(func(opid string, _ TrunkRed) ([]EntryRef, error) {
		if lockHeld {
			t.Fatal("ledger owner was called while the batch lock was held")
		}
		record := load(t, store)
		hold := record.TrunkRed
		if record.State != StateHeldTrunkRed || hold == nil || hold.Opid != opid || len(hold.Entries) != 0 {
			t.Fatalf("durable hold=%+v opid=%q", hold, opid)
		}
		must(t, store.Update("01h00000000000000000000001", func(*Record) error { return nil }))
		return []EntryRef{{ID: "entry-1", Group: "fast"}}, nil
	})
	store = heldTrunkRedStore(t, owner)
	must(t, store.Create(Record{Schema: 1, BatchID: "01h00000000000000000000001", State: StateOpen}))
	store.seams.flock = func(_ int, operation int) error {
		if operation == unix.LOCK_UN {
			lockHeld = false
		} else if lockHeld {
			t.Fatal("batch lock was acquired twice")
		} else {
			lockHeld = true
		}
		return nil
	}
	if outcome := ensureTrunkRedRecorded(t, store, nil, time.Date(2033, 2, 3, 4, 5, 6, 7, time.UTC), "actor"); outcome != TrunkRedRecordRecorded {
		t.Fatalf("record outcome=%q", outcome)
	}
	if hold := load(t, store).TrunkRed; len(hold.Entries) != 1 || hold.Entries[0].ID != "entry-1" {
		t.Fatalf("recorded hold=%+v", hold)
	}
}

func TestTrunkRedRefusalLeavesHeldWithEmptyEntries(t *testing.T) {
	calls := 0
	owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
		calls++
		if calls < 3 {
			return nil, errors.New("ledger unavailable")
		}
		return []EntryRef{{ID: "entry-1", Group: "fast"}}, nil
	})
	store := heldTrunkRedStore(t, owner)
	at := time.Date(2033, 2, 3, 4, 5, 6, 7, time.FixedZone("offset", 3600))
	for range 2 {
		if _, err := store.EnsureTrunkRedRecorded(testBatchID, nil, at, "actor"); err == nil {
			t.Fatal("refusing owner returned no error")
		}
	}
	record := load(t, store)
	if record.State != StateHeldTrunkRed || len(record.TrunkRed.Entries) != 0 || len(record.History) != 2 || record.History[1].Verb != "trunk-red-record-refused" || record.History[1].Detail != "trunk-red-record refused: ledger unavailable" {
		t.Fatalf("refused record=%+v", record)
	}
	if outcome := ensureTrunkRedRecorded(t, store, nil, at, "actor"); outcome != TrunkRedRecordRecorded {
		t.Fatalf("record outcome=%q", outcome)
	}
	record = load(t, store)
	if len(record.TrunkRed.Entries) != 1 || record.TrunkRed.RecordedAt != at.UTC().Format(time.RFC3339Nano) || record.History[len(record.History)-1].Verb != "trunk-red-recorded" || record.History[len(record.History)-1].Detail != "entries=entry-1" {
		t.Fatalf("recorded hold=%+v history=%+v", record.TrunkRed, record.History)
	}
	if outcome := ensureTrunkRedRecorded(t, store, nil, at, "actor"); outcome != TrunkRedRecordAlready {
		t.Fatalf("repeat outcome=%q", outcome)
	}
	if calls != 3 {
		t.Fatalf("owner calls=%d, want 3", calls)
	}
	for _, test := range []struct {
		name  string
		owner LedgerOwner
		want  string
	}{
		{name: "unbound owner", want: "TRUNK_RED_OWNER_UNBOUND: "},
		{name: "empty entries", owner: recordOwner(func(string, TrunkRed) ([]EntryRef, error) { return nil, nil }), want: "ledger owner returned no entries"},
	} {
		t.Run(test.name, func(t *testing.T) {
			store := heldTrunkRedStore(t, test.owner)
			_, err := store.EnsureTrunkRedRecorded(testBatchID, nil, at, "actor")
			record := load(t, store)
			if err == nil || !strings.Contains(err.Error(), test.want) || len(record.TrunkRed.Entries) != 0 || record.History[len(record.History)-1].Verb != "trunk-red-record-refused" {
				t.Fatalf("error=%v record=%+v", err, record)
			}
		})
	}
}

func TestTrunkRedRecordOutcomeDistinguishesAlreadyFromMovedOn(t *testing.T) {
	t.Run("batch state moved on", func(t *testing.T) {
		calls := 0
		store := heldTrunkRedStore(t, recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			calls++
			return []EntryRef{{ID: "entry-1", Group: "fast"}}, nil
		}))
		must(t, store.Update(testBatchID, func(record *Record) error {
			record.State = StateOpen
			return nil
		}))
		outcome, err := store.EnsureTrunkRedRecorded(testBatchID, nil, time.Time{}, "actor")
		if err != nil || outcome != TrunkRedRecordMovedOn || calls != 0 {
			t.Fatalf("outcome=%q error=%v owner calls=%d", outcome, err, calls)
		}
	})

	t.Run("same operation already stored references", func(t *testing.T) {
		var store Store
		entries := []EntryRef{{ID: "entry-1", Group: "fast"}}
		owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			must(t, store.Update(testBatchID, func(record *Record) error {
				record.TrunkRed.Entries = entries
				record.Transition(StateHeldTrunkRed, time.Time{}, "trunk-red-recorded", "other", "entries=entry-1")
				return nil
			}))
			return entries, nil
		})
		store = heldTrunkRedStore(t, owner)
		outcome, err := store.EnsureTrunkRedRecorded(testBatchID, nil, time.Time{}, "actor")
		record := load(t, store)
		if err != nil || outcome != TrunkRedRecordAlready || len(record.History) != 2 || record.History[1].Verb != "trunk-red-recorded" {
			t.Fatalf("outcome=%q error=%v history=%+v", outcome, err, record.History)
		}
	})

	t.Run("same operation stores references before refusal", func(t *testing.T) {
		var store Store
		entries := []EntryRef{{ID: "entry-1", Group: "fast"}}
		owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			must(t, store.Update(testBatchID, func(record *Record) error {
				record.TrunkRed.Entries = entries
				record.Transition(StateHeldTrunkRed, time.Time{}, "trunk-red-recorded", "other", "entries=entry-1")
				return nil
			}))
			return nil, errors.New("git fetch: connection reset")
		})
		store = heldTrunkRedStore(t, owner)
		outcome, err := store.EnsureTrunkRedRecorded(testBatchID, nil, time.Time{}, "actor")
		record := load(t, store)
		if err != nil || outcome != TrunkRedRecordAlready || len(record.History) != 2 || record.History[1].Verb != "trunk-red-recorded" {
			t.Fatalf("outcome=%q error=%v history=%+v", outcome, err, record.History)
		}
	})
}

func TestTrunkRedFailedOutcomeMintsOneNewOpid(t *testing.T) {
	if !strings.HasPrefix(ErrTrunkRedRecordPending.Error(), "TRUNK_RED_RECORD_PENDING: ") {
		t.Fatalf("pending error=%q", ErrTrunkRedRecordPending)
	}
	t.Run("failed then pending", func(t *testing.T) {
		calls := 0
		owner := recordOwner(func(opid string, _ TrunkRed) ([]EntryRef, error) {
			calls++
			if calls == 1 {
				return nil, &TrunkRedRecordFailed{Outcome: "aborted", Evidence: "entry missing"}
			}
			if opid != "op-2" {
				t.Fatalf("retry opid=%q, want op-2", opid)
			}
			return nil, fmt.Errorf("still working: %w", ErrTrunkRedRecordPending)
		})
		store := heldTrunkRedStore(t, owner)
		mintCalls := 0
		mint := func() (string, error) {
			mintCalls++
			return "op-2", nil
		}
		var failed *TrunkRedRecordFailed
		if _, err := store.EnsureTrunkRedRecorded(testBatchID, mint, time.Time{}, "actor"); !errors.As(err, &failed) {
			t.Fatalf("error=%v, want failed transaction", err)
		}
		record := load(t, store)
		if len(record.TrunkRed.Opids) != 2 || record.TrunkRed.Opid != "op-2" || mintCalls != 1 || failed.Error() != "TRUNK_RED_RECORD_FAILED aborted: entry missing" || record.History[len(record.History)-1].Verb != "trunk-red-record-failed" || record.History[len(record.History)-1].Detail != "opid=op-1 outcome=aborted new-opid=op-2" {
			t.Fatalf("record=%+v mint calls=%d", record, mintCalls)
		}
		before := recordBytes(t, store)
		pendingMints := 0
		_, err := store.EnsureTrunkRedRecorded(testBatchID, func() (string, error) { pendingMints++; return "unused", nil }, time.Time{}, "actor")
		if !errors.Is(err, ErrTrunkRedRecordPending) || string(recordBytes(t, store)) != string(before) || pendingMints != 0 {
			t.Fatalf("pending error=%v mint calls=%d", err, pendingMints)
		}
	})
	t.Run("competing caller", func(t *testing.T) {
		var store Store
		owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			must(t, store.Update(testBatchID, func(record *Record) error {
				record.TrunkRed.Opids = append(record.TrunkRed.Opids, "op-other")
				record.TrunkRed.Opid = "op-other"
				return nil
			}))
			return nil, &TrunkRedRecordFailed{Outcome: "aborted", Evidence: "lost race"}
		})
		store = heldTrunkRedStore(t, owner)
		mintCalls := 0
		outcome, err := store.EnsureTrunkRedRecorded(testBatchID, func() (string, error) { mintCalls++; return "op-2", nil }, time.Time{}, "actor")
		hold := load(t, store).TrunkRed
		if err != nil || outcome != TrunkRedRecordMovedOn || mintCalls != 0 || len(hold.Opids) != 2 || hold.Opid != "op-other" {
			t.Fatalf("outcome=%q error=%v mint calls=%d hold=%+v", outcome, err, mintCalls, hold)
		}
	})
	t.Run("mint error", func(t *testing.T) {
		failed := &TrunkRedRecordFailed{Outcome: "aborted", Evidence: "closed"}
		owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
			return nil, failed
		})
		store := heldTrunkRedStore(t, owner)
		before := recordBytes(t, store)
		mintErr := errors.New("mint unavailable")
		_, err := store.EnsureTrunkRedRecorded(testBatchID, func() (string, error) { return "", mintErr }, time.Time{}, "actor")
		if !errors.Is(err, mintErr) || !errors.Is(err, failed) || string(recordBytes(t, store)) != string(before) {
			t.Fatalf("mint error=%v or changed the record", err)
		}
	})
}

func TestTrunkRedFailedOutcomeRefusesMissingMint(t *testing.T) {
	failed := &TrunkRedRecordFailed{Outcome: "aborted", Evidence: "ledger rejected the write"}
	store := heldTrunkRedStore(t, recordOwner(func(string, TrunkRed) ([]EntryRef, error) { return nil, failed }))
	before := recordBytes(t, store)
	var panicValue any
	var err error
	func() {
		defer func() { panicValue = recover() }()
		_, err = store.EnsureTrunkRedRecorded(testBatchID, nil, time.Time{}, "actor")
	}()
	if panicValue != nil {
		t.Fatalf("failed ledger outcome with no mint panicked: %v", panicValue)
	}
	if !errors.Is(err, failed) || !strings.Contains(err.Error(), "mint is unavailable") || string(recordBytes(t, store)) != string(before) {
		t.Fatalf("missing mint error=%v or changed the batch record", err)
	}
}

func applyTrunkRedTestChange(t *testing.T, store Store, action string) {
	t.Helper()
	must(t, store.Update(testBatchID, func(record *Record) error {
		if action == "rotate" {
			record.TrunkRed.Opids = append(record.TrunkRed.Opids, "op-other")
			record.TrunkRed.Opid = "op-other"
		} else {
			record.State = action
		}
		return nil
	}))
}

func TestTrunkRedLeavesABatchThatMovedOnAlone(t *testing.T) {
	ownerErr := errors.New("owner refusal")
	failedErr := &TrunkRedRecordFailed{Outcome: "aborted", Evidence: "not confirmed"}
	tests := []struct {
		name, owner, result, mint, wantState, wantOpids string
		wantMints, wantHistory                          int
	}{
		{"open refs", StateOpen, "refs", "", StateOpen, "op-1", 0, 0},
		{"dissolved refs", StateDissolved, "refs", "", StateDissolved, "op-1", 0, 0},
		{"open failed", StateOpen, "failed", "", StateOpen, "op-1", 0, 0},
		{"dissolved failed", StateDissolved, "failed", "", StateDissolved, "op-1", 0, 0},
		{"open refusal", StateOpen, "refusal", "", StateOpen, "op-1", 0, 0},
		{"dissolved refusal", StateDissolved, "refusal", "", StateDissolved, "op-1", 0, 0},
		{"mint opens", "", "failed", StateOpen, StateOpen, "op-1", 1, 0},
		{"owner rotates before refs", "rotate", "refs", "", StateHeldTrunkRed, "op-1,op-other", 0, 0},
		{"mint rotates", "", "failed", "rotate", StateHeldTrunkRed, "op-1,op-other", 1, 0},
		{"owner rotates before refusal", "rotate", "refusal", "", StateHeldTrunkRed, "op-1,op-other", 0, 0},
		{"nil failed transaction", "", "nil", "", StateHeldTrunkRed, "op-1", 0, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var store Store
			owner := recordOwner(func(string, TrunkRed) ([]EntryRef, error) {
				if test.owner != "" {
					applyTrunkRedTestChange(t, store, test.owner)
				}
				switch test.result {
				case "refs":
					return []EntryRef{{ID: "entry", Group: "fast"}}, nil
				case "failed":
					return nil, failedErr
				case "refusal":
					return nil, ownerErr
				default:
					var failure *TrunkRedRecordFailed
					return nil, failure
				}
			})
			store = heldTrunkRedStore(t, owner)
			mints := 0
			outcome, err := store.EnsureTrunkRedRecorded(testBatchID, func() (string, error) {
				mints++
				if test.mint != "" {
					applyTrunkRedTestChange(t, store, test.mint)
				}
				return "op-2", nil
			}, time.Time{}, "actor")
			record := load(t, store)
			if record.State != test.wantState || len(record.TrunkRed.Entries) != 0 || strings.Join(record.TrunkRed.Opids, ",") != test.wantOpids || mints != test.wantMints || len(record.History) != 1+test.wantHistory {
				t.Fatalf("outcome=%q error=%v state=%q hold=%+v mints=%d history=%+v", outcome, err, record.State, record.TrunkRed, mints, record.History)
			}
			moved := test.owner != "" || test.mint != ""
			if moved && (outcome != TrunkRedRecordMovedOn || err != nil) || !moved && ((test.result == "refs" && err != nil) || (test.result == "failed" && !errors.Is(err, failedErr)) || (test.result == "refusal" && !errors.Is(err, ownerErr)) || (test.result == "nil" && (err == nil || record.History[1].Verb != "trunk-red-record-refused"))) {
				t.Fatalf("result=%q outcome=%q error=%v history=%+v", test.result, outcome, err, record.History)
			}
		})
	}
}
