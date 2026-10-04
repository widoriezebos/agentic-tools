package dispatch

import (
	"path/filepath"
	"testing"
)

func TestResolveToolTakesTheEngineFromTheServingInstallation(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, root, installation, checkout, override, engine string
	}{
		{"primary checkout", "/primary/install", "/primary/install", "", "", "/primary/install/bin/metasystem"},
		{"armed linked worktree", "/goal/install", "/goal/install", "", "", "/goal/install/bin/metasystem"},
		{"unmapped root", "/unmapped", "/unmapped", "", "", "/unmapped/bin/metasystem"},
		{"unarmed linked worktree", "/goal/install", "/primary/install", "/primary", "", "/primary/install/bin/metasystem"},
		{"explicit override", "/goal/install", "/primary/install", "/primary", "/override/engine", "/override/engine"},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			serving := func(root string) (string, string) {
				if root != row.root {
					t.Fatalf("serving root = %q, want %q", root, row.root)
				}
				return row.installation, row.checkout
			}
			lookup := func(key string) (string, bool) {
				if key != "METASYSTEM_BIN" {
					t.Fatalf("environment key = %q", key)
				}
				return row.override, row.override != ""
			}
			installation, checkout, engine := ResolveTool(row.root, serving, lookup)
			if installation != row.installation || checkout != row.checkout || engine != filepath.FromSlash(row.engine) {
				t.Fatalf("tool = (%q, %q, %q), want (%q, %q, %q)", installation, checkout, engine, row.installation, row.checkout, row.engine)
			}
		})
	}
}
