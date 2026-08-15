package missionrunner

import (
	"os"
	"path/filepath"
	"testing"
)

// GOAL-22: runner-record publication is lease-serialized. A acquires the
// lease and publishes; B contends for the same mission, loses, and is
// proven to NEITHER publish NOR finalize — A's record survives
// byte-identical through B's whole attempt.
func TestOverlappingResumeKeepsWinnerRecord(t *testing.T) {
	root := t.TempDir()
	winner := &Engine{Root: root, Mission: "mr-overlap"}
	if err := os.MkdirAll(winner.missionDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	// A: acquire the lease and publish, exactly as internalRun's winner
	// path does (this process IS the winner's process, so the record
	// carries live identity).
	if _, err := winner.acquireLease("winner-tag"); err != nil {
		t.Fatalf("winner could not acquire the lease: %v", err)
	}
	pid := os.Getpid()
	started, err := processStartedAt(pid)
	if err != nil {
		t.Fatal(err)
	}
	recordPath, _, _ := winner.runnerPaths()
	if err := os.MkdirAll(filepath.Dir(recordPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteJSON(recordPath, winner.runnerRecord(pid, pid, started, "winner-tag")); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}

	// B: full internalRun contention for the same mission. It must lose at
	// lease acquisition and touch nothing.
	loser := &Engine{Root: root, Mission: "mr-overlap"}
	signal := filepath.Join(t.TempDir(), "start.signal")
	if code := loser.internalRun("resume", "loser-tag", signal); code == 0 {
		t.Fatal("the losing contender reported success")
	}

	after, err := os.ReadFile(recordPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatalf("the loser touched the winner's record:\nbefore: %s\nafter:  %s", before, after)
	}
	// The winner still owns the lease.
	if !pathExists(filepath.Join(winner.missionDir(), "lease.d")) {
		t.Fatal("the loser released the winner's lease")
	}
}
