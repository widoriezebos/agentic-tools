package missionrunner

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

// TestProcessTreeCPUSecondsCountsDescendants is the reader's wiring proof:
// a busy child under a shell, and the tree's CPU time grows while the
// shell itself sits in wait, so descendant time must be part of the sum.
func TestProcessTreeCPUSecondsCountsDescendants(t *testing.T) {
	// The shell and its burner share a process group of their own, and the
	// group is what ends. A bare Process.Kill ended only the shell, before
	// its own `kill $!` ran, and every run of this test left a `yes` at a
	// full core behind it: ten of them burned through the night of
	// 2026-09-11 under every cadence measurement (found 2026-09-12 08:40,
	// parent launchd, start times matching the runs). The shell's own
	// lifetime pipe keeps cleanup tied to that whole group rather than only
	// to the shell process returned by Start.
	startRead, startWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	lifetimeRead, lifetimeWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	reportRead, reportWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	// The burner starts behind a pipe gate, consumes its own CPU in Bash,
	// reports `times` on a second pipe, then becomes yes. The lifetime pipe's
	// writer is inherited by the shell and burner; EOF therefore proves every
	// member has exited after cleanup.
	script := `bash -c '
printf "%s\n" "$$" >&5
read -r _ <&3
i=0
while [ "$i" -lt 50000 ]; do i=$((i + 1)); done
times >&5
exec yes >/dev/null
' &
wait`
	command := exec.Command("bash", "-c", script)
	command.ExtraFiles = []*os.File{startRead, lifetimeWrite, reportWrite}
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	startRead.Close()
	lifetimeWrite.Close()
	reportWrite.Close()
	group := command.Process.Pid
	burnerPID := 0
	t.Cleanup(func() {
		startWrite.Close()
		_ = syscall.Kill(-group, syscall.SIGKILL)
		if burnerPID > 0 {
			_ = syscall.Kill(burnerPID, syscall.SIGKILL)
		}
		_ = command.Wait()
		if _, err := io.Copy(io.Discard, lifetimeRead); err != nil {
			t.Errorf("read process-group lifetime pipe: %v", err)
		}
		lifetimeRead.Close()
		reportRead.Close()
	})
	reportReader := bufio.NewReader(reportRead)
	pidLine, err := reportReader.ReadString('\n')
	if err != nil {
		t.Fatalf("burner did not report its pid: %v", err)
	}
	burnerPID, err = strconv.Atoi(strings.TrimSpace(pidLine))
	if err != nil || burnerPID < 2 {
		t.Fatalf("burner pid report %q: %v", pidLine, err)
	}
	first, ok := processTreeCPUSeconds(group)
	if !ok {
		t.Skip("platform process reader is unavailable on this test host")
	}
	if _, err := startWrite.Write([]byte("burn\n")); err != nil {
		t.Fatal(err)
	}
	startWrite.Close()
	reported, err := reportReader.ReadString('\n')
	if err != nil {
		t.Fatalf("burner did not report its CPU fact: %v", err)
	}
	second, ok := processTreeCPUSeconds(group)
	if !ok || second <= first {
		t.Fatalf("descendant CPU did not grow after burner report %q: first=%v second=%v ok=%v", reported, first, second, ok)
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
