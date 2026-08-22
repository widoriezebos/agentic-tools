package identity

import (
	"strings"

	"golang.org/x/sys/unix"
)

// TagState classifies a recorded (pid, tag) for the shell's liveness
// ladder: "dead" (no such process), "live" (the
// process's argv carries the tag), "stale" (argv readable, tag absent or
// none recorded — a stranger holds the pid), "unknown" (the process exists
// but cannot be inspected). The old shell ladder was two-way on the
// kill-capable path — an unreadable process table turned live supervisors
// into process-lost verdicts; unknown exists so callers can DEFER, the same
// indeterminacy-never-acts rule the Go reaper and arm-supervision already
// follow. Kernel-only by design: the ladder judges real spawned processes,
// and the fixture-table override belongs to Custodian verdicts.
func TagState(prober Prober, pid int64, tag string) string {
	if pid < 1 {
		return "dead"
	}
	switch unix.Kill(int(pid), 0) {
	case unix.ESRCH:
		return "dead"
	case unix.EPERM:
		// Exists, but another user's: not ours to inspect, never proof of
		// death or of a stranger.
		return "unknown"
	}
	exact, state, err := prober.Probe(pid)
	if err != nil || state == Unknown {
		return "unknown"
	}
	if state == Dead {
		return "dead"
	}
	if !exact.ArgvKnown {
		return "unknown"
	}
	if tag != "" && strings.Contains(strings.Join(exact.Argv, " "), tag) {
		return "live"
	}
	return "stale"
}
