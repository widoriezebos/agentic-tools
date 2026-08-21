package identity

import "strings"

// TaggedSurvivors reports whether any live process OTHER than the
// recorded custodian still carries the instance tag in its argv.
// This is the fact a kill-less reaper must hold before it may claim
// group death: a dead custodian with tagged survivors is a live
// group, and concluding it stamps a proof nobody made — the record
// goes terminal while the orphan keeps running, and no later pass
// ever winds it down. certain=false means the process table could
// not be enumerated; indeterminacy never acts.
func TaggedSurvivors(tag string, exclude int64) (alive bool, certain bool) {
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
	for _, pid := range pids {
		if pid == exclude {
			continue
		}
		exact, state, err := prober.Probe(pid)
		if err != nil || state != Alive || !exact.ArgvKnown {
			continue
		}
		if strings.Contains(strings.Join(exact.Argv, " "), tag) {
			return true, true
		}
	}
	return false, true
}
