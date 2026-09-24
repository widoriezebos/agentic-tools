package branch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBranchReadDefaultDetachedAdapterChecksOutTreeAndCleansUp(t *testing.T) {
	repo := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %q: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.name", "branch read adapter")
	git("config", "user.email", "branch-read@example.invalid")
	file := filepath.Join(repo, "code.go")
	if err := os.WriteFile(file, []byte("requested tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", "code.go")
	git("commit", "-qm", "requested")
	requested := git("rev-parse", "HEAD")
	if err := os.WriteFile(file, []byte("current tree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("commit", "-qam", "current")
	dir, closeDetached, err := (defaultBranchReadRepository{}).Detached(repo, requested)
	if err != nil {
		t.Fatal(err)
	}
	data, readErr := os.ReadFile(filepath.Join(dir, "code.go"))
	if readErr != nil || string(data) != "requested tree\n" {
		t.Errorf("detached content=%q err=%v", data, readErr)
	}
	if err := closeDetached(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("detached worktree remains: %v", err)
	}
	data, err = os.ReadFile(file)
	if err != nil || string(data) != "current tree\n" {
		t.Fatalf("caller tree=%q err=%v", data, err)
	}
}
