package lock

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Error-surface rows: refusal messages are part of the contract (D-5
// loudness) and the defensive branches must hold their shape.

func TestHolderErrorNamesStateAndIdentity(t *testing.T) {
	alive := (&HolderError{Path: "/x", Holder: Identity{Pid: 7, PidStartedAt: 9}, State: Alive}).Error()
	if !strings.Contains(alive, "pid 7") || !strings.Contains(alive, "alive") {
		t.Fatalf("alive holder error underspecified: %q", alive)
	}
	unknown := (&HolderError{Path: "/x", State: Unknown}).Error()
	if !strings.Contains(unknown, "uninspectable is alive") {
		t.Fatalf("unknown holder error must state the rule: %q", unknown)
	}
}

func TestAcquireSurfacesUnwritableParent(t *testing.T) {
	directory := t.TempDir()
	if err := os.Chmod(directory, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(directory, 0o755)
	_, err := Acquire(filepath.Join(directory, "sub", "lock.d"), Identity{Pid: 1, PidStartedAt: 1},
		Options{Wait: time.Second, Probe: alive})
	if err == nil || !strings.Contains(err.Error(), "lock:") {
		t.Fatalf("unwritable parent not surfaced: %v", err)
	}
}

func TestReleaseSurfacesVanishedLock(t *testing.T) {
	path := lockPath(t)
	held, err := Acquire(path, Identity{Pid: 1, PidStartedAt: 1}, Options{Wait: time.Second, Probe: alive})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatal(err)
	}
	if err := held.Release(); err == nil {
		t.Fatal("releasing a vanished lock must fail, not fabricate success")
	}
}

func TestGarbageOwnerFileContentIsUnknownHolder(t *testing.T) {
	path := lockPath(t)
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "owner.json"), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Acquire(path, Identity{Pid: 2, PidStartedAt: 2},
		Options{Wait: 120 * time.Millisecond, Poll: 10 * time.Millisecond, Probe: dead})
	if err == nil {
		t.Fatal("an unparseable owner file must never be taken over")
	}
}

// removeIfHolder re-verifies INSIDE the fence: a lock that changed
// hands between the probe and the fence is left alone.
func TestRemoveIfHolderLeavesSuccessorsAlone(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 5, PidStartedAt: 5}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	// The caller proved pid 4 dead — but the lock names pid 5 now.
	if err := removeIfHolderWith(path, Identity{Pid: 4, PidStartedAt: 4}, dead, identityJSON{}); err != nil {
		t.Fatal(err)
	}
	if holder, err := Holder(path); err != nil || holder.Pid != 5 {
		t.Fatalf("a successor's lock was touched: %+v %v", holder, err)
	}
}

func TestRemoveIfHolderVanishedLockIsFine(t *testing.T) {
	if err := removeIfHolderWith(lockPath(t), Identity{Pid: 4, PidStartedAt: 4}, dead, identityJSON{}); err != nil {
		t.Fatalf("a vanished lock is an already-done takeover: %v", err)
	}
}

func TestRemoveIfOwnerlessRespectsOwnedLocks(t *testing.T) {
	path := lockPath(t)
	if _, err := Acquire(path, Identity{Pid: 6, PidStartedAt: 6}, Options{Wait: time.Second, Probe: alive}); err != nil {
		t.Fatal(err)
	}
	if err := removeIfOwnerless(path); err != nil {
		t.Fatal(err)
	}
	if holder, err := Holder(path); err != nil || holder.Pid != 6 {
		t.Fatal("an owned lock was removed as garbage")
	}
	// And a genuinely ownerless directory goes.
	garbage := lockPath(t)
	if err := os.MkdirAll(garbage, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := removeIfOwnerless(garbage); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(garbage); !os.IsNotExist(err) {
		t.Fatal("ownerless garbage survived the fenced removal")
	}
}
