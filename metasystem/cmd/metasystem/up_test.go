package main

import (
	"encoding/json"
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

func TestArmingDetailSurvivesStop(t *testing.T) {
	component := `component=steward-runner outcome=failed detail="ENROLLMENT_DRIFT" remedy="run metasystem steward restart from an agent-free terminal"`
	aggregate := `up outcome=failed component=steward-runner remedy="ENROLLMENT_DRIFT: run 'metasystem steward restart' from an agent-free terminal"`
	armingResult := component + "\n" + aggregate
	remedy := "restore supervision from an agent-free terminal"
	stdout, stderr, code := captureRelay(t, func() int {
		return runReportStopBlock([]string{
			"--class", "infrastructure", "--refusal-record", filepath.Join(t.TempDir(), "refusals.json"),
			"--session", "arming-detail", "--cause", "supervision arming failed", "--remedy", remedy,
			"--arming-result", armingResult, "arming failed",
		})
	})
	if code != 0 || stderr != "" {
		t.Fatalf("stop notice composition failed: code=%d stderr=%q", code, stderr)
	}
	var response map[string]any
	if err := json.Unmarshal([]byte(stdout), &response); err != nil {
		t.Fatal(err)
	}
	message, _ := response["systemMessage"].(string)
	if !strings.Contains(message, armingResult) {
		t.Fatalf("the stop notice did not preserve the failed component outcome, detail, remedy, and aggregate byte for byte: %q", message)
	}
	if !strings.Contains(message, "Remedy: "+remedy) {
		t.Fatalf("the stop notice omitted its distinct remedy: %q", message)
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
