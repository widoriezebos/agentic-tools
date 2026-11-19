package identity

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestTaggedSurvivorsSeesARealTaggedProcess(t *testing.T) {
	// No tag recorded: nothing to scan for, no claim either way.
	if alive, certain := TaggedSurvivors(FixedProcessTable{}, "", 0, 0); alive || !certain {
		t.Fatalf("an empty tag scans nothing: %v %v", alive, certain)
	}

	// The shell carries the tag in $0. The trailing exit keeps the shell
	// from replacing itself with sleep, which would lose the tag.
	tag := "metasystem-survivor-scan-fixture"
	child := exec.Command("/bin/sh", "-c", "sleep 30; exit 0", tag)
	// Its own process group: the indeterminacy scope the reapers
	// pass is the recorded group, and the test's assertions must not
	// depend on whatever else this machine runs under go test's
	// group.
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	childGroup := int64(child.Process.Pid)
	// The scan reads only the test's own child: no other process of the
	// host (another test's, or a pid reused into childGroup) decides it.
	processes := ListedProcessTable{childGroup}
	reaped := false
	defer func() {
		if !reaped {
			_ = syscall.Kill(-child.Process.Pid, syscall.SIGKILL)
			_ = child.Wait()
		}
	}()
	deadline := time.Now().Add(5 * time.Second)
	if testDeadline, ok := t.Deadline(); ok && testDeadline.Before(deadline) {
		deadline = testDeadline.Add(-100 * time.Millisecond)
	}
	waitForSurvivor := func(wantAlive bool) {
		t.Helper()
		for {
			alive, certain := TaggedSurvivors(processes, tag, 0, childGroup)
			if alive == wantAlive && certain {
				return
			}
			if wantAlive {
				if _, state, err := (KernelProber{}).Probe(childGroup); err == nil && state == Dead {
					t.Fatal("tagged child exited before it could be observed")
				}
			}
			if !time.Now().Before(deadline) {
				t.Fatalf("survivor scan did not settle: alive=%t certain=%t, want alive=%t", alive, certain, wantAlive)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}
	waitForSurvivor(true)

	// The recorded custodian itself is excluded: a scan that counted
	// the dead-but-probed custodian would defer forever.
	if alive, certain := TaggedSurvivors(processes, tag, int64(child.Process.Pid), childGroup); alive || !certain {
		t.Fatalf("the custodian pid is not its own survivor: %v %v", alive, certain)
	}

	// With the child gone the scan comes back clear and the verdict
	// may conclude.
	if err := syscall.Kill(-child.Process.Pid, syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	_ = child.Wait()
	reaped = true
	waitForSurvivor(false)
}
