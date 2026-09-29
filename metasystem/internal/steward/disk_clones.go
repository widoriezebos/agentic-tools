package steward

// The unowned-clone recognizer (design engine-owns-disk-lifetimes Part B,
// 3.10): a git clone beside the checkout that holds the checkout's root
// commit and that no store records is reported with the one act that
// releases it, a person's work workspace --release --path. Machinery never
// acts on it.

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// UnownedClones is the recognizer's class in a checkout pass.
type UnownedClones struct {
	GitRoot string
	Git     diskstore.WorkspaceGit
	// Armed are the git roots of the host's armed checkouts: another
	// seat's checkout is never reported as a clone to remove.
	Armed []string
}

func (UnownedClones) Name() string { return "clones beside the checkout" }

// Plan lists the siblings of the checkout that are clones of it: a
// directory with a .git directory (not a linked worktree's .git file) in
// which the checkout's root commits exist, and which is no armed
// checkout. It reads only.
func (c UnownedClones) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	parent := filepath.Dir(c.GitRoot)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		if !entry.IsDir() || path == c.GitRoot || containsPath(c.Armed, path) {
			continue
		}
		if info, err := os.Lstat(filepath.Join(path, ".git")); err == nil && info.IsDir() {
			candidates = append(candidates, path)
		}
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	out, err := c.Git(ctx, c.GitRoot, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return []diskstore.Item{{Class: c.Name(), Key: "~roots", Path: c.GitRoot, Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the checkout's root commit cannot be read, so its clones are not listed: " + err.Error(), Command: "git -C " + c.GitRoot + " rev-list --max-parents=0 HEAD"}}}, nil
	}
	roots := strings.Fields(string(out))
	var items []diskstore.Item
	for _, path := range candidates {
		if ctx.Err() != nil {
			break
		}
		for _, root := range roots {
			if _, err := c.Git(ctx, path, "cat-file", "-e", root+"^{commit}"); err == nil {
				items = append(items, diskstore.Item{Class: c.Name(), Key: path, Path: path, Verdict: diskstore.Verdict{Decision: diskstore.Keep,
					Reason:  "a clone of this project that no store records; machinery never removes it",
					Command: "a person archives and removes it with metasystem work workspace --release --path <the clone>"}})
				break
			}
		}
	}
	return items, nil
}

func (UnownedClones) Apply(context.Context, *diskstore.Pass, diskstore.Item) diskstore.Verdict {
	return diskstore.Verdict{Decision: diskstore.Keep, Reason: "an unowned clone is released only by a person", Command: "metasystem work workspace --release --path <the clone>"}
}
