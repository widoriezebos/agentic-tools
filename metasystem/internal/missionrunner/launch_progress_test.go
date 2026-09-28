package missionrunner

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestTreeCPUProgressOnAScriptedSampler drives the progress tracker with a
// scripted CPU reader and an artificial clock: the first-sample rule, the
// once-a-second gate, growth, and an unreadable tree, with no wall time.
func TestTreeCPUProgressOnAScriptedSampler(t *testing.T) {
	readings := []struct {
		cpu float64
		ok  bool
	}{{1, true}, {1, true}, {1.5, true}, {0, false}, {1.5, true}, {2, true}}
	taken := 0
	progress := newTreeCPUProgress(4242)
	progress.sample = func(rootPID int) (float64, bool) {
		if rootPID != 4242 {
			t.Fatalf("sampled pid %d, not the tracked root", rootPID)
		}
		reading := readings[taken]
		taken++
		return reading.cpu, reading.ok
	}
	now := time.Date(2026, 9, 12, 16, 0, 0, 0, time.UTC)
	if progress.advanced(now) {
		t.Fatal("the first sample has nothing to compare with")
	}
	if progress.advanced(now.Add(500*time.Millisecond)) || taken != 1 {
		t.Fatalf("a moment inside the interval read the tree (samples %d)", taken)
	}
	if progress.advanced(now.Add(time.Second)) {
		t.Fatal("unchanged CPU counted as progress")
	}
	if !progress.advanced(now.Add(2 * time.Second)) {
		t.Fatal("a growing tree must count as progress")
	}
	if progress.advanced(now.Add(3 * time.Second)) {
		t.Fatal("an unreadable tree counted as progress")
	}
	if progress.advanced(now.Add(4 * time.Second)) {
		t.Fatal("an unreadable sample moved the comparison point")
	}
	if !progress.advanced(now.Add(5*time.Second)) || taken != 6 {
		t.Fatalf("growth after an unreadable sample was missed (samples %d)", taken)
	}

	original := runClock
	artificialNow := now
	nowCalls, sleeps := 0, 0
	runClock.now = func() time.Time { nowCalls++; return artificialNow }
	runClock.sleep = func(wait time.Duration) { artificialNow = artificialNow.Add(wait); sleeps++ }
	t.Cleanup(func() { runClock = original })
	window := newLaunchVerificationWindow(15 * time.Second)
	windowProgress := newTreeCPUProgress(4243)
	windowProgress.interval = 0
	windowProgress.sample = func(int) (float64, bool) { return float64(nowCalls), true }
	window.extendOnProgress(windowProgress) // establishes the first sample
	window.pause(time.Second)
	window.extendOnProgress(windowProgress) // observed growth extends the window
	if window.deadline != now.Add(16*time.Second) || window.ceiling != now.Add(120*time.Second) ||
		sleeps != 1 || nowCalls < 3 || !window.open() {
		t.Fatalf("artificial launch window: deadline=%s ceiling=%s sleeps=%d nowCalls=%d open=%v",
			window.deadline, window.ceiling, sleeps, nowCalls, window.open())
	}
}

// TestProcessTreeCPUSecondsReadsTheKernelNotAProgram is the reader's wiring
// proof (R-138-m1e, Go decides natively): with no program reachable on PATH,
// a tree whose root sits idle while its grandchild computes still reports
// the grandchild's CPU, so the reading comes from the kernel and descendant
// time is summed. An absent root reports no sample.
func TestProcessTreeCPUSecondsReadsTheKernelNotAProgram(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	command := exec.Command("/bin/sh", "-c", "/bin/sh -c 'while :; do :; done' & wait")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Process.Kill()
		_ = command.Wait()
	})
	// The ceiling bounds a broken reader, not the machine's speed: the loop
	// ends on the first reading past the threshold.
	deadline := time.Now().Add(2 * time.Minute)
	for {
		cpu, ok := processTreeCPUSeconds(command.Process.Pid)
		if !ok {
			t.Fatal("a live tree reported no sample")
		}
		if cpu >= 0.2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("idle root with a computing grandchild read %v, %v; want at least 0.2s, true", cpu, ok)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
