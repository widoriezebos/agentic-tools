package batch

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func landingBed(t *testing.T) (assemblyBed, Store) {
	t.Helper()
	bed := newLandingBed(t)
	store := NewStore(bed.root, nil)
	strictReassembly(t, &store)
	must(t, store.Create(bed.record))
	return bed, store
}

func newLandingBed(t *testing.T) assemblyBed {
	t.Helper()
	root := t.TempDir()
	base, moved := testCommit(101), testCommit(102)
	bed := assemblyBed{root: root, base: base, moved: moved, record: Record{
		Schema: 1, BatchID: testBatchID, State: StateOpen, TipTree: base,
		Units: []Unit{
			{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 7, AccountingRevision: 5}, State: UnitJoined},
			{GoalID: "goal-b", Chain: "chain-b", Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 8, AccountingRevision: 6}, State: UnitJoined},
		},
		batchRecordFields: batchRecordFields{BaseTree: base},
	}}
	prefixes := []string{testCommit(103), testCommit(104)}
	bed.record.State, bed.record.PrefixTrees, bed.record.TipTree = StateLanding, prefixes, prefixes[len(prefixes)-1]
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip-attempt"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Receipts = map[string]PrefixReceipt{"goal-a": {GoalID: "goal-a", Tree: prefixes[0], AttemptID: "prefix-attempt"}}
	return bed
}

type expectedReassembly struct {
	kind, base, batchID, leaseTip, actor string
	goals, chains                        []string
	prefixes                             []string
	tip                                  string
	err                                  error
	onCall                               func()
}

// strictReassembly requires every repository effect to be declared by its test.
func strictReassembly(t *testing.T, store *Store, expected ...expectedReassembly) {
	t.Helper()
	called := 0
	next := func(kind, base, batchID, leaseTip, actor string, units []Unit) expectedReassembly {
		t.Helper()
		if called >= len(expected) {
			t.Fatalf("unexpected reassembly %s base=%q batch=%q lease=%q actor=%q units=%+v", kind, base, batchID, leaseTip, actor, units)
		}
		want := expected[called]
		called++
		goals, chains := make([]string, 0, len(units)), make([]string, 0, len(units))
		for _, unit := range units {
			goals, chains = append(goals, unit.GoalID), append(chains, unit.Chain)
		}
		if want.kind != kind || want.base != base || want.batchID != batchID || want.leaseTip != leaseTip || want.actor != actor ||
			!slices.Equal(want.goals, goals) || !slices.Equal(want.chains, chains) {
			t.Fatalf("reassembly call %d: got %s base=%q batch=%q lease=%q actor=%q goals=%v chains=%v, want %+v",
				called, kind, base, batchID, leaseTip, actor, goals, chains, want)
		}
		if want.onCall != nil {
			want.onCall()
		}
		return want
	}
	store.reassembly = reassemblyOperations{
		assemble: func(base string, units []Unit) ([]string, error) {
			want := next("assemble", base, "", "", "", units)
			return slices.Clone(want.prefixes), want.err
		},
		delete: func(id, tip string) error {
			return next("delete", "", id, tip, "", nil).err
		},
		rebuild: func(id, base, tip, actor string, units []Unit) (string, error) {
			want := next("rebuild", base, id, tip, actor, units)
			return want.tip, want.err
		},
	}
	t.Cleanup(func() {
		if called != len(expected) {
			t.Errorf("reassembly calls=%d, want %d; missing=%+v", called, len(expected), expected[called:])
		}
	})
}

func expectedAssembly(base string, goals, chains, prefixes []string) expectedReassembly {
	return expectedReassembly{kind: "assemble", base: base, goals: goals, chains: chains, prefixes: prefixes}
}

func testCommit(number int) string { return fmt.Sprintf("%040x", number) }

func TestBatchRecoveryRequiresEveryMemberGoalSource(t *testing.T) {
	bed := newLandingBed(t)
	bed.record.State = StateLanding
	bed.record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	bed.record.History = append(bed.record.History, HistoryEntry{At: time.Unix(3, 0).UTC().Format(time.RFC3339Nano), Verb: "prove", From: StateProving, To: StateLanding, Actor: "owner"})
	bed.record.Units = bed.record.Units[:1]
	bed.record.Units[0].CommitIDs = []string{"source-a", "source-b"}
	bed.record.Units[0].LastUnit = "10b"
	bed.record.Landing = &LandingProgress{Base: bed.record.BaseTree, PushComplete: true, PushedTip: "tip"}
	store := NewStore(bed.root, nil)
	strictReassembly(t, &store)
	must(t, store.Create(bed.record))
	foundB, finalized := false, 0
	seams := RecoverySeams{
		OriginSource: func(_ Unit, source string) (string, bool, error) {
			return "landed-" + source, source == "source-a" || foundB, nil
		},
		Finalize: func(Unit, string) error { finalized++; return nil },
		Cleanup:  func() error { return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(4, 0), seams))
	if finalized != 0 || load(t, store).Units[0].P6Done {
		t.Fatalf("partial Goal-Source set finalized member: finalized=%d record=%+v", finalized, load(t, store))
	}
	foundB = true
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(5, 0), seams))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(6, 0), seams))
	if finalized != 1 || load(t, store).State != StateLanded {
		t.Fatalf("finalized=%d record=%+v", finalized, load(t, store))
	}
}

