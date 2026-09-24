package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

func executeBatchLanding(root, id, actor string, at time.Time) error {
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if reopened, err := reopenBatchBeforeReceiptsWhenProofBaseMoved(root, store, id, actor, at, record); err != nil || reopened {
		return err
	}
	if record.Landing == nil || !record.Landing.PushComplete {
		if err := batch.ComposePrefixReceipts(store, id, actor, at, batch.PrefixReceiptSeams{
			Authorize: func(unit batch.Unit, _ batch.Claim) error {
				return authorizeBatchMember(root, record, unit)
			},
			Plan: func(units []batch.Unit, tree string) (batch.PrefixDecision, error) {
				return productionPrefixDecision(root, units, tree)
			},
			ExecuteDecision: func(goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
				return executeBatchPrefixReceiptWithDecision(root, id, record, goalID, tree, decision)
			},
			Verify: func(unit batch.Unit, tree string, decision batch.PrefixDecision) error {
				return batchVerifyPrefixEvidence(root, unit, tree, decision)
			},
		}); err != nil {
			return err
		}
		record, err = store.Load(id)
		if err != nil {
			return err
		}
		if err := authorizeBatchSeries(root, store, record, actor, at); err != nil {
			return err
		}
		if err := verifyBatchSeries(root, record, record.PrefixTrees); err != nil {
			return err
		}
		baseCommit, err := commitForTree(root, "origin/main", record.BaseTree)
		if err != nil {
			return err
		}
		seams := batchLandSeams(root, id, record, baseCommit, actor)
		if err := batch.LandSeries(store, id, actor, at, seams); err != nil {
			return err
		}
	}
	return finishBatchLanding(root, store, id, actor, at)
}

func reopenBatchBeforeReceiptsWhenProofBaseMoved(root string, store batch.Store, id, actor string, at time.Time, record batch.Record) (bool, error) {
	if record.Proof == nil || record.Proof.BaseCommit == "" {
		return false, nil
	}
	origin, originTree, err := batchLandFetchOrigin(root)
	if err != nil || origin == record.Proof.BaseCommit {
		return false, err
	}
	changed, err := (gittree.Workspace{Dir: root}).ChangedPaths(record.BaseTree, originTree)
	if err != nil {
		return false, err
	}
	prefix, err := (gittree.Workspace{Dir: root}).Prefix()
	if err != nil {
		return false, err
	}
	if !batchProofInputsMoved(record, changed, prefix) {
		return false, nil
	}
	tip := ""
	if record.Landing != nil {
		tip = record.Landing.CandidateTip
		if tip == "" {
			tip = record.Landing.BranchTip
		}
	}
	if err := batchLandAbandon(root, id, tip, origin); err != nil {
		return false, err
	}
	return true, batch.ReopenMovedTrunk(store, id, originTree, actor, at)
}

var batchLandFetchOrigin = fetchBatchOrigin
var batchLandAbandon = batch.AbandonLandingBranch
var batchLandOriginTree = func(root, commit string) (string, error) {
	return gitOutput(root, "rev-parse", commit+"^{tree}")
}
var batchLandRecoverPush = recoverMovedBatchPush

func batchLandSeams(root, id string, record batch.Record, baseCommit, actor string) batch.LandSeams {
	return batchLandSeamsWithRead(root, id, record, baseCommit, actor, gitOutput)
}

