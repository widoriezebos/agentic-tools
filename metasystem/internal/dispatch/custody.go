package dispatch

// CustodyAdd registers a process in a job's custody list: the set of exact
// process identities (pid plus kernel start time) supervision may wind down.
// The whole read-dedupe-append-write runs under the record lock, so custody
// registration can never race a status transition. Only a live job (pending
// or running) accepts custody; anything else is a silent refusal.
func CustodyAdd(root, job string, pid, pidStartedAt int64) error {
	return withRecordLock(root, job, func(recordPath string) error {
		record, err := readObject(recordPath)
		if err != nil {
			return refuse(1, "cannot register custody for %s: %v", job, err)
		}
		status := asString(record["status"])
		if status != "pending" && status != "running" {
			return silentRefusal(1)
		}
		// The uniform marked-record rule: registration defers during
		// a cancellation — the group kill owns every process the
		// registering child belongs to, and no writer but the
		// conclude advances a marked record.
		if asString(record["phase"]) == "cancelling" {
			return silentRefusal(1)
		}
		tag := asString(record["instanceTag"])
		if tag == "" {
			return refuse(1, "job record %s has no instance tag", job)
		}
		// The same exact identity (pid plus kernel start time) registered
		// twice collapses to one entry. A recycled pid with a different start
		// is a different process and keeps its own entry.
		var items []any
		if existing, ok := record["custodyProcesses"].([]any); ok {
			for _, item := range existing {
				entry, ok := item.(map[string]any)
				if ok && looseEqual(entry["pid"], pid) && looseEqual(entry["pidStartedAt"], pidStartedAt) {
					continue
				}
				items = append(items, item)
			}
		}
		items = append(items, map[string]any{
			"pid": pid, "pidStartedAt": pidStartedAt, "instanceTag": tag,
		})
		record["custodyProcesses"] = items
		return writeRecord(recordPath, record)
	})
}
