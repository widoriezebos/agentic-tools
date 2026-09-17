package main

import (
	"context"
	"encoding/json"
	"errors"
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
	seal   func(string, string, string, string, time.Time) error
	plan   func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error)
	launch func(batchProofLaunch) (proofrun.TestResult, error)
}

var productionBatchProofDependencies = batchProofDependencies{
	rearm: rearmBatchBase,
	seal: func(root, id, actor, baseTree string, at time.Time) error {
		return batch.Seal(batch.NewStore(root, nil), id, baseTree, actor, at, productionJoinPlan, productionJoinGate(root))
	},
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
	if record.State == batch.StateSealed && record.Proof != nil && record.Proof.Status == "union-uncovered" && record.Proof.Tree == record.TipTree {
		return nil
	}
	if err := dependencies.rearm(root, record.BaseTree); err != nil {
		return err
	}
	if record.State == batch.StateOpen {
		if dependencies.seal == nil {
			return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: proof seal seam is absent")
		}
		if err := dependencies.seal(root, id, actor, record.BaseTree, at); err != nil {
			return err
		}
		record, err = store.Load(id)
		if err != nil {
			return err
		}
	}
	if record.State != batch.StateSealed {
		return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s is not sealed", id)
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 {
		return fmt.Errorf("BATCH_PROOF_STATE_REFUSED: batch %s has no joined units", id)
	}
	head := joined[len(joined)-1]
	plan, err := planBatchMemberUnion(root, record.TipTree, joined, testpolicy.ModeAuto, dependencies.plan)
	if err != nil {
		return err
	}
	if plan.RequiredMode == testpolicy.ModeDeep || !proofPlanCovers(plan, record.SelectedGroups) {
		plan, err = planBatchMemberUnion(root, record.TipTree, joined, testpolicy.ModeDeep, dependencies.plan)
		if err != nil {
			return err
		}
	}
	if !proofPlanCovers(plan, record.SelectedGroups) {
		reason := "BATCH_PROOF_UNION_UNCOVERED: selected tip plan omits the sealed union"
		if err := batch.RecordUnionRefusal(store, id, actor, reason, at); err != nil {
			return err
		}
		return errors.New(reason)
	}
	admitted, err := batch.RequireProofPlan(store, id, actor, window, sample, plan, at)
	if err != nil {
		return err
	}
	resultPath := filepath.Join(root, "artifacts", "agents", "proof-runs", "batch", id+".json")
	sealed := admitted.Seal[head.GoalID]
	request := batchProofLaunch{Root: root, BatchID: id, GoalID: head.GoalID, Tree: admitted.TipTree, ResultPath: resultPath,
		Mode: plan.ExecutedMode, GoalRevision: sealed.Revision, AccountingRevision: sealed.AccountingRevision}
	result, launchErr := dependencies.launch(request)
	var refused *batchProofAdmissionRefusal
	if errors.As(launchErr, &refused) {
		switch refused.kind {
		case "budget":
			return batch.WithdrawBudgetMember(store, id, head.GoalID, actor, refused.Error(), at)
		case "revision":
			if err := batch.RequestReturn(store, id, head.GoalID, batch.UnitEjected, refused.Error(), actor, at); err != nil {
				return err
			}
			return batch.ReassembleSurvivors(store, id, actor, at)
		default:
			return batch.RefuseProofAdmission(store, id, actor, "admission-refused", refused.Error(), at)
		}
	}
	if finishErr := batch.FinishProof(store, id, actor, result, launchErr, at); finishErr != nil {
		return finishErr
	}
	return launchErr
}

func planBatchMemberUnion(root, tree string, units []batch.Unit, mode testpolicy.Mode, plan func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error)) (testpolicy.Plan, error) {
	union := testpolicy.Plan{RequestedMode: mode, ExecutedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard}
	for _, unit := range units {
		member, err := plan(root, unit.GoalID, tree, mode)
		if err != nil {
			return testpolicy.Plan{}, err
		}
		union.SelectedGroups = append(union.SelectedGroups, member.SelectedGroups...)
		if member.RequiredMode == testpolicy.ModeDeep {
			union.RequiredMode = testpolicy.ModeDeep
		}
		if member.ExecutedMode == testpolicy.ModeDeep {
			union.ExecutedMode = testpolicy.ModeDeep
		}
	}
	slices.Sort(union.SelectedGroups)
	union.SelectedGroups = slices.Compact(union.SelectedGroups)
	return union, nil
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
		"--require-diagnostic-headroom",
		"--expected-goal-revision", fmt.Sprint(request.GoalRevision),
		"--expected-accounting-revision", fmt.Sprint(request.AccountingRevision)}
	command := exec.Command(binary, args...)
	command.Dir, command.Env = request.Root, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, launchErr := command.CombinedOutput()
	if launchErr != nil && command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitAdmissionRefused {
		reason := strings.TrimSpace(string(output))
		kind := "capacity"
		if strings.Contains(reason, "BATCH_MEMBER_BUDGET_REFUSED") {
			kind = "budget"
		} else if strings.Contains(reason, "GOAL_REVISION_MOVED") {
			kind = "revision"
		}
		return proofrun.TestResult{}, &batchProofAdmissionRefusal{kind: kind, reason: reason}
	}
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

type batchProofAdmissionRefusal struct{ kind, reason string }

func (refusal *batchProofAdmissionRefusal) Error() string { return refusal.reason }

func batchProofExitAccepted(status int, result proofrun.TestResult) bool {
	return status == 0 || status == proofrun.ExitReusableSuccess && result.Delivery.Sufficient
}

var batchBaseRearm = struct {
	fastForward func(context.Context, string, string) error
	rebuild     func(context.Context, string) error
	up          func(context.Context, string, string) (upOutcome, error)
}{landing.FastForwardPreservingRegisters, landedRearmRebuild, landedRearmUp}

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
	baseCommit := head
	if headTree != baseTree {
		command := exec.Command("git", "-C", root, "log", "--first-parent", "--format=%H %T", "origin/main")
		command.Env = gittree.ScrubbedEnviron()
		output, err := command.Output()
		if err != nil {
			return err
		}
		baseCommit = ""
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			fields := strings.Fields(line)
			if len(fields) == 2 && fields[1] == baseTree {
				baseCommit = fields[0]
				break
			}
		}
		if baseCommit == "" {
			return fmt.Errorf("BATCH_BASE_MOVED: origin/main ancestry has no commit for recorded base tree %s", baseTree)
		}
		if err := batchBaseRearm.fastForward(context.Background(), root, baseCommit); err != nil {
			return err
		}
	}
	if err := batchBaseRearm.rebuild(context.Background(), root); err != nil {
		return err
	}
	_, err = batchBaseRearm.up(context.Background(), root, root)
	return err
}
