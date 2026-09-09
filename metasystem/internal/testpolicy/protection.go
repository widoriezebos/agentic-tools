package testpolicy

import (
	"encoding/json"
	"strings"
)

// ProtectionProbeCase is frozen public-version-1 fixture data embedded in a
// retained destination engine. The worker, not candidate package tests,
// interprets these literal cases against the candidate executable.
type ProtectionProbeCase struct {
	ID             string   `json:"id"`
	PublicArgv     []string `json:"publicArgv"`
	ExpectedStatus int      `json:"expectedStatus"`
	ResultField    string   `json:"resultField"`
}

// FrozenProtectionProbeCases returns a defensive copy so candidate contract
// composition cannot edit the base engine's negative corpus in memory.
func FrozenProtectionProbeCases() []ProtectionProbeCase {
	values := []ProtectionProbeCase{
		{ID: "remove-required-provider", PublicArgv: []string{"test", "plan"}, ExpectedStatus: 0, ResultField: "groups.policy-protection"},
		{ID: "shrink-dependency-graph", PublicArgv: []string{"test", "plan"}, ExpectedStatus: 0, ResultField: "plan.affectedSurfaces.proof-and-landing"},
		{ID: "lower-coverage-floor", PublicArgv: []string{"test", "plan"}, ExpectedStatus: 1, ResultField: "error.TEST_POLICY_COVERAGE_FLOOR_LOWERED"},
		{ID: "remove-required-test", PublicArgv: []string{"test", "plan"}, ExpectedStatus: 0, ResultField: "groups.policy-protection.tests"},
		{ID: "emit-zero-tests", PublicArgv: []string{"test", "worker"}, ExpectedStatus: 1, ResultField: "groups.literal.collectionComplete"},
		{ID: "forge-component-reuse", PublicArgv: []string{"test", "worker"}, ExpectedStatus: 1, ResultField: "groups.literal.reuseAttempt"},
	}
	result := make([]ProtectionProbeCase, len(values))
	for index, value := range values {
		result[index] = value
		result[index].PublicArgv = append([]string(nil), value.PublicArgv...)
	}
	return result
}

func ProtectedPolicyChange(paths []string) bool {
	for _, path := range paths {
		path = strings.TrimPrefix(path, "./")
		if path == "metasystem/testing.json" || path == "testing.json" ||
			strings.Contains(path, "/internal/testpolicy/") || strings.HasPrefix(path, "internal/testpolicy/") ||
			strings.Contains(path, "/internal/proofrun/test_") || strings.HasPrefix(path, "internal/proofrun/test_") ||
			strings.HasSuffix(path, "coverage-ratchet.json") || strings.HasSuffix(path, "coverage-ratchet-linux.json") ||
			strings.HasSuffix(path, "validate-section-selector.sh") || strings.HasSuffix(path, "commit.sh") || strings.HasSuffix(path, "land.sh") {
			return true
		}
	}
	return false
}

