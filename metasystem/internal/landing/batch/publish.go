package batch

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

func PublishJoin(store Store, batchID string, unit Unit, actor string, at time.Time, plan func(string, string, string) (testpolicy.Plan, error), handover func() error) error {
	return PublishJoinWithAdmission(store, batchID, unit, actor, at, plan, handover, func(_ string, unit Unit) (JoinAdmission, error) {
		return JoinAdmission{Tree: unit.Admission.Tree, Status: "verified"}, nil
	})
}

func planJoinedUnit(root, baseTree string, unit Unit, unitTree string, plan func(string, string, string) (testpolicy.Plan, error)) (_ testpolicy.Plan, err error) {
	if _, err := batchMergeDriverArgs(); err != nil {
		return testpolicy.Plan{}, err
	}
	baseCommit, err := commitForWorkspaceTree(root, baseTree)
	if err != nil {
		return testpolicy.Plan{}, err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: resolve workspace: %w", unit.GoalID, err)
	}
	detached, err := (gittree.Workspace{Dir: canonicalRoot}).NewDetachedCommitWorktree(baseCommit)
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: create planning worktree: %w", unit.GoalID, err)
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	planningRoot := detached.Workspace().Dir
	policyRef := "refs/remotes/metasystem-batch/" + baseCommit
	if output, refErr := runPlanningGit(canonicalRoot, nil, "update-ref", policyRef, baseCommit); refErr != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: pin planning base: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), refErr)
	}
	if output, configErr := runPlanningGit(canonicalRoot, nil, "config", "extensions.worktreeConfig", "true"); configErr != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: enable isolated planning configuration: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), configErr)
	}
	if output, configErr := runPlanningGit(planningRoot, nil, "config", "--worktree", "metasystem.steward.landing-ref", policyRef); configErr != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: configure planning base: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), configErr)
	}
	if source := filepath.Join(canonicalRoot, "artifacts"); pathExists(source) && !pathExists(filepath.Join(planningRoot, "artifacts")) {
		if err := os.Symlink(source, filepath.Join(planningRoot, "artifacts")); err != nil {
			return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: expose planning records: %w", unit.GoalID, err)
		}
	}
	patch, err := joinedUnitPatch(root, unit)
	if err != nil {
		return testpolicy.Plan{}, err
	}
	if err := contractgit.PreflightPatchAttributes(planningRoot, baseCommit, "unit "+unit.GoalID, patch); err != nil {
		return testpolicy.Plan{}, err
	}
	if output, applyErr := runPlanningMergeGit(planningRoot, patch, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-"); applyErr != nil {
		if contractgit.IsRefusal(applyErr) {
			return testpolicy.Plan{}, applyErr
		}
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: apply at batch base: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), applyErr)
	}
	candidate, err := (gittree.Workspace{Dir: planningRoot}).StagedTree()
	if err != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: candidate tree=%s want=%s: %w", unit.GoalID, candidate, unitTree, err)
	}
	if candidate != unitTree {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: candidate tree=%s want=%s", unit.GoalID, candidate, unitTree)
	}
	if err := contractgit.CheckPatchContract(planningRoot, baseCommit, candidate, patch, "unit "+unit.GoalID); err != nil {
		return testpolicy.Plan{}, err
	}
	if output, commitErr := runPlanningGit(planningRoot, nil, "-c", "user.name=MetaSystem", "-c", "user.email=metasystem@invalid", "-c", "core.hooksPath=/dev/null", "commit", "--quiet", "-m", "temporary batch unit plan"); commitErr != nil {
		return testpolicy.Plan{}, fmt.Errorf("select joined unit %s: commit planning candidate: %s: %w", unit.GoalID, strings.TrimSpace(string(output)), commitErr)
	}
	return plan(planningRoot, unit.GoalID, candidate)
}

