package dispatch

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// criticBed is a main checkout whose installation commits a claude-only
// metasystem.conf, with an engine-made goal worktree under its
// artifacts/agents/workspaces; it answers both conf paths.
func criticBed(t *testing.T) (installConf, worktreeConf string) {
	t.Helper()
	main := t.TempDir()
	install := filepath.Join(main, "metasystem")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(install, "metasystem.conf"), []byte("metasystem.runtimes=claude\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(install, "artifacts", "agents", "workspaces", "goal-bed", "build")
	var env []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			env = append(env, entry)
		}
	}
	for _, args := range [][]string{{"init", "-q"}, {"add", "metasystem/metasystem.conf"}, {"commit", "-q", "-m", "bed"}, {"worktree", "add", "-q", "--detach", tree, "HEAD"}} {
		command := exec.Command("git", append([]string{"-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "commit.gpgsign=false", "-C", main}, args...)...)
		command.Env = append(env, "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	return filepath.Join(install, "metasystem.conf"), filepath.Join(tree, "metasystem", "metasystem.conf")
}

// The model a code-critic dispatch launches on, resolved the way the
// delegation dispatch phase resolves it from the goal worktree's settings:
// Opus 5.5 by default, and the installation's local roster when it names one.
func TestCodeCriticLaunchModelFromAnEngineWorktree(t *testing.T) {
	installConf, worktreeConf := criticBed(t)
	none := func(string) (string, bool) { return "", false }
	roster, err := ResolveRoster(RosterParams{ConfPath: worktreeConf, Role: "code-critic", LookupEnv: none})
	if err != nil || roster.Runtime != "claude" || roster.Model != "claude-opus-5-5" {
		t.Fatalf("default code critic = %s:%s, %v; want claude:claude-opus-5-5", roster.Runtime, roster.Model, err)
	}
	if err := os.WriteFile(installConf+".local", []byte("role.code-critic.model.claude=bed-installation-critic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	roster, err = ResolveRoster(RosterParams{ConfPath: worktreeConf, Role: "code-critic", LookupEnv: none})
	if err != nil || roster.Model != "bed-installation-critic" {
		t.Fatalf("code critic from the worktree = %s:%s, %v; want the installation's roster", roster.Runtime, roster.Model, err)
	}
}
