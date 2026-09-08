package proofrun

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type stopStateProbe struct {
	started map[int64]int64
	states  map[int64]identity.Liveness
}

func (p *stopStateProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(p.started[pid], 0)}, p.states[pid], nil
}

func TestStopUsesRecordIdentityWhenWatchdogIsDeadAndSuiteIsLive(t *testing.T) {
	probe := &stopStateProbe{
		started: map[int64]int64{201: 1201, 202: 1202, 203: 1203},
		states:  map[int64]identity.Liveness{201: identity.Alive, 202: identity.Dead, 203: identity.Dead},
	}
	var signals []string
	record := Record{
		Suite: "fixture", Root: t.TempDir(), FenceGeneration: 4, Status: StatusRunning,
		SuiteProcess: ProcessIdentity{Pid: 201, Pgid: 201, PidStartedAt: 1201},
		Watchdog:     ProcessIdentity{Pid: 202, PidStartedAt: 1202},
		Launcher:     ProcessIdentity{Pid: 203, PidStartedAt: 1203},
	}
	outcomes := Stop(record, StopOptions{
		TermGrace: time.Millisecond, KillGrace: time.Millisecond, Poll: time.Microsecond, Prober: probe,
		Signal: func(target int, signal syscall.Signal) error {
			signals = append(signals, fmt.Sprintf("%d:%s", target, signal))
			if target == -201 && signal == syscall.SIGTERM {
				probe.states[201] = identity.Dead
			}
			return nil
		},
	})
	if len(outcomes) != 3 || outcomes[0].Component != "suite" || outcomes[0].Result != StopStopped || outcomes[0].Signal != StopSignalTerm ||
		outcomes[1].Component != "watchdog" || outcomes[1].Result != StopAlreadyGone ||
		outcomes[2].Component != "launcher" || outcomes[2].Result != StopAlreadyGone {
		t.Fatalf("outcomes = %+v", outcomes)
	}
	want := fmt.Sprint([]string{"-201:continued", "-201:terminated"})
	if fmt.Sprint(signals) != want {
		t.Fatalf("signals = %v, want %s", signals, want)
	}
}

func TestStopRefusesToSignalARecycledRecordedIdentity(t *testing.T) {
	probe := &stopStateProbe{
		started: map[int64]int64{301: 999, 302: 1302, 303: 1303},
		states:  map[int64]identity.Liveness{301: identity.Alive, 302: identity.Dead, 303: identity.Dead},
	}
	signals := 0
	record := Record{
		Suite: "fixture", Root: t.TempDir(), Status: StatusRunning,
		SuiteProcess: ProcessIdentity{Pid: 301, Pgid: 301, PidStartedAt: 1301},
		Watchdog:     ProcessIdentity{Pid: 302, PidStartedAt: 1302},
		Launcher:     ProcessIdentity{Pid: 303, PidStartedAt: 1303},
	}
	outcomes := Stop(record, StopOptions{
		TermGrace: time.Millisecond, KillGrace: time.Millisecond, Prober: probe,
		Signal: func(int, syscall.Signal) error { signals++; return nil },
	})
	if outcomes[0].Result != StopAlreadyGone || outcomes[0].Signal != StopSignalNone || signals != 0 {
		t.Fatalf("outcomes = %+v, signals = %d", outcomes, signals)
	}
}
