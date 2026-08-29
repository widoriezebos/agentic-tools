package dispatch

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// CustodyAdd registers a process in a job's custody list: the set of exact
// process identities (pid plus kernel start time) supervision may wind down.
// The whole read-dedupe-append-write runs under the session and record locks,
// so custody registration cannot race a status transition or its index write.
// Only a live job (pending or running) accepts custody; anything else is a
// silent refusal.
func CustodyAdd(root, job string, pid int64, reader identity.Prober, groupReaders ...func(int64) (int64, error)) error {
	if pid < 1 || reader == nil {
		return fmt.Errorf("dispatch: custody registration requires a pid and identity reader")
	}
	exact, state, err := reader.Probe(pid)
	if err != nil || state != identity.Alive || exact.Pid != pid {
		return fmt.Errorf("dispatch: custody pid %d exact start identity is unavailable", pid)
	}
	ref := exact.Ref()
	if !ref.NativeExact() {
		return fmt.Errorf("dispatch: custody pid %d has no platform-exact start identity", pid)
	}
	groupID := func(pid int64) (int64, error) {
		group, err := unix.Getpgid(int(pid))
		return int64(group), err
	}
	if len(groupReaders) > 0 && groupReaders[0] != nil {
		groupID = groupReaders[0]
	}
	return withRecordSessionLock(root, job, func(recordPath string, transaction *SessionIndexTransaction) error {
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
		pgid, groupErr := groupID(pid)
		if groupErr != nil || pgid < 2 {
			return fmt.Errorf("dispatch: custody pid %d process group is unavailable", pid)
		}
		confirmed, confirmedState, confirmedErr := reader.Probe(pid)
		if confirmedErr != nil || confirmedState != identity.Alive {
			return fmt.Errorf("dispatch: custody pid %d changed while its group was read", pid)
		}
		if !identity.SameIdentity(confirmed, ref) {
			return fmt.Errorf("dispatch: custody pid %d changed while its group was read", pid)
		}
		ref = confirmed.Ref()
		// The same exact identity registered twice collapses to one entry. A
		// recycled pid keeps its own entry even when both starts share a second.
		var items []any
		if existing, ok := record["custodyProcesses"].([]any); ok {
			for _, item := range existing {
				entry, ok := item.(map[string]any)
				if ok {
					if recorded, valid := identityRefFromObject(entry); valid && sameRecordedIdentity(recorded, ref) {
						continue
					}
				}
				items = append(items, item)
			}
		}
		entry := exactIdentityFields(ref)
		entry["instanceTag"] = tag
		entry["pgid"] = pgid
		items = append(items, entry)
		record["custodyProcesses"] = items
		if err := writeRecord(recordPath, record); err != nil {
			return err
		}
		if err := transaction.syncRecord(job, record); err != nil {
			return err
		}
		_, err = SweepSatisfiedPreforkMarker(root, record)
		return err
	})
}
