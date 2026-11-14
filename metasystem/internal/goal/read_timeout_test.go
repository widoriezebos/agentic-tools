package goal

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/boundedexec"
)

type readDeadlineContext struct {
	context.Context
	now *time.Time
	end time.Time
}

func (c readDeadlineContext) Deadline() (time.Time, bool) { return c.end, true }
func (c readDeadlineContext) Err() error {
	if !c.now.Before(c.end) {
		return context.DeadlineExceeded
	}
	return c.Context.Err()
}

func TestReadTimeoutIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"projection", &ReadTimeout{Limit: 4 * time.Second}, true},
		{"process", boundedexec.ErrTimedOut, true},
		{"transport", context.DeadlineExceeded, true},
		{"canceled", context.Canceled, false},
		{"failure", errors.New("unreadable ledger"), false},
		{"success", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsReadTimeout(tc.err); got != tc.want {
				t.Fatalf("timeout = %t, want %t: %v", got, tc.want, tc.err)
			}
			if tc.err != nil && IsReadTimeout(fmt.Errorf("read: %w", tc.err)) != tc.want {
				t.Fatal("wrapping changed timeout identity")
			}
		})
	}
}

func TestProjectionExpiryHasReadTimeoutIdentity(t *testing.T) {
	t.Parallel()
	expired, started, release, done := make(chan time.Time), make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer func() { close(release); <-done }()
	close(expired)
	_, err := project(Endpoint{}, true, time.Time{}, projectionDependencies{
		deadline: expired,
		fetch: func(Endpoint) (AdvanceResult, error) {
			defer close(done)
			close(started)
			<-release
			return AdvanceResult{}, nil
		},
	})
	<-started
	var timeout *ReadTimeout
	if !IsReadTimeout(err) || !errors.As(err, &timeout) || timeout.Limit != 4*time.Second {
		t.Fatalf("projection expiry = %v", err)
	}
}

func TestFreshProjectionDistinguishesOuterAndTransportExpiry(t *testing.T) {
	t.Parallel()
	for _, expiry := range []time.Duration{3 * time.Second, 4 * time.Second} {
		t.Run(expiry.String(), func(t *testing.T) {
			t.Parallel()
			start := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
			now := start
			var outer context.Context
			timing := freshProjectionTiming{now: func() time.Time { return now }, withTimeout: func(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
				ctx := readDeadlineContext{Context: parent, now: &now, end: now.Add(limit)}
				if outer == nil {
					outer = ctx
				}
				return ctx, func() {}
			}}
			released := false
			r := freshTimingRepository{
				capture: func(ctx context.Context, _ string) (string, error) {
					end, _ := ctx.Deadline()
					if end != start.Add(3*time.Second) {
						t.Fatalf("transport bound = %s", end)
					}
					now = start.Add(expiry)
					if (outer.Err() == nil) != (expiry == 3*time.Second) {
						t.Fatal("unexpected outer expiry")
					}
					return "", fmt.Errorf("transport: %w", ctx.Err())
				},
				release: func(context.Context, string) error { released = true; return nil },
			}
			_, observed, err := freshProjection(context.Background(), Endpoint{Repository: r}, func() (time.Time, error) { t.Fatal("expired read reached projection"); return now, nil }, timing)
			var timeout *ReadTimeout
			if !released || !IsReadTimeout(err) || !errors.Is(err, context.DeadlineExceeded) || observed.Outcome != "unavailable" || errors.As(err, &timeout) != (expiry == 4*time.Second) {
				t.Fatalf("expiry = %v, observed=%+v, released=%t", err, observed, released)
			}
		})
	}
}
