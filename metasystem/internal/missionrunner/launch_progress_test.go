package missionrunner

import (
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestParsePSCPUTime(t *testing.T) {
	for value, want := range map[string]float64{"0:00.12": 0.12, "1:02.50": 62.5, "1:01:01": 3661, "2-01:00:00": 176400} {
		got, err := parsePSCPUTime(value)
		if err != nil || got != want {
			t.Fatalf("%s: got %v err=%v want %v", value, got, err, want)
		}
	}
	for _, value := range []string{"", "12", "a:b", "1:-2"} {
		if _, err := parsePSCPUTime(value); err == nil {
			t.Fatalf("%q parsed", value)
		}
	}
}

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
}

// TestProcessTreeCPUSecondsCountsDescendants is the reader's wiring proof:
// a busy child under a shell, and the tree's CPU time grows while the
// shell itself sits in wait, so descendant time must be part of the sum.
// It waits for the growth it asserts, bounded far above what a burner
// needs to show up in ps.
func TestProcessTreeCPUSecondsCountsDescendants(t *testing.T) {
	// The shell and its burner share a process group of their own, and the
	// group is what ends. A bare Process.Kill ended only the shell, before
	// its own `kill $!` ran, and every run of this test left a `yes` at a
	// full core behind it: ten of them burned through the night of
	// 2026-09-11 under every cadence measurement (found 2026-09-12 08:40,
	// parent launchd, start times matching the runs). The shell's own
	// timer is the last resort should this process die before its cleanup,
	// and sits far beyond the cleanup's bound so it never satisfies it.
	command := exec.Command("sh", "-c", "yes >/dev/null & sleep 600; kill $!")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	group := command.Process.Pid
	t.Cleanup(func() {
		_ = syscall.Kill(-group, syscall.SIGKILL)
		_ = command.Wait()
		// The burner is launchd's to reap once the shell is gone; a killed
		// process answers signal 0 until it is reaped (cadence run 12 read
		// the group alive 2 ms after the kill).
		deadline := time.Now().Add(wiringBound)
		for syscall.Kill(-group, 0) == nil {
			if time.Now().After(deadline) {
				t.Errorf("the burner outlived the test in process group %d", group)
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	})
	first, ok := processTreeCPUSeconds(group)
	if !ok {
		t.Skip("platform process reader is unavailable on this test host")
	}
	deadline := time.Now().Add(wiringBound)
	for {
		second, ok := processTreeCPUSeconds(group)
		if ok && second > first {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant CPU did not grow within %s: first=%v last=%v ok=%v", wiringBound, first, second, ok)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
