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
	bed := assemblyFixture(t)
	record := bed.record
	prefixes, err := assembleUnits(bed.root, bed.base, record.Units)
	must(t, err)
	record.PrefixTrees, record.TipTree = prefixes, prefixes[len(prefixes)-1]
	store := NewStore(bed.root, nil)
	must(t, store.Create(record))
	incoming := Unit{GoalID: "goal-c", Chain: "chain-c", State: UnitJoining,
		Claim: Claim{Machine: "seat", Lineage: "l", Epoch: 1, Revision: 9, AccountingRevision: 7}, SelectedGroups: []string{"c"}}
	prospective := append(append([]Unit(nil), record.Units...), incoming)
	all, err := assembleUnits(bed.root, bed.base, prospective)
	must(t, err)
	forecast := CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(3, 0).UTC().Format(time.RFC3339Nano),
		Currency: "snapshot-not-revalidated", Binding: CostBinding(record, prospective, all)}
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
	store, record, incoming, forecast := costJoinBed(t)
	forecast.Binding.Members[0].Claim.AccountingRevision++
	handedOver := false
	err := PublishJoinWithAdmissionForecast(store, record.BatchID, incoming, "owner", time.Unix(3, 0),
		func(_, _, _ string) (testpolicy.Plan, error) {
			return testpolicy.Plan{SelectedGroups: []string{"c"}}, nil
		},
		func() error { handedOver = true; return nil },
		func(_ string, unit Unit) (JoinAdmission, error) {
			return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified"}, nil
		}, forecast)
	if err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") || handedOver {
		t.Fatalf("changed claim crossed cost CAS: err=%v handover=%t", err, handedOver)
	}
	current, loadErr := store.Load(record.BatchID)
	must(t, loadErr)
	if len(current.Units) != len(record.Units) || !reflect.DeepEqual(current.PrefixTrees, record.PrefixTrees) {
		t.Fatalf("failed CAS published incoming member: %+v", current)
	}
}

func TestBatchCostJoinAndClosureRejectReplacedPrefixEpisode(t *testing.T) {
	t.Parallel()
	store, record, incoming, forecast := costJoinBed(t)
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
			return testpolicy.Plan{SelectedGroups: []string{"c"}}, nil
		},
		func() error { handedOver = true; return nil },
		func(_ string, unit Unit) (JoinAdmission, error) {
			return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified"}, nil
		}, forecast)
	if err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") || handedOver {
		t.Fatalf("replaced episode crossed join CAS: err=%v handedOver=%t", err, handedOver)
	}
	if err := CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(3, 0), forecast); err == nil || !strings.Contains(err.Error(), "BATCH_COST_INPUT_MOVED") {
		t.Fatalf("replaced episode closed admission using stale forecast: %v", err)
	}
	current, err = store.Load(record.BatchID)
	must(t, err)
	if len(current.Units) != len(record.Units) || current.ClosedReason != "" {
		t.Fatalf("stale episode moved custody or closed admission: %+v", current)
	}
	recomputed := forecast
	recomputed.Binding = CostBinding(current, prospective, forecast.Binding.PrefixTrees)
	if reflect.DeepEqual(recomputed.Binding, forecast.Binding) || recomputed.Binding.PrefixEpisodes[0].Token != replacement.Token {
		t.Fatalf("recomputed forecast did not bind replacement episode: old=%+v new=%+v", forecast.Binding, recomputed.Binding)
	}
	must(t, CloseAdmissionForCost(store, record.BatchID, "owner", time.Unix(3, 0), recomputed))
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

