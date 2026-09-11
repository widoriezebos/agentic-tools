package testpolicy

import "fmt"

type GoalRisk struct {
	Severity     int `json:"severity"`
	Novelty      int `json:"novelty"`
	Exposure     int `json:"exposure"`
	Accumulation int `json:"accumulation"`
}

type RiskAssessment struct {
	Severity      int      `json:"severity"`
	Novelty       int      `json:"novelty"`
	Exposure      int      `json:"exposure"`
	Accumulation  int      `json:"accumulation"`
	Reversibility string   `json:"reversibility"`
	Detection     string   `json:"detection"`
	Recovery      string   `json:"recovery"`
	Reasons       []string `json:"reasons"`
}

func validateProjectRisk(risk ProjectRisk) error {
	if risk.Severity < 1 || risk.Severity > 3 || risk.Exposure < 1 || risk.Exposure > 3 {
		return fmt.Errorf("testing project risk severity and exposure must be 1 through 3")
	}
	if !validReversibility(risk.Reversibility) || !validDetection(risk.Detection) || !validRecovery(risk.Recovery) {
		return fmt.Errorf("testing project risk has an invalid reversibility, detection, or recovery value")
	}
	return nil
}

func validateRiskRaise(project ProjectRisk, raise RiskRaise) error {
	if raise.Severity != 0 && (raise.Severity < project.Severity || raise.Severity > 3) {
		return fmt.Errorf("surface severity may only raise the project default")
	}
	if raise.Exposure != 0 && (raise.Exposure < project.Exposure || raise.Exposure > 3) {
		return fmt.Errorf("surface exposure may only raise the project default")
	}
	if raise.Reversibility != "" && (!validReversibility(raise.Reversibility) || reversibilityRank(raise.Reversibility) < reversibilityRank(project.Reversibility)) {
		return fmt.Errorf("surface reversibility may only raise the project default")
	}
	if raise.Detection != "" && (!validDetection(raise.Detection) || detectionRank(raise.Detection) < detectionRank(project.Detection)) {
		return fmt.Errorf("surface detection may only raise the project default")
	}
	if raise.Recovery != "" && (!validRecovery(raise.Recovery) || recoveryRank(raise.Recovery) < recoveryRank(project.Recovery)) {
		return fmt.Errorf("surface recovery may only raise the project default")
	}
	return nil
}

func validReversibility(value string) bool {
	return value == "revert" || value == "restore" || value == "irreversible"
}
func validDetection(value string) bool { return value == "immediate" || value == "delayed" }
func validRecovery(value string) bool  { return value == "bounded" || value == "unbounded" }

func reversibilityRank(value string) int {
	return map[string]int{"revert": 1, "restore": 2, "irreversible": 3}[value]
}
func detectionRank(value string) int { return map[string]int{"immediate": 1, "delayed": 2}[value] }
func recoveryRank(value string) int  { return map[string]int{"bounded": 1, "unbounded": 2}[value] }

func assessRisk(contract Contract, surfaces []Surface, goal GoalRisk) RiskAssessment {
	result := RiskAssessment{Severity: contract.ProjectRisk.Severity, Exposure: contract.ProjectRisk.Exposure,
		Novelty: goal.Novelty, Accumulation: goal.Accumulation, Reversibility: contract.ProjectRisk.Reversibility,
		Detection: contract.ProjectRisk.Detection, Recovery: contract.ProjectRisk.Recovery,
		Reasons: []string{"project testing risk defaults"}}
	if goal.Severity > result.Severity {
		result.Severity = goal.Severity
		result.Reasons = append(result.Reasons, "accepted goal severity")
	}
	if goal.Exposure > result.Exposure {
		result.Exposure = goal.Exposure
		result.Reasons = append(result.Reasons, "accepted goal exposure")
	}
	for _, surface := range surfaces {
		if surface.Risk == nil {
			continue
		}
		raise := *surface.Risk
		if raise.Severity > result.Severity {
			result.Severity = raise.Severity
			result.Reasons = append(result.Reasons, "surface "+surface.ID+" severity")
		}
		if raise.Exposure > result.Exposure {
			result.Exposure = raise.Exposure
			result.Reasons = append(result.Reasons, "surface "+surface.ID+" exposure")
		}
		if reversibilityRank(raise.Reversibility) > reversibilityRank(result.Reversibility) {
			result.Reversibility = raise.Reversibility
			result.Reasons = append(result.Reasons, "surface "+surface.ID+" reversibility")
		}
		if detectionRank(raise.Detection) > detectionRank(result.Detection) {
			result.Detection = raise.Detection
			result.Reasons = append(result.Reasons, "surface "+surface.ID+" detection")
		}
		if recoveryRank(raise.Recovery) > recoveryRank(result.Recovery) {
			result.Recovery = raise.Recovery
			result.Reasons = append(result.Reasons, "surface "+surface.ID+" recovery")
		}
	}
	return result
}

// requiresDeep is the per-landing depth law. Its input is the change's own
// risk (the project's declared baseline raised by the affected surfaces),
// never the goal's answers: those scale cadence weight instead. Deep is
// owed for a protected policy change or when a surface raise lifts any
// dimension to 2 or worse, declares a reversibility other than revert,
// delayed detection, or unbounded recovery.
func requiresDeep(risk RiskAssessment, protected bool) bool {
	return protected || risk.Severity >= 2 || risk.Exposure >= 2 || risk.Novelty >= 2 ||
		risk.Accumulation >= 2 || risk.Reversibility != "revert" || risk.Detection == "delayed" || risk.Recovery == "unbounded"
}
