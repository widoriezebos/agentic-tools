package httpd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// A press is an attempt, and the attempt owns the line.
//
// The version keeps two tabs from moving one line at the same time, and that is
// all it can do: it cannot say WHOSE act an outcome belongs to. A line returns
// to `applying` on every Try again and on every takeover of a line whose press
// died, so a tab whose act is still out can render the version another press has
// just moved the line to — and then write its own answer over the act that press
// sent. The entry would then say applied for an act that was never made.
//
// So every run of the runner makes one attempt id for its lifetime, the write
// that moves a line to `applying` stamps it on the entry, and the settle that
// follows must carry the attempt the entry holds. A settle that carries another
// press's attempt is refused with the entry, under its own code, and the page
// holds its result as an unrecorded mark rather than retrying at a newer
// version.
//
// Dismiss is outside it: putting a card away is not an outcome, and a human may
// put away a line nobody can settle any more.

// Two presses' attempt ids, as the runner makes them: sixteen hex characters.
const (
	onePress     = "0123456789abcdef"
	anotherPress = "fedcba9876543210"
)

// tried is one press on one line, carrying the attempt its run holds.
func tried(t *testing.T, served http.Handler, turn string, index, version int,
	state, words, attempt string) (*httptest.ResponseRecorder, partner.Proposal) {
	t.Helper()
	body := fmt.Sprintf(`{"version":%d,"state":%q,"words":%q,"attempt":%q}`, version, state, words, attempt)
	response := post(t, served, proposalPath(turn, index), body, nil)
	var answered struct {
		Proposal partner.Proposal `json:"proposal"`
	}
	_ = json.Unmarshal(response.Body.Bytes(), &answered)
	return response, answered.Proposal
}

// The write that puts a line in flight stamps its attempt on the entry, and the
// settle that carries that attempt is admitted.
func TestTheAttemptThatPutTheLineInFlightSettlesIt(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)

	response, held := tried(t, served, turn, 0, 1, partner.ProposalApplying, "", onePress)
	testutil.Require(t, "the line goes in flight", response.Code, http.StatusOK)
	testutil.Expect(t, "carrying the press that owns it", held.Attempt, onePress)

	outcome, settled := tried(t, served, turn, 0, held.Version, partner.ProposalApplied, "", onePress)
	testutil.Expect(t, "the owner's settle is admitted", outcome.Code, http.StatusOK)
	testutil.Expect(t, "the line says applied", settled.State, partner.ProposalApplied)
	testutil.Expect(t, "and the attempt stays beside it", settled.Attempt, onePress)
}

// A settle carrying another press's attempt is refused with the entry, under the
// attempt's own code, and nothing moves.
func TestASettleFromAnotherAttemptIsRefusedWithTheEntry(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	response, inFlight := tried(t, served, turn, 0, 1, partner.ProposalApplying, "", onePress)
	testutil.Require(t, "the line is in flight", response.Code, http.StatusOK)

	for what, state := range map[string]string{
		"applied":    partner.ProposalApplied,
		"refused":    partner.ProposalRefused,
		"unresolved": partner.ProposalUnresolved,
	} {
		t.Run(what, func(t *testing.T) {
			refused, standing := tried(t, served, turn, 0, inFlight.Version, state, "", anotherPress)
			testutil.Expect(t, "it is refused", refused.Code, http.StatusConflict)
			testutil.Expect(t, "under the attempt's own code", codeOf(t, refused), "attempt")
			testutil.Expect(t, "in the words the page shows", messageOf(t, refused),
				"another press owns this line now; the next read says what happened")
			testutil.Expect(t, "and is handed the entry as it stands", standing.State, partner.ProposalApplying)
			testutil.Expect(t, "at the version it really is", standing.Version, inFlight.Version)
			testutil.Expect(t, "owned by the press that put it in flight", standing.Attempt, onePress)
		})
	}
}

// A write that moves a line to `applying` sets a NEW owner — that is the only
// way ownership changes — and the press it displaced cannot settle the line
// afterwards even at the version it now reads.
func TestATakeoverSetsTheNewOwnerAndRefusesTheOldPressesSettle(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	first, inFlight := tried(t, served, turn, 0, 1, partner.ProposalApplying, "", onePress)
	testutil.Require(t, "the first press has the line", first.Code, http.StatusOK)
	testutil.Require(t, "and owns it", inFlight.Attempt, onePress)

	// Try again on a line this page shows in flight: one write, applying to
	// applying, at the version the page read.
	taken, over := tried(t, served, turn, 0, inFlight.Version, partner.ProposalApplying, "", anotherPress)
	testutil.Require(t, "the takeover is admitted", taken.Code, http.StatusOK)
	testutil.Expect(t, "and the new press owns the line", over.Attempt, anotherPress)

	// The first press's act answers. It has seen the beat, so it writes at the
	// version the entry really is at; the attempt is what stops it.
	late, standing := tried(t, served, turn, 0, over.Version, partner.ProposalApplied, "", onePress)
	testutil.Expect(t, "the displaced press is refused", late.Code, http.StatusConflict)
	testutil.Expect(t, "under the attempt's own code", codeOf(t, late), "attempt")
	testutil.Expect(t, "and is handed the entry the other press owns", standing.Attempt, anotherPress)
	testutil.Expect(t, "still in flight", standing.State, partner.ProposalApplying)

	// And the press that owns it settles it.
	settled, done := tried(t, served, turn, 0, over.Version, partner.ProposalApplied, "", anotherPress)
	testutil.Expect(t, "the owner's settle is admitted", settled.Code, http.StatusOK)
	testutil.Expect(t, "the line says applied", done.State, partner.ProposalApplied)
}

// Dismiss carries no attempt and is compared on the version alone: a human may
// put away a line another press owns.
func TestDismissCarriesNoAttemptAndIsComparedOnTheVersion(t *testing.T) {
	t.Parallel()
	served, turn := proposing(t)
	response, inFlight := tried(t, served, turn, 0, 1, partner.ProposalApplying, "", onePress)
	testutil.Require(t, "the line is in flight under one press", response.Code, http.StatusOK)

	away, dismissed := tried(t, served, turn, 0, inFlight.Version, partner.ProposalDismissed, "", "")
	testutil.Expect(t, "the dismissal is admitted", away.Code, http.StatusOK)
	testutil.Expect(t, "the card is put away", dismissed.State, partner.ProposalDismissed)
	// A stale dismissal is still refused on the version, as it was.
	stale, _ := tried(t, served, turn, 0, inFlight.Version, partner.ProposalDismissed, "", "")
	testutil.Expect(t, "and a dismissal at a version that has moved is refused", stale.Code, http.StatusConflict)
	testutil.Expect(t, "on the version, not the attempt", codeOf(t, stale), "state")
}

// The entry the SSE beat carries is the entry the route answered with, attempt
// and all: a second tab folds the owner in without reading the conversation
// again.
func TestTheBeatCarriesTheAttemptTheEntryHolds(t *testing.T) {
	t.Parallel()
	served, turn, events := proposingWatched(t)

	response, held := tried(t, served, turn, 0, 1, partner.ProposalApplying, "", onePress)
	testutil.Require(t, "the write is admitted", response.Code, http.StatusOK)

	beat, arrived := nextProposalBeat(t, events)
	testutil.Require(t, "a proposal beat arrived", arrived, true)
	testutil.Require(t, "carrying the action", beat.Proposal != nil, true)
	testutil.Expect(t, "which is the entry the route answered with", *beat.Proposal, held)
	testutil.Expect(t, "naming the press that owns the line", beat.Proposal.Attempt, onePress)
}