func TestBatchCostSealPublishesExactSnapshotAndRejectsConcurrentClose(t *testing.T) {
	t.Parallel()
	plan := func(_, _, _ string) (testpolicy.Plan, error) {
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"proof"}}, nil
	}
	seal := func(store Store, record Record, mutate bool) error {
		return SealWithForecast(store, record.BatchID, record.BaseTree, "owner", time.Unix(5, 0), plan,
			func(candidate Record) (CostForecast, error) {
				if mutate {
					if err := store.Update(record.BatchID, func(current *Record) error {
						current.ClosedReason = "concurrent-close"
						return nil
					}); err != nil {
						return CostForecast{}, err
					}
				}
				return CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(5, 0).UTC().Format(time.RFC3339Nano),
					Currency: "snapshot-not-revalidated", Binding: CostBinding(candidate, joinedUnits(candidate.Units), candidate.PrefixTrees)}, nil
			})
	}
	store, record, _, _ := costJoinBed(t)
	must(t, seal(store, record, false))
	sealed, err := store.Load(record.BatchID)
	must(t, err)
	if sealed.State != StateSealed || sealed.CostForecast == nil || !sealed.CostForecast.Matches(sealed) {
		t.Fatalf("seal lost or misbound historical forecast: %+v", sealed)
	}
	store2, record2, _, _ := costJoinBed(t)
	err = seal(store2, record2, true)
	if err == nil || !strings.Contains(err.Error(), "BATCH_SEAL_CHANGED_DURING_GATE") {
		t.Fatalf("concurrent closure crossed seal forecast CAS: %v", err)
	}
	current, loadErr := store2.Load(record2.BatchID)
	must(t, loadErr)
	if current.State != StateOpen || current.CostForecast != nil {
		t.Fatalf("changed seal published stale forecast: %+v", current)
	}
}

func TestBatchCostSealRejectsEpisodeChangedDuringForecast(t *testing.T) {
	t.Parallel()
	store, record, _, _ := costJoinBed(t)
	first := PrefixEpisode{DecisionID: "decision-a", Token: strings.Repeat("a", 64), ExpiresAt: time.Unix(30, 0).UTC().Format(time.RFC3339Nano)}
	replacement := PrefixEpisode{DecisionID: "decision-b", Token: strings.Repeat("b", 64), ExpiresAt: time.Unix(40, 0).UTC().Format(time.RFC3339Nano)}
	must(t, store.Update(record.BatchID, func(current *Record) error {
		current.PrefixEpisodes = map[string]PrefixEpisode{"goal-a": first}
		return nil
	}))
	plan := func(_, _, _ string) (testpolicy.Plan, error) {
		return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"proof"}}, nil
	}
	mutated := false
	forecast := func(candidate Record) (CostForecast, error) {
		if !mutated {
			mutated = true
			if err := store.Update(record.BatchID, func(current *Record) error {
				current.PrefixEpisodes["goal-a"] = replacement
				return nil
			}); err != nil {
				return CostForecast{}, err
			}
		}
		return CostForecast{SchemaVersion: 1, ObservedAt: time.Unix(5, 0).UTC().Format(time.RFC3339Nano),
			Currency: "snapshot-not-revalidated", Binding: CostBinding(candidate, joinedUnits(candidate.Units), candidate.PrefixTrees)}, nil
	}
	err := SealWithForecast(store, record.BatchID, record.BaseTree, "owner", time.Unix(5, 0), plan, forecast)
	if err == nil || !strings.Contains(err.Error(), "BATCH_SEAL_CHANGED_DURING_GATE") {
		t.Fatalf("episode mutation crossed seal CAS: %v", err)
	}
	current, err := store.Load(record.BatchID)
	must(t, err)
	if current.State != StateOpen || current.CostForecast != nil || current.PrefixEpisodes["goal-a"] != replacement {
		t.Fatalf("failed seal published stale snapshot: %+v", current)
	}
	must(t, SealWithForecast(store, record.BatchID, record.BaseTree, "owner", time.Unix(6, 0), plan, forecast))
	sealed, err := store.Load(record.BatchID)
	must(t, err)
	if sealed.CostForecast == nil || !sealed.CostForecast.Matches(sealed) || sealed.CostForecast.Binding.PrefixEpisodes[0].Token != replacement.Token {
		t.Fatalf("recomputed seal did not bind replacement episode: %+v", sealed.CostForecast)
	}
}
