package batch

import "time"

// ProvingWaitVerb is the history verb of a batch whose start waits for the
// host's proving flock (U12): one batch proves at a time on the host.
const ProvingWaitVerb = "proving-wait"

// takeProving takes the host's proving flock for a batch's run. When another
// batch holds it, the batch is not dispatched: it stays where it is (an open
// batch keeps taking joins), its history records the wait once, and a later
// tick starts it once the flock is free. The admission cap still applies
// inside.
func (owner *Owner) takeProving(id string, at time.Time) (release func() error, waited bool, err error) {
	if owner.proving == nil {
		return nil, false, nil
	}
	release, holder, err := owner.proving()
	if err != nil || release != nil {
		return release, false, err
	}
	return nil, true, owner.store.Update(id, func(current *Record) error {
		if n := len(current.History); n > 0 && current.History[n-1].Verb == ProvingWaitVerb {
			return nil
		}
		current.Transition(current.State, at, ProvingWaitVerb, owner.actor,
			"another batch proves on this host ("+holder+"); this batch starts when that proof ends")
		return nil
	})
}

func releaseHost(release func() error) error {
	if release == nil {
		return nil
	}
	return release()
}