// ProtectedContract keeps every base-controlled black-box group and mapping
// while adding candidate requirements. Candidate policy can raise or extend
// delivery rigor, but cannot remove or rewrite the proof that judges its own
// change.
func ProtectedContract(base, candidate Contract) Contract {
	result := cloneContract(base)
	groupIDs := map[string]int{}
	for index, group := range result.Groups {
		groupIDs[group.ID] = index
	}
	for _, group := range candidate.Groups {
		if index, exists := groupIDs[group.ID]; exists {
			result.Groups[index] = mergeProtectedGroup(result.Groups[index], group)
		} else {
			result.Groups = append(result.Groups, cloneGroup(group))
			groupIDs[group.ID] = len(result.Groups) - 1
		}
	}
	surfaceIDs := map[string]bool{}
	for index, surface := range result.Surfaces {
		surfaceIDs[surface.ID] = true
		for _, addition := range candidate.Surfaces {
			if addition.ID != surface.ID {
				continue
			}
			result.Surfaces[index].Paths = unique(surface.Paths, addition.Paths)
			result.Surfaces[index].DependsOn = unique(surface.DependsOn, addition.DependsOn)
			result.Surfaces[index].Standard = unique(surface.Standard, addition.Standard)
			result.Surfaces[index].Deep = unique(surface.Deep, addition.Deep)
			result.Surfaces[index].Critical = unique(surface.Critical, addition.Critical)
			result.Surfaces[index].CrossCutting = unique(surface.CrossCutting, addition.CrossCutting)
			result.Surfaces[index].Risk = mergeRiskRaise(surface.Risk, addition.Risk)
		}
	}
	for _, surface := range candidate.Surfaces {
		if !surfaceIDs[surface.ID] {
			result.Surfaces = append(result.Surfaces, surface)
			surfaceIDs[surface.ID] = true
		}
	}
	result.Always.Canary = unique(result.Always.Canary, candidate.Always.Canary)
	result.Always.Standard = unique(result.Always.Standard, candidate.Always.Standard)
	result.Unknown = unique(result.Unknown, candidate.Unknown)
	result.Cadence = unique(result.Cadence, candidate.Cadence)
	result.ProjectRisk.Severity = max(result.ProjectRisk.Severity, candidate.ProjectRisk.Severity)
	result.ProjectRisk.Exposure = max(result.ProjectRisk.Exposure, candidate.ProjectRisk.Exposure)
	if reversibilityRank(candidate.ProjectRisk.Reversibility) > reversibilityRank(result.ProjectRisk.Reversibility) {
		result.ProjectRisk.Reversibility = candidate.ProjectRisk.Reversibility
	}
	if detectionRank(candidate.ProjectRisk.Detection) > detectionRank(result.ProjectRisk.Detection) {
		result.ProjectRisk.Detection = candidate.ProjectRisk.Detection
	}
	if recoveryRank(candidate.ProjectRisk.Recovery) > recoveryRank(result.ProjectRisk.Recovery) {
		result.ProjectRisk.Recovery = candidate.ProjectRisk.Recovery
	}
	return result
}

func cloneContract(value Contract) Contract {
	result := value
	result.Surfaces = make([]Surface, len(value.Surfaces))
	for index, surface := range value.Surfaces {
		result.Surfaces[index] = surface
		result.Surfaces[index].Paths = append([]string(nil), surface.Paths...)
		result.Surfaces[index].DependsOn = append([]string(nil), surface.DependsOn...)
		result.Surfaces[index].Standard = append([]string(nil), surface.Standard...)
		result.Surfaces[index].Deep = append([]string(nil), surface.Deep...)
		result.Surfaces[index].Critical = append([]string(nil), surface.Critical...)
		result.Surfaces[index].CrossCutting = append([]string(nil), surface.CrossCutting...)
		if surface.Risk != nil {
			copyRisk := *surface.Risk
			result.Surfaces[index].Risk = &copyRisk
		}
	}
	result.Groups = make([]Group, len(value.Groups))
	for index, group := range value.Groups {
		result.Groups[index] = cloneGroup(group)
	}
	result.Always.Canary = append([]string(nil), value.Always.Canary...)
	result.Always.Standard = append([]string(nil), value.Always.Standard...)
	result.Unknown = append([]string(nil), value.Unknown...)
	result.Cadence = append([]string(nil), value.Cadence...)
	return result
}

func cloneGroup(value Group) Group {
	result := value
	result.Inputs = append([]string(nil), value.Inputs...)
	result.Outputs = append([]string(nil), value.Outputs...)
	result.Tools = append([]Tool(nil), value.Tools...)
	for index := range result.Tools {
		result.Tools[index].VersionArgs = append([]string(nil), value.Tools[index].VersionArgs...)
	}
	result.ExternalInputs = append([]ExternalInput(nil), value.ExternalInputs...)
	result.Obligations = append([]string(nil), value.Obligations...)
	result.Platforms = append([]string(nil), value.Platforms...)
	result.Packages = append([]string(nil), value.Packages...)
	result.Tests = append(json.RawMessage(nil), value.Tests...)
	result.Argv = append([]string(nil), value.Argv...)
	result.Reports = append([]string(nil), value.Reports...)
	result.ExpectedTests = append([]ExpectedTest(nil), value.ExpectedTests...)
	result.Env = map[string]string{}
	for key, item := range value.Env {
		result.Env[key] = item
	}
	return result
}

