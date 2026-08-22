package identity

import (
	"strings"

	"golang.org/x/sys/unix"
)

// TaggedSurvivors reports whether any live process OTHER than the
// recorded custodian still carries the instance tag in its argv.
// This is the fact a kill-less reaper must hold before it may claim
// group death: a dead custodian with tagged survivors is a live
// group, and concluding it stamps a proof nobody made — the record
// goes terminal while the orphan keeps running, and no later pass
// ever winds it down. certain=false means the scan could not prove
// absence — an unenumerable table, or a signalable-but-unreadable
// process inside the recorded group (indeterminacy never
// acts). The indeterminacy scope is the GROUP, not the whole
// table: same-uid unreadable processes are chronic on macOS
// (zombies, restricted daemons), group membership stays readable
// when argv does not, and a process outside the dead custodian's
// group that is ALSO unreadable is the census's stray problem, not
// this proof's. pgid<=1 means no group was recorded, and any
// signalable unreadable process then defers conservatively.
func TaggedSurvivors(tag string, exclude, pgid int64) (alive bool, certain bool) {
	if tag == "" {
		// No tag was ever recorded: there is nothing to scan for and
		// no survivor claim to make either way.
		return false, true
	}
	pids, err := AllPids()
	if err != nil {
		return false, false
	}
	prober := KernelProber{}
	uncertain := false
	for _, pid := range pids {
		if pid == exclude {
			continue
		}
		exact, state, err := prober.Probe(pid)
		if err != nil || state == Unknown || (state == Alive && !exact.ArgvKnown) {
			// Signalable (kill-0 nil: ours to own) AND inside the
			// recorded group — or no group recorded at all — is the
			// indeterminacy that defers; keep scanning, because a
			// positive sighting later in the table is definitive.
			if unix.Kill(int(pid), 0) == nil {
				if pgid <= 1 {
					uncertain = true
				} else if got, pgErr := unix.Getpgid(int(pid)); pgErr == nil && int64(got) == pgid {
					uncertain = true
				}
			}
			continue
		}
		if state != Alive {
			continue
		}
		if strings.Contains(strings.Join(exact.Argv, " "), tag) {
			return true, true
		}
	}
	if uncertain {
		return false, false
	}
	return false, true
}
