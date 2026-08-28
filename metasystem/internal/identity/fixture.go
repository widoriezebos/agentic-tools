package identity

import (
	"fmt"
	"runtime"
	"time"
)

// The fake-identity fixture table (METASYSTEM_FAKE_PROCESS_IDENTITY_FILE)
// has ONE reader: scattered private parsers breed divergent key
// spellings, and the shell fixtures end up writing every spelling into
// every entry to satisfy them all.

// FixtureEntry is one pid's recorded identity in the fixture table. The
// Has* flags distinguish an absent field from a zero value.
type FixtureEntry struct {
	StartedAt              int64
	HasStartedAt           bool
	StartedAtExactMicro    int64
	HasStartedAtExactMicro bool
	StartTicks             int64
	HasStartTicks          bool
	BootID                 string
	HasBootID              bool
	Command                string
	HasCommand             bool
	Pgid                   int64
	HasPgid                bool
	Terminal               bool
	HasTerminal            bool
}

// FixtureProbe is the neutral seam fixture-capable identity decisions
// accept: internal/fixtureauth implements it
// behind root-checked authorization; a nil probe refuses every
// fixture read. The file reader itself moved to fixtureauth — this
// foundation package no longer touches the environment.
type FixtureProbe interface {
	FixtureEntry(pid int64) (FixtureEntry, bool)
}

// probeEntry is the nil-safe read every consumer in this package uses.
func probeEntry(probe FixtureProbe, pid int64) (FixtureEntry, bool) {
	if probe == nil {
		return FixtureEntry{}, false
	}
	return probe.FixtureEntry(pid)
}

// FixtureStartReader lets an authorized fixture provide the exact start token
// for a live process. A kernel absence always wins, a present but malformed
// fixture row is indeterminate, and an absent row falls back to the kernel.
type FixtureStartReader struct {
	Kernel  StartReader
	Fixture FixtureProbe
}

func (r FixtureStartReader) ReadStart(pid int64) (Exact, Liveness, error) {
	if r.Kernel == nil {
		return Exact{}, Unknown, fmt.Errorf("identity: exact start reader is unavailable")
	}
	kernelExact, kernelState, kernelErr := r.Kernel.ReadStart(pid)
	if kernelErr == nil && kernelState == Dead {
		return Exact{}, Dead, nil
	}
	entry, present := probeEntry(r.Fixture, pid)
	if !present {
		return kernelExact, kernelState, kernelErr
	}
	exact, err := exactFixtureIdentity(pid, entry)
	if err != nil {
		return Exact{}, Unknown, err
	}
	return exact, Alive, nil
}

func exactFixtureIdentity(pid int64, entry FixtureEntry) (Exact, error) {
	if !entry.HasStartedAt || entry.StartedAt < 1 {
		return Exact{}, fmt.Errorf("identity: fixture pid %d has no valid start second", pid)
	}
	switch runtime.GOOS {
	case "darwin":
		if !entry.HasStartedAtExactMicro || entry.StartedAtExactMicro < 1 ||
			entry.StartedAtExactMicro/1_000_000 != entry.StartedAt {
			return Exact{}, fmt.Errorf("identity: fixture pid %d has no valid Darwin exact start identity", pid)
		}
		return Exact{Pid: pid, StartedAt: time.UnixMicro(entry.StartedAtExactMicro)}, nil
	case "linux":
		if !entry.HasStartTicks || entry.StartTicks < 1 || !entry.HasBootID || entry.BootID == "" {
			return Exact{}, fmt.Errorf("identity: fixture pid %d has no valid Linux exact start identity", pid)
		}
		return Exact{
			Pid: pid, StartedAt: time.Unix(entry.StartedAt, 0),
			StartTicks: entry.StartTicks, BootID: entry.BootID,
		}, nil
	default:
		return Exact{}, fmt.Errorf("identity: fixture exact start identity is unsupported on %s", runtime.GOOS)
	}
}
