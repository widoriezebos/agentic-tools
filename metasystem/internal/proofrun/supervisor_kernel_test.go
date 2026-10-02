//go:build darwin || linux

package proofrun

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
)

// TestProcessTreeReaderReadsTheKernelNotAProgram holds R-138-m1e (Go decides
// natively) for the platform tree reader (Darwin and Linux): with no program reachable on PATH it
// still reads a stopped member's state, start identity and the caller's own
// CPU from the kernel.
func TestProcessTreeReaderReadsTheKernelNotAProgram(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	busyForCPU(50 * time.Millisecond)
	child := exec.Command("/bin/sleep", "60")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	})
	if err := syscall.Kill(child.Process.Pid, syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	reader := newProcessTreeReader()
	// The wait ends on the first sample that shows the stop the signal asked
	// for; the kernel publishes it when the signal is delivered.
	testenv.Await(t, "a SIGSTOPped child to read as stopped", func() bool {
		sample, err := reader.Sample(child.Process.Pid)
		if err != nil {
			t.Fatalf("sample a stopped child without programs on PATH: %v", err)
		}
		if len(sample.Members) != 1 || sample.Members[0] != child.Process.Pid ||
			len(sample.MemberCPU) != 1 || sample.MemberCPU[0].Started == "" {
			t.Fatalf("stopped child sample = %+v", sample)
		}
		return sample.Stopped
	})
	// The stopped child is the test process's descendant; reap it so the
	// own sample reads a tree with nothing stopped in it.
	_ = child.Process.Kill()
	_ = child.Wait()
	own, err := reader.Sample(os.Getpid())
	if err != nil {
		t.Fatalf("sample the test process: %v", err)
	}
	found := false
	for _, member := range own.MemberCPU {
		if member.PID == os.Getpid() {
			found = true
			if member.CPUSeconds < 0.05 {
				t.Fatalf("own CPU read %.3fs after at least 0.05s of computing", member.CPUSeconds)
			}
		}
	}
	if !found || own.Stopped {
		t.Fatalf("own sample = %+v", own)
	}
}
