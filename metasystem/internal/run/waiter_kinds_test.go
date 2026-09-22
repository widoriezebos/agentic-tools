package run

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

type kindWaitClock struct {
	now    time.Time
	boot   time.Duration
	sleeps []time.Duration
	events []string
}

func newKindWaitClock() *kindWaitClock {
	return &kindWaitClock{
		now:  time.Date(2026, 9, 18, 10, 0, 0, 0, time.UTC),
		boot: 2 * time.Hour,
	}
}

func (clock *kindWaitClock) options(observe func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error)) WaitOptions {
	return WaitOptions{
		Now:       func() time.Time { return clock.now },
		BootClock: func() (string, time.Duration, error) { return "boot-test", clock.boot, nil },
		Sleep: func(_ context.Context, duration time.Duration) error {
			clock.sleeps = append(clock.sleeps, duration)
			clock.now = clock.now.Add(duration)
			clock.boot += duration
			return nil
		},
		Deliver: func(context.Context, string, string, time.Time, string) (string, bool, error) {
			return "blocking", false, nil
		},
		Observe: observe,
		EmitEvent: func(_ string, event, _ string, _ map[string]string) error {
			clock.events = append(clock.events, event)
			return nil
		},
	}
}

func (clock *kindWaitClock) returnedEvents() int {
	count := 0
	for _, event := range clock.events {
		if event == "wait-returned" {
			count++
		}
	}
	return count
}

func testWaitKindEndsAtItsBound(t *testing.T, selector WaitSelector, incarnation WaiterTarget, pending, terminal SourceObservation) {
	t.Helper()

	t.Run("deadline", func(t *testing.T) {
		clock := newKindWaitClock()
		startedAt := clock.now
		reads := 0
		options := clock.options(func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			reads++
			return pending, nil
		})
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
			context.Background(),
			WaitRequest{Selector: selector, Owner: mainCaller, RuntimeSession: mainCaller.SessionId, Timeout: 3 * time.Second},
			options,
		)
		if result.ExitCode != ExitWaitDeadline || result.SourceOutcome != "wait-deadline" || result.Reason != "this wait reached its deadline" {
			t.Fatalf("deadline result = %+v", result)
		}
		if result.TargetIncarnation != incarnation || result.ReturnedAt != startedAt.Add(3*time.Second).Format(time.RFC3339Nano) {
			t.Fatalf("deadline timing or incarnation = %+v", result)
		}
		if reads != 2 {
			t.Fatalf("deadline observations = %d, want initial and loop observations", reads)
		}
		if want := []time.Duration{3 * time.Second}; !reflect.DeepEqual(clock.sleeps, want) {
			t.Fatalf("deadline sleeps = %v, want %v", clock.sleeps, want)
		}
		if got := clock.returnedEvents(); got != 1 {
			t.Fatalf("deadline returned events = %d, want one result", got)
		}
	})

	t.Run("recorded verdict", func(t *testing.T) {
		clock := newKindWaitClock()
		startedAt := clock.now
		reads := 0
		options := clock.options(func(context.Context, WaitSelector, WaiterTarget, string) (SourceObservation, error) {
			reads++
			if reads == 3 {
				return terminal, nil
			}
			return pending, nil
		})
		result := (&Store{Root: t.TempDir(), Prober: waitTestProber{live: true}}).Wait(
			context.Background(),
			WaitRequest{Selector: selector, Owner: mainCaller, RuntimeSession: mainCaller.SessionId, Timeout: time.Minute},
			options,
		)
		if result.ExitCode != terminal.ExitCode || result.SourceOutcome != terminal.Outcome || result.SourceEvidence != terminal.Evidence || result.Reason != terminal.Reason {
			t.Fatalf("recorded result = %+v", result)
		}
		if reads != 3 || !clock.now.Before(startedAt.Add(time.Minute)) {
			t.Fatalf("observations=%d fake time=%s, want a third-read result before the bound", reads, clock.now)
		}
		if want := []time.Duration{10 * time.Second}; !reflect.DeepEqual(clock.sleeps, want) {
			t.Fatalf("recorded-verdict sleeps = %v, want %v", clock.sleeps, want)
		}
		if got := clock.returnedEvents(); got != 1 {
			t.Fatalf("recorded-verdict returned events = %d, want one result", got)
		}
	})
}

func TestWaitAttemptEndsAtItsBound(t *testing.T) {
	incarnation := WaiterTarget{ProofDigest: "proof-digest"}
	pending := SourceObservation{Pending: true, Incarnation: incarnation, Outcome: "pending", Evidence: "attempt:attempt-bound:proof-digest"}
	terminal := SourceObservation{Incarnation: incarnation, ExitCode: ExitRed, Outcome: "failed", Reason: "recorded proof verdict", Evidence: pending.Evidence}
	testWaitKindEndsAtItsBound(t, WaitSelector{Kind: "attempt", TargetID: "attempt-bound"}, incarnation, pending, terminal)
}

func TestWaitGoalEndsAtItsBound(t *testing.T) {
	after := strings.Repeat("a", 40)
	selector := WaitSelector{Kind: "goal", TargetID: "goal-bound", GoalID: "goal-bound", Event: "landing", After: after}
	incarnation := WaiterTarget{StartedAt: "ledger:" + after, ProofDigest: "endpoint-digest"}
	pending := SourceObservation{Pending: true, Incarnation: incarnation, Outcome: "pending", Evidence: "ledger:" + after, LedgerTip: after}
	terminal := SourceObservation{Incarnation: incarnation, ExitCode: ExitGreen, Outcome: "landing", Reason: "matching goal landing became reachable", Evidence: "commit:bbbbbbbb:chain", LedgerTip: strings.Repeat("b", 40)}
	testWaitKindEndsAtItsBound(t, selector, incarnation, pending, terminal)
}
