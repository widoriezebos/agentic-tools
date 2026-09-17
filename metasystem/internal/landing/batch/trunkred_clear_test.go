package batch

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

type clearingLedger struct {
	open        []OpenEntry
	openErr     error
	clearErrors map[string][]error
	cleared     []struct {
		opid  string
		ref   EntryRef
		green Green
	}
	recorded []TrunkRed
}

func (ledger *clearingLedger) Record(_ string, red TrunkRed) ([]EntryRef, error) {
	ledger.recorded = append(ledger.recorded, red)
	refs := make([]EntryRef, len(red.Groups))
	for index, group := range red.Groups {
		refs[index] = EntryRef{ID: TrunkRedID(group), Group: group.ID}
	}
	return refs, nil
}

func (ledger *clearingLedger) Clear(opid string, ref EntryRef, green Green) error {
	if scheduled := ledger.clearErrors[ref.ID]; len(scheduled) != 0 {
		ledger.clearErrors[ref.ID] = scheduled[1:]
		if scheduled[0] != nil {
			return scheduled[0]
		}
	}
	ledger.cleared = append(ledger.cleared, struct {
		opid  string
		ref   EntryRef
		green Green
	}{opid: opid, ref: ref, green: green})
	ledger.open = slices.DeleteFunc(ledger.open, func(entry OpenEntry) bool { return entry.ID == ref.ID })
	return nil
}

func (ledger *clearingLedger) Open() ([]OpenEntry, error) {
	return slices.Clone(ledger.open), ledger.openErr
}

func addHeldGroup(t *testing.T, store Store, ledger *clearingLedger, id string) EntryRef {
	t.Helper()
	group := RedGroup{ID: id, Status: "failed"}
	ref := EntryRef{ID: TrunkRedID(group), Group: id}
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.TrunkRed.Red.Groups = append(record.TrunkRed.Red.Groups, group)
		record.TrunkRed.Entries = append(record.TrunkRed.Entries, ref)
		return nil
	}))
	ledger.open = append(ledger.open, OpenEntry{ID: ref.ID, Group: ref.Group, Holds: []string{testBatchID}, LastBaseCommit: "base-commit"})
	return ref
}

func heldReopenBed(t *testing.T) (assemblyBed, Store, *clearingLedger) {
	t.Helper()
	bed := assemblyFixture(t)
	group := RedGroup{ID: "group", Status: "failed"}
	ref := EntryRef{ID: TrunkRedID(group), Group: group.ID}
	bed.record.State = StateHeldTrunkRed
	bed.record.TrunkRed = &TrunkRedHold{Opid: "red-1", Opids: []string{"red-1"},
		Red:     TrunkRed{BatchID: testBatchID, AttemptID: "red-attempt", BaseCommit: "base-commit", BaseTree: bed.base, Groups: []RedGroup{group}},
		Entries: []EntryRef{ref}}
	ledger := &clearingLedger{open: []OpenEntry{{ID: ref.ID, Group: ref.Group, Holds: []string{testBatchID}, LastBaseCommit: "base-commit"}}}
	store := NewStore(bed.root, nil).WithLedgerOwner(ledger)
	must(t, store.Create(bed.record))
	return bed, store, ledger
}

