package plain

import (
	"errors"
	"os/exec"
	"testing"
)

// TestGitKeepsTheExitStatus: a caller that decides by Git's exit status (the
// conflict probe's merge-tree exit 1) can read it through Git's error.
func TestGitKeepsTheExitStatus(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	_, err := Git(dir, "rev-parse", "--verify", "--quiet", "refs/heads/absent")
	var exit *exec.ExitError
	if err == nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("Git's error lost the exit status: %v", err)
	}
}
