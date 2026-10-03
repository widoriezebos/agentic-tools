package main

import (
	"slices"
	"testing"
)

// No critic may read a tier-1 goal, so the status of its committed work
// names the landing as the next step, never a review the tier refuses.
func TestTierOneWorkIsSentToLandNotToReview(t *testing.T) {
	t.Parallel()
	inv := &intentInvocation{}
	next, reason := inv.manualContinuation("g1", manualWorkItem{Unit: "main", Commit: "abc", Goal: "g1", ReadsWaived: true})
	if !slices.Equal(next, []string{"metasystem", "work", "land", "g1"}) || reason == "" {
		t.Fatalf("tier-1 work continues with %q (%s); want metasystem work land g1", next, reason)
	}
}
