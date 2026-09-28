//go:build darwin

package proofrun

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPlatformProcessReaderContract(t *testing.T) {
	parents := map[int]int{100: 1, 101: 100, 102: 101, 200: 1}
	got := descendantProcessIDs(100, parents)
	if len(got) != 3 || got[0] != 100 || got[1] != 101 || got[2] != 102 {
		t.Fatalf("synthetic descendant tree = %v", got)
	}
}

func testPlatformReapedChildCPU(t *testing.T) {
	reader := availableProcessTreeReader(t)
	helper := newSupervisorHelperFixture(t, "retained-child")
	seenChild, seenAfterChild, releasedNested, childCPU := false, false, false, float64(0)
	options := supervisorOptionsForTest(t, 10, 400*time.Millisecond)
	options.Limits.ZeroConsumptionWindow = 0
	helper.gate(&options, reader, func(_ int, sample processTreeSample) {
		liveChild := false
		for _, member := range sample.MemberCPU {
			if member.PID != helper.command.Process.Pid {
				liveChild = true
				if member.CPUSeconds > childCPU {
					childCPU = member.CPUSeconds
				}
			}
		}
		seenChild = seenChild || liveChild
		if !releasedNested && liveChild && childCPU >= 0.25 {
			releasedNested = true
			helper.releaseNested()
		} else if releasedNested && !liveChild {
			seenAfterChild = true
			helper.closeInput()
		}
	})
	outcome := superviseCommand(helper.command, options.supervisorOptions)
	if outcome.Verdict != "" || outcome.WaitErr != nil || !seenChild || !seenAfterChild || childCPU < 0.25 || outcome.CPUSeconds < 0.25 {
		t.Fatalf("Darwin retained-member accounting outcome=%+v seenChild=%v seenAfter=%v childCPU=%.2f", outcome, seenChild, seenAfterChild, childCPU)
	}
}

func processStoppedForTest(pid int) (bool, error) {
	output, err := exec.Command("ps", "-o", "state=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return false, nil
		}
		return false, fmt.Errorf("read Darwin process state: %w", err)
	}
	return strings.ContainsRune(strings.TrimSpace(string(output)), 'T'), nil
}

// TestDarwinReaderReadsTheKernelNotAProgram holds R-138-m1e (Go decides
// natively) for the Darwin tree reader: with no program reachable on PATH it
// still reads a stopped member's state, start identity and the caller's own
// CPU from the kernel.
func TestDarwinReaderReadsTheKernelNotAProgram(t *testing.T) {
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
	stopped := false
	// The ceiling bounds a broken reader; the loop ends on the first sample
	// that shows the stop the signal asked for.
	for deadline := time.Now().Add(time.Minute); !stopped; {
		sample, err := reader.Sample(child.Process.Pid)
		if err != nil {
			t.Fatalf("sample a stopped child without programs on PATH: %v", err)
		}
		if len(sample.Members) != 1 || sample.Members[0] != child.Process.Pid ||
			len(sample.MemberCPU) != 1 || sample.MemberCPU[0].Started == "" {
			t.Fatalf("stopped child sample = %+v", sample)
		}
		stopped = sample.Stopped
		if !stopped && time.Now().After(deadline) {
			t.Fatalf("a SIGSTOPped child never read as stopped: %+v", sample)
		}
	}
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
