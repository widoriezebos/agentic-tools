package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gitOutput runs one read-only git query in dir and returns its trimmed
// stdout. It is a variable so tests stub Git per test instance.
var gitOutput = func(dir string, args ...string) (string, bool) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(output), "\n"), true
}

func realDir(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", false
	}
	return abs, true
}

// jobBuildCacheEnv prints the build cache assignments for a job workspace:
// one build cache per chain (goal delegate-rounds-reuse-a-warm-gate). Every
// round of a chain runs in the chain root's worktree, and the sandboxes grant
// writes only inside that worktree and its derived git roots, so the cache
// lives in the worktree's private git dir beside the quarantine object store.
// Only a job worktree the dispatcher made (under artifacts/agents/worktrees)
// qualifies: a seat checkout that is itself a linked worktree, or a landing's
// detached one, has a git dir the envelope never grants. A job without a
// worktree gets nothing.
func jobBuildCacheEnv(agents, workspace string) []string {
	jobsRoot, ok := realDir(filepath.Join(agents, "worktrees"))
	if !ok {
		return nil
	}
	ws, ok := realDir(workspace)
	if !ok || !strings.HasPrefix(ws+"/", jobsRoot+"/") {
		return nil
	}
	gitdir, ok := gitOutput(workspace, "rev-parse", "--absolute-git-dir")
	if !ok || !strings.Contains(gitdir, "/.git/worktrees/") {
		return nil
	}
	cache := filepath.Join(gitdir, "metasystem-build-cache")
	for _, dir := range []string{"go-cache", "go-tmp", "staticcheck"} {
		if err := os.MkdirAll(filepath.Join(cache, dir), 0o755); err != nil {
			return nil
		}
	}
	return []string{
		"GOCACHE=" + filepath.Join(cache, "go-cache"),
		"GOTMPDIR=" + filepath.Join(cache, "go-tmp"),
		// staticcheck (the fast gate) keeps its own cache and exits 1
		// when it cannot write one.
		"STATICCHECK_CACHE=" + filepath.Join(cache, "staticcheck"),
	}
}

// recordBuildCachePath writes the round's build-cache.txt: the chain cache
// a round used, empty for a job without a worktree.
func recordBuildCachePath(agents, workspace, roundDir string) {
	cache := ""
	for _, assignment := range jobBuildCacheEnv(agents, workspace) {
		if value, found := strings.CutPrefix(assignment, "GOCACHE="); found {
			cache = value
		}
	}
	_ = os.WriteFile(filepath.Join(roundDir, "build-cache.txt"), []byte(cache+"\n"), 0o644)
}

// jobGitQuarantineEnv routes the delegate's git object writes into the
// worktree's private quarantine store (issue #5): derived statelessly from
// the workspace so dispatch and follow-up rounds behave identically, empty
// for non-worktree jobs. The shared object store stays read-only to the
// delegate; reads fall through the alternates link the engine created at
// worktree dispatch.
func jobGitQuarantineEnv(workspace string) []string {
	gitdir, ok := gitOutput(workspace, "rev-parse", "--absolute-git-dir")
	if !ok {
		return nil
	}
	quarantine := filepath.Join(gitdir, "objects-quarantine")
	if info, err := os.Stat(quarantine); err != nil || !info.IsDir() {
		return nil
	}
	common, _ := gitOutput(workspace, "rev-parse", "--git-common-dir")
	common += "/objects"
	if !strings.HasPrefix(common, "/") {
		common = workspace + "/" + common
	}
	return []string{
		"GIT_OBJECT_DIRECTORY=" + quarantine,
		"GIT_ALTERNATE_OBJECT_DIRECTORIES=" + common,
		// Automatic maintenance after a delegate commit would run
		// pack-refs and reflog expiry AGAINST THE SHARED STORE, writing
		// packed-refs and reflogs outside the granted roots. The engine
		// owns maintenance; the delegate's git never triggers it.
		"GIT_CONFIG_COUNT=2",
		"GIT_CONFIG_KEY_0=maintenance.auto",
		"GIT_CONFIG_VALUE_0=false",
		"GIT_CONFIG_KEY_1=gc.auto",
		"GIT_CONFIG_VALUE_1=0",
	}
}

// withEnv returns base with each assignment applied in order, a later
// assignment replacing an earlier value of the same name (the shell's
// `export` semantics).
func withEnv(base []string, assignments ...string) []string {
	out := append([]string(nil), base...)
	for _, assignment := range assignments {
		name, _, _ := strings.Cut(assignment, "=")
		replaced := false
		for index, existing := range out {
			if existingName, _, _ := strings.Cut(existing, "="); existingName == name {
				out[index] = assignment
				replaced = true
			}
		}
		if !replaced {
			out = append(out, assignment)
		}
	}
	return out
}
