package goalrevision

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAcquireReturnsHandleAndReleaseEndsOnlyThatAcquisition(t *testing.T) {
	root := t.TempDir()
	held, err := Acquire(root, "goal-a", 2, "first-holder")
	if err != nil || held == nil {
		t.Fatalf("acquire = %#v, %v; want a held handle", held, err)
	}
	path, err := Path(root, "goal-a", 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, "owner.json")); err != nil {
		t.Fatalf("acquired lock has no readable owner metadata: %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("release left the lock path behind: %v", err)
	}
	if err := held.Release(); err != nil {
		t.Fatalf("second release was not harmless: %v", err)
	}
	var absent *Held
	if err := absent.Release(); err != nil {
		t.Fatalf("nil release was not harmless: %v", err)
	}
}

func TestMatchingOwnerMetadataDoesNotGrantAnotherHandle(t *testing.T) {
	root := t.TempDir()
	first, err := Acquire(root, "goal-a", 3, "same-tag")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Release() })

	second, err := Acquire(root, "goal-a", 3, "same-tag")
	if err == nil || second != nil {
		t.Fatalf("matching process metadata acquired a second handle: handle=%#v err=%v", second, err)
	}
	message := err.Error()
	if !strings.Contains(message, "LOCK_BUSY rank=goal-revision key=goal-a/r3") ||
		!strings.Contains(message, "tag=same-tag") {
		t.Fatalf("busy refusal did not name the local holder: %v", err)
	}
}

func TestAcquireTakesOverLockOrphanedByCrashedHolder(t *testing.T) {
	if os.Getenv("GOALREVISION_CRASH_HOLDER") == "1" {
		if _, err := Acquire(os.Getenv("GOALREVISION_ROOT"), "goal-a", 4, "crash-holder"); err != nil {
			os.Exit(2)
		}
		os.Exit(0)
	}

	root := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestAcquireTakesOverLockOrphanedByCrashedHolder$")
	command.Env = append(os.Environ(), "GOALREVISION_CRASH_HOLDER=1", "GOALREVISION_ROOT="+root)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("crash-holder subprocess: %v: %s", err, output)
	}

	held, err := Acquire(root, "goal-a", 4, "successor")
	if err != nil || held == nil {
		t.Fatalf("take over crashed holder: handle=%#v err=%v", held, err)
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
}

func TestPathRefusesInvalidCoordinates(t *testing.T) {
	for _, test := range []struct {
		goal     string
		revision uint64
	}{
		{goal: "Uppercase", revision: 1},
		{goal: "goal-a", revision: 0},
	} {
		if _, err := Path(t.TempDir(), test.goal, test.revision); err == nil {
			t.Fatalf("Path(%q, %d) accepted invalid coordinates", test.goal, test.revision)
		}
	}
}
