package main

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type stopDeadlineCommandProbe struct {
	exact identity.Exact
	state identity.Liveness
	err   error
}

func (p stopDeadlineCommandProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, p.err
}

func TestStopDeadlineCleanupClassifiesIdentityBeforeParentCustody(t *testing.T) {
	t.Parallel()

	exact := identity.Exact{Pid: 42, StartedAt: time.Unix(100, 123000)}
	worker := exact.Ref()
	for _, test := range []struct {
		name  string
		probe stopDeadlineCommandProbe
		want  string
	}{
		{name: "dead", probe: stopDeadlineCommandProbe{state: identity.Dead}, want: hooks.StopWorkerGone},
		{name: "zombie", probe: stopDeadlineCommandProbe{exact: identity.Exact{Pid: 42, StartedAt: exact.StartedAt, Zombie: true}, state: identity.Alive}, want: hooks.StopWorkerGone},
		{name: "replaced", probe: stopDeadlineCommandProbe{exact: identity.Exact{Pid: 42, StartedAt: exact.StartedAt.Add(time.Second)}, state: identity.Alive}, want: hooks.StopWorkerGone},
		{name: "unknown", probe: stopDeadlineCommandProbe{state: identity.Unknown, err: errors.New("uninspectable")}, want: hooks.StopWorkerUnverified},
	} {
		t.Run(test.name, func(t *testing.T) {
			parentChecked := false
			signalled := false
			result, err := cleanupStopDeadlineWorker(context.Background(), &worker, time.Second, stopDeadlineCleanupDependencies{
				prober: test.probe,
				parentPid: func(int64) (int64, bool) {
					parentChecked = true
					return 7, true
				},
				parent: 7,
				deadline: hooks.StopDeadlineDependencies{Prober: test.probe, Signal: func(int, syscall.Signal) error {
					signalled = true
					return nil
				}},
			})
			if err != nil || result.State != test.want || parentChecked || signalled {
				t.Fatalf("cleanup result=%+v err=%v parentChecked=%v signalled=%v", result, err, parentChecked, signalled)
			}
		})
	}

	signalled := false
	result, err := cleanupStopDeadlineWorker(context.Background(), &worker, time.Second, stopDeadlineCleanupDependencies{
		prober:    stopDeadlineCommandProbe{exact: exact, state: identity.Alive},
		parentPid: func(int64) (int64, bool) { return 8, true },
		parent:    7,
		deadline: hooks.StopDeadlineDependencies{Signal: func(int, syscall.Signal) error {
			signalled = true
			return nil
		}},
	})
	if err != nil || result.State != hooks.StopWorkerUnverified || signalled {
		t.Fatalf("foreign-parent cleanup result=%+v err=%v signalled=%v", result, err, signalled)
	}
}
