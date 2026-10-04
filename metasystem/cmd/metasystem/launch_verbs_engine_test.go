package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLaunchSupervisorUsesTheServingInstallationsEngine(t *testing.T) {
	t.Parallel()
	base := realpathOf(t, t.TempDir())
	primary, worktree := filepath.Join(base, "primary", "install"), filepath.Join(base, "goal", "install")
	primaryEngine, worktreeEngine := filepath.Join(primary, "bin", "metasystem"), filepath.Join(worktree, "bin", "metasystem")
	pinnedEngine := filepath.Join(worktree, writeInputFile(t, worktree, filepath.Join("artifacts", "agents", "steward", "engine-pins", "generation-1-test"), ""))
	override := filepath.Join(base, "override", "engine")
	for _, installation := range []string{primary, worktree} {
		writeInputFile(t, installation, "metasystem.conf", "# Test installation.\n")
	}
	for _, row := range []struct{ name, root, serving, executable, override, want string }{
		{"primary", primary, primary, primaryEngine, "", ""},
		{"armed worktree", worktree, worktree, worktreeEngine, "", ""},
		{"pinned engine", worktree, worktree, pinnedEngine, "", ""},
		{"unarmed worktree", worktree, primary, worktreeEngine, "", primaryEngine},
		{"override", worktree, worktree, worktreeEngine, override, override},
		{"unreadable executable", "", "", "", "", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			serving := func(root string) (string, string) {
				if root != row.root {
					t.Fatalf("installation = %q, want %q", root, row.root)
				}
				return row.serving, ""
			}
			lookup := func(key string) (string, bool) { return row.override, key == "METASYSTEM_BIN" && row.override != "" }
			var executableErr error
			if row.executable == "" {
				executableErr = os.ErrNotExist
			}
			manager := newLaunchManagerFrom(row.executable, executableErr, serving, lookup)
			if executableErr != nil && manager.SettingsError != executableErr {
				t.Fatalf("settings error = %v, want %v", manager.SettingsError, executableErr)
			}
			if got := manager.Supervisor.(launch.OSSupervisorStarter).Executable; got != row.want {
				t.Fatalf("supervisor program = %q, want %q", got, row.want)
			}
		})
	}
}
