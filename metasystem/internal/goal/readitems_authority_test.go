package goal

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestASeatMovesItsOwnNoteToTheGoalThatWaitsForIt(t *testing.T) {
	t.Parallel()
	endpoint, peer := readItemBed(t, "queued-src", "paused")
	confirmed := func(what string, result PublishResult, err error) {
		t.Helper()
		if err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("%s: %+v %v", what, result, err)
		}
	}
	result, err := openClaimForTest(t, readItemRequest(endpoint, 10), "held", "Held work.", OriginMain, "Build it.", testBudget())
	confirmed("claim held", result, err)
	result, err = AddReadItems(readItemRequest(endpoint, 11), "held", "astra", []string{"Keep this finding.", "Keep the next finding."})
	confirmed("note held", result, err)
	result, err = AddReadItems(readItemRequest(endpoint, 12), "queued-src", "astra", []string{"An unheld finding."})
	confirmed("note queued source", result, err)
	for i, source := range []string{"held", "queued-src"} {
		request := readItemRequest(endpoint, 20+i)
		request.Actor.Human = "Wido"
		budget := testBudget()
		result, err = OpenRisked(request, []string{"follow-up", "follow-up-2"}[i], "Follow up.", OriginHuman, "Wait.", nil, []string{source}, edgeRisk(), 0, "", &budget, sessionProofForTest(t, endpoint.Root, request.Now))
		confirmed("open follow-up", result, err)
	}
	request := readItemRequest(endpoint, 22)
	request.Actor.Human = "Wido"
	request.Authority = testTerminalAuthority(t, endpoint.Root, request.Now)
	result, err = Park(request, "paused", "A person's pause.")
	confirmed("park paused", result, err)
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	before := tree.Live["follow-up"]
	if before.Approved != nil || before.State != StateParked || before.Parked == nil {
		t.Fatalf("follow-up must be parked and unapproved: %+v", before)
	}
	target := "follow-up"
	result, err = CloseReadItem(readItemRequest(endpoint, 30), "held", "astra-1", ReadItemClosure{Moved: &target})
	confirmed("move own note", result, err)
	tree, _ = acceptedTreeForEndpoint(t, endpoint)
	held, follow := tree.Live["held"], tree.Live[target]
	var moved ReadItem
	for _, item := range held.ReadItems {
		if item.ID == "astra-1" {
			moved = item
		}
	}
	if moved.State != ReadItemMoved || moved.ClosingReference != target {
		t.Fatalf("source note: %+v", moved)
	}
	if len(follow.ReadItems) != 1 || follow.ReadItems[0].State != ReadItemOpen || !follow.ReadItems[0].isMaterial() || follow.ReadItems[0].Read != "astra" || follow.ReadItems[0].Text != "Keep this finding. (moved from held)" {
		t.Fatalf("moved note: %+v", follow.ReadItems)
	}
	if follow.State != StateParked || !reflect.DeepEqual(follow.Parked, before.Parked) || !reflect.DeepEqual(follow.Blocked, before.Blocked) || follow.Approved != nil || follow.Intent != before.Intent || follow.NextStep != before.NextStep {
		t.Fatalf("move changed follow-up fields: before=%+v after=%+v", before, follow)
	}
	if last := held.History[len(held.History)-1]; last.Displaced != "" {
		t.Fatalf("own move displaced the holder: %+v", last)
	}
	refused := func(what, want string, run func() (PublishResult, error)) {
		t.Helper()
		before := snapshotTerminalLedger(t, endpoint, peer)
		result, err := run()
		if err != nil || result.Outcome != OutcomeRejected || !strings.Contains(result.Detail, want) {
			t.Fatalf("%s = %+v %v; want %q", what, result, err, want)
		}
		if after := snapshotTerminalLedger(t, endpoint, peer); !reflect.DeepEqual(before, after) {
			t.Fatalf("%s changed the ledger", what)
		}
	}
	refused("unheld source", "Goal follow-up-2 is parked; only a person can move this note there", func() (PublishResult, error) {
		target := "follow-up-2"
		return CloseReadItem(readItemRequest(endpoint, 31), "queued-src", "astra-1", ReadItemClosure{Moved: &target})
	})
	refused("another pair's source", "is claimed by mac-a+lin-1", func() (PublishResult, error) {
		request := verbReqFor(endpoint, "01J5X0000000000000000000032", "mac-b")
		return CloseReadItem(request, "held", "astra-2", ReadItemClosure{Moved: &target})
	})
	refused("edit parked target", "editing a parked goal is a human act", func() (PublishResult, error) {
		intent := "Changed intent."
		return Edit(readItemRequest(endpoint, 33), target, EditFields{Intent: &intent})
	})
	refused("unrelated pause", "Goal paused does not wait for held; only a person can move this note there", func() (PublishResult, error) {
		target := "paused"
		return CloseReadItem(readItemRequest(endpoint, 34), "held", "astra-2", ReadItemClosure{Moved: &target})
	})
}

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
