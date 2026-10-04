package usage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
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
func ReadCallEvidence(installationRoot string) (CallEvidence, error) {
	if !filepath.IsAbs(installationRoot) {
		return CallEvidence{}, fmt.Errorf("state root must be absolute: %s", installationRoot)
	}
	maintenance, err := lockCallMaintenance(installationRoot, true, false)
	if err != nil {
		return CallEvidence{}, err
	}
	defer unlockCallFile(maintenance)
	if _, err := recoverAllCallRetirements(installationRoot); err != nil {
		return CallEvidence{}, err
	}
	retention, err := readCallRetention(installationRoot)
	if err != nil {
		return CallEvidence{}, err
	}

	registrations, _, err := callRegistrationsUnderMaintenance(installationRoot)
	if err != nil {
		return CallEvidence{}, err
	}
	observeCallEvidenceSnapshotStep("registrations")

	sessions, err := callSessionsUnderMaintenance(installationRoot)
	if err != nil {
		return CallEvidence{}, err
	}
	observeCallEvidenceSnapshotStep("sessions")

	evidence := CallEvidence{Sessions: sessions, Registrations: registrations, RetainedSince: retention.RetainedSince}
	for _, session := range sessions {
		samples, markers, readErr := callsUnderMaintenance(installationRoot, session.Runtime, session.Session, time.Time{})
		if readErr != nil {
			return CallEvidence{}, fmt.Errorf(
				"cannot read context session %s/%s cursor=%s samples=%s: %w",
				session.Runtime, session.Session,
				CursorPath(installationRoot, session.Runtime, session.Session),
				SamplesPath(installationRoot, session.Runtime, session.Session), readErr)
		}
		for _, sample := range samples {
			if sample.Runtime != session.Runtime || sample.Session != session.Session {
				return CallEvidence{}, fmt.Errorf(
					"context sample identity %s/%s does not match discovered session %s/%s in %s",
					sample.Runtime, sample.Session, session.Runtime, session.Session,
					SamplesPath(installationRoot, session.Runtime, session.Session))
			}
		}
		for _, marker := range markers {
			if marker.Runtime != session.Runtime || marker.Session != session.Session {
				return CallEvidence{}, fmt.Errorf(
					"context marker identity %s/%s does not match discovered session %s/%s in %s",
					marker.Runtime, marker.Session, session.Runtime, session.Session,
					SamplesPath(installationRoot, session.Runtime, session.Session))
			}
		}
		evidence.Samples = append(evidence.Samples, samples...)
		evidence.Markers = append(evidence.Markers, markers...)
		observeCallEvidenceSnapshotStep("rows")
	}
	return evidence, nil
}

func callMaintenancePath(installationRoot string) string {
	return filepath.Join(installationRoot, "artifacts", "agents", "context", "maintenance.lock")
}

// ProbeMaintenance reports whether the maintenance lock is free, probing it
// LOCK_SH|LOCK_NB without creating the lock file or its directory and
// holding nothing afterwards (Part B R15). An absent lock reads as free.
func ProbeMaintenance(installationRoot string) (free bool, err error) {
	file, err := os.OpenFile(callMaintenancePath(installationRoot), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("cannot open call store maintenance lock: %w", err)
	}
	// Unlock before close: a sibling's fork copy would otherwise keep the
	// probe's shared hold after this returned "free".
	defer func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_SH|syscall.LOCK_NB); err != nil {
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return false, nil
		}
		return false, fmt.Errorf("cannot probe call store maintenance lock: %w", err)
	}
	return true, nil
}

func lockCallMaintenance(installationRoot string, exclusive, nonBlocking bool) (*lock.FileLock, error) {
	path := callMaintenancePath(installationRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("cannot create call store lock directory: %w", err)
	}
	observeCallOpen(path)
	mode := lock.Shared
	switch {
	case exclusive && nonBlocking:
		mode = lock.TryExclusive
	case exclusive:
		mode = lock.Exclusive
	case nonBlocking:
		mode = lock.TryShared
	}
	held, err := lock.File(path, 0o644, mode)
	var lockErr *lock.LockError
	switch {
	case err == nil:
		return held, nil
	case !errors.As(err, &lockErr):
		return nil, fmt.Errorf("cannot open call store maintenance lock %s: %w", path, err)
	case nonBlocking && lock.Busy(err):
		return nil, &CallStoreBusyError{Path: path}
	default:
		return nil, fmt.Errorf("cannot lock call store maintenance lock %s: %w", path, err)
	}
}

func observeCallEvidenceSnapshotStep(step string) {
	if callEvidenceSnapshotStep != nil {
		callEvidenceSnapshotStep(step)
	}
}
