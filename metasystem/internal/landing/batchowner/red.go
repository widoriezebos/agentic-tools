package batchowner

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

var ProductionTrunkRedLedgerOwner = productionBatchLedgerOwner
var BatchDiagnosticLauncher = LaunchBatchDiagnostic

var BatchDiagnosisSeams = struct {
	CommitForTree func(string, string, string) (string, error)
	Diagnose      func(batch.Store, string, string, []batch.RedGroup, string, time.Time, batch.RedSeams) error
	RedLanguage   func(string) func(batch.RedGroup) (adapter.Adapter, bool)
	Sources       func(string, batch.Record) (map[string]string, error)
}{commitForTree, batch.DiagnoseRed, productionBatchRedLanguage, BatchRetainedSources}

// batchRedAdapters names each red group's contract adapter, with a command
// group's evidence format, so the lane judges identities from the record.
func batchRedAdapters(root string, groups []batch.RedGroup) []batch.RedGroup {
	_, contract, _, err := testrun.LoadContract(batch.ModuleRoot(root))
	if err != nil {
		return groups
	}
	return BatchNameRedAdapters(contract, groups)
}

func BatchNameRedAdapters(contract testpolicy.Contract, groups []batch.RedGroup) []batch.RedGroup {
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
	_, contract, _, err := testrun.LoadContract(batch.ModuleRoot(root))
	if err != nil {
		return nil
	}
	return BatchRedLanguageFromContract(contract)
}

// BatchRedLanguageFromContract resolves a red group to its adapter; a group
// named TEMPLATE/PACKAGE whose template is a packageSelection selector is an
// expansion, whose manifest is the whole module. An unknown group has none.
func BatchRedLanguageFromContract(contract testpolicy.Contract) func(batch.RedGroup) (adapter.Adapter, bool) {
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

func ExecuteBatchDiagnosis(root, id, actor string, at time.Time) error {
	return ExecuteBatchDiagnosisWithConfig(root, id, actor, at, nil)
}

func ExecuteBatchDiagnosisWithConfig(root, id, actor string, at time.Time, lookup func(string, string) (string, error)) error {
	controlRoot := batch.ModuleRoot(root)
	ledgerOwner, err := ProductionTrunkRedLedgerOwner(controlRoot)
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
		return fmt.Errorf("batch %s has no member or test run to diagnose", id)
	}
	baseCommit, err := BatchDiagnosisSeams.CommitForTree(root, "origin/main", record.BaseTree)
	if err != nil {
		return err
	}
	return BatchDiagnosisSeams.Diagnose(store, id, actor, batchRedAdapters(root, record.Proof.RedGroups), record.Proof.PrefixGoal, at, batch.RedSeams{
		Run: func(request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
			result, err := BatchDiagnosticLauncher(controlRoot, id, request)
			result.Groups = batchRedAdapters(root, result.Groups)
			return result, err
		},
		Sources:  func(record batch.Record) (map[string]string, error) { return BatchDiagnosisSeams.Sources(root, record) },
		Location: time.Local,
		ConfirmFenced: func(unit batch.Unit) (string, bool, error) {
			liveRoot, liveAt, projection, err := batchAuthorityProjection(root)
			if err != nil {
				return "", false, err
			}
			err = AuthorizeBatchMemberInProjection(liveRoot, liveAt, record, unit, projection)
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
			return goal.Opid(ulid, machine, LandingOwnerLineage), nil
		},
		Ledger:     ledgerOwner,
		BaseCommit: baseCommit,
		Adapter:    BatchDiagnosisSeams.RedLanguage(root),
		UpdateNext: func(goalID, status string) error {
			return batchEditNext(controlRoot, goalID, status)
		},
		LaneOwner: func() (string, bool) { return laneRegistrar(controlRoot) },
	})
}

func batchDiagnosticArgs(root string, request batch.DiagnosticRequest, resultPath string) []string {
	args := append(append([]string{"internal", "test", "run", "--root", root}, accountFlag(request.GoalID)...), "--tree", request.Tree, "--mode", "canary",
		"--purpose", "diagnostic", "--groups", strings.Join(request.Groups, ","), "--no-reuse", "--result", resultPath, "--json")
	return append(args, accountRevisions(request.GoalID, request.Claim)...)
}

// BatchDiagnosticExecute runs the diagnostic test run child and reads its
// --json envelope; an unreadable envelope is an error, never a result.
var BatchDiagnosticExecute = func(binary string, args []string, dir string, environment []string) (verbresult.Result, error) {
	command := BatchProofCommand(binary, args, false)
	command.Dir, command.Env = dir, environment
	return runTestRunChild(command)
}

// clearingDiagnostic is the owner's trunk-red clearing run. That path hands the member's claim beside the
// request, and the launcher reads the expected revisions from the request, so the claim goes in first.
func clearingDiagnostic(root string) func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
	return ClearingDiagnosticWithLaunch(root, LaunchBatchDiagnostic)
}

func ClearingDiagnosticWithLaunch(root string,
	launch func(string, string, batch.DiagnosticRequest) (batch.DiagnosticResult, error),
) func(string, batch.DiagnosticRequest, batch.Claim) (batch.DiagnosticResult, error) {
	controlRoot := batch.ModuleRoot(root)
	return func(batchID string, request batch.DiagnosticRequest, claim batch.Claim) (batch.DiagnosticResult, error) {
		request.Claim = claim
		return launch(controlRoot, batchID, request)
	}
}

func LaunchBatchDiagnostic(root, batchID string, request batch.DiagnosticRequest) (batch.DiagnosticResult, error) {
	return LaunchBatchDiagnosticWithExecute(root, batchID, request, BatchDiagnosticExecute)
}

func LaunchBatchDiagnosticWithExecute(root, batchID string, request batch.DiagnosticRequest,
	execute func(string, []string, string, []string) (verbresult.Result, error),
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
	child, err := execute(binary, args, root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+LandingOwnerLineage))
	if err != nil {
		return batch.DiagnosticResult{}, fmt.Errorf("batch diagnostic: %w", err)
	}
	if child.Outcome == verbresult.Refused {
		return batch.DiagnosticResult{}, &batch.DiagnosticRefusal{Status: child.Err().Error()}
	}
	var result proofrun.TestResult
	if readErr := strictjson.Read(resultPath, &result); readErr != nil {
		if child.Outcome != verbresult.Confirmed {
			return batch.DiagnosticResult{}, fmt.Errorf("batch diagnostic: %w", errors.Join(child.Err(), readErr))
		}
		return batch.DiagnosticResult{}, readErr
	}
	diagnostic := batch.DiagnosticResult{AttemptID: result.AttemptID, Groups: batch.ResultToRedGroups(result), Sample: sample}
	for _, group := range result.Groups {
		diagnostic.Evidence = append(diagnostic.Evidence, batch.GroupEvidence{ID: group.ID, Status: group.Status, ExecutionIdentity: group.ExecutionIdentity,
			LogPath: group.LogPath, LogDigest: group.LogDigest, NativeLaunched: group.NativeLaunched, CollectionComplete: group.CollectionComplete})
	}
	if child.Outcome != verbresult.Confirmed && len(diagnostic.Groups) == 0 {
		return diagnostic, fmt.Errorf("batch diagnostic: %w", child.Err())
	}
	return diagnostic, nil
}
