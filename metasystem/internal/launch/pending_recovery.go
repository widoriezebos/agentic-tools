package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// ResumePending resumes an unclaimed reservation without replacing a recorded process identity.
func (m *Manager) ResumePending(spec StartSpec) (Record, error) {
	record, err := m.Status(spec.ID)
	if err != nil {
		return record, err
	}
	if record.State.Terminal() || record.Supervisor != nil || record.Child != nil || record.ProcessGroup != nil {
		return record, nil
	}
	if record.State != Starting || record.Reason == "cancel-requested" {
		return record, errors.New("the launch reservation cannot start; repeat work stop to finish cancellation")
	}
	if _, err := m.update(spec.ID, func(r *Record) error {
		if r.AdapterData == nil {
			return fmt.Errorf("launch %s has unreadable adapter data", spec.ID)
		}
		setString(r.AdapterData, "unitCancellation", readString(spec.AdapterData, "unitCancellation"))
		return nil
	}); err != nil {
		return Record{}, err
	}
	spec.resume = true
	if err := m.Admit(spec); err != nil {
		return record, err
	}
	if err := m.createAdmitted(spec, record); err != nil {
		return record, err
	}
	state, _ := m.Store.StateDir(spec.ID)
	err = spec.wait(func() error {
		var startErr error
		record, startErr = m.startSupervisor(spec.ID, state)
		return startErr
	})
	return record, err
}

// cancelGate serializes the durable unit stop with child creation and recording.
func cancelGate(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	held, err := lock.File(path+".lock", 0600, lock.Exclusive)
	if err != nil {
		return nil, err
	}
	return func() { _ = held.Release() }, nil
}

func cancellationRequested(path string) error {
	if path == "" {
		return nil
	}
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return errLaunchCancelledBeforeChild
}

// RequestCancel keeps a tombstone even when a running command holds tree custody.
func (runner *UnitRunner) RequestCancel(id string) error {
	record, err := runner.read(id)
	if err != nil {
		return err
	}
	path := filepath.Join(runner.runDir(record.ID), "cancelled")
	release, err := cancelGate(path)
	if err != nil {
		return err
	}
	defer release()
	_, err = atomicfile.WriteText(path, "cancel-requested\n", runner.root())
	return err
}

func (driver stepDriver) pendingCancellation(spec *StartSpec) error {
	if !driver.unit {
		return nil
	}
	path := filepath.Join(filepath.Dir(driver.round.Directory), "cancelled")
	if err := cancellationRequested(path); err != nil {
		return err
	}
	data := make(map[string]json.RawMessage, len(spec.AdapterData)+1)
	for key, value := range spec.AdapterData {
		data[key] = value
	}
	setString(data, "unitCancellation", path)
	spec.AdapterData = data
	return nil
}

func (m *Manager) signalRecordedGroup(record Record, signal syscall.Signal) error {
	if record.ProcessGroup == nil {
		return nil
	}
	if m.Prober != nil && refDead(m.Prober, record.ProcessGroup) {
		return nil
	}
	if m.Prober == nil || !refAlive(m.Prober, record.ProcessGroup) {
		return fmt.Errorf("launch %s process identity is not proven live; no signal sent", record.ID)
	}
	return m.Processes.SignalGroup(record.ProcessGroup.Pid, signal)
}
