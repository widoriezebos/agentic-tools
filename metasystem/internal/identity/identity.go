// Package identity answers the one question every supervision decision
// hangs on: is the process I recorded still the process at that pid?
// Identity is kernel facts — pid plus start time — never claims, and
// liveness answers are THREE-WAY (D-2, SLC-R11-002): only a successful
// read proving absence is DEAD; a failed read is UNKNOWN, and UNKNOWN
// never authorizes anything.
//
// The committed shell helpers read start times to whole seconds, which
// is why REG-6's kill proof needs argv as a third factor. This package
// reads exact kernel start times (microseconds on darwin), shrinking
// that residual for live comparisons while still speaking seconds to
// records for compatibility with every artifact the system already has.
package identity

import (
	"time"
)

// Ref is a recorded process identity: the pid and the second its
// process started, the resolution every existing record carries.
type Ref struct {
	Pid          int64
	StartedAtSec int64
}

// Exact is a live identity as the kernel reports it.
type Exact struct {
	Pid       int64
	StartedAt time.Time // kernel-exact (microseconds on darwin)
	Argv      []string  // valid only when ArgvKnown; see below
	// ArgvKnown records whether the argv read SUCCEEDED. Argv is
	// best-effort at probe time — a process whose argv cannot be read is
	// still alive — but a consumer matching a tag against Argv must treat
	// ArgvKnown=false as absence of evidence, never as a failed match:
	// an unreadable argv proves nothing, and Unknown never authorizes
	// anything (go-production-grade B1).
	ArgvKnown bool
}

// Ref converts an exact identity to record resolution.
func (e Exact) Ref() Ref {
	return Ref{Pid: e.Pid, StartedAtSec: e.StartedAt.Unix()}
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
	if exact.StartedAt.Unix() != ref.StartedAtSec {
		// The pid was reused by a process started at a different
		// second: the recorded process is definitively gone.
		return Dead
	}
	return Alive
}
