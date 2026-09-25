package batch

import (
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

type ordinaryJoinOperation struct {
	kind, base, unitTree, planningRoot, goal, planningTree string
	units                                                  []Unit
	unit                                                   Unit
	prefixes                                               []string
	plan                                                   testpolicy.Plan
	err                                                    error
}

type ordinaryJoinBed struct {
	t     *testing.T
	root  string
	base  string
	store Store
	mu    sync.Mutex
	wants []ordinaryJoinOperation
	seen  int
	joins int
}

func newOrdinaryJoinBed(t *testing.T) *ordinaryJoinBed {
	t.Helper()
	bed := &ordinaryJoinBed{t: t, root: t.TempDir(), base: testCommit(200)}
	bed.store = NewStore(bed.root, scriptedProber{})
	must(t, bed.store.Create(Record{Schema: 1, BatchID: testBatchID, BaseTree: bed.base, TipTree: bed.base, State: StateOpen}))
	bed.store.reassembly.assemble = func(base string, units []Unit) ([]string, error) {
		want, err := bed.next("assemble", base, units, Unit{}, "")
		if err != nil {
			return nil, err
		}
		return slices.Clone(want.prefixes), want.err
	}
	bed.store.planJoinedUnit = func(base string, unit Unit, tree string, plan func(string, string, string) (testpolicy.Plan, error)) (testpolicy.Plan, error) {
		want, err := bed.next("plan", base, nil, unit, tree)
		if err != nil {
			return testpolicy.Plan{}, err
		}
		got, err := plan(want.planningRoot, want.goal, want.planningTree)
		if err != nil || !reflect.DeepEqual(got, want.plan) {
			bed.t.Errorf("planning reply: got %+v, err=%v; want %+v", got, err, want.plan)
			return testpolicy.Plan{}, fmt.Errorf("unexpected planning reply: %v", err)
		}
		return cloneJoinPlan(want.plan), nil
	}
	t.Cleanup(func() {
		bed.mu.Lock()
		defer bed.mu.Unlock()
		if bed.seen != len(bed.wants) {
			t.Errorf("join repository calls=%d, want %d; unconsumed=%+v", bed.seen, len(bed.wants), bed.wants[bed.seen:])
		}
	})
	return bed
}

func (bed *ordinaryJoinBed) next(kind, base string, units []Unit, unit Unit, tree string) (ordinaryJoinOperation, error) {
	bed.mu.Lock()
	defer bed.mu.Unlock()
	if bed.seen >= len(bed.wants) {
		bed.t.Errorf("undeclared join repository call: %s base=%q units=%+v unit=%+v tree=%q", kind, base, units, unit, tree)
		return ordinaryJoinOperation{}, fmt.Errorf("undeclared join repository call")
	}
	want := bed.wants[bed.seen]
	bed.seen++
	if want.kind != kind || want.base != base || !reflect.DeepEqual(want.units, units) || !reflect.DeepEqual(want.unit, unit) || want.unitTree != tree {
		bed.t.Errorf("join repository call %d: got %s base=%q units=%+v unit=%+v tree=%q; want %+v", bed.seen, kind, base, units, unit, tree, want)
		return ordinaryJoinOperation{}, fmt.Errorf("unexpected join repository call")
	}
	return want, nil
}

func (bed *ordinaryJoinBed) expectJoin(incoming Unit, reply testpolicy.Plan) string {
	bed.t.Helper()
	live := bed.liveWith(incoming)
	prefixes := make([]string, len(live))
	for index := range prefixes {
		prefixes[index] = testCommit(201 + index)
	}
	unitTree := testCommit(201)
	if bed.joins > 0 {
		unitTree = testCommit(301 + bed.joins)
	}
	bed.mu.Lock()
	bed.wants = append(bed.wants,
		ordinaryJoinOperation{kind: "assemble", base: bed.base, units: live, prefixes: prefixes},
		ordinaryJoinOperation{kind: "assemble", base: bed.base, units: []Unit{live[len(live)-1]}, prefixes: []string{unitTree}},
		ordinaryJoinOperation{kind: "plan", base: bed.base, unit: live[len(live)-1], unitTree: unitTree,
			planningRoot: filepath.Join(bed.root, "planning", incoming.GoalID), goal: incoming.GoalID, planningTree: unitTree, plan: cloneJoinPlan(reply)},
	)
	bed.mu.Unlock()
	bed.joins++
	return unitTree
}

func (bed *ordinaryJoinBed) expectConflict(incoming Unit) {
	bed.t.Helper()
	live := bed.liveWith(incoming)
	bed.mu.Lock()
	bed.wants = append(bed.wants, ordinaryJoinOperation{kind: "assemble", base: bed.base, units: live,
		err: &assemblyConflict{GoalID: incoming.GoalID, Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit "+incoming.GoalID+" paths a.go, b.go")}})
	bed.mu.Unlock()
}

func (bed *ordinaryJoinBed) liveWith(incoming Unit) []Unit {
	bed.t.Helper()
	record := load(bed.t, bed.store)
	live := make([]Unit, 0, len(record.Units)+1)
	for _, unit := range record.Units {
		if unit.State == UnitJoined {
			live = append(live, unit)
		}
	}
	incoming.State = UnitJoining
	return append(live, incoming)
}

func cloneJoinPlan(plan testpolicy.Plan) testpolicy.Plan {
	plan.AffectedSurfaces = slices.Clone(plan.AffectedSurfaces)
	plan.RequiredGroups = slices.Clone(plan.RequiredGroups)
	plan.SelectedGroups = slices.Clone(plan.SelectedGroups)
	plan.Omissions = slices.Clone(plan.Omissions)
	plan.Uncertainty = slices.Clone(plan.Uncertainty)
	plan.Risk.Reasons = slices.Clone(plan.Risk.Reasons)
	plan.Stages = slices.Clone(plan.Stages)
	for index := range plan.Stages {
		plan.Stages[index].Groups = slices.Clone(plan.Stages[index].Groups)
	}
	return plan
}
