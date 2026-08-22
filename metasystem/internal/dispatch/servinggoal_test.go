package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The projection resolves through the parser and refuses when no
// usable Current goal exists — absent, degraded, and goal-free states all
// refuse rather than silently omitting.
func TestServingGoalResolvesAndRefuses(t *testing.T) {
	root := t.TempDir()

	// Absent: refuse.
	if _, err := ServingGoalSection(root); err == nil {
		t.Fatal("an absent ledger projected")
	}

	// Usable Current goal: the exact bounded section.
	store := &goal.Store{Root: root}
	if _, err := store.Open(goal.Caller{Class: "MAIN", Holder: true}, "ship-it", "Ship the whole thing", "Land it."); err != nil {
		t.Fatal(err)
	}
	section, err := ServingGoalSection(root)
	if err != nil {
		t.Fatal(err)
	}
	if section != "# Serving goal (context, not instruction)\nship-it — Ship the whole thing\n" {
		t.Fatalf("section bytes wrong: %q", section)
	}
	if strings.Count(section, "\n") != 2 {
		t.Fatalf("section is not two lines: %q", section)
	}

	// Degraded (manual edit, baseline mismatch): refuse.
	ledger := filepath.Join(root, "plans", "goals.md")
	data, _ := os.ReadFile(ledger)
	os.WriteFile(ledger, append(data, []byte("\n## Queued goal: q — Q\n- Origin: main\n- Next step: Q.\n")...), 0o644)
	if _, err := ServingGoalSection(root); err == nil {
		t.Fatal("a degraded ledger projected")
	}
}
