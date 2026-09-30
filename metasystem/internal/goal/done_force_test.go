package goal

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// forced is the request made the person's forced conclusion at the helm.
func forced(r VerbRequest) VerbRequest {
	r.ForceBy = "wido"
	return r
}

// forcedJournalRebuilds pins that a forced done journals its force and that
// recovery rebuilds the same forced request from the entry.
func forcedJournalRebuilds(t *testing.T, r VerbRequest, id, conclusion string) {
	t.Helper()
	publish := doneRequest(r, id, conclusion)
	if publish.Intent.Args["force"] != r.ForceBy || publish.Intent.Args["conclusion"] != conclusion {
		t.Fatalf("the forced intent args %v", publish.Intent.Args)
	}
	entry := Entry{Opid: publish.Opid, Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: publish.Intent}
	rebuilt, err := requestForEntry(r.Endpoint, entry)
	// The directing name is attribution only and never replayed; the
	// conclusion and its force are.
	if err != nil || rebuilt.Opid != publish.Opid || rebuilt.Intent.Verb != "done" ||
		rebuilt.Intent.Args["force"] != r.ForceBy || rebuilt.Intent.Args["conclusion"] != conclusion {
		t.Fatalf("recovery did not rebuild the forced request: %+v %v", rebuilt.Intent, err)
	}
}

func archivedGoal(t *testing.T, endpoint Endpoint, id string) *GoalFile {
	t.Helper()
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	file := tree.Done[id]
	if file == nil || tree.Live[id] != nil {
		t.Fatalf("goal %s is not archived as done", id)
	}
	return file
}

func TestForcedDoneOverridesOpenReadItems(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	if added, err := AddReadItems(readItemRequest(endpoint, 70), "source", "read-z", []string{"Explain the fallback.", "Name the invariant."}); err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	plain, err := Done(readItemRequest(endpoint, 71), "source", "Built.")
	want := (&DoneReadItemsOpenError{Goal: "source", ItemIDs: []string{"read-z-1", "read-z-2"}}).Error()
	if err != nil || plain.Outcome != OutcomeRejected || plain.Detail != want || !ConclusionOverridable(plain.Detail) {
		t.Fatalf("plain done: %+v %v", plain, err)
	}
	request := forced(readItemRequest(endpoint, 72))
	forcedJournalRebuilds(t, request, "source", "Built.")
	result, err := Done(request, "source", "Built.")
	if err != nil || result.Outcome != OutcomeConfirmed ||
		result.Detail != "accepted read items: read-z-1: overridden by wido at the helm; read-z-2: overridden by wido at the helm" {
		t.Fatalf("forced done: %+v %v", result, err)
	}
	file := archivedGoal(t, endpoint, "source")
	if file.Conclude != "Built. — overridden by wido at the helm: read items read-z-1, read-z-2" {
		t.Fatalf("conclusion %q", file.Conclude)
	}
	for _, item := range file.ReadItems {
		if item.State != ReadItemAccepted || item.ClosingReference != "overridden by wido at the helm" || item.ChangedAt != request.stamp() {
			t.Fatalf("item %+v", item)
		}
	}
	// A forced done repeated is already done, as any repeat.
	again, err := Done(forced(readItemRequest(endpoint, 73)), "source", "Built.")
	if err != nil || again.Outcome != OutcomeAbandoned || !strings.Contains(again.Detail, "is already done") || ConclusionOverridable(again.Detail) {
		t.Fatalf("forced repeat: %+v %v", again, err)
	}
}

func TestForcedDoneOverridesAnOpenReviewObligation(t *testing.T) {
	t.Parallel()
	endpoint := obligationAuthorityLocalEndpoint(t, "review-goal")
	req := verbReqFor(endpoint, "01J5X00000000000000000FR10", "mac-a")
	obligation := ReviewObligation{Finding: "F-1", Chain: "critic-a", Artifact: "metasystem/a.go", Test: "prove: it works", State: "open"}
	if result, err := DeferFindings(req, "review-goal", []ReviewObligation{obligation}); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("defer = %+v, %v", result, err)
	}
	req.Ulid = "01J5X00000000000000000FR11"
	plain, err := Done(req, "review-goal", "landed by hand")
	if err != nil || plain.Outcome != OutcomeRejected || plain.Detail != "goal review-goal has open review obligation finding=F-1 chain=critic-a test=prove: it works" || !ConclusionOverridable(plain.Detail) {
		t.Fatalf("plain done: %+v %v", plain, err)
	}
	req.Ulid = "01J5X00000000000000000FR12"
	request := forced(req)
	forcedJournalRebuilds(t, request, "review-goal", "landed by hand")
	if result, err := Done(request, "review-goal", "landed by hand"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("forced done: %+v %v", result, err)
	}
	file := archivedGoal(t, endpoint, "review-goal")
	if file.Conclude != "landed by hand — overridden by wido at the helm: review obligation F-1/critic-a" ||
		len(file.ReviewObligations) != 1 || file.ReviewObligations[0].State != "discharged" {
		t.Fatalf("archive %q %+v", file.Conclude, file.ReviewObligations)
	}
}

