package testenv

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Fixture repositories must have no writers after a Git command returns.
// gc.auto does not disable non-GC maintenance tasks. gc.autoDetach also
// supplies maintenance.autoDetach's fallback on supported Git versions.
const fixtureGitConfig = `[maintenance]
 auto = false
[gc]
 auto = 0
 autoDetach = false
[fetch]
 writeCommitGraph = false
`

func writeFixtureGitConfig(path string) error {
	return os.WriteFile(path, []byte(fixtureGitConfig), 0o600)
}

// xcrun selects Apple's developer-tool git once per test binary. Calling that
// git directly avoids the wrapper's per-command tool lookup. Unlike the wrapper,
// it does not add SDKROOT, CPATH or LIBRARY_PATH to git's environment.
var developerGit = sync.OnceValues(func() ([]byte, error) {
	return exec.Command("/usr/bin/xcrun", "-f", "git").Output()
})

func resolveTestGit(goos string, lookPath func(string) (string, error), xcrun func() ([]byte, error)) string {
	if goos != "darwin" {
		return ""
	}
	git, err := lookPath("git")
	if err != nil || git != "/usr/bin/git" {
		return ""
	}
	output, err := xcrun()
	if err != nil {
		return ""
	}
	resolved := strings.TrimSpace(string(output))
	if !filepath.IsAbs(resolved) || filepath.Clean(resolved) == "/usr/bin/git" {
		return ""
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 {
		return ""
	}
	return resolved
}

type gitResources struct {
	execPath string
	hooks    map[string]string
}

// Only git lives on PATH: exposing the developer tools directory would also
// change how tests find other tools. Its adjacent resources retain the real
// installation's layout because Apple's git derives its prefix from this path.
func namespaceGitPath(root, git, path string, probe func(string, string) (gitResources, error)) string {
	if git == "" {
		return path
	}
	prefix := filepath.Join(root, "git-prefix")
	if err := os.Mkdir(prefix, 0o700); err != nil {
		return path
	}
	published := false
	defer func() {
		if !published {
			_ = os.RemoveAll(prefix)
		}
	}()
	directory := filepath.Join(prefix, "bin")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return path
	}
	if err := os.Symlink(git, filepath.Join(directory, "git")); err != nil {
		return path
	}
	realPrefix := filepath.Dir(filepath.Dir(git))
	for _, name := range []string{"libexec", "share", "etc"} {
		target := filepath.Join(realPrefix, name)
		info, err := os.Stat(target)
		if name == "etc" && os.IsNotExist(err) {
			continue
		}
		if err != nil || !info.IsDir() {
			return path
		}
		if err := os.Symlink(target, filepath.Join(prefix, name)); err != nil {
			return path
		}
	}
	// Probe the exact entry before publishing it once at test-binary startup.
	// A usable exec path alone does not prove helpers and templates still work.
	wrapper, wrapperErr := probe("/usr/bin/git", root)
	namespaced, namespaceErr := probe(filepath.Join(directory, "git"), root)
	if wrapperErr != nil || namespaceErr != nil || len(wrapper.hooks) == 0 || !maps.Equal(wrapper.hooks, namespaced.hooks) {
		return path
	}
	if !filepath.IsAbs(strings.TrimSpace(wrapper.execPath)) || !filepath.IsAbs(strings.TrimSpace(namespaced.execPath)) {
		return path
	}
	wrapperPath, wrapperErr := filepath.EvalSymlinks(strings.TrimSpace(wrapper.execPath))
	namespacePath, namespaceErr := filepath.EvalSymlinks(strings.TrimSpace(namespaced.execPath))
	info, statErr := os.Stat(namespacePath)
	if wrapperErr != nil || namespaceErr != nil || namespacePath != wrapperPath || statErr != nil || !info.IsDir() {
		return path
	}
	published = true
	return directory + string(os.PathListSeparator) + path
}

func probeGitResources(git, root string) (gitResources, error) {
	directory, err := os.MkdirTemp(root, "git-probe-")
	if err != nil {
		return gitResources{}, err
	}
	defer os.RemoveAll(directory)
	if err := writeFixtureGitConfig(filepath.Join(directory, ".gitconfig")); err != nil {
		return gitResources{}, err
	}
	environment := gitProbeEnvironment(os.Environ(), directory)
	return inspectGitResources(directory, func(args ...string) ([]byte, error) {
		command := exec.Command(git, args...)
		command.Dir = directory
		command.Env = environment
		return command.CombinedOutput()
	})
}

func gitProbeEnvironment(environment []string, home string) []string {
	clean := make([]string, 0, len(environment)+4)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		// Fixtures scrub GIT_* variables. Overrides must not mask a broken
		// prefix, and a person's init.templateDir must not mask missing templates.
		if !strings.HasPrefix(name, "GIT_") && name != "HOME" && name != "XDG_CONFIG_HOME" {
			clean = append(clean, entry)
		}
	}
	return append(clean, "HOME="+home, "XDG_CONFIG_HOME="+home, "GIT_CONFIG_GLOBAL="+filepath.Join(home, ".gitconfig"), "GIT_CONFIG_NOSYSTEM=1")
}

func inspectGitResources(directory string, run func(...string) ([]byte, error)) (gitResources, error) {
	output, err := run("--exec-path")
	if err != nil {
		return gitResources{}, err
	}
	resources := gitResources{execPath: string(output), hooks: make(map[string]string)}
	output, err = run("submodule", "-h")
	if err != nil {
		return gitResources{}, err
	}
	if !bytes.Contains(output, []byte("usage: git submodule")) {
		return gitResources{}, fmt.Errorf("git submodule help is unavailable")
	}
	repo := filepath.Join(directory, "repository")
	if _, err := run("init", "-q", repo); err != nil {
		return gitResources{}, err
	}
	hooks := filepath.Join(repo, ".git", "hooks")
	entries, err := os.ReadDir(hooks)
	if err != nil {
		return gitResources{}, err
	}
	if len(entries) == 0 {
		return gitResources{}, fmt.Errorf("git init created no hook templates")
	}
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(hooks, entry.Name()))
		if err != nil {
			return gitResources{}, err
		}
		resources.hooks[entry.Name()] = string(data)
	}
	return resources, nil
}
