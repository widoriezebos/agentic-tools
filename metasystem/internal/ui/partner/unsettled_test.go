package partner_test

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
)

// The reader the Decisions inbox composes its group from (g1-s60 D1).
//
// What it must get right is which lines are still a human's to answer. A
// proposal that was refused or came back unresolved is a choice waiting on
// somebody — Try again, or Dismiss — and the words beside it are the whole of
// what that choice rests on; a reader that returned waiting lines alone would
// take the row away at the moment it started to carry an explanation. A line
// left at `applying` is one a page went away in the middle of, and nothing will
// settle it by itself.

// unsettledLine is one proposed action as an answer carries it.
func unsettledLine(index int, verb, goalID, state, words string, version int) partner.Proposal {
	return partner.Proposal{
		Index: index, Verb: verb, Goal: goalID, Title: "What " + goalID + " is for",
		Fields:  map[string]string{"because": "superseded by the seat inventory"},
		Why:     "the inventory covers what these were for",
		Offered: true, State: state, Words: words, Version: version,
		At: "2026-09-26T09:0" + string(rune('0'+index)) + ":00Z",
	}
}

// transcriptOf is a service over one human's transcript, with the answers given
// appended in order. It needs no host: reading what was proposed asks the
// runtime nothing.
func transcriptOf(t *testing.T, messages ...partner.Message) *partner.Service {
	t.Helper()
	directory := t.TempDir()
	conversation, err := partner.OpenConversation(directory, "wido")
	testutil.Require(t, "opening the transcript", err, nil)
	for _, message := range messages {
		testutil.Require(t, "appending "+message.ID, conversation.Append(message), nil)
	}
	return partner.NewService(partner.Runtime{Name: "fake"}, nil,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(directory, human) },
		partner.Facts{}, func() time.Time { return at(t, "2026-09-26T12:00:00Z") })
}

// answered is one Partner message carrying the proposals given.
func answered(turn string, proposals ...partner.Proposal) partner.Message {
	return partner.Message{
		ID: turn + "-partner", Turn: turn, Role: partner.RolePartner,
		Text: "I have proposed them.", At: "2026-09-26T09:00:00Z", Outcome: "complete",
		Proposals: proposals,
	}
}

// Every line that is not applied and not dismissed comes back, with its state
// and the words that state carried, newest answer first.
func TestUnsettledIsEveryLineStillWaitingOnTheHuman(t *testing.T) {
	t.Parallel()
	service := transcriptOf(t,
		partner.Message{ID: "t1-human", Turn: "t1", Role: partner.RoleHuman, Text: "put them away"},
		answered("t1",
			unsettledLine(0, "park-goal", "g1-s44", partner.ProposalWaiting, "", 1),
			unsettledLine(1, "park-goal", "g1-s45", partner.ProposalRefused, "goal g1-s45 is claimed by m2a", 3),
			unsettledLine(2, "withdraw-goal", "g1-s14", partner.ProposalApplied, "", 3),
		),
		partner.Message{ID: "t2-human", Turn: "t2", Role: partner.RoleHuman, Text: "and this one"},
		answered("t2",
			unsettledLine(0, "approve-goal", "g1-s47", partner.ProposalUnresolved, "pushed; whether it landed is unresolved", 3),
			unsettledLine(1, "park-goal", "g1-s48", partner.ProposalApplying, "", 2),
			unsettledLine(2, "park-goal", "g1-s49", partner.ProposalDismissed, "", 2),
		),
	)

	held, err := service.Unsettled("wido")

	testutil.Require(t, "reading the transcript", err, nil)
	testutil.Require(t, "how many lines are still waiting", len(held), 4)
	// The newest answer first, and within it the order they were proposed.
	testutil.Expect(t, "the newest answer's unresolved line leads",
		[]string{held[0].Turn, held[0].Goal, held[0].State}, []string{"t2", "g1-s47", partner.ProposalUnresolved})
	testutil.Expect(t, "then the line a page left in flight",
		[]string{held[1].Turn, held[1].Goal, held[1].State}, []string{"t2", "g1-s48", partner.ProposalApplying})
	testutil.Expect(t, "then the older answer's waiting line",
		[]string{held[2].Turn, held[2].Goal, held[2].State}, []string{"t1", "g1-s44", partner.ProposalWaiting})
	testutil.Expect(t, "and its refused line",
		[]string{held[3].Turn, held[3].Goal, held[3].State}, []string{"t1", "g1-s45", partner.ProposalRefused})
	testutil.Expect(t, "the refusal's own words travel with it",
		held[3].Words, "goal g1-s45 is claimed by m2a")
	testutil.Expect(t, "and so does the version every press has to present", held[3].Version, 3)
	testutil.Expect(t, "the verb is the route id the runner dispatches on", held[3].Verb, "park-goal")
	testutil.Expect(t, "the arguments come whole", held[3].Fields["because"], "superseded by the seat inventory")
	testutil.Expect(t, "the explanation is the Partner's own words", held[3].Why,
		"the inventory covers what these were for")
	testutil.Expect(t, "the title is the one the pages say", held[3].Title, "What g1-s45 is for")
	testutil.Expect(t, "and the index names it under its turn", held[3].Index, 1)
	for _, one := range held {
		if one.State == partner.ProposalApplied || one.State == partner.ProposalDismissed {
			t.Fatalf("a settled line is still in the inbox's reading: %+v", one)
		}
	}
}

