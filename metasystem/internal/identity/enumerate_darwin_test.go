//go:build darwin

package identity

import (
	"os"
	"path/filepath"
	"testing"
)

// AllPids must find at least this process and the machine's process set —
// and it must include our own pid, proving the offset/stride are right.
func TestAllPidsFindsSelf(t *testing.T) {
	pids, err := AllPids()
	if err != nil {
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
