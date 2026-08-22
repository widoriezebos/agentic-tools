package audit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTravel(t *testing.T, root, rel, content string) {
	t.Helper()
	abs := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestQualifiedNamesGuardTheTravelingSurfaces(t *testing.T) {
	root := t.TempDir()
	// A bare collision word on a traveling surface is a violation.
	writeTravel(t, root, "scripts/agents/templates/brief.md",
		"Resume the job when the critic returns.\n")
	// A file that QUALIFIES the word first owns its later bare uses.
	writeTravel(t, root, "skills/sample/SKILL.md",
		"Name the acceptance gate before editing.\nOnly the gate accepts a candidate.\n")
	// A qualified heading owns the body's later bare uses — headings
	// are read prose, not comments.
	writeTravel(t, root, "scripts/agents/roles/sample.md",
		"# The mission runner\nOnly the runner restarts a dead host turn.\n")
	violations, err := auditQualifiedNames(root)
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(violations, "\n")
	if !strings.Contains(joined, "templates/brief.md") || !strings.Contains(joined, "the job") {
		t.Fatalf("the bare word on the traveling surface is named: %v", violations)
	}
	if strings.Contains(joined, "SKILL.md") {
		t.Fatalf("a file's first qualified use owns its later bare ones: %v", violations)
	}
	if strings.Contains(joined, "roles/sample.md") {
		t.Fatalf("a qualified heading owns the body: %v", violations)
	}
	if len(violations) != 1 {
		t.Fatalf("exactly the one violation: %v", violations)
	}
	// An absent surface directory is not an error (adopted targets
	// may lack skills entirely).
	if _, err := auditQualifiedNames(t.TempDir()); err != nil {
		t.Fatalf("empty root: %v", err)
	}
	// An UNREADABLE file refuses: a probe that cannot answer must
	// not read as clean prose.
	root2 := t.TempDir()
	writeTravel(t, root2, "skills/locked/SKILL.md", "the job\n")
	if err := os.Chmod(filepath.Join(root2, "skills", "locked", "SKILL.md"), 0o000); err != nil {
		t.Fatal(err)
	}
	if _, err := auditQualifiedNames(root2); err == nil {
		t.Fatal("an unreadable traveling file refuses the audit")
	}
}
