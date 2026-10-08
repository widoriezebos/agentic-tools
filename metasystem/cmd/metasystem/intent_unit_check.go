package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func (inv *intentInvocation) resolveUnitCheck(plan launch.UnitPlan, directory string) (launch.UnitPlan, error) {
	git := func(root string, args ...string) (string, error) {
		data, err := inv.work().git(root, args...)
		return strings.TrimSpace(string(data)), err
	}
	check := &launch.UnitCheck{Directory: plan.Proof[0].Dir, Environment: os.Environ()}
	if inv.input.has("check") {
		actor, _, problem := inv.actingAs("work build manual check", plan.Goal, actorHuman)
		if problem != nil {
			return plan, fmt.Errorf("%s", problem.Summary)
		}
		check.SelectedBy, check.Reason = unitStopActor(actor), inv.input.text("reason")
		if check.Reason == "" {
			return plan, fmt.Errorf("a manual check needs --reason TEXT --by NAME before --check")
		}
		check.Cheap, check.Audits, check.Minutes = shellCommand(inv.input.values["check"]), "true", 15
		impact := "Impact: use your command with no audits and a 15-minute deadline.\nMissing declarations remain unproved; later rounds use committed declarations.\nCancel this run to stop the repair."
		if err := inv.recordUnitStopOverride(plan.Goal, "work-build-check", check.Reason, impact, check.SelectedBy); err != nil {
			return plan, err
		}
	} else {
		commit, err := git(plan.Worktree, "rev-parse", "HEAD")
		if err != nil {
			return plan, err
		}
		check.SourceTree, err = git(plan.Worktree, "rev-parse", commit+"^{tree}")
		if err != nil {
			return plan, err
		}
		var values [3]string
		for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline"} {
			values[i], err = landingProofCommand(filepath.Join(plan.Worktree, inv.layout.InstallationRel), plan.Worktree, commit, key, git)
			if err != nil {
				return plan, err
			}
		}
		check.Cheap, check.Audits = values[0], values[1]
		check.Minutes, _ = strconv.Atoi(values[2])
	}
	executable, err := os.Executable()
	if err != nil {
		return plan, err
	}
	run := filepath.Base(filepath.Dir(directory))
	brief, err := os.ReadFile(plan.Build.Brief)
	if err != nil {
		return plan, err
	}
	plan.Build.Brief = filepath.Join(directory, "checked-build.md")
	_, err = atomicfile.WriteText(plan.Build.Brief, fmt.Sprintf("Before returning, run: metasystem test run --unit-run %s\n\n%s", run, brief), directory)
	plan.Check = check
	plan.Proof = []launch.ProofCommand{{Name: "unit-check", Dir: check.Directory, Argv: []string{executable, "test", "run", "--unit-run", run}, Env: check.Environment}}
	return plan, err
}

func runIntentUnitCheck(inv *intentInvocation) int {
	record, err := inv.unitRunner().Status(inv.input.text("unit-run"))
	var plan launch.UnitPlan
	if err == nil && len(record.Rounds) > 0 {
		plan, err = launch.ReadUnitPlan(filepath.Join(record.Rounds[len(record.Rounds)-1].Directory, "plan.json"))
	}
	if err == nil && plan.Check == nil {
		err = fmt.Errorf("this run has no frozen declared check")
	}
	var execution string
	var exits []launch.CheckExit
	if err == nil {
		var top []byte
		top, err = inv.work().git(inv.cwd, "rev-parse", "--show-toplevel")
		if err == nil {
			relative, relErr := filepath.Rel(realpath.ResolveExisting(plan.Worktree), realpath.ResolveExisting(plan.Check.Directory))
			err = relErr
			if err == nil {
				round := record.Rounds[len(record.Rounds)-1].Directory
				records := filepath.Join(record.Worktree, "artifacts", "unit-checks", record.ID, filepath.Base(round))
				lookup := inv.owners.lookupEnv
				if lookup == nil {
					lookup = os.LookupEnv
				}
				if kind, _ := lookup(launch.KindEnv); kind == "proof" {
					records = round
				}
				execution, exits, err = plan.Check.Run(filepath.Join(strings.TrimSpace(string(top)), relative), records)
			}
		}
	}
	result := intentResult{Outcome: intentConfirmed, Summary: "the frozen cheap check and audits passed", Data: map[string]any{"execution": execution, "exits": exits}}
	if err != nil {
		result.Outcome, result.code, result.Summary = intentFailed, 1, "the unit check failed: "+err.Error()
	}
	return inv.render(result)
}
