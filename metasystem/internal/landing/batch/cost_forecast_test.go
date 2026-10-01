package batch

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func costJoinBed(t *testing.T) (Store, Record, Unit, CostForecast) {
	t.Helper()
	bed := policyFixture(t)
	record := bed.record
	prefixes := []string{testCommit(103), testCommit(104)}
	record.PrefixTrees, record.TipTree = prefixes, prefixes[len(prefixes)-1]
	store := NewStore(bed.root, nil)
	strictReassembly(t, &store)
	must(t, store.Create(record))
	incoming := Unit{GoalID: "goal-c", Chain: "chain-c", State: UnitJoining,
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, SelectedGroups: []string{"c"}}
	prospective := append(append([]Unit(nil), record.Units...), incoming)
	all := []string{prefixes[0], prefixes[1], testCommit(105)}
	forecast := CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(3, 0).UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: CostBinding(record, prospective, all)}
	return store, record, incoming, forecast
}

func costCASBed(t *testing.T) (Store, Record, Unit, CostForecast) {
	t.Helper()
	base, first, second, third := testCommit(101), testCommit(103), testCommit(104), testCommit(105)
	record := Record{Schema: 1, BatchID: testBatchID, State: StateOpen, TipTree: second,
		Units: []Unit{
			{GoalID: "goal-a", Chain: "chain-a", State: UnitJoined,
				Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 7, AccountingRevision: 5}},
			{GoalID: "goal-b", Chain: "chain-b", State: UnitJoined,
				Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 8, AccountingRevision: 6}},
		},
		batchRecordFields: batchRecordFields{BaseTree: base, PrefixTrees: []string{first, second}},
	}
	store := NewStore(t.TempDir(), nil)
	strictReassembly(t, &store)
	must(t, store.Create(record))
	incoming := Unit{GoalID: "goal-c", Chain: "chain-c", State: UnitJoining,
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, SelectedGroups: []string{"c"}}
	prospective := append(append([]Unit(nil), record.Units...), incoming)
	forecast := CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(3, 0).UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: CostBinding(record, prospective, []string{first, second, third})}
	return store, record, incoming, forecast
}

func TestBatchCostClosureKeepsIncomingOwnerAndRequiresMatchingSnapshot(t *testing.T) {
	t.Parallel()
	store, record, incoming, forecast := costJoinBed(t)
	must(t, CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(3, 0), forecast))
	closed, err := store.Load(record.BatchID)
	must(t, err)
	if closed.ClosedReason != "budget-cost" || len(closed.Units) != len(record.Units) || len(closed.History) != 1 ||
		closed.History[0].Verb != "close" || closed.History[0].Detail != "budget-cost" {
		t.Fatalf("over-budget join changed custody or omitted closure: %+v", closed)
	}
	for _, unit := range closed.Units {
		if unit.GoalID == incoming.GoalID {
			t.Fatal("over-budget member was handed over")
		}
	}
	if err := CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(4, 0), forecast); err == nil || !strings.Contains(err.Error(), "BATCH_CLOSED") {
		t.Fatalf("closed admission accepted stale forecast: %v", err)
	}
}

func TestBatchCostJoinCASRefusesChangedMemberBeforeHandover(t *testing.T) {
	t.Parallel()
	store, record, incoming, forecast := costCASBed(t)
	forecast.Binding.Members[0].Claim.AccountingRevision++
	handedOver := false
	err := PublishJoinWithAdmissionForecast(store, record.BatchID, incoming, "owner", time.Unix(3, 0),
		func(_, _, _ string) (testpolicy.Plan, error) {
			t.Fatal("changed claim reached planning")
			return testpolicy.Plan{}, nil
		},
		func() error { handedOver = true; return nil },
		func(_ string, unit Unit) (JoinAdmission, error) {
			t.Fatal("changed claim reached admission")
			return JoinAdmission{}, nil
		}, forecast)
	if err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") || handedOver {
		t.Fatalf("changed claim crossed cost CAS: err=%v handover=%t", err, handedOver)
	}
	current, loadErr := store.Load(record.BatchID)
	must(t, loadErr)
	if len(current.Units) != len(record.Units) || !reflect.DeepEqual(current.Units, record.Units) ||
		!reflect.DeepEqual(current.PrefixTrees, record.PrefixTrees) {
		t.Fatalf("failed CAS published incoming member: %+v", current)
	}
}

