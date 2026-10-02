package plain

import "testing"

// TestLaneCheckDeliberatelyRed exists only to check that the landing lane
// returns a red branch to its seat (2026-10-02 real run). It never lands.
func TestLaneCheckDeliberatelyRed(t *testing.T) {
	t.Fatal("deliberately red: the landing lane must return this branch")
}
