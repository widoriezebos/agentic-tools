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
			fixture := newProcessFixture(recorder, t.Name(), prober, func(pid int, sig syscall.Signal) error {
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
