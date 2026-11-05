package proofrun

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// HostAdmission is a read-only snapshot of test slot holders and waiters.
type HostAdmission struct {
	Held       []hostAdmissionHeld
	Waiting    []hostAdmissionWaiting
	Unreadable int
}

type hostAdmissionHeld struct {
	Owner      ProcessIdentity
	Slot       string
	Priority   string
	OwnerEnded bool
}

type hostAdmissionWaiting struct {
	Owner    ProcessIdentity
	Priority string
	Since    time.Time
}

// ReadHostAdmission observes locked markers without creating or reclaiming them.
func ReadHostAdmission() (HostAdmission, error) {
	return readHostAdmission(hostAdmissionPath, identity.KernelProber{})
}

func readHostAdmission(path func() (string, bool, error), prober identity.Prober) (view HostAdmission, err error) {
	directory, exists, err := path()
	if err != nil || !exists {
		return view, err
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return view, nil
	}
	if err != nil {
		return view, err
	}
	for _, entry := range entries {
		name := entry.Name()
		lease := strings.HasPrefix(name, "lease-heavy-")
		if !lease && !strings.HasPrefix(name, "wait-") {
			continue
		}
		marker := filepath.Join(directory, name)
		file, readErr := os.Open(marker)
		if os.IsNotExist(readErr) {
			continue
		}
		var record hostLeaseRecord
		var owner ProcessIdentity
		var since time.Time
		if readErr == nil {
			if lease {
				record, _, readErr = readHostLeaseRecord(file)
			} else if readErr = validateHostLockFile(marker, file); readErr == nil {
				readErr = json.NewDecoder(io.LimitReader(file, hostLeaseRecordMaxBytes)).Decode(&owner)
				if info, statErr := file.Stat(); statErr != nil {
					readErr = statErr
				} else {
					since = info.ModTime()
				}
				if owner.Pid <= 0 || !owner.Ref().NativeExact() {
					view.Unreadable++
					file.Close()
					continue
				}
			}
			file.Close()
		}
		if readErr != nil {
			view.Unreadable++
			continue
		}
		held, probeErr := heldByAnother(marker)
		if probeErr != nil {
			view.Unreadable++
			continue
		}
		if !held {
			continue
		}
		if lease {
			if record.Slot != "" {
				view.Held = append(view.Held, hostAdmissionHeld{Owner: record.Owner, Slot: record.Slot,
					Priority: record.Priority, OwnerEnded: identity.AliveRef(prober, record.Owner.Ref()) == identity.Dead})
			}
		} else {
			priority, _, _ := strings.Cut(strings.TrimPrefix(name, "wait-"), "-")
			view.Waiting = append(view.Waiting, hostAdmissionWaiting{Owner: owner, Priority: priority, Since: since})
		}
	}
	return view, nil
}
