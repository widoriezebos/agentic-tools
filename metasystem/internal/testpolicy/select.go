package testpolicy

import (
	"fmt"
	"sort"
	"strings"
)

type Purpose string
type Mode string

const (
	PurposeDelivery   Purpose = "delivery"
	PurposeDiagnostic Purpose = "diagnostic"
	PurposeCadence    Purpose = "cadence"
	ModeAuto          Mode    = "auto"
	ModeCanary        Mode    = "canary"
	ModeStandard      Mode    = "standard"
	ModeDeep          Mode    = "deep"
)

type SelectionRequest struct {
	ChangedPaths  []string
	GoalRisk      GoalRisk
	RequestedMode Mode
	Purpose       Purpose
	Groups        []string
}

type Stage struct {
	ID     string   `json:"id"`
	Groups []string `json:"groups"`
}

type Omission struct {
	Group  string `json:"group"`
	Reason string `json:"reason"`
}

type Plan struct {
	Purpose          Purpose        `json:"purpose"`
	RequestedMode    Mode           `json:"requestedMode"`
	RequiredMode     Mode           `json:"requiredMode"`
	ExecutedMode     Mode           `json:"executedMode"`
	AffectedSurfaces []string       `json:"affectedSurfaces"`
	RequiredGroups   []string       `json:"requiredGroups"`
	SelectedGroups   []string       `json:"selectedGroups"`
	Stages           []Stage        `json:"stages"`
	Omissions        []Omission     `json:"omissions"`
	Risk             RiskAssessment `json:"risk"`
	Uncertainty      []string       `json:"uncertainty,omitempty"`
}

var firstTransitionGroups = []string{
	"policy-protection",
	"fast-static-build",
	"section/dispatcher-adapter-and-mission-runner-fixtures",
	"section/goal-cli-fixtures",
	"section/land-fixtures",
	"section/adoption-fixtures",
}

// RequireFirstTransition adds the one-time protected migration battery to a
// delivery plan whose trusted destination has no version-1 contract yet. It
// names the existing fast/policy, dispatcher, goal, receipt and adoption
// owners; it is not a general bootstrap mode and cannot lower later policy.
func RequireFirstTransition(contract Contract, plan Plan) (Plan, error) {
	groups := groupMap(contract.Groups)
	selected, required := set(plan.SelectedGroups), set(plan.RequiredGroups)
	transition := map[string]bool{}
	for _, id := range firstTransitionGroups {
		if _, ok := groups[id]; !ok {
			return Plan{}, fmt.Errorf("initial schema-2 transition requires testing group %s", id)
		}
		selected[id], required[id], transition[id] = true, true, true
	}
	plan.RequiredMode, plan.ExecutedMode = ModeDeep, ModeDeep
	plan.RequiredGroups, plan.SelectedGroups = keys(required), keys(selected)
	for _, stage := range plan.Stages {
		for _, id := range stage.Groups {
			delete(transition, id)
		}
	}
	plan.Stages = appendStage(plan.Stages, "initial-transition", transition)
	plan.Omissions = nil
	for id := range groups {
		if !selected[id] {
			plan.Omissions = append(plan.Omissions, Omission{Group: id, Reason: "outside-impact"})
		}
	}
	sort.Slice(plan.Omissions, func(i, j int) bool { return plan.Omissions[i].Group < plan.Omissions[j].Group })
	return plan, nil
}

