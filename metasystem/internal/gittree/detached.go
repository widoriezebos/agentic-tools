package gittree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// DetachedWorktree is a temporary linked worktree whose workspace subtree
// has been replaced by one exact tree. The rest of a nested repository stays
// at the detached HEAD used to create the worktree.
type DetachedWorktree struct {
	control    Workspace
	parent     string
	top        string
	root       string
	registered bool
	closed     bool
}

// NewDetachedWorktree creates a detached worktree under the system temporary
// directory and checks out tree in the caller's workspace-relative path
// space. Close removes both the linked worktree and its administrative entry.
func (w Workspace) NewDetachedWorktree(tree string) (_ *DetachedWorktree, err error) {
	if !treeID.MatchString(tree) {
		return nil, fmt.Errorf("gittree detached worktree: %q is not a tree id", tree)
	}
	top, err := w.topLevel()
	if err != nil {
		return nil, err
	}
	prefix, err := w.treePrefix()
	if err != nil {
		return nil, err
	}
	rawParent, err := os.MkdirTemp("", "metasystem-landing-receipt.")
	if err != nil {
		return nil, fmt.Errorf("gittree detached worktree: %w", err)
	}
	parent, err := filepath.Abs(rawParent)
	if err == nil {
		parent, err = filepath.EvalSymlinks(parent)
	}
	if err != nil {
		cleanupErr := os.RemoveAll(rawParent)
		if cleanupErr != nil {
			cleanupErr = fmt.Errorf("remove temporary worktree directory: %w", cleanupErr)
		}
		return nil, errors.Join(fmt.Errorf("gittree detached worktree: resolve temporary directory: %w", err), cleanupErr)
	}
	// The worktree's basename is unique per receipt: git names the admin
	// entry under .git/worktrees after it, and two concurrent adds with the
	// same basename against one repository race on each other's half-written
	// entry ("failed to read .git/worktrees/worktree/commondir", cadence run
	// 18, 2026-09-12, two proof groups of one receipt on a busy box).
	detached := &DetachedWorktree{
		control: Workspace{Dir: top},
		parent:  parent,
		top:     filepath.Join(parent, "worktree-"+strings.TrimPrefix(filepath.Base(parent), "metasystem-landing-receipt.")),
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, detached.Close())
		}
	}()

	admin, err := detached.control.lockWorktreeAdmin(true)
	if err != nil {
		return nil, err
	}
	_, err = detached.control.git(nil, "worktree", "add", "--detach", detached.top, "HEAD")
	if releaseErr := admin.release(); releaseErr != nil {
		err = errors.Join(err, releaseErr)
	}
	if err != nil {
		return nil, fmt.Errorf("gittree detached worktree: add: %w", err)
	}
	detached.registered = true

	candidateTop := tree
	if prefix != "" {
		candidateTop, err = detached.graftSubtree(prefix, tree)
		if err != nil {
			return nil, err
		}
	}
	worktree := Workspace{Dir: detached.top}
	if _, err = worktree.git(nil, "read-tree", "--reset", "-u", candidateTop); err != nil {
		return nil, fmt.Errorf("gittree detached worktree: checkout candidate: %w", err)
	}
	baseCommit, err := worktree.gitLine(nil, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil || !treeID.MatchString(baseCommit) {
		return nil, fmt.Errorf("gittree detached worktree: resolve base commit: %w", err)
	}
	candidateCommit, err := worktree.gitLine(nil,
		"-c", "user.name=MetaSystem",
		"-c", "user.email=metasystem@invalid",
		"commit-tree", candidateTop, "-p", baseCommit, "-m", "temporary candidate snapshot")
	if err != nil || !treeID.MatchString(candidateCommit) {
		return nil, fmt.Errorf("gittree detached worktree: create candidate commit: %w", err)
	}
	if _, err = worktree.git(nil, "update-ref", "--no-deref", "HEAD", candidateCommit, baseCommit); err != nil {
		return nil, fmt.Errorf("gittree detached worktree: bind detached HEAD to candidate: %w", err)
	}
	headTree, err := worktree.gitLine(nil, "rev-parse", "HEAD^{tree}")
	if err != nil || headTree != candidateTop {
		return nil, fmt.Errorf("gittree detached worktree: candidate HEAD tree mismatch: got %q want %q: %w", headTree, candidateTop, err)
	}
	indexTree, err := worktree.gitLine(nil, "write-tree")
	if err != nil || indexTree != candidateTop {
		return nil, fmt.Errorf("gittree detached worktree: candidate index tree mismatch: got %q want %q: %w", indexTree, candidateTop, err)
	}
	if _, err = worktree.git(nil, "diff-index", "--quiet", "HEAD", "--"); err != nil {
		return nil, fmt.Errorf("gittree detached worktree: candidate projection is dirty: %w", err)
	}
	detached.root = detached.top
	if prefix != "" {
		detached.root = filepath.Join(detached.top, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
		if err = os.MkdirAll(detached.root, 0o755); err != nil {
			return nil, fmt.Errorf("gittree detached worktree: create workspace root: %w", err)
		}
	}
	return detached, nil
}

// worktreeAdminLock serializes git's worktree administration in one
// repository. `git worktree add` writes .git/worktrees/<name> in steps (the
// directory first, commondir and gitdir after it), and a second add that
// enumerates the worktrees meanwhile dies on the half-written neighbour:
// "failed to read .git/worktrees/<name>/commondir: Undefined error: 0".
// Unique basenames moved that collision from the entry itself to its
// neighbour (the adoption bed's nested proof under a deep attempt,
// 2026-09-12). The lock is a file in the common git dir, so every engine
// on the repository serializes its adds and removes, within one process
// and across processes alike.
type worktreeAdminLock struct{ file *os.File }

func (w Workspace) lockWorktreeAdmin(blocking bool) (*worktreeAdminLock, error) {
	common, err := w.gitPathLine(nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("gittree worktree administration: %w", err)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(w.Dir, common)
	}
	file, err := os.OpenFile(filepath.Join(common, "metasystem-worktree-admin.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("gittree worktree administration: %w", err)
	}
	how := unix.LOCK_EX
	if !blocking {
		how |= unix.LOCK_NB
	}
	if err := unix.Flock(int(file.Fd()), how); err != nil {
		_ = file.Close()
		return nil, err
	}
	return &worktreeAdminLock{file: file}, nil
}

func (l *worktreeAdminLock) release() error {
	if l == nil || l.file == nil {
		return nil
	}
	err := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	return errors.Join(err, closeErr)
}

func (d *DetachedWorktree) graftSubtree(prefix, tree string) (string, error) {
	env, cleanup, err := isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	worktree := Workspace{Dir: d.top}
	if _, err := worktree.git(env, "read-tree", "HEAD"); err != nil {
		return "", fmt.Errorf("gittree detached worktree: seed candidate: %w", err)
	}
	path := strings.TrimSuffix(prefix, "/")
	if _, err := worktree.git(env, "rm", "-r", "--cached", "-f", "--ignore-unmatch", "--", path); err != nil {
		return "", fmt.Errorf("gittree detached worktree: replace workspace subtree: %w", err)
	}
	if _, err := worktree.git(env, "read-tree", "--prefix="+prefix, tree); err != nil {
		return "", fmt.Errorf("gittree detached worktree: graft candidate: %w", err)
	}
	candidateTop, err := worktree.gitLine(env, "write-tree")
	if err != nil {
		return "", fmt.Errorf("gittree detached worktree: write candidate: %w", err)
	}
	if !treeID.MatchString(candidateTop) {
		return "", fmt.Errorf("gittree detached worktree: write-tree returned %q", candidateTop)
	}
	return candidateTop, nil
}

// Workspace returns the candidate's workspace-relative root inside the
// detached worktree.
func (d *DetachedWorktree) Workspace() Workspace {
	return Workspace{Dir: d.root}
}

// Close removes the temporary worktree even when its command dirtied or
// locked it, then removes its temporary directory. It never runs git worktree
// prune because that repository-wide command can erase registrations for
// unrelated worktrees whose directories are temporarily missing.
func (d *DetachedWorktree) Close() error {
	if d == nil || d.closed {
		return nil
	}
	d.closed = true
	var cleanupErrs []error
	if d.registered {
		admin, err := d.control.lockWorktreeAdmin(true)
		if err != nil {
			cleanupErrs = append(cleanupErrs, err)
		} else {
			_, err = d.control.git(nil, "worktree", "remove", "--force", "--force", d.top)
			if releaseErr := admin.release(); releaseErr != nil {
				cleanupErrs = append(cleanupErrs, releaseErr)
			}
			if err != nil {
				cleanupErrs = append(cleanupErrs, fmt.Errorf("remove linked worktree: %w", err))
			}
		}
	}
	if err := os.RemoveAll(d.parent); err != nil {
		cleanupErrs = append(cleanupErrs, fmt.Errorf("remove temporary worktree directory: %w", err))
	}
	return errors.Join(cleanupErrs...)
}
