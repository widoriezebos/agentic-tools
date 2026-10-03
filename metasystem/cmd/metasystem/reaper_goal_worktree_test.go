package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// An unarmed goal worktree runs no reaper of its own: its system is its
// primary checkout's (landpath.SystemInstallation). The primary's reaper
// tick concludes a goal worktree job whose custodian is provably dead, so
// the record stops holding an active-job slot; an armed linked worktree's
// records stay its own reaper's.
func TestReaperTickReapsTheUnarmedGoalWorktreesDeadJobs(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary := filepath.Join(base, "primary")
	primaryInstall := filepath.Join(primary, "metasystem")
	git := func(dir string, args ...string) {
		t.Helper()
		command := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "core.hooksPath=/dev/null"}, args...)...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, output)
		}
	}
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(primaryInstall, "metasystem.conf"), "metasystem.runtimes=fake\n")
	write(filepath.Join(primaryInstall, ".gitignore"), "artifacts/\n")
	git(primary, "init", "-q", "-b", "main")
	git(primary, "add", "-A")
	git(primary, "commit", "-qm", "bed baseline")
	unarmed := filepath.Join(base, "primary-goal-a")
	armed := filepath.Join(base, "primary-goal-b")
	git(primary, "worktree", "add", "-q", "-b", "goal/a", unarmed)
	git(primary, "worktree", "add", "-q", "-b", "goal/b", armed)
	if err := os.MkdirAll(filepath.Join(primaryInstall, "artifacts", "agents", "jobs"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(armed, "metasystem", "artifacts", "agents", "supervision", "state.json"), `{"generation":1}`)
	// A pid no process holds: the kernel prover answers Dead.
	dead := `{"jobId":"%s","status":"running","role":"implementer","pid":1073741817,"pgid":1073741817,"pidStartedAt":100,"instanceTag":"reap-goal-worktree-%s","startedAt":"%s","capMin":60}`
	record := func(install, job string) string {
		return filepath.Join(install, "artifacts", "agents", "jobs", job+".json")
	}
	write(record(filepath.Join(unarmed, "metasystem"), "job-a"), fmt.Sprintf(dead, "job-a", "job-a", time.Now().UTC().Format(time.RFC3339)))
	write(record(filepath.Join(armed, "metasystem"), "job-b"), fmt.Sprintf(dead, "job-b", "job-b", time.Now().UTC().Format(time.RFC3339)))

	setupReaper(t.Output(), primaryInstall, primaryInstall)()

	status := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]any
		if err := json.Unmarshal(data, &fields); err != nil {
			t.Fatal(err)
		}
		value, _ := fields["status"].(string)
		return value
	}
	if got := status(record(filepath.Join(unarmed, "metasystem"), "job-a")); got != "failed" {
		t.Fatalf("the unarmed goal worktree's dead job reads %q after the primary's reaper tick, want failed", got)
	}
	if got := status(record(filepath.Join(armed, "metasystem"), "job-b")); got != "running" {
		t.Fatalf("the primary's reaper concluded an armed worktree's job: status %q", got)
	}
}
