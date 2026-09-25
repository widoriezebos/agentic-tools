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
	expectCommittedGoals(t, &store)
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
	expectCommittedGoals(t, &store)
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

func TestBatchCostSealPublishesExactSnapshotAndRejectsConcurrentClose(t *testing.T) {
	t.Parallel()
	seal := func(store Store, record Record, mutate bool) error {
		first, tip := record.PrefixTrees[0], record.PrefixTrees[1]
		strictReassembly(t, &store,
			expectedAssembly(record.BaseTree, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{first, tip}),
			expectedAssembly(record.BaseTree, []string{"goal-a"}, []string{"chain-a"}, []string{first}),
			expectedAssembly(first, []string{"goal-b"}, []string{"chain-b"}, []string{tip}))
		expectCommittedGoals(t, &store,
			committedGoalReply{module: ModuleRoot(store.root), tree: tip, goal: "goal-a", data: goalBed("goal-a"), present: true},
			committedGoalReply{module: ModuleRoot(store.root), tree: tip, goal: "goal-b", data: goalBed("goal-b"), present: true})
		planned := 0
		plan := func(root, goal, tree string) (testpolicy.Plan, error) {
			want := []string{"goal-a", "goal-b"}
			if planned >= len(want) || root != store.root || goal != want[planned] || tree != tip {
				t.Fatalf("seal plan call %d: root=%q goal=%q tree=%q", planned+1, root, goal, tree)
			}
			planned++
			return testpolicy.Plan{RequiredMode: testpolicy.ModeStandard, SelectedGroups: []string{"proof"}}, nil
		}
		err := SealWithForecast(store, record.BatchID, record.BaseTree, "owner", time.Unix(5, 0), plan,
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
		if planned != 2 {
			t.Fatalf("seal plan calls=%d, want 2", planned)
		}
		return err
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
	firstTree, tip := record.PrefixTrees[0], record.PrefixTrees[1]
	assembly := []expectedReassembly{
		expectedAssembly(record.BaseTree, []string{"goal-a", "goal-b"}, []string{"chain-a", "chain-b"}, []string{firstTree, tip}),
		expectedAssembly(record.BaseTree, []string{"goal-a"}, []string{"chain-a"}, []string{firstTree}),
		expectedAssembly(firstTree, []string{"goal-b"}, []string{"chain-b"}, []string{tip}),
	}
	strictReassembly(t, &store, append(append([]expectedReassembly(nil), assembly...), assembly...)...)
	goals := []committedGoalReply{
		{module: ModuleRoot(store.root), tree: tip, goal: "goal-a", data: goalBed("goal-a"), present: true},
		{module: ModuleRoot(store.root), tree: tip, goal: "goal-b", data: goalBed("goal-b"), present: true},
	}
	expectCommittedGoals(t, &store, append(append([]committedGoalReply(nil), goals...), goals...)...)
	first := PrefixEpisode{DecisionID: "decision-a", Token: strings.Repeat("a", 64), ExpiresAt: time.Unix(30, 0).UTC().Format(time.RFC3339Nano)}
	replacement := PrefixEpisode{DecisionID: "decision-b", Token: strings.Repeat("b", 64), ExpiresAt: time.Unix(40, 0).UTC().Format(time.RFC3339Nano)}
	must(t, store.Update(record.BatchID, func(current *Record) error {
		current.PrefixEpisodes = map[string]PrefixEpisode{"goal-a": first}
		return nil
	}))
	planned := 0
	plan := func(root, goal, tree string) (testpolicy.Plan, error) {
		want := []string{"goal-a", "goal-b", "goal-a", "goal-b"}
		if planned >= len(want) || root != store.root || goal != want[planned] || tree != tip {
			t.Fatalf("seal plan call %d: root=%q goal=%q tree=%q", planned+1, root, goal, tree)
		}
		planned++
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
	if planned != 4 {
		t.Fatalf("seal plan calls=%d, want 4", planned)
	}
	sealed, err := store.Load(record.BatchID)
	must(t, err)
	if sealed.CostForecast == nil || !sealed.CostForecast.Matches(sealed) || sealed.CostForecast.Binding.PrefixEpisodes[0].Token != replacement.Token {
		t.Fatalf("recomputed seal did not bind replacement episode: %+v", sealed.CostForecast)
	}
}
