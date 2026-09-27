package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

const unitExitReadCompacted = 1

func unitJudgementLine(record launch.UnitRunRecord, manager *launch.Manager) string {
	round := record.Rounds[len(record.Rounds)-1]
	build, reads := "-:skipped", []string{}
	passed, total := 0, 0
	counts := true
	var red []string
	for index, step := range round.Steps {
		switch {
		case strings.HasPrefix(step.Name, "build"):
			value := step.LaunchID + ":" + unitLaunchState(manager, step)
			if build == "-:skipped" {
				build = value
			} else {
				build += "," + value
			}
		case strings.HasPrefix(step.Name, "read"):
			if readStepHasCountingRerun(round.Steps, index) {
				continue
			}
			reads = append(reads, step.LaunchID+":"+chooseUnitValue(step.Verdict, "none"))
			counts = counts && step.VerdictCounts != nil && *step.VerdictCounts
		case strings.HasPrefix(step.Name, "proof:"):
			total++
			if step.State == launch.StepPassed {
				passed++
			} else if step.State == launch.StepFailed {
				red = append(red, strings.TrimPrefix(step.Name, "proof:"))
			}
		}
	}
	if len(reads) == 0 {
		reads = []string{"-:none"}
	}
	redValue := "none"
	if len(red) > 0 {
		redValue = strings.Join(red, ",")
	}
	return fmt.Sprintf("run=%s round=%d state=%s outcome=%s build=%s read=%s verdict-counts=%t proof=%d/%d red=%s directory=%s",
		record.ID, round.Number, record.State, round.Outcome, build, strings.Join(reads, ","), counts, passed, total, redValue, filepath.Clean(round.Directory))
}

func readStepHasCountingRerun(steps []launch.UnitStep, index int) bool {
	step := steps[index]
	if step.Rerun {
		return false
	}
	for _, candidate := range steps[index+1:] {
		if candidate.Rerun && candidate.VerdictCounts != nil && *candidate.VerdictCounts && (step.Package == "" || candidate.Package == step.Package) {
			return true
		}
	}
	return false
}

func unitLaunchState(manager *launch.Manager, step launch.UnitStep) string {
	if manager != nil && step.LaunchID != "" {
		if record, err := manager.Store.Read(step.LaunchID); err == nil {
			return string(record.State)
		}
	}
	return string(step.State)
}
func chooseUnitValue(value, fallback string) string {
	if value != "" {
		return strings.TrimSpace(strings.TrimPrefix(value, "VERDICT:"))
	}
	return fallback
}
