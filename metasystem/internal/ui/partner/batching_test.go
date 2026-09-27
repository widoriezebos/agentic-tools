package partner_test

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// How many proposals one answer carries, and what the Partner is told when it
// has carried them all.
//
// The bound is a requisite and not a courtesy: the message is one file, and a
// message that cannot be read back is a conversation that cannot be reloaded
// (R-130-ui). So the fifty-first action of one answer is refused in words that
// tell the Partner what to do instead — say how many remain, and propose them in
// the next answer — and the answer whose lines are all settled is told that the
// human may ask for the next batch.

// batchUnparks is one answer's worth of the same admissible action, as many
// times as asked, each prepared on its own call.
func batchUnparks(count int, when string) []fakeacp.Read {
	reads := []fakeacp.Read{}
	for at := 0; at < count; at++ {
		one := proposed(uitools.ProposeUnpark, "refunds", nil,
			"back to the queue, number "+strconv.Itoa(at+1))
		one.When = when
		reads = append(reads, one)
	}
	return reads
}

// Fifty proposals are admitted and the fifty-first is refused, in the words that
// tell the Partner to propose the rest in its next answer.
//
// The refused line still travels to the card: a refusal holds its place and says
// why, here as everywhere else, because an action that vanished would leave the
// human's card two lines short of what they asked for with nothing to say about
// it.
func TestAnAnswerCarriesAtMostFiftyProposals(t *testing.T) {
	t.Parallel()
	service := serviceProposing(t, batchUnparks(52, "")...)
	_, kept := askProposing(t, service, "unpark every one of them")
	testutil.Require(t, "every prepared action is recorded", len(kept), 52)

	offered := 0
	for _, one := range kept {
		if one.Offered {
			offered++
		}
	}
	testutil.Expect(t, "fifty of them are offered", offered, 50)
	testutil.Expect(t, "the fiftieth among them", kept[49].Offered, true)

	past := "this answer already carries fifty proposals; say how many remain and " +
		"propose them in your next answer, after the human has applied these"
	testutil.Expect(t, "the fifty-first is not offered", kept[50].Offered, false)
	testutil.Expect(t, "with the count as the reason", kept[50].Reason, past)
	testutil.Expect(t, "and it keeps its place on the card", kept[50].Index, 50)
	testutil.Expect(t, "the fifty-second is refused the same way", kept[51].Offered, false)
	testutil.Expect(t, "in the same words", kept[51].Reason, past)
}

// The answer whose proposals are every one of them settled is told that the
// human may ask for the next batch, so a Partner working a long list goes on
// without being told twice.
//
// A line that was never offered is not waiting for anybody, so it cannot hold
// the sentence back; a line the human has not pressed yet can.
func TestAnAnswerWhoseLinesAreSettledIsToldToOfferTheNextBatch(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	first := proposed(uitools.ProposeUnpark, "refunds", nil, "back to the queue")
	first.When = "put them back"
	refused := proposed(uitools.ProposeUnpark, "no-such-goal", nil, "and this one")
	refused.When = "put them back"
	second := proposed(uitools.ProposeUnpark, "bank-sandbox", nil, "and the sandbox")
	second.When = "put them back"
	opener, handed := fakeacp.OpenWatched(fakeacp.Script{
		Reads:  []fakeacp.Read{first, refused, second},
		Chunks: []string{"proposed."},
	})
	host := partner.NewHostOn(
		partner.Runtime{Name: "fake", ReadOnly: "a fake server reads nothing"}, root, opener)
	t.Cleanup(host.Close)
	service := partner.NewService(host.Runtime(), host,
		func(human string) (*partner.Conversation, error) { return partner.OpenConversation(root, human) },
		partner.Facts{Observe: proposingLedger},
		func() time.Time { return time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC) })
	batchAsk := func(key, question string) string {
		t.Helper()
		events, stop := service.Subscribe()
		defer stop()
		id, err := service.Submit(context.Background(), "Wido", key, question, onDecisions())
		testutil.Require(t, "the turn on "+key+" is admitted", err, nil)
		drain(t, events)
		return id
	}
	batchSettle := func(turn string, index int) {
		t.Helper()
		applying, err := service.Proposed("Wido", turn, index, 1, partner.ProposalApplying, "")
		testutil.Require(t, "line "+strconv.Itoa(index)+" is taken", err, nil)
		_, err = service.Proposed("Wido", turn, index, applying.Version, partner.ProposalApplied, "")
		testutil.Require(t, "line "+strconv.Itoa(index)+" is settled", err, nil)
	}

	turn := batchAsk("key-1", "these two are ready; put them back")
	testutil.Require(t, "three lines were prepared", len(handed.Prompts()), 1)

	// One offered line settled, one still waiting for the human.
	batchSettle(turn, 0)
	batchAsk("key-2", "and now?")
	next := "is settled: if a longer list remains, the human may ask for the next batch."
	prompts := handed.Prompts()
	testutil.Require(t, "a prompt for the second turn", len(prompts), 2)
	testutil.Expect(t, "the block is there",
		strings.Contains(prompts[1], "What happened to the actions you proposed"), true)
	testutil.Expect(t, "and says nothing about a next batch while one line waits",
		strings.Contains(prompts[1], next), false)

	// Now the other one, and the refused line is settled by being refused.
	batchSettle(turn, 2)
	batchAsk("key-3", "and after that?")
	prompts = handed.Prompts()
	testutil.Require(t, "a prompt for the third turn", len(prompts), 3)
	testutil.Expect(t, "the next batch is offered once every line is settled",
		strings.Contains(prompts[2], next), true)
}
