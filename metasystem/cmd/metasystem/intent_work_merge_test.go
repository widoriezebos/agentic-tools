package main

import (
	"testing"
	"time"
)

func TestWorkBedRetainsClaimCapabilityAndClock(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	file, _ := bed.acceptedGoal()
	claim := file.Claimed
	capability := file.StopCapability
	if claim == nil || capability == nil || capability.Generation != claim.Revision || capability.Revision != claim.Revision ||
		capability.Machine != claim.Machine || capability.ClaimEpoch != 1 {
		t.Fatalf("work fixture lost its claimed execution authority: claim=%+v capability=%+v", claim, capability)
	}
	claimedAt, err := time.Parse(time.RFC3339, claim.At)
	if err != nil || !claimedAt.Equal(time.Date(2026, 9, 1, 9, 55, 0, 0, time.UTC)) {
		t.Fatalf("claim does not share the work clock: %q %v", claim.At, err)
	}
	for _, row := range []struct {
		name, at string
		offset   time.Duration
	}{
		{"opened", file.OpenedAt, -5 * time.Minute},
		{"approved", file.Approved.At, time.Minute},
		{"open history", file.History[0].At, -5 * time.Minute},
		{"claim history", file.History[1].At, 0},
		{"approval history", file.History[2].At, time.Minute},
	} {
		at, err := time.Parse(time.RFC3339, row.at)
		if err != nil || at.Sub(claimedAt) != row.offset {
			t.Fatalf("%s time lost its relation to the claim: %q %v", row.name, row.at, err)
		}
	}
}
