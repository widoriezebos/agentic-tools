package boundedexec

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestProgressBoundKeepsIdleAndCeilingSeparate(t *testing.T) {
	t.Parallel()
	for _, stop := range []string{"idle", "ceiling"} {
		t.Run(stop, func(t *testing.T) {
			t.Parallel()
			fixture := newPipeHeldCommand(t, "cat <&3 >/dev/null")
			progress := make(chan struct{})
			bound := FixedBound(15*time.Minute, "proof.deadline")
			bound.Progress, bound.Ceiling = progress, 2*time.Hour
			type request struct {
				duration time.Duration
				fire     chan time.Time
			}
			requests := make(chan request)
			done := fixture.startWithDeadline(bound, "check", func(d time.Duration) <-chan time.Time {
				fire := make(chan time.Time, 1)
				if d == killGraceWindow {
					fire <- time.Time{}
				} else {
					requests <- request{d, fire}
				}
				return fire
			})
			initial, ceiling := <-requests, <-requests
			if initial.duration != bound.Limit || ceiling.duration != bound.Ceiling {
				t.Fatalf("initial=%s ceiling=%s", initial.duration, ceiling.duration)
			}
			progress <- struct{}{}
			fresh := <-requests
			if fresh.duration != bound.Limit {
				t.Fatalf("progress changed the ceiling: %s", fresh.duration)
			}
			initial.fire <- time.Time{}
			expected := "no output for 15m0s (proof.deadline)"
			if stop == "idle" {
				fresh.fire <- time.Time{}
			} else {
				ceiling.fire <- time.Time{}
				expected = "still running after 2h0m0s (proof.deadline)"
			}
			err := <-done
			if !errors.Is(err, ErrTimedOut) || !strings.Contains(err.Error(), expected) || len(initial.fire) != 1 {
				t.Fatalf("error=%v stale timer pending=%d", err, len(initial.fire))
			}
		})
	}
}

func TestProgressBoundCancellationRetainsItsCause(t *testing.T) {
	t.Parallel()
	fixture := newPipeHeldCommand(t, "cat <&3 >/dev/null")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	err := runContext(ctx, fixture.command, Bound{Limit: time.Minute, Progress: make(chan struct{})}, "check", func(time.Duration) <-chan time.Time {
		cancel()
		calls++
		fire := make(chan time.Time, 1)
		if calls > 1 {
			fire <- time.Time{}
		}
		return fire
	}, func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) })
	if !errors.Is(err, context.Canceled) || errors.Is(err, ErrTimedOut) {
		t.Fatalf("cancellation became a timeout: %v", err)
	}
}

func TestRunContextExpiredAllowanceStartsNoProcess(t *testing.T) {
	t.Parallel()
	for _, cause := range []string{"cancelled", "total-deadline"} {
		t.Run(cause, func(t *testing.T) {
			t.Parallel()
			fixture := newPipeHeldCommand(t, "cat <&3 >/dev/null")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
			bound := FixedBound(time.Minute, "test allowance")
			expected := context.DeadlineExceeded
			if cause == "cancelled" {
				cancel()
				expected = context.Canceled
			} else {
				bound.TotalDeadline = now
			}
			var err error
			if cause == "cancelled" {
				err = RunContext(ctx, fixture.command, bound, "check")
			} else {
				err = runContext(ctx, fixture.command, bound, "check", func(time.Duration) <-chan time.Time {
					t.Error("an expired allowance scheduled a command timer")
					fired := make(chan time.Time, 1)
					fired <- time.Time{}
					return fired
				}, func() time.Time { return now })
			}
			if !errors.Is(err, expected) || fixture.command.Process != nil {
				t.Fatalf("error=%v process=%v", err, fixture.command.Process)
			}
		})
	}
}
