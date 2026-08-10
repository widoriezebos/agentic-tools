package lease

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func kernelStartSecond(t *testing.T, pid int64) int64 {
	t.Helper()
	exact, state, err := identity.KernelProber{}.Probe(pid)
	if err != nil || state != identity.Alive {
		t.Fatalf("cannot probe own identity: %v", err)
	}
	return exact.StartedAt.Unix()
}

func writeReclaimLease(t *testing.T, dir, body string) {
	t.Helper()
	mains := filepath.Join(dir, "artifacts", "agents", "mains")
	if err := os.MkdirAll(mains, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mains, "worktree-lease.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReclaimRefusesForeignContent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Reclaim(dir)
	if err == nil || !strings.Contains(err.Error(), "foreign content") {
		t.Fatalf("want foreign-content refusal, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "keep.txt")); statErr != nil {
		t.Fatalf("refusal must not delete anything: %v", statErr)
	}
}

func TestReclaimRefusesLiveHolder(t *testing.T) {
	dir := t.TempDir()
	self := os.Getpid()
	started := kernelStartSecond(t, int64(self))
	writeReclaimLease(t, dir, `{"pid":`+itoa(int64(self))+`,"pidStartedAt":`+itoa(started)+`}`)
	err := Reclaim(dir)
	if err == nil || !strings.Contains(err.Error(), "LIVE") {
		t.Fatalf("want live-holder refusal, got %v", err)
	}
}

func TestReclaimDeletesDeadResidue(t *testing.T) {
	dir := t.TempDir()
	writeReclaimLease(t, dir, `{"pid":`+itoa(int64(os.Getpid()))+`,"pidStartedAt":1}`)
	if err := Reclaim(dir); err != nil {
		t.Fatalf("dead residue must reclaim: %v", err)
	}
	if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
		t.Fatalf("target must be gone, stat said %v", statErr)
	}
}

func TestReclaimRefusesShallowAndRelativePaths(t *testing.T) {
	for _, target := range []string{"", "relative/path", "/", "/tmp"} {
		if err := Reclaim(target); err == nil {
			t.Fatalf("shallow or relative target %q must refuse", target)
		}
	}
}
