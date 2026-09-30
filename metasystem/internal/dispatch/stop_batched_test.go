package dispatch

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A claim whose change waits in a landing batch is not breach-stopped for
// elapsed time, as a claim queued to land is not: a stuck lane must not
// stop the goal it holds. Out of the batch, the elapsed fence binds again.
func TestAClaimInALandingBatchIsNotStoppedForElapsedTime(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	templateAdmissionBed(t, bed)
	pastGrace := time.Date(2026, 8, 29, 22, 0, 0, 0, time.UTC)
	batched := false
	bed.reads.LandingBatched = func(root, id string, now time.Time) bool { return batched && id == "bounded" }
	routes, err := bed.stops(pastGrace)
	if err != nil || len(routes) != 1 || routes[0].Reason != goal.StopReasonElapsedLimit {
		t.Fatalf("outside a batch the elapsed breach routes to a stop: %+v %v", routes, err)
	}
	batched = true
	if routes, err := bed.stops(pastGrace); err != nil || len(routes) != 0 {
		t.Fatalf("a claim waiting in a landing batch was routed to a breach stop: %+v %v", routes, err)
	}
}
