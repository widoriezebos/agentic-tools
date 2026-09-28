package supervisor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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

// TestJobBuildCacheEnv is the build cache contract the dispatch fixtures
// checked through runtime-common.sh's job_build_cache_env: a dispatcher-made
// job worktree gets one cache under its private git dir (created, with the
// staticcheck cache), a shared checkout and a seat's own linked worktree get
// none, and the recorded path names the same cache.
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
	git := fakeGit(map[string]map[string]string{
		worktree: {"--absolute-git-dir": gitdir},
		seat:     {"--absolute-git-dir": filepath.Join(root, ".git", "worktrees", "seat")},
		root:     {"--absolute-git-dir": filepath.Join(root, ".git")},
	})
	env := jobBuildCacheEnv(git, agents, worktree)
	cache := filepath.Join(gitdir, "metasystem-build-cache")
	want := []string{
		"GOCACHE=" + filepath.Join(cache, "go-cache"),
		"GOTMPDIR=" + filepath.Join(cache, "go-tmp"),
		"STATICCHECK_CACHE=" + filepath.Join(cache, "staticcheck"),
	}
	if strings.Join(env, "\n") != strings.Join(want, "\n") {
		t.Fatalf("job worktree cache env = %v, want %v", env, want)
	}
	for _, dir := range []string{"go-cache", "go-tmp", "staticcheck"} {
		if info, err := os.Stat(filepath.Join(cache, dir)); err != nil || !info.IsDir() {
			t.Fatalf("cache directory %s was not created: %v", dir, err)
		}
	}
	if env := jobBuildCacheEnv(git, agents, root); env != nil {
		t.Fatalf("a shared checkout exported a cache: %v", env)
	}
	if env := jobBuildCacheEnv(git, agents, seat); env != nil {
		t.Fatalf("a seat's linked worktree outside the job worktrees exported a cache: %v", env)
	}
	round := filepath.Join(realRoot, "round")
	if err := os.MkdirAll(round, 0o755); err != nil {
		t.Fatal(err)
	}
	recordBuildCachePath(git, agents, worktree, round)
	if data, _ := os.ReadFile(filepath.Join(round, "build-cache.txt")); string(data) != filepath.Join(cache, "go-cache")+"\n" {
		t.Fatalf("recorded cache = %q", data)
	}
	recordBuildCachePath(git, agents, root, round)
	if data, _ := os.ReadFile(filepath.Join(round, "build-cache.txt")); string(data) != "\n" {
		t.Fatalf("a shared checkout recorded a cache: %q", data)
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
