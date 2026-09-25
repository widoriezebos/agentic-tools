package testpolicy

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

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
	if !containsSubstring(plan.Risk.Reasons, "because no declared path matched") {
		t.Fatalf("fallback cause was not explained: %v", plan.Risk.Reasons)
	}
}

func TestGLEPathActualBatchWildcardOwnsChangedFiles(t *testing.T) {
	t.Parallel()
	contract, err := Load("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"metasystem/cmd/metasystem/landing_batch_land.go", "metasystem/cmd/metasystem/landing_batch_new.go"} {
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{path}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if !contains(plan.AffectedSurfaces, "proof-and-landing") || contains(plan.AffectedSurfaces, "residual") {
			t.Fatalf("%s ownership = %v", path, plan.AffectedSurfaces)
		}
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
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeCanary, Purpose: PurposeDiagnostic, Groups: []string{"app-deep"}})
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

func TestExplicitGroupsRequireDiagnosticCanaryMode(t *testing.T) {
	contract := fixtureContract()
	contract.Groups = append(contract.Groups, Group{ID: "manual-only"})
	requested := []string{"app-deep", "manual-only"}
	wantSelected := []string{"app-deep", "app-unit", "manual-only"}
	for _, mode := range []Mode{ModeAuto, ModeCanary, ModeStandard, ModeDeep} {
		for _, purpose := range []Purpose{PurposeDelivery, PurposeDiagnostic, PurposeCadence} {
			t.Run(string(mode)+"/"+string(purpose), func(t *testing.T) {
				plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: mode,
					Purpose: purpose, Groups: requested})
				if mode == ModeCanary {
					if err != nil || plan.Purpose != PurposeDiagnostic || !reflect.DeepEqual(plan.SelectedGroups, wantSelected) {
						t.Fatalf("canary selection purpose=%s groups=%v err=%v, want diagnostic %v", plan.Purpose, plan.SelectedGroups, err, wantSelected)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "TEST_GROUP_SELECTION_REFUSED") ||
					!strings.Contains(err.Error(), "--mode canary --groups <ids>") {
					t.Fatalf("explicit groups were not refused with the canary fix-it: plan=%+v err=%v", plan, err)
				}
			})
		}
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto,
		Purpose: PurposeDelivery, BatchRequirements: requested})
	if err != nil || plan.Purpose != PurposeDelivery || !reflect.DeepEqual(plan.SelectedGroups, wantSelected) {
		t.Fatalf("batch prefix delivery selection purpose=%s groups=%v err=%v, want delivery %v", plan.Purpose, plan.SelectedGroups, err, wantSelected)
	}
	if !contains(plan.RequiredGroups, "manual-only") || !contains(plan.RequiredGroups, "app-deep") {
		t.Fatalf("batch admitted groups are selected but not required: %+v", plan)
	}
}

func TestMetaSystemGroupSelectionScenario(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	groups := []string{"section/adoption-fixtures", "section/watch-background-jobs-fixtures"}
	for _, mode := range []Mode{ModeCanary, ModeAuto, ModeStandard, ModeDeep} {
		plan, selectErr := Select(contract, SelectionRequest{ChangedPaths: []string{"internal/testpolicy/select.go"},
			RequestedMode: mode, Purpose: PurposeDiagnostic, Groups: groups})
		if mode == ModeCanary {
			want := []string{"policy-canary", "adapter-canary", "command-interface-smoke", "section/adoption-fixtures",
				"section/watch-background-jobs-fixtures", "fast-static-build", "refusal-register-standard"}
			if selectErr != nil || len(plan.SelectedGroups) != len(want) {
				t.Fatalf("canary selected %v of %d groups, err=%v; want %v", plan.SelectedGroups, len(contract.Groups), selectErr, want)
			}
			for _, id := range want {
				if !contains(plan.SelectedGroups, id) {
					t.Fatalf("canary selection omitted %s: %v", id, plan.SelectedGroups)
				}
			}
			t.Logf("mode=%s selected=%d of %d groups", mode, len(plan.SelectedGroups), len(contract.Groups))
			continue
		}
		if selectErr == nil || !strings.Contains(selectErr.Error(), "TEST_GROUP_SELECTION_REFUSED") {
			t.Fatalf("mode=%s selected %d of %d groups instead of refusing: %v", mode, len(plan.SelectedGroups), len(contract.Groups), selectErr)
		}
		t.Logf("mode=%s refused: %v", mode, selectErr)
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

func TestAutoPlanSelectsDeepOnlyWhenADeepRequestWouldMatch(t *testing.T) {
	t.Parallel()
	raised := fixtureContract()
	raised.Surfaces[0].Risk = &RiskRaise{Severity: 2}
	request := SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery, BatchRequirements: []string{"app-unit"}}
	auto, err := Select(raised, request)
	if err != nil {
		t.Fatal(err)
	}
	request.RequestedMode = ModeDeep
	deep, err := Select(raised, request)
	if err != nil {
		t.Fatal(err)
	}
	if !AutoPlanSelectsDeep(auto) {
		t.Fatalf("auto plan that executed deep was not recognised: %+v", auto)
	}
	relabelled := auto
	relabelled.RequestedMode = ModeDeep
	if !reflect.DeepEqual(relabelled, deep) {
		t.Fatalf("auto deep plan differs from the deep request beyond the requested mode:\nauto=%+v\ndeep=%+v", auto, deep)
	}
	if AutoPlanSelectsDeep(deep) {
		t.Fatal("an explicit deep request is not an auto plan")
	}
	standard, err := Select(fixtureContract(), SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil || AutoPlanSelectsDeep(standard) {
		t.Fatalf("standard auto plan claimed deep: plan=%+v err=%v", standard, err)
	}

	transition := fixtureContract()
	for _, id := range firstTransitionGroups {
		transition.Groups = append(transition.Groups, Group{ID: id, Kind: "integration", Adapter: "command", CWD: ".",
			Inputs: []string{"src/**"}, Platforms: []string{"any"}, TargetMS: 1, Argv: []string{"true"}, Format: "exit-status"})
	}
	raisedAuto, err := Select(transition, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err == nil {
		raisedAuto, err = RequireFirstTransition(transition, raisedAuto)
	}
	if err != nil {
		t.Fatal(err)
	}
	if raisedAuto.ExecutedMode != ModeDeep || contains(raisedAuto.SelectedGroups, "app-deep") {
		t.Fatalf("first transition no longer raises a standard selection to deep without its deep groups: %+v", raisedAuto)
	}
	if AutoPlanSelectsDeep(raisedAuto) {
		t.Fatalf("first-transition plan claimed the deep selection it lacks: %+v", raisedAuto)
	}
}
