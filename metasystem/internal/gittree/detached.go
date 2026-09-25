package gittree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/contractgit"
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
	afterClose func(error) error
	// common is set for a planned worktree: its admin entry must be gone
	// before the recorded tuple may read closed.
	common string
}

// RegisteredDetachedWorktreeOf checks one linked worktree's reciprocal Git
// administrative links. It does not enumerate other worktrees: a sibling may
// still be midway through git worktree add while this one is in use.
func (w Workspace) RegisteredDetachedWorktreeOf(base Workspace) bool {
	sectionTop, err := w.topLevel()
	if err != nil {
		return false
	}
	common, err := base.gitPathLine(nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return false
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(base.Dir, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return false
	}
	gitFile := filepath.Join(sectionTop, ".git")
	if info, statErr := os.Lstat(gitFile); statErr != nil || !info.Mode().IsRegular() {
		return false
	}
	link, err := os.ReadFile(gitFile)
	if err != nil || !strings.HasPrefix(string(link), "gitdir: ") {
		return false
	}
	admin := strings.TrimSuffix(strings.TrimPrefix(string(link), "gitdir: "), "\n")
	if !filepath.IsAbs(admin) {
		admin = filepath.Join(sectionTop, admin)
	}
	admin, err = filepath.EvalSymlinks(admin)
	if err != nil || admin != filepath.Join(common, "worktrees", filepath.Base(sectionTop)) {
		return false
	}
	back, err := os.ReadFile(filepath.Join(admin, "gitdir"))
	if err != nil {
		return false
	}
	backPath := strings.TrimSuffix(string(back), "\n")
	if !filepath.IsAbs(backPath) {
		backPath = filepath.Join(admin, backPath)
	}
	backPath, err = filepath.EvalSymlinks(backPath)
	if err != nil || backPath != gitFile {
		return false
	}
	commonLink, err := os.ReadFile(filepath.Join(admin, "commondir"))
	if err != nil {
		return false
	}
	linkedCommon := strings.TrimSuffix(string(commonLink), "\n")
	if !filepath.IsAbs(linkedCommon) {
		linkedCommon = filepath.Join(admin, linkedCommon)
	}
	linkedCommon, err = filepath.EvalSymlinks(linkedCommon)
	if err != nil || linkedCommon != common {
		return false
	}
	_, detached, err := w.SymbolicHead()
	return err == nil && detached
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
	parent, err := temporaryWorktreeParent("", "metasystem-landing-receipt.")
	if err != nil {
		return nil, err
	}
	// The worktree's basename is unique per receipt: git names the admin
	// entry under .git/worktrees after it, and two concurrent adds with the
	// same basename against one repository race on each other's half-written
	// entry ("failed to read .git/worktrees/worktree/commondir", cadence run
	// 18, 2026-09-12, two proof groups of one receipt on a busy box).
	detached := &DetachedWorktree{
		control: w.at(top),
		parent:  parent,
		top:     filepath.Join(parent, "worktree-"+strings.TrimPrefix(filepath.Base(parent), "metasystem-landing-receipt.")),
	}
	return detached.materialize(w, tree, prefix)
}

func temporaryWorktreeParent(dir, pattern string) (string, error) {
	rawParent, err := os.MkdirTemp(dir, pattern)
	if err != nil {
		return "", fmt.Errorf("gittree detached worktree: %w", err)
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
		return "", errors.Join(fmt.Errorf("gittree detached worktree: resolve temporary directory: %w", err), cleanupErr)
	}
	return parent, nil
}

// WorktreeTuple names one linked worktree exactly: its private parent, its
// top, the control checkout and the repository's common Git directory.
type WorktreeTuple struct {
	Parent, Top, Control, Common string
}

// WorktreePlan is a reserved, not yet created, detached worktree. The caller
// records the tuple before Create runs git worktree add, so an add killed
// midway is recoverable through the record alone.
type WorktreePlan struct {
	WorktreeTuple
	// AfterClose runs after the created worktree's Close with its result.
	AfterClose func(error) error
	workspace  Workspace
	prefix     string
}

// PlanDetachedWorktreeIn allocates a private parent under dir and derives
// the tuple without touching Git state.
func (w Workspace) PlanDetachedWorktreeIn(dir string) (*WorktreePlan, error) {
	top, err := w.topLevel()
	if err != nil {
		return nil, err
	}
	prefix, err := w.treePrefix()
	if err != nil {
		return nil, err
	}
	common, err := w.gitPathLine(nil, "rev-parse", "--git-common-dir")
	if err != nil {
		return nil, fmt.Errorf("gittree detached worktree: common dir: %w", err)
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(w.Dir, common)
	}
	if common, err = filepath.EvalSymlinks(common); err != nil {
		return nil, fmt.Errorf("gittree detached worktree: common dir: %w", err)
	}
	parent, err := temporaryWorktreeParent(dir, "wt-")
	if err != nil {
		return nil, err
	}
	return &WorktreePlan{
		WorktreeTuple: WorktreeTuple{Parent: parent, Top: filepath.Join(parent, "worktree-"+strings.TrimPrefix(filepath.Base(parent), "wt-")),
			Control: top, Common: common},
		workspace: w, prefix: prefix,
	}, nil
}

// Create materializes tree in the planned worktree, as NewDetachedWorktree.
func (p *WorktreePlan) Create(tree string) (*DetachedWorktree, error) {
	if !treeID.MatchString(tree) {
		return nil, fmt.Errorf("gittree detached worktree: %q is not a tree id", tree)
	}
	detached := &DetachedWorktree{control: p.workspace.at(p.Control), parent: p.Parent, top: p.Top, afterClose: p.AfterClose, common: p.Common}
	return detached.materialize(p.workspace, tree, p.prefix)
}

func (detached *DetachedWorktree) materialize(w Workspace, tree, prefix string) (_ *DetachedWorktree, err error) {
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
	worktree := detached.control.at(detached.top)
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

// NewDetachedCommitWorktree checks out commit in a temporary linked
// worktree, preserving the caller's workspace-relative root.
func (w Workspace) NewDetachedCommitWorktree(commit string) (_ *DetachedWorktree, err error) {
	if !treeID.MatchString(commit) {
		return nil, fmt.Errorf("gittree detached worktree: %q is not a commit id", commit)
	}
	top, err := w.topLevel()
	if err != nil {
		return nil, err
	}
	prefix, err := w.treePrefix()
	if err != nil {
		return nil, err
	}
	rawParent, err := os.MkdirTemp("", "metasystem-landing-advance.")
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
	detached := &DetachedWorktree{
		control: w.at(top),
		parent:  parent,
		top:     filepath.Join(parent, "worktree-"+strings.TrimPrefix(filepath.Base(parent), "metasystem-landing-advance.")),
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
	stdout, stderr, code, probeErr := detached.control.gitProbe(top, nil, nil,
		"worktree", "add", "--detach", detached.top, commit)
	releaseErr := admin.release()
	if probeErr != nil {
		return nil, errors.Join(probeErr, releaseErr)
	}
	if code != 0 {
		return nil, errors.Join(fmt.Errorf("gittree detached worktree: add: %w", answerErr("worktree add", stderr, stdout)), releaseErr)
	}
	if releaseErr != nil {
		return nil, releaseErr
	}
	detached.registered = true
	detached.root = detached.top
	if prefix != "" {
		detached.root = filepath.Join(detached.top, filepath.FromSlash(strings.TrimSuffix(prefix, "/")))
	}
	return detached, nil
}

// RebaseResult is the typed outcome of rebasing a detached worktree.
type RebaseResult struct {
	Head       string
	Conflicted bool
	Output     string
}

// Rebase rebases this detached worktree and aborts before returning a
// conflicted answer.
func (d *DetachedWorktree) Rebase(upstream string, gitArgs ...string) (RebaseResult, error) {
	if len(gitArgs) == 0 {
		return RebaseResult{}, &contractgit.Refusal{Code: contractgit.DriverUnresolvedCode, Detail: "detached rebase has no metasystem merge-driver arguments; run the verb from an installed metasystem binary"}
	}
	workspace := d.control.at(d.root)
	args := append(append([]string{}, gitArgs...), "rebase", upstream)
	stdout, stderr, code, err := workspace.gitProbe(d.top, nil, nil, args...)
	if err != nil {
		return RebaseResult{}, err
	}
	if code != 0 {
		result := RebaseResult{Conflicted: true, Output: combinedProbeOutput(stderr, stdout)}
		abortOut, abortErrOut, abortCode, abortErr := workspace.gitProbe(d.top, nil, nil, "rebase", "--abort")
		if abortErr != nil {
			return result, errors.Join(fmt.Errorf("git rebase conflicted: %s", result.Output), abortErr)
		}
		if abortCode != 0 {
			return result, errors.Join(
				fmt.Errorf("git rebase conflicted: %s", result.Output),
				answerErr("rebase --abort", abortErrOut, abortOut),
			)
		}
		return result, nil
	}
	head, unborn, err := workspace.HeadCommit()
	if err != nil {
		return RebaseResult{}, err
	}
	if unborn {
		return RebaseResult{}, fmt.Errorf("gittree detached worktree: rebase left HEAD unborn")
	}
	return RebaseResult{Head: head}, nil
}

func combinedProbeOutput(stderr, stdout string) string {
	parts := []string{}
	if detail := strings.TrimSpace(stderr); detail != "" {
		parts = append(parts, detail)
	}
	if detail := strings.TrimSpace(stdout); detail != "" {
		parts = append(parts, detail)
	}
	return strings.Join(parts, "\n")
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
	return lockWorktreeAdminAt(common, blocking)
}

func lockWorktreeAdminAt(common string, blocking bool) (*worktreeAdminLock, error) {
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
	env, cleanup, err := d.control.isolatedIndex()
	if err != nil {
		return "", err
	}
	defer cleanup()
	worktree := d.control.at(d.top)
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
	return d.control.at(d.root)
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
	if !d.registered && d.common != "" {
		// A failed add may have written its admin entry before it failed;
		// only the recorded tuple may resolve that registration.
		if _, err := os.Lstat(filepath.Join(d.common, "worktrees", filepath.Base(d.top))); !errors.Is(err, os.ErrNotExist) {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("worktree registration %s unresolved: %v", filepath.Base(d.top), err))
		}
	}
	if len(cleanupErrs) == 0 || d.afterClose == nil {
		// A recorded worktree whose registration survived keeps its parent
		// for recovery through the recorded tuple.
		if err := os.RemoveAll(d.parent); err != nil {
			cleanupErrs = append(cleanupErrs, fmt.Errorf("remove temporary worktree directory: %w", err))
		}
	}
	err := errors.Join(cleanupErrs...)
	if d.afterClose != nil {
		err = errors.Join(err, d.afterClose(err))
	}
	return err
}

// Recorded worktree recovery outcomes.
const (
	WorktreeRemoved = "removed"
	WorktreeRefused = "refused"
	WorktreePending = "pending"
)

// RemoveRecordedWorktree removes one recorded tuple under the repository's
// worktree admin lock, classifying it only by the reciprocal links of its
// exact admin entry `<common>/worktrees/<basename(top)>`. It never runs
// worktree prune and never touches another entry. control supplies the Git
// plumbing (RawSource) for the one worktree remove it may run.
func RemoveRecordedWorktree(tuple WorktreeTuple, control Workspace) (string, error) {
	if !filepath.IsAbs(tuple.Parent) || !filepath.IsAbs(tuple.Top) || !filepath.IsAbs(tuple.Common) ||
		filepath.Dir(tuple.Top) != tuple.Parent || tuple.Control == "" {
		return WorktreePending, fmt.Errorf("gittree recorded worktree: tuple %+v is not exact", tuple)
	}
	admin, err := lockWorktreeAdminAt(tuple.Common, true)
	if err != nil {
		return WorktreePending, err
	}
	defer admin.release()
	entry := filepath.Join(tuple.Common, "worktrees", filepath.Base(tuple.Top))
	gitFile := filepath.Join(tuple.Top, ".git")
	gitInfo, gitErr := os.Lstat(gitFile)
	entryInfo, entryErr := os.Lstat(entry)
	if gitErr != nil && !errors.Is(gitErr, os.ErrNotExist) || entryErr != nil && !errors.Is(entryErr, os.ErrNotExist) {
		return WorktreePending, errors.Join(gitErr, entryErr)
	}
	readLink := func(path, prefix string) (string, bool) {
		data, err := os.ReadFile(path)
		if err != nil || !strings.HasPrefix(string(data), prefix) {
			return "", false
		}
		value := strings.TrimSuffix(strings.TrimPrefix(string(data), prefix), "\n")
		if !filepath.IsAbs(value) {
			value = filepath.Join(filepath.Dir(path), value)
		}
		return filepath.Clean(value), true
	}
	removeParent := func() (string, error) {
		if err := os.RemoveAll(tuple.Parent); err != nil {
			return WorktreePending, err
		}
		return WorktreeRemoved, nil
	}
	switch {
	case gitErr == nil:
		if !gitInfo.Mode().IsRegular() || entryErr != nil || !entryInfo.IsDir() {
			return WorktreeRefused, fmt.Errorf("gittree recorded worktree: %s is not linked to %s", gitFile, entry)
		}
		admin, ok := readLink(gitFile, "gitdir: ")
		back, backOK := readLink(filepath.Join(entry, "gitdir"), "")
		common, commonOK := readLink(filepath.Join(entry, "commondir"), "")
		if resolved, err := filepath.EvalSymlinks(common); err == nil {
			common = resolved
		}
		if !ok || !backOK || !commonOK || admin != entry || back != gitFile || common != tuple.Common {
			return WorktreeRefused, fmt.Errorf("gittree recorded worktree: %s and %s are not reciprocal", gitFile, entry)
		}
		if _, err := control.at(tuple.Control).git(nil, "worktree", "remove", "--force", "--force", tuple.Top); err != nil {
			return WorktreePending, fmt.Errorf("remove linked worktree: %w", err)
		}
		return removeParent()
	case entryErr != nil:
		// Neither side registered, or the add never began.
		return removeParent()
	default:
		// An interrupted add: the entry exists, the checkout's .git does not.
		if back, ok := readLink(filepath.Join(entry, "gitdir"), ""); ok && back != gitFile {
			return WorktreeRefused, fmt.Errorf("gittree recorded worktree: %s names %s", entry, back)
		} else if !ok {
			if _, err := os.Lstat(filepath.Join(entry, "gitdir")); err == nil {
				return WorktreeRefused, fmt.Errorf("gittree recorded worktree: %s is unreadable", entry)
			}
		}
		if err := os.RemoveAll(entry); err != nil {
			return WorktreePending, err
		}
		return removeParent()
	}
}
