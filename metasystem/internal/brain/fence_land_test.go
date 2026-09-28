package brain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFenceLandTexts ports the owner half of the former land fixture
// brain-land-refuses: a declared brain's landing fence names the refusal a
// landing prints, and an unreadable declaration fences landing with the
// remedy that names the repair.
func TestFenceLandTexts(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(root)), 0o755); err != nil {
		t.Fatal(err)
	}
	record := Record{Schema: 1, Ledger: testLedger, Machine: "brain", DeclaredBy: "Wido", DeclaredAt: "2026-09-07T00:00:00Z"}
	data, _ := json.Marshal(record)
	if err := os.WriteFile(Path(root), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := Fence(root, "land", testLedger); !strings.HasPrefix(got, "land refused: this checkout is declared the brain; the brain never lands") {
		t.Fatalf("declared land fence = %q", got)
	}
	if err := os.WriteFile(Path(root), []byte("{broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := Fence(root, "land", testLedger)
	for _, want := range []string{"this checkout's brain declaration is unreadable", "nothing here dispatches, lands",
		"metasystem settings coordinator --withdraw --by <name> --repo " + root} {
		if !strings.Contains(got, want) {
			t.Fatalf("corrupt land fence = %q, want %q", got, want)
		}
	}
}
