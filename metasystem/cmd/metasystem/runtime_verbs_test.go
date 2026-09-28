package main

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/runtimes"
)

// The runtime family's pinned contract — exit codes 0/1/2, empty
// stdout on absent capabilities — is table-tested for every verb
// shape.
func TestRuntimeVerbContract(t *testing.T) {
	capture := func(f func([]string) int, args []string) (int, string) {
		code, stdout, _ := captureCommandOutput(t, true, false, func() int { return f(args) })
		return code, stdout
	}
	rows := []struct {
		name   string
		verb   func([]string) int
		args   []string
		code   int
		stdout string // "" means MUST be empty; "*" means non-empty
	}{
		{"list", runRuntimeList, nil, 0, joinLines(runtimes.Names())},
		{"list adoptable", runRuntimeList, []string{"--adoptable"}, 0, joinLines(runtimes.Adoptable())},
		{"list with-adapter", runRuntimeList, []string{"--with-adapter"}, 0, joinLines(runtimes.WithAdapter())},
		{"list with-common-lifecycle", runRuntimeList, []string{"--with-common-lifecycle"}, 0, joinLines(runtimes.WithCommonLifecycle())},
		{"collision-roots", runRuntimeCollisionRoots, nil, 0, joinLines(runtimes.CollisionRootsAll())},
		{"list usage", runRuntimeList, []string{"--bogus"}, 2, ""},
		{"dirs", runRuntimeDirs, []string{"devin"}, 0, ".agents/skills\n.devin/skills\n.devin/agents\n"},
		{"dirs unknown", runRuntimeDirs, []string{"ghostrt"}, 1, ""},
		{"dirs usage", runRuntimeDirs, nil, 2, ""},
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

// joinLines derives the expected population from the declarations —
// a valid new runtime must not fail shared core tests; only
// RELATIONAL policies are pinned elsewhere.
func joinLines(names []string) string {
	out := ""
	for _, name := range names {
		out += name + "\n"
	}
	return out
}
