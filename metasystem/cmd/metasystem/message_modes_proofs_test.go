package main

import (
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// Group 4 of the message rewrite: proofs, tests and gates. Each path below
// speaks in the two lines of "Messages a Person Reads". No file is left out:
// the landing owner reads a child's --json envelope (internal/verbresult),
// never its words, so no message is machine protocol.
var _ = enforceMessages(slices.Concat(
	packageFilesExcept("internal/testrun", ""),
	packageFilesExcept("internal/proofrun", ""),
	packageFilesExcept("cmd/metasystem", "^(proof_run|test|testing|validate_verbs)"),
	[]string{
		"cmd/devgate",
		"internal/audit",
		"internal/candidateengine",
		"internal/enginecause",
		"internal/gaterun",
		"internal/testenv",
		"internal/testpolicy",
		"internal/validate",
	})...)

// packageFilesExcept is every production Go file of a package directory
// (module-relative) whose name matches prefix (a regular expression; empty
// matches all) but the named ones, so a file added later is enforced too.
func packageFilesExcept(dir, prefix string, except ...string) []string {
	match := regexp.MustCompile(prefix)
	// The test binary also runs as a child from other directories (engine
	// workers, fixtures); there the directory is not found and nothing is
	// enforced, which only the audit, run from this package, needs.
	// TestAuditProofsGroupEnforcesItsPackages guards the audit's own run.
	entries, err := os.ReadDir(path.Join("..", "..", dir))
	if err != nil {
		return nil
	}
	var files []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || slices.Contains(except, name) || !match.MatchString(name) {
			continue
		}
		files = append(files, dir+"/"+name)
	}
	return files
}

// TestAuditProofsGroupEnforcesItsPackages: the computed lists hold the
// packages' files when the audit runs, so no file of the group escapes it.
func TestAuditProofsGroupEnforcesItsPackages(t *testing.T) {
	t.Parallel()
	for _, want := range []string{"internal/proofrun/attempt.go", "internal/testrun/prepare.go", "cmd/metasystem/proof_run.go", "cmd/metasystem/test.go"} {
		if auditModes[want].messages != messageModeEnforce {
			t.Errorf("%s is not enforced", want)
		}
	}
	// The frozen protocol files are gone (structured-output U1): their
	// refusals are ordinary messages, read by a parent through the envelope.
	for _, protocol := range []string{"internal/proofrun/protocol.go", "internal/testrun/protocol.go", "cmd/metasystem/proof_run_protocol.go"} {
		if _, err := os.Stat(path.Join("..", "..", protocol)); err == nil {
			t.Errorf("%s still holds frozen protocol text", protocol)
		}
	}
}
