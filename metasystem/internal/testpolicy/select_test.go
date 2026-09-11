package testpolicy

import "testing"

func TestSelectionUsesFallbackForUnownedPath(t *testing.T) {
	contract := fixtureContract()
	contract.Fallback = "residual"
	contract.Surfaces = append(contract.Surfaces, Surface{ID: "residual", Paths: []string{}, Standard: []string{"app-deep"}})

	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"unowned/file"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 0 {
		t.Fatalf("fallback selection raised uncertainty: %v", plan.Uncertainty)
	}
	if !contains(plan.AffectedSurfaces, "residual") || !contains(plan.SelectedGroups, "app-deep") {
		t.Fatalf("fallback groups were not selected: %+v", plan)
	}
}

func TestSelectionWithoutFallbackKeepsUnownedPathUncertain(t *testing.T) {
	contract := fixtureContract()
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"unowned/file"}, RequestedMode: ModeAuto, Purpose: PurposeDiagnostic})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 1 || plan.Uncertainty[0] != "no surface owns changed path unowned/file" {
		t.Fatalf("unowned path uncertainty changed: %v", plan.Uncertainty)
	}
}

func TestFallbackNeverMatchesOwnedPath(t *testing.T) {
	contract := fixtureContract()
	contract.Fallback = "residual"
	contract.Surfaces = append(contract.Surfaces, Surface{ID: "residual", Paths: []string{"src/**"}, Standard: []string{"app-deep"}})

	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 0 || !contains(plan.AffectedSurfaces, "app") || contains(plan.AffectedSurfaces, "residual") {
		t.Fatalf("owned path affected its fallback surface: %+v", plan)
	}
}

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
	raised := fixtureContract()
	raised.Surfaces[0].Risk = &RiskRaise{Severity: 2}
	if _, err := Select(raised, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeStandard, Purpose: PurposeDelivery}); err == nil {
		t.Fatal("standard request lowered a surface-raised deep delivery requirement")
	}
	ordinary := fixtureContract()
	ordinaryPlan, ordinaryErr := Select(ordinary, SelectionRequest{ChangedPaths: []string{"src/output.go"}, GoalRisk: GoalRisk{Severity: 3, Novelty: 3, Exposure: 3, Accumulation: 3},
		RequestedMode: ModeStandard, Purpose: PurposeDelivery})
	if ordinaryErr != nil || ordinaryPlan.RequiredMode != ModeStandard || ordinaryPlan.ExecutedMode != ModeStandard {
		t.Fatalf("a goal's risk answers raised the per-landing depth: plan=%+v err=%v", ordinaryPlan, ordinaryErr)
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

func TestDepthIsDecidedByTheChangeNotTheGoal(t *testing.T) {
	riskiest := GoalRisk{Severity: 3, Novelty: 3, Exposure: 3, Accumulation: 3}
	t.Run("ordinary-change-under-the-riskiest-goal-is-standard", func(t *testing.T) {
		plan, err := Select(fixtureContract(), SelectionRequest{ChangedPaths: []string{"src/output.go"}, GoalRisk: riskiest, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if plan.RequiredMode != ModeStandard || plan.ExecutedMode != ModeStandard || contains(plan.SelectedGroups, "app-deep") {
			t.Fatalf("goal answers chose depth: %+v", plan)
		}
		if plan.Risk.Severity != 1 || !containsSubstring(plan.Risk.Reasons, "severity=3 novelty=3 exposure=3 accumulation=3 scale cadence weight") {
			t.Fatalf("plan risk must be the change's own, with the goal's answers named: %+v", plan.Risk)
		}
	})
	t.Run("surface-raise-is-deep", func(t *testing.T) {
		contract := fixtureContract()
		contract.Surfaces[0].Risk = &RiskRaise{Severity: 2}
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil || plan.RequiredMode != ModeDeep || !contains(plan.SelectedGroups, "app-deep") {
			t.Fatalf("surface raise did not plan deep: plan=%+v err=%v", plan, err)
		}
	})
	t.Run("unowned-path-is-deep", func(t *testing.T) {
		plan, err := Select(fixtureContract(), SelectionRequest{ChangedPaths: []string{"unowned/file"}, RequestedMode: ModeAuto, Purpose: PurposeDiagnostic})
		if err != nil || plan.RequiredMode != ModeDeep || len(plan.Uncertainty) != 1 {
			t.Fatalf("unowned path did not plan deep with uncertainty: plan=%+v err=%v", plan, err)
		}
	})
	t.Run("explicit-deep-runs-deep", func(t *testing.T) {
		plan, err := Select(fixtureContract(), SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeDeep, Purpose: PurposeDelivery})
		if err != nil || plan.ExecutedMode != ModeDeep || !contains(plan.SelectedGroups, "app-deep") {
			t.Fatalf("explicit deep request was not honoured: plan=%+v err=%v", plan, err)
		}
	})
	t.Run("cadence-still-selects-the-whole-battery", func(t *testing.T) {
		plan, err := Select(fixtureContract(), SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeDeep, Purpose: PurposeCadence})
		if err != nil || !contains(plan.SelectedGroups, "app-deep") || !contains(plan.SelectedGroups, "app-unit") {
			t.Fatalf("cadence lost a battery group: plan=%+v err=%v", plan, err)
		}
	})
}
