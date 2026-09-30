package batchowner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

type BatchProofLaunch struct {
	Root, BatchID, GoalID, Tree, CandidateTip, ResultPath string
	Token                                                 string // the plan this run's completion binds to
	FreshEpisode, FreshExpiresAt                          string
	Mode                                                  testpolicy.Mode
	Groups                                                []string
	GoalRevision, AccountingRevision                      uint64
	// Early is a waiting batch's early proof (D14, R27): launched as the tip
	// proof is, but nobody's tip, so it reserves no diagnostic headroom.
	Early bool
	// RetryDecision is the accountable decision to re-execute what an early
	// proof of this very tree failed (U3-03); empty when there is none.
	RetryDecision string
}

type BatchProofDependencies struct {
	Base          func(string) (string, error)
	Rearm         func(string, string) error
	Seal          func(string, string, string, string, time.Time) error
	Plan          func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error)
	Launch        func(BatchProofLaunch) (proofrun.TestResult, error)
	freshDecision func(string, []batch.Unit, string) (batch.PrefixDecision, error)
	Sources       func(string, batch.Record) (map[string]string, error)
	// Attempts reads the retained proof store for the tip's retry decision;
	// nil reads it through batchTipRetryAttempts.
	Attempts func(string) ([]proofrun.Attempt, error)
	// LaneAccount resolves the lane a batch of changes is charged to; nil
	// reads the host's lane record (U11b).
	LaneAccount func(string) (string, error)
}

var ProductionBatchProofDependencies = BatchProofDependencies{
	Base:  fetchBatchTree,
	Rearm: RearmBatchBase,
	Seal: func(root, id, actor, baseTree string, at time.Time) error {
		return batch.SealWithForecast(batch.NewStore(root, nil), id, baseTree, actor, at, productionJoinPlan,
			func(candidate batch.Record) (batch.CostForecast, error) {
				now, err := fixtureauth.GoalNow(batch.ModuleRoot(root))
				if err != nil {
					return batch.CostForecast{}, err
				}
				return forecastBatchCost(root, candidate, nil, now)
			})
	},
	Plan: func(root, goalID, tree string, mode testpolicy.Mode) (testpolicy.Plan, error) {
		return productionBatchPlan(root, goalID, tree, mode)
	},
	Launch:        LaunchBatchTipProof,
	freshDecision: ProductionPrefixDecision,
	Sources:       BatchRetainedSources,
}

// BatchRetainedSources runs the retained verifier once on the batch tip tree
// (D4) and answers, per selected group, the attempt that holds its pass.
func BatchRetainedSources(root string, record batch.Record) (map[string]string, error) {
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 || record.Proof == nil {
		return nil, fmt.Errorf("batch %s has no joined units or test run to resolve", record.BatchID)
	}
	detached, err := openBatchSourcesWorktree(root, record.BatchID, record.TipTree)
	if err != nil {
		return nil, err
	}
	defer detached.Close()
	head := joined[len(joined)-1]
	charge, err := batchChargeID(root, batch.ChargeUnit(joined), nil)
	if err != nil {
		return nil, err
	}
	episode := record.PrefixEpisodes[head.GoalID]
	result, err := Engine.VerifyRetainedTesting(testrun.SelectionRequest{Root: batch.ModuleRoot(detached.Workspace().Dir), ControlRoot: batch.ModuleRoot(root),
		GoalID: charge, Tree: record.TipTree, Mode: testpolicy.ModeAuto, Purpose: testpolicy.PurposeDelivery,
		BatchRequirements: slices.Clone(record.Proof.SelectedGroups), BatchPrefixReceipt: true, FreshEpisode: episode.Token, FreshExpiresAt: episode.ExpiresAt})
	return BatchSourcesFromVerification(result), err
}