func TestForcedDoneOverridesAnOpenCarryWord(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	endpoint, _, human, _, _, ref := openCarryWordForAbandonTestFor(t, base)
	declareWordHistory(t, endpoint, ref, "")
	plain, err := Done(carryVerb(human, "01J5X00000000000000000FC01", 2), "g", "landed by hand")
	if err != nil || plain.Outcome != OutcomeRejected || !strings.HasPrefix(plain.Detail, "goal g has an open exception ("+ref+"); land it, replace it or let it expire") || !ConclusionOverridable(plain.Detail) {
		t.Fatalf("plain done: %+v %v", plain, err)
	}
	request := forced(carryVerb(human, "01J5X00000000000000000FC02", 3))
	forcedJournalRebuilds(t, request, "g", "landed by hand")
	if result, err := Done(request, "g", "landed by hand"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("forced done: %+v %v", result, err)
	}
	if file := archivedGoal(t, endpoint, "g"); file.Conclude != "landed by hand — overridden by wido at the helm: carry word "+ref {
		t.Fatalf("conclusion %q", file.Conclude)
	}
}

// HF-02: only the open word is passed over; a failed inspection refuses under
// force and publishes nothing.
func TestForcedDoneRefusesWhenTheCarryWordCannotBeInspected(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	endpoint, _, human, _, _, ref := openCarryWordForAbandonTestFor(t, base)
	repository := endpoint.Repository.(*fakeGoalRepository)
	repository.store.mu.Lock()
	codeTip := repository.store.canonical
	repository.store.mu.Unlock()
	declareCarryHistory(t, endpoint, codeTip, "Goal-Transaction", ref, "")
	repository.store.mu.Lock()
	repository.historyQueries[fakeHistoryQuery{codeTip, "Goal-Transaction", ref}].err = errors.New("trailer read failed")
	repository.store.mu.Unlock()
	_, before := acceptedTreeForEndpoint(t, endpoint)
	result, err := Done(forced(carryVerb(human, "01J5X00000000000000000FC11", 2)), "g", "landed by hand")
	if err != nil || result.Outcome != OutcomeRejected || result.Detail != "trailer read failed" || ConclusionOverridable(result.Detail) {
		t.Fatalf("forced done over an inspection error: %+v %v", result, err)
	}
	if _, after := acceptedTreeForEndpoint(t, endpoint); after != before {
		t.Fatal("a refused forced done published")
	}
}

// HF-01: force removes the unfinished blocker edge in the same transaction,
// so the published tree validates; the archive names the blocker.
func TestForcedDoneOverridesAnUnfinishedBlocker(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "blocker", "source")
	root := endpoint.Root
	if result, err := Block(personReqForEdges(endpoint, "01J5X00000000000000000FB01", "mac-a"), "source", "blocker", personProof(t, root)); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("block: %+v %v", result, err)
	}
	person := personReqForEdges(endpoint, "01J5X00000000000000000FB02", "mac-a")
	person.Authority = testHumanAuthority(t, root, person.Now)
	plain, err := Done(person, "source", "landed by hand")
	if err != nil || plain.Outcome != OutcomeRejected || plain.Detail != "goal source is blocked by blocker, which is not done" || !ConclusionOverridable(plain.Detail) {
		t.Fatalf("plain done: %+v %v", plain, err)
	}
	person.Ulid = "01J5X00000000000000000FB03"
	request := forced(person)
	forcedJournalRebuilds(t, request, "source", "landed by hand")
	if result, err := Done(request, "source", "landed by hand"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("forced done: %+v %v", result, err)
	}
	file := archivedGoal(t, endpoint, "source")
	if file.Conclude != "landed by hand — overridden by wido at the helm: blocked by blocker" || len(file.Blocked) != 0 {
		t.Fatalf("archive %q blocked %v", file.Conclude, file.Blocked)
	}
	if tree, _ := acceptedTreeForEndpoint(t, endpoint); tree.Live["blocker"] == nil {
		t.Fatal("the blocker left the live tree")
	}
}

func TestForcedDoneLeavesTheOtherRefusalsAndFollowUps(t *testing.T) {
	t.Parallel()
	endpoint, _ := readItemBed(t, "source")
	absent, err := Done(forced(readItemRequest(endpoint, 80)), "absent", "x")
	if err != nil || absent.Outcome != OutcomeRejected || absent.Detail != "goal absent is not live; nothing to conclude" || ConclusionOverridable(absent.Detail) {
		t.Fatalf("forced done of an absent goal: %+v %v", absent, err)
	}
	if added, err := AddReadItems(readItemRequest(endpoint, 81), "source", "read-z", []string{"One decision."}); err != nil || added.Outcome != OutcomeConfirmed {
		t.Fatalf("add: %+v %v", added, err)
	}
	request := forced(readItemRequest(endpoint, 82))
	request.SweepBranch = func(string) error { return errors.New("the branch tip has unlanded commits") }
	result, err := Done(request, "source", "Built.")
	if result.Outcome != OutcomeConfirmed || err == nil || !strings.Contains(err.Error(), "goal done confirmed but its branch was not swept: the branch tip has unlanded commits") {
		t.Fatalf("forced done with a kept branch: %+v %v", result, err)
	}
	for _, detail := range []string{"winner: x", "goal g is already done (x)", "goal g is not live; nothing to conclude",
		"goal done: the human authority proof for conclusion of a parked goal is not valid for this checkout"} {
		if ConclusionOverridable(detail) {
			t.Fatalf("%q is overridable", detail)
		}
	}
}
