package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfValue(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "metasystem.conf")
	os.WriteFile(conf, []byte(strings_join([]string{
		"# a comment",
		"",
		"watch.interval-sec=60",
		"metasystem.runtimes=claude,codex",
		"watch.interval-sec=90", // last wins
		"  spaced.key = value ",
		"noequalsline",
	})), 0o644)

	cases := []struct{ key, def, want string }{
		{"watch.interval-sec", "1", "90"}, // last matching wins
		{"metasystem.runtimes", "", "claude,codex"},
		{"spaced.key", "", "value"},            // trimmed both sides
		{"absent.key", "fallback", "fallback"}, // default on missing key
	}
	for _, c := range cases {
		if got := ConfValue(conf, c.key, c.def); got != c.want {
			t.Fatalf("ConfValue(%q) = %q, want %q", c.key, got, c.want)
		}
	}
	if got := ConfValue(filepath.Join(dir, "nope.conf"), "any", "def"); got != "def" {
		t.Fatalf("missing file must return default, got %q", got)
	}
}

func strings_join(lines []string) string {
	out := ""
	for i, l := range lines {
		if i > 0 {
			out += "\n"
		}
		out += l
	}
	return out
}
