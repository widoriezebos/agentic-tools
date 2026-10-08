package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func reviewSubjectIdentity(subject launch.UnitSubject) string {
	if subject.Commit != "" {
		return subject.Commit
	}
	return subject.DiffDigest
}

// roundPatchPending checks the committed tree independently of the owned index.
func (inv *intentInvocation) roundPatchPending(work *reviewWorkContext, head string) (bool, error) {
	review := work.review
	if review.Round.Result == nil || review.Subject != nil && review.Subject.Commit != "" {
		return false, nil
	}
	path := filepath.Join(review.Round.Directory, "result.patch")
	patch, err := os.ReadFile(path)
	if err != nil || len(patch) == 0 || launch.UnitResultDigest(string(patch)) != review.Round.Result.PatchDigest {
		return false, fmt.Errorf("the retained pending patch cannot be verified: %v", err)
	}
	temporary, done, err := diskstore.ScratchDir("unit-drop-index-*")
	if err != nil {
		return false, err
	}
	defer done()
	git := inv.unitRunner().Git
	if git == nil {
		git = launch.OSGitRunner{}
	}
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(temporary, "index")}
	if _, err := git.Run(review.Record.Worktree, env, "read-tree", head); err != nil {
		return false, err
	}
	_, err = git.Run(review.Record.Worktree, env, "apply", "--cached", "--reverse", "--check", "--binary", path)
	return err != nil, nil
}

func (inv *intentInvocation) preparePendingRemoval(work *reviewWorkContext, dir string) error {
	drop, review, git := work.subject.Drop, work.review, inv.work().git
	fail := func(err error) error {
		drop.Subject.Conflict = err.Error()
		_ = work.retain(*work.subject)
		return fmt.Errorf("pending patch conflict retained in %s: %w; correct the owned tree and resume the same bound review", dir, err)
	}
	patch := filepath.Join(review.Round.Directory, "result.patch")
	full := filepath.Join(review.Round.Directory, drop.Subject.Operation+"-pending.patch")
	if err := os.WriteFile(full, drop.PendingPatch, 0600); err != nil {
		return fail(err)
	}
	if _, err := git(dir, "apply", "--index", "--binary", full); err != nil {
		return fail(err)
	}
	if _, err := git(dir, "apply", "--reverse", "--index", "--binary", patch); err != nil {
		return fail(err)
	}
	remaining, err := git(dir, "diff", "--cached", "--binary", "HEAD")
	if err != nil {
		return fail(err)
	}
	return os.WriteFile(filepath.Join(review.Round.Directory, drop.Subject.Operation+"-remaining.patch"), remaining, 0600)
}

func (inv *intentInvocation) installPendingRemoval(work *reviewWorkContext, check func() error) (func() error, error) {
	drop, git, owned := work.subject.Drop, inv.work().git, work.review.Record.Worktree
	if err := check(); err != nil {
		return nil, err
	}
	head, result, err := inv.unitRunner().WorktreeResult(owned)
	if err == nil && head == drop.Subject.ExpectedParent && len(drop.Covered) == 0 && drop.Subject.GateSnapshot != nil && result == drop.Subject.GateSnapshot.Tree {
		return nil, nil
	}

	if err == nil && head == drop.Subject.ExpectedParent && len(drop.Covered) > 0 && drop.CommitTree != "" {
		remaining, readErr := os.ReadFile(filepath.Join(work.review.Round.Directory, drop.Subject.Operation+"-remaining.patch"))
		current, diffErr := inv.unitRunner().WorktreeDiff(owned, head)
		if readErr == nil && diffErr == nil && bytes.Equal(current, remaining) {
			return nil, nil
		}
	}
	if err != nil || head != drop.Subject.ExpectedParent || result != drop.StartingResult {
		return nil, fmt.Errorf("the owned tree moved after proof: %v", err)
	}
	path := filepath.Join(work.review.Round.Directory, "result.patch")
	body, err := os.ReadFile(path)
	if err != nil || launch.UnitResultDigest(string(body)) != drop.PatchDigest {
		return nil, fmt.Errorf("the retained pending patch changed: %v", err)
	}
	_, stagedErr := git(owned, "apply", "--cached", "--reverse", "--check", "--binary", path)
	if stagedErr != nil {
		if _, err := git(owned, "apply", "--cached", "--check", "--binary", path); err != nil {
			return nil, fmt.Errorf("the pending patch is partly staged; preserve it in the owned tree and resume after correcting its staging: %w", err)
		}
	}
	if _, err := git(owned, "apply", "--reverse", "--binary", path); err != nil {
		return nil, err
	}
	undo := func() error {
		if stagedErr == nil {
			if _, err := git(owned, "apply", "--cached", "--binary", path); err != nil {
				return err
			}
		}
		_, err := git(owned, "apply", "--binary", path)
		return err
	}
	if stagedErr == nil {
		if _, err := git(owned, "apply", "--cached", "--reverse", "--binary", path); err != nil {
			_, restore := git(owned, "apply", "--binary", path)
			return nil, fmt.Errorf("%v; working patch restoration: %v", err, restore)
		}
	}
	return undo, nil
}
