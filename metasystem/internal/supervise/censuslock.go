package supervise

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The census-writer lock keeps at most one component publishing the census
// verdict for a supervision directory. Two rules make it safe, and both are
// enforced here rather than left to the caller.
//
// A claim publishes the lock and its owner in ONE step: the owner file is built
// inside a staging directory that is renamed into place, and a directory rename
// replaces only an EMPTY directory. So a claim takes an absent lock, heals an
// ownerless husk a crash left behind, and refuses one a live process owns — and
// no observer ever sees a lock without an owner.
//
// A release frees the lock only while this process still owns it. Takeover
// requires proven death, so a process alive enough to run its own release
// cannot have been replaced; but a process whose lock WAS taken over must not
// delete its successor's owner file or hand the lock to a third writer.

// CensusWriterLock is one component's claim on the sole census-writer role for
// a supervision directory.
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

func (l *CensusWriterLock) lockDir() string   { return filepath.Join(l.Dir, "census-writer.d") }
func (l *CensusWriterLock) ownerPath() string { return filepath.Join(l.lockDir(), "owner.json") }

type censusOwner struct {
	Function        string `json:"function"`
	Pid             int64  `json:"pid"`
	PidStartedAt    int64  `json:"pidStartedAt"`
	InstanceTag     string `json:"instanceTag"`
	ObservedAtEpoch int64  `json:"observedAtEpoch"`
}

// Claim takes the census-writer lock, healing a dead owner's husk by takeover.
// It returns an error when a LIVE writer already owns the lock (the caller must
// not publish a second census stream) or when a takeover repeatedly loses the
// race to a concurrent claimant.
func (l *CensusWriterLock) Claim() error {
	if err := os.MkdirAll(l.Dir, 0o755); err != nil {
		return fmt.Errorf("prepare supervision dir: %w", err)
	}
	payload, err := json.Marshal(censusOwner{
		Function: "census-writer", Pid: l.Self.Pid, PidStartedAt: l.Self.StartedAtSec,
		InstanceTag: l.Tag, ObservedAtEpoch: time.Now().Unix(),
	})
	if err != nil {
		return fmt.Errorf("census writer owner: %w", err)
	}
	for attempt := 0; attempt < 3; attempt++ {
		staging, err := os.MkdirTemp(l.Dir, "census-writer.claim.")
		if err != nil {
			return fmt.Errorf("stage census writer claim: %w", err)
		}
		if err := os.WriteFile(filepath.Join(staging, "owner.json"), append(payload, '\n'), 0o644); err != nil {
			os.RemoveAll(staging)
			return fmt.Errorf("stage census writer owner: %w", err)
		}
		if err := os.Rename(staging, l.lockDir()); err == nil {
			return nil // claimed an absent or freshly-healed lock
		}
		os.RemoveAll(staging)

		owner, err := l.readOwner()
		if err != nil {
			return fmt.Errorf("census writer lock has malformed owner identity: %w", err)
		}
		switch identity.AliveRef(l.Prober, identity.Ref{Pid: owner.Pid, StartedAtSec: owner.PidStartedAt}) {
		case identity.Alive:
			return fmt.Errorf("live census writer already owns %s", l.Dir)
		case identity.Unknown:
			// UNKNOWN NEVER AUTHORIZES (the three-way rule; review finding
			// dispatch-supervise-3): a prior owner whose liveness cannot be
			// proven keeps its lock exactly as a live one does. Taking over
			// on indeterminacy could destroy a live writer's lock.
			return fmt.Errorf("census writer liveness is unprovable; refusing takeover in %s", l.Dir)
		}
		// The owner is provably dead: move its husk aside so the next attempt
		// can claim an empty lock. Losing that rename means another writer
		// healed it first — retry and race for the claim again.
		husk := filepath.Join(l.Dir, fmt.Sprintf("census-writer.dead.%d.%d", l.Self.Pid, attempt))
		if err := os.Rename(l.lockDir(), husk); err != nil {
			continue
		}
		os.RemoveAll(husk)
	}
	return fmt.Errorf("census writer takeover lost a race in %s", l.Dir)
}

// Release frees the lock only while this process still owns it; a lock taken
// over by a successor is left untouched. It never returns an error — a release
// that cannot prove ownership simply does nothing.
func (l *CensusWriterLock) Release() {
	owner, err := l.readOwner()
	if err != nil || owner.Pid != l.Self.Pid || owner.PidStartedAt != l.Self.StartedAtSec {
		return // absent, unreadable, or a successor's lock: leave it alone
	}
	retiring := filepath.Join(l.Dir, fmt.Sprintf("census-writer.retiring.%d", l.Self.Pid))
	if err := os.Rename(l.lockDir(), retiring); err != nil {
		return
	}
	os.RemoveAll(retiring)
}

func (l *CensusWriterLock) readOwner() (censusOwner, error) {
	data, err := os.ReadFile(l.ownerPath())
	if err != nil {
		return censusOwner{}, err
	}
	var owner censusOwner
	if err := json.Unmarshal(data, &owner); err != nil {
		return censusOwner{}, err
	}
	if owner.Pid < 1 {
		return censusOwner{}, fmt.Errorf("owner file names no pid")
	}
	return owner, nil
}
