// Package identity answers the one question every supervision decision
// hangs on: is the process I recorded still the process at that pid?
// Identity is kernel facts — pid plus start time — never claims, and
// liveness answers are THREE-WAY (D-2, SLC-R11-002): only a successful
// read proving absence is DEAD; a failed read is UNKNOWN, and UNKNOWN
// never authorizes anything.
//
// The committed shell helpers read start times to whole seconds, which
// is why REG-6's kill proof needs argv as a third factor. This package
// reads exact kernel start times — microseconds on darwin, 10ms
// clock-tick resolution on linux, both far finer than the whole-second
// resolution every decision actually compares at — while still speaking
// seconds to records for compatibility with every artifact the system
// already has.
//
// SAME-USER SCOPE INVARIANT (B2): every consumer that acts on Dead —
// the reapers, lock takeover, the lease sweep — judges processes this
// engine's own user spawned. The platform probers must never misread
// another user's LIVE process as dead (permission denial is Unknown or
// existence, never death), and supervision refuses to arm where the
// platform cannot keep that promise (restricted procfs; see procfs.go).
package identity

import (
	"time"
)

// Ref is a recorded process identity: the pid and the second its
// process started, the resolution every existing record carries.
// StartTicks and BootID are the clock-step-immune pair (issue #1): on
// Linux the btime-anchored epoch second MOVES when the realtime clock
// steps (time-synced guests step every ~30s), so equality on seconds
// reads live processes as Dead. Ticks are boot-relative and constant
// for the process's life; the boot id changes only across reboots —
// the one case where ticks could be ambiguous. Records that predate
// the pair carry zero values and fall back to the seconds comparison.
type Ref struct {
	Pid          int64
	StartedAtSec int64
	StartTicks   int64
	BootID       string
}

// Exact is a live identity as the kernel reports it.
type Exact struct {
	Pid        int64
	StartedAt  time.Time // kernel-exact (microseconds on darwin, 10ms ticks on linux)
	StartTicks int64     // linux: /proc/<pid>/stat field 22, clock-step-immune; 0 elsewhere
	BootID     string    // linux: /proc/sys/kernel/random/boot_id; "" elsewhere
	Argv       []string  // valid only when ArgvKnown; see below
	// ArgvKnown records whether the argv read SUCCEEDED. Argv is
	// best-effort at probe time — a process whose argv cannot be read is
	// still alive — but a consumer matching a tag against Argv must treat
	// ArgvKnown=false as absence of evidence, never as a failed match:
	// an unreadable argv proves nothing, and Unknown never authorizes
	// anything (go-production-grade B1).
	ArgvKnown bool
}

// Ref converts an exact identity to record resolution, carrying the
// clock-step-immune pair when the platform supplies one.
func (e Exact) Ref() Ref {
	return Ref{Pid: e.Pid, StartedAtSec: e.StartedAt.Unix(), StartTicks: e.StartTicks, BootID: e.BootID}
}

// sameIdentity is the ONE equality rule between a live probe and a
// recorded identity: when both sides carry the clock-step-immune pair,
// the pair decides — the btime-derived second is not consulted at all;
// otherwise the legacy whole-second comparison stands (darwin's
// fork-captured start time is stable, and legacy records have nothing
// stronger to offer).
func sameIdentity(exact Exact, ref Ref) bool {
	if ref.StartTicks > 0 && ref.BootID != "" && exact.StartTicks > 0 && exact.BootID != "" {
		return exact.StartTicks == ref.StartTicks && exact.BootID == ref.BootID
	}
	return exact.StartedAt.Unix() == ref.StartedAtSec
}

// Liveness is the three-way verdict.
type Liveness int

const (
	Alive Liveness = iota
	Dead
	Unknown
)

func (l Liveness) String() string {
	switch l {
	case Alive:
		return "alive"
	case Dead:
		return "dead"
	default:
		return "unknown"
	}
}

// Prober reads live process facts. The kernel-backed implementation
// lives in this package per platform; tests inject fakes. Small and
// consumer-defined on purpose.
type Prober interface {
	// Probe returns the exact identity at pid. A SUCCESSFUL
	// determination that no such process exists returns (zero, Dead,
	// nil). A read that could not determine anything returns
	// (zero, Unknown, err).
	Probe(pid int64) (Exact, Liveness, error)
}

// AliveRef reports whether a recorded identity is the process still at
// its pid, three-way. The seconds comparison is intentional: records
// carry seconds, and a mismatch at that resolution is a DIFFERENT
// process, definitively.
func AliveRef(prober Prober, ref Ref) Liveness {
	exact, state, _ := prober.Probe(ref.Pid)
	switch state {
	case Dead:
		return Dead
	case Unknown:
		return Unknown
	}
	if !sameIdentity(exact, ref) {
		// The pid was reused by a different process: definitively gone.
		// (With the pair recorded this cannot fire from a clock step;
		// legacy seconds-only records keep the old semantics.)
		return Dead
	}
	return Alive
}
