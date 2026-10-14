package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/realpath"
)

func (inv *intentInvocation) resolveUnitCheck(plan launch.UnitPlan, directory string) (launch.UnitPlan, error) {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return plan, fmt.Errorf("%s", problem.Summary)
	}
	if !inv.input.has("check") && plan.Check != nil && plan.Check.Declaration != nil && plan.Path == filepath.Join(directory, "plan.json") {
		return plan, inv.admitUnitDeclaration(&plan, directory, nil)
	}
	check := &launch.UnitCheck{Base: plan.Base, Directory: plan.Proof[0].Dir,
		Environment: commandenv.EngineEnvironmentInDirectory(plan.Proof[0].Env, filepath.Join(inv.layout.InstallationRoot.Path(), "bin"))}
	laneFix := inv.connection().laneFix(inv.layout.InstallationRoot.Path(), plan.Worktree, plan.Goal)
	laneCheck := laneFix != nil && slices.Equal(inv.input.values["check"], []string{"metasystem test impact"})
	if laneCheck {
		check.Cheap, check.Audits, check.Minutes = "metasystem test impact --check", "true", 15
	} else if inv.input.has("check") {
		if laneFix != nil {
			if _, _, problem := inv.actingAs("work build manual check", plan.Goal, actorHuman); problem != nil {
				return plan, fmt.Errorf("%s", problem.Summary)
			}
			if inv.input.text("reason") == "" {
				return plan, fmt.Errorf("a manual check needs --reason TEXT --by NAME before --check")
			}
		}
		plan.Check = check
		if err := inv.admitUnitDeclaration(&plan, directory, func() error { return nil }); err != nil {
			return plan, err
		}
		check.ProcessAct = ""
		check.SelectedBy, check.Reason = inv.input.text("by"), inv.input.text("reason")
		check.Cheap, check.Audits, check.Minutes = shellCommand(inv.input.values["check"]), "true", 15
		if err := inv.admitProcessCheck(plan, directory, check); err != nil {
			return plan, err
		}
		check.Declaration = nil
		plan.FullArgv = check.FullArgv
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
	if laneCheck {
		brief = append([]byte("Stage the repair before running its check.\n"), brief...)
	}
	plan.Build.Brief = filepath.Join(directory, "checked-build.md")
	plan.Check = check
	plan.Proof = []launch.ProofCommand{{Name: "unit-check", Dir: check.Directory, Argv: []string{executable, "test", "run", "--unit-run", run, "--repo", inv.layout.InstallationRoot.Path()}, Env: check.Environment}}
	publish := func() error {
		if _, err := atomicfile.WriteText(plan.Build.Brief, fmt.Sprintf("Before returning, run: metasystem test run --unit-run %s\n\n%s", run, brief), directory); err != nil {
			return err
		}
		data, err := json.Marshal(plan)
		if err == nil {
			_, err = atomicfile.WriteFile(filepath.Join(directory, "plan.json"), append(data, '\n'), 0600, "")
		}
		return err
	}
	if !inv.input.has("check") {
		err = inv.admitUnitDeclaration(&plan, directory, publish)
		if err == nil {
			err = inv.admitProcessCheck(plan, directory, check)
		}
		return plan, err
	}
	return plan, publish()
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
				parent, role, brief := launch.CheckParent(record.Rounds[len(record.Rounds)-1], inv.unitRunner().Manager.Store, inv.unitRunner().Manager.Prober)
				records := filepath.Join(record.Worktree, "artifacts", "unit-checks", record.ID, filepath.Base(round))
				lookup := inv.owners.lookupEnv
				if lookup == nil {
					lookup = os.LookupEnv
				}
				kind, _ := lookup(launch.KindEnv)
				if role == "attestation" || kind == "proof" {
					records = round
				}
				body, briefErr := os.ReadFile(brief)
				context := launch.CheckContext{CheckExecution: launch.CheckExecution{Goal: record.Goal, Run: record.ID, Round: filepath.Base(round), Role: role, BriefSHA256: fmt.Sprintf("%x", sha256.Sum256(body))}, Parent: parent, Now: inv.unitRunner().Manager.Now}
				if briefErr != nil {
					context.BriefSHA256 = ""
				}
				execution, exits, err = plan.Check.Run(filepath.Join(strings.TrimSpace(string(top)), relative), records, context)
			}
		}
	}
	for _, command := range exits {
		if _, writeErr := fmt.Fprint(inv.stderr, command.Output); writeErr != nil {
			err = writeErr
			break
		}
	}
	result := intentResult{Outcome: intentConfirmed, Summary: "the frozen cheap check and audits passed", Data: map[string]any{"execution": execution, "exits": exits}}
	if err != nil {
		result.Outcome, result.code, result.Summary = intentFailed, 1, "the unit check failed: "+err.Error()
	}
	return inv.render(result)
}

