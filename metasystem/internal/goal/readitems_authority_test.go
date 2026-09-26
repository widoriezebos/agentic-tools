package goal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// TestReadItemChangesOnProtectedGoalsNeedAProvenPerson covers the owner's
// authority for read items: the owning pair changes its own claimed goal, and
// another pair's claim or a parked goal takes a proven person. A person's
// name without the proof, and another agent, change nothing.
func TestReadItemChangesOnProtectedGoalsNeedAProvenPerson(t *testing.T) {
	t.Parallel()
	endpoint, peer := readItemBed(t, "resting")
	if res, err := openClaimForTest(t, readItemRequest(endpoint, 10), "held", "Held work.", OriginMain, "Build it.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim: %+v %v", res, err)
	}
	if res, err := Park(readItemRequest(endpoint, 11), "resting", "waiting for review"); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("park: %+v %v", res, err)
	}
	if res, err := AddReadItems(readItemRequest(endpoint, 12), "held", "critic", []string{"The owning pair records its own note."}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("the owning pair's add: %+v %v", res, err)
	}

	foreign := func(sequence int) VerbRequest {
		return verbReqFor(endpoint, fmt.Sprintf("01J5X%021d", 100+sequence), "mac-b")
	}
	named := func(sequence int) VerbRequest {
		request := foreign(sequence)
		request.Actor.Human = "Wido"
		return request
	}
	refused := func(what string, run func() (PublishResult, error), want string) {
		t.Helper()
		before := snapshotTerminalLedger(t, endpoint, peer)
		res, err := run()
		detail := res.Detail
		if err != nil {
			detail = err.Error()
		}
		if res.Outcome == OutcomeConfirmed || !strings.Contains(detail, want) {
			t.Fatalf("%s = %+v %v; want a refusal naming %q", what, res, err, want)
		}
		if after := snapshotTerminalLedger(t, endpoint, peer); !reflect.DeepEqual(before, after) {
			t.Fatalf("%s changed the ledger", what)
		}
	}
	refused("another agent's add on a claimed goal", func() (PublishResult, error) {
		return AddReadItems(foreign(1), "held", "critic", []string{"Not mine."})
	}, "is a human act")
	refused("a name without proof on another pair's claim", func() (PublishResult, error) {
		return AddReadItems(named(2), "held", "critic", []string{"Named only."})
	}, "no human authority proof accompanied it")
	refused("a name without proof on a parked goal", func() (PublishResult, error) {
		return AddReadItems(named(3), "resting", "critic", []string{"Named only."})
	}, "no human authority proof accompanied it")
	reason := "accepted by a person"
	refused("a name without proof closing on another pair's claim", func() (PublishResult, error) {
		return CloseReadItem(named(4), "held", "critic-1", ReadItemClosure{Accepted: &reason})
	}, "no human authority proof accompanied it")

	proven := func(sequence int) VerbRequest {
		request := named(sequence)
		request.Authority = testTerminalAuthority(t, endpoint.Root, request.Now)
		return request
	}
	if res, err := AddReadItems(proven(5), "resting", "critic", []string{"A proven person notes the parked goal."}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a proven person's add on a parked goal: %+v %v", res, err)
	}
	if res, err := CloseReadItem(proven(6), "held", "critic-1", ReadItemClosure{Accepted: &reason}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("a proven person's close on another pair's claim: %+v %v", res, err)
	}
}
