package testpolicy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestContractRejectsMissingNoopAndUnknownFields(t *testing.T) {
	valid := fixtureContract()
	encoded, err := json.Marshal(valid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(encoded); err != nil {
		t.Fatalf("valid contract refused: %v", err)
	}
	unknown := strings.Replace(string(encoded), `"schemaVersion":1`, `"schemaVersion":1,"surprise":true`, 1)
	if _, err := Decode([]byte(unknown)); err == nil {
		t.Fatal("unknown contract field was accepted")
	}
	noop := fixtureContract()
	noop.Groups[0].Adapter = "command"
	noop.Groups[0].Kind = "unit"
	noop.Groups[0].Argv = []string{"true"}
	noop.Groups[0].Format = "junit-xml"
	noop.Groups[0].Reports = []string{"reports"}
	noop.Groups[0].ExpectedTests = []ExpectedTest{{Report: "reports/test.xml", Name: "test"}}
	noop.Groups[0].Packages, noop.Groups[0].Tests = nil, nil
	if err := noop.Validate(); err == nil {
		t.Fatal("blanket success command was accepted")
	}
}

func TestContractValidatesFallbackSurface(t *testing.T) {
	t.Run("missing-surface", func(t *testing.T) {
		contract := fixtureContract()
		contract.Fallback = "missing"
		if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "missing fallback surface missing") {
			t.Fatalf("missing fallback surface was accepted: %v", err)
		}
	})
	t.Run("only-fallback-may-have-no-paths", func(t *testing.T) {
		contract := fixtureContract()
		contract.Fallback = "residual"
		contract.Surfaces = append(contract.Surfaces, Surface{ID: "residual", Paths: []string{}})
		contract.Surfaces[0].Paths = []string{}
		if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "has no paths") {
			t.Fatalf("non-fallback surface without paths was accepted: %v", err)
		}
	})
	t.Run("fallback-must-have-no-paths", func(t *testing.T) {
		contract := fixtureContract()
		contract.Fallback = "app"
		if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "fallback surface app") || !strings.Contains(err.Error(), "paths") {
			t.Fatalf("fallback surface with paths was accepted: %v", err)
		}
	})
	t.Run("fallback-must-have-no-dependencies", func(t *testing.T) {
		contract := fixtureContract()
		contract.Fallback = "residual"
		contract.Surfaces = append(contract.Surfaces, Surface{ID: "residual", Paths: []string{}, DependsOn: []string{"app"}})
		if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "fallback surface residual") || !strings.Contains(err.Error(), "dependsOn") {
			t.Fatalf("fallback surface with dependsOn was accepted: %v", err)
		}
	})
	t.Run("pathless-dependency-free-fallback-is-valid", func(t *testing.T) {
		contract := fixtureContract()
		contract.Fallback = "residual"
		contract.Surfaces = append(contract.Surfaces, Surface{ID: "residual", Paths: []string{}})
		if err := contract.Validate(); err != nil {
			t.Fatalf("pathless dependency-free fallback surface was refused: %v", err)
		}
	})
}

