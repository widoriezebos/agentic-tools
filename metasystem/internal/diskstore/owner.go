package diskstore

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// OwnerKind names who a store belongs to; each kind has exactly one proof
// that its owner ended and nothing still uses the store (3.1's table).
type OwnerKind string

const (
	OwnerProcess      OwnerKind = "process"
	OwnerTestBinary   OwnerKind = "test-binary"
	OwnerRun          OwnerKind = "run"
	OwnerDelegate     OwnerKind = "delegate"
	OwnerGoal         OwnerKind = "goal"
	OwnerWorkspace    OwnerKind = "workspace"
	OwnerSession      OwnerKind = "session"
	OwnerLaunch       OwnerKind = "launch"
	OwnerUnit         OwnerKind = "unit"
	OwnerAttempt      OwnerKind = "attempt"
	OwnerLease        OwnerKind = "lease"
	OwnerInstallation OwnerKind = "installation"
	OwnerMachine      OwnerKind = "machine"
)

var ownerKinds = []OwnerKind{OwnerProcess, OwnerTestBinary, OwnerRun, OwnerDelegate, OwnerGoal, OwnerWorkspace,
	OwnerSession, OwnerLaunch, OwnerUnit, OwnerAttempt, OwnerLease, OwnerInstallation, OwnerMachine}

// KnownOwnerKind reports whether kind is one of the table's owner kinds.
func KnownOwnerKind(kind OwnerKind) bool {
	for _, known := range ownerKinds {
		if kind == known {
			return true
		}
	}
	return false
}

// Decision is what a proof says about one store this pass.
type Decision string

const (
	// Release: the owner ended and nothing needed or using the store remains.
	Release Decision = "release"
	// Keep: the owner lives, or the store holds something still needed; the
	// report names the reason and the command that ends the owner.
	Keep Decision = "keep"
	// Pending: the proof could not be completed (Unknown, a held lock, an
	// unreadable record, an incomplete census); never a refusal, never a
	// deletion.
	Pending Decision = "pending"
)

// Verdict is a proof's answer for one store.
type Verdict struct {
	Decision Decision `json:"decision"`
	Reason   string   `json:"reason"`
	// Command is the public command a person runs to end the owner, capture
	// the work, or settle what kept the store. Every Keep and Pending carries
	// one (H1: a bare pid is never a remedy).
	Command string `json:"command,omitempty"`
}

// OwnerProof is an owner kind's proof, split into an observation and a
// mutation (R15, DL2-13). Observe reads only and creates nothing; Apply runs
// inside the store's critical section, after Observe's Release has been
// re-checked there, and removes or releases the store.
type OwnerProof interface {
	Kind() OwnerKind
	Observe(ctx context.Context, record Record) Verdict
	Apply(ctx context.Context, critical *Critical) error
}

// DefaultWorkspaceName is the name of a workspace requested without one.
const DefaultWorkspaceName = "default"

// ReservationKey is the stable identity of a workspace request (3.6):
// (owner kind, owner ref, name), so one name under two sessions, or under a
// goal and a session, is two workspaces.
func ReservationKey(owner Owner, name string) (string, error) {
	if !KnownOwnerKind(owner.Kind) || strings.TrimSpace(owner.Ref) == "" {
		return "", fmt.Errorf("a reservation needs a known owner kind and reference")
	}
	if name == "" {
		name = DefaultWorkspaceName
	}
	if strings.ContainsAny(name, "/\x00") || strings.ContainsRune(string(owner.Kind)+owner.Ref, 0) {
		return "", fmt.Errorf("workspace name %q may not contain a slash or NUL", name)
	}
	sum := sha256.Sum256([]byte(string(owner.Kind) + "\x00" + owner.Ref + "\x00" + name))
	return hex.EncodeToString(sum[:]), nil
}

// ReservationPath is the reservation entry of key; its lock is beside it.
func (r Registry) ReservationPath(key string) string {
	return filepath.Join(r.Dir, ".reservations", key)
}
