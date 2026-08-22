package dispatch

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The dispatch owner lock is a THIN BINDING over internal/lock, the one
// home of the rename-born directory-lock protocol:
// this file contributes only the owner-file schema
// and the holder classification. Takeover is legal only when the recorded
// holder is provably dead or provably stale — a live pid whose READ argv
// no longer carries the recorded tag is a stranger, which the probe
// reports as Dead (the custodian rule); a live or unreadable holder keeps
// the lock.

// ErrOwnerLockBusy and ErrOwnerLockNotOwner are the distinguished refusals.
var (
	ErrOwnerLockBusy     = fmt.Errorf("owner lock is busy")
	ErrOwnerLockNotOwner = fmt.Errorf("owner lock is held by someone else")
)

// ownerLockCodec keeps this lock's historical owner.json bytes:
// {"acquiredAt":...,"instanceTag":...,"pid":...}.
type ownerLockCodec struct{}

func (ownerLockCodec) Encode(self lock.Identity) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"pid": self.Pid, "instanceTag": self.Tag,
		"acquiredAt": time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	})
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func (ownerLockCodec) Decode(data []byte) (lock.Identity, error) {
	var value struct {
		Pid         *int64 `json:"pid"`
		InstanceTag string `json:"instanceTag"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return lock.Identity{}, err
	}
	if value.Pid == nil {
		return lock.Identity{}, fmt.Errorf("owner file names no pid")
	}
	return lock.Identity{Pid: *value.Pid, Tag: value.InstanceTag}, nil
}

// ownerHolderProbe classifies the recorded holder three-way. Dead covers
// both a vanished pid and a STALE one — a successfully read argv without
// the recorded tag proves the recorded holder is gone and a stranger
// recycled its pid. An unreadable argv on a live pid is absence of
// evidence, never evidence of a stranger: the holder keeps its lock.
func ownerHolderProbe(holder lock.Identity) lock.Liveness {
	switch unix.Kill(int(holder.Pid), 0) {
	case unix.ESRCH:
		return lock.Dead
	case unix.EPERM:
		return lock.Alive
	}
	exact, state, err := identity.KernelProber{}.Probe(holder.Pid)
	if err != nil || state != identity.Alive {
		return lock.Unknown
	}
	if !exact.ArgvKnown {
		return lock.Alive
	}
	command := strings.Join(exact.Argv, " ")
	if holder.Tag != "" && strings.Contains(command, holder.Tag) {
		return lock.Alive
	}
	return lock.Dead
}

// OwnerLockClaim takes the lock for (pid, tag). It returns nil when claimed,
// ErrOwnerLockBusy when a live or unreadable holder keeps it.
func OwnerLockClaim(directory string, pid int64, tag string) error {
	self := lock.Identity{Pid: pid, Tag: tag}
	_, err := lock.Acquire(directory, self, lock.Options{
		Probe: ownerHolderProbe,
		Codec: ownerLockCodec{},
	})
	if err == nil {
		return nil
	}
	var holder *lock.HolderError
	if errors.As(err, &holder) {
		return ErrOwnerLockBusy
	}
	return err
}

// OwnerLockRelease frees the lock only when (pid, tag) still owns it. An
// absent lock releases cleanly; someone else's lock is refused.
func OwnerLockRelease(directory string, pid int64, tag string) error {
	err := lock.ReleaseNamed(directory, lock.Identity{Pid: pid, Tag: tag}, ownerLockCodec{})
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "no longer names this holder") {
		return ErrOwnerLockNotOwner
	}
	// Absent or already replaced-and-gone: nothing of ours remains.
	return nil
}
