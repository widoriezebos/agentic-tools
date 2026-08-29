package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

func TestTopLevelUpPrintsButDoesNotInstallSchedulerEntry(t *testing.T) {
	root := t.TempDir()
	if out, err := exec.Command("git", "-C", root, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	stdout, stderr, code := captureRelay(t, func() int {
		return dispatch([]string{
			"up", "--metasystem-root", root, "--repo", root, "--print-scheduler-entry",
		})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("scheduler print failed: code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "up --metasystem-root") ||
		!strings.Contains(stdout, "--recover-only --if-down") {
		t.Fatalf("scheduler entry is not recovery-only: %q", stdout)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("scheduler printing installed repository state: %v", err)
	}
}

func TestTopLevelUpKeepsTemplateStateSeparateFromGitScope(t *testing.T) {
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", "")
	appRoot := t.TempDir()
	metasystemRoot := filepath.Join(appRoot, "metasystem")
	if err := os.MkdirAll(filepath.Join(appRoot, "development"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appRoot, "development", "metasystem-design.md"), []byte("design\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(metasystemRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metasystemRoot, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("git", "-C", appRoot, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}

	pid := int64(os.Getppid())
	started, ok := lease.StartedAt(pid, nil)
	if !ok {
		t.Skip("cannot read the parent process identity")
	}
	announcement, err := lease.Announce(metasystemRoot, "template-state", pid, started, "tag", "fake", "")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(announcement)
	if err != nil {
		t.Fatal(err)
	}
	gitScopeAnnouncement := filepath.Join(appRoot, "artifacts", "agents", "mains", filepath.Base(announcement))
	if err := os.MkdirAll(filepath.Dir(gitScopeAnnouncement), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitScopeAnnouncement, data, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := captureRelay(t, func() int {
		return runUp([]string{
			"--metasystem-root", metasystemRoot, "--repo", appRoot, "--retire",
			"--session", "template-state", "--pid", fmt.Sprint(pid), "--start-time", fmt.Sprint(started), "--runtime", "fake",
		})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("template retirement failed: code=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(announcement); !os.IsNotExist(err) {
		t.Fatalf("template announcement was not retired from the metasystem state root: %v", err)
	}
	if _, err := os.Stat(gitScopeAnnouncement); err != nil {
		t.Fatalf("template retirement touched the separate Git scope: %v", err)
	}
}
