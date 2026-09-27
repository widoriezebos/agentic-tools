package hooks

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

// The bootstrap fence names its holder's pid: a live holder keeps it, a dead
// one leaves it stale for the next claimant, and a fence of any other shape
// is refused rather than guessed at.
func TestBootstrapFenceClaim(t *testing.T) {
	alive := func(pid int) bool { return pid == os.Getpid() }
	holder := func(t *testing.T, installation string) string {
		t.Helper()
		target, err := os.Readlink(filepath.Join(installation, filepath.FromSlash(BootstrapFencePath)))
		if err != nil {
			t.Fatal(err)
		}
		return target
	}
	installation := t.TempDir()
	if claimed, err := ClaimBootstrapFence(installation, os.Getpid(), alive); !claimed || err != nil {
		t.Fatalf("fresh claim = %v %v", claimed, err)
	}
	if claimed, err := ClaimBootstrapFence(installation, 7, alive); claimed || err != nil || !BootstrapFenceHeld(installation, alive) {
		t.Fatalf("claim over a live holder = %v %v", claimed, err)
	}
	finished := exec.Command("true")
	if err := finished.Run(); err != nil {
		t.Fatal(err)
	}
	if err := PointBootstrapFence(installation, finished.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if BootstrapFenceHeld(installation, alive) {
		t.Fatal("a dead holder still holds the fence")
	}
	if claimed, err := ClaimBootstrapFence(installation, os.Getpid(), alive); !claimed || err != nil || holder(t, installation) != strconv.Itoa(os.Getpid()) {
		t.Fatalf("claim over a dead holder = %v %v holder %s", claimed, err, holder(t, installation))
	}
	entries, _ := os.ReadDir(filepath.Dir(filepath.Join(installation, filepath.FromSlash(BootstrapFencePath))))
	if len(entries) != 1 {
		t.Fatalf("the fence left staging links: %v", entries)
	}

	directory := t.TempDir()
	if err := os.MkdirAll(filepath.Join(directory, filepath.FromSlash(BootstrapFencePath)), 0o755); err != nil {
		t.Fatal(err)
	}
	if claimed, err := ClaimBootstrapFence(directory, os.Getpid(), alive); claimed || err == nil {
		t.Fatalf("claim over a directory = %v %v", claimed, err)
	}
}
