package testenv

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

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

// Only git lives here: exposing the developer tools directory would also change
// how tests find other tools. Failure leaves the inherited PATH usable.
func namespaceGitPath(root, git, path string, execPath func(string) ([]byte, error)) string {
	if git == "" {
		return path
	}
	directory := filepath.Join(root, "git-bin")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return path
	}
	if err := os.Symlink(git, filepath.Join(directory, "git")); err != nil {
		return path
	}
	// Apple's git can lose its installation prefix when invoked through a
	// symlink. Probe that exact entry before publishing it on PATH, while the
	// process namespace is being created once at test-binary startup.
	wrapper, wrapperErr := execPath("/usr/bin/git")
	namespaced, namespaceErr := execPath(filepath.Join(directory, "git"))
	resources := strings.TrimSpace(string(namespaced))
	info, statErr := os.Stat(resources)
	if wrapperErr != nil || namespaceErr != nil || resources != strings.TrimSpace(string(wrapper)) || statErr != nil || !info.IsDir() {
		_ = os.RemoveAll(directory)
		return path
	}
	return directory + string(os.PathListSeparator) + path
}

func gitExecPath(git string) ([]byte, error) {
	command := exec.Command(git, "--exec-path")
	// Fixtures can scrub GIT_* variables, so an inherited override must not
	// hide a broken installation prefix during the probe.
	command.Env = []string{}
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_EXEC_PATH=") {
			command.Env = append(command.Env, entry)
		}
	}
	return command.Output()
}