func batchLandSeamsWithRead(root, id string, record batch.Record, baseCommit, actor string, readGit func(root string, args ...string) (string, error)) batch.LandSeams {
	controlRoot := batch.ModuleRoot(root)
	return batch.LandSeams{
		Prepare: func(_ string) error { return batch.PrepareLandingBranch(root, id, baseCommit) },
		Apply:   func(unit batch.Unit) error { return batch.ApplyCertifiedPatch(root, root, unit.Chain) },
		AppendReceipt: func(unit batch.Unit, receipt batch.PrefixReceipt) error {
			return batch.RunCommand(batch.CommandSpec{Dir: controlRoot, Name: filepath.Join(controlRoot, "scripts", "receipt.sh"), Args: []string{"add", "--type", "implement", "--outcome", "shipped", "--goal", unit.GoalID, "--built-by", "coordinator", "--note", "batch " + id + " prefix " + receipt.Tree}})
		},
		Commit: func(unit batch.Unit, receipt batch.PrefixReceipt) (string, error) {
			path := receipt.ResultPath
			if path == "" {
				path = filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+".json")
			}
			message := fmt.Sprintf("land %s in batch %s\n\nOriginal join order; prefix tree %s.\n", unit.GoalID, id, receipt.Tree)
			if err := batch.CommitWithWrapperWithRead(controlRoot, batch.ChainDeclaration(unit.Chain), unit.GoalID, path, message, unit.AuthorName, unit.AuthorEmail, actor, readGit); err != nil {
				return "", err
			}
			return readGit(root, "rev-parse", "HEAD")
		},
		ApplyBuild: func(_ batch.Unit, build batch.BranchBuild) error {
			return batch.ApplyBranchBuild(root, root, build)
		},
		AppendBuildReceipt: func(unit batch.Unit, build batch.BranchBuild, receipt batch.PrefixReceipt) error {
			return batch.RunCommand(batch.CommandSpec{Dir: controlRoot, Name: filepath.Join(controlRoot, "scripts", "receipt.sh"), Args: []string{"add", "--type", "implement", "--outcome", "shipped", "--goal", unit.GoalID, "--built-by", "coordinator", "--note", "batch " + id + " unit " + strings.Join(build.Units, "+") + " prefix " + receipt.Tree}})
		},
		CommitBuild: func(unit batch.Unit, build batch.BranchBuild, receipt batch.PrefixReceipt) (string, error) {
			path := receipt.ResultPath
			if path == "" {
				path = filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+".json")
			}
			last := unit.GoalLast && len(unit.CommitIDs) != 0 && build.Commit == unit.CommitIDs[len(unit.CommitIDs)-1]
			declaration := batch.AttestedDeclaration(build.Commit, unit.BranchTip, baseCommit)
			if err := batch.CommitWithWrapper(controlRoot, declaration, unit.GoalID, path, batch.BranchLandingMessage(unit.GoalID, build, last), unit.AuthorName, unit.AuthorEmail, actor); err != nil {
				return "", err
			}
			commit, err := gitOutput(root, "rev-parse", "HEAD")
			if err != nil {
				return "", err
			}
			if err := batch.RequirePassingCommitVerdict(root, unit.GoalID, commit); err != nil {
				return "", err
			}
			return commit, nil
		},
		Held: func(_, tip string) error {
			return batchChildRunner(controlRoot, landingOwnerLineage, "landing", "held", "--root", controlRoot, "--base", baseCommit, "--commit", tip, "--remote", "origin", "--ref", "refs/heads/main")
		},
		VerifySeries: func(units []batch.Unit, commits map[string]string) error {
			if err := authorizeBatchSeries(root, batch.NewStore(root, nil), record, actor, time.Now().UTC()); err != nil {
				return err
			}
			return verifyBatchCommittedSeries(root, record, units, commits)
		},
		PublishBranch: func(expected, tip string) error {
			return batch.PublishLandingBranch(root, id, expected, tip)
		},
		Push: func(_, tip string) error {
			return batch.LandLandingBranch(root, id, baseCommit, tip)
		},
		Origin: func() (string, error) {
			commit, _, err := batchLandFetchOrigin(root)
			return commit, err
		},
		OriginTree: func(commit string) (string, error) {
			return batchLandOriginTree(root, commit)
		},
		Abandon:        func(tip, detachAt string) error { return batchLandAbandon(root, id, tip, detachAt) },
		SeriesOnOrigin: func(origin, tip string) (bool, error) { return batchSeriesOnEndpoint(root, origin, tip) },
		LeaseBase:      baseCommit,
		RecoverPush: func(origin, baseTree, tip string) (batch.PushRecovery, error) {
			return batchLandRecoverPush(root, id, record, actor, baseCommit, origin, baseTree, tip)
		},
		Reset: func(_ string) error {
			command := exec.Command("git", "-C", root, "reset", "--hard", baseCommit)
			command.Env = gittree.ScrubbedEnviron()
			return command.Run()
		},
		Cleanup: func() error {
			return batch.CleanupLandingBranch(root, id, gitHead(root))
		},
	}
}

func finishBatchLanding(root string, store batch.Store, id, actor string, at time.Time) error {
	landed, err := store.Load(id)
	if err != nil {
		return err
	}
	if landed.State == batch.StateOpen || landed.State == batch.StateDissolved {
		return nil
	}
	if landed.State == batch.StateLanding && landed.Landing != nil && !landed.Landing.PushComplete && landed.Landing.PushRejection != nil {
		return nil
	}
	return recoverBatchLanding(root, store, id, actor, at)
}

