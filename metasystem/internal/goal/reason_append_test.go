package goal

import (
	"testing"
)

func lastHistory(t *testing.T, endpoint Endpoint, tip, id string) HistoryLine {
	t.Helper()
	tree, err := loadTreeFor(endpoint, tip)
	if err != nil {
		t.Fatal(err)
	}
	file := tree.Live[id]
	if file == nil {
		t.Fatalf("goal %s is not live at %s", id, tip)
	}
	return file.History[len(file.History)-1]
}

// replay publishes a request the way journal recovery rebuilds it from its
// stored intent.
func replay(t *testing.T, endpoint Endpoint, request PublishRequest) PublishResult {
	t.Helper()
	rebuilt, err := requestForEntry(endpoint, Entry{Opid: request.Opid, Machine: request.Machine, Lineage: request.Lineage, Intent: request.Intent})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Publish(endpoint, rebuilt)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("replayed %s: %+v %v", request.Intent.Verb, res, err)
	}
	return res
}

func TestStealAndReleaseRecordTheirReason(t *testing.T) {
	t.Parallel()
	aEndpoint, bEndpoint := fakeGoalEndpointPair(t)
	if res, err := openClaimForTest(t, verbReqFor(aEndpoint, "01J5X0000000000000000000R1", "mac-a"), "wanted", "Wanted work.", "main", "Go.", testBudget()); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open --claim: %+v %v", res, err)
	}
	humanReq := verbReqFor(bEndpoint, "01J5X0000000000000000000R2", "mac-b")
	humanReq.Actor.Human = "wido"
	if _, err := StealWithReason(humanReq, "wanted", "two\nlines"); err == nil {
		t.Fatal("a reason with a line break was accepted")
	}
	request := stealRequestWithReason(humanReq, "wanted", "the holder is gone for the day")
	if request.Intent.Args["reason"] != "the holder is gone for the day" || request.Intent.Args["by"] != "wido" {
		t.Fatalf("the journaled steal intent does not carry its reason: %+v", request.Intent.Args)
	}
	res, err := StealWithReason(humanReq, "wanted", "the holder is gone for the day")
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("steal: %+v %v", res, err)
	}
	if last := lastHistory(t, bEndpoint, res.Tip, "wanted"); last.Verb != "steal" || last.Reason != "the holder is gone for the day" {
		t.Fatalf("the steal line does not record its reason: %+v", last)
	}

	// The journal's replay of a release records the same reason.
	release := releaseRequestWithReason(verbReqFor(bEndpoint, "01J5X0000000000000000000R3", "mac-b"), "wanted", "handing it back")
	res = replay(t, bEndpoint, release)
	if last := lastHistory(t, bEndpoint, res.Tip, "wanted"); last.Verb != "release" || last.Reason != "handing it back" {
		t.Fatalf("the replayed release line does not record its reason: %+v", last)
	}
}

// TestEditNextStepAppendKeepsAConcurrentEdit interleaves a competing next-step
// edit between building an append and publishing it: the append is applied to
// the tip it publishes, so both edits survive, and a journal replay of the
// append carries the appended text, not a stale whole next step.
func TestEditNextStepAppendKeepsAConcurrentEdit(t *testing.T) {
	t.Parallel()
	aEndpoint, _ := fakeGoalEndpointPair(t)
	if res, err := Open(verbReqFor(aEndpoint, "01J5X0000000000000000000R4", "mac-a"), "drafting", "Draft it.", "main", "Draft."); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("open: %+v %v", res, err)
	}
	addition := "Then ship."
	appending, err := editRequest(verbReqFor(aEndpoint, "01J5X0000000000000000000R5", "mac-a"), "drafting", EditFields{NextStepAppend: &addition})
	if err != nil {
		t.Fatal(err)
	}
	competing := "Draft, then review."
	if res, err := Edit(verbReqFor(aEndpoint, "01J5X0000000000000000000R6", "mac-a"), "drafting", EditFields{NextStep: &competing}); err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("competing edit: %+v %v", res, err)
	}
	res, err := Publish(aEndpoint, appending)
	if err != nil || res.Outcome != OutcomeConfirmed {
		t.Fatalf("append: %+v %v", res, err)
	}
	tree, err := loadTreeFor(aEndpoint, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if next := tree.Live["drafting"].NextStep; next != "Draft, then review. Then ship." {
		t.Fatalf("the append lost the concurrent edit: %q", next)
	}

	again := "And announce it."
	stored, err := editRequest(verbReqFor(aEndpoint, "01J5X0000000000000000000R7", "mac-a"), "drafting", EditFields{NextStepAppend: &again})
	if err != nil {
		t.Fatal(err)
	}
	for _, delta := range stored.Intent.Deltas {
		if delta.Field == "next" {
			t.Fatalf("the journaled append stores a whole next step: %+v", stored.Intent.Deltas)
		}
	}
	res = replay(t, aEndpoint, stored)
	tree, err = loadTreeFor(aEndpoint, res.Tip)
	if err != nil {
		t.Fatal(err)
	}
	if next := tree.Live["drafting"].NextStep; next != "Draft, then review. Then ship. And announce it." {
		t.Fatalf("the replayed append = %q", next)
	}
	both := "x"
	if _, err := editRequest(verbReqFor(aEndpoint, "01J5X0000000000000000000R8", "mac-a"), "drafting", EditFields{NextStep: &both, NextStepAppend: &both}); err == nil {
		t.Fatal("an edit that both replaces and appends the next step was accepted")
	}
}
