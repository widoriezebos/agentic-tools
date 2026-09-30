package batchowner

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

	"context"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	receiptpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/receipt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// ExecuteBatchLanding lands one batch with the lane checkout held: concurrent
// landings and base re-arms move the same checkout, so they take turns.
func ExecuteBatchLanding(root, id, actor string, at time.Time) error {
	return WithLaneCheckout(root, func() error { return landBatchInCheckout(root, id, actor, at) })
}

func landBatchInCheckout(root, id, actor string, at time.Time) error {
	store := batch.NewStore(root, nil)
	record, err := store.Load(id)
	if err != nil {
		return err
	}
	if reopened, err := ReopenBatchBeforeReceiptsWhenProofBaseMoved(root, store, id, actor, at, record); err != nil || reopened {
		return err
	}
	if record.Landing == nil || !record.Landing.PushComplete {
		if err := batch.ComposePrefixReceipts(store, id, actor, at, batch.PrefixReceiptSeams{
			Authorize: func(unit batch.Unit, _ batch.Claim) error {
				return AuthorizeBatchMember(root, record, unit)
			},
			Plan: func(units []batch.Unit, tree string) (batch.PrefixDecision, error) {
				return ProductionPrefixDecision(root, units, tree)
			},
			ExecuteDecision: func(goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
				return executeBatchPrefixReceiptWithDecision(root, id, record, goalID, tree, decision)
			},
			Verify: func(unit batch.Unit, tree string, decision batch.PrefixDecision) error {
				return BatchVerifyPrefixEvidence(root, unit, tree, decision)
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
		if err := VerifyBatchSeries(root, record, record.PrefixTrees); err != nil {
			return err
		}
		baseCommit, err := commitForTree(root, "origin/main", record.BaseTree)
		if err != nil {
			return err
		}
		seams := BatchLandSeams(root, id, record, baseCommit, actor)
		if err := batch.LandSeries(store, id, actor, at, seams); err != nil {
			return err
		}
	}
	return FinishBatchLanding(root, store, id, actor, at)
}

func ReopenBatchBeforeReceiptsWhenProofBaseMoved(root string, store batch.Store, id, actor string, at time.Time, record batch.Record) (bool, error) {
	if record.Proof == nil || record.Proof.BaseCommit == "" {
		return false, nil
	}
	origin, originTree, err := BatchLandFetchOrigin(root)
	if err != nil || origin == record.Proof.BaseCommit {
		return false, err
	}
	changed, prefix, err := batchMovedPaths(root, record.BaseTree, originTree)
	if err != nil || !batch.DecideMovedBase(record, changed, prefix).Reopen {
		return false, err
	}
	tip := ""
	if record.Landing != nil {
		tip = record.Landing.CandidateTip
		if tip == "" {
			tip = record.Landing.BranchTip
		}
	}
	if err := BatchLandAbandon(root, id, tip, origin); err != nil {
		return false, err
	}
	return true, batch.ReopenMovedBase(store, id, originTree, landedBatchOn(root, GitOutput, record.Proof.BaseCommit, origin), actor, at)
}

// batchMovedPaths reads the paths main changed between two base trees and
// the installation prefix they are relative to.
func batchMovedPaths(root, fromTree, toTree string) ([]string, string, error) {
	workspace := gittree.Workspace{Dir: root}
	changed, err := workspace.ChangedPaths(fromTree, toTree)
	if err != nil {
		return nil, "", err
	}
	prefix, err := workspace.Prefix()
	return changed, prefix, err
}

// landedBatchOn names the batch whose pushed tip is in the range main moved
// by, for a member's conflict return; empty when no batch landed it.
func landedBatchOn(root string, readGit func(string, ...string) (string, error), fromCommit, toCommit string) string {
	output, err := readGit(root, "rev-list", fromCommit+".."+toCommit)
	if err != nil {
		return ""
	}
	return batch.LandedBatch(batch.NewStore(root, nil), strings.Fields(output))
}

// batchBaseMove is the owner's view of a moved base between two trees.
func batchBaseMove(root string) func(string, string) (batch.BaseMove, error) {
	return func(fromTree, toTree string) (batch.BaseMove, error) {
		changed, prefix, err := batchMovedPaths(root, fromTree, toTree)
		if err != nil {
			return batch.BaseMove{}, err
		}
		move := batch.BaseMove{Changed: changed, Prefix: prefix}
		if from, commitErr := commitForTree(root, "refs/remotes/origin/main", fromTree); commitErr == nil {
			move.LandedBy = landedBatchOn(root, GitOutput, from, "refs/remotes/origin/main")
		}
		return move, nil
	}
}

var BatchLandFetchOrigin = FetchBatchOrigin
var BatchLandAbandon = batch.AbandonLandingBranch
var BatchLandOriginTree = func(root, commit string) (string, error) {
	return GitOutput(root, "rev-parse", commit+"^{tree}")
}
var BatchLandRecoverPush = recoverMovedBatchPush

// batchLandReceipt appends a landed unit's implement receipt to the control
// checkout's receipt ledger.
func batchLandReceipt(controlRoot, goalID, note string) error {
	result, err := receiptpkg.AddToInstallation(controlRoot, receiptpkg.Options{
		Type: "implement", Outcome: "shipped", Goal: goalID, BuiltBy: "coordinator", Note: note,
	})
	for _, line := range result.Err {
		if err == nil {
			fmt.Fprintln(os.Stderr, line)
		}
	}
	return err
}

// BatchCommitBoundary is the commit boundary the landing owner commits each
// unit through: the landing path in this process, which the engine sets at
// start.
var BatchCommitBoundary batch.CommitBoundary

func BatchLandSeams(root, id string, record batch.Record, baseCommit, actor string) batch.LandSeams {
	return BatchLandSeamsWithRead(root, id, record, baseCommit, actor, GitOutput, BatchCommitBoundary)
}

func BatchLandSeamsWithRead(root, id string, record batch.Record, baseCommit, actor string, readGit func(root string, args ...string) (string, error), boundary batch.CommitBoundary) batch.LandSeams {
	controlRoot := batch.ModuleRoot(root)
	return batch.LandSeams{
		Prepare: func(_ string) error { return batch.PrepareLandingBranch(root, id, baseCommit) },
		Apply:   func(unit batch.Unit) error { return batch.ApplyCertifiedPatch(root, root, unit.Chain) },
		AppendReceipt: func(unit batch.Unit, receipt batch.PrefixReceipt) error {
			return batchLandReceipt(controlRoot, unit.GoalID, "batch "+id+" prefix "+receipt.Tree)
		},
		Commit: func(unit batch.Unit, receipt batch.PrefixReceipt) (string, error) {
			path := receipt.ResultPath
			if path == "" {
				path = filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+".json")
			}
			message := fmt.Sprintf("land %s in batch %s\n\nOriginal join order; prefix tree %s.\n", unit.GoalID, id, receipt.Tree)
			if err := batch.CommitWithWrapperWithRead(controlRoot, batch.ChainDeclaration(unit.Chain), unit.GoalID, path, message, unit.AuthorName, unit.AuthorEmail, actor, LandingOwnerLineage, boundary, readGit); err != nil {
				return "", err
			}
			return readGit(root, "rev-parse", "HEAD")
		},
		ReplayChange: func(unit batch.Unit) (string, error) {
			return batch.ReplayChange(root, unit)
		},
		ApplyBuild: func(_ batch.Unit, build batch.BranchBuild) error {
			return batch.ApplyBranchBuild(root, root, build)
		},
		AppendBuildReceipt: func(unit batch.Unit, build batch.BranchBuild, receipt batch.PrefixReceipt) error {
			return batchLandReceipt(controlRoot, unit.GoalID, "batch "+id+" unit "+strings.Join(build.Units, "+")+" prefix "+receipt.Tree)
		},
		CommitBuild: func(unit batch.Unit, build batch.BranchBuild, receipt batch.PrefixReceipt) (string, error) {
			path := receipt.ResultPath
			if path == "" {
				path = filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+".json")
			}
			last := unit.GoalLast && len(unit.CommitIDs) != 0 && build.Commit == unit.CommitIDs[len(unit.CommitIDs)-1]
			declaration := batch.AttestedDeclaration(build.Commit, unit.BranchTip, baseCommit)
			if err := batch.CommitWithWrapperWithRead(controlRoot, declaration, unit.GoalID, path, batch.BranchLandingMessage(unit.GoalID, build, last), unit.AuthorName, unit.AuthorEmail, actor, LandingOwnerLineage, boundary, readGit); err != nil {
				return "", err
			}
			commit, err := readGit(root, "rev-parse", "HEAD")
			if err != nil {
				return "", err
			}
			if err := batch.RequirePassingCommitVerdictWithRead(root, unit.GoalID, commit, readGit); err != nil {
				return "", err
			}
			return commit, nil
		},
		Held: func(_, tip string) error {
			return BatchOwnerCalls.Held(controlRoot, baseCommit, tip, "origin", "refs/heads/main")
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
			commit, _, err := BatchLandFetchOrigin(root)
			return commit, err
		},
		OriginTree: func(commit string) (string, error) {
			return BatchLandOriginTree(root, commit)
		},
		FlakeRegister: func() ([]batch.OpenEntry, error) {
			ledger, err := ProductionTrunkRedLedgerOwner(controlRoot)
			if err != nil {
				return nil, err
			}
			return ledger.Open()
		},
		Now:            func() (time.Time, error) { return fixtureauth.GoalNow(controlRoot) },
		Abandon:        func(tip, detachAt string) error { return BatchLandAbandon(root, id, tip, detachAt) },
		SeriesOnOrigin: func(origin, tip string) (bool, error) { return batchSeriesOnEndpoint(root, origin, tip) },
		LeaseBase:      baseCommit,
		RecoverPush: func(origin, baseTree, tip string, recheck func() error) (batch.PushRecovery, error) {
			return BatchLandRecoverPush(root, id, record, actor, baseCommit, origin, baseTree, tip, recheck)
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

func FinishBatchLanding(root string, store batch.Store, id, actor string, at time.Time) error {
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
	return RecoverBatchLanding(root, store, id, actor, at)
}

var BatchMovedEndpointPush = batch.LandLandingBranch
var BatchVerifyRebasedSeries = VerifyBatchSeries
var BatchAuthorizeRebasedSeries = authorizeBatchSeries
var BatchGoalBranchSweep = goalbranch.Sweep
var BatchRecoveryRearm = RearmBatchTip
var BatchRecoveryGoalNext = func(root, goalID string, at time.Time) (string, error) {
	endpoint, err := goalbranch.MainEndpoint(root)
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

func recoverMovedBatchPush(root, id string, record batch.Record, actor, expectedBase, originCommit, baseTree, tip string, recheck func() error) (batch.PushRecovery, error) {
	edges := productionMovedBaseEdges(GitOutput, func(cmd *exec.Cmd) error { return cmd.Run() })
	edges.Recheck = recheck
	return RecoverMovedBatchPushWith(root, id, record, actor, expectedBase, originCommit, baseTree, tip, edges)
}

// MovedBaseEdges are the Git and proof edges of the moved-base rebase at the
// push; production binds them in productionMovedBaseEdges, a test stubs each.
type MovedBaseEdges struct {
	ReadGit    func(string, ...string) (string, error)
	OnEndpoint func(root, origin, tip string) (bool, error)
	Paths      func(root, fromTree, toTree string) ([]string, string, error)
	Advance    func(root string) error
	Held       func(root, base, commit, remote, ref string) error
	Verify     func(string, batch.Record, []string) error
	Authorize  func(string, batch.Store, batch.Record, string, time.Time) error
	Now        func(string) (time.Time, error)
	Publish    func(root, id, expected, tip string) error
	Push       func(root, id, base, tip string) error
	Fetch      func(string) (string, string, error)
	// Recheck is the lane's flake-allowance recheck, run immediately before
	// the endpoint push; nil when the proof carried no known flake seam.
	Recheck func() error
}

func productionMovedBaseEdges(readGit func(string, ...string) (string, error), runGit func(*exec.Cmd) error) MovedBaseEdges {
	return MovedBaseEdges{ReadGit: readGit,
		OnEndpoint: func(root, origin, tip string) (bool, error) {
			return batchSeriesOnEndpointWithRunner(root, origin, tip, runGit)
		},
		Paths: batchMovedPaths,
		Advance: func(root string) error {
			var output bytes.Buffer
			if err := landing.Advance(root, "refs/remotes/origin/main", &output, &output); err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(output.String()))
			}
			return nil
		},
		Held: func(root, base, commit, remote, ref string) error {
			return BatchOwnerCalls.Held(root, base, commit, remote, ref)
		},
		Verify: func(root string, record batch.Record, trees []string) error {
			return BatchVerifyRebasedSeries(root, record, trees)
		},
		Authorize: func(root string, store batch.Store, record batch.Record, actor string, at time.Time) error {
			return BatchAuthorizeRebasedSeries(root, store, record, actor, at)
		},
		Now: fixtureauth.GoalNow, Publish: batch.PublishLandingBranch,
		Push:  func(root, id, base, tip string) error { return BatchMovedEndpointPush(root, id, base, tip) },
		Fetch: FetchBatchOrigin,
	}
}

func RecoverMovedBatchPushWithInputs(root, id string, record batch.Record, actor, expectedBase, originCommit, baseTree, tip string,
	readGit func(root string, args ...string) (string, error), runGit func(*exec.Cmd) error) (batch.PushRecovery, error) {
	return RecoverMovedBatchPushWith(root, id, record, actor, expectedBase, originCommit, baseTree, tip, productionMovedBaseEdges(readGit, runGit))
}

// RecoverMovedBatchPushWith is the moved-base decision at the push: a changed
// input reopens; otherwise the series rebases onto origin, the retained
// verifier re-verifies it by identity on the rebased tip, and it is pushed.
func RecoverMovedBatchPushWith(root, id string, record batch.Record, actor, expectedBase, originCommit, baseTree, tip string, edges MovedBaseEdges) (batch.PushRecovery, error) {
	controlRoot := batch.ModuleRoot(root)
	originTree, err := edges.ReadGit(root, "rev-parse", originCommit+"^{tree}")
	recovery := batch.PushRecovery{Origin: originCommit, BaseTree: originTree}
	if err != nil {
		return recovery, err
	}
	landed, err := edges.OnEndpoint(root, originCommit, tip)
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
	changed, prefix, err := edges.Paths(root, baseTree, originTree)
	if err != nil {
		return recovery, err
	}
	recovery.LandedBy = landedBatchOn(root, edges.ReadGit, expectedBase, originCommit)
	if batch.DecideMovedBase(record, changed, prefix).Reopen {
		recovery.Reopen = true
		return recovery, nil
	}
	if err := edges.Advance(root); err != nil {
		// A conflicting rebase reopens: the reassembly on the new base ejects
		// the member whose changes do not apply, naming the files.
		return ReopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, fmt.Errorf("rebase landing series: %w", err))
	}
	rebasedTip, err := edges.ReadGit(root, "rev-parse", "HEAD")
	if err != nil {
		return recovery, err
	}
	if err := edges.Held(controlRoot, originCommit, rebasedTip, "origin", "refs/heads/main"); err != nil {
		return ReopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, fmt.Errorf("rebased landing held: %w", err))
	}
	joined := slices.DeleteFunc(slices.Clone(record.Units), func(unit batch.Unit) bool { return unit.State != batch.UnitJoined })
	if len(joined) == 0 {
		return recovery, fmt.Errorf("rebased landing has no joined authority member")
	}
	finalTrees, err := RebasedPrefixTreesWith(root, originCommit, rebasedTip, joined, edges.ReadGit)
	if err != nil {
		return ReopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, err)
	}
	if err := edges.Verify(root, record, finalTrees); err != nil {
		return ReopenMovedBatchAfterRecoveryFailure(root, id, tip, originCommit, recovery, fmt.Errorf("rebased prefix verification: %w", err))
	}
	// The first endpoint attempt's authority check cannot authorize this
	// retry: a member can be fenced or revised while the moved series is
	// rebased and reproved. A typed refusal returns that member and rebuilds
	// the survivors before either rebased publication or endpoint push.
	at, err := edges.Now(controlRoot)
	if err != nil {
		return recovery, err
	}
	if err := edges.Authorize(root, batch.NewStore(root, nil), record, actor, at); err != nil {
		return recovery, err
	}
	if err := edges.Publish(root, id, tip, rebasedTip); err != nil {
		return recovery, err
	}
	recovery.Tip = rebasedTip
	if edges.Recheck != nil {
		if err := edges.Recheck(); err != nil {
			return recovery, err
		}
	}
	if err := edges.Push(root, id, originCommit, rebasedTip); err != nil {
		latest, latestTree, fetchErr := edges.Fetch(root)
		if fetchErr == nil {
			recovery.Origin, recovery.BaseTree = latest, latestTree
		}
		return recovery, errors.Join(err, fetchErr)
	}
	recovery.Pushed = true
	return recovery, nil
}

