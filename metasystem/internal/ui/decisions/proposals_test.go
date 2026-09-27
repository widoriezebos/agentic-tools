package decisions

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// The inbox's one new kind: an act the Project Partner proposed and nobody has
// answered.
//
// The conversation is where a proposal is made and answered, and the wrong place
// to keep one that waits. What this page has to carry is everything a human
// needs before they press — the verb's own word, the subject, every argument,
// the Partner's explanation and the version the press must present — and the
// state a line already stands in, so a refused or unresolved row keeps its words
// and its recovery across the re-read that follows a run.

// proposedPark is one park the Partner proposed on the goal named, waiting.
func proposedPark(turn string, index int, goalID, state, words string, version int) partner.Unsettled {
	return partner.Unsettled{Turn: turn, Proposal: partner.Proposal{
		Index: index, Verb: "park-goal", Goal: goalID, Title: "What " + goalID + " is for",
		Fields:  map[string]string{"because": "superseded by the seat inventory (g1-s42)"},
		Why:     "the inventory covers what these were for",
		Offered: true, State: state, Words: words, Version: version,
		At: ago(2 * time.Hour),
	}}
}

// proposingInputs is a workspace with one goal at the tip and the proposals
// given, and nothing else waiting on anybody.
func proposingInputs(proposals ...partner.Unsettled) Inputs {
	subject := row("g1-s44", "The seat census answers which machines are alive", backlog.LaneWaiting)
	return Inputs{
		Rows:      []backlog.Row{subject},
		Proposals: proposals,
		Human:     Standing{Proven: true},
		Since:     observed.Add(-24 * time.Hour),
	}
}

// One proposal is one row, carrying the whole action and the coordinate that
// names it.
func TestAProposalIsARowCarryingTheWholeAction(t *testing.T) {
	t.Parallel()
	page := Compose(proposingInputs(proposedPark("t7", 2, "g1-s44", partner.ProposalWaiting, "", 1)), observed)

	need := needOf(t, page, KindProposal)
	testutil.Expect(t, "the id is the conversation's own coordinate", need.ID, "t7/2")
	testutil.Expect(t, "the title is the subject as the pages say it",
		need.Title, "What g1-s44 is for")
	testutil.Expect(t, "what is asked is the verb's word, the subject and every argument",
		need.Asked, "Not now · What g1-s44 is for · Because: superseded by the seat inventory (g1-s42)")
	testutil.Expect(t, "who asks", need.By, "the Partner")
	testutil.Expect(t, "since when", need.Since, ago(2*time.Hour))
	testutil.Expect(t, "what silence does", need.Silence, "it stays proposed; nothing is applied")
	testutil.Expect(t, "where it opens", need.Where, Where{Kind: WhereGoal, ID: "g1-s44"})
	testutil.Expect(t, "and the act the press makes", need.Act, ActApply)
	testutil.Expect(t, "it counts in the inbox", page.Counts.NeedsYou, 1)
	testutil.Expect(t, "as something asked rather than an approval waiting", page.Counts.Asked, 1)

	testutil.Require(t, "the row carries the action", need.Proposal != nil, true)
	testutil.Expect(t, "the turn and the index the outcome route names",
		[]int{need.Proposal.Index, need.Proposal.Version}, []int{2, 1})
	testutil.Expect(t, "the turn", need.Proposal.Turn, "t7")
	testutil.Expect(t, "the verb is the route id the runner dispatches on", need.Proposal.Verb, "park-goal")
	testutil.Expect(t, "the arguments come whole under the route body's names",
		need.Proposal.Fields, map[string]string{"because": "superseded by the seat inventory (g1-s42)"})
	testutil.Expect(t, "the Partner's own words are the explanation",
		need.Proposal.Explanation, "the inventory covers what these were for")
	testutil.Expect(t, "the state it stands in", need.Proposal.State, partner.ProposalWaiting)
	testutil.Expect(t, "and the schema says which shape this is", page.SchemaVersion, 6)
}

// The goal's own row is joined where the ledger carries it, and nothing is
// invented where it does not: an `open` proposes a goal that is not there yet.
func TestAProposalJoinsTheGoalsRowWhereTheLedgerHasIt(t *testing.T) {
	t.Parallel()
	opened := partner.Unsettled{Turn: "t8", Proposal: partner.Proposal{
		Index: 0, Verb: "open-goal", Goal: "refund-worker", Title: "Every refund lands within a day",
		Fields:  map[string]string{"intent": "Every refund lands within a day.", "severity": "2"},
		Offered: true, State: partner.ProposalWaiting, Version: 1, At: ago(time.Hour),
	}}
	page := Compose(proposingInputs(
		proposedPark("t7", 0, "g1-s44", partner.ProposalWaiting, "", 1),
		opened,
	), observed)

	joined, unjoined := Need{}, Need{}
	for _, need := range page.NeedsYou {
		if need.Kind != KindProposal {
			continue
		}
		if need.ID == "t7/0" {
			joined = need
		}
		if need.ID == "t8/0" {
			unjoined = need
		}
	}
	testutil.Require(t, "the row of the goal at the tip is joined", joined.Row != nil, true)
	testutil.Expect(t, "and it is that goal's row", joined.Row.ID, "g1-s44")
	testutil.Expect(t, "the row of a goal nobody has yet is nil", unjoined.Row == nil, true)
	testutil.Expect(t, "and what is asked still says what would happen",
		unjoined.Asked,
		"Open goal · Every refund lands within a day · Intent: Every refund lands within a day. · Severity: 2")
}