func TestBatchCostJoinAndClosureRejectReplacedPrefixEpisode(t *testing.T) {
	t.Parallel()
	store, record, incoming, forecast := costCASBed(t)
	first := PrefixEpisode{DecisionID: "decision-a", Token: strings.Repeat("a", 64), ExpiresAt: time.Unix(30, 0).UTC().Format(time.RFC3339Nano)}
	replacement := PrefixEpisode{DecisionID: "decision-b", Token: strings.Repeat("b", 64), ExpiresAt: time.Unix(40, 0).UTC().Format(time.RFC3339Nano)}
	must(t, store.Update(record.BatchID, func(current *Record) error {
		current.PrefixEpisodes = map[string]PrefixEpisode{"goal-a": first}
		return nil
	}))
	current, err := store.Load(record.BatchID)
	must(t, err)
	prospective := append(append([]Unit(nil), current.Units...), incoming)
	forecast.Binding = CostBinding(current, prospective, forecast.Binding.PrefixTrees)
	must(t, store.Update(record.BatchID, func(current *Record) error {
		current.PrefixEpisodes["goal-a"] = replacement
		return nil
	}))
	handedOver := false
	err = PublishJoinWithAdmissionForecast(store, record.BatchID, incoming, "owner", time.Unix(3, 0),
		func(_, _, _ string) (testpolicy.Plan, error) {
			t.Fatal("replaced episode reached planning")
			return testpolicy.Plan{}, nil
		},
		func() error { handedOver = true; return nil },
		func(_ string, unit Unit) (JoinAdmission, error) {
			t.Fatal("replaced episode reached admission")
			return JoinAdmission{}, nil
		}, forecast)
	if err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") || handedOver {
		t.Fatalf("replaced episode crossed join CAS: err=%v handedOver=%t", err, handedOver)
	}
	if err := CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(3, 0), forecast); err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") {
		t.Fatalf("replaced episode closed admission using stale forecast: %v", err)
	}
	current, err = store.Load(record.BatchID)
	must(t, err)
	if len(current.Units) != len(record.Units) || !reflect.DeepEqual(current.Units, record.Units) ||
		!reflect.DeepEqual(current.PrefixTrees, record.PrefixTrees) || current.ClosedReason != "" {
		t.Fatalf("stale episode moved custody or closed admission: %+v", current)
	}
	recomputed := forecast
	recomputed.Binding = CostBinding(current, prospective, forecast.Binding.PrefixTrees)
	if reflect.DeepEqual(recomputed.Binding, forecast.Binding) || recomputed.Binding.PrefixEpisodes[0].Token != replacement.Token {
		t.Fatalf("recomputed forecast did not bind replacement episode: old=%+v new=%+v", forecast.Binding, recomputed.Binding)
	}
	must(t, CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(3, 0), recomputed))
	closed, err := store.Load(record.BatchID)
	must(t, err)
	if closed.ClosedReason != "budget-cost" || len(closed.Units) != len(record.Units) ||
		len(closed.History) != 1 || closed.History[0].Verb != "close" {
		t.Fatalf("recomputed forecast did not close admission without changing custody: %+v", closed)
	}
}

func TestBatchCostBindingMarksReassembledOrReselectedSnapshotStale(t *testing.T) {
	t.Parallel()
	_, record, _, forecast := costJoinBed(t)
	// This snapshot describes the prospective three-member series, so it is
	// stale against the still two-member record.
	if forecast.Matches(record) {
		t.Fatal("prospective snapshot matched an unjoined record")
	}
	prospective := record
	prospective.Units = append(append([]Unit(nil), record.Units...), Unit{GoalID: "goal-c", Chain: "chain-c", State: UnitJoining,
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, SelectedGroups: []string{"c"}})
	prospective.PrefixTrees = append([]string(nil), forecast.Binding.PrefixTrees...)
	prospective.TipTree = prospective.PrefixTrees[len(prospective.PrefixTrees)-1]
	// A sealed forecast has only joined members; changing its base or selected
	// union must make status label it stale without scanning evidence.
	for index := range prospective.Units {
		prospective.Units[index].State = UnitJoined
	}
	sealed := CostForecast{Binding: CostBinding(prospective, prospective.Units, prospective.PrefixTrees)}
	if !sealed.Matches(prospective) {
		t.Fatal("exact immutable binding is stale")
	}
	prospective.BaseTree = "moved"
	if sealed.Matches(prospective) {
		t.Fatal("moved base retained current snapshot")
	}
	prospective.BaseTree = sealed.Binding.BaseTree
	prospective.SelectedGroups = []string{"new-required-group"}
	if sealed.Matches(prospective) {
		t.Fatal("changed protected selection retained current snapshot")
	}
}
