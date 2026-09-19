package testenv

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"testing"
	"time"
)

const fixtureCleanupBound = 15 * time.Second

// FixtureProcessGroup identifies a test-owned child that is expected to be
// the leader of its own process group. Resolve runs at cleanup before orderly
// stop actions can remove the child's record.
type FixtureProcessGroup struct {
	Verb    string
	Resolve func() (pid int, present bool, err error)
}

// FixtureCleanup is one orderly stop action. Each action receives a fresh
// bound so one slow stop cannot prevent a later stop from running.
type FixtureCleanup struct {
	Verb string
	Run  func(context.Context) error
}

type fixtureProcessGroupTB interface {
	Helper()
	Cleanup(func())
	Errorf(string, ...any)
}

type fixtureProcessGroupOps struct {
	groupID        func(int) (int, error)
	signal         func(int, syscall.Signal) error
	wait           func(context.Context, int) error
	cleanupContext func() (context.Context, context.CancelFunc)
	exitContext    func() (context.Context, context.CancelFunc)
}

type resolvedFixtureProcessGroup struct {
	pid, signalTarget int
	verb              string
}

// ReapFixtureProcessGroups makes detached fixture children part of the test's
// lifecycle. It runs every orderly stop, kills each remaining process group,
// and does not return from cleanup until every group is gone or the fixture
// exit bound expires. Resolve callbacks should read the durable pid record
// written by the child they own.
func ReapFixtureProcessGroups(t testing.TB, groups []FixtureProcessGroup, cleanups ...FixtureCleanup) {
	t.Helper()
	exitBound, err := FixtureExitWaitBound()
	if err != nil {
		t.Fatalf("derive fixture process-group exit bound: %v", err)
	}
	reapFixtureProcessGroups(t, groups, cleanups, fixtureProcessGroupOps{
		groupID: syscall.Getpgid,
		signal:  syscall.Kill,
		wait:    waitForFixtureProcessTarget,
		cleanupContext: func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), fixtureCleanupBound)
		},
		exitContext: func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(context.Background(), exitBound)
		},
	})
}

func reapFixtureProcessGroups(t fixtureProcessGroupTB, groups []FixtureProcessGroup, cleanups []FixtureCleanup, ops fixtureProcessGroupOps) {
	t.Cleanup(func() {
		resolved := make([]resolvedFixtureProcessGroup, 0, len(groups))
		for _, group := range groups {
			if group.Resolve == nil {
				t.Errorf("fixture process group has no resolver: verb=%q", group.Verb)
				continue
			}
			pid, present, err := group.Resolve()
			if err != nil {
				t.Errorf("resolve fixture process group: verb=%q: %v", group.Verb, err)
				continue
			}
			if !present {
				continue
			}
			if pid < 1 {
				t.Errorf("resolve fixture process group: verb=%q returned invalid pid=%d", group.Verb, pid)
				continue
			}
			pgid, err := ops.groupID(pid)
			if errors.Is(err, syscall.ESRCH) {
				continue
			}
			if err != nil {
				t.Errorf("inspect fixture process group: pid=%d verb=%q: %v", pid, group.Verb, err)
				continue
			}
			target := -pid
			if pgid != pid {
				t.Errorf("fixture child was not started in its own process group: pid=%d pgid=%d verb=%q", pid, pgid, group.Verb)
				target = pid
			}
			resolved = append(resolved, resolvedFixtureProcessGroup{pid: pid, signalTarget: target, verb: group.Verb})
		}

		for _, cleanup := range cleanups {
			if cleanup.Run == nil {
				t.Errorf("fixture cleanup has no runner: verb=%q", cleanup.Verb)
				continue
			}
			ctx, cancel := ops.cleanupContext()
			err := cleanup.Run(ctx)
			contextErr := ctx.Err()
			cancel()
			if err != nil || contextErr != nil {
				t.Errorf("fixture cleanup failed: verb=%q context=%v error=%v", cleanup.Verb, contextErr, err)
			}
		}

		for _, group := range resolved {
			if err := ops.signal(group.signalTarget, syscall.SIGKILL); err != nil && !errors.Is(err, syscall.ESRCH) {
				t.Errorf("kill fixture process group: pid=%d verb=%q: %v", group.pid, group.verb, err)
			}
		}
		for _, group := range resolved {
			ctx, cancel := ops.exitContext()
			err := ops.wait(ctx, group.signalTarget)
			cancel()
			if err != nil {
				t.Errorf("fixture child outlived test: pid=%d verb=%q: %v", group.pid, group.verb, err)
			}
		}
	})
}

func waitForFixtureProcessTarget(ctx context.Context, target int) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		err := syscall.Kill(target, 0)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		if err != nil && !errors.Is(err, syscall.EPERM) {
			return fmt.Errorf("probe process target %d: %w", target, err)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
