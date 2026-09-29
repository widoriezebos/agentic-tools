package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
)

// workspaceGit is one test's git for the workspace verb: a worktree is a
// directory with a .git file, refs a map. It never runs git.
type workspaceGit struct {
	mu        sync.Mutex
	refs      map[string]string
	dirty     map[string]string
	worktrees map[string]string
}

func (g *workspaceGit) run(_ context.Context, dir string, args ...string) ([]byte, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch {
	case len(args) == 4 && args[0] == "rev-parse" && args[1] == "--verify":
		ref := strings.TrimSuffix(args[3], "^{commit}")
		if ref == "HEAD" {
			for branch, worktree := range g.worktrees {
				if worktree == dir {
					ref = "refs/heads/" + branch
				}
			}
		}
		if sha, ok := g.refs[ref]; ok {
			return []byte(sha + "\n"), nil
		}
		return nil, cmdGitNotFound{}
	case len(args) == 6 && args[0] == "worktree" && args[1] == "add":
		path := args[4]
		gitdir := filepath.Join(dir, ".git", "worktrees", filepath.Base(path))
		if err := os.MkdirAll(gitdir, 0o700); err != nil {
			return nil, err
		}
		if err := os.MkdirAll(path, 0o700); err != nil {
			return nil, err
		}
		g.refs["refs/heads/"+args[3]] = args[5]
		if g.worktrees == nil {
			g.worktrees = map[string]string{}
		}
		g.worktrees[args[3]] = path
		return nil, os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o600)
	case len(args) >= 3 && args[0] == "worktree" && args[1] == "remove":
		return nil, os.RemoveAll(args[len(args)-1])
	case args[0] == "status":
		return []byte(g.dirty[dir]), nil
	case args[0] == "update-ref":
		g.refs[args[1]] = args[2]
		return nil, nil
	case args[0] == "log", args[0] == "ls-files", args[0] == "for-each-ref":
		return nil, nil
	case args[0] == "merge-base":
		if args[2] == args[3] {
			return nil, nil
		}
		return nil, cmdGitNotFound{}
	case args[0] == "rev-list":
		return []byte("0\n"), nil
	case len(args) == 3 && args[0] == "branch" && args[1] == "-D":
		delete(g.refs, "refs/heads/"+args[2])
		return nil, nil
	}
	return nil, fmt.Errorf("unexpected git %q", args)
}

type workspaceVerbBed struct {
	*intentBed
	git    *workspaceGit
	person error
	census *diskstore.UseCensus
}

func newWorkspaceVerbBed(t *testing.T) *workspaceVerbBed {
	t.Helper()
	bed := &workspaceVerbBed{intentBed: newIntentBed(t, false, nil), git: &workspaceGit{refs: map[string]string{"HEAD": "0123456789abcdef0123"}, dirty: map[string]string{}},
		census: &diskstore.UseCensus{Taken: true}}
	return bed
}

func (b *workspaceVerbBed) verbOwners() intentOwners {
	owners := b.owners()
	owners.disk = diskOwners{git: b.git.run, now: func() time.Time { return diskNow }, census: func() *diskstore.UseCensus { return b.census },
		person: func(string) (string, error) { return "Wido", b.person }}
	return owners
}

func (b *workspaceVerbBed) do(args ...string) (int, intentResult) {
	b.t.Helper()
	return b.runJSON(b.verbOwners(), args...)
}

func (b *workspaceVerbBed) text(args ...string) (int, string) {
	b.t.Helper()
	code, stdout, stderr := b.run(b.verbOwners(), args...)
	return code, stdout + stderr
}

// work workspace makes a registered plain workspace for a live goal and
// prints its path and the environment to work in; the same request again
// returns it and writes nothing; --release removes it and a second
// --release is success (3.6, U6b, R-129).
func TestWorkWorkspacePlainLifecycle(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceVerbBed(t)
	code, out := bed.text("work", "workspace", "standing-validation")
	if code != 0 || !strings.Contains(out, "is ready at") || !strings.Contains(out, "export TMPDIR=") || !strings.Contains(out, "export GOCACHE=") {
		t.Fatalf("create: code=%d\n%s", code, out)
	}
	path := filepath.Join(bed.root(), "artifacts", "agents", "workspaces", "goal-standing-validation", "default")
	if _, err := os.Stat(filepath.Join(path, diskstore.MarkerName)); err != nil {
		t.Fatalf("the workspace is a marker directory at %s: %v\n%s", path, err, out)
	}
	registry := filepath.Join(bed.root(), "artifacts", "agents", "stores")
	before := snapshotFiles(t, registry)
	code, again := bed.do("work", "workspace", "standing-validation")
	if code != 0 || again.Outcome != intentUnchanged {
		t.Fatalf("repeat: %d %+v", code, again)
	}
	if after := snapshotFiles(t, registry); !equalSnapshots(before, after) {
		t.Fatal("a repeat writes nothing")
	}
	code, released := bed.do("work", "workspace", "standing-validation", "--release")
	if code != 0 || released.Outcome != intentConfirmed {
		t.Fatalf("release: %d %+v", code, released)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("released workspace remains: %v", err)
	}
	if code, twice := bed.do("work", "workspace", "standing-validation", "--release"); code != 0 || twice.Outcome != intentUnchanged {
		t.Fatalf("second release: %d %+v", code, twice)
	}
}

