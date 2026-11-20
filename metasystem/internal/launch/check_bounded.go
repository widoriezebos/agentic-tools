package launch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func (check UnitCheck) Kind() string {
	if check.Declaration == nil {
		return "legacy"
	}
	return check.Declaration.EffectiveValues()[4]
}

// StatusLines reports incomplete feedback without presenting it as coverage.
func (result CheckResult) StatusLines() []string {
	if result.Kind != "bounded" {
		return nil
	}
	lines := []string{fmt.Sprintf("check: bounded; outcome: %s", result.Outcome)}
	if result.Outcome == CheckBudgetExhausted {
		lines = append(lines, result.BudgetSummary())
	}
	if len(result.Skipped) > 0 {
		lines = append(lines, fmt.Sprintf("%d nominations were skipped", len(result.Skipped)))
	}
	for _, exit := range result.Exits {
		if exit.Exit != 0 && !exit.Cut {
			lines = append(lines, "check stopped at: "+exit.Name)
		}
	}
	return lines
}

func (result CheckResult) BudgetSummary() string {
	return fmt.Sprintf("the prerequisites passed; the feedback budget ended with %d of %d selected tests not run; the round continues and the candidate's complete check covers the rest", len(result.NotRun), len(result.Selected))
}

func (check UnitCheck) runBounded(directory, records string, clock CheckContext) (string, []CheckExit, error) {
	if err := check.admit(); err != nil {
		return "", nil, err
	}
	if clock.Now == nil {
		clock.Now = time.Now
	}
	if clock.After == nil {
		clock.After = time.After
	}
	if clock.Context == nil {
		clock.Context = context.Background()
	}
	if clock.ChangedPaths == nil {
		return "", nil, fmt.Errorf("the changed files are unavailable")
	}
	paths, err := clock.ChangedPaths(directory, check.Base)
	if err != nil {
		return "", nil, err
	}
	commands, selected, skipped, selectionErr := feedbackSelection(directory, paths, clock.Nominations, check.EnvironmentValues())
	if selectionErr != nil {
		skipped = append(skipped, "test selection is unavailable: "+selectionErr.Error())
	}
	if err := os.MkdirAll(records, 0700); err != nil {
		return "", nil, err
	}
	execution, err := os.MkdirTemp(records, "check-")
	if err != nil {
		return "", nil, err
	}
	log, err := os.OpenFile(filepath.Join(execution, "output.log"), os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return execution, nil, err
	}
	defer log.Close()
	result := CheckResult{Version: 2, ExecutionID: filepath.Base(execution), Directory: directory, Check: check, Kind: "bounded", Selected: selected, Skipped: skipped, AdmissionWaitSeconds: clock.AdmissionWait.Seconds()}
	observation := clock.CheckExecution
	observation.ExecutionID = result.ExecutionID
	observation.Kind = "bounded"
	if check.Declaration != nil {
		observation.SourceSHA256 = check.Declaration.SourceSHA256
	}
	environment := commandenv.EngineEnvironmentInDirectory(check.EnvironmentValues(), clock.EngineDirectory)
	minutes, _ := strconv.Atoi(check.Declaration.EffectiveValues()[5])
	budget := time.Duration(minutes) * time.Minute
	var elapsed time.Duration
	ran := map[string]bool{}
	failed, environmentFailure, exhausted, cancelled := false, false, false, false
	before := ""
	if clock.Tree != nil {
		before, _ = clock.Tree()
	}
	run := func(name string, args []string, tests []string, feedback bool) error {
		command := checkCommand(`exec "$@"`, append([]string{"check"}, args...)...)
		command.Dir, command.Env = directory, slices.Clone(environment)
		if clock.Attach != nil {
			clock.Attach(command)
		}
		output := newOutputProgress(log)
		command.Stdout, command.Stderr = output, output
		started := clock.Now()
		remaining := time.Duration(0)
		if feedback {
			remaining = budget - elapsed
		}
		step := processmeasure.Step{ID: clock.Run + "/" + clock.Round + "/" + result.ExecutionID + ":" + name, Kind: "attest", Parent: clock.Parent, Act: check.ProcessAct, Start: started.UTC().Format(time.RFC3339Nano), Argv: args, Coverage: "unknown"}
		observation.Steps = append(observation.Steps, step)
		if err := writeUnitJSON(filepath.Join(execution, "observation.json"), observation, execution); err != nil {
			return err
		}
		why, runErr := checkProcess(clock.Context, command, output.wrote, time.Duration(check.Minutes)*time.Minute, remaining, clock)
		duration := max(time.Duration(0), clock.Now().Sub(started))
		if feedback {
			elapsed += duration
			result.FeedbackSeconds = elapsed.Seconds()
		} else {
			result.PrerequisiteSeconds += duration.Seconds()
		}
		exit := CheckExit{Name: name, Output: output.buffer.String(), Minutes: duration.Minutes()}
		step.End, step.Terminal, step.Outcome = clock.Now().UTC().Format(time.RFC3339Nano), true, "passed"
		// Go's structured terminal events preserve an observed red even when the
		// package is cut before its process exits. Partial output is never a red.
		late := feedback && duration > remaining && why == "completed"
		for _, line := range strings.Split(exit.Output, "\n") {
			var event struct{ Action, Test string }
			if json.Unmarshal([]byte(line), &event) == nil && event.Test != "" {
				for _, test := range tests {
					_, name, _ := strings.Cut(test, "/Test")
					if "Test"+name == event.Test {
						if !late && (event.Action == "pass" || event.Action == "skip" || event.Action == "fail") {
							ran[test] = true
						}
					}
				}
				if event.Action == "fail" {
					failed = true
				}
			}
		}
		if feedback && duration > remaining && why == "completed" {
			why = "budget"
		}
		switch why {
		case "budget":
			exhausted = true
			exit.Cut = true
			step.Outcome = "budget-exhausted"
		case "cancelled":
			cancelled = true
			exit.Cut = true
			step.Outcome = "cancelled"
		case "deadline", "environment":
			environmentFailure = true
			step.Outcome = "deadline"
		default:
			if runErr != nil {
				// The runtime's explicit allocation failure is evidence. An exit code
				// or signal alone does not identify an out-of-memory failure.
				if strings.Contains(exit.Output, "fatal error: out of memory") {
					environmentFailure = true
					step.Outcome = "deadline"
				} else {
					failed = true
					step.Outcome = "failed"
				}
			} else if feedback {
				for _, test := range tests {
					if !ran[test] {
						environmentFailure = true
						step.Outcome = "deadline"
						exit.Exit = -1
						exit.Error = "a selected test returned no terminal result"
					}
				}
			}
		}
		if runErr != nil && !exit.Cut {
			exit.Exit = -1
			exit.Error = runErr.Error()
			var code *exec.ExitError
			if errors.As(runErr, &code) {
				exit.Exit = code.ExitCode()
			}
		}
		if exit.Cut {
			exit.Exit = 0
			exit.Error = ""
		}
		result.Exits = append(result.Exits, exit)
		observation.Steps[len(observation.Steps)-1] = step
		return writeUnitJSON(filepath.Join(execution, "observation.json"), observation, execution)
	}
	// Prerequisites finish independently of the feedback budget, including
	// audits after a static red. Feedback has no value after a prerequisite red.
	for _, pre := range []struct {
		name string
		args []string
	}{{"static", []string{"metasystem", "test", "impact", "--publication-only"}}, {"audits", []string{"/bin/sh", "-c", check.Audits}}} {
		if err := run(pre.name, pre.args, nil, false); err != nil {
			return execution, result.Exits, err
		}
		if cancelled {
			break
		}
	}
	if selectionErr != nil {
		environmentFailure = true
	}
	if !failed && !environmentFailure && !cancelled {
		for _, command := range commands {
			if elapsed >= budget {
				exhausted = true
				break
			}
			args := []string{"go", "test", "-p", "2", "-json", "-count=1", command.Package}
			if command.Pattern != "" {
				args = append(args, "-run", command.Pattern)
			}
			if err := run(command.Package+" "+command.Pattern, args, command.Tests, true); err != nil {
				return execution, result.Exits, err
			}
			if failed || exhausted || cancelled || environmentFailure {
				break
			}
		}
	}
	for _, name := range selected {
		if ran[name] {
			result.Ran = append(result.Ran, name)
		} else {
			result.NotRun = append(result.NotRun, name)
		}
	}
	result.Outcome = CheckPassed
	switch {
	case failed:
		result.Outcome = CheckFailed
	case environmentFailure:
		result.Outcome = CheckEnvironmentFailure
	case exhausted:
		result.Outcome = CheckBudgetExhausted
	case cancelled:
		result.Outcome = CheckCancelled
	}
	observation.Outcome = result.Outcome
	if err := writeUnitJSON(filepath.Join(execution, "observation.json"), observation, execution); err != nil {
		return execution, result.Exits, err
	}
	if err := writeUnitJSON(filepath.Join(execution, "result.json"), result, execution); err != nil {
		return execution, result.Exits, err
	}
	if result.Outcome == CheckPassed {
		evidence := check.evidence(directory, directory, execution, before, clock, result.Outcome)
		// Feedback coverage is named and deliberately weaker than the independent
		// publication gate. Old engines reject this evidence version too.
		evidence.Version = 2
		evidence.Tests = slices.Clone(result.Ran)
		evidence.Strength.Cheap = true
		evidence.Strength.Complete = before != "" && before == evidence.Content.After
		if err := writeUnitJSON(filepath.Join(execution, "evidence.json"), evidence, execution); err != nil {
			return execution, result.Exits, err
		}
	}
	if result.Outcome == CheckFailed || result.Outcome == CheckEnvironmentFailure || result.Outcome == CheckCancelled {
		return execution, result.Exits, fmt.Errorf("the check ended with %s", result.Outcome)
	}
	return execution, result.Exits, nil
}
