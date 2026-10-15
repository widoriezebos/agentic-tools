package steward

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testchannel"
)

func TestHelmSettlesReplies(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"fresh", "matched", "recorded-commit", "recorded", "closed"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			replies := testchannel.New(t, loop.root)
			replies.CommitReply(t, replies.Question)
			takeHelmFixture(t, loop.root)
			now := replies.Sent
			deps := idleRunnerDependencies(&now)
			ticks, settlements := 0, 0
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				ticks++
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				t.Fatal("helm launched a seat")
				return SeatRecord{}, nil
			}
			deps.Channel = func(context.Context, string) (int, error) {
				t.Fatal("helm received or delivered messages")
				return 0, nil
			}
			deps.SettleReplies = func(ctx context.Context, _ string) (int, error) {
				settlements++
				c := replies.Config()
				c.Machine, c.Lineage = "helm", "steward"
				if settlements == 1 && phase != "fresh" {
					c.FailurePoint = func(p string) error {
						if p == phase {
							return errors.New("interrupted")
						}
						return nil
					}
				}
				result, err := channel.SettleReplies(ctx, c, replies.Resolve)
				if settlements == 1 && phase != "fresh" && err == nil {
					t.Fatal("interruption did not fire")
				}
				return result.Dispositions, err
			}
			deps.Sleep = func(d time.Duration) {
				now = now.Add(d)
				if ticks == 3 {
					loop.stop(t)
				}
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { t.Fatal("helm revived an agent"); return nil }, time.Second, TickConfig{Now: now}, deps); err != nil {
				t.Fatal(err)
			}
			if settlements != 3 {
				t.Fatalf("settlements=%d", settlements)
			}
			replies.AssertAnswer(t)
		})
	}
}

func TestHelmDefaultSettlesReplies(t *testing.T) {
	t.Parallel()
	for _, phase := range []string{"fresh", "matched"} {
		t.Run(phase, func(t *testing.T) {
			t.Parallel()
			loop := newHelmLoop(t)
			replies := testchannel.New(t, loop.root)
			replies.CommitReply(t, replies.Question)
			if phase == "matched" {
				replies.Interrupt(t, phase)
			}
			takeHelmFixture(t, loop.root)
			now := replies.Sent
			deps := idleRunnerDependencies(&now)
			deps.ReplyEndpoint = replies.Resolve
			ticks := 0
			deps.Tick = func(string, TickConfig, WorkerCensus) (TickResult, error) {
				ticks++
				return TickResult{Decision: Decision{Action: ActRevive}, Seat: &SeatSelection{}}, nil
			}
			deps.StartSeat = func(string, TickConfig, WorkerCensus, SeatSelection) (SeatRecord, error) {
				t.Fatal("helm launched a seat")
				return SeatRecord{}, nil
			}
			deps.Channel = func(context.Context, string) (int, error) {
				t.Fatal("helm received or delivered messages")
				return 0, nil
			}
			deps.Sleep = func(d time.Duration) {
				now = now.Add(d)
				replies.AssertAnswer(t)
				if ticks == 2 {
					loop.stop(t)
				}
			}
			if deps.SettleReplies != nil {
				t.Fatal("test replaced the production default")
			}
			if err := runLoopWithDependencies(loop.root, fakeCensus{}, func() error { t.Fatal("helm revived an agent"); return nil }, time.Second, TickConfig{Now: now}, deps); err != nil {
				t.Fatal(err)
			}
			if ticks != 2 {
				t.Fatalf("ticks=%d", ticks)
			}
		})
	}
}
