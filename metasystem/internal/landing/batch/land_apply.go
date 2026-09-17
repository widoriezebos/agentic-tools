package batch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

// ApplyCertifiedPatch stages exactly the transported, certified chain patch.
func ApplyCertifiedPatch(root, worktree, chain string) error {
	if _, err := batchMergeDriverArgs(); err != nil {
		return err
	}
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", chain, "diff.patch"))
	if err != nil {
		return err
	}
	before, err := (gittree.Workspace{Dir: worktree}).StagedTree()
	if err != nil {
		return err
	}
	if err := contractgit.PreflightPatchAttributes(worktree, before, "certified patch "+chain, patch); err != nil {
		return err
	}
	if output, err := runBatchMergeGit(worktree, patch, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-"); err != nil {
		if contractgit.IsRefusal(err) {
			return err
		}
		return fmt.Errorf("apply certified patch for %s: %s: %w", chain, strings.TrimSpace(string(output)), err)
	}
	after, err := (gittree.Workspace{Dir: worktree}).StagedTree()
	if err != nil {
		return err
	}
	return contractgit.CheckPatchContract(worktree, before, after, patch, "certified patch "+chain)
}

// ApplyBranchBuild stages one certified contribution and its folds in their
// recorded order, preserving the member's commit boundaries on the landing branch.
func ApplyBranchBuild(root, worktree string, build BranchBuild) error {
	if _, err := batchMergeDriverArgs(); err != nil {
		return err
	}
	for _, fold := range build.Folds {
		if err := applyBranchCommit(root, worktree, fold.ID); err != nil {
			return err
		}
	}
	return applyBranchCommit(root, worktree, build.Commit)
}

func BranchLandingMessage(goalID string, build BranchBuild, goalLast bool) string {
	var message strings.Builder
	fmt.Fprintf(&message, "land %s/%s\n\nGoal-Unit: %s/%s\nGoal-Digest: %s\n", goalID, strings.Join(build.Units, "+"), goalID, strings.Join(build.Units, "+"), build.Digest)
	for _, fold := range build.Folds {
		fmt.Fprintf(&message, "Goal-Source: %s\n", fold.ID)
	}
	for _, path := range build.FoldPaths {
		fmt.Fprintf(&message, "Goal-Fold: %s\n", path)
	}
	fmt.Fprintf(&message, "Goal-Source: %s\n", build.Commit)
	for _, coauthor := range build.CoAuthors {
		fmt.Fprintf(&message, "Co-Authored-By: %s\n", coauthor)
	}
	if goalLast {
		fmt.Fprintf(&message, "Goal-Last: %s\n", goalID)
	}
	return message.String()
}
