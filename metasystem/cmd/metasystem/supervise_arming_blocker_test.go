package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The arming blocker scan fails CLOSED. A record it
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

// lease-census-7: the identity fixture must never ride into an armed fleet
// of a real checkout. This verb runs at every arming gate, so it is the
// fence.
func TestBlockingReservedCapRefusesLeakedIdentityFixture(t *testing.T) {
	root := t.TempDir()
	agents := filepath.Join(root, "artifacts", "agents")
	if err := os.MkdirAll(agents, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FAKE_PROCESS_IDENTITY_FILE", filepath.Join(root, "identities.json"))

	// No conf (not fake): arming refuses.
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("a leaked identity fixture must refuse arming without a fake conf")
	}
	// Real runtimes: still refuses.
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=claude,codex\n"), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code == 0 {
		t.Fatal("a leaked identity fixture must refuse arming in a real-runtime checkout")
	}
	// Fake checkout: the fixture is sanctioned.
	os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644)
	if code := runSuperviseBlockingReservedCap([]string{"--agents", agents, "--ceiling", "60"}); code != 0 {
		t.Fatal("a fake-runtime checkout must still arm with the fixture installed")
	}
}
