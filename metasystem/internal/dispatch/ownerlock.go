package dispatch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The dispatch owner lock is a rename-born directory lock: a claim stages a
// directory containing owner.json and renames it into place, so the lock is
// born owning and no window exists between taking it and naming its holder.
// A release renames the directory aside before removing it, so a racing
// claimant never sees a half-deleted lock. Takeover is legal only when the
// recorded holder is provably dead or provably stale (its pid lives but no
// longer carries the recorded tag); a live or unreadable holder keeps the
// lock.

// ErrOwnerLockBusy and ErrOwnerLockNotOwner are the distinguished refusals.
var (
	ErrOwnerLockBusy     = fmt.Errorf("owner lock is busy")
	ErrOwnerLockNotOwner = fmt.Errorf("owner lock is held by someone else")
)

type ownerIdentity struct {
	pid int64
	tag string
}

func readOwnerIdentity(ownerPath string) *ownerIdentity {
	data, err := os.ReadFile(ownerPath)
	if err != nil {
		return nil
	}
	var value struct {
		Pid         *int64 `json:"pid"`
		InstanceTag string `json:"instanceTag"`
	}
	if json.Unmarshal(data, &value) != nil || value.Pid == nil {
		return nil
	}
	return &ownerIdentity{pid: *value.Pid, tag: value.InstanceTag}
}

// holderState classifies the recorded holder: dead (pid gone), live (running
// and carrying the tag, or unreadable-but-present), stale (running without
// the tag), or unknown (existence provable but identity unreadable).
func holderState(holder *ownerIdentity) string {
	switch unix.Kill(int(holder.pid), 0) {
	case unix.ESRCH:
		return "dead"
	case unix.EPERM:
		return "live"
	}
	exact, state, err := identity.KernelProber{}.Probe(holder.pid)
	if err != nil || state != identity.Alive {
		return "unknown"
	}
	command := strings.Join(exact.Argv, " ")
	if holder.tag != "" && strings.Contains(command, holder.tag) {
		return "live"
	}
	return "stale"
}

// OwnerLockClaim takes the lock for (pid, tag). It returns nil when claimed,
// ErrOwnerLockBusy when a live or unreadable holder keeps it.
func OwnerLockClaim(directory string, pid int64, tag string) error {
	parent := filepath.Dir(directory)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	payload, err := json.Marshal(map[string]any{
		"pid": pid, "instanceTag": tag,
		"acquiredAt": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	for attempt := 0; attempt < 3; attempt++ {
		staging, err := os.MkdirTemp(parent, filepath.Base(directory)+".claim.")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(staging, "owner.json"), payload, 0o644); err != nil {
			_ = os.RemoveAll(staging)
			return err
		}
		if err := os.Rename(staging, directory); err == nil {
			return nil
		}
		_ = os.RemoveAll(staging)
		holder := readOwnerIdentity(filepath.Join(directory, "owner.json"))
		if holder == nil {
			return ErrOwnerLockBusy
		}
		switch holderState(holder) {
		case "live", "unknown":
			return ErrOwnerLockBusy
		}
		husk := filepath.Join(parent, fmt.Sprintf("%s.dead.%d.%d", filepath.Base(directory), pid, attempt))
		if err := os.Rename(directory, husk); err != nil {
			continue // lost the takeover race; try again
		}
		_ = os.RemoveAll(husk)
	}
	return ErrOwnerLockBusy
}

// OwnerLockRelease frees the lock only when (pid, tag) still owns it. An
// absent lock releases cleanly; someone else's lock is refused.
func OwnerLockRelease(directory string, pid int64, tag string) error {
	holder := readOwnerIdentity(filepath.Join(directory, "owner.json"))
	if holder == nil {
		return nil
	}
	if holder.pid != pid || holder.tag != tag {
		return ErrOwnerLockNotOwner
	}
	retiring := filepath.Join(filepath.Dir(directory), fmt.Sprintf("%s.retiring.%d", filepath.Base(directory), pid))
	if err := os.Rename(directory, retiring); err != nil {
		return nil // already gone or replaced; nothing of ours remains
	}
	_ = os.RemoveAll(retiring)
	return nil
}
