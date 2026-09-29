package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
)

var productionTrunkRedLedgerOwner = productionBatchLedgerOwner
var batchDiagnosticLauncher = launchBatchDiagnostic

var batchDiagnosisSeams = struct {
	commitForTree func(string, string, string) (string, error)
	diagnose      func(batch.Store, string, string, []batch.RedGroup, string, time.Time, batch.RedSeams) error
	redLanguage   func(string) func(batch.RedGroup) (adapter.Adapter, bool)
	sources       func(string, batch.Record) (map[string]string, error)
}{commitForTree, batch.DiagnoseRed, productionBatchRedLanguage, batchRetainedSources}

// batchRedAdapters names each red group's contract adapter, with a command
// group's evidence format, so the lane judges identities from the record.
func batchRedAdapters(root string, groups []batch.RedGroup) []batch.RedGroup {
	_, contract, _, err := loadPhysicalTestingContract(batch.ModuleRoot(root))
	if err != nil {
		return groups
	}
	return batchNameRedAdapters(contract, groups)
}

func batchNameRedAdapters(contract testpolicy.Contract, groups []batch.RedGroup) []batch.RedGroup {
	named := slices.Clone(groups)
	for index := range named {
		if group, _, found := batchContractGroup(contract, named[index].ID); found {
			named[index].Adapter = group.Adapter
			if group.Adapter == "command" {
				named[index].Adapter += ":" + group.Format
			}
		}
	}
	return named
}

// batchContractGroup finds a red group's contract group; TEMPLATE/PACKAGE
// under a packageSelection template is an expansion of that template.
func batchContractGroup(contract testpolicy.Contract, id string) (testpolicy.Group, bool, bool) {
	find := func(id string) (testpolicy.Group, bool) {
		for _, group := range contract.Groups {
			if group.ID == id {
				return group, true
			}
		}
		return testpolicy.Group{}, false
	}
	if group, found := find(id); found {
		return group, false, true
	}
	template, _, cut := strings.Cut(id, "/")
	group, found := find(template)
	return group, true, cut && found && group.PackageSelection != ""
}

// productionBatchRedLanguage binds red groups to their adapters through the
// installation's testing contract; an unreadable contract binds none, so
// naming falls back on declared manifests alone.
func productionBatchRedLanguage(root string) func(batch.RedGroup) (adapter.Adapter, bool) {
	_, contract, _, err := loadPhysicalTestingContract(batch.ModuleRoot(root))
	if err != nil {
		return nil
	}
	return batchRedLanguageFromContract(contract)
}

// batchRedLanguageFromContract resolves a red group to its adapter; a group
// named TEMPLATE/PACKAGE whose template is a packageSelection selector is an
// expansion, whose manifest is the whole module. An unknown group has none.
func batchRedLanguageFromContract(contract testpolicy.Contract) func(batch.RedGroup) (adapter.Adapter, bool) {
	return func(red batch.RedGroup) (adapter.Adapter, bool) {
		group, expansion, found := batchContractGroup(contract, red.ID)
		if !found {
			return nil, false
		}
		language, err := adapter.Resolve(group)
		if err != nil {
			return nil, false
		}
		return language, expansion
	}
}

func executeBatchDiagnosis(root, id, actor string, at time.Time) error {
	return executeBatchDiagnosisWithConfig(root, id, actor, at, nil)
}

