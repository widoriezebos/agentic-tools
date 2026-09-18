package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type unitAdvancer interface {
	Advance(launch.UnitRequest) (launch.UnitResult, error)
}

var unitRunner = func() unitAdvancer {
	manager := launchManager()
	return &launch.UnitRunner{Manager: manager, Git: launch.OSGitRunner{}}
}

func init() {
	base := launchManager
	launchManager = func() *launch.Manager {
		manager := base()
		manager.Adapters["plain-exec"] = launch.PlainExec{}
		return manager
	}
}

func runUnitRun(args []string) int {
	flags := flag.NewFlagSet("unit run", flag.ContinueOnError)
	plan := flags.String("plan", "", "unit plan")
	resume := flags.String("resume", "", "run id")
	followUp := flags.String("follow-up", "", "follow-up brief")
	if flags.Parse(args) != nil || flags.NArg() != 0 || (*plan == "") == (*resume == "") || (*plan != "" && *followUp != "") {
		fmt.Fprintln(os.Stderr, "usage: metasystem unit run (--plan <file>|--resume <run>) [--follow-up <file>]")
		return 2
	}
	runner := unitRunner()
	result, err := runner.Advance(launch.UnitRequest{Plan: *plan, Resume: *resume, FollowUp: *followUp})
	if err != nil {
		fmt.Fprintln(os.Stderr, "unit run:", err)
		return 1
	}
	if result.Capped {
		fmt.Printf("run=%s round=%d state=running step=%s launch=%s\n", result.Record.ID, result.Round, result.Step, result.Launch)
		return 3
	}
	var manager *launch.Manager
	if concrete, ok := runner.(*launch.UnitRunner); ok {
		manager = concrete.Manager
	}
	fmt.Println(unitJudgementLine(result.Record, manager))
	return 0
}

func unitJudgementLine(record launch.UnitRunRecord, manager *launch.Manager) string {
	round := record.Rounds[len(record.Rounds)-1]
	build, reads := "-:skipped", []string{}
	passed, total := 0, 0
	counts := true
	var red []string
	for _, step := range round.Steps {
		switch {
		case step.Name == "build":
			build = step.LaunchID + ":" + unitLaunchState(manager, step)
		case strings.HasPrefix(step.Name, "read"):
			reads = append(reads, step.LaunchID+":"+chooseUnitValue(step.Verdict, "none"))
			counts = counts && (step.VerdictCounts == nil || *step.VerdictCounts)
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
