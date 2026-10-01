package main

import (
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// laneRefusalLines are the two lines work land of a job or of a hand-made
// change prints while this computer has a landing lane: the lane takes only
// a goal's branch, so a seat never silently lands its own work beside it;
// line 2 is the one command.
const (
	laneRefusalLine1 = "✗ this computer has a landing lane, which takes only a goal's branch (work land G)"
	laneRefusalLine2 = "  → metasystem landing unset  lets this seat land its own work"
)

func expectLaneRefusal(t *testing.T, label string, code int, stderr string) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(stderr, "\n"), "\n")
	if code != 1 || len(lines) != 2 || lines[0] != laneRefusalLine1 || lines[1] != laneRefusalLine2 {
		t.Fatalf("%s: code=%d stderr=%q", label, code, stderr)
	}
}

// TestWorkLandRefusesWhileALaneIsRegistered: with a landing lane registered,
// work land of a job and of a hand-made change each refuse with the two
// lines and land nothing (a goal is handed in instead:
// landing_plain_handin_test.go); without a lane the goal is read for its
// own landing as before.
func TestWorkLandRefusesWhileALaneIsRegistered(t *testing.T) {
	t.Parallel()
	b := newDeliveryBed(t)
	configured := true
	b.owners.laneRoot = func(string, time.Time) (string, bool, error) { return "/landing", configured, nil }
	branchReads := 0
	b.owners.branchState = func(string, string) (intentBranchState, error) {
		branchReads++
		return intentBranchState{EndpointTip: "e1"}, nil
	}
	lands := 0
	b.owners.landPath = func(landpath.Owners, landpath.LandRequest, io.Writer, io.Writer) int { lands++; return 0 }
	run := func(args ...string) (int, string) {
		owners := b.intentBed.owners()
		owners.delivery = b.owners
		code, _, stderr := b.run(owners, args...)
		return code, stderr
	}

	b.writeJob(map[string]any{"jobId": "j-1", "goalId": "g1"})
	code, stdout := run("work", "land", "j2:j-1")
	expectLaneRefusal(t, "job", code, stdout)

	if output, err := exec.Command("git", "-C", b.install, "-c", "user.name=W", "-c", "user.email=w@example.com",
		"commit", "-q", "--allow-empty", "-m", "base").CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	message := filepath.Join(t.TempDir(), "message.txt")
	if err := os.WriteFile(message, []byte("record: notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout = run("work", "land", "--message", message, "--staged")
	expectLaneRefusal(t, "change", code, stdout)
	if lands != 0 {
		t.Fatalf("change: the landing path ran %d times beside a lane", lands)
	}

	configured = false
	code, result := b.do("work", "land", "g1")
	if branchReads != 1 || strings.Contains(result.Summary, "landing lane") {
		t.Fatalf("without a lane: code=%d branchReads=%d result=%+v", code, branchReads, result)
	}
}
