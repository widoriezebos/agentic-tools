package gaterun

import (
	"errors"
	"fmt"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

func TestLaunchDetachedRecordsIdentityBeforeReap(t *testing.T) {
	t.Parallel()
	done := make(chan error, 1)
	var ref identity.Ref
	pid, err := LaunchDetached(DetachedLaunch{Argv: []string{"/usr/bin/true"}, Completed: done, Started: func(pid int64) error {
		exact, state, err := identity.KernelProber{}.Probe(pid)
		if err != nil || state != identity.Alive {
			return fmt.Errorf("identity was reaped before recording: %v %v", state, err)
		}
		ref = exact.Ref()
		return nil
	}})
	if err != nil || ref.Pid != pid || !ref.NativeExact() {
		t.Fatalf("missing durable identity: %+v pid=%d error=%v", ref, pid, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestLaunchDetachedRecordFailureEndsItsGroup(t *testing.T) {
	t.Parallel()
	refused := errors.New("record failed")
	var ref identity.Ref
	pid, err := LaunchDetached(DetachedLaunch{Argv: []string{"/bin/sleep", "600"}, Started: func(pid int64) error {
		exact, state, err := identity.KernelProber{}.Probe(pid)
		if err != nil || state != identity.Alive {
			return fmt.Errorf("cannot inspect child: %v %v", state, err)
		}
		ref = exact.Ref()
		return refused
	}})
	if pid != 0 || !errors.Is(err, refused) || ref.Pid == 0 {
		t.Fatalf("record failure: pid=%d ref=%+v error=%v", pid, ref, err)
	}
	t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL) })
	testenv.Await(t, "failed command group reaped", func() bool {
		return identity.LiveRef(identity.KernelProber{}, ref) == identity.Dead && errors.Is(syscall.Kill(-int(ref.Pid), 0), syscall.ESRCH)
	})
}