func ReopenMovedBatchAfterRecoveryFailure(_, _, _, _ string, recovery batch.PushRecovery, _ error) (batch.PushRecovery, error) {
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
func FetchBatchOrigin(root string) (string, string, error) {
	command := exec.Command("git", "-C", root, "fetch", "origin", "+refs/heads/main:refs/remotes/origin/main")
	command.Env = gittree.ScrubbedEnviron()
	if output, err := command.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("%s: fetch origin/main: %s: %w", codeLandPushRefused, strings.TrimSpace(string(output)), err)
	}
	commit, err := GitOutput(root, "rev-parse", "refs/remotes/origin/main")
	if err != nil {
		return "", "", err
	}
	tree, err := GitOutput(root, "rev-parse", commit+"^{tree}")
	return commit, tree, err
}

func RebasedPrefixTreesWith(root, base, tip string, units []batch.Unit, readGit func(string, ...string) (string, error)) ([]string, error) {
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
		return nil, fmt.Errorf("%s: rebased series has %d commits, want %d", codePrefixProofRefused, len(commits), want)
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
	tip, _ := GitOutput(root, "rev-parse", "HEAD")
	return tip
}

var BatchPrefixReceiptExecutable = os.Executable

type BatchExecutionDependencies struct {
	Executable func() (string, error)
	Checkout   func(string, string) (string, func() error, error)
	TopLevel   func(string) (string, error)
	ReadGit    func(string, ...string) (string, error)
}

