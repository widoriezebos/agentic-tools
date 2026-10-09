package main

import "fmt"

import "github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"

func (inv *intentInvocation) runProcessSetting(key, value string) int {
	destination, checkout, err := inv.policyDestination("process.change")
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
	}
	root := checkoutAuthorityRoot(destination)
	params, err := inv.helmPolicyParams(checkout, "process.change")
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
	}
	now, err := inv.owners.commandNow(root)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: err.Error()})
	}
	act := processchange.ProcessAct{Goal: inv.input.text("goal"), Lineage: inv.claimLineage(), Actor: "agent", Citation: "citation unavailable", Checkout: checkout, Key: key, Layer: "conf-local", After: value, Class: "setting", Rule: inv.input.text("rule"), Measure: inv.input.text("measure"), Reason: inv.input.text("reason")}
	act.Undo, act.Unset = inv.input.text("undo"), inv.command.action == "unset"
	by, _, refusal := inv.settingsPerson(destination, "process setting", &act.Proof)
	person := refusal == nil
	if person {
		act.Actor, act.Citation = "direct-person", "person-directed"
	}
	remedy := []string{"metasystem", "settings", "set", key, value, "--repo", checkout}
	if act.Unset {
		remedy = []string{"metasystem", "settings", "unset", key, "--repo", checkout}
	}
	if act.Undo != "" {
		remedy = append(remedy, "--undo", act.Undo)
		if !person {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "a person must undo this process change", next: remedy, nextReason: "run this at your enrolled terminal"})
		}
	}
	if !person {
		inv.stateRoot = root
		problem := inv.refreshProcessDrift(act.Goal)
		state, readErr := processchange.ReadState(root, "")
		if problem != nil || readErr != nil || len(state.Unknown) > 0 || len(state.Stops) > 0 {
			return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "automatic process changes are held; a person can apply this explicit setting", Details: []string{fmt.Sprint(problem, readErr, state.Unknown)}, Data: state, next: remedy, nextReason: "run this at your enrolled terminal after examining the drift"})
		}
	}
	applied, err := processchange.ApplySetting(processchange.Setting{Root: root, Conf: params.ConfPath, Act: inv.input.text("act"), ProcessAct: act, PersonName: by, Person: person, Now: now,
		Remedy: func(id string) string { return shellCommand(append(remedy, "--act", id)) }})
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Summary: "the process setting was not completed: " + err.Error(), Data: applied})
	}
	if applied.Status == "superseded" {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "the process act was superseded; the setting was preserved", Data: applied, next: remedy, nextReason: "run this at your enrolled terminal to replace the current value"})
	}
	if applied.Status == "unchanged" {
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: []intentTarget{{Kind: "setting", ID: key}}, Summary: key + " already holds that value in " + params.ConfPath + ".local; nothing was changed", Data: map[string]any{"key": key, "value": value, "file": params.ConfPath + ".local"}, view: settingSetView(key + " already holds " + value + " in " + inv.shownPath(params.ConfPath+".local") + "; nothing was changed")})
	}
	if applied.Status != "applied" {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Summary: "the setting awaits a person's exact act; its value was preserved", Data: applied, next: append(remedy, "--act", applied.ID), nextReason: "run this at your enrolled terminal to apply this proposal"})
	}
	summary := key + " is set to " + value
	if act.Unset {
		summary = key + " inherits its value; the local override was removed"
	}
	return inv.render(intentResult{Outcome: intentConfirmed, Summary: summary, Data: applied})
}

var processSettingFlags = []intentFlag{
	{name: "goal", value: "GOAL", usage: "the goal requesting this process setting"},
	{name: "rule", value: "FILE#SECTION", usage: "the repository section supporting the setting"},
	{name: "measure", value: "NAME", usage: "the expected measurement"},
	{name: "reason", value: "TEXT", usage: "why the setting should improve the measurement"},
	{name: "act", value: "ID", usage: "execute the exact retained proposal with fresh authority proof"},
	{name: "undo", value: "ID", usage: "restore a process act's original setting layer as a new act"},
}

func runIntentSettingsUnset(inv *intentInvocation) int {
	if len(inv.input.args) != 1 || inv.input.text("undo") == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Summary: "settings unset needs KEY and --undo ID naming the original change"})
	}
	inv.input.args = append(inv.input.args, "")
	return runIntentSettingsSet(inv)
}
