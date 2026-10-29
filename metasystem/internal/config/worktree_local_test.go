package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Decision 20: Opus 5.5 only; no role, mode or launch lane defaults to Fable.
func TestNoRoleModeOrLaunchDefaultNamesFable(t *testing.T) {
	t.Parallel()
	for _, setting := range CompiledSettings() {
		lane := strings.HasPrefix(setting.Key, "role.") || strings.HasPrefix(setting.Key, "mode.") || strings.HasPrefix(setting.Key, "launch.")
		if lane && strings.Contains(setting.Default, "fable") {
			t.Errorf("%s defaults to %s", setting.Key, setting.Default)
		}
	}
	env := mapEnv(map[string]string{EnvName("metasystem.runtimes"): "claude"})
	for _, key := range []string{"role.code-critic.model.claude", "mode.design.role.implementer.model.claude", "launch.design.model.claude", "launch.read.model.claude"} {
		if value, _, err := Get(GetParams{Key: key, LookupEnv: env}); err != nil || value != "claude-opus-5-5" {
			t.Errorf("%s = %q, %v; want claude-opus-5-5", key, value, err)
		}
	}
}

// worktreeBed builds a main checkout whose installation holds a committed
// metasystem.conf and an engine-made goal worktree under its
// artifacts/agents/workspaces, and answers both installations' conf paths.
func worktreeBed(t *testing.T) (installConf, worktreeConf string) {
	t.Helper()
	main := t.TempDir()
	install := filepath.Join(main, "metasystem")
	if err := os.MkdirAll(install, 0o755); err != nil {
		t.Fatal(err)
	}
	putFile(t, filepath.Join(install, "metasystem.conf"), "metasystem.runtimes=claude\n")
	tree := filepath.Join(install, "artifacts", "agents", "workspaces", "goal-bed", "build")
	for _, args := range [][]string{{"init", "-q"}, {"add", "metasystem/metasystem.conf"}, {"commit", "-q", "-m", "bed"}, {"worktree", "add", "-q", "--detach", tree, "HEAD"}} {
		command := exec.Command("git", append([]string{"-c", "user.name=bed", "-c", "user.email=bed@example.invalid", "-c", "commit.gpgsign=false", "-C", main}, args...)...)
		command.Env = append(scrubbedGitEnv(), "GIT_CONFIG_NOSYSTEM=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, output)
		}
	}
	return filepath.Join(install, "metasystem.conf"), filepath.Join(tree, "metasystem", "metasystem.conf")
}

func scrubbedGitEnv() []string {
	var clean []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			clean = append(clean, entry)
		}
	}
	return clean
}

// An engine-made goal worktree has no metasystem.conf.local of its own (it
// is untracked), so its reads take the installation's roster; the
// worktree's own .local, when present, still wins.
func TestEngineWorktreeReadsTheInstallationsLocalRoster(t *testing.T) {
	t.Parallel()
	installConf, worktreeConf := worktreeBed(t)
	putFile(t, installConf+".local", "role.code-critic.model.claude=bed-installation-critic\nmodel.tier.1=claude:bed-installation-critic\n")
	params := GetParams{Key: "role.code-critic.model.claude", ConfPath: worktreeConf, LookupEnv: noEnv}
	if value, _, err := Get(params); err != nil || value != "bed-installation-critic" {
		t.Fatalf("worktree code critic model = %q, %v; want the installation's local value", value, err)
	}
	if origin, err := KeyOrigin(params); err != nil || origin != "conf-local" {
		t.Fatalf("worktree code critic model source = %q, %v; want conf-local", origin, err)
	}
	if keys := Keys(worktreeConf, "model.tier.", nil); len(keys) != 1 || keys[0] != "model.tier.1" {
		t.Fatalf("worktree tier keys = %q; want the installation's local tier", keys)
	}
	putFile(t, worktreeConf+".local", "role.code-critic.model.claude=bed-worktree-critic\n")
	if value, _, err := Get(params); err != nil || value != "bed-worktree-critic" {
		t.Fatalf("worktree code critic model with its own .local = %q, %v; want the worktree's value", value, err)
	}
}

// A conf outside any engine-made worktree reads only its own sibling.
func TestPlainCheckoutReadsOnlyItsOwnLocal(t *testing.T) {
	t.Parallel()
	installConf, _ := worktreeBed(t)
	putFile(t, installConf+".local", "role.code-critic.model.claude=bed-installation-critic\n")
	other := filepath.Join(t.TempDir(), "metasystem.conf")
	putFile(t, other, "metasystem.runtimes=claude\n")
	if value, _, err := Get(GetParams{Key: "role.code-critic.model.claude", ConfPath: other, LookupEnv: noEnv}); err != nil || value != "claude-opus-5-5" {
		t.Fatalf("plain checkout code critic model = %q, %v; want the compiled default", value, err)
	}
}
