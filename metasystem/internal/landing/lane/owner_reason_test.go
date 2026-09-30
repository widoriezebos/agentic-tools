package lane

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// nestedLane makes root a checkout that nests the module (root/metasystem
// holds go.mod), as every real landing checkout of this repository does.
func nestedLane(t *testing.T, root string) string {
	t.Helper()
	module := filepath.Join(root, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return module
}

func writeOwnerError(t *testing.T, root, text string) {
	t.Helper()
	path := LastErrorPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// The owner's last error lives in its installation's supervision, which in
// a nested checkout is <lane>/metasystem/artifacts/agents/supervision: the
// path the lane names is the one the owner writes.
func TestLastErrorPathIsTheInstallationsSupervision(t *testing.T) {
	t.Parallel()
	_, root, _ := laneDirs(t)
	if got, want := LastErrorPath(root), filepath.Join(root, "artifacts", "agents", "supervision", "landing-owner.last-error"); got != want {
		t.Fatalf("flat checkout: %s, want %s", got, want)
	}
	module := nestedLane(t, root)
	if got, want := LastErrorPath(root), filepath.Join(module, "artifacts", "agents", "supervision", "landing-owner.last-error"); got != want {
		t.Fatalf("nested checkout: %s, want %s", got, want)
	}
}

// An owner that keeps dying says why: status's one line carries the owner's
// last error in plain words, not only a restart count.
func TestViewOfARestartingOwnerSaysWhy(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	nestedLane(t, root)
	register(t, home, root)
	if err := writeJSON(home, keeperPath(home), KeeperState{Failures: 1, Restarts: 1, Since: laneNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	writeOwnerError(t, root, "the owner could not read its batch store\n")
	view := BuildView(viewSources(home, false, nil))
	if view.Owner.State != OwnerRestarting || view.Owner.LastExit == nil || *view.Owner.LastExit != "the owner could not read its batch store" {
		t.Fatalf("owner = %+v", view.Owner)
	}
	for _, want := range []string{"owner restarting", "1 restart so far", "the owner could not read its batch store"} {
		if !strings.Contains(view.Summary, want) {
			t.Errorf("summary %q lacks %q", view.Summary, want)
		}
	}
	if strings.Contains(view.Summary, "1 restarts") {
		t.Errorf("summary %q miscounts one restart", view.Summary)
	}
}

// A lane whose owner cannot run (its checkout's supervision is not armed)
// shows the owner not started, why, and the one command a person runs.
func TestViewOfALaneThatCannotStartNamesTheFix(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	if err := writeJSON(home, keeperPath(home), KeeperState{Failures: 1, Restarts: 1, Since: laneNow.Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	sources := viewSources(home, false, nil)
	sources.Ready = func(root string) error { return UnarmedRefusal(root) }
	view := BuildView(sources)
	fix := "metasystem system start --repo " + resolved(root)
	if view.Owner.State != OwnerNotStarted || view.Owner.LastExit == nil || !strings.Contains(*view.Owner.LastExit, "supervision is not armed") ||
		view.Owner.RetryHint == nil || !strings.Contains(*view.Owner.RetryHint, fix) {
		t.Fatalf("owner = %+v", view.Owner)
	}
	if strings.Join(view.Owner.Fix, " ") != fix {
		t.Fatalf("fix argv = %q, want %q", view.Owner.Fix, fix)
	}
	for _, want := range []string{"owner not-started", "supervision is not armed", fix} {
		if !strings.Contains(view.Summary, want) {
			t.Errorf("summary %q lacks %q", view.Summary, want)
		}
	}
	// A running owner is never second-guessed.
	sources = viewSources(home, true, nil)
	sources.Ready = func(string) error { t.Fatal("asked whether a running owner can start"); return nil }
	if view := BuildView(sources); view.Owner.State != OwnerRunning {
		t.Fatalf("running owner = %+v", view.Owner)
	}
}

// The keeper never restarts an owner that cannot run, and never counts it
// as a death: it says why and what a person runs.
func TestKeeperDoesNotRestartAnOwnerThatCannotStart(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	clock := laneNow
	owner := &fakeOwner{}
	keeper := owner.keeper(home, &clock)
	keeper.Ready = func(root string) error { return NoMachineRefusal(root) }
	line := keeper.Step()
	for _, want := range []string{"no machine nickname", "git -C " + resolved(root) + " config metasystem.goal.machine landing"} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q lacks %q", line, want)
		}
	}
	if len(owner.starts) != 0 || ReadKeeper(home) != (KeeperState{}) {
		t.Fatalf("starts %v, state %+v; want no start and no death counted", owner.starts, ReadKeeper(home))
	}
	if strings.Contains(line, "restarted") {
		t.Fatalf("line %q claims a restart", line)
	}
}

// A restart the keeper asks for is a request to the checkout's supervision:
// the line says so and never claims the owner runs; the next cycle says
// whether it does. A later death names the owner's last error.
func TestKeeperLineNeverClaimsTheOwnerRuns(t *testing.T) {
	t.Parallel()
	home, root, _ := laneDirs(t)
	register(t, home, root)
	clock := laneNow
	owner := &fakeOwner{}
	keeper := owner.keeper(home, &clock)
	line := keeper.Step()
	if len(owner.starts) != 1 || !strings.Contains(line, "asked its supervision to start it (restart 1)") || strings.Contains(line, "restarted it") {
		t.Fatalf("restart line %q, starts %v", line, owner.starts)
	}
	writeOwnerError(t, root, "the owner could not read its batch store\n")
	clock = clock.Add(90 * time.Second)
	if line := keeper.Step(); !strings.Contains(line, "the owner could not read its batch store") {
		t.Fatalf("death line %q lacks the owner's last error", line)
	}
	if line := GiveUpLine(resolved(root), KeeperState{Failures: 5, Restarts: 4, Since: laneNow.Format(time.RFC3339), GaveUp: laneNow.Format(time.RFC3339)}); !strings.Contains(line, "the owner could not read its batch store") {
		t.Fatalf("give-up line %q lacks the owner's last error", line)
	}
}
