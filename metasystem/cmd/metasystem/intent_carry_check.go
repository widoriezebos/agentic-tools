package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
	check := launch.UnitCheck{Base: subject.Parent, SourceTree: subject.Tree, Directory: directory, Environment: os.Environ()}
	data, readErr := os.ReadFile(path)
	if readErr == nil && (json.Unmarshal(data, &check) != nil || check.SourceTree != subject.Tree || check.Cheap == "" || check.Audits == "" || check.Minutes <= 0 || check.Environment == nil || check.Declaration != nil && (check.Cheap != check.Declaration.Values[0] || check.Audits != check.Declaration.Values[1])) {
		if !inv.input.has("check") {
			return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("the carry snapshot is unavailable\nrun: %s", repair)}
		}
		if _, err := atomicfile.WriteFile(path+fmt.Sprintf(".damaged-%x", sha256.Sum256(data)), data, 0600, ""); err != nil {
			return branch.GateObservation{}, err
		}
		check = launch.UnitCheck{Base: subject.Parent, SourceTree: subject.Tree, Directory: directory, Environment: os.Environ()}
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
		check.Cheap, check.Audits, check.Minutes = shellCommand(inv.input.values["check"]), "true", 15
		err = inv.admitProcessCheck(plan, records, &check)
		check.Declaration = nil
		if err == nil {
			err = publish()
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
	if data, err := os.ReadFile(gatePath); err == nil {
		var gate branch.GateObservation
		var consumed struct {
			Check       launch.UnitCheck
			ExecutionID string
			Exits       []launch.CheckExit
		}
		gateErr := json.Unmarshal(data, &gate)
		evidenceErr := json.Unmarshal([]byte(gate.Evidence), &consumed)
		commands, _ := json.Marshal(consumed.Check)
		if gateErr == nil && evidenceErr == nil && fmt.Sprintf("%x", sha256.Sum256(commands)) == gate.CommandDigest && consumed.ExecutionID == gate.RunID && len(consumed.Exits) == 2 && consumed.Exits[0].Exit == 0 && consumed.Exits[1].Exit == 0 && consumed.Check.ProcessAct == check.ProcessAct && consumed.Check.Cheap == check.Cheap && consumed.Check.Audits == check.Audits && gate.Tree == subject.Tree {
			return gate, nil
		}
	}
	var execution, latest string
	var newest os.FileInfo
	entries, _ := os.ReadDir(records)
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "check-") {
			continue
		}
		info, problem := entry.Info()
		if problem != nil {
			return branch.GateObservation{}, problem
		}
		if newest == nil || info.ModTime().After(newest.ModTime()) {
			newest, latest = info, filepath.Join(records, entry.Name())
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
			if !inv.input.has("check") || refusal != nil {
				return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: fmt.Errorf("the newest subject execution is unavailable\nrun: %s", repair)}
			}
		}
		if prior.Check.ProcessAct == check.ProcessAct && prior.Check.Cheap == check.Cheap && prior.Check.Audits == check.Audits && prior.Check.SourceTree == subject.Tree {
			if len(prior.Exits) != 2 || prior.Exits[0].Exit != 0 || prior.Exits[1].Exit != 0 {
				return branch.GateObservation{}, fmt.Errorf("the retained subject check failed")
			}
			execution = latest
		}
	}
	if execution == "" {
		execution, _, err = check.Run(directory, records)
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
	}
	if err := json.Unmarshal(evidence, &retained); err != nil {
		return branch.GateObservation{}, err
	}
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
