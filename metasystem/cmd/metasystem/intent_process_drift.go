package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
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
	drift := processmeasure.Decide(unit.Record.Goal+"/"+unit.Unit+"/"+unit.Run, m, in.CheckMinutes)
	state, _ := processchange.ReadState(inv.stateRoot, unit.Record.Goal)
	var consumed processchange.ProcessAct
	var executions []string
	for _, run := range all.Runs {
		for _, step := range run.Steps {
			if step.ID == "" {
				continue
			}
			executions = append(executions, step.ID)
			record, err := inv.unitRunner().Manager.Store.Read(step.ID)
			var id string
			if err != nil || json.Unmarshal(record.AdapterData["sandboxAct"], &id) != nil {
				continue
			}
			for _, act := range state.Acts {
				if act.ID == id && act.Status == "applied" && act.AppliedAt.After(consumed.AppliedAt) && !slices.ContainsFunc(state.Acts, func(later processchange.ProcessAct) bool {
					return later.Status == "applied" && later.Key == act.Key && later.Checkout == act.Checkout && (later.AppliedAt.After(act.AppliedAt) || later.Undo == act.ID)
				}) {
					consumed = act
				}
			}
		}
	}
	slices.Sort(executions)
	evidence, _ := json.Marshal(slices.Compact(executions))
	for index := range drift.Stops {
		drift.Stops[index].Evidence = string(evidence)
		if consumed.ID != "" && drift.Stops[index].Measure.Name == "unit elapsed minutes" {
			drift.Stops[index].Cause = &loopstop.Cause{Kind: "process-change", Name: consumed.ID, Evidence: "consumed sandbox act; attribution provisional from initial admission"}
		}
	}
	return drift
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

// consumedSandboxAct binds the resolved local sandbox to this execution's act.
func (inv *intentInvocation) consumedSandboxAct(record launch.Record, settings launch.Settings) string {
	state, problem := processchange.ReadState(inv.stateRoot, record.Goal)
	var latest processchange.ProcessAct
	if inv.stateRoot == "" || problem != nil || len(state.Unknown) > 0 {
		return ""
	}
	for _, act := range state.Acts {
		if act.Key == launch.CodexSandboxKey && act.Status == "applied" && act.AppliedAt.After(latest.AppliedAt) {
			latest = act
		}
	}
	for _, value := range settings.Values {
		if value.Key == launch.CodexSandboxKey && value.Source == "conf-local" && value.Value == latest.After && latest.SettingsSHA256 != "" && latest.SettingsSHA256 == settings.LocalSHA256 && latest.Undo == "" && !latest.Unset {
			return latest.ID
		}
	}
	return ""
}
