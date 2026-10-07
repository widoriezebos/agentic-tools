package goal

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Time advances only when the test moves it; cancellation still propagates.
func freshTestTiming(now *time.Time) freshProjectionTiming {
	return freshProjectionTiming{
		now: func() time.Time { return *now },
		withTimeout: func(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
			deadline := now.Add(limit)
			if inherited, ok := parent.Deadline(); ok && inherited.Before(deadline) {
				deadline = inherited
			}
			ctx, cancel := context.WithCancel(parent)
			return fixedDeadline{ctx, deadline}, cancel
		},
	}
}

type freshTimingRepository struct {
	Repository
	ctx     context.Context
	capture func(context.Context, string) (string, error)
	release func(context.Context, string) error
}

func (r freshTimingRepository) WithContext(ctx context.Context) Repository { r.ctx = ctx; return r }
func (r freshTimingRepository) Capture(opid string) (string, error)        { return r.capture(r.ctx, opid) }
func (r freshTimingRepository) Release(opid string) error                  { return r.release(r.ctx, opid) }

func TestFreshProjectionReservesAllowanceAndStartsCleanupAfterCancellation(t *testing.T) {
	t.Parallel()
	for _, allowance := range []time.Duration{4 * time.Second, 2 * time.Second} {
		t.Run(allowance.String(), func(t *testing.T) {
			t.Parallel()
			start := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
			now := start
			ctx, cancel := context.WithCancel(fixedDeadline{context.Background(), start.Add(allowance)})
			defer cancel()
			released := false
			r := freshTimingRepository{
				capture: func(c context.Context, opid string) (string, error) {
					deadline, ok := c.Deadline()
					want := start.Add(allowance * 3 / 4)
					if !ok || !deadline.Equal(want) {
						t.Fatalf("transport deadline = %s, want %s", deadline, want)
					}
					// Advance past the entire read allowance before cancelling. Cleanup
					// must start with a fresh bound rather than inherit that expired one.
					now = start.Add(allowance + time.Hour)
					cancel()
					return "", c.Err()
				},
				release: func(c context.Context, opid string) error {
					released = true
					deadline, ok := c.Deadline()
					if c.Err() != nil || !ok || !deadline.Equal(now.Add(2*time.Second)) {
						t.Fatalf("cleanup deadline = %s, want %s; cancellation = %v", deadline, now.Add(2*time.Second), c.Err())
					}
					return nil
				},
			}
			p, observed, err := freshProjection(ctx, Endpoint{Repository: r}, func() (time.Time, error) { t.Fatal("cancelled read reached projection"); return now, nil }, freshTestTiming(&now))
			if !released || !errors.Is(err, context.Canceled) || observed.Outcome != "unavailable" || p.Tree != nil {
				t.Fatalf("cancelled read = %+v %+v %v; released=%v", p, observed, err, released)
			}
		})
	}
}
