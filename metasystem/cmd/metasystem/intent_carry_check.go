package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// carrySubjectCheck freezes declarations from the detached subject before
// running either command. Its retained execution survives the workspace.
func (inv *intentInvocation) carrySubjectCheck(directory string, subject branch.AttestationSubject) (branch.GateObservation, error) {
	top, err := goalBranchGit(directory, "rev-parse", "--show-toplevel")
	if err != nil {
		return branch.GateObservation{}, err
	}
	check := launch.UnitCheck{SourceTree: subject.Tree, Directory: directory, Environment: os.Environ()}
	var values [3]string
	for i, key := range []string{"proof.cheap", "proof.audits", "proof.deadline"} {
		values[i], err = landingProofCommand(directory, top, subject.Commit, key, goalBranchGit)
		if err != nil {
			return branch.GateObservation{}, &branch.DeclarationUnavailableError{Err: err}
		}
	}
	check.Cheap, check.Audits = values[0], values[1]
	check.Minutes, _ = strconv.Atoi(values[2])
	records := inv.layout.InstallationRoot.Path("artifacts", "unit-checks", "carry", subject.Commit)
	execution, _, err := check.Run(directory, records)
	if err != nil {
		return branch.GateObservation{}, err
	}
	evidence, err := os.ReadFile(filepath.Join(execution, "result.json"))
	if err != nil {
		return branch.GateObservation{}, err
	}
	// A check that edits its workspace cannot authorize the original subject.
	status, err := goalBranchGit(directory, "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return branch.GateObservation{}, err
	}
	tree, err := goalBranchGit(directory, "rev-parse", "HEAD^{tree}")
	if err != nil {
		return branch.GateObservation{}, err
	}
	if tree != subject.Tree || status != "" {
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
	return branch.GateObservation{Kind: "unit-check", Tree: subject.Tree, RunID: filepath.Base(execution), CommandDigest: hex.EncodeToString(digest[:]), Evidence: string(evidence)}, err
}
