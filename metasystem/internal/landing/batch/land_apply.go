package batch

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// ApplyCertifiedPatch stages exactly the transported, certified chain patch.
func ApplyCertifiedPatch(root, worktree, chain string) error {
	patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", chain, "diff.patch"))
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", worktree, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-")
	command.Env, command.Stdin = gittree.ScrubbedEnviron(), bytes.NewReader(patch)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("apply certified patch for %s: %s: %w", chain, bytes.TrimSpace(output), err)
	}
	return nil
}

// ApplyBranchBuild stages one certified contribution and its folds in their
// recorded order, preserving the member's commit boundaries on the landing branch.
func ApplyBranchBuild(root, worktree string, build BranchBuild) error {
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
