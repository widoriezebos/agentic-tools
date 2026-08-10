package validate

import (
	"path/filepath"
	"strings"
	"testing"
)

func quoteFixture(t *testing.T, quoted string) (root, rolesDir string) {
	t.Helper()
	root = t.TempDir()
	writeFile(t, filepath.Join(root, "docs", "source.md"), "# Rules\n\nThe exact rule holds.\n")
	rolesDir = filepath.Join(root, "scripts", "agents", "roles")
	writeFile(t, filepath.Join(rolesDir, "role.md"),
		"# Role\n\n<!-- quote source=\"docs/source.md\" -->\n"+quoted+"<!-- /quote -->\n")
	return root, rolesDir
}

func TestPreambleQuotesAccepts(t *testing.T) {
	root, rolesDir := quoteFixture(t, "The exact rule holds.\n")
	if violations := PreambleQuotes(root, rolesDir); len(violations) != 0 {
		t.Fatalf("byte-exact quote flagged: %v", violations)
	}
}

func TestPreambleQuotesRejectsDrift(t *testing.T) {
	root, rolesDir := quoteFixture(t, "The exact rule held.\n")
	violations := PreambleQuotes(root, rolesDir)
	if len(violations) != 1 || !strings.Contains(violations[0], "quote drifted from docs/source.md") {
		t.Fatalf("violations = %v, want one drift", violations)
	}
}

func TestPreambleQuotesRejectsEscapingSource(t *testing.T) {
	root, rolesDir := quoteFixture(t, "The exact rule holds.\n")
	writeFile(t, filepath.Join(rolesDir, "sneaky.md"),
		"<!-- quote source=\"../outside.md\" -->\nanything\n<!-- /quote -->\n")
	violations := PreambleQuotes(root, rolesDir)
	if len(violations) != 1 || !strings.Contains(violations[0], "quote source escapes the metasystem root: ../outside.md") {
		t.Fatalf("violations = %v, want one escape", violations)
	}
}

func TestPreambleQuotesRejectsMissingBlockAndUnpairedMarkers(t *testing.T) {
	root, rolesDir := quoteFixture(t, "The exact rule holds.\n")
	writeFile(t, filepath.Join(rolesDir, "bare.md"), "# No quotes here\n")
	violations := PreambleQuotes(root, rolesDir)
	if len(violations) != 1 || !strings.Contains(violations[0], "no verbatim quote block") {
		t.Fatalf("violations = %v, want one missing block", violations)
	}

	writeFile(t, filepath.Join(rolesDir, "bare.md"),
		"<!-- quote source=\"docs/source.md\" -->\nThe exact rule holds.\n<!-- /quote -->\n"+
			"<!-- quote source=\"docs/source.md\" -->\nunclosed\n")
	violations = PreambleQuotes(root, rolesDir)
	if len(violations) != 1 || !strings.Contains(violations[0], "malformed or unpaired quote marker") {
		t.Fatalf("violations = %v, want one unpaired marker", violations)
	}
}