func batchDetachedCheckout(root, tree string) (string, func() error, error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(tree)
	if err != nil {
		return "", nil, err
	}
	return detached.Workspace().Dir, detached.Close, nil
}

func executeBatchPrefixReceiptWithDecision(root, id string, record batch.Record, goalID, tree string, decision batch.PrefixDecision) (batch.PrefixRunResult, error) {
	return ExecuteBatchPrefixReceiptWithDependencies(root, id, record, goalID, tree, decision, BatchExecutionDependencies{
		Executable: BatchPrefixReceiptExecutable, Checkout: batchDetachedCheckout,
	})
}

func ExecuteBatchPrefixReceiptWithDependencies(root, id string, record batch.Record, goalID, tree string, decision batch.PrefixDecision, dependencies BatchExecutionDependencies) (batch.PrefixRunResult, error) {
	controlRoot := batch.ModuleRoot(root)
	var unit batch.Unit
	for _, candidate := range record.Units {
		if candidate.GoalID == goalID {
			unit = candidate
			break
		}
	}
	resultPath := filepath.Join(controlRoot, "artifacts", "agents", "proof-runs", "batch", id+"-prefix-"+goalID+".json")
	detachedRoot, closeDetached, err := dependencies.Checkout(root, tree)
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	defer closeDetached()
	executionRoot := batch.ModuleRoot(detachedRoot)
	binary, err := dependencies.Executable()
	if err != nil {
		return batch.PrefixRunResult{}, err
	}
	args := BatchPrefixReceiptArgsWithFresh(executionRoot, controlRoot, goalID, tree, resultPath, decision.Groups, unit.Claim, decision.FreshEpisode, decision.FreshExpiresAt)
	command := exec.Command(binary, args...)
	command.Dir, command.Env = executionRoot, append(gittree.ScrubbedEnviron(), "METASYSTEM_OWNER_LINEAGE="+LandingOwnerLineage)
	child, err := runTestRunChild(command)
	if err != nil {
		return batch.PrefixRunResult{}, fmt.Errorf("run the batch's test run: %w", err)
	}
	if child.Outcome == verbresult.Refused {
		return batch.PrefixRunResult{}, admissionRefusal(child)
	}
	var result proofrun.TestResult
	if err := strictjson.Read(resultPath, &result); err != nil {
		if child.Outcome != verbresult.Confirmed {
			return batch.PrefixRunResult{}, fmt.Errorf("run the batch's test run: %w", child.Err())
		}
		return batch.PrefixRunResult{}, err
	}
	out := batch.PrefixRunResult{AttemptID: result.AttemptID, ResultPath: resultPath, Reused: map[string]string{}}
	reusableExit := child.Outcome == verbresult.Unchanged
	for _, group := range result.Groups {
		switch PrefixGroupExecution(group, reusableExit) {
		case "cached":
			out.CachedPass = append(out.CachedPass, group.ID)
		case "executed":
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
	if len(out.Red) != 0 || BatchProofOutcomeAccepted(child, result) {
		return out, nil
	}
	return out, fmt.Errorf("run the batch's test run: %w", child.Err())
}

// PrefixGroupExecution is how a prefix proof's group counts: "executed"
// when this command launched it and a package ran, "cached" when the test tool
// served it wholly from its result cache (a pass by cache, not an execution),
// else "" (reused or not launched here).
func PrefixGroupExecution(group proofrun.GroupResult, reusableExit bool) string {
	switch {
	case !group.NativeLaunched || reusableExit:
		return ""
	case group.PassedByGoTestCache():
		return "cached"
	}
	return "executed"
}

func BatchPrefixReceiptArgsWithFresh(root, controlRoot, goalID, tree, resultPath string, groups []string, claim batch.Claim, episode, expiresAt string) []string {
	args := []string{"internal", "test", "run", "--root", root, "--control-root", controlRoot, "--goal", goalID, "--tree", tree, "--mode", "auto", "--purpose", "delivery", "--batch-requirements", testrun.BatchRequirementsArgument(groups), "--result", resultPath,
		"--expected-goal-revision", fmt.Sprint(claim.Revision), "--expected-accounting-revision", fmt.Sprint(claim.AccountingRevision), "--batch-prefix", "--json"}
	if episode != "" {
		args = append(args, "--fresh-episode", episode)
		if expiresAt != "" {
			args = append(args, "--fresh-expires-at", expiresAt)
		}
	}
	return args
}

func RecoverBatchLanding(root string, store batch.Store, id, actor string, at time.Time) error {
	return batch.RecoverPushedSeries(store, id, actor, at, BatchRecoverySeamsWithGit(root, store, id, at, GitOutput))
}

func BatchRecoverySeamsWithGit(root string, store batch.Store, id string, at time.Time, gitRead func(string, ...string) (string, error)) batch.RecoverySeams {
	return recoverySeamsAt(root, batch.ModuleRoot(root), store, id, at, gitRead, &BatchOwnerCalls)
}

// recoverySeamsAt are the landed-trailer recovery seams of the lane whose
// checkout is root and whose installation is controlRoot, editing goals
// through calls, read when each edit is made.
func recoverySeamsAt(root, controlRoot string, store batch.Store, id string, at time.Time, gitRead func(string, ...string) (string, error), calls *BatchOwnerCallSet) batch.RecoverySeams {
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
		OriginChange: func(unit batch.Unit) (string, bool, error) {
			return findTrailer(func(line string) bool { return line == batch.LandingChangeTrailer+": "+unit.GoalID })
		},
		OriginSource: func(_ batch.Unit, source string) (string, bool, error) {
			return findTrailer(func(line string) bool { return line == "Goal-Source: "+source })
		},
		SweepGoalBranch: func(unit batch.Unit, _ string) error {
			endpoint, err := goalbranch.MainEndpoint(controlRoot)
			if err != nil {
				return err
			}
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			transport := ""
			if _, remoteErr := goalbranch.ScrubbedGit(controlRoot, "remote", "get-url", "transport"); remoteErr == nil {
				transport = "transport"
			}
			// Inside the critical section of the goal's registered
			// worktrees, as goal done's sweep (Round D3 N1).
			return steward.SweepGoalWorktrees(controlRoot, unit.GoalID, func(ctx context.Context) error {
				_, err := BatchGoalBranchSweep(goalbranch.SweepRequest{Repo: controlRoot, Remote: endpoint.Remote, Transport: transport,
					EndpointTip: record.Landing.PushedTip, GoalID: unit.GoalID, CheckClaim: Engine.BranchClaimCheck(controlRoot, unit.GoalID, endpoint), Context: ctx})
				return err
			})
		},
		Finalize: func(unit batch.Unit, commit string) error {
			next := RecoveredBatchNext(unit, commit)
			current, err := BatchRecoveryGoalNext(controlRoot, unit.GoalID, at)
			if err != nil {
				return err
			}
			if current == next {
				return nil
			}
			return calls.EditNext(LandingOwnerInvocation(), controlRoot, unit.GoalID, next)
		},
		Rearm: func(tip string) error {
			return BatchRecoveryRearm(root, tip)
		},
		Cleanup: func() error {
			record, err := store.Load(id)
			if err != nil {
				return err
			}
			return batch.CleanupLandingBranch(root, id, record.Landing.PushedTip)
		},
		Release: ReleaseMemberSet(id, at),
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

func RecoveredBatchNext(unit batch.Unit, commit string) string {
	if len(unit.CommitIDs) != 0 {
		return "landed commit:" + commit + ":source=" + unit.CommitIDs[len(unit.CommitIDs)-1]
	}
	return "landed commit:" + commit + ":chain=" + unit.Chain
}

func commitForTree(root, ref, tree string) (string, error) {
	output, err := GitOutput(root, "log", "--first-parent", "--format=%H %T", ref)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == tree {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s: no %s commit has base tree %s", codeLandTrunkMoved, ref, tree)
}

func GitOutput(root string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", root}, args...)...)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.Output()
	return strings.TrimSpace(string(output)), err
}
