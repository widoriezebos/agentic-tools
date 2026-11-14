package goal

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

// The barrier is reached only by whole-tree validation after capture succeeds.
type validationReadRepository struct {
	Repository
	ctx       context.Context
	validate  func(context.Context) error
	completed chan struct{}
}

func (r validationReadRepository) WithContext(ctx context.Context) Repository { r.ctx = ctx; return r }
func (r validationReadRepository) Files(tip string, prefixes ...string) (map[string][]byte, error) {
	if len(prefixes) == 2 && prefixes[0] == goalsPrefix {
		if err := r.validate(r.ctx); err != nil {
			return nil, err
		}
	}
	return r.Repository.Files(tip, prefixes...)
}
func (r validationReadRepository) Release(opid string) error {
	defer close(r.completed)
	return r.Repository.Release(opid)
}

func TestProjectionExpiryHasReadTimeoutIdentity(t *testing.T) {
	t.Parallel()
	for _, fresh := range []bool{false, true} {
		for _, command := range []bool{false, true} {
			t.Run(fmt.Sprintf("fresh=%t/command=%t", fresh, command), func(t *testing.T) {
				t.Parallel()
				endpoint, repository := fakeGoalEndpoint(t)
				repository.accepted = ""
				endpoint.UnlockedCommandRead = command
				limit := 4 * time.Second
				if command {
					limit = 30 * time.Second
					if fresh {
						limit = 35 * time.Second
					}
				}
				now := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
				reached, release, completed := make(chan struct{}), make(chan struct{}), make(chan struct{})
				endpoint.Repository = validationReadRepository{Repository: repository, completed: completed, validate: func(ctx context.Context) error {
					close(reached)
					<-release
					if ctx != nil {
						return ctx.Err()
					}
					return nil
				}}
				result := make(chan error, 1)
				deadline := make(chan time.Time)
				go func() {
					var err error
					if fresh {
						timing := freshProjectionTiming{now: func() time.Time { return now }, withTimeout: func(parent context.Context, wait time.Duration) (context.Context, context.CancelFunc) {
							return readDeadlineContext{Context: parent, now: &now, end: now.Add(wait)}, func() {}
						}}
						_, _, err = freshProjection(context.Background(), endpoint, func() (time.Time, error) { return now, nil }, timing)
					} else {
						_, err = ProjectWithDeadline(endpoint, true, now, func(wait time.Duration) <-chan time.Time {
							if wait != limit {
								t.Errorf("outer bound = %s, want %s", wait, limit)
							}
							return deadline
						})
					}
					result <- err
				}()
				<-reached
				if fresh {
					now = now.Add(limit)
					close(release)
				} else {
					close(deadline)
				}
				err := <-result
				if !fresh {
					close(release)
				}
				<-completed
				var timeout *ReadTimeout
				if !IsReadTimeout(err) || !errors.As(err, &timeout) || timeout.Limit != limit || !strings.Contains(ReadTimeoutMessage(err), fmt.Sprintf("within %g s", limit.Seconds())) {
					t.Fatalf("validation expiry = %v; want %s", err, limit)
				}
			})
		}
	}
}

func TestFreshProjectionDistinguishesOuterAndTransportExpiry(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                              string
		command                           bool
		elapsed, transport, expiry, limit time.Duration
	}{
		{"default transport", false, 0, 3 * time.Second, 3 * time.Second, 3 * time.Second},
		{"default outer", false, 0, 3 * time.Second, 4 * time.Second, 4 * time.Second},
		{"computed transport", false, time.Second, 2250 * time.Millisecond, 3250 * time.Millisecond, 2250 * time.Millisecond},
		{"command transport", true, 0, 26 * time.Second, 26 * time.Second, 26 * time.Second},
		{"command outer", true, 0, 26 * time.Second, 35 * time.Second, 35 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			start := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)
			now := start
			var outer context.Context
			timing := freshProjectionTiming{now: func() time.Time { return now }, withTimeout: func(parent context.Context, limit time.Duration) (context.Context, context.CancelFunc) {
				ctx := readDeadlineContext{Context: parent, now: &now, end: now.Add(limit)}
				if outer == nil {
					want := 4 * time.Second
					if tc.command {
						want = 35 * time.Second
					}
					if limit != want {
						t.Fatalf("outer = %s, want %s", limit, want)
					}
					outer = ctx
					now = now.Add(tc.elapsed)
				}
				return ctx, func() {}
			}}
			released := false
			r := freshTimingRepository{
				capture: func(ctx context.Context, _ string) (string, error) {
					end, _ := ctx.Deadline()
					if end != start.Add(tc.elapsed+tc.transport) {
						t.Fatalf("transport bound = %s", end)
					}
					now = start.Add(tc.expiry)
					if (outer.Err() == nil) != (tc.expiry < 4*time.Second || tc.command && tc.expiry < 35*time.Second) {
						t.Fatal("unexpected outer expiry")
					}
					return "", fmt.Errorf("transport: %w", ctx.Err())
				},
				release: func(context.Context, string) error { released = true; return nil },
			}
			_, observed, err := freshProjection(context.Background(), Endpoint{Repository: r, UnlockedCommandRead: tc.command}, func() (time.Time, error) { t.Fatal("expired read reached projection"); return now, nil }, timing)
			var timeout *ReadTimeout
			if !released || !IsReadTimeout(err) || !errors.Is(err, context.DeadlineExceeded) || observed.Outcome != "unavailable" || !errors.As(err, &timeout) || timeout.Limit != tc.limit || !strings.Contains(ReadTimeoutMessage(err), fmt.Sprintf("within %g s", tc.limit.Seconds())) {
				t.Fatalf("expiry = %v, observed=%+v, released=%t", err, observed, released)
			}
		})
	}
}
