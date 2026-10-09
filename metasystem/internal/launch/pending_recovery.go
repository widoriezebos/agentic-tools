package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// ResumePending resumes an unclaimed reservation without replacing a recorded process identity.
func (m *Manager) ResumePending(spec StartSpec) (record Record, err error) {
	record, err = m.Status(spec.ID)
	if err != nil || record.State.Terminal() || record.Supervisor != nil || record.Child != nil || record.ProcessGroup != nil {
		return record, err
	}
	defer func() {
		_, clearErr := m.update(spec.ID, func(r *Record) error { delete(r.AdapterData, "unitPersonInvocation"); return nil })
		err = errors.Join(err, clearErr)
	}()
	if record.State != Starting || record.Reason == "cancel-requested" {
		return record, errors.New("the launch reservation cannot start; repeat work stop to finish cancellation")
	}
	spec.resume = true
	if err := m.Admit(spec); err != nil {
		return record, err
	}
	if err := m.createAdmitted(spec, record); err != nil {
		return record, err
	}
	state, _ := m.Store.StateDir(spec.ID)
	err = spec.wait(func() (startErr error) {
		if _, err := m.update(spec.ID, func(r *Record) error {
			if r.AdapterData == nil {
				return fmt.Errorf("launch %s has unreadable adapter data", spec.ID)
			}
			setString(r.AdapterData, "unitCancellation", readString(spec.AdapterData, "unitCancellation"))
			if context, present := spec.AdapterData["unitAuthorityRoot"]; present {
				r.AdapterData["unitAuthorityRoot"] = context
			}
			r.AdapterData["unitPersonInvocation"] = spec.AdapterData["unitPersonInvocation"]
			if strings.HasPrefix(r.Reason, "authority-held: ") {
				r.Reason = ""
			}
			return nil
		}); err != nil {
			return err
		}
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

func (runner *UnitRunner) checkPendingIdentity(record *UnitRunRecord) error {
	for r := range record.Rounds {
		round := &record.Rounds[r]
		for i := range round.Steps {
			step := &round.Steps[i]
			act := step.PendingAct
			if act == nil {
				continue
			}
			invalid := func(reason string) error {
				act.Code, act.Reason = "UNIT_PENDING_INVALIDATED", reason
				runner.driver(record, round).closeCapacityWait(i, "invalidated", runner.Manager.Now().UTC().Format(time.RFC3339Nano))
				if err := runner.save(*record); err != nil {
					return err
				}
				return coded(act.Code, "run="+record.ID, errors.New(reason))
			}
			if act.Code == "UNIT_PENDING_INVALIDATED" {
				return coded(act.Code, "run="+record.ID, errors.New(act.Reason))
			}
			if step.State != StepStarting {
				continue
			}
			execution, err := runner.Manager.Store.Read(step.LaunchID)
			if err != nil && !os.IsNotExist(err) {
				return err
			}
			if err == nil && (execution.State.Terminal() || execution.Supervisor != nil || execution.Child != nil || execution.ProcessGroup != nil) {
				continue
			}
			if runner.named != nil {
				if err := runner.named.verify(runner); IsCode(err, "UNIT_NAMED_INPUT_CHANGED") {
					return invalid(err.Error())
				} else if err != nil {
					return err
				}
			}
			git := runner.Git
			if git == nil {
				git = OSGitRunner{}
			}
			worktree, base, args := act.Worktree, act.Base, []string{"rev-parse", "HEAD"}
			for _, revision := range record.Revisions {
				if revision.Attempt == round.Number && revision.Rebase != nil {
					worktree, base = revision.Rebase.Worktree, revision.Rebase.Base
					args = []string{"rev-parse", "--verify", "HEAD"}
				}
			}
			head, err := git.Run(worktree, nil, args...)
			if err != nil {
				return err
			}
			if strings.TrimSpace(string(head)) != base {
				return invalid("the queued build's target changed; prepare fresh work at the current branch tip")
			}
			registration := runner.Manager.CapacitySources.Registration
			if registration == nil {
				registration = lane.Read
			}
			owner, known, err := registration(runner.Manager.CapacityHome)
			if err != nil {
				return err
			}
			if act.Registration != (lane.Record{}) && (!known || owner != act.Registration) {
				return invalid("the queued build's host registration changed; prepare fresh work under the current registration")
			}
		}
	}
	return nil
}
