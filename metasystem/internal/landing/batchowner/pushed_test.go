package batchowner

import (
	"errors"
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// TestSettlePushedRetriesAMemberWhoseFinalizeFailed (simple lane, rail 1;
// unit A critique): a push whose settlement recorded a member landed but
// failed to conclude its goal (the ledger write refused) leaves the member
// returning; the next landing push settles it, instead of skipping it and
// leaving its goal held by the lane with an empty queue.
func TestSettlePushedRetriesAMemberWhoseFinalizeFailed(t *testing.T) {
	t.Parallel()
	var tip string
	bed := newUnsetBedWithTip(t, func(bed *unsetBed) string { tip = bed.publishLanding(t); return tip })
	bed.publish = true
	// The lane checkout pushed tip, so it holds it.
	unsetGit(t, bed.checkout, "fetch", "-q", "origin")
	refused := errors.New("main moved under the ledger write")
	calls := LaneCallSet{
		Handover: func(ownercall.Invocation, ownercall.HandoverRequest) error {
			t.Fatalf("goal-a was handed back to a seat that holds another goal")
			return nil
		},
		EditNext: func(ownercall.Invocation, string, string, string) error { bed.edits++; return refused },
		Release: func(_ ownercall.Invocation, _ string, goalID string) error {
			bed.releases++
			bed.publishRelease(t, goalID)
			return nil
		},
	}
	request := PushedSettlement{Home: bed.home, Checkout: bed.checkout, Install: string(bed.layout.Install), Commit: tip, Actor: "lane", Now: unsetNow, Calls: &calls}
	if _, err := SettlePushed(request); !errors.Is(err, refused) {
		t.Fatalf("the first settlement = %v; want the refused finalization", err)
	}
	if unit := bed.unit(t, "goal-a"); unit.State != batch.UnitReturnPending || unit.Outcome != batch.UnitLanded || unit.P6Done {
		t.Fatalf("goal-a after a failed finalization = %s/%s p6=%v; want it returning as landed, unfinalized", unit.State, unit.Outcome, unit.P6Done)
	}
	calls.EditNext = func(ownercall.Invocation, string, string, string) error { bed.edits++; return nil }
	landed, err := SettlePushed(request)
	if err != nil || !slices.Contains(landed, "goal-a") {
		t.Fatalf("the retried settlement = %v %v; want goal-a landed", landed, err)
	}
	if unit := bed.unit(t, "goal-a"); unit.State != batch.UnitLanded || !unit.P6Done || unit.LandedCommit != tip || bed.releases != 1 {
		t.Fatalf("goal-a after the retry = %s p6=%v commit=%s releases=%d; want landed at %s and its goal released once", unit.State, unit.P6Done, unit.LandedCommit, bed.releases, tip)
	}
}
