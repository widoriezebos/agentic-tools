package launch

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type CheckOutcome string

const (
	CheckCancelled          CheckOutcome = "cancelled"
	CheckPassed             CheckOutcome = "passed"
	CheckFailed             CheckOutcome = "failed"
	CheckBudgetExhausted    CheckOutcome = "budget-exhausted"
	CheckEnvironmentFailure CheckOutcome = "environment-failure"
)

// CheckResult records execution, independently of whether its evidence applies
// to a later tree. Only a complete pass can authorize publication or carry.
type CheckResult struct {
	Kind                 string       `json:"kind,omitempty"`
	Selected             []string     `json:"selected,omitempty"`
	Ran                  []string     `json:"ran,omitempty"`
	NotRun               []string     `json:"notRun,omitempty"`
	Skipped              []string     `json:"skipped,omitempty"`
	AdmissionWaitSeconds float64      `json:"admissionWaitSeconds,omitempty"`
	PrerequisiteSeconds  float64      `json:"prerequisiteSeconds,omitempty"`
	FeedbackSeconds      float64      `json:"feedbackSeconds,omitempty"`
	Version              int          `json:"version,omitempty"`
	Outcome              CheckOutcome `json:"outcome,omitempty"`
	ExecutionID          string       `json:"executionId"`
	Directory            string       `json:"runDirectory,omitempty"`
	Check                UnitCheck    `json:"check"`
	Exits                []CheckExit  `json:"exits"`
}

// RequirePassed also reads retained results from before outcomes were recorded.
// Those results must contain both successful commands; an explicit incomplete
// outcome never falls back to exit codes.
func (result CheckResult) RequirePassed() error {
	if err := result.Check.RequireSupportedKind(); err != nil {
		return err
	}
	if result.Kind == "bounded" {
		return fmt.Errorf("bounded feedback cannot satisfy the complete check")
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
		if exit := result.Exits[i]; exit.Name != name || exit.Exit != 0 || exit.Error != "" || exit.Cut {
			return fmt.Errorf("the unit check command %s did not pass", name)
		}
	}
	return nil
}

// RequireFeedbackPassed accepts only bounded feedback, never a publication gate.
func (result CheckResult) RequireFeedbackPassed() error {
	if result.Version == 2 && result.Kind == "bounded" && result.Check.Kind() == "bounded" {
		if result.Outcome != CheckPassed || result.ExecutionID == "" || len(result.NotRun) != 0 || len(result.Exits) < 2 || result.Exits[0].Name != "static" || result.Exits[1].Name != "audits" {
			return fmt.Errorf("the bounded feedback did not pass")
		}
		for _, exit := range result.Exits {
			if exit.Exit != 0 || exit.Error != "" || exit.Cut {
				return fmt.Errorf("the bounded check has an incomplete command")
			}
		}
		return nil
	}
	return fmt.Errorf("the result is not bounded feedback")
}

// LatestCheckResult reads the newest execution in a round, including an
// unfinished retry. A prior completion cannot describe that retry.
func LatestCheckResult(round string) (CheckResult, error) {
	paths, err := filepath.Glob(filepath.Join(round, "check-*"))
	if err != nil {
		return CheckResult{}, err
	}
	var latest string
	var newest, modified time.Time
	for _, path := range paths {
		var observation CheckExecution
		if err := readJSONFile(filepath.Join(path, "observation.json"), &observation); err != nil || len(observation.Steps) == 0 {
			return CheckResult{}, fmt.Errorf("the check observation is unavailable")
		}
		started, err := time.Parse(time.RFC3339Nano, observation.Steps[0].Start)
		info, statErr := os.Stat(path)
		if err != nil || statErr != nil {
			return CheckResult{}, fmt.Errorf("the check start is unavailable")
		}
		if latest == "" || started.After(newest) || started.Equal(newest) && info.ModTime().After(modified) {
			latest, newest, modified = path, started, info.ModTime()
		}
	}
	if latest == "" {
		return CheckResult{}, os.ErrNotExist
	}
	return ReadCheckResult(latest)
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
	if execution.Kind == "bounded" && execution.Outcome != "" {
		return execution.Outcome
	}
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
