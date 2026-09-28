package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestValidateMovedEffectsReport drives the moved-effects owner that design
// review --check-only runs (the internal validate moved-effects printed the
// same lines): a row without a new owner is a problem, a complete row is
// reported with its code path checked, and a page without the inventory says
// so.
func TestValidateMovedEffectsReport(t *testing.T) {
	metasystem, _ := filepath.Abs("../..")
	repo, dir := filepath.Dir(metasystem), t.TempDir()
	write := func(name, body string) []byte {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	page := func(to string) string {
		return "## Moved effects\n| Effect | From | To | Code |\n|---|---|---|---|\n| effect | old | " + to + " | `metasystem/go.mod` |\n"
	}
	for _, test := range []struct {
		name     string
		page     []byte
		root     string
		problems bool
		want     string
	}{
		{"weak", write("weak.md", page("none")), repo, true, "MOVED-EFFECT-NO-NEW-OWNER"},
		{"valid", write("valid.md", page("new")), repo, false, "moved-effects: inventory=present rows=1 problems=0"},
		{"absent", write("absent.md", "# No inventory\n"), "..", false, "inventory=absent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lines, problems := movedEffectsReport(test.page, test.root)
			text := strings.Join(lines, "\n")
			if (problems > 0) != test.problems || !strings.Contains(text, test.want) {
				t.Fatalf("problems=%d lines=%q, want problems=%v containing %q", problems, lines, test.problems, test.want)
			}
			if test.name == "valid" && lines[len(lines)-1] != test.want {
				t.Fatalf("last line = %q", lines[len(lines)-1])
			}
		})
	}
	t.Run("default-root", func(t *testing.T) {
		t.Chdir(metasystem)
		lines, problems := movedEffectsReport(write("default-root.md", page("new")), "..")
		text := strings.Join(lines, "\n")
		if problems != 0 || !strings.Contains(text, "code metasystem/go.mod ok") || lines[len(lines)-1] != "moved-effects: inventory=present rows=1 problems=0" {
			t.Fatalf("problems=%d lines=%q", problems, lines)
		}
	})
}
