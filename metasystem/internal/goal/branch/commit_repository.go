package branch

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

type commitWorktree struct{ Path, Branch string }

type CommitWorktree = commitWorktree

type commitFacts struct {
	Tip       func(repo, ref string) (string, bool, error)
	Head      func(repo string) (string, error)
	Tree      func(repo string) (string, error)
	Index     func(repo string) (string, error)
	HeadRef   func(repo string) string
	Staged    func(repo string) ([]string, error)
	Unstaged  func(repo string) ([]string, error)
	Patch     func(repo string) ([]byte, error)
	Range     func(repo, endpoint, tip, goal string) ([]Commit, error)
	Ancestor  func(repo, older, newer string) (bool, error)
	Suffix    func(repo, target, tip string) ([]string, error)
	Kind      func(repo, commit, goal string) (KindInfo, error)
	Entries   func(repo, commit string) ([]Entry, error)
	Changes   func(repo, index, tip string) ([]string, error)
	Worktrees func(repo string) ([]commitWorktree, error)
}

type CommitFacts = commitFacts

func (f commitFacts) complete() bool {
	return f.Tip != nil && f.Head != nil && f.Tree != nil && f.Index != nil && f.HeadRef != nil && f.Staged != nil && f.Unstaged != nil && f.Patch != nil && f.Range != nil && f.Ancestor != nil && f.Suffix != nil && f.Kind != nil && f.Entries != nil && f.Changes != nil && f.Worktrees != nil
}

type commitEffects struct {
	ClearFetch   func(repo, ref string) error
	Open         func(repo, base string, amend bool) (dir string, close func(), err error)
	Apply        func(dir string, patch []byte) error
	Commit       func(dir, subject, trailer string, amend bool) error
	Replay       func(dir, commit string) error
	WithoutPaths func(repo, tree string, paths []string) (string, error)
	Checkout     func(repo, before, after string) error
	Attach       func(repo, ref string) error
	Restore      func(repo, ref, commit string) error
	Publish      func(repo, goal, old, next, origin string) error
}

type CommitEffects = commitEffects

func (e commitEffects) complete() bool {
	return e.ClearFetch != nil && e.Open != nil && e.Apply != nil && e.Commit != nil && e.Replay != nil && e.WithoutPaths != nil && e.Checkout != nil && e.Attach != nil && e.Restore != nil && e.Publish != nil
}

type commitRepository struct {
	facts   commitFacts
	effects commitEffects
}

func gitCommitRepository() commitRepository {
	value := func(repo string, args ...string) (string, error) {
		out, err := gitOutput(repo, args...)
		return strings.TrimSpace(string(out)), err
	}
	names := func(repo string, args ...string) ([]string, error) {
		out, err := gitOutput(repo, args...)
		if err != nil {
			return nil, err
		}
		var result []string
		for _, item := range bytes.Split(out, []byte{0}) {
			if len(item) != 0 {
				result = append(result, string(item))
			}
		}
		return result, nil
	}
	return commitRepository{
		facts: commitFacts{
			Tip:   localBranchTip,
			Head:  func(repo string) (string, error) { return value(repo, "rev-parse", "HEAD^{commit}") },
			Tree:  func(repo string) (string, error) { return value(repo, "rev-parse", "HEAD^{tree}") },
			Index: func(repo string) (string, error) { return value(repo, "write-tree") },
			HeadRef: func(repo string) string {
				ref, _ := value(repo, "symbolic-ref", "-q", "HEAD")
				return ref
			},
			Staged:   stagedPaths,
			Unstaged: func(repo string) ([]string, error) { return names(repo, "diff", "--name-only", "-z") },
			Patch: func(repo string) ([]byte, error) {
				return gitOutput(repo, "diff", "--cached", "--binary", "--full-index")
			},
			Range:    ValidateRange,
			Ancestor: ancestor,
			Suffix: func(repo, target, tip string) ([]string, error) {
				out, err := value(repo, "rev-list", "--first-parent", "--reverse", target+".."+tip)
				return strings.Fields(out), err
			},
			Kind:    KindOf,
			Entries: RawEntries,
			Changes: func(repo, index, tip string) ([]string, error) {
				return names(repo, "diff", "--name-only", "-z", "--no-renames", index, tip)
			},
			Worktrees: func(repo string) ([]commitWorktree, error) {
				out, err := value(repo, "worktree", "list", "--porcelain")
				if err != nil {
					return nil, err
				}
				var result []commitWorktree
				for _, record := range strings.Split(out, "\n\n") {
					item := commitWorktree{}
					for _, line := range strings.Split(record, "\n") {
						if strings.HasPrefix(line, "worktree ") {
							item.Path = strings.TrimPrefix(line, "worktree ")
						}
						if strings.HasPrefix(line, "branch ") {
							item.Branch = strings.TrimPrefix(line, "branch ")
						}
					}
					result = append(result, item)
				}
				return result, nil
			},
		},
		effects: commitEffects{
			ClearFetch: clearPushTxn,
			Open: func(repo, base string, amend bool) (string, func(), error) {
				pattern := "goal-branch-adopt-*"
				if amend {
					pattern = "goal-branch-amend-*"
				}
				scratch, err := os.MkdirTemp("", pattern)
				if err != nil {
					return "", nil, err
				}
				worktree := filepath.Join(scratch, "worktree")
				if _, err := gitOutput(repo, "worktree", "add", "--quiet", "--detach", worktree, base); err != nil {
					_ = os.RemoveAll(scratch)
					return "", nil, err
				}
				return worktree, func() {
					_, _ = gitOutput(repo, "worktree", "remove", "--force", worktree)
					_ = os.RemoveAll(scratch)
				}, nil
			},
			Apply: func(dir string, patch []byte) error {
				_, err := gitInput(dir, patch, "apply", "--index", "--3way", "-")
				return err
			},
			Commit: func(dir, subject, trailer string, amend bool) error {
				args := []string{"commit", "--quiet"}
				if amend {
					args = append(args, "--amend")
				}
				_, err := gitOutput(dir, append(args, "-m", subject, "-m", trailer)...)
				return err
			},
			Replay: func(dir, commit string) error {
				_, err := gitOutput(dir, "cherry-pick", "--quiet", commit)
				return err
			},
			WithoutPaths: treeWithoutPaths,
			Checkout: func(repo, before, after string) error {
				_, err := gitOutput(repo, "read-tree", "-m", "-u", before, after)
				return err
			},
			Attach: func(repo, ref string) error {
				_, err := gitOutput(repo, "symbolic-ref", "HEAD", ref)
				return err
			},
			Restore: restoreHead,
			Publish: updateBranchAndOrigin,
		},
	}
}
