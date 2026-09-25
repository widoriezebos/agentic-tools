package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

// The migration battery the first testing transition adds to every delivery
// plan. RequireFirstTransition refuses a contract missing any of them.
var prefixPlanTransitionGroups = []string{"policy-protection", "fast-static-build", "section/dispatcher-adapter-and-mission-runner-fixtures",
	"section/goal-cli-fixtures", "section/land-fixtures", "section/adoption-fixtures"}

func prefixPlanContract() testpolicy.Contract {
	contract := testpolicy.Contract{ProjectRisk: testpolicy.ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces: []testpolicy.Surface{
			{ID: "a", Paths: []string{"a.txt"}, Standard: []string{"first"}, Deep: []string{"first-deep"}},
			{ID: "b", Paths: []string{"b.txt"}, Standard: []string{"second"}, Deep: []string{"second-deep"}, Risk: &testpolicy.RiskRaise{Severity: 2}},
		}, Groups: []testpolicy.Group{{ID: "first"}, {ID: "first-deep"}, {ID: "second"}, {ID: "second-deep"}}}
	for _, id := range prefixPlanTransitionGroups {
		contract.Groups = append(contract.Groups, testpolicy.Group{ID: id})
	}
	return contract
}

// strictPrefixPlanner answers only the members it knows on the one prefix
// tree and records every call, so a repeated or unexpected plan is visible.
func strictPrefixPlanner(t *testing.T, changed map[string][]string, firstTransition bool, calls *[]string) func(string, string, string, testpolicy.Mode, []string) (testingPlanOutput, error) {
	contract := prefixPlanContract()
	return func(_, goalID, tree string, mode testpolicy.Mode, admitted []string) (testingPlanOutput, error) {
		*calls = append(*calls, goalID+"/"+string(mode))
		paths, ok := changed[goalID]
		if !ok || tree != "prefix-tree" || (mode != testpolicy.ModeAuto && mode != testpolicy.ModeDeep) {
			t.Errorf("unexpected planner call goal=%s tree=%s mode=%s", goalID, tree, mode)
			return testingPlanOutput{}, fmt.Errorf("unexpected planner call")
		}
		plan, err := testpolicy.Select(contract, testpolicy.SelectionRequest{ChangedPaths: paths, RequestedMode: mode,
			Purpose: testpolicy.PurposeDelivery, BatchRequirements: admitted})
		if err == nil && firstTransition {
			plan, err = testpolicy.RequireFirstTransition(contract, plan)
		}
		return testingPlanOutput{PolicyBaseCommit: "base-commit", CandidateTree: tree, ContractDigest: "candidate-policy",
			BaseContractDigest: "base-policy", Plan: plan, Groups: contract.Groups}, err
	}
}

func prefixPlanContextModes(t *testing.T, decision batch.PrefixDecision) []string {
	t.Helper()
	var contexts []struct{ Plan testpolicy.Plan }
	if err := json.Unmarshal([]byte(decision.PolicyContext), &contexts); err != nil {
		t.Fatal(err)
	}
	modes := []string{}
	for _, context := range contexts {
		modes = append(modes, string(context.Plan.RequestedMode)+">"+string(context.Plan.ExecutedMode))
	}
	return modes
}

func TestBatchPrefixPlanKeepsAutoPlansThatSelectedDeep(t *testing.T) {
	t.Parallel()
	changed := map[string][]string{"goal-a": {"a.txt", "b.txt"}, "goal-b": {"a.txt", "b.txt"}}
	units := []batch.Unit{{GoalID: "goal-a", SelectedGroups: []string{"first"}}, {GoalID: "goal-b", SelectedGroups: []string{"second"}}}
	calls := []string{}
	decision, err := planPrefixDecisionWith("", units, "prefix-tree", strictPrefixPlanner(t, changed, false, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"goal-a/auto", "goal-b/auto"}) {
		t.Fatalf("planner calls = %v, want one auto plan per member", calls)
	}
	if modes := prefixPlanContextModes(t, decision); !slices.Equal(modes, []string{"auto>deep", "auto>deep"}) {
		t.Fatalf("decision context modes = %v, want the honest auto request that executed deep", modes)
	}
	want := []string{"first", "first-deep", "second", "second-deep"}
	if !slices.Equal(decision.Groups, want) {
		t.Fatalf("decision groups = %v, want the deep selection %v", decision.Groups, want)
	}
}

func TestBatchPrefixPlanReplansOnlyMembersWhoseAutoPlanStayedStandard(t *testing.T) {
	t.Parallel()
	changed := map[string][]string{"goal-a": {"a.txt"}, "goal-b": {"b.txt"}}
	units := []batch.Unit{{GoalID: "goal-a", SelectedGroups: []string{"first"}}, {GoalID: "goal-b", SelectedGroups: []string{"second"}}}
	calls := []string{}
	decision, err := planPrefixDecisionWith("", units, "prefix-tree", strictPrefixPlanner(t, changed, false, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"goal-a/auto", "goal-b/auto", "goal-a/deep"}) {
		t.Fatalf("planner calls = %v, want a deep replan for the standard member only", calls)
	}
	if modes := prefixPlanContextModes(t, decision); !slices.Equal(modes, []string{"deep>deep", "auto>deep"}) {
		t.Fatalf("decision context modes = %v", modes)
	}
	if want := []string{"first", "first-deep", "second", "second-deep"}; !slices.Equal(decision.Groups, want) {
		t.Fatalf("decision groups = %v, want the replanned member's deep groups in %v", decision.Groups, want)
	}
}

func TestBatchPrefixPlanReplansAFirstTransitionPlan(t *testing.T) {
	t.Parallel()
	units := []batch.Unit{{GoalID: "goal-a", SelectedGroups: []string{"first"}}}
	calls := []string{}
	decision, err := planPrefixDecisionWith("", units, "prefix-tree", strictPrefixPlanner(t, map[string][]string{"goal-a": {"a.txt"}}, true, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"goal-a/auto", "goal-a/deep"}) {
		t.Fatalf("planner calls = %v, want the raised transition plan replanned deep", calls)
	}
	if modes := prefixPlanContextModes(t, decision); !slices.Equal(modes, []string{"deep>deep"}) {
		t.Fatalf("decision context modes = %v", modes)
	}
	if !slices.Contains(decision.Groups, "first-deep") || !slices.Contains(decision.Groups, "policy-protection") {
		t.Fatalf("transition decision groups = %v, want deep groups and the migration battery", decision.Groups)
	}
}

func TestBatchPrefixPlanKeepsStandardPrefixesStandard(t *testing.T) {
	t.Parallel()
	units := []batch.Unit{{GoalID: "goal-a", SelectedGroups: []string{"first"}}}
	calls := []string{}
	decision, err := planPrefixDecisionWith("", units, "prefix-tree", strictPrefixPlanner(t, map[string][]string{"goal-a": {"a.txt"}}, false, &calls))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(calls, []string{"goal-a/auto"}) || !slices.Equal(decision.Groups, []string{"first"}) {
		t.Fatalf("standard prefix calls=%v groups=%v", calls, decision.Groups)
	}
	if modes := prefixPlanContextModes(t, decision); !slices.Equal(modes, []string{"auto>standard"}) {
		t.Fatalf("decision context modes = %v", modes)
	}
}
