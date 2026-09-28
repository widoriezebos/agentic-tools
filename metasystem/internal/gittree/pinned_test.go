package gittree

import (
	"errors"
	"os/exec"
	"slices"
	"testing"
)

func TestPinnedGitBuildsTheAnchorSurfaceCommand(t *testing.T) {
	t.Parallel()
	cmd, _ := PinnedGit("/repo", []string{"GIT_INDEX_FILE=/own"}, "rev-parse", "HEAD")
	wantArgs := []string{"git", "-C", "/repo", "-c", "core.useReplaceRefs=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "rev-parse", "HEAD"}
	if !slices.Equal(cmd.Args, wantArgs) {
		t.Fatalf("args = %q, want %q", cmd.Args, wantArgs)
	}
	if len(cmd.Env) == 0 || cmd.Env[len(cmd.Env)-1] != "GIT_INDEX_FILE=/own" {
		t.Fatalf("the extra environment is appended after the scrub: %q", cmd.Env)
	}
}

func TestExitCodeSeparatesAnswersFromCouldNotRun(t *testing.T) {
	t.Parallel()
	if ExitCode(nil) != 0 {
		t.Fatal("success is exit 0")
	}
	failed := exec.Command("sh", "-c", "exit 3").Run()
	if ExitCode(failed) != 3 {
		t.Fatalf("git's own exit is its answer: %v", failed)
	}
	if ExitCode(errors.New("timed out")) != -1 {
		t.Fatal("could-not-run is -1")
	}
}
