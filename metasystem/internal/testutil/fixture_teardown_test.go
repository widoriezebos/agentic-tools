package testutil

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestReapKeySurvivorsRequiresCertainLiveIdentityBeforeSignal(t *testing.T) {
	scanErr := errors.New("fixture scan failed")
	survivorPID := int64(os.Getpid()) + 1
	pidText := fmt.Sprintf("pid=%d", survivorPID)
	for _, test := range []struct {
		name         string
		class        identity.FixtureSurvivorClass
		scanErr      error
		zombie       bool
		diesOnSignal bool
		wantSignal   bool
		wantFailures []string
	}{
		{"unproven ownership", identity.FixtureSurvivorUnreadable, nil, false, false, false, []string{"ownership unproven", pidText}},
		{"scan error", "", scanErr, false, false, false, []string{scanErr.Error()}},
		{"certain zombie", identity.FixtureSurvivorCertain, nil, true, false, false, nil},
		{"certain live survivor", identity.FixtureSurvivorCertain, nil, false, true, true, []string{"unrecorded fixture child", pidText}},
	} {
		t.Run(test.name, func(t *testing.T) {
			owner := fixtureTeardownExact(int64(os.Getpid()), 1)
			survivor := fixtureTeardownExact(survivorPID, 2)
			survivor.Zombie = test.zombie
			signaled := false
			prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
				switch pid {
				case owner.Pid:
					return owner, identity.Alive, nil
				case survivor.Pid:
					if signaled && test.diesOnSignal {
						return identity.Exact{}, identity.Dead, nil
					}
					return survivor, identity.Alive, nil
				default:
					return identity.Exact{}, identity.Dead, nil
				}
			})
			type sentSignal struct {
				pid int
				sig syscall.Signal
			}
			var sent []sentSignal
			recorder := &recordingTB{}
			fixture := newProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(pid int, sig syscall.Signal) error {
				signaled = true
				sent = append(sent, sentSignal{pid, sig})
				return nil
			})
			fixture.scan = func(identity.FixtureKey) ([]identity.FixtureSurvivor, error) {
				if test.scanErr != nil {
					return nil, test.scanErr
				}
				return []identity.FixtureSurvivor{{Class: test.class, Ref: survivor.Ref(), Exe: "/fixture-child", Argv: []string{"fixture-child"}}}, nil
			}
			recorder.cleanups[0]()
			if test.wantSignal {
				if len(sent) != 1 || int64(sent[0].pid) != survivorPID || sent[0].sig != syscall.SIGKILL {
					t.Fatalf("signals = %#v; want one SIGKILL for pid %d", sent, survivorPID)
				}
			} else if len(sent) != 0 {
				t.Fatalf("signals = %#v; want none", sent)
			}
			wantFailureCount := 0
			if len(test.wantFailures) > 0 {
				wantFailureCount = 1
			}
			if len(recorder.errs) != wantFailureCount {
				t.Fatalf("failures = %q; want %d", recorder.errs, wantFailureCount)
			}
			failures := strings.Join(recorder.errs, "\n")
			for _, want := range test.wantFailures {
				if !strings.Contains(failures, want) {
					t.Fatalf("failures = %q; want %q", failures, want)
				}
			}
		})
	}
}

func fixtureTeardownExact(pid, token int64) identity.Exact {
	exact := identity.Exact{Pid: pid, StartedAt: time.Unix(100, token*1_000)}
	if runtime.GOOS == "linux" {
		exact.StartTicks, exact.BootID = token, fmt.Sprintf("fixture-boot-%d", token)
	}
	return exact
}

// TestFixtureTeardownOutwaitsASlowExit: teardown waits for the exit its own
// SIGKILL caused, however slow, and reports no failure. The child answers
// Alive until its 100th probe (or until the recorder holds a failure), then
// Dead. Nothing is timed: under a teardown bound the deadline, once fired,
// stays ready, and the 10 ms tick would have to win every one of the 99
// selects that follow.
func TestFixtureTeardownOutwaitsASlowExit(t *testing.T) {
	t.Parallel()
	owner := fixtureTeardownExact(int64(os.Getpid()), 1)
	child := fixtureTeardownExact(500, 2)
	recorder := &recordingTB{}
	childProbes := 0
	prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid == owner.Pid {
			return owner, identity.Alive, nil
		}
		childProbes++
		if len(recorder.errs) > 0 || childProbes >= 100 {
			return identity.Exact{}, identity.Dead, nil
		}
		return child, identity.Alive, nil
	})
	signals := 0
	fixture := newProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(pid int, sig syscall.Signal) error {
		if pid == int(child.Pid) && sig == syscall.SIGKILL {
			signals++
		}
		return nil
	})
	fixture.scan = noFixtureSurvivors
	fixture.Hold(int(child.Pid))
	recorder.cleanups[0]()
	if len(recorder.errs) != 0 || signals != 1 || childProbes < 100 {
		t.Fatalf("teardown of a slow exit: failures=%q signals=%d child probes=%d", recorder.errs, signals, childProbes)
	}
}

// TestFixtureTeardownBoundsTheWaitForAChildItDidNotKill: a recorded child
// whose identity teardown cannot prove is never signaled, so its exit is not
// guaranteed and its wait keeps a bound: teardown reports it and returns. The
// child answers Unknown after it is recorded. If teardown is still probing it
// at probe 300, the wait has no bound; the prober notes that and answers Dead
// so the test ends. Nothing is timed: once the 1 ms bound has fired it stays
// ready, so reaching probe 300 under it needs the 10 ms tick to win about 300
// selects in a row.
func TestFixtureTeardownBoundsTheWaitForAChildItDidNotKill(t *testing.T) {
	t.Parallel()
	owner := fixtureTeardownExact(int64(os.Getpid()), 1)
	child := fixtureTeardownExact(500, 2)
	recorder := &recordingTB{}
	recorded, unbounded, childProbes := false, false, 0
	prober := fixtureProbeFunc(func(pid int64) (identity.Exact, identity.Liveness, error) {
		if pid == owner.Pid {
			return owner, identity.Alive, nil
		}
		if !recorded {
			return child, identity.Alive, nil
		}
		childProbes++
		if childProbes >= 300 {
			unbounded = true
			return identity.Exact{}, identity.Dead, nil
		}
		return identity.Exact{}, identity.Unknown, errors.New("unreadable")
	})
	signals := 0
	fixture := newProcessFixture(recorder, t.Name(), owner.Ref(), true, prober, func(int, syscall.Signal) error { signals++; return nil })
	fixture.scan = noFixtureSurvivors
	fixture.unkilledWaitBound = time.Millisecond
	fixture.Hold(int(child.Pid))
	recorded = true
	recorder.cleanups[0]()
	failures := strings.Join(recorder.errs, "\n")
	if unbounded || signals != 0 || !strings.Contains(failures, "child identity unproven at teardown") || !strings.Contains(failures, "child did not exit") {
		t.Fatalf("teardown of an unproven child: unbounded=%t signals=%d child probes=%d failures=%q", unbounded, signals, childProbes, failures)
	}
}
