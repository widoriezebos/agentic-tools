package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func (inv *intentInvocation) unitMeasureInput(work launch.NamedWork) processmeasure.Input {
	runner := inv.unitRunner()
	in := processmeasure.Input{Now: runner.Manager.Now()}
	if work.Record != nil {
		in.Run, in.Unit = work.Record.ID, work.Unit
		if subject := currentSubject(work); subject != nil {
			in.PublishedAt = subject.PublishedAt
		}
		// Retained measurements survive removal of the execution worktree.
		var plan launch.UnitPlan
		body, err := os.ReadFile(work.Record.Plan)
		if err == nil && json.Unmarshal(body, &plan) == nil {
			in.FullArgv = plan.FullArgv
			if plan.Estimate != nil {
				in.EstimateMinutes, in.CheckMinutes = &plan.Estimate.ElapsedMinutes, plan.Estimate.CheckMinutes
			}
		}
		for _, revision := range work.Record.Revisions {
			in.Revisions = append(in.Revisions, revision.After)
		}
		for _, round := range work.Record.Rounds {
			for _, step := range round.Steps {
				one := processmeasure.Step{ID: step.LaunchID, Kind: step.Kind, Pending: step.StartedAt, Collected: step.FinishedAt, FullArgv: in.FullArgv, Terminal: step.State == launch.StepPassed || step.State == launch.StepFailed}
				if step.Command != nil {
					one.Argv = step.Command.Argv
				}
				if record, err := runner.Manager.Store.Read(step.LaunchID); err == nil {
					one.Start, one.End, one.Terminal = record.StartedAt, record.FinishedAt, record.State.Terminal()
					if usage := record.Measurement; usage.UsageRead {
						one.Usage = &processmeasure.Tokens{Input: usage.InputTokens, CacheRead: usage.CacheReadTokens, CacheCreation: usage.CacheCreationTokens, Output: usage.OutputTokens}
					}
				}
				in.Steps = append(in.Steps, one)
			}
		}
	}
	// A question binds to this run only within the run's window: the goal's
	// questions from earlier units or runs are not this unit's person wait.
	// A published run ends at publication; other finished runs end at collection.
	// A run with no started step has no window and no person number.
	var runStart, runEnd time.Time
	finished := len(in.Steps) > 0
	for _, step := range in.Steps {
		if at, err := time.Parse(time.RFC3339Nano, step.Pending); err == nil && (runStart.IsZero() || at.Before(runStart)) {
			runStart = at
		}
		at, err := time.Parse(time.RFC3339Nano, step.Collected)
		if !step.Terminal || err != nil {
			finished = false
		} else if at.After(runEnd) {
			runEnd = at
		}
	}
	if !finished {
		runEnd = time.Time{}
	}
	if at, err := time.Parse(time.RFC3339Nano, in.PublishedAt); err == nil {
		runEnd = at
	}
	questions, unreadable := channel.WalkQuestions(inv.stateRoot)
	in.Unknown = unreadable
	if work.Record != nil && runStart.IsZero() {
		in.Questions = append(in.Questions, processmeasure.Interval{End: time.Time{}.Add(time.Nanosecond)})
	}
	for _, question := range questions {
		if work.Record == nil || question.Goal != work.Record.Goal || runStart.IsZero() {
			continue
		}
		if !runEnd.IsZero() && !question.OpenedAt.Before(runEnd) {
			continue
		}
		interval := processmeasure.Interval{Start: question.OpenedAt}
		if question.Answer != nil {
			interval.End = question.Answer.At
			if !interval.End.After(runStart) {
				continue
			}
		} else if question.State != "open" {
			if question.OpenedAt.Before(runStart) {
				continue
			}
			interval.End = time.Time{}.Add(time.Nanosecond)
		}
		if interval.Start.Before(runStart) {
			interval.Start = runStart
		}
		if !runEnd.IsZero() && (interval.End.IsZero() || interval.End.After(runEnd)) && interval.End != (time.Time{}.Add(time.Nanosecond)) {
			interval.End = runEnd
		}
		in.Questions = append(in.Questions, interval)
	}
	return in
}

func measureNumber[T any](value *T) string {
	if value == nil {
		return "unavailable"
	}
	return fmt.Sprint(*value)
}

func (inv *intentInvocation) unitMeasures(work launch.NamedWork) processmeasure.Measures {
	return processmeasure.Read(inv.unitMeasureInput(work))
}

