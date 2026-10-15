package launch

import (
	"errors"
	"fmt"
)

// ResumePending starts the supervisor of an unclaimed landing reservation.
// The reservation owns its settings and inputs; the current lane admission
// and the supervisor's atomic claim still own permission to create a child.
func (m *Manager) ResumePending(id string) (Record, error) {
	record, err := m.Status(id)
	if err != nil {
		return record, err
	}
	if record.Kind != LandingKind || record.State != Starting || record.Supervisor != nil || record.Child != nil || record.ProcessGroup != nil {
		return record, fmt.Errorf("launch %s is not an unclaimed landing reservation", record.ID)
	}
	if record.Reason == "cancel-requested" {
		return record, errors.New("the launch reservation cannot start; repeat work stop to finish cancellation")
	}
	if m.Supervisor == nil {
		return record, errors.New("launch supervisor is unavailable")
	}
	// Admission uses the retained operation, never replacement inputs from
	// the caller or settings from another installation.
	fenceRoot := readString(record.AdapterData, "fenceRoot")
	spec := StartSpec{ID: record.ID, Kind: record.Kind, WorkingDirectory: record.WorkingDirectory,
		FenceRoot: fenceRoot, resume: true}
	if err := m.admitOnLane(spec); err != nil {
		return record, err
	}
	if fenceRoot == "" {
		return record, errors.New(landingNoFenceRoot)
	}
	if reason := seatFenceClosed(fenceRoot); reason != "" {
		return record, errors.New(reason)
	}
	state, err := m.Store.StateDir(record.ID)
	if err != nil {
		return record, err
	}
	return m.startSupervisor(record.ID, state)
}
