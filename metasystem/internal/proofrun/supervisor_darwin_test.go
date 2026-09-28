//go:build darwin

package proofrun

import (
	"errors"
	"fmt"
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
