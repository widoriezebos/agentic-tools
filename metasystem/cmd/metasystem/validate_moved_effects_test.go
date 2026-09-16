package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMovedEffectsVerb(t *testing.T) {
	metasystem, _ := filepath.Abs("../..")
	repo, dir := filepath.Dir(metasystem), t.TempDir()
	write := func(name, body string) string {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	page := func(to string) string {
		return "## Moved effects\n| Effect | From | To | Code |\n|---|---|---|---|\n| effect | old | " + to + " | `metasystem/go.mod` |\n"
	}
	weak := write("weak.md", page("none"))
	valid := write("valid.md", page("new"))
	absent := write("absent.md", "# No inventory\n")
	for _, test := range []struct {
		name string
		args []string
		code int
		want string
	}{
		{"weak", []string{"--file", weak, "--root", repo}, 1, "MOVED-EFFECT-NO-NEW-OWNER"},
		{"valid", []string{"--file", valid, "--root", repo}, 0, "moved-effects: inventory=present rows=1 problems=0\n"},
		{"absent", []string{"--file", absent}, 0, "inventory=absent"},
		{"missing-file", nil, 2, "--file is required"},
	} {
		t.Run(test.name, func(t *testing.T) {
			code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
				return dispatch(append([]string{"validate", "moved-effects"}, test.args...))
			})
			output := stdout
			if test.code == 2 {
				output = stderr
			}
			if code != test.code || !strings.Contains(output, test.want) {
				t.Fatalf("code=%d stdout=%q stderr=%q, want code=%d containing %q", code, stdout, stderr, test.code, test.want)
			}
			if test.name == "valid" && !strings.HasSuffix(stdout, test.want) {
				t.Fatalf("last line = %q", stdout)
			}
		})
	}
	t.Run("default-root", func(t *testing.T) {
		t.Chdir(metasystem)
		defaultRoot := write("default-root.md", page("new"))
		code, stdout, stderr := captureCommandOutput(t, true, true, func() int {
			return dispatch([]string{"validate", "moved-effects", "--file", defaultRoot})
		})
		want := "moved-effects: inventory=present rows=1 problems=0\n"
		if code != 0 || stderr != "" || !strings.Contains(stdout, "code metasystem/go.mod ok") || !strings.HasSuffix(stdout, want) {
			t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
		}
	})
}
