package lane

// The pre-push hook an earlier engine installed in the lane checkout is
// gone (simple lane): landing set and landing unset remove one they find,
// so the checkout pushes as any other.

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// hookMarker is the line that made a pre-push hook the lane's own.
const hookMarker = "# the landing lane's pre-push hook, installed by landing set"

// removeHook removes the lane's earlier pre-push hook from checkout; a hook
// that is not the lane's, or a checkout that is gone, is left as it is.
func removeHook(checkout string) error {
	if checkout == "" || gone(checkout) {
		return nil
	}
	hooks, err := laneGit(checkout, nil, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	if err != nil {
		return nil
	}
	path := filepath.Join(hooks, "pre-push")
	existing, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(existing, []byte(hookMarker)) {
		return nil
	}
	return removeIfPresent(path)
}

// gitSteering is the environment that would point git at another
// repository, configuration or object store than the one named: a lane
// publication never inherits it.
func gitSteering(name string) bool {
	switch name {
	case "GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_CEILING_DIRECTORIES",
		"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_CONFIG", "GIT_CONFIG_PARAMETERS",
		"GIT_CONFIG_COUNT", "GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_NOSYSTEM",
		"GIT_GRAFT_FILE", "GIT_SHALLOW_FILE", "GIT_REPLACE_REF_BASE":
		return true
	}
	return strings.HasPrefix(name, "GIT_CONFIG_KEY_") || strings.HasPrefix(name, "GIT_CONFIG_VALUE_")
}

// laneGit runs git in dir with the steering environment removed and extra
// added, and returns its trimmed standard output (standard error with it
// when it fails).
func laneGit(dir string, extra []string, args ...string) (string, error) {
	return laneGitInput(dir, extra, "", args...)
}

// laneGitInput is laneGit with input on git's standard input.
func laneGitInput(dir string, extra []string, input string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stdin = strings.NewReader(input)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !gitSteering(name) {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "LC_ALL=C")
	command.Env = append(command.Env, extra...)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return strings.TrimSpace(stdout.String() + "\n" + stderr.String()), fmt.Errorf("git %s: %w", args[0], err)
	}
	return strings.TrimSpace(stdout.String()), nil
}
