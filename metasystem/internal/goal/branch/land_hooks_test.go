package branch_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// TestPrepareLandingScratchSkipsInstalledHooks: the checkout's installed
// hooks (an enrolled pre-commit guard refuses a new plan file) judge real
// commits, not the private scratch in which the owner composes a goal's
// candidate. A goal whose folds add plans still composes, and no hook runs.
// Git is the claim because hook selection is Git's own behavior.
func TestPrepareLandingScratchSkipsInstalledHooks(t *testing.T) {
	t.Parallel()
	f := newLandFixture(t)
	hooks := git(t, f.root, "rev-parse", "--path-format=absolute", "--git-path", "hooks")
	marker := filepath.Join(t.TempDir(), "hook-ran")
	for _, hook := range []string{"pre-commit", "post-checkout", "commit-msg"} {
		write(t, hooks, hook, "#!/bin/sh\necho "+hook+" >>"+marker+"\nexit 1\n")
		if err := os.Chmod(filepath.Join(hooks, hook), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	request := landRequest(t, f, "")
	request.Repo = filepath.Join(f.root, "metasystem")
	request.CandidateOnly = true
	result, err := branch.PrepareLanding(request)
	if err != nil {
		t.Fatalf("an installed hook stopped the scratch composition: %v", err)
	}
	if result.Endpoint != f.base || len(result.Candidate) != 40 {
		t.Fatalf("candidate = %+v", result)
	}
	if ran, err := os.ReadFile(marker); !os.IsNotExist(err) {
		t.Fatalf("installed hooks ran in the scratch: %q %v", ran, err)
	}
	if got, err := exec.Command("git", "-C", f.root, "config", "--get", "core.hooksPath").Output(); err == nil {
		t.Fatalf("the checkout's hooks configuration changed to %q", got)
	}
}