var batchMovedEndpointPush = batch.LandLandingBranch
var batchVerifyRebasedSeries = verifyBatchSeries
var batchAuthorizeRebasedSeries = authorizeBatchSeries
var batchGoalBranchSweep = goalbranch.Sweep
var batchRecoveryRearm = rearmBatchTip
var batchRecoveryGoalNext = func(root, goalID string, at time.Time) (string, error) {
	endpoint, err := goalBranchEndpoint(root)
	if err != nil {
		return "", err
	}
	projection, err := goal.Project(endpoint, true, at)
	if err != nil {
		return "", err
	}
	for _, goals := range []map[string]*goal.GoalFile{projection.Tree.Live, projection.Tree.Done, projection.Tree.Abandoned} {
		if file := goals[goalID]; file != nil {
			return file.NextStep, nil
		}
	}
	return "", fmt.Errorf("goal %s is absent from the current ledger", goalID)
}

func recoverMovedBatchPush(root, id string, record batch.Record, actor, expectedBase, originCommit, baseTree, tip string) (batch.PushRecovery, error) {
	return recoverMovedBatchPushWithInputs(root, id, record, actor, expectedBase, originCommit, baseTree, tip, gitOutput, func(cmd *exec.Cmd) error { return cmd.Run() })
}

func recoverMovedBatchPushWithInputs(root, id string, record batch.Record, actor, expectedBase, originCommit, baseTree, tip string,
	readGit func(root string, args ...string) (string, error), runGit func(*exec.Cmd) error) (batch.PushRecovery, error) {
	controlRoot := batch.ModuleRoot(root)
	originTree, err := readGit(root, "rev-parse", originCommit+"^{tree}")
	recovery := batch.PushRecovery{Origin: originCommit, BaseTree: originTree}
	if err != nil {
		return recovery, err
	}
	landed, err := batchSeriesOnEndpointWithRunner(root, originCommit, tip, runGit)
	if err != nil {
		return recovery, err
	}
	if landed {
		recovery.Tip, recovery.Pushed = tip, true
		return recovery, nil
	}
	if originCommit == expectedBase {
		return recovery, nil
	}
	changed, err := (gittree.Workspace{Dir: root}).ChangedPaths(baseTree, originTree)
	if err != nil {
		return recovery, err
	}
	prefix, err := (gittree.Workspace{Dir: root}).Prefix()
	if err != nil {
		return recovery, err
	}
	if batchProofInputsMoved(record, changed, prefix) {
		recovery.Reopen = true
		return recovery, nil
	}
	var output bytes.Buffer
	if err := landing.Advance(root, "refs/remotes/origin/main", &output, &output); err != nil {
		return reopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery,
			fmt.Errorf("rebase landing series: %w: %s", err, strings.TrimSpace(output.String())))
	}
	rebasedTip, err := gitOutput(root, "rev-parse", "HEAD")
	if err != nil {
		return recovery, err
	}
	if err := batchChildRunner(controlRoot, landingOwnerLineage, "landing", "held", "--root", controlRoot, "--base", originCommit, "--commit", rebasedTip, "--remote", "origin", "--ref", "refs/heads/main"); err != nil {
		return reopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, fmt.Errorf("rebased landing held: %w", err))
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 {
		return recovery, fmt.Errorf("rebased landing has no joined authority member")
	}
	finalTrees, err := rebasedPrefixTrees(root, originCommit, rebasedTip, joined)
	if err != nil {
		return reopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, err)
	}
	if err := batchVerifyRebasedSeries(root, record, finalTrees); err != nil {
		return reopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, fmt.Errorf("rebased prefix verification: %w", err))
	}
	// The first endpoint attempt's authority check cannot authorize this
	// retry: a member can be fenced or revised while the moved series is
	// rebased and reproved. A typed refusal returns that member and rebuilds
	// the survivors before either rebased publication or endpoint push.
	at, err := goalCommandNow(controlRoot)
	if err != nil {
		return recovery, err
	}
	if err := batchAuthorizeRebasedSeries(root, batch.NewStore(root, nil), record, actor, at); err != nil {
		return recovery, err
	}
	if err := batch.PublishLandingBranch(root, id, tip, rebasedTip); err != nil {
		return recovery, err
	}
	recovery.Tip = rebasedTip
	if err := batchMovedEndpointPush(root, id, originCommit, rebasedTip); err != nil {
		latest, latestTree, fetchErr := fetchBatchOrigin(root)
		if fetchErr == nil {
			recovery.Origin, recovery.BaseTree = latest, latestTree
		}
		return recovery, errors.Join(err, fetchErr)
	}
	recovery.Pushed = true
	return recovery, nil
}