func mergeProtectedGroup(base, candidate Group) Group {
	result := cloneGroup(base)
	result.Inputs = unique(base.Inputs, candidate.Inputs)
	result.Outputs = unique(base.Outputs, candidate.Outputs)
	result.Obligations = unique(base.Obligations, candidate.Obligations)
	result.Platforms = unique(base.Platforms, candidate.Platforms)
	result.Packages = unique(base.Packages, candidate.Packages)
	result.Reports = unique(base.Reports, candidate.Reports)
	result.ExpectedTests = uniqueExpectedTests(base.ExpectedTests, candidate.ExpectedTests)
	result.Race = base.Race || candidate.Race
	result.Coverage = base.Coverage || candidate.Coverage
	if candidate.TargetMS > result.TargetMS {
		result.TargetMS = candidate.TargetMS
	}
	result.Tests = mergeGoTests(base.Tests, candidate.Tests)
	for key, value := range candidate.Env {
		if _, protected := result.Env[key]; !protected {
			result.Env[key] = value
		}
	}
	for _, tool := range candidate.Tools {
		found := false
		for _, existing := range result.Tools {
			found = found || existing.ID == tool.ID
		}
		if !found {
			result.Tools = append(result.Tools, tool)
		}
	}
	for _, external := range candidate.ExternalInputs {
		found := false
		for _, existing := range result.ExternalInputs {
			found = found || existing.ID == external.ID
		}
		if !found {
			result.ExternalInputs = append(result.ExternalInputs, external)
		}
	}
	return result
}

func mergeGoTests(base, candidate json.RawMessage) json.RawMessage {
	if len(base) == 0 {
		return append(json.RawMessage(nil), candidate...)
	}
	if len(candidate) == 0 {
		return append(json.RawMessage(nil), base...)
	}
	baseGroup, candidateGroup := Group{Tests: base}, Group{Tests: candidate}
	baseAll, baseNames, baseErr := GoTests(baseGroup)
	candidateAll, candidateNames, candidateErr := GoTests(candidateGroup)
	if baseErr != nil || candidateErr != nil {
		return append(json.RawMessage(nil), base...)
	}
	if baseAll || candidateAll {
		return json.RawMessage(`"all"`)
	}
	encoded, _ := json.Marshal(unique(baseNames, candidateNames))
	return encoded
}

func uniqueExpectedTests(sets ...[]ExpectedTest) []ExpectedTest {
	seen := map[string]bool{}
	var result []ExpectedTest
	for _, values := range sets {
		for _, value := range values {
			key := value.Report + "\x00" + value.Classname + "\x00" + value.Name
			if !seen[key] {
				seen[key] = true
				result = append(result, value)
			}
		}
	}
	return result
}

func mergeRiskRaise(base, candidate *RiskRaise) *RiskRaise {
	if base == nil && candidate == nil {
		return nil
	}
	result := RiskRaise{}
	if base != nil {
		result = *base
	}
	if candidate == nil {
		return &result
	}
	result.Severity = max(result.Severity, candidate.Severity)
	result.Exposure = max(result.Exposure, candidate.Exposure)
	if reversibilityRank(candidate.Reversibility) > reversibilityRank(result.Reversibility) {
		result.Reversibility = candidate.Reversibility
	}
	if detectionRank(candidate.Detection) > detectionRank(result.Detection) {
		result.Detection = candidate.Detection
	}
	if recoveryRank(candidate.Recovery) > recoveryRank(result.Recovery) {
		result.Recovery = candidate.Recovery
	}
	return &result
}

func unique(sets ...[]string) []string {
	seen := map[string]bool{}
	var result []string
	for _, values := range sets {
		for _, value := range values {
			if !seen[value] {
				seen[value] = true
				result = append(result, value)
			}
		}
	}
	return result
}
