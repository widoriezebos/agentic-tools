package testutil

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type recordingTB struct {
	cleanups []func()
	errs     []string
}

func (*recordingTB) Logf(string, ...any)              {}
func (r *recordingTB) Fatalf(format string, a ...any) { panic(fmt.Sprintf(format, a...)) }
func (r *recordingTB) Cleanup(cleanup func())         { r.cleanups = append(r.cleanups, cleanup) }
func (r *recordingTB) Errorf(f string, a ...any)      { r.errs = append(r.errs, fmt.Sprintf(f, a...)) }

// Custodian-log assertions belong with the cleanup path that owns and retains
// that log; this witness proves the fixture's direct recorded-child cleanup.
func TestHelperFailingBeforeItsKillPointLeavesNoChild(t *testing.T) {
	for index, helperFailure := range []string{"helper write failed", "helper ownership mismatched", "helper did not observe exit"} {
		t.Run(helperFailure, func(t *testing.T) {
			prober, recorder := identity.KernelProber{}, &recordingTB{}
			fixture := newProcessFixture(recorder, t.Name(), prober, syscall.Kill)
			command := fixture.Shell("trap '' TERM\nkill -STOP $$")
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = command.Process.Kill() }()
			var status syscall.WaitStatus
			if _, err := syscall.Wait4(command.Process.Pid, &status, syscall.WUNTRACED, nil); err != nil || status&0xff != 0x7f {
				t.Fatalf("child did not stop: status=%#x err=%v", status, err)
			}
			stopped, _, err := prober.Probe(int64(command.Process.Pid))
			if err != nil {
				t.Fatalf("probe stopped child: %v", err)
			}
			fixture.Record(command.Process.Pid)
			ref := fixture.refs[0]
			recorder.Errorf("%s", helperFailure)
			switch index {
			case 1:
				mismatch := ref
				if mismatch.StartTicks != 0 {
					mismatch.StartTicks++
				} else {
					mismatch.StartedAtUnixMicro++
				}
				if err := identity.SignalExact(prober, mismatch, syscall.SIGKILL); !errors.Is(err, identity.ErrGone) {
					t.Fatalf("helper mismatch check: %v", err)
				}
			case 2:
				if err := command.Process.Kill(); err != nil {
					t.Fatal(err)
				}
				waitForFixtureZombie(t, prober, ref)
			}
			recorder.cleanups[0]()
			waited := make(chan error, 1)
			go func() { waited <- command.Wait() }()
			select {
			case <-waited:
			case <-time.After(30 * time.Second):
				_ = command.Process.Kill()
				<-waited
				t.Fatal("fixture child remained after cleanup")
			}
			failures := strings.Join(recorder.errs, "\n")
			if index == 2 {
				if failures != helperFailure {
					t.Fatalf("zombie failures = %q, want only %q", failures, helperFailure)
				}
				return
			}
			for _, want := range []string{"finished child found running at teardown", fmt.Sprintf("pid=%d", command.Process.Pid), fmt.Sprintf("exe=%q", stopped.Exe), `argv=["/bin/sh"`} {
				if !strings.Contains(failures, want) {
					t.Fatalf("failure text %q does not contain %q", failures, want)
				}
			}
		})
	}
}

func waitForFixtureZombie(t *testing.T, prober identity.Prober, ref identity.Ref) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && state == identity.Alive && identity.SameIdentity(exact, ref) && exact.Zombie {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("killed child did not become a zombie: state=%v err=%v", state, err)
		}
		<-time.After(10 * time.Millisecond)
	}
}
