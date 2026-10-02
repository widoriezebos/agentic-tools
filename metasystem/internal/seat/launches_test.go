package seat

// A machine's work as presence reads it, where that work runs as a launch of
// the host's launch lane rather than as a delegate job record: a steward seat
// (`claude -p`, launch kind seat) in the checkout, and a goal's build in its
// sibling worktree. Each test makes its own checkout and its own launch store
// under its own temporary directory.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

// launchBed is a checkout (its top folder holds .git, its metasystem folder
// is the root presence reads) beside an empty launch store.
func launchBed(t *testing.T) (top, root, store string) {
	t.Helper()
	base := t.TempDir()
	top = filepath.Join(base, "agentic-tools-m1f")
	root = filepath.Join(top, "metasystem")
	store = filepath.Join(base, "launch")
	for _, dir := range []string{filepath.Join(top, ".git"), filepath.Join(root, "artifacts", "agents", "steward", "seats"), store} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return top, root, store
}

func writeLaunch(t *testing.T, store string, record launch.Record) {
	t.Helper()
	if err := (launch.Store{Root: store}).Create(record); err != nil {
		t.Fatal(err)
	}
}

// writeSeatStart is the steward's own record of the seat it started, which
// is where the goal a seat launch serves is written down.
func writeSeatStart(t *testing.T, root, launchID, goal string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"schema": 1, "launchId": launchID, "goal": goal, "machine": "m1f"})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts", "agents", "steward", "seats", launchID+".json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func composedWords(t *testing.T, jobs JobSet) string {
	t.Helper()
	record, _, err := Compose("m1f", RunnerContext{Generation: 1, Engine: "abc1234"}, jobs, nil, fixtureClock)
	if err != nil {
		t.Fatal(err)
	}
	return PhaseWords(record.Working, fixtureClock)
}

func TestAStewardSeatLaunchRunningInTheCheckoutReadsAsWorkingOnItsGoal(t *testing.T) {
	t.Parallel()
	top, root, store := launchBed(t)
	writeLaunch(t, store, launch.Record{ID: "seat-13b7096f13144397", Kind: "seat", WorkingDirectory: top,
		State: launch.Running, StartedAt: fixtureClock.Add(-27 * time.Minute).Format(time.RFC3339Nano)})
	writeSeatStart(t, root, "seat-13b7096f13144397", "one-folder-deployed-and-evolved")

	words := composedWords(t, readWork(root, store))
	if words != "working on one-folder-deployed-and-evolved · running 27 min" {
		t.Fatalf("the Running column says %q", words)
	}
}

func TestAMachineWithNothingRunningReadsIdle(t *testing.T) {
	t.Parallel()
	top, root, store := launchBed(t)
	// An ended seat launch of this checkout is not work in hand.
	writeLaunch(t, store, launch.Record{ID: "seat-00000000000000aa", Kind: "seat", WorkingDirectory: top,
		State: launch.Completed, StartedAt: fixtureClock.Add(-2 * time.Hour).Format(time.RFC3339Nano)})

	if words := composedWords(t, readWork(root, store)); words != "idle" {
		t.Fatalf("the Running column says %q", words)
	}
}

func TestALaunchInAnotherMachinesCheckoutIsNotThisMachinesWork(t *testing.T) {
	t.Parallel()
	top, root, store := launchBed(t)
	// agentic-tools-m1f-other is not the worktree of a goal this launch names,
	// and agentic-tools-m1 is a different checkout altogether.
	writeLaunch(t, store, launch.Record{ID: "seat-00000000000000bb", Kind: "seat", WorkingDirectory: top + "-other",
		State: launch.Running, StartedAt: fixtureClock.Format(time.RFC3339Nano)})
	writeLaunch(t, store, launch.Record{ID: "seat-00000000000000cc", Kind: "seat", WorkingDirectory: strings.TrimSuffix(top, "f"),
		State: launch.Running, StartedAt: fixtureClock.Format(time.RFC3339Nano)})

	if words := composedWords(t, readWork(root, store)); words != "idle" {
		t.Fatalf("the Running column says %q", words)
	}
}

func TestABuildLaunchInTheGoalWorktreeShowsItsStage(t *testing.T) {
	t.Parallel()
	top, root, store := launchBed(t)
	writeLaunch(t, store, launch.Record{ID: "seat-13b7096f13144397", Kind: "seat", WorkingDirectory: top,
		State: launch.Running, StartedAt: fixtureClock.Add(-40 * time.Minute).Format(time.RFC3339Nano)})
	writeSeatStart(t, root, "seat-13b7096f13144397", "one-folder-deployed-and-evolved")
	// The newer launch is the seat's revise round, in the goal's sibling
	// worktree, so it is what the row names.
	writeLaunch(t, store, launch.Record{ID: "20261002t205107-54912daf08-r3-s1", Kind: "build", Goal: "one-folder-deployed-and-evolved",
		WorkingDirectory: top + "-one-folder-deployed-and-evolved", State: launch.Running, Round: 3, MaxRounds: 20,
		StartedAt: fixtureClock.Add(-14 * time.Minute).Format(time.RFC3339Nano)})

	words := composedWords(t, readWork(root, store))
	if words != "revise round 3 of 20 on one-folder-deployed-and-evolved · running 14 min" {
		t.Fatalf("the Running column says %q", words)
	}
	// This machine's own row lists both.
	working, problem := WorkingInFlight(readWork(root, store), nil)
	if problem != "" || len(working) != 2 {
		t.Fatalf("in flight = %d, %q", len(working), problem)
	}
}
