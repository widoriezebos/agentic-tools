package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// Direct tests for the owner-lock protocol (Phase 6): claim, re-claim
// against every holder class, and identity-checked release. Previously
// exercised only through the suite's supervision fixtures.

func TestOwnerLockClaimAndRelease(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "owner.lock.d")
	self := int64(os.Getpid())
	if err := OwnerLockClaim(lock, self, "tag-a"); err != nil {
		t.Fatalf("fresh claim: %v", err)
	}
	// Our own live claim blocks a second claimant: we are a live holder
	// whose argv does not carry tag-a, which is STALE by the table — but a
	// second claim by a DIFFERENT identity must take over a stale holder.
	// First: release by the wrong identity is refused.
	if err := OwnerLockRelease(lock, self, "tag-b"); !errors.Is(err, ErrOwnerLockNotOwner) {
		t.Fatalf("foreign release accepted: %v", err)
	}
	// Release by the right identity frees it; a second release is a no-op.
	if err := OwnerLockRelease(lock, self, "tag-a"); err != nil {
		t.Fatalf("own release refused: %v", err)
	}
	if err := OwnerLockRelease(lock, self, "tag-a"); err != nil {
		t.Fatalf("releasing an absent lock must be clean: %v", err)
	}
	if _, err := os.Stat(lock); !os.IsNotExist(err) {
		t.Fatal("the lock directory survived release")
	}
}

func TestOwnerLockTakesOverADeadHolder(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "owner.lock.d")
	// A child that exits immediately is a provably dead holder.
	child := exec.Command("true")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	deadPid := int64(child.Process.Pid)
	child.Wait()
	if err := OwnerLockClaim(lock, deadPid, "dead-tag"); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	if err := OwnerLockClaim(lock, int64(os.Getpid()), "tag-new"); err != nil {
		t.Fatalf("takeover of a dead holder refused: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(lock, "owner.json"))
	if !strings.Contains(string(data), "tag-new") {
		t.Fatalf("the lock does not carry the new holder: %s", data)
	}
}

func TestOwnerLockKeepsALiveHolder(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "owner.lock.d")
	// A live child whose argv carries the recorded tag is a LIVE holder.
	tag := "ol-live-91b2"
	holder := exec.Command("bash", "-c", "exec -a sleep-"+tag+" sleep 20")
	holder.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := holder.Start(); err != nil {
		t.Fatal(err)
	}
	defer holder.Process.Kill()
	go holder.Wait()
	if err := OwnerLockClaim(lock, int64(holder.Process.Pid), tag); err != nil {
		t.Fatalf("seed claim: %v", err)
	}
	err := OwnerLockClaim(lock, int64(os.Getpid()), "tag-thief")
	if !errors.Is(err, ErrOwnerLockBusy) {
		t.Fatalf("a live holder was not kept: %v", err)
	}
}

func TestReadOwnerIdentityDegrades(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "owner.json")
	if got := readOwnerIdentity(path); got != nil {
		t.Fatal("absent identity must read nil")
	}
	os.WriteFile(path, []byte("{broken"), 0o644)
	if got := readOwnerIdentity(path); got != nil {
		t.Fatal("malformed identity must read nil")
	}
	os.WriteFile(path, []byte(`{"instanceTag":"t"}`), 0o644)
	if got := readOwnerIdentity(path); got != nil {
		t.Fatal("identity without a pid must read nil")
	}
}
