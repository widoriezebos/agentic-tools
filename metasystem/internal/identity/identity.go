// Package identity answers the one question every supervision decision
// hangs on: is the process I recorded still the process at that pid?
// Identity is kernel facts — pid plus its platform-native start identity —
// never claims. Liveness answers are three-way: only a successful read
// proving absence is Dead; a failed read is Unknown, and Unknown never
// authorizes anything.
//
// SAME-USER SCOPE INVARIANT: every consumer that acts on Dead —
// the reapers, lock takeover, the lease sweep — judges processes this
// engine's own user spawned. The platform probers must never misread
// another user's LIVE process as dead (permission denial is Unknown or
// existence, never death), and supervision refuses to arm where the
// platform cannot keep that promise (restricted procfs; see procfs.go).
package identity

import (
	"runtime"
	"time"
)

// Ref is a recorded process identity. New Darwin records carry the kernel's
// microsecond start token; new Linux records carry start ticks plus boot ID.
// StartedAtSec remains alongside either exact shape for older readers. A ref
// with neither exact shape is a legacy record and compares by whole seconds.
type Ref struct {
	Pid                int64
	StartedAtSec       int64
	StartedAtUnixMicro int64
	StartTicks         int64
	BootID             string
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
	// anything.
	ArgvKnown bool
}

// Ref converts a live identity to exactly one native record shape.
func (e Exact) Ref() Ref {
	ref := Ref{Pid: e.Pid, StartedAtSec: e.StartedAt.Unix()}
	if e.StartTicks != 0 || e.BootID != "" {
		ref.StartTicks = e.StartTicks
		ref.BootID = e.BootID
		return ref
	}
	ref.StartedAtUnixMicro = e.StartedAt.UnixMicro()
	return ref
}

// ComparisonMode labels the representation that decided an identity join.
// The label makes the weaker legacy fallback visible to callers.
type ComparisonMode string

const (
	CompareInvalid            ComparisonMode = "invalid"
	CompareDarwinMicroseconds ComparisonMode = "darwin-microseconds"
	CompareLinuxTicksBootID   ComparisonMode = "linux-ticks-boot-id"
	CompareLegacySeconds      ComparisonMode = "legacy-seconds"
)

// Mode reports the one representation carried by the ref. A mixed or partial
// exact shape is invalid and never falls back to seconds.
func (r Ref) Mode() ComparisonMode {
	hasMicro := r.StartedAtUnixMicro > 0
	hasTicks := r.StartTicks > 0
	hasBoot := r.BootID != ""
	if hasMicro && (hasTicks || hasBoot) {
		return CompareInvalid
	}
	if hasTicks != hasBoot {
		return CompareInvalid
	}
	if hasMicro {
		return CompareDarwinMicroseconds
	}
	if hasTicks {
		return CompareLinuxTicksBootID
	}
	if r.StartedAtSec > 0 {
		return CompareLegacySeconds
	}
	return CompareInvalid
}

// NativeExact reports whether the ref carries this host platform's required
// exact representation.
func (r Ref) NativeExact() bool {
	switch runtime.GOOS {
	case "darwin":
		return r.Mode() == CompareDarwinMicroseconds
	case "linux":
		return r.Mode() == CompareLinuxTicksBootID
	default:
		return false
	}
}

// Comparison is the result of joining one live kernel observation to one
// durable ref.
type Comparison struct {
	Matches bool
	Mode    ComparisonMode
}

// Compare is the one equality rule between a live probe and a recorded
// identity. Exact fields are exclusive: a malformed or unavailable exact
// representation never weakens itself to the legacy seconds rule.
func Compare(exact Exact, ref Ref) Comparison {
	mode := ref.Mode()
	comparison := Comparison{Mode: mode}
	if exact.Pid != ref.Pid || mode == CompareInvalid {
		return comparison
	}
	switch mode {
	case CompareDarwinMicroseconds:
		comparison.Matches = exact.StartTicks == 0 && exact.BootID == "" &&
			exact.StartedAt.UnixMicro() == ref.StartedAtUnixMicro
	case CompareLinuxTicksBootID:
		comparison.Matches = exact.StartTicks == ref.StartTicks && exact.BootID == ref.BootID
	case CompareLegacySeconds:
		comparison.Matches = exact.StartedAt.Unix() == ref.StartedAtSec
	}
	return comparison
}

func sameIdentity(exact Exact, ref Ref) bool {
	return Compare(exact, ref).Matches
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

// StartReader reads only the identity token. It is used when argv is either
// unnecessary or must be read between two independent start observations.
type StartReader interface {
	ReadStart(pid int64) (Exact, Liveness, error)
}

// AliveRef reports whether a recorded identity is the process still at its
// pid, three-way.
func AliveRef(prober Prober, ref Ref) Liveness {
	state, _ := AliveRefComparison(prober, ref)
	return state
}

// AliveRefComparison also reports which recorded representation decided the
// join, including the labeled legacy fallback.
func AliveRefComparison(prober Prober, ref Ref) (Liveness, ComparisonMode) {
	var exact Exact
	var state Liveness
	if reader, ok := prober.(StartReader); ok {
		exact, state, _ = reader.ReadStart(ref.Pid)
	} else {
		exact, state, _ = prober.Probe(ref.Pid)
	}
	switch state {
	case Dead:
		return Dead, ref.Mode()
	case Unknown:
		return Unknown, ref.Mode()
	}
	comparison := Compare(exact, ref)
	if comparison.Mode == CompareInvalid {
		return Unknown, comparison.Mode
	}
	if !comparison.Matches {
		// The pid was reused by a different process: definitively gone.
		return Dead, comparison.Mode
	}
	return Alive, comparison.Mode
}
