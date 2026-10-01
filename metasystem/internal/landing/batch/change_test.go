package batch

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const changeCommit = "abcdef0123456789abcdef0123456789abcdef01"

func changeMemberUnit() Unit {
	return NewChangeUnit(ChangeMember{Commit: changeCommit, Parent: testCommit(90), AskedBy: "m1e+human", Subject: "record: notes"},
		"/seats/m1e/metasystem", "m1e", "human", []string{"records/notes.md"}, nil)
}

func openBatchWithGoalMember(base string) Record {
	return Record{Schema: 1, BatchID: testBatchID, State: StateOpen, TipTree: testCommit(103),
		Units:             []Unit{{GoalID: "goal-a", Chain: "chain-a", Claim: Claim{Machine: "landing", Lineage: "owner", Epoch: 1, Revision: 7, AccountingRevision: 5}, State: UnitJoined}},
		History:           []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "join", Detail: "goal-a joined"}},
		batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(103)}}}
}

// TestChangeRecoveryFindsLandingChangeTrailer (U11b): after the push a change
// is recognized by its Landing-Change trailer on origin, never by the chain
// provenance a certified chain carries, and has no goal Next to finalize; a
// repeat changes nothing, and an unreadable origin log fails the recovery
// instead of reading as "not landed yet".
func TestChangeRecoveryFindsLandingChangeTrailer(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	record := openBatchWithGoalMember(base)
	change := changeMemberUnit()
	change.State = UnitJoined
	record.Units = append(record.Units, change)
	record.State, record.PrefixTrees, record.TipTree = StateLanding, []string{testCommit(103), testCommit(104)}, testCommit(104)
	record.Proof = &Proof{Status: "green", AttemptID: "tip"}
	record.Landing = &LandingProgress{Base: base, PushComplete: true, PushedTip: "pushed", CandidateTip: "pushed", ReceiptTip: "pushed"}
	must(t, store.Create(record))
	finalized := map[string]int{}
	failing := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) { return "origin-" + unit.Chain, true, nil },
		OriginChange: func(Unit) (string, bool, error) { return "", false, errors.New("git log: unreadable") },
		Finalize:     func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
	}
	if err := RecoverPushedSeries(store, testBatchID, "owner", ten, failing); err == nil || !strings.Contains(err.Error(), "unreadable") {
		t.Fatalf("unreadable origin recovery=%v", err)
	}
	if load(t, store).State == StateLanded {
		t.Fatal("an unreadable origin landed the batch")
	}
	seams := RecoverySeams{
		OriginCommit: func(unit Unit) (string, bool, error) {
			if unit.IsChange() {
				t.Fatalf("a change was looked up by chain provenance")
			}
			return "origin-" + unit.Chain, true, nil
		},
		OriginChange: func(unit Unit) (string, bool, error) {
			return "origin-" + strings.TrimPrefix(unit.GoalID, "change:"), true, nil
		},
		Finalize: func(unit Unit, _ string) error { finalized[unit.GoalID]++; return nil },
		Cleanup:  func() error { return nil },
	}
	must(t, RecoverPushedSeries(store, testBatchID, "owner", ten, seams))
	must(t, RecoverPushedSeries(store, testBatchID, "owner", ten.Add(time.Minute), seams))
	landed := load(t, store)
	got := landed.Units[1]
	if landed.State != StateLanded || got.Outcome != UnitLanded || !got.P6Done || got.LandedCommit != "origin-abcdef012345" || finalized["goal-a"] != 1 || finalized[change.GoalID] != 0 {
		t.Fatalf("recovered state=%s change=%+v finalized=%v", landed.State, got, finalized)
	}
}

func stackedChange(parent Unit) Unit {
	child := NewChangeUnit(ChangeMember{Commit: "c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0c0", Parent: parent.Change.Commit, AskedBy: "m1e+human", Subject: "record: more"},
		"/seats/m1e/metasystem", "m1e", "human", []string{"records/notes.md"}, nil)
	child.State = UnitJoined
	return child
}

// TestStackedChangeInAnotherBatchIsRefused (U11b B-1a): a change stacked on
// a live change of another batch is refused at the join, naming the parent
// and its batch; nothing is assembled or written.
func TestStackedChangeInAnotherBatchIsRefused(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store)
	parent := changeMemberUnit()
	parent.State = UnitJoined
	proving := Record{Schema: 1, BatchID: "01j5x00000000000000000ba71", State: StateProving, TipTree: testCommit(104),
		Units: []Unit{parent}, History: []HistoryEntry{{At: ten.Format(time.RFC3339Nano), Verb: "join", Detail: parent.GoalID + " joined"}},
		batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{testCommit(104)}}}
	must(t, store.Create(proving))
	child := stackedChange(parent)
	child.State = UnitJoining
	_, err := JoinChange(store, ChangeJoin{Unit: child, BaseTree: base, NewID: "01j5x00000000000000000ba72", Actor: "m1e+human", At: ten})
	if err == nil || !strings.Contains(err.Error(), "change "+child.GoalID+" is stacked on change "+parent.GoalID+", which is in batch 01j5x00000000000000000ba71 (proving); run the same command after "+parent.GoalID+" lands") {
		t.Fatalf("join=%v", err)
	}
	var stacked *StackedChangeRefusal
	if !errors.As(err, &stacked) {
		t.Fatalf("not typed: %v", err)
	}
	if records, _ := store.Records(); len(records) != 1 {
		t.Fatalf("records=%d", len(records))
	}
}

// TestParentChangeLeavingTakesItsChildren (U11b B-1b): when a change leaves
// its batch, every live change stacked on it leaves in the same step with
// the parent's reason; reassembly never keeps a child without its parent.
func TestParentChangeLeavingTakesItsChildren(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	base := testCommit(101)
	store := NewStore(root, nil)
	strictReassembly(t, &store, expectedAssembly(base, []string{"goal-a"}, []string{"chain-a"}, []string{testCommit(103)}))
	record := openBatchWithGoalMember(base)
	parent := changeMemberUnit()
	parent.State = UnitJoined
	child := stackedChange(parent)
	grandchild := NewChangeUnit(ChangeMember{Commit: "d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0d0", Parent: child.Change.Commit, AskedBy: "m1e+human"}, "/s", "m1e", "human", nil, nil)
	grandchild.State = UnitJoined
	record.Units = append(record.Units, parent, child, grandchild)
	record.PrefixTrees = []string{testCommit(103), testCommit(104), testCommit(105), testCommit(106)}
	must(t, store.Create(record))
	must(t, ReassembleSurvivorsWithReturns(store, testBatchID, "owner", ten, []ReturnDecision{{GoalID: parent.GoalID, Outcome: UnitEjected, Reason: "EJECTED: TestNotes failed"}}))
	after := load(t, store)
	for _, unit := range after.Units[2:] {
		if unit.State != UnitReturnPending || unit.Outcome != UnitEjected || !strings.Contains(unit.Failure, "EJECTED: TestNotes failed") {
			t.Fatalf("stacked %s stayed or lost the reason: %+v", unit.GoalID, unit)
		}
	}
	if !strings.Contains(after.Units[2].Failure, "its parent change "+parent.GoalID+" left the batch") || after.Units[0].State != UnitJoined {
		t.Fatalf("child=%+v goal=%+v", after.Units[2], after.Units[0])
	}
}
