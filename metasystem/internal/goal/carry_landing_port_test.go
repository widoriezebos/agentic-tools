package goal

// Ledger-owner behavior the former land-fixtures.sh carried scenarios
// reached through a real landing (carried-intent-failure,
// carried-red-battery, carried-debt-abandoned, carried-debt-expired); the
// landing path's side of those scenarios is in internal/landing/landpath.

import (
	"os"
	"strings"
	"testing"
	"time"
)

// carriedReservationFor opens a word and a reservation on goal g and returns
// the reservation row and the seat that holds it.
func carriedReservationFor(t *testing.T, base time.Time) (endpoint, other Endpoint, seat VerbRequest, word CarryArgs, ref, row string) {
	t.Helper()
	endpoint, other, human, _, word, ref := openCarryWordForAbandonTestFor(t, base)
	declareWordHistory(t, endpoint, ref, "")
	seat = carryVerb(human, "01J5X00000000000000000E001", 2)
	seat.Actor.Human = ""
	result, row, err := Carrying(seat, CarryingArgs{Goal: "g", ApprovedRef: ref, Workspace: word.Workspace, Project: strings.Repeat("b", 40)})
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("open reservation: %+v %q %v", result, row, err)
	}
	return endpoint, other, seat, word, ref, row
}

// TestCarryDebtUnrecordedAfterTheReservationEnds covers carried-debt-abandoned
// and carried-debt-expired: seat A's carried commit reached origin and its
// seat stopped before the ledger row. While the reservation is open the debt
// is the in-flight row and its seat; once the row is abandoned, or the word
// and row expire, the debt is the word landed without its ledger row, which
// every other seat's reservation is refused on.
func TestCarryDebtUnrecordedAfterTheReservationEnds(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	t.Run("abandoned", func(t *testing.T) {
		t.Parallel()
		endpoint, other, seat, _, ref, row := carriedReservationFor(t, base)
		commit := fakeCodeCarryCommit(t, other, ref)
		tree, tip := acceptedTreeForEndpoint(t, endpoint)
		debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", seat.Now)
		if err != nil || !found || debt.Kind != "inflight" || debt.ID != row || !strings.Contains(debt.Detail, "seat=mac-a") {
			t.Fatalf("in-flight debt = %+v %v %v", debt, found, err)
		}
		abandon := carryVerb(seat, "01J5X00000000000000000E002", 1)
		if result, err := AbandonCarrying(abandon, "g", row, "fixture releases the crashed reservation"); err != nil || result.Outcome != OutcomeConfirmed {
			t.Fatalf("abandon: %+v %v", result, err)
		}
		declareWordHistory(t, endpoint, ref, commit)
		tree, tip = acceptedTreeForEndpoint(t, endpoint)
		if state := CarryReservationAt(tree, "g", ref, abandon.Now); state.State != "abandoned" || state.History.Opid != row {
			t.Fatalf("reservation after abandon = %+v", state)
		}
		debt, found, err = carryDebtAtFor(endpoint, tree, tip, "", abandon.Now)
		if err != nil || !found || debt.Kind != "unrecorded" || debt.ID != ref || debt.Detail != commit {
			t.Fatalf("unrecorded debt after abandon = %+v %v %v", debt, found, err)
		}
		if text := carryDebtText(debt); !strings.Contains(text, "carries word "+ref+" without its ledger row") {
			t.Fatalf("debt text %q", text)
		}
	})
	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		endpoint, other, _, word, ref, row := carriedReservationFor(t, base)
		commit := fakeCodeCarryCommit(t, other, ref)
		declareWordHistory(t, endpoint, ref, commit)
		later := word.Expires.Add(time.Hour)
		// The debt is read at the code tip: the pushed commit.
		tree, _ := acceptedTreeForEndpoint(t, endpoint)
		tip := commit
		if state := CarryReservationAt(tree, "g", ref, later); state.State != "expired" || state.History.Opid != row {
			t.Fatalf("reservation after expiry = %+v", state)
		}
		debt, found, err := carryDebtAtFor(endpoint, tree, tip, "", later)
		if err != nil || !found || debt.Kind != "unrecorded" || debt.ID != ref || debt.Detail != commit {
			t.Fatalf("unrecorded debt after expiry = %+v %v %v", debt, found, err)
		}
	})
}

