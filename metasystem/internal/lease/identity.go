// Package lease owns the checkout write-authority model: it identifies the
// main process of a session, classifies who is calling a gated command
// (MAIN, DELEGATE, SUPERVISION, ADAPTER-SUPERVISOR, or HUMAN), and holds the
// single checkout lease that decides which main may write. It uses real types
// and error returns, and fixes the KI-33 same-process lockout.
package lease

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/fixtureauth"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/census"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Identity is a process's start time and command line, read from one source.
// The single-source rule is load-bearing: a main authenticates its own
// announcement by matching (pid, pidStartedAt, commandHash), so the start
// time it announces and the start time a later read sees must come from the
// same place. Splitting them across two sources once made a main unable to
// recognise itself and classify as a delegate through its own ancestor.
type Identity struct {
	Pid       int64
	StartedAt int64
	Command   string
}

// CommandHash is the announcement's stable fingerprint of a command line.
func CommandHash(command string) string {
	sum := sha256.Sum256([]byte(command))
	return hex.EncodeToString(sum[:])
}

// ProcessIdentity reads a live process's start time and command from the one
// authoritative source — the fake process table when a fixture installs one,
// otherwise the kernel — via the census. ok is false when the pid is not a
// live, readable process.
func ProcessIdentity(pid int64, probe identity.FixtureProbe) (Identity, bool) {
	proc, err := census.AuthIdentity(pid, probe)
	if err != nil {
		return Identity{}, false
	}
	return Identity{Pid: pid, StartedAt: proc.PidStartedAt, Command: proc.Command}, true
}

// StartedAt is the process's start second from the one source. ok is false
// when the pid is not live/readable.
func StartedAt(pid int64, probe identity.FixtureProbe) (int64, bool) {
	id, ok := ProcessIdentity(pid, probe)
	if !ok {
		return 0, false
	}
	return id.StartedAt, true
}

// ProcessCommand is the process's command line from the one source.
func ProcessCommand(pid int64, probe identity.FixtureProbe) (string, bool) {
	id, ok := ProcessIdentity(pid, probe)
	if !ok {
		return "", false
	}
	return id.Command, true
}

// ParentPid returns a process's parent pid, or ok=false when there is no
// distinct live ancestor to walk to.
func ParentPid(pid int64) (int64, bool) {
	return identity.ParentPid(pid)
}

// Live reports whether the process identified by (pid, startedAt) is still
// running. The zero signal proves some process owns the pid now; an
// unreadable start time cannot prove the recorded holder died, so the only
// safe answer is alive. A readable start-time mismatch does prove pid reuse,
// so it reports dead. This is what lets a live holder keep its lease and a
// dead one lose it.
func Live(pid, startedAt int64, probe identity.FixtureProbe) bool {
	if pid < 1 || startedAt < 1 {
		return false
	}
	switch err := unix.Kill(int(pid), 0); err {
	case nil:
		// A process owns the pid; confirm it is the recorded one.
	case unix.EPERM:
		// Owned by another user — alive, and not ours to inspect further.
		return true
	default:
		// ESRCH and anything else: no such process.
		return false
	}
	actual, ok := StartedAt(pid, probe)
	if !ok {
		return true
	}
	return actual == startedAt
}

// fixtureProbe constructs the root-checked fixture authority for this
// package's entry points (agnosticism B1). The error is the loud
// leaked-fixture refusal; a nil probe refuses fixtures.
func fixtureProbe(root string) (identity.FixtureProbe, error) {
	authorization, err := fixtureauth.New(root)
	if err != nil {
		return nil, err
	}
	return authorization.Identity(), nil
}
