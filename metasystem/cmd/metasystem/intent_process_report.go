package main

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

// Process history belongs to project state; channel wait evidence belongs to
// the installation that served the run.
func readProcessReport(root, installation, goal, unit string, runner *launch.UnitRunner, now time.Time, boundary processmeasure.Watermark) processmeasure.Projection {
	if runner == nil {
		runner = &launch.UnitRunner{Manager: &launch.Manager{Store: launch.Store{}, Now: func() time.Time { return now }}}
	}
	runs, unknown, err := runner.GoalRuns(goal)
	if err != nil {
		unknown = append(unknown, "process measurements unavailable: "+err.Error())
	}
	runUnknown := len(unknown) > 0
	state, err := processchange.ReadState(root, goal)
	if root == "" {
		state = processchange.State{Unknown: []string{"process report state root unavailable"}}
	}
	if err != nil {
		unknown = append(unknown, "process history unavailable: "+err.Error())
	}
	all := processmeasure.Input{Now: now, Unknown: append(unknown, state.Unknown...)}
	var carryUnits []string
	if unit != "" {
		carryUnits = append(carryUnits, unit)
	}
	for _, run := range runs {
		if unit != "" && run.Unit != unit {
			continue
		}
		if unit != "" && run.Record != nil {
			for _, subject := range run.Record.Subjects {
				if subject.Commit != "" {
					carryUnits = append(carryUnits, subject.Commit)
				}
			}
		}
		input := unitMeasureInput(run, runner, installation)
		all.Runs = append(all.Runs, input)
		all.Steps = append(all.Steps, input.Steps...)
	}
	var acts []processmeasure.Act
	checks, missing := carryMeasures(installation, goal, carryUnits...)
	all.Unknown = append(all.Unknown, missing...)
	all.Steps = append(all.Steps, checks...)
	for _, act := range state.Acts {
		if act.AppliedAt.IsZero() {
			continue
		}
		grant := ""
		proof := act.Proof
		if proof.Helm == nil {
			proof = act.AppliedProof
		}
		if proof.Helm != nil {
			grant = fmt.Sprintf("by %s; grant %s", proof.Helm.By, proof.Helm.Grant)
		}
		acts = append(acts, processmeasure.Act{ID: act.ID, Actor: act.Actor, Lineage: act.Lineage, Grant: grant, Requester: act.Proof, Applier: act.AppliedProof, Change: act.Key + " = " + act.After})
	}
	var stops []string
	for _, stop := range state.Stops {
		stops = append(stops, fmt.Sprintf("Automatic process changes are held: %s; observed %v against allowance %v; %s; %s", stop.Stop.Class, stop.Stop.Measure.Now, stop.Stop.Measure.Previous, stop.Stop.Handoff, stop.Impact))
	}
	m := processmeasure.Read(all)
	if runUnknown {
		clear(m.Hours)
		m.Tokens, m.SuiteMinutes = nil, nil
	}
	return processmeasure.Report(m, all.Steps, acts, stops, boundary)
}

func runIntentChannelStatus(inv *intentInvocation) int {
	if problem := inv.selectRoot(); problem != nil {
		return inv.render(*problem)
	}
	var out, problem bytes.Buffer
	code := channelStatus(inv.stateRoot, inv.input.switched("post"), &out, &problem, inv.owners.dependencies.machine, inv.owners.dependencies.endpoint, nil, func(root string, _ time.Time, boundary processmeasure.Watermark) processmeasure.Projection {
		return readProcessReport(root, inv.layout.InstallationRoot.Path(), "", "", inv.unitRunner(), inv.unitRunner().Manager.Now(), boundary)
	})
	result := intentResult{Outcome: intentConfirmed, code: code, Summary: "this seat's status", text: strings.Split(strings.TrimSpace(out.String()), "\n"), Details: strings.Split(strings.TrimSpace(problem.String()), "\n")}
	if code != 0 {
		result.Outcome = intentFailed
	}
	return inv.render(result)
}