// TestCarriedIntentBeforePushClosesWithoutARow covers carried-intent-failure:
// a landing that stops before its push first completes its local intent (as
// its cleanup does), which cannot record a commit origin does not hold; the
// intent ends terminal, no carried row is written, and the reservation can
// then be abandoned although the intent's owner process is still alive.
func TestCarriedIntentBeforePushClosesWithoutARow(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	endpoint, other, seat, word, ref, row := carriedReservationFor(t, base)
	repository := other.Repository.(*fakeGoalRepository)
	parent, err := repository.Capture(ref)
	if err != nil {
		t.Fatal(err)
	}
	commit, err := repository.Build(ref+"-unpushed", parent, []Change{{Path: "carried-unpushed.txt", Content: []byte("local\n")}}, "carried change")
	if err != nil {
		t.Fatal(err)
	}
	_, ledger := acceptedTreeForEndpoint(t, endpoint)
	intent := CarryingArgs{Goal: "g", ApprovedRef: ref, Carrying: row, Commit: commit, Project: strings.Repeat("b", 40), Workspace: word.Workspace,
		Past: word.Past, Battery: "green", Missing: "-", Failing: "-", Judge: "live", JudgeDigest: strings.Repeat("e", 64), Ledger: ledger,
		By: "human:wido", OwnerPID: int64(os.Getpid())}
	result, entry, err := Carrying(carryVerb(seat, "01J5X00000000000000000E011", 1), intent)
	if err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("carried intent: %+v %q %v", result, entry, err)
	}
	declareWordHistory(t, endpoint, ref, "")
	completed, err := Carried(carryVerb(seat, "01J5X00000000000000000E012", 2), entry)
	if err == nil && completed.Outcome == OutcomeConfirmed || !strings.Contains(fmtCarried(completed, err), "is not on origin") {
		t.Fatalf("completing an unpushed intent = %+v %v", completed, err)
	}
	closed, err := ReadEntry(endpoint.Root, entry)
	if err != nil || closed.Phase != PhaseTerminal {
		t.Fatalf("the unpushed intent stayed open: %+v %v", closed, err)
	}
	abandon := carryVerb(seat, "01J5X00000000000000000E013", 3)
	if result, err := AbandonCarrying(abandon, "g", row, "carried landing exited before its push (status 143)"); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("abandon after the intent closed: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	if _, _, found := carriedRow(tree, ref); found {
		t.Fatal("an unpushed commit was recorded as carried")
	}
	if state := CarryReservationAt(tree, "g", ref, abandon.Now); state.State != "abandoned" {
		t.Fatalf("reservation = %+v", state)
	}
}

func fmtCarried(result PublishResult, err error) string {
	if err != nil {
		return err.Error()
	}
	return result.Detail
}

// TestCarriedRedBatteryOpensTheRedObligation covers carried-red-battery: a
// carried landing recorded with a red battery opens the review obligation
// carried:<commit>:battery-red on the human-carried chain and counts one
// budget exception.
func TestCarriedRedBatteryOpensTheRedObligation(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 9, 13, 9, 0, 0, 0, time.UTC)
	endpoint, other, seat, word, ref, row := carriedReservationFor(t, base)
	_, ledger := acceptedTreeForEndpoint(t, endpoint)
	commit := fakeCodeCarryCommit(t, other, ref)
	declareWordHistory(t, endpoint, ref, commit)
	record := carryVerb(seat, "01J5X00000000000000000E021", 3)
	record.Endpoint.ConfigureCarriedCounselorAppend(func(string, string, HistoryLine, time.Time) error { return nil })
	args := CarriedArgs{Goal: "g", ApprovedRef: ref, Carrying: row, Commit: commit, Project: strings.Repeat("b", 40), Workspace: word.Workspace,
		Past: word.Past, Battery: "red", Missing: "fixture-carry", Failing: "-", Judge: "live", JudgeDigest: strings.Repeat("d", 64), Ledger: ledger, By: "human:wido"}
	if result, err := CarriedFromCommit(record, args); err != nil || result.Outcome != OutcomeConfirmed {
		t.Fatalf("record red carried landing: %+v %v", result, err)
	}
	tree, _ := acceptedTreeForEndpoint(t, endpoint)
	file := tree.Live["g"]
	if len(file.ReviewObligations) != 1 || file.ReviewObligations[0].Finding != "carried:"+commit+":battery-red" ||
		file.ReviewObligations[0].Chain != HumanCarriedChain || file.ReviewObligations[0].Artifact != "commit:"+commit || file.BudgetExceptions != 1 {
		t.Fatalf("red obligation = %+v exceptions=%d", file.ReviewObligations, file.BudgetExceptions)
	}
}
