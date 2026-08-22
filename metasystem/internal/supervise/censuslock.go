package supervise

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// The census-writer lock keeps at most one component publishing the census
// verdict for a supervision directory. It is a THIN BINDING over
// internal/lock, the one home of the rename-born directory-lock
// protocol: this file contributes the owner-file
// schema, the liveness mapping, and the caller-facing refusal texts.
// Takeover requires proven death; UNKNOWN NEVER AUTHORIZES (the three-way
// rule) — a prior owner whose liveness cannot
// be proven keeps its lock exactly as a live one does.

// CensusWriterLock is one component's claim on the sole census-writer role
// for a supervision directory.
type CensusWriterLock struct {
	// Dir is the supervision directory the lock guards.
	Dir string
	// Self identifies this process: proven death of a prior owner is what
	// authorises a takeover, and identity match is what authorises a release.
	Self identity.Ref
	// Tag names this process in the published owner file.
	Tag string
	// Prober proves a prior owner's liveness three-way for takeover.
	Prober identity.Prober
}

func (l *CensusWriterLock) lockDir() string { return filepath.Join(l.Dir, "census-writer.d") }

// censusOwnerCodec keeps this lock's historical owner.json schema:
// {"function","pid","pidStartedAt","instanceTag","observedAtEpoch"}.
type censusOwnerCodec struct{}

func (censusOwnerCodec) Encode(self lock.Identity) ([]byte, error) {
	payload, err := json.Marshal(map[string]any{
		"function": "census-writer", "pid": self.Pid, "pidStartedAt": self.PidStartedAt,
		"instanceTag": self.Tag, "observedAtEpoch": time.Now().Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("census writer owner: %w", err)
	}
	return append(payload, '\n'), nil
}

func (censusOwnerCodec) Decode(data []byte) (lock.Identity, error) {
	var owner struct {
		Pid          int64  `json:"pid"`
		PidStartedAt int64  `json:"pidStartedAt"`
		InstanceTag  string `json:"instanceTag"`
	}
	if err := json.Unmarshal(data, &owner); err != nil {
		return lock.Identity{}, err
	}
	if owner.Pid < 1 {
		return lock.Identity{}, fmt.Errorf("owner file names no pid")
	}
	return lock.Identity{Pid: owner.Pid, PidStartedAt: owner.PidStartedAt, Tag: owner.InstanceTag}, nil
}

func (l *CensusWriterLock) self() lock.Identity {
	return lock.Identity{Pid: l.Self.Pid, PidStartedAt: l.Self.StartedAtSec, Tag: l.Tag}
}

// Claim takes the census-writer lock, healing a dead owner's husk by
// takeover. It returns an error when a LIVE writer already owns the lock
// (the caller must not publish a second census stream), when the prior
// owner's liveness is unprovable, or when the owner file is malformed.
func (l *CensusWriterLock) Claim() error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return fmt.Errorf("prepare supervision dir: %w", err)
	}
	probe := func(holder lock.Identity) lock.Liveness {
		switch identity.AliveRef(l.Prober, identity.Ref{Pid: holder.Pid, StartedAtSec: holder.PidStartedAt}) {
		case identity.Alive:
			return lock.Alive
		case identity.Dead:
			return lock.Dead
		default:
			return lock.Unknown
		}
	}
	_, err := lock.Acquire(l.lockDir(), l.self(), lock.Options{
		Probe: probe,
		Codec: censusOwnerCodec{},
	})
	if err == nil {
		return nil
	}
	var holder *lock.HolderError
	if errors.As(err, &holder) {
		switch {
		case holder.Cause != nil && !os.IsNotExist(holder.Cause):
			return fmt.Errorf("census writer lock has malformed owner identity: %w", holder.Cause)
		case holder.State == lock.Alive:
			return fmt.Errorf("live census writer already owns %s", l.Dir)
		default:
			return fmt.Errorf("census writer liveness is unprovable; refusing takeover in %s", l.Dir)
		}
	}
	return err
}

// Release frees the lock only while this process still owns it; a lock taken
// over by a successor is left untouched. It never returns an error — a release
// that cannot prove ownership simply does nothing.
func (l *CensusWriterLock) Release() {
	_ = lock.ReleaseNamed(l.lockDir(), l.self(), censusOwnerCodec{})
}