func TestMetaSystemContractPinsFallbackDeclarationAndGroupUnions(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	// A contract may declare no fallback. In that state, no surface owns by
	// exclusion and the residual declaration remains absent.
	if contract.Fallback == "" {
		for _, surface := range contract.Surfaces {
			if len(surface.Paths) == 0 {
				t.Fatalf("contract without a fallback has pathless surface %s", surface.ID)
			}
			if surface.ID == "residual" {
				t.Fatal("contract without a fallback declares the residual surface")
			}
		}
		return
	}
	var residual Surface
	standard, deep, critical := []string{}, []string{}, []string{}
	for _, surface := range contract.Surfaces {
		if surface.ID == contract.Fallback {
			residual = surface
			continue
		}
		standard = appendUnique(standard, surface.Standard...)
		deep = appendUnique(deep, surface.Deep...)
		critical = appendUnique(critical, surface.Critical...)
	}
	if residual.ID == "" {
		t.Fatalf("fallback surface %q is absent", contract.Fallback)
	}
	if len(residual.Paths) != 0 || len(residual.DependsOn) != 0 {
		t.Fatalf("fallback surface must own by exclusion with empty paths and dependsOn: %+v", residual)
	}
	if !reflect.DeepEqual(residual.Standard, standard) || !reflect.DeepEqual(residual.Deep, deep) || !reflect.DeepEqual(residual.Critical, critical) {
		t.Fatalf("fallback surface groups do not equal the other surface unions: residual=%+v standard=%v deep=%v critical=%v", residual, standard, deep, critical)
	}
	for _, surface := range contract.Surfaces {
		if surface.ID == contract.Fallback || surface.Risk == nil {
			continue
		}
		if residual.Risk == nil || residual.Risk.Severity < surface.Risk.Severity || residual.Risk.Exposure < surface.Risk.Exposure ||
			reversibilityRank(residual.Risk.Reversibility) < reversibilityRank(surface.Risk.Reversibility) ||
			detectionRank(residual.Risk.Detection) < detectionRank(surface.Risk.Detection) ||
			recoveryRank(residual.Risk.Recovery) < recoveryRank(surface.Risk.Recovery) {
			t.Fatalf("fallback surface risk raise is lower than surface %s: fallback=%+v surface=%+v", surface.ID, residual.Risk, surface.Risk)
		}
	}
}

func appendUnique(values []string, additions ...string) []string {
	seen := make(map[string]bool, len(values)+len(additions))
	for _, value := range values {
		seen[value] = true
	}
	for _, addition := range additions {
		if !seen[addition] {
			values = append(values, addition)
			seen[addition] = true
		}
	}
	return values
}

