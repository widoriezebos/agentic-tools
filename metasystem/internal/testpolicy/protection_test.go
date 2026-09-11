package testpolicy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBasePolicyAndBlackBoxGuardsSurviveCandidateRemovalAndInternalAPIChange(t *testing.T) {
	base := fixtureContract()
	candidate := fixtureContract()
	candidate.Groups = candidate.Groups[1:]
	candidate.Always.Canary = nil
	candidate.Surfaces[0].Standard = nil
	protected := ProtectedContract(base, candidate)
	if err := protected.Validate(); err != nil {
		t.Fatal(err)
	}
	plan, err := Select(protected, SelectionRequest{ChangedPaths: []string{"internal/testpolicy/select.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.RequiredGroups, "app-unit") || plan.RequiredMode != ModeDeep {
		t.Fatalf("base guard or protected-policy deep requirement disappeared: %+v", plan)
	}
}

func TestProtectedContractUnionsCandidateExtensionsWithoutReplacingBaseGroups(t *testing.T) {
	base := fixtureContract()
	candidate := fixtureContract()
	candidate.Surfaces[0].Paths = []string{"cmd/new/**"}
	candidate.Surfaces[0].DependsOn = []string{"shared"}
	candidate.Surfaces[0].Standard = []string{"new-command"}
	candidate.Surfaces = append(candidate.Surfaces, Surface{ID: "shared", Paths: []string{"shared/**"}, Standard: []string{"new-command"}, Critical: []string{"new"}})
	candidate.Groups[0].Packages = []string{"tampered"}
	candidate.Groups = append(candidate.Groups, Group{ID: "new-command", Kind: "unit", Adapter: "command", CWD: ".", Inputs: []string{"shared/**"}, Outputs: []string{"reports/new"},
		Tools: []Tool{{ID: "shell", Executable: "sh", VersionArgs: []string{"--version"}}}, Obligations: []string{"new"}, Platforms: []string{"any"}, TargetMS: 1,
		Argv: []string{"sh", "-c", "true"}, Reports: []string{"reports/new"}, Format: "junit-xml", ExpectedTests: []ExpectedTest{{Report: "reports/new/result.xml", Name: "new"}}})

	protected := ProtectedContract(base, candidate)
	if got := protected.Groups[0].Packages[0]; got == "tampered" {
		t.Fatal("candidate rewrote a base-controlled group")
	}
	plan, err := Select(protected, SelectionRequest{ChangedPaths: []string{"shared/value.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(plan.RequiredGroups, "new-command") || !contains(plan.AffectedSurfaces, "shared") {
		t.Fatalf("candidate surface or dependency extension was not protected: %+v", plan)
	}
}

func TestProtectedContractKeepsBaseCPUBudgetAndAcceptsNewGroupBudget(t *testing.T) {
	base, candidate := fixtureContract(), fixtureContract()
	baseBudget, raisedBudget, newBudget := int64(40), int64(400), int64(25)
	base.Groups[0].CPUBudgetSeconds = &baseBudget
	candidate.Groups[0].CPUBudgetSeconds = &raisedBudget
	candidate.Groups = append(candidate.Groups, Group{ID: "new-budgeted", Kind: "unit", Adapter: "go", CWD: ".",
		Inputs: []string{"go.mod", "src/**"}, Platforms: []string{"any"}, TargetMS: 1000, CPUBudgetSeconds: &newBudget,
		Packages: []string{"src"}, Tests: json.RawMessage(`"all"`)})

	protected := ProtectedContract(base, candidate)
	groups := groupMap(protected.Groups)
	if groups["app-unit"].CPUBudgetSeconds == nil || *groups["app-unit"].CPUBudgetSeconds != baseBudget {
		t.Fatalf("candidate raised protected CPU budget: %+v", groups["app-unit"].CPUBudgetSeconds)
	}
	if groups["new-budgeted"].CPUBudgetSeconds == nil || *groups["new-budgeted"].CPUBudgetSeconds != newBudget {
		t.Fatalf("new group lost its candidate CPU budget: %+v", groups["new-budgeted"].CPUBudgetSeconds)
	}
	for _, invalid := range []int64{0, -1} {
		contract := fixtureContract()
		contract.Groups[0].CPUBudgetSeconds = &invalid
		if err := contract.Validate(); err == nil {
			t.Fatalf("invalid CPU budget %d was accepted", invalid)
		}
	}
}

func TestMainConformanceAcceptedPolicyBoundaries(t *testing.T) {
	t.Run("low-risk-standard-excludes-deep-requirements", func(t *testing.T) {
		contract := fixtureContract()
		contract.Surfaces[0].Critical = nil
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if plan.RequiredMode != ModeStandard || contains(plan.RequiredGroups, "app-deep") {
			t.Fatalf("low-risk standard includes unselected deep requirements: %+v", plan)
		}
	})
	t.Run("candidate-surface-risk-raise-survives-policy-union", func(t *testing.T) {
		base, candidate := fixtureContract(), fixtureContract()
		base.Surfaces[0].Critical, candidate.Surfaces[0].Critical = nil, nil
		candidate.Surfaces[0].Risk = &RiskRaise{Severity: 2}
		plan, err := Select(ProtectedContract(base, candidate), SelectionRequest{ChangedPaths: []string{"src/output.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Risk.Severity != 2 || plan.RequiredMode != ModeDeep {
			t.Fatalf("candidate severity increase was lost: %+v", plan)
		}
	})
	t.Run("static-only-contract-cannot-provide-application-tests", func(t *testing.T) {
		contract := fixtureContract()
		contract.Surfaces[0].Critical, contract.Surfaces[0].Standard, contract.Surfaces[0].Deep = nil, []string{"static"}, nil
		contract.Groups = []Group{{ID: "static", Kind: "static", Adapter: "command", CWD: ".", Inputs: []string{"src/**"}, Outputs: []string{}, Tools: []Tool{}, Platforms: []string{"any"}, TargetMS: 1000, Argv: []string{"echo", "ok"}, Format: "exit-status"}}
		contract.Always, contract.Unknown, contract.Cadence = Always{Standard: []string{"static"}}, []string{"static"}, []string{"static"}
		if err := contract.Validate(); err == nil {
			t.Fatal("static-only contract reported valid without a standard application test provider")
		}
	})
	t.Run("candidate-test-addition-survives-policy-union", func(t *testing.T) {
		base, candidate := fixtureContract(), fixtureContract()
		base.Groups[0].Tests = json.RawMessage(`["TestOutput"]`)
		candidate.Groups[0].Tests = json.RawMessage(`["TestOutput","TestNew"]`)
		baseBefore, _ := json.Marshal(base)
		candidateBefore, _ := json.Marshal(candidate)
		merged := ProtectedContract(base, candidate)
		all, names, err := GoTests(merged.Groups[0])
		if err != nil {
			t.Fatal(err)
		}
		if !all && !contains(names, "TestNew") {
			t.Fatalf("candidate required test was dropped: %v", names)
		}
		baseAfter, _ := json.Marshal(base)
		candidateAfter, _ := json.Marshal(candidate)
		if string(baseAfter) != string(baseBefore) || string(candidateAfter) != string(candidateBefore) {
			t.Fatal("protected composition mutated a policy input")
		}
	})
	t.Run("accumulation-scales-cadence-weight-not-depth", func(t *testing.T) {
		contract := fixtureContract()
		contract.Surfaces[0].Critical = nil
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{"src/output.go"}, GoalRisk: GoalRisk{Accumulation: 2}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if plan.RequiredMode != ModeStandard || contains(plan.RequiredGroups, "app-deep") {
			t.Fatalf("a goal's accumulation answer chose per-landing depth: %+v", plan)
		}
		if !containsSubstring(plan.Risk.Reasons, "accumulation=2 scale cadence weight") {
			t.Fatalf("the plan does not say where the goal's answers went: %+v", plan.Risk.Reasons)
		}
	})
}

func containsSubstring(values []string, wanted string) bool {
	for _, value := range values {
		if strings.Contains(value, wanted) {
			return true
		}
	}
	return false
}
