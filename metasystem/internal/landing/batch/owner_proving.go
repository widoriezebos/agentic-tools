package batch

import "time"

// ProvingWaitVerb is the history verb of a batch whose start waits for the
// host's proving flock (U12): one batch proves at a time on the host.
const ProvingWaitVerb = "proving-wait"

// waitsForProving probes the host's proving flock before a batch's proof or
// diagnostic starts. The flock is held by the proof child that runs, for its
// whole life, never by the owner, so an owner's pause, restart or lane move
// cannot strand it. While a proof holds it the batch is not started: it stays
// where it is (an open batch keeps taking joins), its history records the
// wait once, and a later tick starts it once the flock is free. A probe that
// races a start only makes the second child wait for the first.
func (owner *Owner) waitsForProving(id string, at time.Time) (bool, error) {
	if owner.proving == nil {
		return false, nil
	}
	holder, busy, err := owner.proving()
	if err != nil || !busy {
		return false, err
	}
	return true, owner.store.Update(id, func(current *Record) error {
		if _, waiting := ProvingWait(*current); waiting {
			return nil
		}
		current.Transition(current.State, at, ProvingWaitVerb, owner.actor,
			"another batch proves on this host ("+holder+"); this batch starts when that proof ends")
		return nil
	})
}

// ProvingWait is a batch's current wait for the host's proving flock: the
// last proving-wait entry, while no state change has followed it.
func ProvingWait(record Record) (HistoryEntry, bool) {
	for index := len(record.History) - 1; index >= 0; index-- {
		entry := record.History[index]
		switch {
		case entry.Verb == ProvingWaitVerb:
			return entry, true
		case entry.From != entry.To:
			return HistoryEntry{}, false
		}
	}
	return HistoryEntry{}, false
}
