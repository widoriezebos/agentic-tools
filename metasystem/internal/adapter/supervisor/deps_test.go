package supervisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProcessDepsTakesTheServingInstallationsEngine(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
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
			deps := processDeps(root, query, func(key string) (string, bool) {
				return row.override, key == "METASYSTEM_BIN" && row.override != ""
			})
			want := filepath.Join(row.serving, row.name, "install", "bin", "metasystem")
			if row.override != "" {
				want = row.override
			}
			if deps.Engine != want || deps.Dispatch.(EngineDispatcher).Engine != want || deps.Root != root {
				t.Fatalf("dependencies = %+v, want engine %q and root %q", deps, want, root)
			}
		})
	}
}
