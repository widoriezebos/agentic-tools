package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// Code critique finding 12: the runtime family's pinned contract —
// exit codes 0/1/2, empty stdout on absent capabilities — is
// table-tested for every verb shape.
func TestRuntimeVerbContract(t *testing.T) {
	capture := func(f func([]string) int, args []string) (int, string) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		code := f(args)
		w.Close()
		os.Stdout = old
		var buf bytes.Buffer
		buf.ReadFrom(r)
		return code, buf.String()
	}
	rows := []struct {
		name   string
		verb   func([]string) int
		args   []string
		code   int
		stdout string // "" means MUST be empty; "*" means non-empty
	}{
		{"list", runRuntimeList, nil, 0, "codex\ndevin\nclaude\nfake\n"},
		{"list adoptable", runRuntimeList, []string{"--adoptable"}, 0, "codex\ndevin\nclaude\n"},
		{"list usage", runRuntimeList, []string{"--bogus"}, 2, ""},
		{"adoption-default", runRuntimeAdoptionDefault, nil, 0, "claude\n"},
		{"adoption-default usage", runRuntimeAdoptionDefault, []string{"x"}, 2, ""},
		{"dirs", runRuntimeDirs, []string{"devin"}, 0, ".agents/skills\n.devin/skills\n.devin/agents\n"},
		{"dirs unknown", runRuntimeDirs, []string{"ghostrt"}, 1, ""},
		{"dirs usage", runRuntimeDirs, nil, 2, ""},
		{"enforcement-config", runRuntimeEnforcementConfig, []string{"codex"}, 0, "codex-hooks.json\n"},
		{"enforcement-config absent", runRuntimeEnforcementConfig, []string{"fake"}, 1, ""},
		{"self-check", runRuntimeSelfCheck, []string{"claude"}, 0, "$CLAUDE_PROJECT_DIR/metasystem\n"},
		{"self-check absent", runRuntimeSelfCheck, []string{"codex"}, 1, ""},
		{"instruction-file", runRuntimeInstructionFile, []string{"claude"}, 0, "CLAUDE.md\n"},
		{"session-env", runRuntimeSessionEnv, []string{"devin"}, 0, "DEVIN_PROJECT_DIR\n"},
		{"session-env absent", runRuntimeSessionEnv, []string{"codex"}, 1, ""},
		{"config-identity-filter", runRuntimeConfigIdentityFilter, []string{"claude"}, 0, "claude-config-filter.v1.json\n"},
		{"config-identity-filter absent", runRuntimeConfigIdentityFilter, []string{"fake"}, 1, ""},
		{"config-identity-filter unknown", runRuntimeConfigIdentityFilter, []string{"nope"}, 1, ""},
	}
	for _, row := range rows {
		t.Run(strings.ReplaceAll(row.name, " ", "-"), func(t *testing.T) {
			code, out := capture(row.verb, row.args)
			if code != row.code {
				t.Fatalf("exit %d, want %d", code, row.code)
			}
			if out != row.stdout {
				t.Fatalf("stdout %q, want %q", out, row.stdout)
			}
		})
	}
}
