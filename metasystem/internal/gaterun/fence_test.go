package gaterun

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// A marker registered by the asking process itself, or by any of its
// ancestors, must not block the fence: that is the suite consulting the
// fence about its own run.
func TestFenceExemptsOwnChain(t *testing.T) {
	root := t.TempDir()
	self := int64(os.Getpid())
	if _, err := Register(root, self, "self-gate"); err != nil {
		t.Fatalf("register self: %v", err)
	}
	if _, err := Register(root, int64(os.Getppid()), "ancestor-gate"); err != nil {
		t.Fatalf("register ancestor: %v", err)
	}
	if holders := Fence(root, self); len(holders) != 0 {
		t.Fatalf("own chain blocked the fence: %+v", holders)
	}
}

// A live foreign process's marker blocks the fence and names its gate.
func TestFenceBlocksForeignLiveRun(t *testing.T) {
	root := t.TempDir()
	foreign := exec.Command("sleep", "60")
	if err := foreign.Start(); err != nil {
		t.Fatalf("start foreign process: %v", err)
	}
	defer func() {
		_ = foreign.Process.Kill()
		_, _ = foreign.Process.Wait()
	}()
	pid := int64(foreign.Process.Pid)
	if _, err := Register(root, pid, "foreign-gate"); err != nil {
		t.Fatalf("register foreign: %v", err)
	}
	holders := Fence(root, int64(os.Getpid()))
	if len(holders) != 1 || holders[0].Pid != pid || holders[0].Gate != "foreign-gate" {
		t.Fatalf("foreign live run did not block as itself: %+v", holders)
	}
}

// Dead and unparsable markers are pruned, exactly like Running prunes them:
// a marker only counts while its exact process is alive.
func TestFencePrunesDeadAndUnparsableMarkers(t *testing.T) {
	root := t.TempDir()
	foreign := exec.Command("sleep", "60")
	if err := foreign.Start(); err != nil {
		t.Fatalf("start foreign process: %v", err)
	}
	pid := int64(foreign.Process.Pid)
	if _, err := Register(root, pid, "dying-gate"); err != nil {
		t.Fatalf("register foreign: %v", err)
	}
	if err := foreign.Process.Kill(); err != nil {
		t.Fatalf("kill foreign process: %v", err)
	}
	if _, err := foreign.Process.Wait(); err != nil {
		t.Fatalf("wait foreign process: %v", err)
	}
	garbage := filepath.Join(markerDir(root), "garbage.json")
	if err := os.WriteFile(garbage, []byte("not json\n"), 0o644); err != nil {
		t.Fatalf("write garbage marker: %v", err)
	}
	if holders := Fence(root, int64(os.Getpid())); len(holders) != 0 {
		t.Fatalf("dead or garbage marker blocked the fence: %+v", holders)
	}
	if _, err := os.Stat(garbage); !os.IsNotExist(err) {
		t.Fatalf("garbage marker was not pruned")
	}
	markers, _ := filepath.Glob(filepath.Join(markerDir(root), "*.json"))
	if len(markers) != 0 {
		t.Fatalf("dead marker was not pruned: %v", markers)
	}
}
