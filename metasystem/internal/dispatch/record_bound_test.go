package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// codex-5: dispatch's record lock is bounded like the lease lock — a
// wedged holder produces a refusal naming the job, not a hang under the
// global lease lock.
func TestWithRecordLockIsBounded(t *testing.T) {
	t.Setenv("METASYSTEM_LEASE_LOCK_WAIT_SEC", "0.2")
	root := t.TempDir()
	locks := filepath.Join(root, "artifacts", "agents", "record-locks")
	if err := os.MkdirAll(locks, 0o755); err != nil {
		t.Fatal(err)
	}
	holder, err := os.OpenFile(filepath.Join(locks, "wedged.lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	defer holder.Close()
	if err := unix.Flock(int(holder.Fd()), unix.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = withRecordLock(root, "wedged", func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "wedged") || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("a held record lock must refuse by name: %v", err)
	}
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Fatalf("the bound did not release the caller: %v", elapsed)
	}
}

// D12: the job-record writer derives its durable anchor from the path —
// the checkout root above artifacts/ — and transient paths anchor nowhere.
func TestArtifactsAnchorDerivation(t *testing.T) {
	if got := artifactsAnchor("/repo/x/artifacts/agents/jobs/j.json"); got != "/repo/x" {
		t.Fatalf("anchor = %q, want /repo/x", got)
	}
	if got := artifactsAnchor("/tmp/staging/file.json"); got != "" {
		t.Fatalf("non-artifacts path must not anchor: %q", got)
	}
	if got := artifactsAnchor("/a/artifacts/agents/x/artifacts/agents/jobs/j.json"); got != "/a/artifacts/agents/x" {
		t.Fatalf("nested artifacts anchors at the DEEPEST checkout: %q", got)
	}
}