func TestHCL31ContractOwnsFivePackages(t *testing.T) {
	contract := hclContract(t)
	changed := []string{
		"metasystem/internal/humanauthority/authority.go",
		"metasystem/internal/channel/question.go",
		"metasystem/internal/governance/types.go",
		"metasystem/internal/counselor/register.go",
		"metasystem/internal/refusal/register.go",
		"metasystem/cmd/metasystem/channel_verbs_test.go",
		"metasystem/cmd/metasystem/goalsync_mutations_test.go",
		"metasystem/cmd/metasystem/landing_verbs_test.go",
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: changed, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 0 {
		t.Fatalf("carried owner paths are uncertain: %v", plan.Uncertainty)
	}
	for _, group := range []string{"authority-standard", "refusal-register-standard", "carry-goal-standard", "carry-landing-standard", "carry-plumbing-standard"} {
		if !contains(plan.SelectedGroups, group) {
			t.Errorf("carried owner selection omitted %s", group)
		}
	}
}

func TestHCL34PlanExecutesEveryFixture(t *testing.T) {
	contract := hclContract(t)
	module := filepath.Clean(filepath.Join("..", ".."))
	type owner struct {
		changed string
		pkg     string
		scan    string
	}
	owners := []owner{
		{"metasystem/internal/goal/hcl_carry_test.go", "internal/goal", filepath.Join(module, "internal", "goal")},
		{"metasystem/internal/landing/hcl_carried_test.go", "internal/landing", filepath.Join(module, "internal", "landing")},
		{"metasystem/internal/channel/question_test.go", "internal/channel", filepath.Join(module, "internal", "channel")},
		{"metasystem/internal/dispatch/review_reference_test.go", "internal/dispatch", filepath.Join(module, "internal", "dispatch")},
		{"metasystem/internal/refusal/register_test.go", "internal/refusal", filepath.Join(module, "internal", "refusal")},
		{"metasystem/internal/config/budget_test.go", "internal/config", filepath.Join(module, "internal", "config")},
		{"metasystem/internal/counselor/register_test.go", "internal/counselor", filepath.Join(module, "internal", "counselor")},
		{"metasystem/internal/steward/health_test.go", "internal/steward", filepath.Join(module, "internal", "steward")},
		{"metasystem/internal/testpolicy/contract_test.go", "internal/testpolicy", filepath.Join(module, "internal", "testpolicy")},
		{"metasystem/cmd/metasystem/channel_verbs_test.go", "cmd/metasystem", filepath.Join(module, "cmd", "metasystem", "channel_verbs_test.go")},
		{"metasystem/cmd/metasystem/goalsync_mutations_test.go", "cmd/metasystem", filepath.Join(module, "cmd", "metasystem", "goalsync_mutations_test.go")},
		{"metasystem/cmd/metasystem/landing_verbs_test.go", "cmd/metasystem", filepath.Join(module, "cmd", "metasystem", "landing_verbs_test.go")},
	}
	for _, owner := range owners {
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{owner.changed}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatalf("select %s: %v", owner.changed, err)
		}
		if len(plan.Uncertainty) != 0 {
			t.Fatalf("select %s: %v", owner.changed, plan.Uncertainty)
		}
		for _, name := range hclFixtureNames(t, owner.scan) {
			if !hclPlanCovers(contract, plan, owner.pkg, name) {
				t.Errorf("owner %s does not execute %s", owner.changed, name)
			}
		}
	}

	// The two required negative drives prove that explicit lists cannot lose a
	// newly added fixture silently.
	for _, negative := range []owner{owners[0], owners[10]} {
		plan, err := Select(contract, SelectionRequest{ChangedPaths: []string{negative.changed}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
		if err != nil {
			t.Fatal(err)
		}
		if hclPlanCovers(contract, plan, negative.pkg, "TestHCL99NegativeInventoryProbe") {
			t.Errorf("negative inventory probe was unexpectedly covered for %s", negative.changed)
		}
	}

	allowedCommandFiles := map[string]bool{"channel_verbs_test.go": true, "goalsync_mutations_test.go": true, "landing_verbs_test.go": true}
	commandFiles, err := filepath.Glob(filepath.Join(module, "cmd", "metasystem", "*_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range commandFiles {
		if !allowedCommandFiles[filepath.Base(path)] && len(hclFixtureNames(t, path)) != 0 {
			t.Errorf("command fixtures must live in the three owned files, found TestHCL in %s", filepath.Base(path))
		}
	}
}

func hclContract(t *testing.T) Contract {
	t.Helper()
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return contract
}

var hclTestDeclaration = regexp.MustCompile(`(?m)^func (TestHCL[[:alnum:]_]+)\(`)

func hclFixtureNames(t *testing.T, path string) []string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{path}
	if info.IsDir() {
		paths, err = filepath.Glob(filepath.Join(path, "*_test.go"))
		if err != nil {
			t.Fatal(err)
		}
	}
	seen := map[string]bool{}
	for _, source := range paths {
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		for _, match := range hclTestDeclaration.FindAllSubmatch(data, -1) {
			seen[string(match[1])] = true
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func hclPlanCovers(contract Contract, plan Plan, pkg, name string) bool {
	selected := set(plan.SelectedGroups)
	for _, group := range contract.Groups {
		if !selected[group.ID] || !contains(group.Packages, pkg) {
			continue
		}
		all, names, err := GoTests(group)
		if err == nil && (all || contains(names, name)) {
			return true
		}
	}
	return false
}

func TestOutputOwnershipPreservesTrackedBinAndInputIntegrity(t *testing.T) {
	contract := fixtureContract()
	contract.Groups[0].Outputs = []string{"../escape"}
	if err := contract.Validate(); err == nil {
		t.Fatal("escaping output path was accepted")
	}
	contract = fixtureContract()
	contract.Groups[0].Inputs = []string{"bin/tool", "src/**"}
	contract.Groups[0].Outputs = []string{"reports"}
	if err := contract.Validate(); err != nil {
		t.Fatalf("tracked bin input was blanket-excluded: %v", err)
	}
	contract = fixtureContract()
	contract.Groups[0].Inputs = append(contract.Groups[0].Inputs, "reports/**")
	contract.Groups[0].Outputs = []string{"reports"}
	if err := contract.Validate(); err == nil {
		t.Fatal("overlapping input and output ownership was accepted")
	}
}

func TestSectionGroupNamespaceIsAccepted(t *testing.T) {
	contract := fixtureContract()
	contract.Groups = append(contract.Groups, Group{ID: "section/example", Kind: "integration", Adapter: "section", CWD: ".", Inputs: []string{"scripts/**"}, Platforms: []string{"any"}, TargetMS: 1000, Section: "example"})
	contract.Cadence = append(contract.Cadence, "section/example")
	if err := contract.Validate(); err != nil {
		t.Fatalf("canonical section group id refused: %v", err)
	}
}

func TestExternalInputsRequireExplicitAbsoluteOrEnvironmentLocators(t *testing.T) {
	contract := fixtureContract()
	contract.Groups[0].ExternalInputs = []ExternalInput{{ID: "tool-config", Path: "${TOOL_HOME}/config.json"}, {ID: "system-ca", Path: "/etc/hosts"}}
	if err := contract.Validate(); err != nil {
		t.Fatalf("declared external input locators were refused: %v", err)
	}
	contract.Groups[0].ExternalInputs[0].Path = "relative/config.json"
	if err := contract.Validate(); err == nil {
		t.Fatal("undeclared relative external input authority was accepted")
	}
}

func TestCoverageRequiresWholePackageTestInventory(t *testing.T) {
	contract := fixtureContract()
	contract.Groups[0].Coverage = true
	contract.Groups[0].Tests = json.RawMessage(`["TestOutput"]`)
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "tests=all") {
		t.Fatalf("named subset claimed whole-package coverage: %v", err)
	}
	contract.Groups[0].Tests = json.RawMessage(`"all"`)
	if err := contract.Validate(); err != nil {
		t.Fatalf("whole-package coverage contract was refused: %v", err)
	}
}

func TestMetaSystemContractKeepsMixedPackageCoverageExplicit(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	groups := groupMap(contract.Groups)
	for _, id := range []string{"goal-full-coverage", "missionrunner-full-coverage"} {
		group, ok := groups[id]
		if !ok {
			t.Fatalf("explicit coverage group %s is absent", id)
		}
		all, _, err := GoTests(group)
		if err != nil || !all || !group.Coverage {
			t.Fatalf("coverage group %s is not a whole-package measurement: group=%+v err=%v", id, group, err)
		}
		if !contains(contract.Cadence, id) {
			t.Fatalf("coverage group %s is absent from cadence", id)
		}
	}

	standard, err := Select(contract, SelectionRequest{ChangedPaths: []string{"metasystem/internal/goal/stop.go"}, RequestedMode: ModeStandard, Purpose: PurposeDiagnostic})
	if err != nil {
		t.Fatal(err)
	}
	if contains(standard.SelectedGroups, "goal-full-coverage") || contains(standard.SelectedGroups, "missionrunner-full-coverage") {
		t.Fatalf("focused standard selection expanded to whole-package coverage: %+v", standard)
	}
	deep, err := Select(contract, SelectionRequest{ChangedPaths: []string{"metasystem/internal/testpolicy/select.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if deep.RequiredMode != ModeDeep {
		t.Fatalf("a protected policy change did not plan deep: %+v", deep)
	}
	// Whole-package coverage is measured at the weight cadence, never per
	// landing (slice 1 of plans/suite-speed-plan.md).
	if contains(deep.RequiredGroups, "goal-full-coverage") || contains(deep.RequiredGroups, "missionrunner-full-coverage") {
		t.Fatalf("per-landing deep selection pulled in whole-package coverage: %+v", deep)
	}
	cadence, err := Select(contract, SelectionRequest{ChangedPaths: []string{"metasystem/internal/testpolicy/select.go"}, RequestedMode: ModeDeep, Purpose: PurposeCadence})
	if err != nil {
		t.Fatal(err)
	}
	if !contains(cadence.SelectedGroups, "goal-full-coverage") || !contains(cadence.SelectedGroups, "missionrunner-full-coverage") {
		t.Fatalf("cadence omitted the package coverage floors: %+v", cadence)
	}
}

func TestMetaSystemContractSelectsStaticProofForGoalRecords(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Select(contract, SelectionRequest{
		ChangedPaths:  []string{"metasystem/records/misc/goal-records-owned-by-the-testing-contract.md"},
		RequestedMode: ModeAuto,
		Purpose:       PurposeDelivery,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 0 {
		t.Fatalf("goal record has unresolved delivery impact: %+v", plan)
	}
	if !contains(plan.AffectedSurfaces, "goal-records") {
		t.Fatalf("goal record did not select its owning surface: %+v", plan)
	}
	for _, id := range []string{"section/static-contract-audits", "section/return-schema-fixtures"} {
		if !contains(plan.SelectedGroups, id) || !contains(plan.RequiredGroups, id) {
			t.Fatalf("goal record delivery omitted static group %s: %+v", id, plan)
		}
	}
}

func TestMetaSystemContractOwnsDeliveryBoundaryAndSelectsFastBeforeBroadProof(t *testing.T) {
	data, err := os.ReadFile("../../testing.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	boundary := []string{
		"metasystem/cmd/metasystem/adoption_comparison_test.go", "metasystem/cmd/metasystem/cap_contract_test.go",
		"metasystem/cmd/metasystem/config_verbs.go", "metasystem/cmd/metasystem/dispatch_verbs.go",
		"metasystem/cmd/metasystem/dispatch_verbs_test.go", "metasystem/cmd/metasystem/goal_test.go",
		"metasystem/cmd/metasystem/landing_verbs.go", "metasystem/cmd/metasystem/landing_verbs_test.go",
		"metasystem/cmd/metasystem/main.go", "metasystem/cmd/metasystem/process_verbs.go",
		"metasystem/cmd/metasystem/process_verbs_test.go", "metasystem/cmd/metasystem/proof_run.go",
		"metasystem/cmd/metasystem/proof_run_test.go", "metasystem/cmd/metasystem/run.go",
		"metasystem/cmd/metasystem/runtime_setup.go", "metasystem/cmd/metasystem/runtime_setup_test.go",
		"metasystem/cmd/metasystem/test.go", "metasystem/cmd/metasystem/test_protection.go",
		"metasystem/cmd/metasystem/test_test.go", "metasystem/cmd/metasystem/testing_pruning_test.go",
		"metasystem/cmd/metasystem/up.go", "metasystem/docs/collaboration.md",
		"metasystem/docs/design/design-obligation-gate.md", "metasystem/docs/orchestration.md",
		"metasystem/docs/project-adaptation.md", "metasystem/docs/project-rules.md",
		"metasystem/internal/behaviorsurface/policy.v2.json", "metasystem/internal/behaviorsurface/policy_test.go",
		"metasystem/internal/config/model_alias_validate_test.go", "metasystem/internal/config/validate.go",
		"metasystem/internal/config/validate_test.go", "metasystem/internal/dispatch/budget.go",
		"metasystem/internal/dispatch/build.go", "metasystem/internal/dispatch/census_wait.go",
		"metasystem/internal/dispatch/census_wait_test.go", "metasystem/internal/dispatch/critique_chain_test.go",
		"metasystem/internal/dispatch/decisions_test.go", "metasystem/internal/dispatch/proof_attempt_test.go",
		"metasystem/internal/dispatch/stop.go", "metasystem/internal/dispatch/testing_contract_test.go",
		"metasystem/internal/gaterun/weight.go", "metasystem/internal/gaterun/weight_test.go",
		"metasystem/internal/gittree/detached.go", "metasystem/internal/gittree/detached_test.go",
		"metasystem/internal/gittree/snapshotscope.go", "metasystem/internal/gittree/snapshotscope_test.go",
		"metasystem/internal/goal/stop.go", "metasystem/internal/landing/observe.go",
		"metasystem/internal/landing/proof_receipt_test.go", "metasystem/internal/landing/receipt.go",
		"metasystem/internal/landing/testing.go", "metasystem/internal/landing/testing_test.go",
		"metasystem/internal/landing/tierone.go", "metasystem/internal/proofrun/attempt.go",
		"metasystem/internal/proofrun/attempt_test.go", "metasystem/internal/proofrun/coverage.go",
		"metasystem/internal/proofrun/coverage_script_test.go", "metasystem/internal/proofrun/coverage_test.go",
		"metasystem/internal/proofrun/evidence.go", "metasystem/internal/proofrun/execution_context.go",
		"metasystem/internal/proofrun/execution_context_test.go", "metasystem/internal/proofrun/freeze.go",
		"metasystem/internal/proofrun/launcher.go", "metasystem/internal/proofrun/launcher_test.go",
		"metasystem/internal/proofrun/manifest.go", "metasystem/internal/proofrun/record.go",
		"metasystem/internal/proofrun/stop.go", "metasystem/internal/proofrun/test_build.go",
		"metasystem/internal/proofrun/test_command.go", "metasystem/internal/proofrun/test_command_test.go",
		"metasystem/internal/proofrun/test_cost.go", "metasystem/internal/proofrun/test_cost_test.go",
		"metasystem/internal/proofrun/test_go.go", "metasystem/internal/proofrun/test_result.go",
		"metasystem/internal/proofrun/test_result_test.go", "metasystem/internal/proofrun/test_section.go",
		"metasystem/internal/proofrun/watchdog.go", "metasystem/internal/proofrun/witness_gate_test.go",
		"metasystem/internal/refusal/register.go", "metasystem/internal/stateroot/owner.go",
		"metasystem/internal/steward/identity.go", "metasystem/internal/steward/rearm_resolver.go",
		"metasystem/internal/steward/rearm_test.go", "metasystem/internal/steward/validation_window.go",
		"metasystem/internal/steward/validation_window_test.go", "metasystem/internal/stoptransition/families.go",
		"metasystem/internal/stoptransition/proof_attempt_inventory_test.go", "metasystem/internal/testpolicy/contract.go",
		"metasystem/internal/testpolicy/contract_test.go", "metasystem/internal/testpolicy/metasystem.go",
		"metasystem/internal/testpolicy/protection.go", "metasystem/internal/testpolicy/protection_test.go",
		"metasystem/internal/testpolicy/risk.go", "metasystem/internal/testpolicy/select.go",
		"metasystem/internal/testpolicy/select_test.go", "metasystem/internal/validate/recertification.go",
		"metasystem/metasystem.conf", "metasystem/scripts/adopt-fixture-helpers.sh",
		"metasystem/scripts/adopt-fixtures.sh", "metasystem/scripts/adopt.sh",
		"metasystem/scripts/agents/commit.sh", "metasystem/scripts/agents/coverage-delta.sh",
		"metasystem/scripts/agents/dispatch-fixtures.sh", "metasystem/scripts/agents/dispatch.sh",
		"metasystem/scripts/agents/fixture-budget.sh", "metasystem/scripts/agents/go-gate.sh",
		"metasystem/scripts/agents/land.sh", "metasystem/scripts/agents/landing-classes.json",
		"metasystem/scripts/agents/witness-gate.sh", "metasystem/scripts/validate-metasystem.sh",
		"metasystem/testing.json",
		"development/project-rules-local.md", "metasystem/memory/instruction-ledger.md",
		"metasystem/memory/receipts.log", "metasystem/plans/application-testing-contract-design.md",
		"metasystem/plans/coordinator-loop-investigation.md", "metasystem/plans/coordinator-loop-prevention-design-r2-dispositions.md",
		"metasystem/plans/coordinator-loop-prevention-design.md", "metasystem/plans/coordinator-loop-prevention-progress.md",
		"metasystem/plans/coordinator-loop-prevention-testing-design-r1-dispositions.md",
		"metasystem/plans/coordinator-loop-prevention-testing-design-r2-dispositions.md",
		"metasystem/plans/coordinator-loop-prevention-testing-design-r3-dispositions.md",
		"metasystem/plans/coordinator-loop-prevention-testing-design.md",
	}
	plan, err := Select(contract, SelectionRequest{ChangedPaths: boundary, GoalRisk: GoalRisk{Severity: 3, Novelty: 2, Exposure: 2, Accumulation: 3}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Uncertainty) != 0 {
		t.Fatalf("complete accumulated delivery boundary has unowned paths: %v", plan.Uncertainty)
	}
	if !contains(plan.RequiredGroups, "fast-static-build") {
		t.Fatalf("delivery omitted the fast static build: %+v", plan)
	}
	// The two coverage groups run at cadence only (slice 1 of
	// plans/suite-speed-plan.md): even the riskiest goal touching the whole
	// delivery boundary pays them at the weight cadence, not per landing. The
	// big process sections left every deep list too; they still run here only
	// because surfaces in this boundary name them as critical providers.
	for _, id := range []string{"goal-full-coverage", "missionrunner-full-coverage", "section/go-engine-gate"} {
		if contains(plan.SelectedGroups, id) {
			t.Fatalf("delivery pulled in cadence-only group %s: %+v", id, plan)
		}
	}
	for _, surface := range contract.Surfaces {
		for _, id := range surface.Deep {
			if id == "section/adoption-fixtures" || id == "section/supervision-and-census-fixtures" || id == "section/dispatcher-adapter-and-mission-runner-fixtures" {
				t.Fatalf("surface %s still lists the cadence-only section %s as deep", surface.ID, id)
			}
		}
	}
	cadence, err := Select(contract, SelectionRequest{ChangedPaths: boundary, RequestedMode: ModeDeep, Purpose: PurposeCadence})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"goal-full-coverage", "missionrunner-full-coverage", "section/dispatcher-adapter-and-mission-runner-fixtures",
		"section/adoption-fixtures", "section/supervision-and-census-fixtures"} {
		if !contains(cadence.SelectedGroups, id) {
			t.Fatalf("cadence lost %s: %+v", id, cadence)
		}
	}

	ordinary, err := Select(contract, SelectionRequest{ChangedPaths: []string{"metasystem/cmd/metasystem/goal_test.go"}, RequestedMode: ModeAuto, Purpose: PurposeDelivery})
	if err != nil {
		t.Fatal(err)
	}
	ordinary, err = RequireFirstTransition(contract, ordinary)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(ordinary.SelectedGroups, "fast-static-build") || contains(ordinary.SelectedGroups, "section/go-engine-gate") ||
		contains(ordinary.SelectedGroups, "goal-full-coverage") || contains(ordinary.SelectedGroups, "missionrunner-full-coverage") {
		t.Fatalf("first transition did not keep fast proof distinct from the full Go battery: %+v", ordinary)
	}
}

func TestIncompleteAdoptionTemplateCannotReportReady(t *testing.T) {
	data, err := IncompleteTemplate()
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil || value["tailoringRequired"] != true {
		t.Fatalf("incomplete template is not explicit: value=%v err=%v", value, err)
	}
	if _, err := Decode(data); err == nil || !strings.Contains(err.Error(), "TEST_CONTRACT_REQUIRED") {
		t.Fatalf("incomplete template reported ready: %v", err)
	}
}

func fixtureContract() Contract {
	return Contract{SchemaVersion: 1,
		ProjectRisk: ProjectRisk{Severity: 1, Exposure: 1, Reversibility: "revert", Detection: "immediate", Recovery: "bounded"},
		Surfaces:    []Surface{{ID: "app", Paths: []string{"src/**"}, Standard: []string{"app-unit"}, Deep: []string{"app-deep"}, Critical: []string{"app-output"}}},
		Groups: []Group{
			{ID: "app-unit", Kind: "unit", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "src/**"}, Outputs: []string{}, Tools: []Tool{}, Obligations: []string{"app-output"}, Platforms: []string{"any"}, TargetMS: 1000, Packages: []string{"src"}, Tests: json.RawMessage(`"all"`)},
			{ID: "app-deep", Kind: "integration", Adapter: "go", CWD: ".", Inputs: []string{"go.mod", "src/**"}, Outputs: []string{}, Tools: []Tool{}, Obligations: []string{"app-recovery"}, Platforms: []string{"any"}, TargetMS: 2000, Packages: []string{"src"}, Tests: json.RawMessage(`["TestRestart"]`)},
		},
		Always: Always{Canary: []string{"app-unit"}, Standard: []string{}}, Unknown: []string{"app-unit"}, Cadence: []string{"app-unit", "app-deep"}}
}
