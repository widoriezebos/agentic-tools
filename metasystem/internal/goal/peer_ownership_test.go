package goal

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// TestPeerOwnershipIsProjectedOncePerTip (R26; the read's F-3): the goals'
// ownership a peer message is delivered by is projected once per accepted
// tip and shared by every seat of the host through a cache under
// ~/.metasystem/host/ keyed by the tip's SHA; N reads at one tip are one
// projection, a moved tip is one more; a missing or unreadable cache is
// recomputed; an unresolved tip is an empty ledger and projects nothing.
func TestPeerOwnershipIsProjectedOncePerTip(t *testing.T) {
	t.Parallel()
	home := filepath.Join(t.TempDir(), ".metasystem")
	tip := "aaaa"
	tipOf := func(string) (string, bool, error) { return tip, true, nil }
	projections := 0
	project := func(_, at string) (board.Ownership, error) {
		projections++
		return board.Ownership{Live: map[string]string{"goal-x": "m1b@" + at}, Concluded: map[string]string{"goal-old": "done on 2026-09-20"}}, nil
	}
	for range 5 {
		ownership, err := peerOwnership("/repo-a", home, tipOf, project)
		if err != nil || ownership.Live["goal-x"] != "m1b@aaaa" || ownership.Concluded["goal-old"] != "done on 2026-09-20" {
			t.Fatalf("ownership = %+v, %v", ownership, err)
		}
	}
	// Another seat of the host at the same tip shares the projection.
	if _, err := peerOwnership("/repo-b", home, tipOf, project); err != nil || projections != 1 {
		t.Fatalf("%d projections across six reads at one tip, want 1 (%v)", projections, err)
	}
	info, err := os.Stat(filepath.Join(home, "host", "peer-ownership.json"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("the cache file: %v %v", info, err)
	}
	tip = "bbbb"
	if ownership, _ := peerOwnership("/repo-a", home, tipOf, project); projections != 2 || ownership.Live["goal-x"] != "m1b@bbbb" {
		t.Fatalf("after the tip moved: %d projections, %+v", projections, ownership)
	}
	if err := os.WriteFile(filepath.Join(home, "host", "peer-ownership.json"), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := peerOwnership("/repo-a", home, tipOf, project); err != nil || projections != 3 {
		t.Fatalf("an unreadable cache: %d projections, %v", projections, err)
	}
	unresolved := func(string) (string, bool, error) { return "", false, nil }
	if ownership, err := peerOwnership("/repo-a", home, unresolved, project); err != nil || len(ownership.Live) != 0 || projections != 3 {
		t.Fatalf("an unresolved tip: %+v %v, %d projections", ownership, err, projections)
	}
	failing := func(string) (string, bool, error) { return "", false, errors.New("broken ref") }
	if _, err := peerOwnership("/repo-a", home, failing, project); err == nil {
		t.Fatal("a broken accepted ref read as an empty ledger")
	}
}
