package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// fakeGit answers rev-parse queries from a table keyed by directory; any
// other query refuses.
func fakeGit(answers map[string]map[string]string) GitQuery {
	return func(dir string, args ...string) (string, bool) {
		if len(args) != 2 || args[0] != "rev-parse" {
			return "", false
		}
		value, ok := answers[dir][args[1]]
		return value, ok
	}
}

// TestJobBuildCacheEnv is the delegate cache contract (disk-lifetimes A7):
// every job, worktree or shared checkout, exports the one machine delegate
// cache pair (created), a dispatcher-made job worktree adds a chain-private
// GOTMPDIR under its git dir, and the recorded path names the delegate
// cache for every job. Nothing lands in the chain's git dir but GOTMPDIR.
func TestJobBuildCacheEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	agents := filepath.Join(root, "artifacts", "agents")
	worktree := filepath.Join(agents, "worktrees", "chain-a")
	seat := filepath.Join(root, "seat")
	gitdir := filepath.Join(root, ".git", "worktrees", "chain-a")
	for _, dir := range []string{worktree, seat, gitdir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	caches := gocache.Paths{GoCache: filepath.Join(root, "user-cache", "metasystem-delegate-go-build"), StaticcheckCache: filepath.Join(root, "user-cache", "metasystem-delegate-staticcheck")}
	git := fakeGit(map[string]map[string]string{
		worktree: {"--absolute-git-dir": gitdir},
		seat:     {"--absolute-git-dir": filepath.Join(root, ".git", "worktrees", "seat")},
		root:     {"--absolute-git-dir": filepath.Join(root, ".git")},
	})
	env := jobBuildCacheEnv(git, agents, worktree, caches)
	want := []string{
		"GOCACHE=" + caches.GoCache,
		"STATICCHECK_CACHE=" + caches.StaticcheckCache,
		"GOTMPDIR=" + filepath.Join(gitdir, "metasystem-go-tmp"),
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Fatalf("job worktree cache env = %v, want %v", env, want)
	}
	for _, dir := range []string{caches.GoCache, caches.StaticcheckCache, filepath.Join(gitdir, "metasystem-go-tmp")} {
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("cache directory %s was not created: %v", dir, err)
		}
	}
	if _, err := os.Stat(filepath.Join(gitdir, "metasystem-build-cache")); !os.IsNotExist(err) {
		t.Fatalf("a per-chain build cache was made: %v", err)
	}
	for _, workspace := range []string{root, seat} {
		env := jobBuildCacheEnv(git, agents, workspace, caches)
		if strings.Join(env, "\n") != strings.Join(want[:2], "\n") {
			t.Fatalf("%s: a job outside the job worktrees = %v, want the delegate cache without GOTMPDIR", workspace, env)
		}
	}
	if env := jobBuildCacheEnv(git, agents, worktree, gocache.Paths{}); strings.Join(env, "\n") != want[2] {
		t.Fatalf("an unresolved delegate cache exported %v", env)
	}
	round := filepath.Join(realRoot, "round")
	if err := os.MkdirAll(round, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, workspace := range []string{worktree, root} {
		recordBuildCachePath(git, agents, workspace, round, caches)
		if data, _ := os.ReadFile(filepath.Join(round, "build-cache.txt")); string(data) != caches.GoCache+"\n" {
			t.Fatalf("%s: recorded cache = %q", workspace, data)
		}
	}
}

// TestJobGitQuarantineEnv routes object writes into the worktree's private
// quarantine store and disables automatic maintenance; a workspace without a
// quarantine store gets nothing.
func TestJobGitQuarantineEnv(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	gitdir := filepath.Join(root, "gitdir")
	if err := os.MkdirAll(filepath.Join(gitdir, "objects-quarantine"), 0o755); err != nil {
		t.Fatal(err)
	}
	git := fakeGit(map[string]map[string]string{
		"/ws":       {"--absolute-git-dir": gitdir, "--git-common-dir": "../common/.git"},
		"/abs":      {"--absolute-git-dir": gitdir, "--git-common-dir": "/common/.git"},
		"/no-store": {"--absolute-git-dir": root},
	})
	env := jobGitQuarantineEnv(git, "/ws")
	want := []string{
		"GIT_OBJECT_DIRECTORY=" + filepath.Join(gitdir, "objects-quarantine"),
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=/ws/../common/.git/objects",
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=maintenance.auto",
		"GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto",
		"GIT_CONFIG_VALUE_1=0",
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Fatalf("quarantine env = %v", env)
	}
	if env := jobGitQuarantineEnv(git, "/abs"); env[1] != "GIT_ALTERNATE_OBJECT_DIRECTORIES=/common/.git/objects" {
		t.Fatalf("an absolute common dir was rewritten: %v", env)
	}
	if env := jobGitQuarantineEnv(git, "/no-store"); env != nil {
		t.Fatalf("a workspace without a quarantine store exported %v", env)
	}
	if env := jobGitQuarantineEnv(git, "/not-git"); env != nil {
		t.Fatalf("a non-git workspace exported %v", env)
	}
}

// TestWithEnvReplacesLikeExport: a later assignment replaces an earlier
// value of the same name, as the shell's export did.
func TestWithEnvReplacesLikeExport(t *testing.T) {
	t.Parallel()
	got := withEnv([]string{"A=1", "B=2"}, "B=3", "C=4", "A=5")
	if strings.Join(got, " ") != "A=5 B=3 C=4" {
		t.Fatalf("withEnv = %v", got)
	}
}
