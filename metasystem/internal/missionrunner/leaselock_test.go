package missionrunner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
)

// The mission lease's acquire and stale-cleanup exclude each other
// under ONE flock: a cleanup that classifies a half-published claim
// as stale can mint two runners for one mission.
func TestMissionLeaseAcquireAndCleanupShareOneLock(t *testing.T) {
	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0.2")
	engine := &Engine{Root: t.TempDir(), Mission: "mr-ki38"}
	dir := engine.missionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	release, err := lease.LockBounded(filepath.Join(dir, "lease.lock"), "witness")
	if err != nil {
		t.Fatal(err)
	}
	// Held elsewhere: BOTH sides refuse within the bound instead of
	// proceeding into the other's critical section.
	if _, err := engine.acquireLease("witness-tag"); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("acquire under a held lock refuses by name: %v", err)
	}
	if err := engine.cleanupStaleLease(); err == nil || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("cleanup under a held lock refuses by name: %v", err)
	}
	release()
	// Released: acquisition proceeds and publishes the full record.
	if _, err := engine.acquireLease("witness-tag"); err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "lease.d", "owner.json")); err != nil {
		t.Fatal("the claim published its owner record")
	}
	engine.releaseLease()
	// And cleanup on the emptied world is a clean no-op.
	if err := engine.cleanupStaleLease(); err != nil {
		t.Fatalf("cleanup after release: %v", err)
	}
}