func TestReopenClearsOnGreenDescendantOnly(t *testing.T) {
	now := time.Unix(10, 0)
	t.Run("green descendant clears then reopens", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		requestSeen := DiagnosticRequest{}
		seams := trunkRedClearSeams{
			run: func(request DiagnosticRequest, _ Claim) (DiagnosticResult, error) {
				requestSeen = request
				return DiagnosticResult{AttemptID: "green"}, nil
			},
			mint: func() (string, error) { return "clear-1", nil }, ledger: ledger,
			descendsFrom: func(descendant, ancestor string) (bool, error) {
				return descendant == "next-commit" && ancestor == "base-commit", nil
			},
		}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams))
		record := load(t, store)
		if record.State != StateOpen || record.BaseTree != bed.moved || len(ledger.cleared) != 1 || ledger.cleared[0].green.AttemptID != "green" ||
			ledger.cleared[0].green.BaseCommit != "next-commit" || !requestSeen.NeverReuse || !slices.Equal(requestSeen.Groups, []string{"group"}) {
			t.Fatalf("record=%+v clears=%+v request=%+v", record, ledger.cleared, requestSeen)
		}
	})
	t.Run("green same commit stays held", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		seams := trunkRedClearSeams{run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
			return DiagnosticResult{AttemptID: "green"}, nil
		},
			mint: func() (string, error) { return "unused", nil }, ledger: ledger,
			descendsFrom: func(string, string) (bool, error) { t.Fatal("same commit asked for ancestry"); return false, nil }}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "base-commit", "owner", now, seams))
		if record := load(t, store); record.State != StateHeldTrunkRed || len(ledger.cleared) != 0 {
			t.Fatalf("record=%+v clears=%+v", record, ledger.cleared)
		}
	})
	t.Run("red descendant records another sighting", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		seams := trunkRedClearSeams{run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
			return DiagnosticResult{AttemptID: "red-next", Groups: []RedGroup{{ID: "group", Status: "failed"}}}, nil
		}, mint: func() (string, error) { return "record-2", nil }, ledger: ledger,
			descendsFrom: func(string, string) (bool, error) { return true, nil }}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams))
		record := load(t, store)
		if record.State != StateHeldTrunkRed || record.TrunkRed.Red.AttemptID != "red-next" || record.TrunkRed.Red.BaseCommit != "next-commit" ||
			len(ledger.recorded) != 1 || len(ledger.cleared) != 0 {
			t.Fatalf("record=%+v recorded=%+v clears=%+v", record, ledger.recorded, ledger.cleared)
		}
	})
}

func TestReopenRetriesPartialClearAndSkipsClosedEntries(t *testing.T) {
	now := time.Unix(10, 0)
	t.Run("partial clear converges", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		second := addHeldGroup(t, store, ledger, "second")
		ledger.clearErrors = map[string][]error{second.ID: {fmt.Errorf("publish pending")}}
		mints := 0
		seams := trunkRedClearSeams{
			run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
				return DiagnosticResult{AttemptID: "green"}, nil
			},
			mint: func() (string, error) { mints++; return fmt.Sprintf("clear-%d", mints), nil }, ledger: ledger,
			descendsFrom: func(string, string) (bool, error) { return true, nil },
		}
		if err := reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams); err == nil {
			t.Fatal("partial clear unexpectedly completed")
		}
		if record := load(t, store); record.State != StateHeldTrunkRed || len(ledger.cleared) != 1 {
			t.Fatalf("after partial clear record=%+v clears=%+v", record, ledger.cleared)
		}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams))
		if record := load(t, store); record.State != StateOpen || len(ledger.cleared) != 2 || ledger.cleared[1].ref.ID != second.ID {
			t.Fatalf("after retry record=%+v clears=%+v", record, ledger.cleared)
		}
	})
	t.Run("entry closed elsewhere does not block reopen", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		ledger.open = nil
		seams := trunkRedClearSeams{
			run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
				return DiagnosticResult{AttemptID: "green"}, nil
			},
			mint: func() (string, error) { return "unused", nil }, ledger: ledger,
			descendsFrom: func(string, string) (bool, error) { t.Fatal("closed entry asked for ancestry"); return false, nil },
		}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams))
		if record := load(t, store); record.State != StateOpen || len(ledger.cleared) != 0 {
			t.Fatalf("record=%+v clears=%+v", record, ledger.cleared)
		}
	})
	t.Run("concurrent close refusal does not block reopen", func(t *testing.T) {
		bed, store, ledger := heldReopenBed(t)
		ref := load(t, store).TrunkRed.Entries[0]
		ledger.clearErrors = map[string][]error{ref.ID: {fmt.Errorf("TRUNK_RED_CLOSED: entry %s is closed", ref.ID)}}
		seams := trunkRedClearSeams{
			run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
				return DiagnosticResult{AttemptID: "green"}, nil
			},
			mint: func() (string, error) { return "clear-race", nil }, ledger: ledger,
			descendsFrom: func(string, string) (bool, error) { return true, nil },
		}
		must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", now, seams))
		if record := load(t, store); record.State != StateOpen || len(ledger.cleared) != 0 {
			t.Fatalf("record=%+v clears=%+v", record, ledger.cleared)
		}
	})
}

