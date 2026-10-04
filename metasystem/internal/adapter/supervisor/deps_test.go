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
	for _, row := range []struct {
		name, checkout, serving, armed string
	}{
		{"primary", primary, primary, ""},
		{"unarmed worktree", linked, primary, ""},
		{"armed worktree", linked, linked, "state.json"},
		{"censused worktree", linked, linked, "last-census.json"},
	} {
		t.Run(row.name, func(t *testing.T) {
			root := filepath.Join(row.checkout, row.name, "install")
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
				if dir == root && len(args) == 3 && args[2] == "--git-common-dir" {
					return filepath.Join(primary, ".git"), true
				}
				return git(dir, args...)
			}
			deps := processDeps(root, query, func(string) (string, bool) { return "", false })
			want := filepath.Join(row.serving, row.name, "install", "bin", "metasystem")
			if deps.Engine != want || deps.Dispatch.(EngineDispatcher).Engine != want || deps.Root != root {
				t.Fatalf("dependencies = %+v, want engine %q and root %q", deps, want, root)
			}
		})
	}
}