func reopenMovedBatchAfterRecoveryFailure(_, _, _, _ string, recovery batch.PushRecovery, _ error) (batch.PushRecovery, error) {
	recovery.Reopen = true
	return recovery, nil
}

func batchSeriesOnEndpoint(root, origin, tip string) (bool, error) {
	return batchSeriesOnEndpointWithRunner(root, origin, tip, func(cmd *exec.Cmd) error { return cmd.Run() })
}

func batchSeriesOnEndpointWithRunner(root, origin, tip string, runGit func(*exec.Cmd) error) (bool, error) {
	if origin == "" || tip == "" {
		return false, nil
	}
	command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", tip, origin)
	command.Env = gittree.ScrubbedEnviron()
	err := runGit(command)
	if err == nil {
		return true, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() == 1 {
		return false, nil
	}
	return false, err
}
func fetchBatchOrigin(root string) (string, string, error) {
	command := exec.Command("git", "-C", root, "fetch", "origin", "+refs/heads/main:refs/remotes/origin/main")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("BATCH_LAND_PUSH_REFUSED: fetch origin/main: %s: %w", strings.TrimSpace(string(output)), err)
	}
	commit, err := gitOutput(root, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return "", "", err
	}
	tree, err := gitOutput(root, "rev-parse", commit+"^{tree}")
	return commit, tree, err
}

func batchProofInputsMoved(record batch.Record, changed []string, installationPrefix string) bool {
	if record.Proof == nil {
		return true
	}
	policy, err := behaviorsurface.Load()
	if err != nil {
		return true
	}
	installationPrefix = strings.Trim(filepath.ToSlash(installationPrefix), "/")
	for _, changedPath := range changed {
		policyPath := filepath.ToSlash(changedPath)
		if installationPrefix != "" && policyPath != installationPrefix && !strings.HasPrefix(policyPath, installationPrefix+"/") {
			continue
		}
		// ENGINE changes invalidate proof inputs only when the changed path is
		// part of this installation; sibling repositories have separate engines.
		included, err := policy.Includes(behaviorsurface.Engine, policyPath, installationPrefix)
		if err != nil || included {
			return true
		}
	}
	for _, groupID := range record.Proof.SelectedGroups {
		manifest := record.Proof.InputManifests[groupID]
		if len(manifest) == 0 {
			return true
		}
		for _, changedPath := range changed {
			if testInputManifestContains(manifest, changedPath) {
				return true
			}
		}
	}
	return false
}

func rebasedPrefixTrees(root, base, tip string, units []batch.Unit) ([]string, error) {
	return rebasedPrefixTreesWith(root, base, tip, units, gitOutput)
}

func rebasedPrefixTreesWith(root, base, tip string, units []batch.Unit, readGit func(string, ...string) (string, error)) ([]string, error) {
	output, err := readGit(root, "log", "--first-parent", "--reverse", "--format=%T", base+".."+tip)
	if err != nil {
		return nil, err
	}
	commits := strings.Fields(output)
	want := 0
	for _, unit := range units {
		if len(unit.Builds) == 0 {
			want++
		} else {
			want += len(unit.Builds)
		}
	}
	if len(commits) != want {
		return nil, fmt.Errorf("BATCH_PREFIX_PROOF_REFUSED: rebased series has %d commits, want %d", len(commits), want)
	}
	trees := make([]string, 0, len(units))
	offset := 0
	for _, unit := range units {
		if len(unit.Builds) == 0 {
			offset++
		} else {
			offset += len(unit.Builds)
		}
		trees = append(trees, commits[offset-1])
	}
	return trees, nil
}

func gitHead(root string) string {
	tip, _ := gitOutput(root, "rev-parse", "HEAD")
	return tip
}

var batchPrefixReceiptExecutable = os.Executable

type batchExecutionDependencies struct {
	executable func() (string, error)
	checkout   func(string, string) (string, func() error, error)
	topLevel   func(string) (string, error)
	readGit    func(string, ...string) (string, error)
}

func batchDetachedCheckout(root, tree string) (string, func() error, error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return "", nil, err
	}
	return detached.Workspace().Dir, detached.Close, nil
}

func executeBatchPrefixReceiptWithDecision(root, id string, record batch.Record, goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
	return executeBatchPrefixReceiptWithDependencies(root, id, record, goalID, tree, decision, batchExecutionDependencies{
		executable: batchPrefixReceiptExecutable, checkout: batchDetachedCheckout,
	})
}

