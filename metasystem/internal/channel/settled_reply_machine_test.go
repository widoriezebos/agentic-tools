package channel_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// newSettleBed is the tap bed with the channel adapter committed, so the
// steward's settlement pass (phase.SettleReplies) runs on the seat.
func newSettleBed(t *testing.T) *tapBed {
	t.Helper()
	bed := newInboxBed(t, inboxSetup{now: tapClock, conf: "channel.human.answer-code=off\nchannel.destination.fleet.adapter=fake\n"})
	return &tapBed{inboxBed: bed, inst: bed.seat}
}

// postedWithoutMachine posts a contract question asked, as the steward asks
// one, with no machine or lineage of its own.
func (b *tapBed) postedWithoutMachine(t *testing.T) channel.Question {
	t.Helper()
	request := b.request()
	request.Machine, request.Lineage = "", ""
	q, err := channel.Ask(request)
	if err != nil {
		t.Fatal(err)
	}
	b.pollOnce(t)
	q = b.question(t, b.inst, q.ID)
	if q.Thread == nil {
		t.Fatal("the question was not posted")
	}
	return q
}

// receiveOnLane has the other installation on the one bot receive the reply,
// so the seat's next tick finds it already authenticated.
func (b *tapBed) receiveOnLane(t *testing.T) {
	t.Helper()
	if _, err := channel.Poll(context.Background(), b.config(b.lane)); err != nil {
		t.Fatalf("lane poll: %v", err)
	}
}

func TestASettledReplyNamesItsMachine(t *testing.T) {
	t.Parallel()
	lineage := os.Getenv("METASYSTEM_OWNER_LINEAGE")
	if lineage == "" {
		lineage = "steward"
	}
	for _, how := range []string{"tap", "typed reply"} {
		t.Run(how, func(t *testing.T) {
			t.Parallel()
			b := newSettleBed(t)
			q := b.postedWithoutMachine(t)
			if how == "tap" {
				b.tap(t, q.Thread.ID, "Continue", 7001)
			} else {
				b.reply(t, q.Thread.ID, "Continue", tapClock)
			}
			b.receiveOnLane(t)
			// The steward's tick: the settlement pass, then the channel duty's poll.
			if _, err := phase.SettleReplies(context.Background(), b.inst.root, goal.ResolveMachine, nil); err != nil {
				t.Fatalf("settlement pass: %v", err)
			}
			got := b.question(t, b.inst, q.ID)
			if got.Answer == nil || got.Answer.Machine != b.inst.machine || got.Answer.Lineage != lineage || got.Answer.Phase != "recorded" || !strings.Contains(got.Answer.Opid, "-"+b.inst.machine+"-") {
				t.Fatalf("the settlement pass recorded %+v", got.Answer)
			}
			b.pollOnce(t)
			if got = b.question(t, b.inst, q.ID); len(b.calls) != 1 || got.Answer.Phase != "acted" {
				t.Fatalf("the channel duty: act runs %q, answer %+v", b.calls, got.Answer)
			}
		})
	}
}

func TestAnAnswerWithoutAMachineIsRefused(t *testing.T) {
	t.Parallel()
	b := newSettleBed(t)
	q := b.postedWithoutMachine(t)
	b.tap(t, q.Thread.ID, "Continue", 7001)
	b.receiveOnLane(t)
	c := b.config(b.inst)
	c.Machine, c.Lineage, c.Provider = "", "", nil
	_, err := channel.SettleReplies(context.Background(), c, nil)
	if err == nil || !strings.Contains(err.Error(), "no machine") {
		t.Fatalf("settlement without a machine: %v", err)
	}
	if got := b.question(t, b.inst, q.ID); got.Answer != nil || got.State != "open" || len(b.calls) != 0 {
		t.Fatalf("an answer without a machine was written: %+v calls %q", got.Answer, b.calls)
	}
}
