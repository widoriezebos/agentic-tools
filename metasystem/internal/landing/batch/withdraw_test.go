package batch

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func withdrawalBed(t *testing.T) (policyBed, Store) {
	t.Helper()
	bed := policyFixture(t)
	for index := range bed.record.Units {
		bed.record.Units[index].SeatRoot = "/seat"
		bed.record.Units[index].SelectedGroups = []string{"shared", bed.record.Units[index].GoalID}
	}
	prefixes := []string{"prefix-a", "prefix-ab"}
	bed.record.PrefixTrees, bed.record.TipTree = prefixes, prefixes[len(prefixes)-1]
	bed.record.SelectedGroups = []string{"goal-a", "goal-b", "shared"}
	store := NewStore(bed.root, nil)
	must(t, store.Create(bed.record))
	return bed, store
}

func TestBatchWithdrawBeforeSealAndRefusesAfterSeal(t *testing.T) {
	t.Run("before seal leaves the queue and owner returns custody", func(t *testing.T) {
		bed, store := withdrawalBed(t)
		want := []string{"prefix-b"}
		strictReassembly(t, &store, expectedAssembly(bed.base, []string{"goal-b"}, []string{"chain-b"}, want))
		record, err := RequestWithdrawal(store, "goal-a", "seat", "l", "/seat", "seat+l", time.Unix(2, 0))
		must(t, err)
		if record.Units[0].State != UnitReturnPending || record.Units[0].Outcome != UnitWithdrawn ||
			record.History[len(record.History)-1].Verb != "withdraw-requested" || record.TipTree != want[0] ||
			!slices.Equal(record.PrefixTrees, want) || !slices.Equal(record.SelectedGroups, []string{"goal-b", "shared"}) {
			t.Fatalf("withdrawn record=%+v want-prefixes=%v", record, want)
		}
		handBacks := 0
		must(t, ReturnUnits(store, testBatchID, "tree", "owner", time.Unix(3, 0), ReturnSeams{
			Read: func(string, string, string) (ReturnLedgerGoal, error) {
				return ReturnLedgerGoal{Claimed: true, Machine: "landing", Lineage: "owner", Batch: testBatchID}, nil
			},
			Target:   func(Unit) ReturnTarget { return ReturnTarget{State: ReturnTargetLive, Epoch: 9} },
			HandBack: func(string, Claim, uint64) error { handBacks++; return nil },
			Release:  func(string, string) error { t.Fatal("live joiner claim was released"); return nil },
		}))
		unit := load(t, store).Units[0]
		if unit.State != UnitWithdrawn || unit.ReturnDisposition != ReturnHandedBack || handBacks != 1 {
			t.Fatalf("returned withdrawal unit=%+v handbacks=%d", unit, handBacks)
		}
	})

	t.Run("after seal refuses with remedy and preserves bytes", func(t *testing.T) {
		_, store := withdrawalBed(t)
		strictReassembly(t, &store)
		must(t, store.Update(testBatchID, func(record *Record) error { record.State = StateSealed; return nil }))
		path := filepath.Join(store.root, "artifacts", "agents", "landing-batches", testBatchID+".json")
		before, err := os.ReadFile(path)
		must(t, err)
		_, err = RequestWithdrawal(store, "goal-a", "seat", "l", "/seat", "seat+l", time.Unix(4, 0))
		after, readErr := os.ReadFile(path)
		must(t, readErr)
		if err == nil || !strings.Contains(err.Error(), "BATCH_SEALED") || !strings.Contains(err.Error(), "landing batch wait --goal goal-a") || !slices.Equal(before, after) {
			t.Fatalf("sealed refusal=%v changed=%t", err, !slices.Equal(before, after))
		}
	})
}

func TestBatchWithdrawRefusesUnspecifiedCases(t *testing.T) {
	tests := []struct {
		name, goal, machine, lineage, seat string
		change                             func(*Record)
	}{
		{name: "absent goal", goal: "goal-z", machine: "seat", lineage: "l", seat: "/seat"},
		{name: "wrong lineage", goal: "goal-a", machine: "seat", lineage: "other", seat: "/seat"},
		{name: "wrong machine", goal: "goal-a", machine: "other", lineage: "l", seat: "/seat"},
		{name: "wrong checkout", goal: "goal-a", machine: "seat", lineage: "l", seat: "/other"},
		{name: "joining", goal: "goal-a", machine: "seat", lineage: "l", seat: "/seat", change: func(record *Record) { record.Units[0].State = UnitJoining }},
		{name: "held batch", goal: "goal-a", machine: "seat", lineage: "l", seat: "/seat", change: func(record *Record) { record.State = StateHeldUnclassified }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, store := withdrawalBed(t)
			strictReassembly(t, &store)
			if test.change != nil {
				must(t, store.Update(testBatchID, func(record *Record) error { test.change(record); return nil }))
			}
			_, err := RequestWithdrawal(store, test.goal, test.machine, test.lineage, test.seat, "actor", time.Unix(2, 0))
			if err == nil || !strings.Contains(err.Error(), "BATCH_WITHDRAW_REFUSED") {
				t.Fatalf("refusal=%v", err)
			}
		})
	}
}

func TestBatchWithdrawalAssemblyFailurePreservesRecord(t *testing.T) {
	bed, store := withdrawalBed(t)
	wantErr := errors.New("survivor assembly unavailable")
	strictReassembly(t, &store, expectedReassembly{kind: "assemble", base: bed.base,
		goals: []string{"goal-b"}, chains: []string{"chain-b"}, err: wantErr})
	path := filepath.Join(store.root, "artifacts", "agents", "landing-batches", testBatchID+".json")
	before, err := os.ReadFile(path)
	must(t, err)
	_, err = RequestWithdrawal(store, "goal-a", "seat", "l", "/seat", "seat+l", time.Unix(2, 0))
	after, readErr := os.ReadFile(path)
	must(t, readErr)
	if !errors.Is(err, wantErr) || !slices.Equal(before, after) {
		t.Fatalf("assembly error=%v record changed=%t", err, !slices.Equal(before, after))
	}
}
