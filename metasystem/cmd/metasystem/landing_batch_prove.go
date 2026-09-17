package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
)

func init() { compiledBatchCapabilities[proofPlanningAndTipLaunch] = struct{}{} }

type batchProofLaunch struct {
	Root, BatchID, GoalID, Tree, ResultPath string
	Mode                                    testpolicy.Mode
	GoalRevision, AccountingRevision        uint64
}

type batchProofDependencies struct {
	rearm  func(string, string) error
	plan   func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error)
	launch func(batchProofLaunch) (proofrun.TestResult, error)
}

var productionBatchProofDependencies = batchProofDependencies{
	rearm: rearmBatchBase,
	plan: func(root, goalID, tree string, mode testpolicy.Mode) (testpolicy.Plan, error) {
		return productionBatchPlan(root, goalID, tree, mode)
	},
	launch: launchBatchTipProof,
}

func proofPlanCovers(plan testpolicy.Plan, union []string) bool {
	selected := slices.Clone(plan.SelectedGroups)
	slices.Sort(selected)
	for _, id := range union {
		if _, ok := slices.BinarySearch(selected, id); !ok {
			return false
		}
	}
	return true
}

func executeBatchProof(root, id, actor, window string, sample proofrun.LoadSample, at time.Time, dependencies batchProofDependencies) error {
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if len(record.Units) == 0 {
		return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s has no joined units", id)
	}
	if err := dependencies.rearm(root, record.BaseTree); err != nil {
		return err
	}
	head := record.Units[len(record.Units)-1]
	plan, err := dependencies.plan(root, head.GoalID, record.TipTree, testpolicy.ModeAuto)
	if err != nil {
		return err
	}
	if plan.RequiredMode == testpolicy.ModeDeep || !proofPlanCovers(plan, record.SelectedGroups) {
		plan, err = dependencies.plan(root, head.GoalID, record.TipTree, testpolicy.ModeDeep)
		if err != nil {
			return err
		}
	}
	admitted, err := batch.RequireProofPlan(store, id, actor, window, sample, plan, at)
	if err != nil {
		return err
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", id+".json")
	request := batchProofLaunch{Root: root, BatchID: id, GoalID: head.GoalID, Tree: admitted.TipTree, ResultPath: resultPath,
		Mode: plan.ExecutedMode, GoalRevision: head.Claim.Revision, AccountingRevision: head.Claim.AccountingRevision}
	result, launchErr := dependencies.launch(request)
	if launchErr != nil && strings.Contains(launchErr.Error(), "BATCH_MEMBER_BUDGET_REFUSED") {
		return batch.WithdrawBudgetMember(store, id, head.GoalID, actor, launchErr.Error(), at)
	}
	if finishErr := batch.FinishProof(store, id, actor, result, launchErr, at); finishErr != nil {
		return finishErr
	}
	return launchErr
}

func productionBatchPlan(root, goalID, tree string, mode testpolicy.Mode) (testpolicy.Plan, error) {
	binary, err := os.Executable()
	if err != nil {
		return testpolicy.Plan{}, err
	}
	command := exec.Command(binary, "test", "plan", "--root", root, "--goal", goalID, "--tree", tree,
		"--mode", string(mode), "--purpose", "delivery", "--json")
	command.Dir, command.Env = root, gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("plan batch tip: %w", err)
	}
	var planned testingPlanOutput
	if err := json.Unmarshal(output, &planned); err != nil {
		return testpolicy.Plan{}, err
	}
	return planned.Plan, nil
}

func launchBatchTipProof(request batchProofLaunch) (proofrun.TestResult, error) {
	binary, err := os.Executable()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	args := []string{"test", "run", "--root", request.Root, "--goal", request.GoalID, "--tree", request.Tree,
		"--mode", string(request.Mode), "--purpose", "delivery", "--result", request.ResultPath,
		"--expected-goal-revision", fmt.Sprint(request.GoalRevision),
		"--expected-accounting-revision", fmt.Sprint(request.AccountingRevision)}
	command := exec.Command(binary, args...)
	command.Dir, command.Env = request.Root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, launchErr := command.CombinedOutput()
	var result proofrun.TestResult
	if readErr := readStrictJSON(request.ResultPath, &result); readErr != nil {
		if launchErr == nil {
			launchErr = readErr
		}
	}
	if launchErr != nil && command.ProcessState != nil && batchProofExitAccepted(command.ProcessState.ExitCode(), result) {
		launchErr = nil
	}
	if launchErr != nil {
		launchErr = fmt.Errorf("batch tip proof: %s: %w", strings.TrimSpace(string(output)), launchErr)
	}
	return result, launchErr
}

func batchProofExitAccepted(status int, result proofrun.TestResult) bool {
	return status == 0 || status == proofrun.ExitReusableSuccess && result.Delivery.Sufficient
}

func rearmBatchBase(root, baseTree string) error {
	workspace := gittree.Workspace{Dir: root}
	head, unborn, err := workspace.HeadCommit()
	if err != nil || unborn {
		return fmt.Errorf("re-arm batch base: committed HEAD required")
	}
	headTree, err := workspace.TreeOf(head)
	if err != nil {
		return err
	}
	if headTree == baseTree {
		return nil
	}
	command := exec.Command("git", "-C", root, "log", "--all", "--format=%H %T")
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	if err != nil {
		return err
	}
	baseCommit := ""
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == baseTree {
			baseCommit = fields[0]
			break
		}
	}
	if baseCommit == "" {
		return fmt.Errorf("BATCH_BASE_MOVED: no retained commit has recorded base tree %s", baseTree)
	}
	return landing.FastForwardPreservingRegisters(context.Background(), root, baseCommit)
}
