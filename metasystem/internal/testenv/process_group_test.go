package testenv

import (
	"context"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestExpiredOrderlyStopEscalatesToTheKill: an orderly stop whose bound has
// expired is not a failure; cleanup moves on to the group kill. The stop's
// context is already cancelled, so nothing is timed.
func TestExpiredOrderlyStopEscalatesToTheKill(t *testing.T) {
	recorder := &fixtureProcessGroupRecorder{}
	var kills []string
	reapFixtureProcessGroups(recorder, []FixtureProcessGroup{{
		Verb: "steward run", Resolve: func() (int, bool, error) { return 42, true, nil },
	}}, []FixtureCleanup{{Verb: "steward disarm", Run: func(ctx context.Context) error { return ctx.Err() }}}, fixtureProcessGroupOps{
		groupID: func(pid int) (int, error) { return pid, nil },
		signal: func(target int, signal syscall.Signal) error {
			kills = append(kills, fmt.Sprintf("target=%d signal=%v", target, signal))
			return nil
		},
		birth: func(int) (time.Time, bool) { return time.Unix(1, 0), true },
		wait:  func(context.Context, int) error { return nil },
		cleanupContext: func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		},
		exitContext: testFixtureContext,
	})
	recorder.runCleanups()
	if len(recorder.errors) != 0 || strings.Join(kills, ",") != "target=-42 signal=killed" {
		t.Fatalf("expired orderly stop: errors=%v kills=%v", recorder.errors, kills)
	}
}

// TestSignaledGroupExitWaitHasNoDeadline: after the reaper's own SIGKILL the
// wait for the group to be reaped ends on that fact, not on a clock. The
// production wiring is used with only the kernel seams swapped, and the fake
// wait records whether its context carries a deadline.
func TestSignaledGroupExitWaitHasNoDeadline(t *testing.T) {
	ops, err := defaultFixtureProcessGroupOps()
	if err != nil {
		t.Fatal(err)
	}
	killed := false
	var waits []bool
	ops.groupID = func(pid int) (int, error) { return pid, nil }
	ops.signal = func(target int, signal syscall.Signal) error {
		killed = killed || target == -42 && signal == syscall.SIGKILL
		return nil
	}
	ops.birth = func(int) (time.Time, bool) { return time.Unix(1, 0), true }
	ops.wait = func(ctx context.Context, target int) error {
		_, hasDeadline := ctx.Deadline()
		waits = append(waits, hasDeadline)
		return nil
	}
	recorder := &fixtureProcessGroupRecorder{}
	reapFixtureProcessGroups(recorder, []FixtureProcessGroup{{
		Verb: "steward run", Resolve: func() (int, bool, error) { return 42, true, nil },
	}}, nil, ops)
	recorder.runCleanups()
	if !killed || len(waits) != 1 || waits[0] || len(recorder.errors) != 0 {
		t.Fatalf("signaled group: killed=%t waits-with-deadline=%v errors=%v", killed, waits, recorder.errors)
	}
}
