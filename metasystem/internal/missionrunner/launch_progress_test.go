package missionrunner

import (
	"bufio"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
// a tree whose root shell only waits while its grandchild has computed still
// reports the grandchild's CPU on top of the root's, so the reading comes
// from the kernel and descendant time is summed. The grandchild computes a
// fixed amount, says so, then blocks on its input, so the test waits on a
// read and never on the clock. An absent root reports no sample.
func TestProcessTreeCPUSecondsReadsTheKernelNotAProgram(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	command := exec.Command("/bin/sh", "-c", `/usr/bin/awk 'BEGIN { for (i = 0; i < 2000000; i++) s += i; print "computed"; fflush(); getline line < "-" }'; :`)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		_ = command.Wait()
	})
	line, err := bufio.NewReader(output).ReadString('\n')
	if err != nil || line != "computed\n" {
		t.Fatalf("grandchild said %q, %v", line, err)
	}
	census, err := identity.TakeProcessCensus()
	if err != nil {
		t.Fatal(err)
	}
	root := int64(command.Process.Pid)
	var grandchild int64
	for _, pid := range census.Pids() {
		if parent, known := census.Parent(pid); known && parent == root {
			grandchild = pid
		}
	}
	if grandchild == 0 {
		t.Fatal("the root shell has no computing child")
	}
	rootCPU, rootErr := identity.ProcessCPUSeconds(root)
	childCPU, childErr := identity.ProcessCPUSeconds(grandchild)
	if rootErr != nil || childErr != nil || childCPU <= 0 {
		t.Fatalf("per-process CPU root=%v (%v) child=%v (%v)", rootCPU, rootErr, childCPU, childErr)
	}
	total, ok := processTreeCPUSeconds(command.Process.Pid)
	if !ok || total < rootCPU+childCPU {
		t.Fatalf("tree CPU = %v, %v; want at least root %v plus grandchild %v", total, ok, rootCPU, childCPU)
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
