//go:build darwin

package identity

import (
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

// AllPids must find at least this process and the machine's process set —
// and it must include our own pid, proving the offset/stride are right.
func TestAllPidsFindsSelf(t *testing.T) {
	pids, err := AllPids()
	if err != nil {
		if errors.Is(err, syscall.EPERM) {
			t.Skipf("process enumeration is restricted: %v", err)
		}
		t.Fatal(err)
	}
	if len(pids) < 10 {
		t.Fatalf("implausibly few processes: %d", len(pids))
	}
	self := int64(os.Getpid())
	found := false
	for _, pid := range pids {
		if pid == self {
			found = true
		}
	}
	if !found {
		t.Fatalf("AllPids did not include our own pid %d (offset/stride wrong?)", self)
	}
	// Every returned pid must be probeable as a real identity (a garbage
	// offset would yield junk pids that don't resolve).
	live := 0
	for _, pid := range pids {
		if _, state, _ := (KernelProber{}).Probe(pid); state == Alive {
			live++
		}
	}
	if live < len(pids)/2 {
		t.Fatalf("only %d/%d pids are live — offset likely wrong", live, len(pids))
	}
}

func TestDecodeAllPidsRejectsDriftAndFiltersNonProcesses(t *testing.T) {
	if pids, err := decodeAllPids(nil); err != nil || pids != nil {
		t.Fatalf("empty process table: pids=%v err=%v", pids, err)
	}
	if _, err := decodeAllPids([]byte{1}); err == nil {
		t.Fatal("misaligned process table was accepted")
	}
	raw := make([]byte, 2*648)
	binary.LittleEndian.PutUint32(raw[40:], uint32(41))
	binary.LittleEndian.PutUint32(raw[648+40:], uint32(0))
	pids, err := decodeAllPids(raw)
	if err != nil || len(pids) != 1 || pids[0] != 41 {
		t.Fatalf("decoded process table: pids=%v err=%v", pids, err)
	}
}

func TestProcessCwdSelf(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := ProcessCwd(int64(os.Getpid()))
	if !ok {
		t.Fatal("could not read our own cwd via proc_info")
	}
	// Resolve both through symlinks for a stable compare (/var vs /private/var).
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(wd)
	if gotResolved != wantResolved {
		t.Fatalf("ProcessCwd = %q, want %q", gotResolved, wantResolved)
	}
}

// ParentPid is validated against two independent ground truths: our own
// parent (os.Getppid) and a child we spawn ourselves (whose parent is us).
// A wrong pbi_ppid offset fails both, so the ABI is pinned, not assumed.
func TestParentPidMatchesGroundTruth(t *testing.T) {
	got, ok := ParentPid(int64(os.Getpid()))
	if !ok {
		t.Fatal("ParentPid could not read our own parent")
	}
	if want := int64(os.Getppid()); got != want {
		t.Fatalf("ParentPid(self) = %d, want os.Getppid() = %d (offset wrong?)", got, want)
	}

	child := exec.Command("/bin/sleep", "30")
	if err := child.Start(); err != nil {
		t.Fatalf("could not spawn a child to test against: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()
	got, ok = ParentPid(int64(child.Process.Pid))
	if !ok {
		t.Fatal("ParentPid could not read our spawned child's parent")
	}
	if want := int64(os.Getpid()); got != want {
		t.Fatalf("ParentPid(child) = %d, want our pid %d", got, want)
	}
}

// A pid that does not exist reports no parent.
func TestParentPidDeadProcess(t *testing.T) {
	if _, ok := ParentPid(2147483645); ok {
		t.Fatal("ParentPid claimed a parent for a nonexistent pid")
	}
}
