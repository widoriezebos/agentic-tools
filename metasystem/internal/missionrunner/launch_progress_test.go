package missionrunner

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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
// the real ps command path, parser, and tree traversal see a child whose CPU
// grows while the root stays unchanged, so descendant time must be summed.
func TestProcessTreeCPUSecondsCountsDescendants(t *testing.T) {
	bin := t.TempDir()
	calls := filepath.Join(bin, "ps-calls")
	ps := `#!/usr/bin/env bash
set -euo pipefail
[[ "$*" == "-axo pid=,ppid=,cputime=" ]] || exit 97
count=0
if [[ -f "${PS_FIXTURE_CALLS:?}" ]]; then
  read -r count <"$PS_FIXTURE_CALLS"
fi
count=$((count + 1))
printf '%d\n' "$count" >"$PS_FIXTURE_CALLS"
child_cpu=0:00
[[ "$count" -eq 1 ]] || child_cpu=0:01
printf '4242 1 0:02\n4243 4242 %s\n9000 1 9:59\n' "$child_cpu"
`
	if err := testexec.WriteFile(filepath.Join(bin, "ps"), []byte(ps), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PS_FIXTURE_CALLS", calls)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	first, ok := processTreeCPUSeconds(4242)
	if !ok || first != 2 {
		t.Fatalf("first scripted process-tree sample = %v, %v; want 2, true", first, ok)
	}
	second, ok := processTreeCPUSeconds(4242)
	if !ok || second != 3 {
		t.Fatalf("second scripted process-tree sample = %v, %v; want 3, true", second, ok)
	}
	if _, ok := processTreeCPUSeconds(os.Getpid() + 1_000_000); ok {
		t.Fatal("an absent process must report no sample")
	}
}
