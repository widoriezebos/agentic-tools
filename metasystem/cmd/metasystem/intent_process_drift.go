package main

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func (inv *intentInvocation) processDrift(unit launch.NamedWork) processmeasure.Drift {
	in := inv.unitMeasureInput(unit)
	runs, unknown, err := inv.unitRunner().GoalRuns(unit.Record.Goal)
	if err != nil {
		unknown = append(unknown, "process measurement unavailable: "+err.Error())
	}
	projection, _, problem := inv.projection()
	file, _ := goalRecord(projection, unit.Record.Goal)
	var episodeAt string
	if problem != nil || file == nil || file.Claimed == nil {
		unknown = append(unknown, "process episode unavailable for goal "+unit.Record.Goal)
	} else {
		episodeAt = file.Claimed.EpisodeAt
		if episodeAt == "" {
			episodeAt = file.Claimed.At
		}
	}
	episodeStart, err := time.Parse(time.RFC3339Nano, episodeAt)
	if err != nil {
		unknown = append(unknown, "process episode start unavailable")
	}
	all := processmeasure.Input{Now: in.Now}
	for _, run := range runs {
		if run.Unit != unit.Unit {
			continue
		}
		input := inv.unitMeasureInput(run)
		input.Current = run.Run == unit.Run
		var start time.Time
		for _, step := range input.Steps {
			at, err := time.Parse(time.RFC3339Nano, step.Pending)
			if err == nil && (start.IsZero() || at.Before(start)) {
				start = at
			}
		}
		if start.IsZero() {
			unknown = append(unknown, "process run start unavailable: "+run.Run)
		} else if !start.Before(episodeStart) {
			all.Runs = append(all.Runs, input)
		}
	}
	if len(unknown) > 0 || len(all.Runs) == 0 {
		drift := processmeasure.Decide(unit.Record.Goal+"/"+unit.Unit+"/"+unit.Run, processmeasure.Measures{}, in.CheckMinutes)
		drift.Unknown = unknown
		return drift
	}
	m := processmeasure.Read(all)
	return processmeasure.Decide(unit.Record.Goal+"/"+unit.Unit+"/"+unit.Run, m, in.CheckMinutes)
}

func (inv *intentInvocation) observeProcessDrift(unit launch.UnitRunRecord) error {
	projection, _, problem := inv.projection()
	if problem != nil {
		return fmt.Errorf("process episode unavailable: %s", problem.Summary)
	}
	file, _ := goalRecord(projection, unit.Goal)
	if file == nil || file.Claimed == nil {
		return fmt.Errorf("process episode unavailable for goal %s", unit.Goal)
	}
	episode := fmt.Sprintf("%s/%d", file.Claimed.EpisodeAt, file.Claimed.EpisodeRevision)
	if file.Claimed.EpisodeAt == "" {
		episode = fmt.Sprintf("%s/%d", file.Claimed.At, file.Claimed.Revision)
	}
	drift := inv.processDrift(launch.NamedWork{Unit: unit.Unit, Run: unit.ID, Record: &unit})
	if len(drift.Unknown) > 0 {
		return fmt.Errorf("process measurement unavailable: %v", drift.Unknown)
	}
	return processchange.Observe(inv.stateRoot, unit.Goal, episode, drift)
}

func (inv *intentInvocation) refreshProcessDrift(goal string) error {
	runs, unknown, err := inv.unitRunner().GoalRuns(goal)
	if err != nil || len(unknown) > 0 {
		return fmt.Errorf("process measurement unavailable: %v %v", err, unknown)
	}
	for _, run := range runs {
		if err := inv.observeProcessDrift(*run.Record); err != nil {
			return err
		}
	}
	return nil
}