// An action the human was never offered is not a choice waiting on them: the
// refusal is something to read on the card, beside the answer that explains it.
func TestUnsettledLeavesOutAnActionNobodyWasOffered(t *testing.T) {
	t.Parallel()
	refused := unsettledLine(0, "park-goal", "g1-s90", partner.ProposalWaiting, "", 1)
	refused.Offered = false
	refused.Reason = "the accepted tip carries no goal g1-s90"
	service := transcriptOf(t, answered("t1", refused,
		unsettledLine(1, "park-goal", "g1-s44", partner.ProposalWaiting, "", 1)))

	held, err := service.Unsettled("wido")

	testutil.Require(t, "reading the transcript", err, nil)
	testutil.Require(t, "how many lines are waiting", len(held), 1)
	testutil.Expect(t, "and it is the one that was offered", held[0].Goal, "g1-s44")
}

// A proposal keeps the time it was PROPOSED, however often it is written.
//
// Every admitted write used to restamp the entry's At, so an answered row sorted
// to the end of its group and read "today": the inbox's ages were the ages of the
// human's own presses rather than of the asking, and a proposal from last week
// counted as new because somebody had just refused it (Sol's read of g1-s60,
// deferred). The instant of the write is not lost — it is stamped beside the
// asking's, because both facts are true of a line proposed on Monday and refused
// on Friday.
func TestAWriteStampsUpdatedAtAndLeavesTheProposalsOwnInstantAlone(t *testing.T) {
	t.Parallel()
	proposed := unsettledLine(0, "park-goal", "g1-s44", partner.ProposalWaiting, "", 1)
	service := transcriptOf(t, answered("t1", proposed))

	inFlight, err := service.Proposed("wido", "t1", 0, 1, partner.ProposalApplying, "")

	testutil.Require(t, "the write is admitted", err, nil)
	testutil.Expect(t, "the asking's own instant is untouched", inFlight.At, proposed.At)
	testutil.Expect(t, "and the write's instant stands beside it",
		inFlight.UpdatedAt, "2026-09-26T12:00:00Z")

	// And again on the second write, which is the one that used to make a
	// week-old refusal read as today's asking.
	refused, err := service.Proposed("wido", "t1", 0, inFlight.Version, partner.ProposalRefused,
		"goal g1-s44 is claimed by m2a")
	testutil.Require(t, "the outcome is admitted", err, nil)
	testutil.Expect(t, "the asking is still the asking", refused.At, proposed.At)
	testutil.Expect(t, "the last write is the last write", refused.UpdatedAt, "2026-09-26T12:00:00Z")

	// The reader the inbox composes from carries both, so the row dates the
	// asking and can still say when the last press was.
	held, err := service.Unsettled("wido")
	testutil.Require(t, "reading the transcript", err, nil)
	testutil.Require(t, "the refused line is still waiting on the human", len(held), 1)
	testutil.Expect(t, "dated by when it was proposed", held[0].At, proposed.At)
	testutil.Expect(t, "with the write's own instant beside it",
		held[0].UpdatedAt, "2026-09-26T12:00:00Z")
}

// A line nothing has written since it was admitted says so by carrying no write
// at all, rather than by repeating the asking's instant twice.
func TestAProposalNobodyHasWrittenCarriesNoUpdateInstant(t *testing.T) {
	t.Parallel()
	service := transcriptOf(t, answered("t1",
		unsettledLine(0, "park-goal", "g1-s44", partner.ProposalWaiting, "", 1)))

	held, err := service.Unsettled("wido")

	testutil.Require(t, "reading the transcript", err, nil)
	testutil.Require(t, "the waiting line is there", len(held), 1)
	testutil.Expect(t, "and nothing has written it", held[0].UpdatedAt, "")
}

// A human who has never talked to the Partner has an empty reading and not a
// failure: a transcript nobody has written is an ordinary state of a seat.
func TestUnsettledIsEmptyWhereNothingWasProposed(t *testing.T) {
	t.Parallel()
	service := transcriptOf(t)

	held, err := service.Unsettled("nobody")

	testutil.Require(t, "reading a transcript that does not exist", err, nil)
	testutil.Expect(t, "nothing is waiting", len(held), 0)
}
