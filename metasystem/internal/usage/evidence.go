package usage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// CallEvidence is one complete copy of the evidence used by a context report.
type CallEvidence struct {
	Sessions      []CallSession
	Registrations []CallRegistration
	Samples       []CallSample
	Markers       []Marker
	RetainedSince time.Time
}

// CallStoreBusyError reports that an immediate usage operation could not
// acquire the store-wide maintenance lock.
type CallStoreBusyError struct {
	Path string
}

func (e *CallStoreBusyError) Error() string {
	return fmt.Sprintf("call store is busy: %s", e.Path)
}

var callEvidenceSnapshotStep func(string)

// ReadCallEvidence copies the registry, discovered sessions, and their
// committed rows while excluding concurrent writers and maintenance.
func ReadCallEvidence(stateRoot string) (CallEvidence, error) {
	if !filepath.IsAbs(stateRoot) {
		return CallEvidence{}, fmt.Errorf("state root must be absolute: %s", stateRoot)
	}
	maintenance, err := lockCallMaintenance(stateRoot, true, false)
	if err != nil {
		return CallEvidence{}, err
	}
	defer unlockCallFile(maintenance)
	if _, err := recoverAllCallRetirements(stateRoot); err != nil {
		return CallEvidence{}, err
	}
	retention, err := readCallRetention(stateRoot)
	if err != nil {
		return CallEvidence{}, err
	}

	registrations, _, err := callRegistrationsUnderMaintenance(stateRoot)
	if err != nil {
		return CallEvidence{}, err
	}
	observeCallEvidenceSnapshotStep("registrations")

	sessions, err := callSessionsUnderMaintenance(stateRoot)
	if err != nil {
		return CallEvidence{}, err
	}
	observeCallEvidenceSnapshotStep("sessions")

	evidence := CallEvidence{Sessions: sessions, Registrations: registrations, RetainedSince: retention.RetainedSince}
	for _, session := range sessions {
		samples, markers, readErr := callsUnderMaintenance(stateRoot, session.Runtime, session.Session, time.Time{})
		if readErr != nil {
			return CallEvidence{}, fmt.Errorf(
				"cannot read context session %s/%s cursor=%s samples=%s: %w",
				session.Runtime, session.Session,
				CursorPath(stateRoot, session.Runtime, session.Session),
				SamplesPath(stateRoot, session.Runtime, session.Session), readErr)
		}
		for _, sample := range samples {
			if sample.Runtime != session.Runtime || sample.Session != session.Session {
				return CallEvidence{}, fmt.Errorf(
					"context sample identity %s/%s does not match discovered session %s/%s in %s",
					sample.Runtime, sample.Session, session.Runtime, session.Session,
					SamplesPath(stateRoot, session.Runtime, session.Session))
			}
		}
		for _, marker := range markers {
			if marker.Runtime != session.Runtime || marker.Session != session.Session {
				return CallEvidence{}, fmt.Errorf(
					"context marker identity %s/%s does not match discovered session %s/%s in %s",
					marker.Runtime, marker.Session, session.Runtime, session.Session,
					SamplesPath(stateRoot, session.Runtime, session.Session))
			}
		}
		evidence.Samples = append(evidence.Samples, samples...)
		evidence.Markers = append(evidence.Markers, markers...)
		observeCallEvidenceSnapshotStep("rows")
	}
	return evidence, nil
}

func callMaintenancePath(stateRoot string) string {
	return filepath.Join(stateRoot, "artifacts", "agents", "context", "maintenance.lock")
}

func lockCallMaintenance(stateRoot string, exclusive, nonBlocking bool) (*os.File, error) {
	path := callMaintenancePath(stateRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create call store lock directory: %w", err)
	}
	observeCallOpen(path)
	lock, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("cannot open call store maintenance lock %s: %w", path, err)
	}
	operation := unix.LOCK_SH
	if exclusive {
		operation = unix.LOCK_EX
	}
	if nonBlocking {
		operation |= unix.LOCK_NB
	}
	if err := unix.Flock(int(lock.Fd()), operation); err != nil {
		_ = lock.Close()
		if nonBlocking && (errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return nil, &CallStoreBusyError{Path: path}
		}
		return nil, fmt.Errorf("cannot lock call store maintenance lock %s: %w", path, err)
	}
	return lock, nil
}

func observeCallEvidenceSnapshotStep(step string) {
	if callEvidenceSnapshotStep != nil {
		callEvidenceSnapshotStep(step)
	}
}
