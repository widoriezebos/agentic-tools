package steward

// The unowned-clone report (design engine-owns-disk-lifetimes Part B, 3.10;
// Round B3-3): a git clone beside the checkout that holds the checkout's
// root commit and is no armed checkout is listed with its size, for a
// person. It never acts and names no removal command: the engine offers no
// clone release.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// UnownedClones is the report's class in a person's disk pass.
type UnownedClones struct {
	GitRoot string
	Git     diskstore.WorkspaceGit
	// Armed are the git roots of the host's armed checkouts; ArmedErr is
	// why the host registry could not be read, which holds the report.
	Armed    []string
	ArmedErr error
	// Lane are the landing lane's checkout and installation, read through
	// the host lane resolver; the lane is owned, never listed. LaneErr is
	// why the lane could not be resolved, which holds the report.
	Lane    []string
	LaneErr error
}

func (UnownedClones) Name() string { return "clones beside the checkout" }

// Plan lists the siblings of the checkout that are clones of it: a
// directory with a .git directory in which the checkout's root commit
// exists, and which is no armed checkout. It reads only.
func (c UnownedClones) Plan(ctx context.Context, pass *diskstore.Pass) ([]diskstore.Item, error) {
	if c.ArmedErr != nil {
		return []diskstore.Item{{Class: c.Name(), Key: "~armed", Path: c.GitRoot, Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the host registry of armed checkouts cannot be read (" + c.ArmedErr.Error() + "), so clones are not listed", Command: "metasystem system check"}}}, nil
	}
	if c.LaneErr != nil {
		return []diskstore.Item{{Class: c.Name(), Key: "~lane", Path: c.GitRoot, Verdict: diskstore.Verdict{Decision: diskstore.Pending,
			Reason: "the landing lane cannot be resolved (" + c.LaneErr.Error() + "), so clones are not listed", Command: "metasystem landing status"}}}, nil
	}
	parent := filepath.Dir(c.GitRoot)
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, err
	}
	var candidates []string
	for _, entry := range entries {
		path := filepath.Join(parent, entry.Name())
		if !entry.IsDir() || path == c.GitRoot || containsPath(c.Armed, path) || containsPath(c.Lane, path) {
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
				bytes, _, complete := diskstore.Measure(ctx, path)
				size := fmt.Sprintf("%.1f GiB", float64(bytes)/(1<<30))
				if !complete {
					size = "at least " + size
				}
				items = append(items, diskstore.Item{Class: c.Name(), Key: path, Path: path, Bytes: bytes, Foreign: true,
					Verdict: diskstore.Verdict{Reason: "a clone of this project, " + size + ", that no store records; the engine never acts on it: remove it yourself if you no longer need it"}})
				break
			}
		}
	}
	return items, nil
}

// Apply is never reached: every item is foreign.
func (UnownedClones) Apply(context.Context, *diskstore.Pass, diskstore.Item) diskstore.Verdict {
	return diskstore.Verdict{Decision: diskstore.Keep, Reason: "the engine never acts on a clone", Command: "metasystem disk show"}
}
