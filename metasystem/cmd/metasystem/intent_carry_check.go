package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/commandenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// carrySubjectCheck freezes declarations from the detached subject before
// running either command. Its retained execution survives the workspace.
func (inv *intentInvocation) carrySubjectCheck(directory string, subject branch.AttestationSubject) (branch.GateObservation, error) {
	if problem := inv.selectLayoutRoot(); problem != nil {
		return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("%s", problem.Summary)}
	}
	if subject.Parent == "" {
		return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: errors.New("the unit has no parent to compare its check against")}
	}
	goalID := inv.input.text("goal")
	if goalID == "" && len(inv.input.args) != 0 {
		goalID = inv.input.args[0]
		if strings.HasPrefix(goalID, unitRunPrefix) {
			record, err := inv.unitRunner().Status(strings.TrimPrefix(goalID, unitRunPrefix))
			if err != nil {
				return branch.GateObservation{}, err
			}
			goalID = record.Goal
		}
	}
	if kind, err := branch.KindOfWithRaw(directory, subject.Commit, goalID, inv.work().git); err != nil || kind.Kind != branch.Unit {
		return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("the carry subject is not a unit of goal %s: %v", goalID, err)}
	}
	repair := shellCommand(inv.publicArgv("work", "review", "--commit", subject.Commit, "--goal", goalID, "--repo", inv.layout.InstallationRoot.Path(), "--check-only", "--reason", "Repair unavailable declarations", "--by", "NAME", "--check", "COMMAND"))
	records := inv.layout.InstallationRoot.Path("artifacts", "unit-checks", "carry", subject.Commit)
	path := filepath.Join(records, "subject.json")
	check := launch.UnitCheck{Base: subject.Parent, SourceTree: subject.Tree, Directory: directory, Environment: commandenv.EngineEnvironmentInDirectory(os.Environ(), filepath.Join(inv.layout.InstallationRoot.Path(), "bin"))}
	data, readErr := os.ReadFile(path)
	if readErr == nil && (json.Unmarshal(data, &check) != nil || check.SourceTree != subject.Tree || check.Cheap == "" || check.Audits == "" || check.Minutes <= 0 || check.Environment == nil || check.Declaration != nil && (check.Cheap != check.Declaration.Values[0] || check.Audits != check.Declaration.Values[1])) {
		if !inv.input.has("check") {
			return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("the carry snapshot is unavailable\nrun: %s", repair)}
		}
		if _, err := atomicfile.WriteFile(path+fmt.Sprintf(".damaged-%x", sha256.Sum256(data)), data, 0600, ""); err != nil {
			return branch.GateObservation{}, err
		}
		check = launch.UnitCheck{Base: subject.Parent, SourceTree: subject.Tree, Directory: directory, Environment: commandenv.EngineEnvironmentInDirectory(os.Environ(), filepath.Join(inv.layout.InstallationRoot.Path(), "bin"))}
	} else if readErr != nil && !os.IsNotExist(readErr) {
		return branch.GateObservation{}, readErr
	}
	check.Directory, check.Base = directory, subject.Parent
	plan := launch.UnitPlan{Goal: goalID, Unit: subject.Commit, Worktree: directory, Check: &check}
	publish := func() error {
		data, err := json.Marshal(check)
		if err == nil {
			_, err = atomicfile.WriteFile(path, data, 0600, "")
		}
		return err
	}
	if readErr == nil && !inv.input.has("check") {
		publish = nil
	}
	var err error
	if inv.input.has("check") {
		err = inv.admitUnitDeclaration(&plan, records, func() error { return nil }, subject)
		if err == nil {
			check.ProcessAct = ""
			check.Cheap, check.Audits, check.Minutes = shellCommand(inv.input.values["check"]), "true", 15
			err = inv.admitProcessCheck(plan, records, &check)
			check.Declaration = nil
			if err == nil {
				err = publish()
			}
		}
	} else {
		err = inv.admitUnitDeclaration(&plan, records, publish, subject)
	}
	if err != nil {
		var held *processCheckHeld
		if errors.As(err, &held) {
			err = fmt.Errorf("%s\nrun: %s", held.Error(), shellCommand(held.remedy))
		}
		return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: err}
	}
	gatePath := filepath.Join(records, "gate.json")
	var execution, latest string
	var flake *branch.CheckFlake
	var newest time.Time
	entries, _ := os.ReadDir(records)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "check-") {
			continue
		}
		info, problem := entry.Info()
		if problem != nil {
			return branch.GateObservation{}, problem
		}
		observations, _ := launch.ReadCheckExecutions(filepath.Join(records, entry.Name()))
		at := info.ModTime()
		if len(observations) > 0 {
			if started, err := time.Parse(time.RFC3339Nano, observations[0].Steps[0].Start); err == nil {
				at = started
			}
		}
		if latest == "" || at.After(newest) {
			newest, latest = at, filepath.Join(records, entry.Name())
		}
	}
	if latest != "" {
		data, problem := os.ReadFile(filepath.Join(latest, "result.json"))
		var prior struct {
			Check launch.UnitCheck
			Exits []launch.CheckExit
		}
		if problem != nil || json.Unmarshal(data, &prior) != nil {
			_, _, refusal := inv.settingsPerson(inv.layout, "repair the unavailable subject execution")
			if (!inv.input.has("check") && !inv.input.has("act") && !inv.input.switched("rerun")) || refusal != nil {
				return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("the newest subject execution is unavailable\nrun: %s", repair)}
			}
		}
		if prior.Check.ProcessAct == check.ProcessAct && prior.Check.Cheap == check.Cheap && prior.Check.Audits == check.Audits && prior.Check.SourceTree == subject.Tree {
			if len(prior.Exits) != 2 || prior.Exits[0].Exit != 0 || prior.Exits[1].Exit != 0 {
				rerun := inv.publicArgv("work", "review", "--commit", subject.Commit, "--goal", goalID, "--repo", inv.layout.InstallationRoot.Path(), "--check-only", "--rerun")
				if check.ProcessAct != "" {
					rerun = append(rerun, "--act", check.ProcessAct)
				}
				if !inv.input.switched("rerun") {
					if flake = inv.carryFlake(check, goalID, subject.Commit, records, latest, prior.Exits); flake == nil {
						return branch.GateObservation{}, fmt.Errorf("the retained subject check failed\nrun: %s", shellCommand(rerun))
					}
				}
			}
			if !inv.input.switched("rerun") {
				execution = latest
			}
		}
	}
	if execution == "" {
		var exits []launch.CheckExit
		execution, exits, err = check.Run(directory, records, launch.CheckContext{CheckExecution: launch.CheckExecution{Goal: goalID, Run: subject.Commit, Role: "carry"}, Now: inv.unitRunner().Manager.Now})
		if err != nil && execution != "" {
			// A retained result means only the commands failed.
			if _, problem := os.Stat(filepath.Join(execution, "result.json")); problem == nil {
				if flake = inv.carryFlake(check, goalID, subject.Commit, records, execution, exits); flake != nil {
					err = nil
				}
			}
		}
	}
	if err != nil {
		return branch.GateObservation{}, err
	}
	evidence, err := os.ReadFile(filepath.Join(execution, "result.json"))
	if err != nil {
		return branch.GateObservation{}, err
	}
	// A check that edits its workspace cannot authorize the original subject.
	status, err := inv.work().git(directory, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return branch.GateObservation{}, err
	}
	tree, err := inv.work().git(directory, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return branch.GateObservation{}, err
	}
	if strings.TrimSpace(string(tree)) != subject.Tree || strings.TrimSpace(string(status)) != "" {
		return branch.GateObservation{}, errors.New("the declared check changed its subject workspace")
	}
	var retained struct {
		ExecutionID string             `json:"executionId"`
		Check       launch.UnitCheck   `json:"check"`
		Exits       []launch.CheckExit `json:"exits"`
		Flake       *branch.CheckFlake `json:"flake,omitempty"`
	}
	if err := json.Unmarshal(evidence, &retained); err != nil {
		return branch.GateObservation{}, err
	}
	retained.Flake = flake
	// The execution retains the environment locally; a committed read carries
	// only the declared commands and their observed exits.
	retained.Check.Environment = []string{}
	evidence, err = json.Marshal(retained)
	if err != nil {
		return branch.GateObservation{}, err
	}
	commands, err := json.Marshal(retained.Check)
	digest := sha256.Sum256(commands)
	gate := branch.GateObservation{Kind: "unit-check", Tree: subject.Tree, RunID: filepath.Base(execution), CommandDigest: hex.EncodeToString(digest[:]), Evidence: string(evidence)}
	if err == nil {
		data, problem := json.Marshal(gate)
		err = problem
		if err == nil {
			_, err = atomicfile.WriteFile(gatePath, data, 0600, "")
		}
	}
	return gate, err
}

