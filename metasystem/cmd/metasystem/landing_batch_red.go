package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/trunkredmap"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func init() {
	compiledBatchCapabilities[freshBaseDiagnosis] = struct{}{}
	compiledBatchCapabilities[ejectionAndRedScheduling] = struct{}{}
}

var productionTrunkRedLedgerOwner = productionBatchLedgerOwner
var batchDiagnosticLauncher = launchBatchDiagnostic

var batchDiagnosisSeams = struct {
	commitForTree func(string, string, string) (string, error)
	diagnose      func(batch.Store, string, string, []batch.RedGroup, string, time.Time, batch.RedSeams) error
}{commitForTree, batch.DiagnoseRed}

func executeBatchDiagnosis(root, id, actor string, at time.Time) error {
	ledgerOwner, err := productionTrunkRedLedgerOwner(root)
	if err != nil {
		return err
	}
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 || record.Proof == nil {
		return fmt.Errorf("batch %s has no diagnostic authority member or proof", id)
	}
	baseCommit, err := batchDiagnosisSeams.commitForTree(root, "origin/main", record.BaseTree)
	if err != nil {
		return err
	}
	return batchDiagnosisSeams.diagnose(store, id, actor, record.Proof.RedGroups, record.Proof.PrefixGoal, at, batch.RedSeams{
		Run: func(request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
			return batchDiagnosticLauncher(root, id, request)
		},
		MintOpid: func() (string, error) {
			machine, err := goal.ResolveMachine(root)
			if err != nil {
				return "", err
			}
			ulid, err := goal.NewOperationULID()
			if err != nil {
				return "", err
			}
			return goal.Opid(ulid, machine, landingOwnerLineage), nil
		},
		Ledger:     ledgerOwner,
		BaseCommit: baseCommit,
		UpdateNext: func(goalID, status string) error {
			return batchChildRunner(root, landingOwnerLineage, "goal", "edit", "--root", root, "--id", goalID, "--next", status, "--lineage", landingOwnerLineage)
		},
	})
}

func batchDiagnosticArgs(root string, request batch.DiagnosticRequest, resultPath string) []string {
	return []string{"test", "run", "--root", root, "--goal", request.GoalID, "--tree", request.Tree, "--mode", "canary",
		"--purpose", "diagnostic", "--groups", strings.Join(request.Groups, ","), "--no-reuse", "--result", resultPath,
		"--expected-goal-revision", fmt.Sprint(request.Claim.Revision), "--expected-accounting-revision", fmt.Sprint(request.Claim.AccountingRevision)}
}

var batchDiagnosticExecute = func(binary string, args []string, dir string, environment []string) ([]byte, int, error) {
	command := exec.Command(binary, args...)
	command.Dir, command.Env = dir, environment
	output, err := command.CombinedOutput()
	status := -1
	if command.ProcessState != nil {
		status = command.ProcessState.ExitCode()
	}
	return output, status, err
}

// clearingDiagnostic is the owner's trunk-red clearing run. That path hands the member's claim beside the
// request, and the launcher reads the expected revisions from the request, so the claim goes in first.
func clearingDiagnostic(root string) func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
	return func(batchID string, request batch.DiagnosticRequest, claim batch.Claim) (batch.DiagnosticResult, error) {
		request.Claim = claim
		return launchBatchDiagnostic(root, batchID, request)
	}
}

func launchBatchDiagnostic(root, batchID string, request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
	binary, err := os.Executable()
	if err != nil {
		return batch.DiagnosticResult{}, err
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", batchID+"-diagnostic.json")
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return batch.DiagnosticResult{}, err
	}
	args := batchDiagnosticArgs(root, request, resultPath)
	output, status, runErr := batchDiagnosticExecute(binary, args, root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage))
	if runErr != nil && status == proofrun.ExitAdmissionRefused {
		return batch.DiagnosticResult{}, &batch.DiagnosticRefusal{Status: strings.TrimSpace(string(output))}
	}
	var result proofrun.TestResult
	if readErr := readStrictJSON(resultPath, &result); readErr != nil {
		if runErr != nil {
			return batch.DiagnosticResult{}, fmt.Errorf("batch diagnostic: %s: %w", strings.TrimSpace(string(output)), errors.Join(runErr, readErr))
		}
		return batch.DiagnosticResult{}, readErr
	}
	diagnostic := batch.DiagnosticResult{AttemptID: result.AttemptID, Groups: trunkredmap.ResultToRedGroups(result)}
	if runErr != nil && len(diagnostic.Groups) == 0 {
		return diagnostic, fmt.Errorf("batch diagnostic: %s: %w", strings.TrimSpace(string(output)), runErr)
	}
	return diagnostic, nil
}