// A line's own state travels, so a refused or unresolved row keeps its words
// after the re-read that follows a run, and a line a page left in flight says so.
func TestAProposalCarriesTheStateItStandsIn(t *testing.T) {
	t.Parallel()
	page := Compose(proposingInputs(
		proposedPark("t7", 0, "g1-s44", partner.ProposalRefused, "goal g1-s44 is claimed by m2a", 3),
		proposedPark("t7", 1, "g1-s44", partner.ProposalApplying, "", 2),
	), observed)

	states := map[string]*Proposed{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindProposal {
			states[need.ID] = need.Proposal
		}
	}
	testutil.Require(t, "both rows are listed", len(states), 2)
	testutil.Expect(t, "the refusal's state", states["t7/0"].State, partner.ProposalRefused)
	testutil.Expect(t, "and the engine's own sentence with it",
		states["t7/0"].Words, "goal g1-s44 is claimed by m2a")
	testutil.Expect(t, "the version every press presents", states["t7/0"].Version, 3)
	testutil.Expect(t, "the line a page left in flight", states["t7/1"].State, partner.ProposalApplying)
}

// New follows the proposal's own instant, as it does for every kind.
func TestAProposalIsNewByWhenItWasProposed(t *testing.T) {
	t.Parallel()
	in := proposingInputs(
		proposedPark("t7", 0, "g1-s44", partner.ProposalWaiting, "", 1),
		proposedPark("t6", 0, "g1-s44", partner.ProposalWaiting, "", 1),
	)
	in.Proposals[1].At = ago(3 * 24 * time.Hour)
	in.Since = observed.Add(-12 * time.Hour)

	page := Compose(in, observed)

	fresh := map[string]bool{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindProposal {
			fresh[need.ID] = need.New
		}
	}
	testutil.Expect(t, "the one proposed inside the window is new", fresh["t7/0"], true)
	testutil.Expect(t, "the one from three days ago is not", fresh["t6/0"], false)
}

// The row dates the asking and not the last press, and the last press travels
// beside it.
//
// It is the same claim the "new" test makes, from the other end: a line that has
// been answered once — refused, unresolved, left in flight — has been written
// since it was proposed, and a row dated by that write would sort to the end of
// its group and read "today" at exactly the moment it started to carry a
// recovery (Sol's read of g1-s60, deferred). The group is read oldest asking
// first, so the row that has waited longest leads whatever has been pressed on
// it since.
func TestAProposalsRowIsDatedByWhenItWasProposedAndNotByTheLastWrite(t *testing.T) {
	t.Parallel()
	answered := proposedPark("t6", 0, "g1-s44", partner.ProposalRefused, "goal g1-s44 is claimed by m2a", 3)
	answered.At = ago(3 * 24 * time.Hour)
	answered.UpdatedAt = ago(time.Minute)
	untouched := proposedPark("t7", 0, "g1-s44", partner.ProposalWaiting, "", 1)

	page := Compose(proposingInputs(untouched, answered), observed)

	rows := map[string]Need{}
	order := []string{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindProposal {
			rows[need.ID] = need
			order = append(order, need.ID)
		}
	}
	testutil.Require(t, "both rows are listed", len(rows), 2)
	testutil.Expect(t, "the answered row is dated by its asking three days ago",
		rows["t6/0"].Since, ago(3*24*time.Hour))
	testutil.Expect(t, "and carries the instant of the write that answered it",
		rows["t6/0"].Proposal.UpdatedAt, ago(time.Minute))
	testutil.Expect(t, "a row nothing has written carries no write instant",
		rows["t7/0"].Proposal.UpdatedAt, "")
	// A row dated by the write would have gone to the end of the group; dated by
	// the asking, the three-day-old refusal is still the one that has waited
	// longest.
	testutil.Expect(t, "oldest asking first", order, []string{"t6/0", "t7/0"})
}

// A seat with no Partner has no proposals and no group, and every other kind is
// composed exactly as it was.
func TestNoProposalsIsNoRowsAtAll(t *testing.T) {
	t.Parallel()
	page := Compose(proposingInputs(), observed)

	for _, need := range page.NeedsYou {
		if need.Kind == KindProposal {
			t.Fatalf("a page with no proposals invented a row: %+v", need)
		}
	}
	testutil.Expect(t, "and no row of any other kind carries an action",
		everyProposalMemberIsNil(page), true)
}

func everyProposalMemberIsNil(page Page) bool {
	for _, need := range page.NeedsYou {
		if need.Proposal != nil {
			return false
		}
	}
	return true
}

// The word on the row is the button word of the page that offers that act, for
// every one of the nine: the row, the card and the button say one thing.
func TestTheWordOnAProposalRowIsThePagesOwnButtonWord(t *testing.T) {
	t.Parallel()
	for verb, word := range map[string]string{
		"park-goal":         "Not now",
		"unpark-goal":       "Return to queue",
		"approve-goal":      "Approve",
		"withdraw-goal":     "Withdraw approval",
		"set-goal-priority": "Set priority",
		"open-goal":         "Open goal",
		"edit-goal":         "Edit",
		"block-goal":        "Waits for",
		"unblock-goal":      "No longer waits for",
	} {
		testutil.Expect(t, "the word for "+verb, proposalWord(verb), word)
	}
	testutil.Expect(t, "a verb this build has no word for says the route id",
		proposalWord("retire-goal"), "retire-goal")
}