// carryFlake classifies a red execution of the frozen check as the unit run
// does: a red the alone rerun of its named tests clears is a flake; anything
// else refuses. A completed rerun of the execution is reused, not repeated.
func (inv *intentInvocation) carryFlake(check launch.UnitCheck, goalID, commit, records, execution string, exits []launch.CheckExit) *branch.CheckFlake {
	if len(exits) != 2 {
		return nil
	}
	evidence := loopstop.RedEvidence{Goal: goalID}
	for _, exit := range exits {
		if exit.Exit == 0 && exit.Error == "" {
			continue
		}
		var units []loopstop.RedUnit
		// Only a cheap command that ended by itself with a complete report names its tests.
		if exit.Name == "cheap" && exit.Exit > 0 {
			if _, named, complete := checkRedUnits([]byte(exit.Output), check.Directory); complete {
				units = named
			}
		}
		if len(units) == 0 {
			units = []loopstop.RedUnit{{Unit: "section/" + exit.Name}}
		}
		evidence.Failed = append(evidence.Failed, units...)
	}
	verdict := loopstop.ClassifyRed(evidence)
	if verdict.Next != "alone" || slices.ContainsFunc(evidence.Failed, func(unit loopstop.RedUnit) bool { return len(unit.Tests) == 0 }) {
		return nil
	}
	// Reruns stay beside, not among, the subject's executions: no directory
	// of theirs is named like one.
	reruns := filepath.Join(records, "alone", "of-"+filepath.Base(execution))
	completed := func(path string) (launch.CheckExit, bool) {
		var result struct {
			Selectors []string           `json:"selectors"`
			Exits     []launch.CheckExit `json:"exits"`
		}
		data, err := os.ReadFile(filepath.Join(path, "result.json"))
		if err != nil || json.Unmarshal(data, &result) != nil || len(result.Exits) != 1 || !slices.Equal(result.Selectors, verdict.Selectors) {
			return launch.CheckExit{}, false
		}
		return result.Exits[0], true
	}
	alone, fresh := "", false
	var exit launch.CheckExit
	entries, _ := os.ReadDir(reruns)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "alone-") {
			continue
		}
		if found, ok := completed(filepath.Join(reruns, entry.Name())); ok {
			alone, exit = filepath.Join(reruns, entry.Name()), found
			break
		}
	}
	if alone == "" {
		var ok bool
		alone, _, _ = check.RunAlone(check.Directory, reruns, verdict.Selectors, launch.CheckContext{CheckExecution: launch.CheckExecution{Goal: goalID, Run: commit, Role: "carry"}, Now: inv.unitRunner().Manager.Now})
		if alone == "" {
			return nil
		}
		if exit, ok = completed(alone); !ok {
			return nil
		}
		fresh = true
	}
	run := loopstop.RedRun{Attempted: true, Exited: exit.Exit >= 0, Launch: filepath.Base(alone), Selectors: verdict.Selectors}
	if run.Exited {
		_, run.Failed, run.Complete = checkRedUnits([]byte(exit.Output), check.Directory)
	}
	run.Passed = run.Exited && exit.Exit == 0 && run.Complete && len(run.Failed) == 0
	evidence.Alone = run
	verdict = loopstop.ClassifyRed(evidence)
	if verdict.Cause == nil || verdict.Cause.Kind != "flake" {
		return nil
	}
	if fresh {
		fmt.Fprintf(inv.stderr, "warning: %s\n", verdict.Words)
	}
	return &branch.CheckFlake{Cause: verdict.Cause.Kind, Flakes: verdict.Flaked, Red: verdict.Words, Alone: run.Launch, Selectors: run.Selectors, AloneExit: exit}
}
