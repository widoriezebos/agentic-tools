package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
)

// GitQuery runs one read-only git query in dir and returns its trimmed
// stdout; ok is false when git refuses.
type GitQuery func(dir string, args ...string) (string, bool)

// gitOutput is the GitQuery a Deps without its own Git uses; a package
// TestMain may fix it once for the whole package.
var gitOutput GitQuery = runGit

// git is the dependency's GitQuery.
func (d Deps) git() GitQuery {
	if d.Git != nil {
		return d.Git
	}
	return gitOutput
}

// runGit is the production GitQuery.
func runGit(dir string, args ...string) (string, bool) {
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

// jobBuildCacheEnv prints the build cache assignments for a job workspace
// (disk-lifetimes A7): every job, in a worktree or a shared checkout,
// builds in the one machine delegate cache pair (caches, created here and
// granted by every sandbox), so follow-up rounds and other chains start warm
// and a delegate can plant nothing the engine's own builds reuse. A job
// worktree the dispatcher made (under artifacts/agents/worktrees, with a
// linked worktree's git dir) adds a chain-private GOTMPDIR in its git dir,
// which the envelope grants and which goes with the worktree. An empty
// caches (the user cache dir did not resolve) exports no cache.
func jobBuildCacheEnv(git GitQuery, agents, workspace string, caches gocache.Paths) []string {
	var env []string
	if caches.GoCache != "" && caches.StaticcheckCache != "" &&
		os.MkdirAll(caches.GoCache, 0o755) == nil && os.MkdirAll(caches.StaticcheckCache, 0o755) == nil {
		env = append(env, "GOCACHE="+caches.GoCache, "STATICCHECK_CACHE="+caches.StaticcheckCache)
	}
	if gitdir, ok := jobWorktreeGitDir(git, agents, workspace); ok {
		tmp := filepath.Join(gitdir, "metasystem-go-tmp")
		if os.MkdirAll(tmp, 0o755) == nil {
			env = append(env, "GOTMPDIR="+tmp)
		}
	}
	return env
}

// jobWorktreeGitDir is the private git dir of a job worktree the dispatcher
// made; a seat checkout that is itself a linked worktree, or a landing's
// detached one, is not one.
func jobWorktreeGitDir(git GitQuery, agents, workspace string) (string, bool) {
	jobsRoot, ok := realDir(filepath.Join(agents, "worktrees"))
	if !ok {
		return "", false
	}
	ws, ok := realDir(workspace)
	if !ok || !strings.HasPrefix(ws+"/", jobsRoot+"/") {
		return "", false
	}
	gitdir, ok := git(workspace, "rev-parse", "--absolute-git-dir")
	if !ok || !strings.Contains(gitdir, "/.git/worktrees/") {
		return "", false
	}
	return gitdir, true
}

// recordBuildCachePath writes the round's build-cache.txt: the delegate
// cache the round builds in, empty when it did not resolve.
func recordBuildCachePath(git GitQuery, agents, workspace, roundDir string, caches gocache.Paths) {
	cache := ""
	for _, assignment := range jobBuildCacheEnv(git, agents, workspace, caches) {
		if value, found := strings.CutPrefix(assignment, "GOCACHE="); found {
			cache = value
		}
	}
	_ = os.WriteFile(filepath.Join(roundDir, "build-cache.txt"), []byte(cache+"\n"), 0o644)
}

// delegateCaches is the machine delegate cache pair under the user cache
// dir (the Deps seam, else os.UserCacheDir); an unresolvable dir is empty.
func (d Deps) delegateCaches() gocache.Paths {
	paths, err := gocache.DomainPathsUsing(gocache.DomainDelegate, d.UserCacheDir)
	if err != nil {
		return gocache.Paths{}
	}
	return paths
}

// engineCaches is the machine engine cache pair a delegate's sandbox is
// denied.
func (d Deps) engineCaches() gocache.Paths {
	paths, err := gocache.DomainPathsUsing(gocache.DomainEngine, d.UserCacheDir)
	if err != nil {
		return gocache.Paths{}
	}
	return paths
}

// delegateRoundEnv is what every delegate launch form adds for its
// workspace: the quarantine object store and the machine delegate cache.
// Devin has no OS sandbox, so for it the export is the whole protection
// (disk-lifetimes 3.1's stated limit).
func delegateRoundEnv(d Deps, workspace string) []string {
	return withEnv(jobGitQuarantineEnv(d.git(), workspace), jobBuildCacheEnv(d.git(), d.agents(), workspace, d.delegateCaches())...)
}

// jobGitQuarantineEnv routes the delegate's git object writes into the
// worktree's private quarantine store (issue #5): derived statelessly from
// the workspace so dispatch and follow-up rounds behave identically, empty
// for non-worktree jobs. The shared object store stays read-only to the
// delegate; reads fall through the alternates link the engine created at
// worktree dispatch.
func jobGitQuarantineEnv(git GitQuery, workspace string) []string {
	gitdir, ok := git(workspace, "rev-parse", "--absolute-git-dir")
	if !ok {
		return nil
	}
	quarantine := filepath.Join(gitdir, "objects-quarantine")
	if info, err := os.Stat(quarantine); err != nil || !info.IsDir() {
		return nil
	}
	common, _ := git(workspace, "rev-parse", "--git-common-dir")
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
