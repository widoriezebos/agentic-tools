package testpolicy

import "testing"

func TestSelectionIncludesConsumersCrossCuttingAndUnknown(t *testing.T) {
	contract := fixtureContract()
	contract.Surfaces = append(contract.Surfaces,
		Surface{ID: "consumer", Paths: []string{"consumer/**"}, DependsOn: []string{"app"}, Standard: []string{"consumer-unit"}, Deep: []string{"consumer-deep"}, CrossCutting: []string{"consumer-cross"}})
	contract.Groups = append(contract.Groups,
		Group{ID: "consumer-unit", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"consumer/**"}, Outputs: []string{}, Tools: []Tool{}, Obligations: []string{}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"consumer"}, Tests: []byte(`"all"`)},
		Group{ID: "consumer-deep", Kind: "integration", Adapter: "go", CWD: ".", Inputs: []string{"consumer/**"}, Outputs: []string{}, Tools: []Tool{}, Obligations: []string{"consumer-cross"}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"consumer"}, Tests: []byte(`"all"`)})
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, GoalRisk: GoalRisk{Accumulation: 2}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.AffectedSurfaces, "consumer") || !contains(plan.RequiredGroups, "consumer-deep") {
		t.Fatalf("consumer/cross-cutting selection missing: %+v", plan)
	}
	unknown, err := Select(contract, SelectionRequest{ChangedPaths: []string{"unowned/file"}, RequestedMode: ModeAuto, Purpose: PurposeDiagnostic})
	if err != nil {
		t.Fatal(err)
	}
	if len(unknown.Uncertainty) != 1 || !contains(unknown.SelectedGroups, "app-unit") {
		t.Fatalf("unknown fallback = %+v", unknown)
	}
}

func TestFirstTransitionRetainsOriginalAndSchemaTwoProtectionGroups(t *testing.T) {
	contract := fixtureContract()
	for _, id := range firstTransitionGroups {
		contract.Groups = append(contract.Groups, Group{ID: id, Kind: "integration", Adapter: "command", CWD: ".",
			Inputs: []string{"src/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"true"}, Format: "exit-status"})
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	plan, err = RequireFirstTransition(contract, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range firstTransitionGroups {
		if !contains(plan.RequiredGroups, id) || !contains(plan.SelectedGroups, id) {
			t.Fatalf("first transition omitted %s: %+v", id, plan)
		}
	}
	if plan.RequiredMode != ModeDeep || plan.ExecutedMode != ModeDeep {
		t.Fatalf("first transition did not retain its complete migration battery: %+v", plan)
	}
	candidate := contract
	candidate.Groups = candidate.Groups[:len(candidate.Groups)-1]
	if _, err := RequireFirstTransition(candidate, plan); err == nil {
		t.Fatal("first transition accepted a missing adoption/protection owner")
	}
}

func TestModesDoNotLowerRequiredRisk(t *testing.T) {
	contract := fixtureContract()
	contract.ProjectRisk.Severity = 2
	if _, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeStandard, Purpose: PurposeDelivery}); err == nil {
		t.Fatal("standard request lowered a deep delivery requirement")
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeStandard, Purpose: PurposeDiagnostic})
	if err != nil || plan.ExecutedMode != ModeStandard {
		t.Fatalf("standard diagnostic was unavailable: plan=%+v err=%v", plan, err)
	}
}

func TestCanaryIsRequiredAndExplicitDiagnosticGetsAStage(t *testing.T) {
	contract := fixtureContract()
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeStandard, Purpose: PurposeDiagnostic, Groups: []string{"app-deep"}})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.RequiredGroups, "app-unit") {
		t.Fatalf("canary group was omitted from required groups: %+v", plan)
	}
	last := plan.Stages[len(plan.Stages)-1]
	if last.ID != "diagnostic" || !contains(last.Groups, "app-deep") {
		t.Fatalf("explicit diagnostic group has no executable stage: %+v", plan.Stages)
	}
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
