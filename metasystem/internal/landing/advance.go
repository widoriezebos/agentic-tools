package landing

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
)

type advanceRefusal struct {
	code   string
	detail string
}

func (r *advanceRefusal) Error() string {
	return fmt.Sprintf("advance refused: %s: %s", r.code, r.detail)
}

func (r *advanceRefusal) IsAdvanceRefusal() {}

var advanceBeforeSwap func()

// Advance rebases the landing commit in a private worktree, classifies every
// dirty path that the result would change, and moves the real branch only
// through reset --keep while holding the checkout mutation lock. It never
// writes, moves, or restores an append-only register.
func Advance(root, upstream string, stdout, stderr io.Writer) error {
	return advanceWithWorkspace(root, upstream, stdout, stderr, gittree.Workspace{Dir: root})
}

func advanceWithWorkspace(root, upstream string, stdout, stderr io.Writer, workspace gittree.Workspace) error {
	if root == "" || upstream == "" {
		return fmt.Errorf("landing advance requires --root and --upstream")
	}
	driverArgs, err := contractgit.RuntimeDriverArgs()
	if err != nil {
		return err
	}
	release, err := lease.LockBounded(lease.LockPath(root), "landing advance")
	if err != nil {
		return &advanceRefusal{code: "advance-checkout-locked", detail: err.Error()}
	}
	defer release()

	branch, detached, err := workspace.SymbolicHead()
	if err != nil {
		return err
	}
	if detached {
		return &advanceRefusal{code: "advance-not-on-branch", detail: "HEAD is detached"}
	}
	landingCommit, unborn, err := workspace.HeadCommit()
	if err != nil {
		return err
	}
	if unborn {
		return &advanceRefusal{code: "advance-not-on-branch", detail: "HEAD is unborn"}
	}
	landingTree, err := workspace.ResolveRef(landingCommit + "^{tree}")
	if err != nil {
		return err
	}
	posture, err := workspace.TopStagedPosture()
	if err != nil {
		return err
	}
	if len(posture.Unmerged) != 0 || posture.Tree != landingTree {
		return &advanceRefusal{code: "advance-index-not-empty", detail: "the repository index does not equal HEAD"}
	}
	upstreamCommit, err := workspace.ResolveCommit(upstream)
	if err != nil {
		return err
	}

	ancestor, err := workspace.IsAncestor(upstreamCommit, landingCommit)
	if err != nil {
		return err
	}
	if ancestor {
		fmt.Fprintf(stdout, "advance: up to date with %s\n", upstream)
		return nil
	}

	detachedWorktree, err := workspace.NewDetachedCommitWorktree(landingCommit)
	if err != nil {
		return err
	}
	rebase, rebaseErr := detachedWorktree.Rebase(upstream, driverArgs...)
	closeErr := detachedWorktree.Close()
	if rebaseErr != nil || closeErr != nil {
		return errors.Join(rebaseErr, closeErr)
	}
	if rebase.Conflicted {
		return &advanceRefusal{code: "advance-rebase-conflict", detail: rebase.Output}
	}
	rebasedCommit := rebase.Head
	rebasedTree, err := workspace.ResolveRef(rebasedCommit + "^{tree}")
	if err != nil {
		return err
	}
	top, err := workspace.TopLevel()
	if err != nil {
		return err
	}
	topWorkspace := gittree.Workspace{Dir: top, RawSource: workspace.RawSource}
	changed, err := topWorkspace.ChangedPaths(landingTree, rebasedTree)
	if err != nil {
		return err
	}
	dirty, err := workspace.Status()
	if err != nil {
		return err
	}
	for _, entry := range dirty {
		if entry.Index != ' ' && !(entry.Index == '?' && entry.Worktree == '?') {
			return &advanceRefusal{code: "advance-index-not-empty", detail: entry.Path + " is staged"}
		}
	}

	prefix, err := workspace.Prefix()
	if err != nil {
		return err
	}
	changedSet := make(map[string]bool, len(changed))
	for _, path := range changed {
		changedSet[path] = true
	}
	overlap := []gittree.StatusEntry{}
	dirtyRegisters := []string{}
	for _, entry := range dirty {
		if register, path := workspaceRegisterPath(prefix, entry.Path); register {
			dirtyRegisters = append(dirtyRegisters, path)
		}
		if changedSet[entry.Path] {
			overlap = append(overlap, entry)
		}
	}
	if len(overlap) != 0 {
		paths := make([]string, 0, len(overlap))
		for _, entry := range overlap {
			paths = append(paths, entry.Path)
		}
		beforeEntries, err := topWorkspace.Entries(landingTree, paths)
		if err != nil {
			return err
		}
		afterEntries, err := topWorkspace.Entries(rebasedTree, paths)
		if err != nil {
			return err
		}
		removedRegisters := []string{}
		unstagedDrift := []string{}
		contendedRegisters := []string{}
		for _, entry := range overlap {
			register, workspacePath := workspaceRegisterPath(prefix, entry.Path)
			if !register {
				unstagedDrift = append(unstagedDrift, entry.Path)
				continue
			}
			before, existed := beforeEntries[entry.Path]
			after, present := afterEntries[entry.Path]
			if !present || (existed && before.Mode != after.Mode) {
				removedRegisters = append(removedRegisters, workspacePath)
				continue
			}
			appendShape, err := isRegisterAppend(workspace, workspacePath)
			if err != nil {
				return err
			}
			if !appendShape {
				unstagedDrift = append(unstagedDrift, entry.Path)
				continue
			}
			contendedRegisters = append(contendedRegisters, workspacePath)
		}
		switch {
		case len(removedRegisters) != 0:
			return &advanceRefusal{
				code:   "advance-register-removed",
				detail: fmt.Sprintf("%s removed or changed mode in %s", strings.Join(removedRegisters, ", "), rebasedCommit),
			}
		case len(unstagedDrift) != 0:
			return &advanceRefusal{
				code:   "advance-unstaged-drift",
				detail: fmt.Sprintf("%s would be changed by %s", strings.Join(unstagedDrift, ", "), rebasedCommit),
			}
		case len(contendedRegisters) != 0:
			return &advanceRefusal{
				code:   "advance-register-contended",
				detail: fmt.Sprintf("%s would be changed by %s", strings.Join(contendedRegisters, ", "), rebasedCommit),
			}
		}
	}

	if advanceBeforeSwap != nil {
		advanceBeforeSwap()
	}
	currentCommit, currentUnborn, err := workspace.HeadCommit()
	if err != nil {
		return err
	}
	currentBranch, currentDetached, err := workspace.SymbolicHead()
	if err != nil {
		return err
	}
	if currentUnborn || currentDetached || currentCommit != landingCommit || currentBranch != branch {
		return &advanceRefusal{
			code:   "advance-head-moved",
			detail: fmt.Sprintf("HEAD moved from %s on %s to %s on %s", landingCommit, branch, currentCommit, currentBranch),
		}
	}
	reset, err := workspace.ResetKeep(rebasedCommit)
	if err != nil {
		return err
	}
	if !reset.Moved {
		return &advanceRefusal{code: "advance-unstaged-drift", detail: reset.Output}
	}
	currentCommit, currentUnborn, err = workspace.HeadCommit()
	if err != nil {
		return err
	}
	if currentUnborn || currentCommit != rebasedCommit {
		return &advanceRefusal{
			code:   "advance-head-moved",
			detail: fmt.Sprintf("HEAD should be %s after reset but is %s", rebasedCommit, currentCommit),
		}
	}
	fmt.Fprintf(stdout, "advance: %s -> %s; registers untouched: %s\n",
		landingCommit, rebasedCommit, strings.Join(dirtyRegisters, ", "))
	return nil
}
