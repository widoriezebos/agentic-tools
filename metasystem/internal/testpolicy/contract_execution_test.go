package testpolicy

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestLegacyContractWireOmitsExecutionFields(t *testing.T) {
	t.Parallel()
	contract := fixtureContract()
	encoded, err := json.Marshal(contract)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	var groups []map[string]json.RawMessage
	if err := json.Unmarshal(wire["groups"], &groups); err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		for _, field := range []string{"resources", "freshness", "freshnessMaxAgeMs", "packageSelection"} {
			if _, present := group[field]; present {
				t.Fatalf("legacy group emitted schema-2 field %s: %s", field, group[field])
			}
		}
	}
	if _, err := Decode(encoded); err != nil {
		t.Fatalf("legacy contract did not round trip: %v", err)
	}
}

func executionFixtureContract() Contract {
	contract := fixtureContract()
	contract.SchemaVersion = ExecutionContractSchemaVersion
	for index := range contract.Groups {
		contract.Groups[index].Phase = "acceptance"
		contract.Groups[index].EnvironmentMode = "inherit"
	}
	contract.Groups[0].Phase = "admission"
	contract.Groups[1].Requires = []string{"app-unit"}
	return contract
}

func TestGLEBuildTagsFollowGoConstraintTagGrammar(t *testing.T) {
	t.Parallel()
	for _, tags := range [][]string{{"go1.24"}, {"386"}, {"feature_x"}, {"étiquette"}, {"feature.one", "feature_two"}} {
		contract := executionFixtureContract()
		contract.Groups[0].BuildTags = tags
		if err := contract.Validate(); err != nil {
			t.Errorf("valid Go build tags %v refused: %v", tags, err)
		}
	}
	for _, tags := range [][]string{{""}, {"feature-x"}, {"feature/name"}, {"feature name"}, {"feature!name"}, {"feature_x", "feature_x"}} {
		contract := executionFixtureContract()
		contract.Groups[0].BuildTags = tags
		if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "buildTags") {
			t.Errorf("invalid Go build tags %v accepted: %v", tags, err)
		}
	}
}

func TestGLEPrerequisiteValidationAndSelectionClosure(t *testing.T) {
	t.Parallel()
	contract := executionFixtureContract()
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	missing := cloneContract(contract)
	missing.Groups[1].Requires = []string{"absent"}
	if err := missing.Validate(); err == nil || !strings.Contains(err.Error(), "requires missing group absent") {
		t.Fatalf("missing prerequisite: %v", err)
	}
	cyclic := cloneContract(contract)
	cyclic.Groups[0].Requires = []string{"app-deep"}
	if err := cyclic.Validate(); err == nil || !strings.Contains(err.Error(), "prerequisite cycle") {
		t.Fatalf("cycle: %v", err)
	}
	invalidEnvironment := cloneContract(contract)
	invalidEnvironment.Groups[0].EnvironmentMode = "explicit"
	invalidEnvironment.Groups[0].Env = map[string]string{"BAD=KEY": "value"}
	if err := invalidEnvironment.Validate(); err == nil || !strings.Contains(err.Error(), "invalid entry") {
		t.Fatalf("invalid explicit environment: %v", err)
	}
	plan, err := Select(contract, SelectionRequest{RequestedMode: ModeCanary, Purpose: PurposeDiagnostic, Groups: []string{"app-deep"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.SelectedGroups, []string{"app-deep", "app-unit"}) {
		t.Fatalf("selected prerequisite closure %v", plan.SelectedGroups)
	}
	if len(plan.Stages) != 2 || !reflect.DeepEqual(plan.Stages[0].Groups, []string{"app-unit"}) || !reflect.DeepEqual(plan.Stages[1].Groups, []string{"app-deep"}) {
		t.Fatalf("prerequisite not ordered before dependent: %+v", plan.Stages)
	}
}

func TestGLEAdmissionPlanCannotReduceDeliveryFloor(t *testing.T) {
	t.Parallel()
	contract := executionFixtureContract()
	contract.Surfaces[0].Standard = []string{"app-deep"}
	selection, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/change.go"}, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	admission, err := AdmissionPlan(contract, selection)
	if err != nil {
		t.Fatal(err)
	}
	if admission.Purpose != PurposeDiagnostic || !reflect.DeepEqual(admission.SelectedGroups, []string{"app-unit"}) || !reflect.DeepEqual(selection.RequiredGroups, []string{"app-deep", "app-unit"}) {
		t.Fatalf("admission=%+v delivery=%+v", admission, selection)
	}
	legacy := fixtureContract()
	if err := legacy.Validate(); err != nil {
		t.Fatalf("legacy contract: %v", err)
	}
	legacySelection, err := Select(legacy, SelectionRequest{ChangedPaths: []string{"src/change.go"}, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	legacyAdmission, err := AdmissionPlan(legacy, legacySelection)
	if err != nil || !reflect.DeepEqual(legacyAdmission, legacySelection) {
		t.Fatalf("legacy admission must retain complete selection: %+v %v", legacyAdmission, err)
	}
	legacy.Groups[1].Requires = []string{"app-unit"}
	if err := legacy.Validate(); err == nil || !strings.Contains(err.Error(), "require schemaVersion") {
		t.Fatalf("new fields under legacy schema: %v", err)
	}
}

func TestGLEProtectedPrerequisiteEdgeAndExecutionContract(t *testing.T) {
	t.Parallel()
	base := executionFixtureContract()
	base.Always.Canary = nil
	base.Surfaces[0].Standard = []string{"app-deep"}
	base.Surfaces[0].Critical = []string{"app-recovery"}
	base.Groups[1].EnvironmentMode = "explicit"
	base.Groups[1].Env = map[string]string{"PATH": "/usr/bin"}
	candidate := cloneContract(base)
	candidate.Groups[1].Requires = nil
	candidate.Groups[1].Phase = "acceptance"
	candidate.Groups[1].EnvironmentMode = "inherit"
	merged := ProtectedContract(base, candidate)
	if err := merged.Validate(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(merged.Groups[1].Requires, []string{"app-unit"}) || merged.Groups[1].EnvironmentMode != "explicit" {
		t.Fatalf("protected edge or environment removed: %+v", merged.Groups[1])
	}
	plan, err := Select(merged, SelectionRequest{ChangedPaths: []string{"src/change.go"}, Purpose: PurposeDelivery})
	if err != nil || !reflect.DeepEqual(plan.RequiredGroups, []string{"app-deep", "app-unit"}) {
		t.Fatalf("removed candidate edge lowered protected delivery: %+v %v", plan, err)
	}
	legacy := fixtureContract()
	promoted := ProtectedContract(legacy, base)
	if promoted.SchemaVersion != ExecutionContractSchemaVersion || promoted.Groups[0].Phase != "admission" || promoted.Groups[1].EnvironmentMode != "explicit" {
		t.Fatalf("version promotion: %+v", promoted)
	}
}