func TestBatchRecoverySweepsOnlyLastBranchMember(t *testing.T) {
	newStore := func(t *testing.T, units []Unit) Store {
		t.Helper()
		root := t.TempDir()
		store := NewStore(root, nil)
		must(t, store.Create(Record{Schema: 1, BatchID: testBatchID, State: StateLanding, Units: units,
			Landing: &LandingProgress{PushComplete: true, PushedTip: testCommit(9)}}))
		return store
	}
	claim := Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 1, AccountingRevision: 1}
	last := Unit{GoalID: "goal-a", Chain: "branch-a", Claim: claim, State: UnitJoined,
		CommitIDs: []string{"source-a"}, BranchTip: "branch-a", GoalLast: true}
	originSource := func(_ Unit, source string) (string, bool, error) { return "landed-" + source, true, nil }

	t.Run("missing seam refuses", func(t *testing.T) {
		store := newStore(t, []Unit{last})
		finalized := 0
		err := RecoverPushedSeries(store, testBatchID, "owner", time.Unix(2, 0), RecoverySeams{
			OriginSource: originSource,
			Finalize:     func(Unit, string) error { finalized++; return nil },
		})
		if err == nil || !strings.Contains(err.Error(), "goal branch sweep helper is absent") || finalized != 0 || load(t, store).Units[0].P6Done {
			t.Fatalf("error=%v finalized=%d record=%+v", err, finalized, load(t, store))
		}
	})

	t.Run("failed sweep retries before finalization", func(t *testing.T) {
		store := newStore(t, []Unit{last})
		sweeps, finalized := 0, 0
		seams := RecoverySeams{
			OriginSource: originSource,
			SweepGoalBranch: func(Unit, string) error {
				sweeps++
				if sweeps == 1 {
					return errors.New("sweep refused")
				}
				return nil
			},
			Finalize: func(Unit, string) error { finalized++; return nil },
			Cleanup:  func() error { return nil },
		}
		if err := RecoverPushedSeries(store, testBatchID, "owner", time.Unix(2, 0), seams); err == nil || !strings.Contains(err.Error(), "sweep refused") {
			t.Fatalf("first recovery=%v", err)
		}
		if finalized != 0 || load(t, store).Units[0].P6Done {
			t.Fatalf("failed sweep finalized=%d record=%+v", finalized, load(t, store))
		}
		must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(3, 0), seams))
		if sweeps != 2 || finalized != 1 || !load(t, store).Units[0].P6Done {
			t.Fatalf("sweeps=%d finalized=%d record=%+v", sweeps, finalized, load(t, store))
		}
	})

	t.Run("through and chain members do not sweep", func(t *testing.T) {
		through := last
		through.GoalID, through.GoalLast = "goal-through", false
		through.CommitIDs = []string{"source-through"}
		chain := Unit{GoalID: "goal-chain", Chain: "chain-a", Claim: claim, State: UnitJoined, GoalLast: true}
		store := newStore(t, []Unit{through, chain})
		finalized := 0
		must(t, RecoverPushedSeries(store, testBatchID, "owner", time.Unix(2, 0), RecoverySeams{
			OriginSource:    originSource,
			OriginCommit:    func(Unit) (string, bool, error) { return "landed-chain", true, nil },
			SweepGoalBranch: func(Unit, string) error { return errors.New("unexpected sweep") },
			Finalize:        func(Unit, string) error { finalized++; return nil },
			Cleanup:         func() error { return nil },
		}))
		if finalized != 2 || load(t, store).State != StateLanded {
			t.Fatalf("finalized=%d record=%+v", finalized, load(t, store))
		}
	})
}

// pushedLanding records the batch's series as pushed to main, as a
// publication that reached main leaves it for landed-trailer recovery.
func pushedLanding(t *testing.T, store Store) {
	t.Helper()
	must(t, store.Update(testBatchID, func(record *Record) error {
		record.Landing = &LandingProgress{Base: record.BaseTree, PushComplete: true, PushedTip: "commit-goal-b"}
		return nil
	}))
}

func TestBatchAfterPushRecoveryFinalizesEachTrailerOnce(t *testing.T) {
	_, store := landingBed(t)
	pushedLanding(t, store)
	finalized := map[string]int{}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		Finalize:     func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
		Cleanup:      func() error { return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "lane:test", time.Unix(5, 0), seams))
	must(t, RecoverPushedSeries(store, testBatchID, "lane:test", time.Unix(6, 0), seams))
	if finalized["goal-a"] != 1 || finalized["goal-b"] != 1 || load(t, store).State != StateLanded {
		t.Fatalf("finalized=%v record=%+v", finalized, load(t, store))
	}
}

func TestBatchRecoveryRefusalsNameCauses(t *testing.T) {
	t.Run("finalize", func(t *testing.T) {
		_, store := landingBed(t)
		pushedLanding(t, store)
		err := RecoverPushedSeries(store, testBatchID, "lane:test", time.Unix(5, 0), RecoverySeams{
			OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
			Finalize:     func(Unit, string) error { return errors.New("finalize cause") },
		})
		if err == nil || !strings.Contains(err.Error(), "BATCH_P6_REFUSED") || !strings.Contains(err.Error(), "finalize cause") {
			t.Fatalf("finalize refusal=%v", err)
		}
	})
	t.Run("cleanup", func(t *testing.T) {
		_, store := landingBed(t)
		pushedLanding(t, store)
		err := RecoverPushedSeries(store, testBatchID, "lane:test", time.Unix(5, 0), RecoverySeams{
			OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
			Finalize:     func(Unit, string) error { return nil },
			Cleanup:      func() error { return errors.New("cleanup cause") },
		})
		if err == nil || !strings.Contains(err.Error(), "BATCH_P6_REFUSED") || !strings.Contains(err.Error(), "cleanup cause") {
			t.Fatalf("cleanup refusal=%v", err)
		}
	})
}