// batchSourcesRoot is where one lane's retained-verification worktrees live:
// a directory of the host's temporary root (diskstore.HostShared, never
// TMPDIR) named for the lane's control root, so a later owner of the same
// lane finds what a killed one left.
func batchSourcesRoot(root string) (string, error) {
	control, err := filepath.EvalSymlinks(batch.ModuleRoot(root))
	if err != nil {
		return "", err
	}
	// A shared host path, which no owner's TMPDIR moves: a later owner of
	// the lane runs with its own process scratch (Part B U1b-2).
	sum := sha256.Sum256([]byte(control))
	shared, err := diskstore.HostShared("metasystem-batch-sources-" + hex.EncodeToString(sum[:6]))
	if err != nil {
		return "", err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(shared))
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(shared)), nil
}

// batchSourcesTuple is the exact recorded worktree of one batch's retained
// verification: <sources root>/<id>/sources-<id>, linked to the lane.
func batchSourcesTuple(root, id string) (gittree.WorktreeTuple, error) {
	if !batch.ValidID(id) {
		return gittree.WorktreeTuple{}, fmt.Errorf("batch id %q is not a lowercase ULID", id)
	}
	base, err := batchSourcesRoot(root)
	if err != nil {
		return gittree.WorktreeTuple{}, err
	}
	top, err := (gittree.Workspace{Dir: root}).TopLevel()
	if err != nil {
		return gittree.WorktreeTuple{}, err
	}
	common, err := GitOutput(root, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err == nil {
		common, err = filepath.EvalSymlinks(common)
	}
	if err != nil {
		return gittree.WorktreeTuple{}, err
	}
	parent := filepath.Join(base, id)
	return gittree.WorktreeTuple{Parent: parent, Top: filepath.Join(parent, "sources-"+id), Control: top, Common: common}, nil
}

// openBatchSourcesWorktree projects tree for one batch's retained
// verification at its recorded path, first removing what an owner killed
// mid-verification left there.
func openBatchSourcesWorktree(root, id, tree string) (*gittree.DetachedWorktree, error) {
	tuple, err := batchSourcesTuple(root, id)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Lstat(tuple.Parent); statErr == nil {
		if outcome, removeErr := gittree.RemoveRecordedWorktree(tuple, gittree.Workspace{Dir: root}); outcome != gittree.WorktreeRemoved {
			return nil, fmt.Errorf("retained verification worktree of batch %s left by an earlier run is %s: %v", id, outcome, removeErr)
		}
	}
	if err := os.MkdirAll(filepath.Dir(tuple.Parent), 0o700); err != nil {
		return nil, err
	}
	plan, err := (gittree.Workspace{Dir: root}).PlanDetachedWorktreeAt(tuple.Parent, filepath.Base(tuple.Top))
	if err != nil {
		return nil, err
	}
	return plan.Create(tree)
}

// sweepBatchSourcesWorktrees removes every retained-verification worktree an
// earlier owner of this lane left behind; the owner runs it once at start,
// while it holds the lane, so none of them is in use.
func sweepBatchSourcesWorktrees(root string) error {
	base, err := batchSourcesRoot(root)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(base)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if !entry.IsDir() || !batch.ValidID(entry.Name()) {
			continue
		}
		tuple, err := batchSourcesTuple(root, entry.Name())
		if err == nil {
			var outcome string
			if outcome, err = gittree.RemoveRecordedWorktree(tuple, gittree.Workspace{Dir: root}); err == nil && outcome != gittree.WorktreeRemoved {
				err = fmt.Errorf("retained verification worktree %s is %s", tuple.Top, outcome)
			}
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// BatchSourcesFromVerification is the verifier's per-group source: a group
// the verification resolved to a passing attempt, and nothing else.
func BatchSourcesFromVerification(result proofrun.TestResult) map[string]string {
	sources := map[string]string{}
	for _, group := range result.Groups {
		if group.Status == "reused" && group.ReuseAttempt != "" {
			sources[group.ID] = group.ReuseAttempt
		}
	}
	return sources
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

func ExecuteBatchProof(root, id, actor, window, token string, sample proofrun.LoadSample, at time.Time, dependencies BatchProofDependencies) error {
	controlRoot := batch.ModuleRoot(root)
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if len(record.Units) == 0 {
		return fmt.Errorf("%s: batch %s has no joined units", codeProofStateRefused, id)
	}
	if record.State == batch.StateSealed && record.Proof != nil && record.Proof.Status == "union-uncovered" && record.Proof.Tree == record.TipTree {
		return nil
	}
	baseTree := record.BaseTree
	if record.State == batch.StateOpen && dependencies.Base != nil {
		baseTree, err = dependencies.Base(root)
		if err != nil {
			return err
		}
	}
	if err := dependencies.Rearm(root, baseTree); err != nil {
		return err
	}
	if record.State == batch.StateOpen {
		if dependencies.Seal == nil {
			return fmt.Errorf("%s: the test run's seal step is missing", codeProofStateRefused)
		}
		if err := dependencies.Seal(root, id, actor, baseTree, at); err != nil {
			if laneHold(err) {
				return batch.RecordHold(store, id, actor, err.Error(), at)
			}
			return err
		}
		record, err = store.Load(id)
		if err != nil {
			return err
		}
	}
	if record.State != batch.StateSealed {
		return fmt.Errorf("%s: batch %s is not sealed", codeProofStateRefused, id)
	}
	if record.CostForecast != nil {
		if !record.CostForecast.Matches(record) {
			return fmt.Errorf("%s: sealed cost snapshot no longer binds the batch", codeCostInputMoved)
		}
		for _, budget := range record.CostForecast.Budgets {
			if budget.Fits {
				continue
			}
			if budget.Status != "KNOWN" {
				return &batch.PrefixAdmissionRefusal{Code: "BATCH_MEMBER_BUDGET_UNKNOWN", Reason: "BATCH_COST_HEADROOM_REFUSED: " + budget.GoalID + " " + budget.Reason}
			}
			cause := &batch.PrefixBudgetRefusal{Reason: "BATCH_COST_HEADROOM_REFUSED: " + budget.GoalID + " " + budget.Reason}
			if err := batch.HandlePrefixMemberRefusal(store, id, budget.GoalID, actor, at, cause); err != nil {
				return err
			}
			return cause
		}
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 {
		return fmt.Errorf("%s: batch %s has no joined units", codeProofStateRefused, id)
	}
	if (record.Landing == nil || record.Landing.Base != record.BaseTree) && slices.ContainsFunc(joined, func(unit batch.Unit) bool { return len(unit.Builds) != 0 }) {
		expected := ""
		if progress := record.Landing; progress != nil {
			if progress.ReceiptTip != "" || progress.HeldChecked || progress.PushComplete || progress.PushedTip != "" ||
				progress.RearmComplete || progress.CleanupDone || len(progress.Commits) != 0 || len(progress.BuildCommits) != 0 ||
				progress.RefusedOrigin != "" || progress.RefusedBase != "" || progress.PushRounds != 0 || progress.PushRejection != nil {
				return fmt.Errorf("%s: stale landing candidate has publication or receipt progress", codeProofStateRefused)
			}
			expected = progress.CandidateTip
			if expected == "" {
				expected = progress.BranchTip
			}
			if expected == "" {
				return fmt.Errorf("%s: stale landing candidate has no branch tip", codeProofStateRefused)
			}
		}
		tip, branchErr := batch.RebuildLandingBranch(root, id, record.BaseTree, expected, actor, joined)
		if branchErr != nil {
			return branchErr
		}
		candidateTree, treeErr := GitOutput(root, "rev-parse", tip+"^{tree}")
		if treeErr != nil || candidateTree != record.TipTree {
			return fmt.Errorf("%s: rebuilt landing candidate %s has tree %s, want %s: %v", codeProofTreeMoved, tip, candidateTree, record.TipTree, treeErr)
		}
		if err := store.Update(id, func(current *batch.Record) error {
			if !reflect.DeepEqual(*current, record) {
				return fmt.Errorf("%s: sealed batch changed during landing branch rebuild", codeProofStateMoved)
			}
			current.Landing = &batch.LandingProgress{Base: current.BaseTree, BranchTip: tip, CandidateTip: tip}
			return nil
		}); err != nil {
			return err
		}
		record, err = store.Load(id)
		if err != nil {
			return err
		}
	}
	// The tip's keys are its last member's; its proof is charged to the last
	// goal member, or, when every member is a change, to the lane (U11b).
	head := joined[len(joined)-1]
	charge := batch.ChargeUnit(joined)
	chargeID, chargeErr := batchChargeID(root, charge, dependencies.LaneAccount)
	if chargeErr != nil {
		// Fail closed: nothing launches, and the batch holds on the sealed
		// edge with the plain reason until the lane can be named.
		if _, err := batch.RequireProofPlan(store, id, actor, window, token, sample, testpolicy.Plan{}, at); err != nil {
			return errors.Join(chargeErr, err)
		}
		return batch.RefuseProofAdmission(store, id, actor, token, "lane-unresolved",
			"the batch's changes are charged to the landing lane, whose identity cannot be resolved: "+chargeErr.Error()+"; the batch holds until it can", at)
	}
	plan, err := PlanBatchMemberUnion(root, record.TipTree, joined, chargeID, testpolicy.ModeAuto, dependencies.Plan)
	if err != nil {
		if laneHold(err) {
			return batch.RecordHold(store, id, actor, err.Error(), at)
		}
		return err
	}
	if plan.RequiredMode == testpolicy.ModeDeep || !proofPlanCovers(plan, record.SelectedGroups) {
		plan, err = PlanBatchMemberUnion(root, record.TipTree, joined, chargeID, testpolicy.ModeDeep, dependencies.Plan)
		if err != nil {
			return err
		}
	}
	if !proofPlanCovers(plan, record.SelectedGroups) {
		reason := "the tests chosen for the batch miss some of its changes; nothing to do, its owner chooses again"
		if err := batch.RecordUnionRefusal(store, id, actor, reason, at); err != nil {
			return err
		}
		return errors.New(reason)
	}
	admitted, err := batch.RequireProofPlan(store, id, actor, window, token, sample, plan, at)
	if err != nil {
		return err
	}
	resultPath := batchProofResultPath(controlRoot, id)
	sealed := admitted.Seal[charge.GoalID]
	request := BatchProofLaunch{Root: controlRoot, BatchID: id, GoalID: chargeID, Tree: admitted.TipTree, CandidateTip: admitted.Proof.CandidateTip, ResultPath: resultPath, Token: token,
		Mode: plan.ExecutedMode, Groups: slices.Clone(plan.SelectedGroups), GoalRevision: sealed.Revision, AccountingRevision: sealed.AccountingRevision}
	attempts := BatchTipRetryAttempts
	if dependencies.Attempts != nil {
		attempts = dependencies.Attempts
	}
	if request.RetryDecision, err = TipRetryDecision(controlRoot, admitted, chargeID, attempts); err != nil {
		return err
	}
	if dependencies.freshDecision != nil {
		joined := make([]batch.Unit, 0, len(admitted.Units))
		for _, unit := range admitted.Units {
			if unit.State == batch.UnitJoined {
				joined = append(joined, unit)
			}
		}
		decision, decisionErr := dependencies.freshDecision(root, joined, admitted.TipTree)
		if decisionErr != nil {
			return decisionErr
		}
		if decision.FreshRequired {
			decision.Groups = append(decision.Groups, admitted.SelectedGroups...)
			slices.Sort(decision.Groups)
			decision.Groups = slices.Compact(decision.Groups)
			decisionID, idErr := batch.PrefixDecisionID(admitted.BaseTree, admitted.TipTree, joined, admitted.Seal, decision)
			if idErr != nil {
				return idErr
			}
			episode, episodeErr := batch.EnsureDecisionEpisode(store, id, admitted, head.GoalID, len(joined)-1, admitted.TipTree, decisionID, decision.FreshMaxAgeMS, at)
			if episodeErr != nil {
				return episodeErr
			}
			request.FreshEpisode, request.FreshExpiresAt = episode.Token, episode.ExpiresAt
		}
	}
	result, launchErr := dependencies.Launch(request)
	var refused *BatchProofAdmissionRefusal
	if errors.As(launchErr, &refused) {
		switch refused.Kind {
		case "budget":
			return batch.WithdrawBudgetMember(store, id, charge.GoalID, actor, refused.Error(), at)
		case "revision", "fenced":
			return batch.ReassembleSurvivorsWithReturns(store, id, actor, at,
				[]batch.ReturnDecision{{GoalID: charge.GoalID, Outcome: batch.UnitEjected, Reason: refused.Error()}})
		default:
			return batch.RefuseProofAdmission(store, id, actor, token, "admission-refused", refused.Error(), at)
		}
	}
	if finishErr := batch.FinishProof(store, id, actor, token, result, launchErr, at); finishErr != nil {
		return finishErr
	}
	if launchErr == nil && dependencies.Sources != nil {
		return batch.RecordSources(store, id, actor, at, func(record batch.Record) (map[string]string, error) { return dependencies.Sources(root, record) })
	}
	return launchErr
}

// BatchTipProofArgs is the tip (or early) proof's argv up to its optional
// flags: charged to a goal at its sealed revisions with the diagnostic
// headroom reserved, or to the lane, which has neither (U11b).
func BatchTipProofArgs(request BatchProofLaunch, executionRoot string) []string {
	args := append(append([]string{"internal", "test", "run", "--root", executionRoot, "--control-root", request.Root, "--batch-tip"},
		accountFlag(request.GoalID)...), "--tree", request.Tree, "--mode", string(request.Mode), "--purpose", "delivery", "--result", request.ResultPath, "--json")
	if !request.Early && !lane.IsAccount(request.GoalID) {
		args = append(args, "--require-diagnostic-headroom")
	}
	return append(args, accountRevisions(request.GoalID, batch.Claim{Revision: request.GoalRevision, AccountingRevision: request.AccountingRevision})...)
}

func PlanBatchMemberUnion(root, tree string, units []batch.Unit, charge string, mode testpolicy.Mode, plan func(string, string, string, testpolicy.Mode) (testpolicy.Plan, error)) (testpolicy.Plan, error) {
	union := testpolicy.Plan{RequestedMode: mode, ExecutedMode: testpolicy.ModeStandard, RequiredMode: testpolicy.ModeStandard}
	planned := []batch.Unit{}
	for _, unit := range units {
		// A change is planned by no goal: the goal members' plans on the tip
		// tree hold its paths; a batch of changes is planned by the lane.
		if !unit.IsChange() {
			planned = append(planned, unit)
		}
	}
	if len(planned) == 0 {
		planned = []batch.Unit{{GoalID: charge}}
	}
	for _, unit := range planned {
		member, err := plan(root, unit.GoalID, tree, mode)
		if err != nil {
			return testpolicy.Plan{}, err
		}
		union.SelectedGroups = append(union.SelectedGroups, member.SelectedGroups...)
		union.SelectedGroups = append(union.SelectedGroups, unit.SelectedGroups...)
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
	return ProductionBatchTreePlan(root, goalID, tree, mode)
}

var BatchTipProofExecutable = os.Executable

func LaunchBatchTipProof(request BatchProofLaunch) (proofrun.TestResult, error) {
	return LaunchBatchTipProofWithDependencies(request, BatchExecutionDependencies{
		Executable: BatchTipProofExecutable,
		Checkout:   batchDetachedCheckout,
		TopLevel: func(root string) (string, error) {
			return (gittree.Workspace{Dir: root}).TopLevel()
		},
		ReadGit: GitOutput,
	})
}

func LaunchBatchTipProofWithDependencies(request BatchProofLaunch, dependencies BatchExecutionDependencies) (proofrun.TestResult, error) {
	if request.CandidateTip != "" {
		candidateTree, treeErr := dependencies.ReadGit(request.Root, "rev-parse", request.CandidateTip+"^{tree}")
		if treeErr != nil {
			return proofrun.TestResult{}, fmt.Errorf("resolve the batch's tested tip %s: %w", request.CandidateTip, treeErr)
		}
		if candidateTree != request.Tree {
			return proofrun.TestResult{}, fmt.Errorf("the batch's tested tip %s has tree %s, want %s", request.CandidateTip, candidateTree, request.Tree)
		}
	}
	binary, err := dependencies.Executable()
	if err != nil {
		return proofrun.TestResult{}, err
	}
	// A delivery run proves the exact index it is handed and refuses to move a
	// checkout out from under a named tree, so the caller owns the positioning.
	// The control root stays on the batch base, because the base is what arms
	// the engine that judges. The tip is therefore projected into its own
	// detached worktree, whose index git itself verifies against the tree, and
	// the run is pointed back at the control root for every durable write.
	projectRoot, err := dependencies.TopLevel(request.Root)
	if err != nil {
		return proofrun.TestResult{}, err
	}
	detachedRoot, closeDetached, err := dependencies.Checkout(projectRoot, request.Tree)
	if err != nil {
		return proofrun.TestResult{}, fmt.Errorf("project batch tip %s: %w", request.Tree, err)
	}
	defer closeDetached()
	executionRoot := batch.ModuleRoot(detachedRoot)
	args := BatchTipProofArgs(request, executionRoot)
	if request.RetryDecision != "" {
		args = append(args, "--retry-decision", request.RetryDecision)
	}
	if len(request.Groups) != 0 {
		args = append(args, "--batch-prefix", "--batch-requirements", testrun.BatchRequirementsArgument(request.Groups))
	}
	if request.FreshEpisode != "" {
		args = append(args, "--fresh-episode", request.FreshEpisode)
		if request.FreshExpiresAt != "" {
			args = append(args, "--fresh-expires-at", request.FreshExpiresAt)
		}
	}
	// The early proof runs on spare capacity and launches without the host's
	// proving flock (U12): speculative work never delays a real proof. The
	// tip proof holds it for the child's life.
	command := BatchProofCommand(binary, args, request.Early)
	command.Dir, command.Env = executionRoot, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+LandingOwnerLineage)
	child, err := runTestRunChild(command)
	if err != nil {
		return proofrun.TestResult{}, fmt.Errorf("the batch tip's test run: %w", err)
	}
	if child.Outcome == verbresult.Refused {
		return proofrun.TestResult{}, &BatchProofAdmissionRefusal{Kind: string(classifyAdmission(child)), Reason: child.Err().Error()}
	}
	var result proofrun.TestResult
	readErr := strictjson.Read(request.ResultPath, &result)
	if readErr == nil && BatchProofOutcomeAccepted(child, result) {
		return result, nil
	}
	if child.Outcome == verbresult.Confirmed {
		return result, fmt.Errorf("the batch tip's test run: %w", readErr)
	}
	return result, fmt.Errorf("the batch tip's test run: %w", child.Err())
}

type BatchProofAdmissionRefusal struct{ Kind, Reason string }

func (refusal *BatchProofAdmissionRefusal) Error() string { return refusal.Reason }

// BatchProofOutcomeAccepted is whether a test run child's result stands as
// a pass: it confirmed, or it reused an earlier success whose evidence is
// sufficient.
func BatchProofOutcomeAccepted(child verbresult.Result, result proofrun.TestResult) bool {
	return child.Outcome == verbresult.Confirmed || child.Outcome == verbresult.Unchanged && result.Delivery.Sufficient
}

var BatchBaseRearm = struct {
	FastForward func(context.Context, string, string) error
	Rebuild     func(context.Context, string) error
	Up          func(context.Context, string, string) (testrun.UpOutcome, error)
}{landing.FastForwardPreservingRegisters, testrun.RebuildLandedEngine, OwnerUpLandedEngine}

// OwnerUpLandedEngine is the landing owner's run of the rebuilt engine's
// ordinary up. The owner is machinery, not a session, so that up re-arms a
// landed rebuild (its authority is the landed bytes, never the caller) and
// then ends with a non-zero exit at its session step, before the supervision
// step. The owner never reads up's words: it judges the run by the state it
// must leave. The enrolled engine is current, and the lane's supervision is
// armed, starting the missing rings itself when they are down.
func OwnerUpLandedEngine(ctx context.Context, installation, projectRoot string) (testrun.UpOutcome, error) {
	return ownerUpLandedEngineWith(ctx, installation, projectRoot, testrun.UpLandedEngine, enrolledEngineCurrent,
		func(root string) (bool, error) { return supervisionArmedOrRecovered(ctx, root) })
}

func ownerUpLandedEngineWith(ctx context.Context, installation, projectRoot string, up func(context.Context, string, string) (testrun.UpOutcome, error),
	current func(string) error, armed func(string) (bool, error)) (testrun.UpOutcome, error) {
	outcome, upErr := up(ctx, installation, projectRoot)
	if upErr == nil {
		return outcome, nil
	}
	if err := current(installation); err != nil {
		return outcome, fmt.Errorf("%w; the lane's engine is not re-armed: %v", upErr, err)
	}
	running, err := armed(installation)
	if err != nil {
		return outcome, fmt.Errorf("%w; whether the lane's supervision is armed is unknown: %v", upErr, err)
	}
	if !running {
		return outcome, fmt.Errorf("%w; the lane's supervision is not armed after the re-arm", upErr)
	}
	return outcome, nil
}

// enrolledEngineCurrent says whether the installation's enrolled engine is
// the one on disk: a rebuild the re-arm did not enroll reads as not current.
func enrolledEngineCurrent(installation string) error {
	enrolled, err := steward.OpenEnrolledBinary(installation)
	if err != nil {
		return err
	}
	return enrolled.Close()
}

// supervisionArmedOrRecovered reads whether the lane's supervision runs and,
// when it does not, starts the missing rings the way the scheduler does (up's
// recovery, which never re-arms) and reads again. Only up's exit is read.
func supervisionArmedOrRecovered(ctx context.Context, installation string) (bool, error) {
	if running, err := LandingLaneArmed(installation); err != nil || running {
		return running, err
	}
	command := exec.CommandContext(ctx, filepath.Join(installation, "bin", "metasystem"), "up", "--recover-only", "--if-down",
		"--repo", installation, "--metasystem-root", installation)
	command.Dir, command.Env = installation, os.Environ()
	_ = command.Run()
	return LandingLaneArmed(installation)
}

// laneCheckout serializes every step that moves the one lane checkout
// (settings.Root) across the owner's concurrent runs: a landing (branch prep,
// apply, commit, reset, push recovery, the re-arm at the pushed tip) and a
// proof start's re-arm at its base. Proofs still overlap: each runs in its own
// detached worktree, and a re-arm holds the checkout only while it re-arms.
type laneCheckout struct {
	slot    chan struct{}
	Waiting atomic.Int32
}

var laneCheckouts sync.Map // cleaned root -> *laneCheckout

func LaneCheckoutFor(root string) *laneCheckout {
	value, _ := laneCheckouts.LoadOrStore(filepath.Clean(root), &laneCheckout{slot: make(chan struct{}, 1)})
	return value.(*laneCheckout)
}

// WithLaneCheckout runs one checkout-moving step with the checkout held.
func WithLaneCheckout(root string, run func() error) error {
	checkout := LaneCheckoutFor(root)
	checkout.Waiting.Add(1)
	checkout.slot <- struct{}{}
	checkout.Waiting.Add(-1)
	defer func() { <-checkout.slot }()
	return run()
}

func (checkout *laneCheckout) Held() bool { return len(checkout.slot) == 1 }

// BatchRearmEdges are the control root's Git reads and the re-arm steps.
type BatchRearmEdges struct {
	Head        func(root string) (commit, tree string, err error)
	BaseCommit  func(root, tree string) (string, error)
	Descends    func(root, descendant, ancestor string) (bool, error)
	FastForward func(context.Context, string, string) error
	Rebuild     func(context.Context, string) error
	Up          func(context.Context, string, string) (testrun.UpOutcome, error)
}

func RearmBatchBase(root, baseTree string) error {
	return RearmBatchBaseWith(root, baseTree, BatchRearmEdges{
		Head: func(root string) (string, string, error) {
			workspace := gittree.Workspace{Dir: root}
			head, unborn, err := workspace.HeadCommit()
			if err != nil || unborn {
				return "", "", fmt.Errorf("re-arm batch base: committed HEAD required")
			}
			tree, err := workspace.TreeOf(head)
			return head, tree, err
		},
		BaseCommit: func(root, tree string) (string, error) {
			commit, err := commitForTree(root, "origin/main", tree)
			if err != nil {
				return "", fmt.Errorf("%s: origin/main ancestry has no commit for recorded base tree %s", codeBaseMoved, tree)
			}
			return commit, nil
		},
		Descends: func(root, descendant, ancestor string) (bool, error) {
			command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", ancestor, descendant)
			command.Env = gittree.ScrubbedEnviron()
			err := command.Run()
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == 1 {
				return false, nil
			}
			return err == nil, err
		},
		FastForward: func(ctx context.Context, root, commit string) error {
			return BatchBaseRearm.FastForward(ctx, root, commit)
		},
		Rebuild: func(ctx context.Context, root string) error { return BatchBaseRearm.Rebuild(ctx, root) },
		Up: func(ctx context.Context, root, control string) (testrun.UpOutcome, error) {
			return BatchBaseRearm.Up(ctx, root, control)
		},
	})
}

// RearmBatchBaseWith arms the control root at a batch's base, one run at a
// time, and never moves it back: a root already past the base (a later
// landing moved it) stays, since the tip proof runs in its own detached
// worktree at the batch tree.
func RearmBatchBaseWith(root, baseTree string, edges BatchRearmEdges) error {
	return WithLaneCheckout(root, func() error { return rearmBatchBaseHeld(root, baseTree, edges) })
}

func rearmBatchBaseHeld(root, baseTree string, edges BatchRearmEdges) error {
	controlRoot := batch.ModuleRoot(root)
	head, headTree, err := edges.Head(root)
	if err != nil {
		return err
	}
	if headTree != baseTree {
		baseCommit, err := edges.BaseCommit(root, baseTree)
		if err != nil {
			return err
		}
		if past, err := edges.Descends(root, head, baseCommit); err != nil || past {
			return err
		}
		if err := edges.FastForward(context.Background(), controlRoot, baseCommit); err != nil {
			return err
		}
	}
	if err := edges.Rebuild(context.Background(), controlRoot); err != nil {
		return err
	}
	_, err = edges.Up(context.Background(), controlRoot, controlRoot)
	return err
}

func RearmBatchTip(root, tip string) error {
	if tip == "" {
		return fmt.Errorf("re-arm landed batch: pushed tip is absent")
	}
	controlRoot := batch.ModuleRoot(root)
	if err := BatchBaseRearm.FastForward(context.Background(), controlRoot, tip); err != nil {
		return err
	}
	if err := BatchBaseRearm.Rebuild(context.Background(), controlRoot); err != nil {
		return err
	}
	_, err := BatchBaseRearm.Up(context.Background(), controlRoot, controlRoot)
	return err
}
