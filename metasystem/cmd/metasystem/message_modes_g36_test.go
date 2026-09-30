package main

import (
	"os"
	"path"
	"slices"
	"strings"
	"testing"
)

// Groups G3 (disk and evidence) and G6 (the passthroughs: receipt,
// experiment, session, test) of the round-2 rewrite. Every production file
// the traced reading gives either group speaks in the two lines of
// "Messages a Person Reads", in the direct and the traced reading alike.
// internal/config/context.go keeps CONTEXT_CONFIG_INVALID (the steward's
// health line reads it), so the direct reading leaves it out, as the
// system group does.
var (
	_ = enforceTracedMessages(groupSourceFiles("G3", "G6")...)
	_ = enforceMessages(slices.DeleteFunc(groupSourceFiles("G3", "G6"), func(file string) bool {
		return file == "internal/config/context.go"
	})...)
)

// groupSourceFiles is every production Go file of the module the traced
// reading assigns to one of groups, so a file added later is enforced too.
// Run from another directory (the test binary as a child) it finds none,
// which only the audits, run from this package, need.
func groupSourceFiles(groups ...string) []string {
	var files []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, err := os.ReadDir(path.Join("..", "..", dir))
		if err != nil {
			return
		}
		for _, entry := range entries {
			name := entry.Name()
			switch {
			case entry.IsDir() && name != "testdata":
				walk(dir + "/" + name)
			case !entry.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") &&
				slices.Contains(groups, messageTraceGroupOf(dir+"/"+name)):
				files = append(files, dir+"/"+name)
			}
		}
	}
	walk("internal")
	walk("cmd")
	return files
}

// TestAuditGroupsThreeAndSixEnforceTheirFiles: the computed list holds the
// groups' files when the audits run, so no file of either escapes them.
func TestAuditGroupsThreeAndSixEnforceTheirFiles(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"cmd/metasystem/intent_disk.go", "internal/evidence/person.go", "internal/gocache/resolve_domain.go",
		"cmd/metasystem/report.go", "cmd/metasystem/test_cold_budget.go", "cmd/devgate/main.go", "internal/validate/brief_bounds_source.go"} {
		if messageTracedModes[want] != auditEnforce || auditModes[want].messages != messageModeEnforce {
			t.Errorf("%s is not enforced in both readings", want)
		}
	}
	if auditModes["internal/config/context.go"].messages == messageModeEnforce {
		t.Error("internal/config/context.go keeps its protocol code, and the direct reading enforces it")
	}
}
