package batch

// ProvingWaitVerb is the history verb of a batch whose start waits for the
// host's proving flock (U12): one batch proves at a time on the host.
const ProvingWaitVerb = "proving-wait"

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
