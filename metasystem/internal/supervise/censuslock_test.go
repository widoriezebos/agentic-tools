package supervise

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// livenessProber reports a pid alive only when listed, at its listed start.
type livenessProber struct {
	alive map[int64]int64 // pid -> startedAt seconds
}

func (p livenessProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if start, ok := p.alive[pid]; ok {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(start, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

func lockOwnerPath(dir string) string {
	return filepath.Join(dir, "census-writer.d", "owner.json")
}

func seedLockOwner(t *testing.T, dir string, pid, start int64, tag string) {
	t.Helper()
	lockDir := filepath.Join(dir, "census-writer.d")
	if err := os.MkdirAll(lockDir, 0o755); err != nil {
		t.Fatal(err)
	}
	owner := censusOwner{Function: "census-writer", Pid: pid, PidStartedAt: start, InstanceTag: tag}
	encoded, _ := json.Marshal(owner)
	if err := os.WriteFile(filepath.Join(lockDir, "owner.json"), append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

// A claim on an empty directory publishes this process as owner; release frees
// it.
func TestCensusLockClaimAndRelease(t *testing.T) {
	dir := t.TempDir()
	lock := &CensusWriterLock{
		Dir:    dir,
		Self:   identity.Ref{Pid: 100, StartedAtSec: 5},
		Tag:    "watcher-a",
		Prober: livenessProber{alive: map[int64]int64{100: 5}},
	}
	if err := lock.Claim(); err != nil {
		t.Fatalf("claim on empty dir: %v", err)
	}
	owner, err := lock.readOwner()
	if err != nil {
		t.Fatalf("owner unreadable after claim: %v", err)
	}
	if owner.Pid != 100 || owner.InstanceTag != "watcher-a" {
		t.Fatalf("wrong owner after claim: %+v", owner)
	}
	lock.Release()
	if _, err := os.Stat(filepath.Join(dir, "census-writer.d")); !os.IsNotExist(err) {
		t.Fatal("release must remove the lock this process owns")
	}
}

// A claim is refused while a LIVE writer owns the lock — a second census stream
// must never start.
func TestCensusLockRefusesLiveOwner(t *testing.T) {
	dir := t.TempDir()
	seedLockOwner(t, dir, 200, 7, "watcher-incumbent")
	contender := &CensusWriterLock{
		Dir:    dir,
		Self:   identity.Ref{Pid: 300, StartedAtSec: 8},
		Tag:    "watcher-contender",
		Prober: livenessProber{alive: map[int64]int64{200: 7}}, // incumbent is live
	}
	if err := contender.Claim(); err == nil {
		t.Fatal("claim must be refused while a live writer owns the lock")
	}
	// The incumbent's ownership is untouched.
	if owner, err := contender.readOwner(); err != nil || owner.Pid != 200 {
		t.Fatalf("refused claim must leave the incumbent owner in place: %+v %v", owner, err)
	}
}

// A dead owner's husk is healed: the claim takes over.
func TestCensusLockTakesOverDeadOwner(t *testing.T) {
	dir := t.TempDir()
	seedLockOwner(t, dir, 400, 9, "watcher-crashed")
	successor := &CensusWriterLock{
		Dir:    dir,
		Self:   identity.Ref{Pid: 500, StartedAtSec: 10},
		Tag:    "watcher-successor",
		Prober: livenessProber{alive: map[int64]int64{}}, // pid 400 is dead
	}
	if err := successor.Claim(); err != nil {
		t.Fatalf("claim must take over a dead owner's husk: %v", err)
	}
	if owner, err := successor.readOwner(); err != nil || owner.Pid != 500 {
		t.Fatalf("takeover must publish the successor as owner: %+v %v", owner, err)
	}
}

// A process whose lock was taken over must not delete its successor's lock.
func TestCensusLockReleaseLeavesSuccessorAlone(t *testing.T) {
	dir := t.TempDir()
	original := &CensusWriterLock{
		Dir:    dir,
		Self:   identity.Ref{Pid: 600, StartedAtSec: 11},
		Tag:    "watcher-original",
		Prober: livenessProber{alive: map[int64]int64{600: 11}},
	}
	if err := original.Claim(); err != nil {
		t.Fatalf("initial claim: %v", err)
	}
	// A successor overwrites ownership (as a takeover would).
	seedLockOwner(t, dir, 700, 12, "watcher-successor")
	original.Release()
	data, err := os.ReadFile(lockOwnerPath(dir))
	if err != nil {
		t.Fatalf("successor's lock must survive the original's release: %v", err)
	}
	var owner censusOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		t.Fatal(err)
	}
	if owner.Pid != 700 {
		t.Fatalf("release freed the successor's lock: %+v", owner)
	}
}
