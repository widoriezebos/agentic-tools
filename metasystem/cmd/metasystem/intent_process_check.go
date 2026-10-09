package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
)

type processCheckHeld struct {
	act     processchange.ProcessAct
	remedy  []string
	problem string
}

func (h *processCheckHeld) Error() string {
	if h.problem != "" {
		return "the check was not admitted: " + h.problem
	}
	return "the changed check awaits a person's exact command; nothing was launched"
}

func (inv *intentInvocation) admitProcessCheck(plan launch.UnitPlan, directory string, check *launch.UnitCheck) error {
	act := processchange.ProcessAct{Goal: plan.Goal, Unit: plan.Unit, Operation: directory, Checkout: plan.Worktree, Key: "check", Layer: "unit-check", Lineage: inv.claimLineage(), Actor: "agent", Reason: inv.input.text("reason"), After: check.Cheap}
	person := false
	if inv.input.has("check") {
		by, _, refusal := inv.settingsPerson(inv.layout, "work build check", &act.Proof)
		person = refusal == nil
		if !person && inv.input.text("by") != "" {
			return fmt.Errorf("only a proved person supplies --by for an explicit repair check")
		}
		if person {
			act.Actor, act.Citation = "direct-person", "person-directed"
			check.SelectedBy = by
		}
	}
	act.AfterArgv = strings.Fields(check.Cheap)
	if inv.input.has("check") {
		act.AfterArgv = inv.input.values["check"]
	}
	remedy := inv.typedArgvLess("act", "by", "reason", "json", "verbose", "lineage", "repo", "brief")
	if index := slices.Index(remedy, "--check"); index >= 0 {
		remedy = remedy[:index]
	}
	if !inv.input.has("work") && len(inv.input.args) == 1 {
		remedy = append(remedy, "--work", plan.Unit)
	}
	remedy = append(append(remedy, "--repo", inv.layout.InstallationRoot.Path(), "--brief", inv.callerPath(inv.input.text("brief")), "--reason", "Apply the retained check selection", "--check"), act.AfterArgv...)
	withAct := func(id string) []string {
		return slices.Insert(slices.Clone(remedy), len(remedy)-len(act.AfterArgv)-1, "--act", id)
	}
	applicable := func() []string {
		git := func(root string, args ...string) (string, error) {
			data, err := inv.work().git(root, args...)
			return string(data), err
		}
		cheap, _ := landingProofCommand(filepath.Join(plan.Worktree, inv.layout.InstallationRel), plan.Worktree, "HEAD", "proof.cheap", git)
		return strings.Fields(cheap)
	}
	if person && inv.input.has("check") && inv.input.text("act") == "" && inv.input.text("reason") == "" {
		return fmt.Errorf("a manual check needs --reason TEXT --by NAME before --check")
	}
	admitted, err := processchange.AdmitCheck(processchange.Check{Root: inv.stateRoot, Act: inv.input.text("act"), ProcessAct: act, Applicable: applicable, Person: person, Observation: !inv.input.has("check"), Now: inv.unitRunner().Manager.Now(), Remedy: func(id string) string { return shellCommand(withAct(id)) }})
	if err != nil && (inv.input.has("check") || admitted.Status != "observation-unknown") {
		return &processCheckHeld{act: admitted, remedy: remedy, problem: err.Error()}
	}
	if err != nil {
		fmt.Fprintln(inv.stderr, "warning: check observation unavailable:", err)
	}
	if inv.input.has("check") && admitted.Status != "applied" && admitted.Status != "unchanged" && admitted.Status != "superseded" {
		return &processCheckHeld{act: admitted, remedy: withAct(admitted.ID)}
	}
	if inv.input.has("check") && !person {
		// The no-audits repair is a person's act: an agent running a selected check keeps the declared audits and deadline.
		git := func(root string, args ...string) (string, error) {
			data, err := inv.work().git(root, args...)
			return strings.TrimSpace(string(data)), err
		}
		installation := filepath.Join(plan.Worktree, inv.layout.InstallationRel)
		audits, err := landingProofCommand(installation, plan.Worktree, "HEAD", "proof.audits", git)
		if err != nil {
			return err
		}
		deadline, err := landingProofCommand(installation, plan.Worktree, "HEAD", "proof.deadline", git)
		if err != nil {
			return err
		}
		check.Audits = audits
		if check.Minutes, err = strconv.Atoi(deadline); err != nil {
			return fmt.Errorf("the declared proof.deadline %q is not a number of minutes", deadline)
		}
	}
	if admitted.Status == "superseded" {
		return &processCheckHeld{act: admitted, remedy: remedy, problem: "the earlier selection was superseded"}
	}
	check.ProcessAct = admitted.ID
	return nil
}