// --copy-of makes a worktree at the named commit; the same name at another
// revision is refused naming --release and --name; a dirty worktree is kept
// naming work land and the person's discard, which needs the enrolled
// terminal and a reason.
func TestWorkWorkspaceCopyRefusalsAndDiscard(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceVerbBed(t)
	bed.git.refs["main"] = "aaaa1111"
	bed.git.refs["other"] = "bbbb2222"
	code, made := bed.do("work", "workspace", "standing-validation", "--name", "bed", "--copy-of", "main")
	if code != 0 || made.Outcome != intentConfirmed || !strings.Contains(made.Summary, "a worktree at aaaa1111") {
		t.Fatalf("copy: %d %+v", code, made)
	}
	code, conflict := bed.do("work", "workspace", "standing-validation", "--name", "bed", "--copy-of", "other")
	if code != 2 || !strings.Contains(conflict.Summary, "--release --name bed") || !strings.Contains(conflict.Summary, "--name") {
		t.Fatalf("conflict: %d %+v", code, conflict)
	}
	if code, missing := bed.do("work", "workspace", "standing-validation", "--copy-of", "nowhere"); code != 2 || !strings.Contains(missing.Summary, "names no commit") {
		t.Fatalf("unknown revision: %d %+v", code, missing)
	}
	path := filepath.Join(bed.root(), "artifacts", "agents", "workspaces", "goal-standing-validation", "bed")
	bed.git.dirty[path] = "?? new.go\n"
	code, kept := bed.do("work", "workspace", "standing-validation", "--release", "--name", "bed")
	if code == 0 || !strings.Contains(kept.Decision, "metasystem work land standing-validation") || !strings.Contains(kept.Decision, "--release --discard --name bed") {
		t.Fatalf("dirty release: %d %+v", code, kept)
	}
	bed.person = errors.New("not the enrolled terminal")
	if code, refused := bed.do("work", "workspace", "standing-validation", "--release", "--discard", "--name", "bed", "--reason", "scratch"); code != 3 || refused.Outcome != intentRefused {
		t.Fatalf("an agent's discard: %d %+v", code, refused)
	}
	bed.person = nil
	code, discarded := bed.do("work", "workspace", "standing-validation", "--release", "--discard", "--name", "bed", "--reason", "scratch")
	if code != 0 || discarded.Outcome != intentConfirmed {
		t.Fatalf("a person's discard: %d %+v", code, discarded)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the discarded worktree is gone: %v", err)
	}
	if bed.git.refs["refs/archive/goal-standing-validation/bed/workspace/goal-standing-validation/bed"] != "aaaa1111" {
		t.Fatalf("the branch tip is archived: %v", bed.git.refs)
	}
}

// A workspace belongs to a known, open goal.
func TestWorkWorkspaceNeedsALiveGoal(t *testing.T) {
	t.Parallel()
	bed := newWorkspaceVerbBed(t)
	if code, result := bed.do("work", "workspace", "no-such-goal"); code == 0 || result.Outcome == intentConfirmed {
		t.Fatalf("unknown goal: %d %+v", code, result)
	}
	if code, result := bed.do("work", "workspace"); code != 2 {
		t.Fatalf("no goal: %d %+v", code, result)
	}
}

func snapshotFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err == nil && !entry.IsDir() {
			data, _ := os.ReadFile(path)
			files[path] = string(data)
		}
		return nil
	})
	return files
}

func equalSnapshots(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for key, value := range a {
		if b[key] != value {
			return false
		}
	}
	return true
}

func init() {
	registerIdempotency("work workspace", idemStateful,
		"the same goal and name again return the same workspace and write nothing; a release of a released or absent workspace is success and writes nothing",
		func(t *testing.T) {
			bed := newWorkspaceVerbBed(t)
			if code, _ := bed.do("work", "workspace", "standing-validation"); code != 0 {
				t.Fatal("create")
			}
			registry := filepath.Join(bed.root(), "artifacts", "agents", "stores")
			before := snapshotFiles(t, registry)
			if code, again := bed.do("work", "workspace", "standing-validation"); code != 0 || again.Outcome != intentUnchanged {
				t.Fatalf("repeat: %+v", again)
			}
			if code, _ := bed.do("work", "workspace", "standing-validation", "--release"); code != 0 {
				t.Fatal("release")
			}
			released := snapshotFiles(t, registry)
			if code, again := bed.do("work", "workspace", "standing-validation", "--release"); code != 0 || again.Outcome != intentUnchanged {
				t.Fatalf("repeat release: %+v", again)
			}
			if !equalSnapshots(released, snapshotFiles(t, registry)) || len(before) == 0 {
				t.Fatal("a repeated release wrote")
			}
		})
}

// cmdGitNotFound is a fake git's exit code 1 (a ref that does not exist).
type cmdGitNotFound struct{}

func (cmdGitNotFound) Error() string { return "exit status 1" }
func (cmdGitNotFound) ExitCode() int { return 1 }