func TestMixedDiagnosticClearsPassedEntryBeforeRehold(t *testing.T) {
	bed, store, ledger := heldReopenBed(t)
	passed := addHeldGroup(t, store, ledger, "passed")
	mints := 0
	seams := trunkRedClearSeams{
		run: func(DiagnosticRequest, Claim) (DiagnosticResult, error) {
			return DiagnosticResult{AttemptID: "mixed", Groups: []RedGroup{{ID: "group", Status: "failed"}}}, nil
		},
		mint: func() (string, error) { mints++; return fmt.Sprintf("op-%d", mints), nil }, ledger: ledger,
		descendsFrom: func(string, string) (bool, error) { return true, nil },
	}
	must(t, reopenHeldAfterDiagnostic(store, testBatchID, bed.moved, "next-commit", "owner", time.Unix(10, 0), seams))
	record := load(t, store)
	if record.State != StateHeldTrunkRed || len(record.TrunkRed.Entries) != 1 || record.TrunkRed.Entries[0].Group != "group" ||
		len(ledger.cleared) != 1 || ledger.cleared[0].ref.ID != passed.ID || ledger.cleared[0].green.AttemptID != "mixed" ||
		len(ledger.recorded) != 1 || len(ledger.recorded[0].Groups) != 1 || ledger.recorded[0].Groups[0].ID != "group" {
		t.Fatalf("record=%+v clears=%+v recorded=%+v", record, ledger.cleared, ledger.recorded)
	}
}

func TestGreenTipProofClearsHeldlessEntryOnlyWhenExecuted(t *testing.T) {
	store := NewStore(t.TempDir(), nil)
	must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateProving, Proof: &Proof{Status: "planned"}}))
	passed := func(id string) proofrun.GroupResult {
		return proofrun.GroupResult{ID: id, Status: "passed", NativeLaunched: true}
	}
	result := proofrun.TestResult{AttemptID: "tip-green", BaseCommit: "next-commit", CandidateTree: "next-tree",
		Delivery: proofrun.DeliveryJudgment{Sufficient: true}, Groups: []proofrun.GroupResult{passed("fast"), passed("held"), passed("same"), passed("wrong-tree"),
			{ID: "failed", Status: "failed", NativeLaunched: true}, {ID: "reused", Status: "reused"}}}
	must(t, FinishProof(store, testBatchID, "owner", result, nil, time.Unix(11, 0)))
	proof := load(t, store).Proof
	record := ownerRecord(testBatchID, StateLanding, time.Time{})
	record.Proof = proof
	bed := newOwnerBed(t, record, time.Unix(12, 0))
	ledger := &clearingLedger{open: []OpenEntry{
		{ID: "eligible", Group: "fast", LastBaseCommit: "old-commit"},
		{ID: "reused", Group: "reused", LastBaseCommit: "old-commit"},
		{ID: "held", Group: "held", Holds: []string{"batch"}, LastBaseCommit: "old-commit"},
		{ID: "same", Group: "same", LastBaseCommit: "next-commit"},
		{ID: "wrong-tree", Group: "wrong-tree", LastBaseCommit: "other-commit"},
		{ID: "failed", Group: "failed", LastBaseCommit: "old-commit"},
	}}
	calls := 0
	bed.store = bed.store.WithLedgerOwner(ledger)
	bed.owner.store = bed.store
	bed.owner.mint = func() (string, error) { calls++; return fmt.Sprintf("clear-%d", calls), nil }
	bed.owner.descendsFrom = func(_ string, ancestor string) (bool, error) { return ancestor != "other-commit", nil }
	must(t, bed.owner.Tick(testBatchID))
	if proof.BaseCommit != "next-commit" || proof.BaseTree != "next-tree" || len(ledger.cleared) != 1 || ledger.cleared[0].ref.ID != "eligible" ||
		ledger.cleared[0].green.AttemptID != "tip-green" || ledger.cleared[0].green.BaseTree != "next-tree" || bed.launches != 1 {
		t.Fatalf("proof=%+v clears=%+v launches=%d", proof, ledger.cleared, bed.launches)
	}
}
