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

// An owner with a native process reference is held until that process exits.
// Owners recorded with only a pid and tag use the command-line probe.

// ErrOwnerLockBusy and ErrOwnerLockNotOwner are the distinguished refusals.
var (
	ErrOwnerLockBusy     = fmt.Errorf("owner lock is busy")
	ErrOwnerLockNotOwner = fmt.Errorf("owner lock is held by someone else")
)

// ownerLockCodec keeps this lock's historical owner.json bytes:
// {"acquiredAt":...,"instanceTag":...,"pid":...}.
type ownerLockCodec struct{ now func() time.Time }

func (c ownerLockCodec) Encode(self lock.Identity) ([]byte, error) {
	fields := map[string]any{
		"pid": self.Pid, "instanceTag": self.Tag,
		"acquiredAt": c.now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	if self.Process != (identity.Ref{}) {
		fields["process"] = self.Process
	}
	payload, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	return append(payload, '\n'), nil
}

func (ownerLockCodec) Decode(data []byte) (lock.Identity, error) {
	var value struct {
		Pid         *int64        `json:"pid"`
		InstanceTag string        `json:"instanceTag"`
		Process     *identity.Ref `json:"process"`
	}
	if err := json.Unmarshal(data, &value); err != nil {
		return lock.Identity{}, err
	}
	if value.Pid == nil {
		return lock.Identity{}, fmt.Errorf("owner file names no pid")
	}
	holder := lock.Identity{Pid: *value.Pid, Tag: value.InstanceTag}
	if value.Process != nil {
		ref := *value.Process
		if ref.Pid != holder.Pid || (ref.Mode() != identity.CompareLinuxTicksBootID && ref.Mode() != identity.CompareDarwinMicroseconds) {
			return lock.Identity{}, errors.New("owner file has an invalid process reference")
		}
		holder.Process = ref
	}
	return holder, nil
}

// KernelOwnerLockProber preserves the existence check for pid-and-tag owners.
// Exact owners use KernelProber.ReadStart through identity.LiveRef.
type KernelOwnerLockProber struct{ identity.KernelProber }

func (p KernelOwnerLockProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	switch unix.Kill(int(pid), 0) {
	case unix.ESRCH:
		return identity.Exact{}, identity.Dead, nil
	case unix.EPERM:
		return identity.Exact{Pid: pid}, identity.Alive, nil
	}
	exact, state, err := p.KernelProber.Probe(pid)
	if state != identity.Alive {
		// Only ESRCH on the existence check proves a legacy owner absent.
		return identity.Exact{}, identity.Unknown, err
	}
	return exact, state, err
}

func (opts OwnerLockOptions) probe(holder lock.Identity) lock.Liveness {
	prober := opts.Prober
	if prober == nil {
		prober = KernelOwnerLockProber{}
	}
	if holder.Process != (identity.Ref{}) {
		switch identity.LiveRef(prober, holder.Process) {
		case identity.Alive:
			return lock.Alive
		case identity.Dead:
			return lock.Dead
		default:
			return lock.Unknown
		}
	}
	exact, state, err := prober.Probe(holder.Pid)
	if err == nil && state == identity.Dead {
		return lock.Dead
	}
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
	_, err := OwnerLockClaimWithHolder(directory, pid, tag)
	return err
}

// OwnerLockClaimWithHolder also returns the holder observed on a busy claim.
// An unreadable owner record leaves the identity empty.
func OwnerLockClaimWithHolder(directory string, pid int64, tag string) (lock.Identity, error) {
	return OwnerLockClaimWith(directory, lock.Identity{Pid: pid, Tag: tag}, OwnerLockOptions{})
}

// OwnerLockOptions defaults to the wall clock and the kernel process reader.
type OwnerLockOptions struct {
	Now    func() time.Time
	Sleep  func(time.Duration)
	Prober identity.Prober
}

// OwnerLockClaimWith carries the claimant's clock and kernel reader through
// publication and every inspection, including fenced takeover.
func OwnerLockClaimWith(directory string, self lock.Identity, opts OwnerLockOptions) (lock.Identity, error) {
	if opts.Now == nil {
		opts.Now = time.Now
	}
	_, err := lock.Acquire(directory, self, lock.Options{
		Now: opts.Now, Sleep: opts.Sleep,
		Probe: opts.probe,
		Codec: ownerLockCodec{now: opts.Now},
	})
	var holder *lock.HolderError
	if errors.As(err, &holder) {
		return holder.Holder, ErrOwnerLockBusy
	}
	return lock.Identity{}, err
}

// OwnerLockRelease frees the lock only when (pid, tag) still owns it. An
// absent lock releases cleanly; someone else's lock is refused.
func OwnerLockRelease(directory string, pid int64, tag string) error {
	err := lock.ReleaseNamed(directory, lock.Identity{Pid: pid, Tag: tag}, ownerLockCodec{})
	if err == nil {
		return nil
	}
	if errors.As(err, new(*lock.NotHolderError)) {
		return ErrOwnerLockNotOwner
	}
	// Absent or already replaced-and-gone: nothing of ours remains.
	return nil
}
