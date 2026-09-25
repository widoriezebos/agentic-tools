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

func upAdapterGit(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	command := exec.Command("git", args...)
	command.Env = []string{"HOME=" + os.Getenv("HOME"), "PATH=" + os.Getenv("PATH"), "TMPDIR=" + os.Getenv("TMPDIR")}
	return command
}

func TestTopLevelUpPrintsButDoesNotInstallSchedulerEntry(t *testing.T) {
	root := t.TempDir()
	repositoryTop := declaredRepositoryTop(t, root, map[string]int{root: 1})
	stdout, stderr, code := captureRelay(t, func() int {
		return dispatchWithRepositoryTop([]string{
			"up", "--metasystem-root", root, "--repo", root, "--print-scheduler-entry",
		}, repositoryTop)
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
	repositoryTop := declaredRepositoryTop(t, appRoot, map[string]int{appRoot: 1})
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
	pid := int64(os.Getppid())
	started, ok := lease.StartedAt(pid, nil)
	if !ok {
		t.Fatal("cannot read the parent process identity")
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
		return runUpWith([]string{
			"--metasystem-root", metasystemRoot, "--repo", appRoot, "--retire",
			"--session", "template-state", "--pid", fmt.Sprint(pid), "--start-time", fmt.Sprint(started), "--runtime", "fake",
		}, repositoryTop)
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

// A linked worktree and Git steering variables require the real repository adapter.
func TestUpRepositoryScopeIgnoresGitSteeringEnvironment(t *testing.T) {
	primary := t.TempDir()
	if out, err := upAdapterGit(t, "-C", primary, "init", "-q", "-b", "main").CombinedOutput(); err != nil {
		t.Fatalf("git init primary: %v: %s", err, out)
	}
	if err := os.WriteFile(filepath.Join(primary, "tracked"), []byte("fixture\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := upAdapterGit(t, "-C", primary, "add", "tracked").CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, out)
	}
	commit := upAdapterGit(t, "-C", primary, "-c", "user.name=fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture")
	if out, err := commit.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, out)
	}
	worktree := filepath.Join(t.TempDir(), "worktree")
	if out, err := upAdapterGit(t, "-C", primary, "worktree", "add", "-q", worktree, "HEAD").CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %v: %s", err, out)
	}
	t.Setenv("GIT_DIR", filepath.Join(primary, ".git"))
	t.Setenv("GIT_WORK_TREE", worktree)

	got, err := upRepositoryScope(primary)
	if err != nil {
		t.Fatal(err)
	}
	want, err := canonicalPath(primary)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("upRepositoryScope() with Git steering = %q; want %q", got, want)
	}
}
