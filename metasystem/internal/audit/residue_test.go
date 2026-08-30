package audit

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAuditResidueMarkers(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans", "goals"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "plans", "goals", "scheduled-item.md"), []byte("# goal\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	design := filepath.Join(root, "plans", "sample-design.md")
	body := "# Design\n" +
		"RESIDUE: the cache half waits — goal:scheduled-item\n" + // linked, resolves
		"prose residue words are not policed\n" + // free prose, ignored
		"RESIDUE: an unscheduled debt\n" + // no link
		"RESIDUE: linked to nothing real goal:ghost-item\n" // dangling
	if err := os.WriteFile(design, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	hits, err := auditResidueMarkers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("hits = %v, want the unlinked and dangling markers only", hits)
	}
	for _, hit := range hits {
		if hit == "" {
			t.Fatal("empty hit line")
		}
	}
	if want := "sample-design.md:4"; !contains(hits, want) {
		t.Fatalf("unlinked marker not reported by line: %v", hits)
	}
	if want := "goal:ghost-item"; !contains(hits, want) {
		t.Fatalf("dangling link not reported by id: %v", hits)
	}
}

func contains(hits []string, fragment string) bool {
	for _, hit := range hits {
		if len(hit) >= len(fragment) && (hit == fragment || indexOf(hit, fragment) >= 0) {
			return true
		}
	}
	return false
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestAuditResidueMarkersEdges(t *testing.T) {
	// No plans tree at all: a clean, empty sweep.
	empty := t.TempDir()
	hits, err := auditResidueMarkers(empty)
	if err != nil || len(hits) != 0 {
		t.Fatalf("bare root sweep = %v, %v", hits, err)
	}

	// An unreadable design doc refuses: an audit that cannot read its
	// subject must never report clean.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "plans"), 0o755); err != nil {
		t.Fatal(err)
	}
	blocked := filepath.Join(root, "plans", "blocked-design.md")
	if err := os.WriteFile(blocked, []byte("RESIDUE: hidden\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	if os.Getuid() != 0 {
		if _, err := auditResidueMarkers(root); err == nil {
			t.Fatal("an unreadable design doc must refuse the sweep")
		}
	}
	_ = os.Chmod(blocked, 0o644)
}
