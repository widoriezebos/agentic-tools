package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTheSupervisorLoadsTheServingInstallationsRegistry(t *testing.T) {
	t.Parallel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	primary, linked := filepath.Join(base, "primary"), filepath.Join(base, "goal")
	for _, dir := range []string{filepath.Join(primary, ".git"), linked} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(linked, ".git"), []byte("gitdir: "+filepath.Join(primary, ".git", "worktrees", "goal")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, row := range []struct {
		name, checkout, serving, armed, override string
	}{
		{"primary", primary, primary, "", ""},
		{"inside primary checkout", primary, primary, "", ""},
		{"outside repository", base, base, "", ""},
		{"primary override", primary, primary, "", filepath.Join(base, "explicit-engine")},
		{"outside override", base, base, "", filepath.Join(base, "explicit-engine")},
		{"unarmed worktree", linked, primary, "", ""},
		{"armed worktree", linked, linked, "state.json", ""},
		{"censused worktree", linked, linked, "last-census.json", ""},
		{"unmapped root", linked, linked, "", ""},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := filepath.Join(row.checkout, row.name, "install")
			if err := os.MkdirAll(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if row.armed != "" {
				state := filepath.Join(root, "artifacts", "agents", "supervision")
				if err := os.MkdirAll(state, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(state, row.armed), []byte("{}"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			git := fakeGit(map[string]map[string]string{root: {"--show-toplevel": row.checkout}})
			query := func(dir string, args ...string) (string, bool) {
				if row.checkout != linked || row.armed != "" {
					t.Fatalf("unexpected Git query in %s: %v", dir, args)
				}
				if row.name == "unmapped root" {
					return "", false
				}
				if dir == root && len(args) == 3 && args[2] == "--git-common-dir" {
					return filepath.Join(primary, ".git"), true
				}
				return git(dir, args...)
			}
			installation := filepath.Join(row.serving, row.name, "install")
			os.MkdirAll(installation, 0o755)
			mustWrite(t, filepath.Join(installation, "metasystem.conf"), "watch.cap-min=210\n")
			mustWrite(t, filepath.Join(installation, "metasystem.conf.local"), "watch.cap-min=230\n")
			installExternalAdapter(t, installation, "newagent", "newagent", 0o700, true)
			deps := processDeps(root, query, func(key string) (string, bool) {
				return row.override, key == "METASYSTEM_BIN" && row.override != ""
			})
			deps.Environ = nil
			if _, err := OperationsAt(deps, "newagent"); err != nil {
				t.Fatalf("serving adapter was not discovered: %v", err)
			}
			if value, err := deps.configValue("watch.cap-min", "1"); err != nil || value != "230" {
				t.Fatalf("serving local setting = %q, %v", value, err)
			}
			if row.name == "unarmed worktree" {
				if _, err := OperationsAt(Deps{Root: root}, "newagent"); err == nil {
					t.Fatal("the worktree alone must have no external adapter")
				}
			}

			want := filepath.Join(row.serving, row.name, "install", "bin", "metasystem")
			if row.override != "" {
				want = row.override
			}
			if deps.Engine != want || deps.Dispatch.(EngineDispatcher).Engine != want || deps.Root != root || deps.ServingInstallation != filepath.Join(row.serving, row.name, "install") {
				t.Fatalf("engine %q, installation %q, root %q; want engine %q and root %q", deps.Engine, deps.ServingInstallation, deps.Root, want, root)
			}
		})
	}
}