// unitCheckPerson uses terminal enrollment when no name is supplied and
// returns the person's refusal when a supplied name cannot be proved.
func (inv *intentInvocation) unitCheckPerson(observed *humanauthority.Proof) (string, bool, error) {
	if inv.input.text("by") == "" {
		by, _, refusal := inv.settingsPerson(inv.layout, "work build check", observed)
		return by, refusal == nil, nil
	}
	actor, proof, refusal := inv.actingAs("work build check", "", actorHuman)
	if refusal != nil {
		return "", false, &processCheckHeld{problem: refusal.Summary, remedy: refusal.next}
	}
	if proof == nil || !proof.EnrolledTerminalFor(checkoutAuthorityRoot(inv.layout)) || proof.Helm != nil {
		return "", false, fmt.Errorf("only a proved person supplies --by for an explicit repair check")
	}
	*observed = *proof
	return unitStopActor(actor), true, nil
}

func (inv *intentInvocation) admitUnitDeclaration(plan *launch.UnitPlan, directory string, publish func() error, subject ...branch.AttestationSubject) error {
	check := plan.Check
	act := processchange.ProcessAct{Goal: plan.Goal, Unit: plan.Unit, Operation: directory, Checkout: checkoutAuthorityRoot(inv.layout), Key: "declarations", Layer: "committed", After: check.Cheap, AfterDeclaration: check.Declaration, Lineage: inv.claimLineage(), Reason: "Approve the committed unit checks"}
	var by string
	var person bool
	if inv.input.has("check") {
		var err error
		by, person, err = inv.unitCheckPerson(&act.Proof)
		if err != nil {
			return err
		}
		// A person's explicit command repairs even unavailable declaration history.
		if person {
			return nil
		}
	} else {
		actor, proof, refusal := inv.actingAs("committed unit checks", plan.Goal, actorHuman)
		by = unitStopActor(actor)
		if proof != nil {
			act.Proof = *proof
		}
		person = refusal == nil && act.Proof.EnrolledTerminalFor(act.Checkout) && act.Proof.Helm == nil
	}
	var carrySubject string
	if len(subject) != 0 {
		carrySubject = subject[0].Commit
	}
	remedy := inv.typedArgvLess("act", "json", "verbose", "repo", "brief", "plan")
	if inv.input.has("check") {
		remedy = inv.publicArgv("work", "build", unitRunPrefix+filepath.Base(filepath.Dir(directory)))
	} else {
		for _, flag := range []string{"brief", "plan"} {
			if inv.input.has(flag) {
				remedy = append(remedy, "--"+flag, inv.callerPath(inv.input.text(flag)))
			}
		}
	}
	remedy = append(remedy, "--repo", inv.layout.InstallationRoot.Path())
	if len(subject) != 0 {
		remedy = inv.publicArgv("work", "review", "--commit", subject[0].Commit, "--goal", plan.Goal, "--repo", inv.layout.InstallationRoot.Path(), "--check-only")
	}
	withAct := func(id string) []string { return append(slices.Clone(remedy), "--act", id) }
	git := func(args ...string) (string, error) {
		data, err := inv.work().git(plan.Worktree, args...)
		return strings.TrimSpace(string(data)), err
	}
	var candidateCommit, branch string
	source := func() (candidate, main, inherited processchange.Declaration, err error) {
		readGit := func(args ...string) string {
			if err != nil {
				return ""
			}
			data, problem := inv.work().git(plan.Worktree, args...)
			err = problem
			if args[0] == "show" {
				return string(data)
			}
			return strings.TrimSpace(string(data))
		}
		candidateCommit = readGit("rev-parse", "HEAD")
		branch = "goal/" + plan.Goal
		if len(subject) == 0 {
			branch = readGit("symbolic-ref", "--short", "HEAD")
		} else if candidateCommit != subject[0].Commit || readGit("rev-parse", "HEAD^{tree}") != subject[0].Tree {
			err = fmt.Errorf("the carry subject moved")
		}
		if err == nil && branch != "goal/"+plan.Goal {
			err = fmt.Errorf("the worktree is not this goal's branch: %s", branch)
		}
		endpoint, problem := inv.connection().endpoint(inv.layout.InstallationRoot.Path())
		if problem != nil {
			return candidate, main, inherited, problem
		}
		mainRef := endpoint.Remote + "/" + strings.TrimPrefix(endpoint.Branch, "refs/heads/")
		if endpoint.Remote == "local" {
			mainRef = endpoint.Branch
		}
		mainCommit := readGit("rev-parse", mainRef)
		base := readGit("merge-base", candidateCommit, mainCommit)
		read := func(commit string) (snapshot processchange.Declaration) {
			snapshot.Commit, snapshot.Branch = commit, branch
			snapshot.Tree = readGit("rev-parse", commit+"^{tree}")
			snapshot.Directory, _ = filepath.Rel(plan.Worktree, check.Directory)
			snapshot.EnvironmentSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(check.Environment, "\x00"))))
			snapshot.Content = readGit("show", commit+":"+filepath.ToSlash(filepath.Join(inv.layout.InstallationRel, "metasystem.conf")))
			snapshot.SourceSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(snapshot.Content)))
			for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline", "proof.full"} {
				if err != nil {
					break
				}
				var present bool
				snapshot.Values[i], present, err = config.CommittedContentLookup(snapshot.Content, key)
				// Carrying a read requires the subject's cheap checks; a full
				// command is compared only when the repository declares one.
				if len(subject) != 0 && key == "proof.full" && err == nil && !present {
					continue
				}
				if err == nil {
					err = config.SettingValueProblem(key, snapshot.Values[i])
				}
			}
			if snapshot.Values[3] != "" {
				snapshot.FullArgv = []string{"/bin/sh", "-c", snapshot.Values[3]}
			}
			return
		}
		candidate, main, inherited = read(candidateCommit), read(mainCommit), read(base)
		// A distinct explicit check bypasses full-suite classification only
		// when the declared audits also differ from the full command.
		if inv.input.has("check") && candidate.Values == main.Values && candidate.Values == inherited.Values && shellCommand(inv.input.values["check"]) != candidate.Values[3] && candidate.Values[1] != candidate.Values[3] {
			candidate.FullArgv = nil
		}
		if err != nil {
			err = fmt.Errorf("proof.cheap, proof.audits, proof.deadline or proof.full declaration unavailable: %w", err)
		}
		check.Declaration = &candidate
		check.Cheap, check.Audits, check.SourceTree = candidate.Values[0], candidate.Values[1], candidate.Tree
		check.Minutes, _ = strconv.Atoi(candidate.Values[2])
		plan.FullArgv = candidate.FullArgv
		return
	}
	selectedAct := inv.input.text("act")
	if inv.input.has("check") {
		// The selection act belongs to the separate check admission below.
		selectedAct = ""
	}
	if publish == nil {
		selectedAct = check.ProcessAct
	}
	admitted, err := processchange.AdmitDeclaration(processchange.DeclarationAdmission{Check: processchange.Check{Root: inv.stateRoot, Installation: inv.layout.InstallationRoot, Act: selectedAct, Audits: check.Audits, ProcessAct: act, Person: person, Now: inv.unitRunner().Manager.Now(), Remedy: func(id string) string { return shellCommand(withAct(id)) }}, Resume: publish == nil, Source: source,
		Recheck: func() error {
			tip, err := git("rev-parse", "HEAD")
			currentBranch, branchErr := branch, error(nil)
			if len(subject) == 0 {
				currentBranch, branchErr = git("symbolic-ref", "--short", "HEAD")
			}
			if err != nil || branchErr != nil || tip != candidateCommit || currentBranch != branch {
				return fmt.Errorf("the goal branch or tree moved before declaration admission")
			}
			return nil
		},
		Impact: func() error {
			return inv.recordUnitStopOverride(plan.Goal, "work-declarations", act.Reason, "Impact: this admission uses these committed checks, audits and deadline.\nExact full-suite selections run the full suite for this subject. Earlier rounds keep their frozen checks.", by)
		},
		Publish: func(admitted processchange.ProcessAct) error { check.ProcessAct = admitted.ID; return publish() },
		Subject: carrySubject,
	})
	if err != nil {
		if len(subject) != 0 {
			return &processCheckHeld{act: admitted, remedy: append(slices.Clone(remedy), "--reason", "Repair unavailable declarations", "--by", "NAME", "--check", "COMMAND"), problem: err.Error()}
		}
		return &processCheckHeld{act: admitted, remedy: inv.publicArgv("work", "build", unitRunPrefix+filepath.Base(filepath.Dir(directory)), "--reason", "Repair unavailable declarations", "--by", "NAME", "--check", "COMMAND"), problem: err.Error()}
	}
	if admitted.Status == "superseded" {
		return &processCheckHeld{act: admitted, remedy: remedy, problem: "the earlier declaration proposal was superseded"}
	}
	if admitted.Status == "proposed" {
		return &processCheckHeld{act: admitted, remedy: withAct(admitted.ID), problem: ""}
	}
	return nil
}