func executeBatchPrefixReceiptWithDependencies(root, id string, record batch.Record, goalID, tree string, decision batch.PrefixDecision, dependencies batchExecutionDependencies) (batch.PrefixRunResult, error) {
	controlRoot := batch.ModuleRoot(root)
	var unit batch.Unit
	for _, candidate := range record.Units {
		if candidate.GoalID == goalID {
			unit = candidate
			break
		}
	}
	resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+"-prefix-"+goalID+".json")
	detachedRoot, closeDetached, err := dependencies.checkout(root, tree)
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	defer closeDetached()
	executionRoot := batch.ModuleRoot(detachedRoot)
	binary, err := dependencies.executable()
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	args := batchPrefixReceiptArgsWithFresh(executionRoot, controlRoot, goalID, tree, resultPath, decision.Groups, unit.Claim, decision.FreshEpisode, decision.FreshExpiresAt)
	command := exec.Command(binary, args...)
	command.Dir, command.Env = executionRoot, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+landingOwnerLineage)
	output, runErr := command.CombinedOutput()
	if runErr != nil && command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitAdmissionRefused {
		reason := strings.TrimSpace(string(output))
		code := batchAdmissionRefusalCode(reason)
		switch code {
		case "GOAL_REVISION_MOVED":
			return batch.PrefixRunResult{}, &batch.PrefixRevisionRefusal{Reason: reason}
		case "BATCH_MEMBER_BUDGET_REFUSED", "BUDGET_REFUSED":
			return batch.PrefixRunResult{}, &batch.PrefixBudgetRefusal{Reason: reason}
		case "CANDIDATE_GOAL_REFUSED":
			if strings.Contains(reason, "state=fenced") {
				return batch.PrefixRunResult{}, &batch.PrefixFencedRefusal{Reason: reason}
			}
			return batch.PrefixRunResult{}, &batch.PrefixAdmissionRefusal{Code: code, Reason: reason}
		default:
			return batch.PrefixRunResult{}, &batch.PrefixAdmissionRefusal{Code: code, Reason: reason}
		}
	}
	var result proofrun.TestResult
	if err := readStrictJSON(resultPath, &result); err != nil {
		if runErr != nil {
			return batch.PrefixRunResult{}, fmt.Errorf("run batch prefix proof: %s: %w", strings.TrimSpace(string(output)), runErr)
		}
		return batch.PrefixRunResult{}, err
	}
	out := batch.PrefixRunResult{AttemptID: result.AttemptID, ResultPath: resultPath, Reused: map[string]string{}}
	reusableExit := command.ProcessState != nil && command.ProcessState.ExitCode() == proofrun.ExitReusableSuccess
	for _, group := range result.Groups {
		if group.NativeLaunched && !reusableExit {
			out.Executed = append(out.Executed, group.ID)
		}
		if group.ReuseAttempt != "" {
			out.Reused[group.ID] = group.ReuseAttempt
		} else if reusableExit && group.NativeLaunched && result.AttemptID != "" {
			// A no-child reusable result can be the original native result.
			// Its nativeLaunched field describes that source attempt, not
			// work launched by this prefix command.
			out.Reused[group.ID] = result.AttemptID
		}
		if group.Status == "failed" {
			out.Red = append(out.Red, batch.RedGroup{ID: group.ID, Status: group.Status, LogPath: group.LogPath, LogDigest: group.LogDigest, InputManifest: slices.Clone(group.InputManifest)})
		}
	}
	if command.ProcessState != nil && batchProofExitAccepted(command.ProcessState.ExitCode(), result) {
		runErr = nil
	}
	if len(out.Red) != 0 {
		return out, nil
	}
	return out, runErr
}

func batchAdmissionRefusalCode(reason string) string {
	for _, field := range strings.Fields(reason) {
		candidate := strings.Trim(field, ":,;()[]")
		if candidate == "" {
			continue
		}
		valid := true
		for _, char := range candidate {
			if char != '_' && (char < 'A' || char > 'Z') && (char < '0' || char > '9') {
				valid = false
				break
			}
		}
		if valid && (strings.HasSuffix(candidate, "_REFUSED") || strings.HasSuffix(candidate, "_MOVED")) {
			return candidate
		}
	}
	return ""
}

func batchPrefixReceiptArgs(root, controlRoot, goalID, tree, resultPath string, groups []string, claim batch.Claim) []string {
	return batchPrefixReceiptArgsWithFresh(root, controlRoot, goalID, tree, resultPath, groups, claim, "", "")
}