// PlanJoinedUnit exposes the existing exact-base selection for the bounded
// pre-handover cost snapshot. Publication repeats it under the CAS guard.
func PlanJoinedUnit(root, baseTree string, unit Unit, unitTree string, plan func(string, string, string) (testpolicy.Plan, error)) (testpolicy.Plan, error) {
	return planJoinedUnit(root, baseTree, unit, unitTree, plan)
}

func commitForWorkspaceTree(root, tree string) (string, error) {
	workspace := gittree.Workspace{Dir: root}
	top, err := workspace.TopLevel()
	if err != nil {
		return "", err
	}
	search := func(args ...string) string {
		command := exec.Command("git", append([]string{"-C", top}, args...)...)
		command.Env = gittree.ScrubbedEnviron()
		output, commandErr := command.Output()
		if commandErr != nil {
			return ""
		}
		for _, commit := range strings.Fields(string(output)) {
			if candidate, candidateErr := workspace.TreeOf(commit); candidateErr == nil && candidate == tree {
				return commit
			}
		}
		return ""
	}
	if commit := search("rev-list", "--all", "--reflog"); commit != "" {
		return commit, nil
	}
	if commit := search("rev-list", "--first-parent", "FETCH_HEAD"); commit != "" {
		return commit, nil
	}
	return "", fmt.Errorf("select joined unit: no commit names batch base tree %s", tree)
}

func joinedUnitPatch(root string, unit Unit) ([]byte, error) {
	if len(unit.Builds) != 0 {
		member, _ := branchMemberOf(unit)
		return branchMemberPatch(root, member)
	}
	return os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", unit.Chain, "diff.patch"))
}

func runPlanningGit(dir string, stdin []byte, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir, command.Env, command.Stdin = dir, gittree.ScrubbedEnviron(), bytes.NewReader(stdin)
	return command.CombinedOutput()
}

func runPlanningMergeGit(dir string, stdin []byte, args ...string) ([]byte, error) {
	return runBatchMergeGit(dir, stdin, args...)
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
func ReconcileJoins(store Store, batchID, tree, actor string, at time.Time, read func(string, string, string, string) (Claim, error)) error {
	if read == nil {
		read = claimAt
	}
	return store.locked(func() error {
		return store.updateLocked(batchID, func(record *Record) error {
			for index := range record.Units {
				unit := &record.Units[index]
				if unit.State != UnitJoining {
					continue
				}
				if unit.Admission != nil {
					if unit.Admission.Status == "handed-over" {
						// The admission owner must reconsume or execute evidence;
						// claim equality alone cannot promote membership.
						continue
					}
					claim, claimErr := read(store.root, tree, batchID, unit.GoalID)
					if claimErr == nil && claim.Machine == unit.Claim.Machine && claim.Lineage == unit.Claim.Lineage && claim.Revision == unit.Claim.Revision && claim.AccountingRevision == unit.Claim.AccountingRevision {
						// Handover completed before the joiner crashed while writing
						// its marker. Resume its proof; never promote it here.
						unit.Admission.Status = "handed-over"
						continue
					}
				}
				claim, claimErr := read(store.root, tree, batchID, unit.GoalID)
				unit.State, unit.Outcome, unit.Failure = UnitReturnPending, UnitEjected, "join-incomplete"
				if claimErr == nil && claim.Machine == unit.Claim.Machine && claim.Lineage == unit.Claim.Lineage && claim.Revision == unit.Claim.Revision && claim.AccountingRevision == unit.Claim.AccountingRevision {
					unit.State, unit.Outcome, unit.Failure = UnitJoined, "", ""
				}
				appendUnitHistory(record, at, "reconcile", actor, unit.GoalID, UnitJoining, unit.State)
			}
			return nil
		})
	})
}
func appendUnitHistory(record *Record, at time.Time, verb, actor, goalID, from, to string) {
	record.History = append(record.History, HistoryEntry{At: at.UTC().Format(time.RFC3339Nano), Verb: verb, From: from, To: to, Actor: actor, Detail: goalID + " " + to})
}