func Select(contract Contract, request SelectionRequest) (Plan, error) {
	if request.RequestedMode == "" {
		request.RequestedMode = ModeAuto
	}
	if request.Purpose == "" {
		request.Purpose = PurposeDelivery
	}
	if request.RequestedMode != ModeAuto && request.RequestedMode != ModeCanary && request.RequestedMode != ModeStandard && request.RequestedMode != ModeDeep {
		return Plan{}, fmt.Errorf("test mode must be auto, canary, standard, or deep")
	}
	if request.Purpose != PurposeDelivery && request.Purpose != PurposeDiagnostic && request.Purpose != PurposeCadence {
		return Plan{}, fmt.Errorf("test purpose must be delivery, diagnostic, or cadence")
	}
	if request.RequestedMode == ModeCanary && request.Purpose == PurposeDelivery {
		return Plan{}, fmt.Errorf("canary mode is diagnostic and cannot satisfy delivery")
	}
	byID := make(map[string]Surface, len(contract.Surfaces))
	consumers := make(map[string][]string)
	for _, surface := range contract.Surfaces {
		byID[surface.ID] = surface
		for _, provider := range surface.DependsOn {
			consumers[provider] = append(consumers[provider], surface.ID)
		}
	}
	affected := map[string]bool{}
	var uncertainty []string
	for _, path := range request.ChangedPaths {
		matched := false
		for _, surface := range contract.Surfaces {
			for _, pattern := range surface.Paths {
				if matchesPath(pattern, path) {
					affected[surface.ID], matched = true, true
				}
			}
		}
		if !matched {
			uncertainty = append(uncertainty, "no surface owns changed path "+path)
		}
	}
	queue := keys(affected)
	for len(queue) > 0 {
		provider := queue[0]
		queue = queue[1:]
		for _, consumer := range consumers[provider] {
			if !affected[consumer] {
				affected[consumer] = true
				queue = append(queue, consumer)
			}
		}
	}
	var surfaces []Surface
	for _, id := range keys(affected) {
		surfaces = append(surfaces, byID[id])
	}
	risk := assessRisk(contract, surfaces, request.GoalRisk)
	protected := ProtectedPolicyChange(request.ChangedPaths)
	requiredMode := ModeStandard
	if requiresDeep(risk, protected) {
		requiredMode = ModeDeep
	}
	if len(uncertainty) > 0 {
		requiredMode = ModeDeep
	}
	groups := groupMap(contract.Groups)
	standard := set(contract.Always.Standard)
	deep := map[string]bool{}
	obligations := map[string]bool{}
	for _, surface := range surfaces {
		addAll(standard, surface.Standard)
		addAll(deep, surface.Deep)
		addAll(obligations, surface.Critical)
		if request.GoalRisk.Accumulation >= 2 {
			addAll(obligations, surface.CrossCutting)
		}
	}
	for obligation := range obligations {
		for _, provider := range providers(groups, obligation) {
			standard[provider] = true
			if requiredMode == ModeDeep {
				deep[provider] = true
			}
		}
	}
	if protected {
		for id, group := range groups {
			for _, obligation := range group.Obligations {
				if strings.HasPrefix(obligation, "testing-policy-") {
					deep[id] = true
				}
			}
		}
	}
	if len(uncertainty) > 0 {
		standard = set(contract.Unknown)
		deep = map[string]bool{}
	}
	canary := set(contract.Always.Canary)
	required := union(canary, standard)
	if requiredMode == ModeDeep {
		required = union(required, deep)
	}
	selectedMode := request.RequestedMode
	if selectedMode == ModeAuto {
		selectedMode = requiredMode
	}
	if request.Purpose == PurposeDelivery && selectedMode == ModeStandard && requiredMode == ModeDeep {
		return Plan{}, fmt.Errorf("standard delivery is insufficient; required deep groups: %s", strings.Join(keys(deep), ","))
	}
	selected := map[string]bool{}
	stages := []Stage{}
	if request.Purpose == PurposeCadence {
		selected = set(contract.Cadence)
		required = set(contract.Cadence)
		stages = append(stages, Stage{ID: "cadence", Groups: keys(selected)})
	} else {
		addAll(selected, keys(canary))
		stages = appendStage(stages, "canary", canary)
		if selectedMode == ModeStandard || selectedMode == ModeDeep {
			addAll(selected, keys(standard))
			stages = appendStage(stages, "standard", subtract(standard, canary))
		}
		if selectedMode == ModeDeep {
			deepStage := subtract(deep, selected)
			addAll(selected, keys(deep))
			stages = appendStage(stages, "deep", deepStage)
		}
	}
	if request.Purpose != PurposeDelivery {
		diagnostic := map[string]bool{}
		for _, id := range request.Groups {
			if _, ok := groups[id]; !ok {
				return Plan{}, fmt.Errorf("requested diagnostic group %s does not exist", id)
			}
			if !selected[id] {
				diagnostic[id] = true
				selected[id] = true
			}
		}
		stages = appendStage(stages, "diagnostic", diagnostic)
	}
	plan := Plan{Purpose: request.Purpose, RequestedMode: request.RequestedMode, RequiredMode: requiredMode,
		ExecutedMode: selectedMode, AffectedSurfaces: keys(affected), RequiredGroups: keys(required), SelectedGroups: keys(selected),
		Stages: stages, Risk: risk, Uncertainty: uncertainty}
	for id := range groups {
		if !selected[id] {
			reason := "outside-impact"
			if standard[id] || deep[id] {
				reason = "below-requested-mode"
			}
			plan.Omissions = append(plan.Omissions, Omission{Group: id, Reason: reason})
		}
	}
	sort.Slice(plan.Omissions, func(i, j int) bool { return plan.Omissions[i].Group < plan.Omissions[j].Group })
	return plan, nil
}

func matchesPath(pattern, path string) bool {
	pattern, path = strings.TrimPrefix(pattern, "./"), strings.TrimPrefix(path, "./")
	if strings.HasSuffix(pattern, "/**") {
		prefix := strings.TrimSuffix(pattern, "**")
		return strings.HasPrefix(path, prefix)
	}
	return pattern == path
}

func groupMap(groups []Group) map[string]Group {
	result := make(map[string]Group, len(groups))
	for _, group := range groups {
		result[group.ID] = group
	}
	return result
}
func set(values []string) map[string]bool {
	result := map[string]bool{}
	addAll(result, values)
	return result
}
func addAll(target map[string]bool, values []string) {
	for _, value := range values {
		target[value] = true
	}
}
func keys(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
func union(values ...map[string]bool) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		addAll(result, keys(value))
	}
	return result
}
func subtract(left, right map[string]bool) map[string]bool {
	result := map[string]bool{}
	for value := range left {
		if !right[value] {
			result[value] = true
		}
	}
	return result
}
func appendStage(stages []Stage, id string, groups map[string]bool) []Stage {
	if selected := keys(groups); len(selected) > 0 {
		return append(stages, Stage{ID: id, Groups: selected})
	}
	return stages
}
