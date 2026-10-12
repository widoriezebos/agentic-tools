package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
)

// A rebuild made after the read that covered its unit is unread: the hand-in names it and offers the read that covers it.
func TestWorkLandRebuildAfterCoveringReadStaysUnread(t *testing.T) {
	t.Parallel()
	page := filepath.Join(t.TempDir(), "design.md")
	if err := os.WriteFile(page, []byte("| Unit | Lines |\n| --- | --- |\n| U1 | 100 |\n| U2 | 100 |\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a, b, c := strings.Repeat("a", 40), strings.Repeat("b", 40), strings.Repeat("c", 40)
	state := intentBranchState{BranchTip: c, Sources: []string{"critic-root", "critic-root"}, Status: branch.Status{Prefix: 2,
		Units: []branch.UnitStatus{{Unit: "U1", Units: []string{"U1"}, Commit: a, ReadState: "read clean"},
			{Unit: "U2", Units: []string{"U2"}, Commit: b, ReadState: "read clean"},
			{Unit: "U2", Units: []string{"U2"}, Commit: c, ReadState: "needs read"}},
		CleanReads: []branch.Attestation{{CoveredUnits: []string{"U1", "U2"}}}}}
	subject, n, refusal := handLandingSubject(nil, "g", "", state, false, true, page)
	if refusal == nil {
		t.Fatalf("unread later build admitted: subject=%s units=%d", subject, n)
	}
	if !strings.Contains(refusal.Summary, "U2 has no clean read") ||
		!slices.Equal(refusal.next, []string{"metasystem", "work", "review", "--commit", c, "--goal", "g", "--units", "U2"}) {
		t.Fatalf("the rebuild's refusal lost its unit or its remedy: %s next=%v", refusal.Summary, refusal.next)
	}
}
