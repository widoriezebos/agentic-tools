package main

import (
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func init() {
	compiledBatchCapabilities[freshBaseDiagnosis] = struct{}{}
	compiledBatchCapabilities[ejectionAndRedScheduling] = struct{}{}
}

var productionTrunkRedLedgerOwner = func(string) batch.LedgerOwner { return batch.UnboundLedgerOwner{} }

func executeBatchDiagnosis(root, id, actor string, at time.Time) error {
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 || record.Proof == nil {
		return fmt.Errorf("batch %s has no diagnostic authority member or proof", id)
	}
	head := joined[len(joined)-1]
	baseCommit, err := commitForTree(root, "origin/main", record.BaseTree)
	if err != nil {
		return err
	}
	return batch.DiagnoseRed(store, id, actor, record.Proof.RedGroups, "", at, batch.RedSeams{
		Run: func(request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
			return launchBatchDiagnostic(root, id, request, head.Claim)
		},
		MintOpid:   func() (string, error) { return goal.NewOperationULID() },
		Ledger:     productionTrunkRedLedgerOwner(root),
		BaseCommit: baseCommit,
		UpdateNext: func(goalID, status string) error {
			return batchChildRunner(root, landingOwnerLineage, "goal", "edit", "--root", root, "--id", goalID, "--next", status, "--lineage", landingOwnerLineage)
		},
	})
}

func launchBatchDiagnostic(root, batchID string, request batch.DiagnosticRequest, claim batch.Claim) (batch.DiagnosticResult, error) {
	binary, err := os.Executable()
	if err != nil {
		return batch.DiagnosticResult{}, err
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", batchID+"-diagnostic.json")
	args := []string{"test", "run", "--root", root, "--goal", request.GoalID, "--tree", request.Tree, "--mode", "canary",
		"--purpose", "diagnostic", "--groups", strings.Join(request.Groups, ","), "--no-reuse", "--result", resultPath,
		"--expected-goal-revision", fmt.Sprint(claim.Revision), "--expected-accounting-revision", fmt.Sprint(claim.AccountingRevision)}
	command := exec.Command(binary, args...)
	command.Dir, command.Env = root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, runErr := command.CombinedOutput()
	if runErr != nil && command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitAdmissionRefused {
		return batch.DiagnosticResult{}, &batch.DiagnosticRefusal{Status: strings.TrimSpace(string(output))}
	}
	var result proofrun.TestResult
	if err := readStrictJSON(resultPath, &result); err != nil {
		return batch.DiagnosticResult{}, err
	}
	diagnostic := batch.DiagnosticResult{AttemptID: result.AttemptID}
	for _, group := range result.Groups {
		if group.Status == "passed" || group.Status == "reused" {
			continue
		}
		red := batch.RedGroup{ID: group.ID, Status: group.Status, NotRunReason: group.NotRunReason, LogPath: group.LogPath,
			LogDigest: group.LogDigest, InputManifest: slices.Clone(group.InputManifest)}
		for _, observed := range group.Observed {
			if observed.Status == "failed" {
				red.Failures = append(red.Failures, batch.Failure{Report: observed.Report, Classname: observed.Classname, Name: observed.Name, Status: observed.Status, Reason: observed.Reason})
			}
		}
		diagnostic.Groups = append(diagnostic.Groups, red)
	}
	if runErr != nil && len(diagnostic.Groups) == 0 {
		return diagnostic, fmt.Errorf("batch diagnostic: %s: %w", strings.TrimSpace(string(output)), runErr)
	}
	return diagnostic, nil
}
