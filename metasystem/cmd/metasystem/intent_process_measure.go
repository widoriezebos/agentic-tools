package main

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func (inv *intentInvocation) unitMeasures(work launch.NamedWork) processmeasure.Measures {
	runner := inv.unitRunner()
	in := processmeasure.Input{Now: runner.Manager.Now()}
	if work.Record != nil {
		in.Run = work.Record.ID
		plan, err := launch.ReadUnitPlan(work.Record.Plan)
		if err == nil {
			in.FullArgv = plan.FullArgv
			if plan.Estimate != nil {
				in.EstimateMinutes = &plan.Estimate.ElapsedMinutes
			}
		}
		for _, revision := range work.Record.Revisions {
			in.Revisions = append(in.Revisions, revision.After)
		}
		for _, round := range work.Record.Rounds {
			for _, step := range round.Steps {
				one := processmeasure.Step{ID: step.LaunchID, Kind: step.Kind, Pending: step.StartedAt, Collected: step.FinishedAt, Terminal: step.State == launch.StepPassed || step.State == launch.StepFailed}
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
	var runStart time.Time
	for _, step := range in.Steps {
		if at, err := time.Parse(time.RFC3339Nano, step.Pending); err == nil && (runStart.IsZero() || at.Before(runStart)) {
			runStart = at
		}
	}
	questions, unreadable := channel.WalkQuestions(inv.stateRoot)
	in.Unknown = unreadable
	for _, question := range questions {
		if work.Record == nil || question.Goal != work.Record.Goal || runStart.IsZero() {
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
		in.Questions = append(in.Questions, interval)
	}
	return processmeasure.Read(in)
}

func measureNumber[T any](value *T) string {
	if value == nil {
		return "unavailable"
	}
	return fmt.Sprint(*value)
}