func executeBatchDiagnosisWithConfig(root, id, actor string, at time.Time, lookup func(string, string) (string, error)) error {
	controlRoot := batch.ModuleRoot(root)
	ledgerOwner, err := productionTrunkRedLedgerOwner(controlRoot)
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
	return batchDiagnosisSeams.diagnose(store, id, actor, batchRedAdapters(root, record.Proof.RedGroups), record.Proof.PrefixGoal, at, batch.RedSeams{
		Run: func(request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
			result, err := batchDiagnosticLauncher(controlRoot, id, request)
			result.Groups = batchRedAdapters(root, result.Groups)
			return result, err
		},
		Sources:  func(record batch.Record) (map[string]string, error) { return batchDiagnosisSeams.sources(root, record) },
		Location: time.Local,
		ConfirmFenced: func(unit batch.Unit) (string, bool, error) {
			liveRoot, liveAt, projection, err := batchAuthorityProjection(root)
			if err != nil {
				return "", false, err
			}
			err = authorizeBatchMemberInProjection(liveRoot, liveAt, record, unit, projection)
			var fenced *batch.PrefixFencedRefusal
			if errors.As(err, &fenced) {
				return fenced.Error(), true, nil
			}
			return "", false, nil
		},
		MintOpid: func() (string, error) {
			resolveMachine := goal.ResolveMachine
			if lookup != nil {
				resolveMachine = func(root string) (string, error) { return goal.ResolveMachineWithConfig(root, lookup) }
			}
			machine, err := resolveMachine(controlRoot)
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
		Adapter:    batchDiagnosisSeams.redLanguage(root),
		UpdateNext: func(goalID, status string) error {
			return batchEditNext(controlRoot, goalID, status)
		},
	})
}

func batchDiagnosticArgs(root string, request batch.DiagnosticRequest, resultPath string) []string {
	return append([]string{"internal", "test", "run", "--root", root, "--tree", request.Tree, "--mode", "canary",
		"--purpose", "diagnostic", "--groups", strings.Join(request.Groups, ","), "--no-reuse", "--result", resultPath},
		accountArgs(request.GoalID, request.Claim)...)
}

var batchDiagnosticExecute = func(binary string, args []string, dir string, environment []string) ([]byte, int, error) {
	command := batchProofCommand(binary, args, false)
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
	return clearingDiagnosticWithLaunch(root, launchBatchDiagnostic)
}

func clearingDiagnosticWithLaunch(root string,
	launch func(string, string, batch.DiagnosticRequest) (batch.DiagnosticResult, error),
) func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
	controlRoot := batch.ModuleRoot(root)
	return func(batchID string, request batch.DiagnosticRequest, claim batch.Claim) (batch.DiagnosticResult, error) {
		request.Claim = claim
		return launch(controlRoot, batchID, request)
	}
}

func launchBatchDiagnostic(root, batchID string, request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
	return launchBatchDiagnosticWithExecute(root, batchID, request, batchDiagnosticExecute)
}

func launchBatchDiagnosticWithExecute(root, batchID string, request batch.DiagnosticRequest,
	execute func(string, []string, string, []string) ([]byte, int, error),
) (batch.DiagnosticResult, error) {
	binary, err := os.Executable()
	if err != nil {
		return batch.DiagnosticResult{}, err
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", batchID+"-diagnostic.json")
	if err := os.Remove(resultPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return batch.DiagnosticResult{}, err
	}
	// A batch of changes is diagnosed on the lane's account (U11b).
	if request.GoalID, err = batchChargeID(root, batch.Unit{GoalID: request.GoalID}, nil); err != nil {
		return batch.DiagnosticResult{}, &batch.DiagnosticRefusal{Status: err.Error()}
	}
	args := batchDiagnosticArgs(root, request, resultPath)
	// The host load when the run starts, recorded with a flake's sightings.
	sample := proofrun.SampleLoad(root, "", int64(os.Getpid()), time.Now())
	output, status, runErr := execute(binary, args, root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage))
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
	diagnostic := batch.DiagnosticResult{AttemptID: result.AttemptID, Groups: batch.ResultToRedGroups(result), Sample: sample}
	for _, group := range result.Groups {
		diagnostic.Evidence = append(diagnostic.Evidence, batch.GroupEvidence{ID: group.ID, Status: group.Status, ExecutionIdentity: group.ExecutionIdentity,
			LogPath: group.LogPath, LogDigest: group.LogDigest, NativeLaunched: group.NativeLaunched, CollectionComplete: group.CollectionComplete})
	}
	if runErr != nil && len(diagnostic.Groups) == 0 {
		return diagnostic, fmt.Errorf("batch diagnostic: %s: %w", strings.TrimSpace(string(output)), runErr)
	}
	return diagnostic, nil
}