func measureLine(m processmeasure.Measures) string {
	finish := "unavailable"
	if m.ElapsedFinish != nil {
		finish = m.ElapsedFinish.In(time.Local).Format(time.RFC3339)
	}
	return fmt.Sprintf("hours: build %s, check %s, read %s, correction %s; estimate %s minutes; corrections %d; fix units unavailable; job pending %s, collection %s; person %s (lower bound %t); input/cache-read/cache-creation/output tokens %s (%d/%d usage records); full suite minutes %s; elapsed hours %s (lower bound %t); elapsed finish %s", measureNumber(m.Hours["build"]), measureNumber(m.Hours["attest"]), measureNumber(m.Hours["read"]), measureNumber(m.Hours["correction"]), measureNumber(m.EstimateMinutes), m.Corrections, measureNumber(m.Hours["pending"]), measureNumber(m.Hours["collection"]), measureNumber(m.Hours["person"]), m.PersonLowerBound, measureNumber(m.Tokens), m.UsageKnown, m.UsageExpected, measureNumber(m.SuiteMinutes), measureNumber(m.ElapsedHours), m.ElapsedLowerBound, finish)
}

func (inv *intentInvocation) goalCosts(id string, current []launch.NamedWork, result *intentResult) error {
	runs, unknown, err := inv.unitRunner().GoalRuns(id)
	if err != nil {
		unknown = append(unknown, "process measurement unavailable: "+err.Error())
		runs = current
	}
	state, err := processchange.ReadState(inv.stateRoot, id)
	acts, missing := state.Acts, state.Unknown
	if err != nil {
		return fmt.Errorf("process history unavailable: %w", err)
	}
	result.text = append(result.text, append(unknown, missing...)...)
	data := result.Data.(map[string]any)
	data["processUnknown"] = unknown
	data["processStops"] = state.Stops
	data["processResolved"] = state.Resolved
	for _, stop := range state.Resolved {
		result.text = append(result.text, "  Process change "+stop.Resolution+"; recorded cost is unchanged")
	}
	for _, stop := range state.Stops {
		result.text = append(result.text, "  Automatic process changes are held: "+stop.Stop.Class+"; observed "+fmt.Sprint(stop.Stop.Measure.Now)+" against allowance "+fmt.Sprint(stop.Stop.Measure.Previous)+"; "+stop.Stop.Handoff, stop.Impact)
	}
	for _, act := range acts {
		actor := "own-caused agent"
		if act.Actor == "direct-person" && act.Proof.Helm == nil {
			actor = "person-directed"
		}
		result.text = append(result.text, fmt.Sprintf("  process act %s: %s, %s, %s; %s", act.ID, actor, act.Key, act.Status, act.Reason))
	}
	all := processmeasure.Input{Now: inv.unitRunner().Manager.Now(), GoalOpen: true, Unknown: append(unknown, missing...)}
	if projection, _, problem := inv.projection(); problem == nil {
		file, _ := goalRecord(projection, id)
		all.GoalOpen = file == nil || file.State != goal.StateDone
	}
	byUnit := map[string][]processmeasure.Input{}
	for _, run := range runs {
		in := inv.unitMeasureInput(run)
		in.Current = slices.ContainsFunc(current, func(work launch.NamedWork) bool { return work.Run == run.Run })
		byUnit[run.Unit] = append(byUnit[run.Unit], in)
		all.Runs = append(all.Runs, in)
	}
	views := data["work"].([]map[string]any)
	for _, name := range slices.Sorted(maps.Keys(byUnit)) {
		m := processmeasure.Read(processmeasure.Input{Now: all.Now, Runs: byUnit[name]})
		index := slices.IndexFunc(views, func(view map[string]any) bool { return view["work"] == name })
		if index < 0 {
			views = append(views, map[string]any{"work": name, "stage": "retained runs; no current worktree run"})
			index = len(views) - 1
		}
		views[index]["measures"] = m
		for _, run := range runs {
			if run.Unit == name && slices.ContainsFunc(current, func(work launch.NamedWork) bool { return work.Run == run.Run }) {
				drift := inv.processDrift(run)
				views[index]["processBands"] = drift.Bands
				views[index]["processUnknown"] = drift.Unknown
				result.text = append(result.text, drift.Unknown...)
				result.text = append(result.text, "  process bands: "+fmt.Sprint(drift.Bands))
			}
		}
		result.text = append(result.text, "  "+name+": "+measureLine(m))
	}
	m := processmeasure.Read(all)
	data["work"], data["measures"], data["processActs"] = views, m, acts
	result.text = append(result.text, "  goal total: "+measureLine(m))
	return nil
}
