package launch

import (
	"fmt"
	"path/filepath"
)

type CheckOutcome string

const (
	CheckPassed             CheckOutcome = "passed"
	CheckFailed             CheckOutcome = "failed"
	CheckBudgetExhausted    CheckOutcome = "budget-exhausted"
	CheckEnvironmentFailure CheckOutcome = "environment-failure"
)

// CheckResult records execution, independently of whether its evidence applies
// to a later tree. Only a complete pass can authorize publication or carry.
type CheckResult struct {
	Version     int          `json:"version,omitempty"`
	Outcome     CheckOutcome `json:"outcome,omitempty"`
	ExecutionID string       `json:"executionId"`
	Directory   string       `json:"runDirectory,omitempty"`
	Check       UnitCheck    `json:"check"`
	Exits       []CheckExit  `json:"exits"`
}

// RequirePassed also reads retained results from before outcomes were recorded.
// Those results must contain both successful commands; an explicit incomplete
// outcome never falls back to exit codes.
func (result CheckResult) RequirePassed() error {
	if err := result.Check.RequireSupportedKind(); err != nil {
		return err
	}
	if result.Version != 0 && result.Version != 1 {
		return fmt.Errorf("the unit check result has an unsupported version")
	}
	if result.Outcome != CheckPassed && (result.Version != 0 || result.Outcome != "") {
		return fmt.Errorf("the unit check outcome %q is not a complete pass", result.Outcome)
	}
	if result.ExecutionID == "" || len(result.Exits) != 2 {
		return fmt.Errorf("the unit check did not finish both declared commands")
	}
	for i, name := range []string{"cheap", "audits"} {
		if exit := result.Exits[i]; exit.Name != name || exit.Exit != 0 || exit.Error != "" {
			return fmt.Errorf("the unit check command %s did not pass", name)
		}
	}
	return nil
}

func ReadCheckResult(execution string) (CheckResult, error) {
	var result CheckResult
	err := readJSONFile(filepath.Join(execution, "result.json"), &result)
	if err == nil && result.ExecutionID != filepath.Base(execution) {
		err = fmt.Errorf("the unit check result names a different execution")
	}
	return result, err
}

// outcome also owns round attribution: only a deadline with no other failure
// is an environment failure. A signal kill keeps the failed-round consequence.
func (execution CheckExecution) outcome() CheckOutcome {
	outcome := CheckPassed
	complete := len(execution.Steps) == 2
	for _, step := range execution.Steps {
		switch step.Outcome {
		case "passed":
		case "deadline":
			outcome = CheckEnvironmentFailure
		default:
			return CheckFailed
		}
		complete = complete && step.Terminal
	}
	if outcome == CheckPassed && !complete {
		return CheckFailed
	}
	return outcome
}
