package main

import (
	"os"
	"path/filepath"
	"testing"
)

// codex-4 (the review): the arming blocker scan fails CLOSED. A record it
// cannot read might carry the very reservation that blocks the ceiling, so
// unreadable and malformed inputs refuse arming; only genuinely absent
// files skip.
func TestBlockingReservedCapFailsClosed(t *testing.T) {
	agents := t.TempDir()
	jobs := filepath.Join(agents, "jobs")
	os.MkdirAll(jobs, 0o755)

	// A clean scan with a live blocking reservation still reports it.
	os.WriteFile(filepath.Join(jobs, "j1.json"),
		[]byte(`{"jobId":"j1","status":"running","capMin":120}`), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code != 0 {
		t.Fatalf("clean scan refused: %d", code)
	}

	// A corrupt job record refuses arming.
	os.WriteFile(filepath.Join(jobs, "j2.json"), []byte("{broken"), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("a corrupt job record scanned clean (fail-open)")
	}
	os.Remove(filepath.Join(jobs, "j2.json"))

	// A malformed capMin refuses arming.
	os.WriteFile(filepath.Join(jobs, "j3.json"),
		[]byte(`{"jobId":"j3","status":"running","capMin":"lots"}`), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("a malformed capMin scanned clean")
	}
	os.Remove(filepath.Join(jobs, "j3.json"))

	// Corrupt fence counters refuse arming.
	missions := filepath.Join(agents, "missions", "m1")
	os.MkdirAll(missions, 0o755)
	os.WriteFile(filepath.Join(missions, "fences.json"), []byte("{broken"), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("corrupt fences scanned clean")
	}

	// A malformed reservation entry refuses arming.
	os.WriteFile(filepath.Join(missions, "fences.json"),
		[]byte(`{"reservations":{"jx":"not-an-object"}}`), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("a malformed reservation scanned clean")
	}
}
