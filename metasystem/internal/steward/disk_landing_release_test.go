package steward

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// releaseGit is one test's git for workspaces: a worktree is a directory
// with a .git file; every commit is an ancestor of every tip.
type releaseGit struct{ refs map[string]string }

func (g *releaseGit) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	switch {
	case args[0] == "worktree" && args[1] == "add":
		path := args[4]
		gitdir := filepath.Join(dir, ".git", "worktrees", filepath.Base(path))
		if err := os.MkdirAll(gitdir, 0o700); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		g.refs["refs/heads/"+args[3]] = args[5]
		return nil, os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o600)
	case args[0] == "worktree" && args[1] == "remove":
		return nil, os.RemoveAll(args[len(args)-1])
	case args[0] == "rev-parse":
		if sha, ok := g.refs[args[len(args)-1]]; ok {
			return []byte(sha + "\n"), nil
		}
		return nil, stewardGitNotFound{}
	case args[0] == "update-ref":
		g.refs[args[1]] = args[2]
		return nil, nil
	case args[0] == "rev-list":
		return []byte("0\n"), nil
	case args[0] == "branch":
		delete(g.refs, "refs/heads/"+args[2])
		return nil, nil
	case args[0] == "status", args[0] == "merge-base", args[0] == "log", args[0] == "ls-files", args[0] == "for-each-ref":
		return nil, nil
	}
	return nil, errors.New("unexpected git " + strings.Join(args, " "))
}

// The checkout pass finishes a swept landing's unfinished release set
// (3.6: "an unfinished set is retried by the route's own recovery and by
// the sweeper"), independently of the goal's conclusion; a preview lists
// it and releases nothing; a finished set is not visited again.
func TestCheckoutPassFinishesAnUnfinishedLandingReleaseSet(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
	git := &releaseGit{refs: map[string]string{"HEAD": "base"}}
	registry := diskstore.CheckoutRegistry(root)
	workspace, err := diskstore.ObtainWorkspace(context.Background(), diskstore.WorkspaceRequest{Registry: registry, Control: root, GitRoot: root,
		Owner: diskstore.Owner{Kind: diskstore.OwnerGoal, Ref: "g"}, Name: "slice", CopyOf: "c1", Now: now, Entropy: rand.Reader, Git: git.run})
	if err != nil {
		t.Fatal(err)
	}
	landedPath := filepath.Join(root, "artifacts", "agents", "landing-intent", "g", "c1-e1", "landed.json")
	if err := os.MkdirAll(filepath.Dir(landedPath), 0o700); err != nil {
		t.Fatal(err)
	}
	record := map[string]any{"Landing": "land1", "Swept": true, "ReleaseSet": diskstore.ReleaseSet{Tip: "c1",
		Stores: []diskstore.ReleaseEntry{{ID: workspace.Record.ID, Path: workspace.Record.Path, State: diskstore.ReleasePending}}}}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(landedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	class := LandingReleaseSets{Installation: root, StateRoot: root, GitRoot: root, Git: git.run}
	pass := func(mode diskstore.Mode) diskstore.Report {
		report, err := diskstore.RunPass(context.Background(), diskstore.PassOptions{Kind: "checkout", Name: root, Registry: registry, Mode: mode,
			Now: now, Clock: func() time.Time { return now }, Classes: []diskstore.Class{class},
			CensusReader: &diskstore.CensusReader{Pids: func() ([]int64, error) { return nil, nil }}})
		if err != nil {
			t.Fatal(err)
		}
		return report
	}
	if report := pass(diskstore.ModeReport); len(report.Planned) != 1 {
		t.Fatalf("a report pass lists the unfinished set: %+v", report)
	}
	if _, err := os.Stat(workspace.Record.Path); err != nil {
		t.Fatalf("a report pass releases nothing: %v", err)
	}
	if report := pass(diskstore.ModeApply); len(report.Actions) != 1 {
		t.Fatalf("the apply pass finishes the set: %+v", report)
	}
	if _, err := os.Stat(workspace.Record.Path); !os.IsNotExist(err) {
		t.Fatalf("the recorded workspace is released: %v", err)
	}
	var after struct{ ReleaseSet diskstore.ReleaseSet }
	written, _ := os.ReadFile(landedPath)
	if json.Unmarshal(written, &after) != nil || !after.ReleaseSet.Finished() || after.ReleaseSet.Stores[0].State != diskstore.ReleaseReleased {
		t.Fatalf("the landing record says released: %s", written)
	}
	if report := pass(diskstore.ModeApply); len(report.Actions) != 0 || len(report.Pending) != 0 {
		t.Fatalf("a finished set is not visited again: %+v", report)
	}
}

// stewardGitNotFound is a fake git's exit code 1 (a ref that does not exist).
type stewardGitNotFound struct{}

func (stewardGitNotFound) Error() string { return "exit status 1" }
func (stewardGitNotFound) ExitCode() int { return 1 }
