package decisions

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// A row whose record leaves its naming fields empty is still named: by the
// ids the record does carry, never by an invented label.

// A proposal with no title is called by its goal's id, and a proposal with no
// arguments carries an empty object rather than an absent one.
func TestAnUntitledProposalIsCalledByItsGoalAndCarriesEmptyFields(t *testing.T) {
	t.Parallel()
	bare := partner.Unsettled{Turn: "t9", Proposal: partner.Proposal{
		Index: 0, Verb: "park-goal", Goal: "g1-s44", Title: "  ",
		Offered: true, State: partner.ProposalWaiting, Version: 1, At: ago(time.Hour),
	}}
	page := Compose(proposingInputs(bare), observed)

	need := needOf(t, page, KindProposal)
	testutil.Expect(t, "the title falls back to the goal's id", need.Title, "g1-s44")
	testutil.Expect(t, "what is asked is the verb's word and the subject alone", need.Asked, "Pause · g1-s44")
	testutil.Require(t, "the row carries the action", need.Proposal != nil, true)
	testutil.Expect(t, "no arguments is an empty object", need.Proposal.Fields, map[string]string{})
}

// A seat question that names no goal is called by its own id and asked by "a
// seat"; one that names a goal but no kind is called by the goal alone.
func TestASeatQuestionWithoutGoalKindOrMachineIsStillNamed(t *testing.T) {
	t.Parallel()
	in := Inputs{
		Human: Standing{Proven: true},
		Asks: []channel.Question{
			{ID: "ask-7", OpenedAt: observed.Add(-2 * time.Hour), State: "open", Wants: "a word on the order"},
			{ID: "ask-8", Goal: "g1-s50", Machine: "m1c", OpenedAt: observed.Add(-time.Hour), State: "open", Wants: "a go"},
		},
	}
	page := Compose(in, observed)

	byID := map[string]Need{}
	for _, need := range page.NeedsYou {
		if need.Kind == KindAsk {
			byID[need.ID] = need
		}
	}
	testutil.Require(t, "both questions are rows", len(byID), 2)
	testutil.Expect(t, "no goal: the question's own id", byID["ask-7"].Title, "ask-7")
	testutil.Expect(t, "no machine: a seat", byID["ask-7"].By, "a seat")
	testutil.Expect(t, "a goal and no kind: the goal alone", byID["ask-8"].Title, "g1-s50")
	testutil.Expect(t, "a machine: that seat", byID["ask-8"].By, "seat m1c")
}
