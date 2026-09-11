//go:build darwin

package proofrun

import (
	"errors"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestPlatformProcessReaderContract(t *testing.T) {
	for value, want := range map[string]float64{
		"00:07":         7,
		"12:34":         12*60 + 34,
		"02:03:04":      2*60*60 + 3*60 + 4,
		"3-02:03:04":    3*24*60*60 + 2*60*60 + 3*60 + 4,
		"0:00.00":       0,
		"0:00.01":       0.01,
		"137:14.32":     137*60 + 14.32,
		"12:34.5":       12*60 + 34.5,
		"1-02:03:04.50": 24*60*60 + 2*60*60 + 3*60 + 4.5,
	} {
		got, err := parseCPUTime(value)
		if err != nil || math.Abs(got-want) > 1e-9 {
			t.Fatalf("parse cputime %q = %.9f, want %.9f, err=%v", value, got, want, err)
		}
	}
	if _, err := parseCPUTime("12.5:34"); err == nil {
		t.Fatal("fractional minutes were accepted")
	}
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
	outcome := superviseCommand(helper.command, options)
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