func batchPrefixReceiptArgsWithFresh(root, controlRoot, goalID, tree, resultPath string, groups []string, claim batch.Claim, episode, expiresAt string) []string {
	args := []string{"test", "run", "--root", root, "--control-root", controlRoot, "--goal", goalID, "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--batch-requirements", batchRequirementsArgument(groups), "--result", resultPath,
		"--expected-goal-revision", fmt.Sprint(claim.Revision), "--expected-accounting-revision", fmt.Sprint(claim.AccountingRevision), "--batch-prefix"}
	if episode != "" {
		args = append(args, "--fresh-episode", episode)
		if expiresAt != "" {
			args = append(args, "--fresh-expires-at", expiresAt)
		}
	}
	return args
}

func recoverBatchLanding(root string, store batch.Store, id, actor string, at time.Time) error {
	return batch.RecoverPushedSeries(store, id, actor, at, batchRecoverySeamsWithGit(root, store, id, at, gitOutput))
}

func batchRecoverySeamsWithGit(root string, store batch.Store, id string, at time.Time, gitRead func(string, ...string) (string, error)) batch.RecoverySeams {
	controlRoot := batch.ModuleRoot(root)
	findTrailer := func(matches func(string) bool) (string, bool, error) {
		format := "%H%x00%B%x00"
		output, err := gitRead(root, "log", "--first-parent", "origin/main", "--format="+format)
		if err != nil {
			return "", false, err
		}
		parts := strings.Split(output, "\x00")
		for index := 0; index+1 < len(parts); index += 2 {
			for _, line := range strings.Split(parts[index+1], "\n") {
				if matches(strings.TrimSpace(line)) {
					return strings.TrimSpace(parts[index]), true, nil
				}
			}
		}
		return "", false, nil
	}
	return batch.RecoverySeams{
		OriginCommit: func(unit batch.Unit) (string, bool, error) {
			return findTrailer(func(line string) bool { return landingProvenanceNamesChain(line, unit.Chain) })
		},
		OriginSource: func(_ batch.Unit, source string) (string, bool, error) {
			return findTrailer(func(line string) bool { return line == "Goal-Source: "+source })
		},
		SweepGoalBranch: func(unit batch.Unit, _ string) error {
			endpoint, err := goalBranchEndpoint(controlRoot)
			if err != nil {
				return err
			}
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			transport := ""
			if _, remoteErr := goalBranchGit(controlRoot, "remote", "get-url", "transport"); remoteErr == nil {
				transport = "transport"
			}
			_, err = batchGoalBranchSweep(goalbranch.SweepRequest{Repo: controlRoot, Remote: endpoint.Remote, Transport: transport,
				EndpointTip: record.Landing.PushedTip, GoalID: unit.GoalID, CheckClaim: goalBranchClaimCheck(controlRoot, unit.GoalID, endpoint)})
			return err
		},
		Finalize: func(unit batch.Unit, commit string) error {
			next := recoveredBatchNext(unit, commit)
			current, err := batchRecoveryGoalNext(controlRoot, unit.GoalID, at)
			if err != nil {
				return err
			}
			if current == next {
				return nil
			}
			return batchChildRunner(controlRoot, landingOwnerLineage, "goal", "edit", "--root", controlRoot, "--id", unit.GoalID, "--next", next, "--lineage", landingOwnerLineage)
		},
		Rearm: func(tip string) error {
			return batchRecoveryRearm(root, tip)
		},
		Cleanup: func() error {
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			return batch.CleanupLandingBranch(root, id, record.Landing.PushedTip)
		},
	}
}

// landingProvenanceNamesChain mirrors the field parsing in internal/landing/held.go.
func landingProvenanceNamesChain(line, chain string) bool {
	value, found := strings.CutPrefix(line, "Landing-Provenance:")
	if !found {
		return false
	}
	for _, field := range strings.Fields(value) {
		if field == "chain="+chain {
			return true
		}
	}
	return false
}

func recoveredBatchNext(unit batch.Unit, commit string) string {
	if len(unit.CommitIDs) != 0 {
		return "landed commit:" + commit + ":source=" + unit.CommitIDs[len(unit.CommitIDs)-1]
	}
	return "landed commit:" + commit + ":chain=" + unit.Chain
}

func commitForTree(root, ref, tree string) (string, error) {
	output, err := gitOutput(root, "log", "--first-parent", "--format=%H %T", ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == tree {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("BATCH_LAND_TRUNK_MOVED: no %s commit has base tree %s", ref, tree)
}

func gitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
