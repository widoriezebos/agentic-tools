package partner_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// Asking what happened (g1-s68 D2, §6): one press sends one fixed sentence in
// the human's name, and the trouble travels as a block the server composes
// from the payload.

// landRefused is the §3 trouble: Land pressed on a goal's card, refused.
func landRefused() partner.Trouble {
	return partner.Trouble{
		Text: "work land is refused: goal/backlog-ordered-by-priority has moved past the tip the review named",
		Code: "REVIEW_STALE",
		Where: partner.TroubleWhere{Section: "Backlog", Path: "/backlog",
			Subject: "backlog-ordered-by-priority", Kind: "goal"},
		Act: &partner.TroubleAct{Verb: "Land", Object: "goal", Target: "backlog-ordered-by-priority"},
		At:  "2026-09-28T19:41:00Z",
		Tip: "7ed3baf",
	}
}

func TestTheTroubleRequestIsOneFixedSentence(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the sentence", partner.TroubleRequest,
		"What just happened here, why, and how do I recover?")
}

func TestTheTroubleBlockCarriesEveryField(t *testing.T) {
	t.Parallel()
	trouble := landRefused()
	trouble.SignIn = true
	block := partner.TroubleBlock(trouble)
	for _, want := range []string{
		"The trouble:\n",
		"- What the screen said: work land is refused: goal/backlog-ordered-by-priority has moved past the tip the review named\n",
		"- Code: REVIEW_STALE\n",
		"- Where: Backlog (/backlog), goal backlog-ordered-by-priority\n",
		"- The act: Land on goal backlog-ordered-by-priority\n",
		"- When: 2026-09-28T19:41:00Z\n",
		"- The tip the page had: 7ed3baf\n",
		"- The remedy the page offered: the sign-in sheet\n",
	} {
		testutil.Expect(t, "the block says "+want, strings.Contains(block, want), true)
	}
	bare := partner.TroubleBlock(partner.Trouble{Text: "The backlog could not be read",
		Where: partner.TroubleWhere{Section: "Backlog", Path: "/backlog"}, At: "2026-09-28T19:41:00Z"})
	testutil.Expect(t, "a pane's trouble has no code line", strings.Contains(bare, "- Code:"), false)
	testutil.Expect(t, "and no act line", strings.Contains(bare, "- The act:"), false)
}

func TestTheTroubleIsBounded(t *testing.T) {
	t.Parallel()
	over := landRefused()
	over.Text = strings.Repeat("é", partner.MaxTroubleText+1)
	_, err := over.Bound()
	testutil.Expect(t, "a sentence over the bound is refused", errors.Is(err, partner.ErrTroubleTooLong), true)
	at := landRefused()
	at.Text = strings.Repeat("é", partner.MaxTroubleText)
	_, err = at.Bound()
	testutil.Expect(t, "a sentence at the bound is kept", err, nil)
	empty := landRefused()
	empty.Text = "  "
	_, err = empty.Bound()
	testutil.Expect(t, "a trouble with no sentence is refused", err != nil, true)
	long := landRefused()
	long.Code = strings.Repeat("X", 1000)
	_, err = long.Bound()
	testutil.Expect(t, "a field past its bound is refused", errors.Is(err, partner.ErrTroubleTooLong), true)
}

// A turn with a trouble is recorded like any turn: the fixed sentence in the
// human's name, marked as the interface's, keeping the trouble for its chip;
// and the runtime is given the block beside the page.
func TestATroubleTurnIsRecordedAndTheRuntimeReceivesTheBlock(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.AskTrouble(context.Background(), "Wido", "", "key-t", landRefused(),
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "admitted", err, nil)
	drain(t, events)

	snapshot, err := service.SnapshotIn("Wido", "", 0)
	testutil.Require(t, "the snapshot reads", err, nil)
	testutil.Require(t, "a question and an answer", len(snapshot.Messages), 2)
	asked := snapshot.Messages[0]
	testutil.Expect(t, "the human's turn is the fixed sentence", asked.Text, partner.TroubleRequest)
	testutil.Expect(t, "asked by the interface at their press", asked.Interface, true)
	testutil.Require(t, "keeping the trouble", asked.Trouble != nil, true)
	testutil.Expect(t, "whole", *asked.Trouble, landRefused())
	testutil.Expect(t, "and the answer is an ordinary answer", snapshot.Messages[1].Outcome, "complete")

	prompts := handed.Prompts()
	testutil.Require(t, "one prompt", len(prompts), 1)
	testutil.Expect(t, "the runtime receives the block",
		strings.Contains(prompts[0], partner.TroubleBlock(landRefused())), true)
	testutil.Expect(t, "beside the page", strings.Index(prompts[0], "Where the human is") <
		strings.LastIndex(prompts[0], "The trouble:\n"), true)
	testutil.Expect(t, "and the fixed sentence as the question",
		strings.HasSuffix(prompts[0], "The human asks:\n"+partner.TroubleRequest), true)

	// The same key twice is the same turn once, as for every turn.
	again, err := service.AskTrouble(context.Background(), "Wido", "", "key-t", landRefused(),
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Require(t, "the retry is answered", err, nil)
	testutil.Expect(t, "with the same turn", again, asked.Turn)
}

func TestATroubleOverTheBoundIsNeverATurn(t *testing.T) {
	t.Parallel()
	service, handed := composing(t, indexedProject())
	over := landRefused()
	over.Text = strings.Repeat("a", partner.MaxTroubleText+1)
	_, err := service.AskTrouble(context.Background(), "Wido", "", "key-over", over,
		partner.Page{Section: "Backlog", Path: "/backlog"})
	testutil.Expect(t, "refused", errors.Is(err, partner.ErrTroubleTooLong), true)
	snapshot, err := service.SnapshotIn("Wido", "", 0)
	testutil.Require(t, "the snapshot reads", err, nil)
	testutil.Expect(t, "nothing was written", len(snapshot.Messages), 0)
	testutil.Expect(t, "nothing was sent", len(handed.Prompts()), 0)
}

// A recovery that is not one of the ten goal acts never arrives as a card
// (g1-s68 D3, S68-04): the tool refuses "open the room" at the call, and a
// frame that names a route the catalogue does not carry is refused by the
// service rather than offered with an Apply nothing could perform.
func TestARecoveryThatIsNotAGoalActIsNeverACard(t *testing.T) {
	t.Parallel()
	called := uitools.Readers{}.Answer(uitools.OpPropose, uitools.Args{
		"verb": "open the room", "goal": "fleet-presence", "explanation": "the room offers Review the new tip",
	})
	testutil.Expect(t, "the tool refuses the call",
		strings.HasPrefix(called.Text(), "Outcome: this call was refused"), true)

	service := serviceProposing(t, proposed("open-room", "fleet-presence", nil,
		"the room offers Review the new tip"))
	events, stop := service.Subscribe()
	defer stop()
	_, err := service.AskTrouble(context.Background(), "Wido", "", "key-room", landRefused(), onDecisions())
	testutil.Require(t, "the trouble is asked", err, nil)
	drain(t, events)
	read, err := service.Snapshot("Wido", 100)
	testutil.Require(t, "the conversation reads back", err, nil)
	kept := read.Messages[len(read.Messages)-1].Proposals
	for _, proposal := range kept {
		testutil.Expect(t, "no card is offered for "+proposal.Verb, proposal.Offered, false)
	}
	testutil.Require(t, "the refused line is kept", len(kept), 1)
	testutil.Expect(t, "and says why", kept[0].Reason != "", true)
}
