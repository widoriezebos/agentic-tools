package dispatch

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// Direct tests for the owner-lock protocol: claim, re-claim
// against every holder class, and identity-checked release — in-process,
// not through the suite's supervision fixtures.

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

func TestOwnerCodecDegrades(t *testing.T) {
	codec := ownerLockCodec{}
	if _, err := codec.Decode([]byte("{broken")); err == nil {
		t.Fatal("malformed identity must refuse to decode")
	}
	if _, err := codec.Decode([]byte(`{"instanceTag":"t"}`)); err == nil {
		t.Fatal("identity without a pid must refuse to decode")
	}
	holder, err := codec.Decode([]byte(`{"pid":42,"instanceTag":"t","acquiredAt":"x"}`))
	if err != nil || holder.Pid != 42 || holder.Tag != "t" {
		t.Fatalf("round trip lost fields: %+v %v", holder, err)
	}
}

// codex-2 (the review's owner-lock finding): a live holder whose argv is
// unreadable is BUSY, never stale — absence of evidence must not permit
// takeover. With the binding over internal/lock, "stale" is the probe
// answering Dead for a live pid; whatever branch this host takes for
// pid 1 (EPERM at kill, or Alive with unreadable argv), the probe must
// never answer Dead.
func TestHolderProbeUnreadableArgvIsNeverDead(t *testing.T) {
	holder := lock.Identity{Pid: 1, Tag: "tag-no-init-carries"}
	if state := ownerHolderProbe(holder); state == lock.Dead {
		t.Fatal("a holder with unverifiable argv was ruled dead (takeover permitted)")
	}
}
