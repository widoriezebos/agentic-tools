package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

type landingPendingBinding struct {
	Registration lane.Record `json:"registration"`
	Batch        string      `json:"batch"`
}

func unclaimedLanding(record launch.Record) bool {
	return record.State == launch.Starting && record.Supervisor == nil && record.Child == nil && record.ProcessGroup == nil
}

// unclaimed lets only the keeper's start decision reach pending recovery.
// An unreadable reservation stays running until its state can be established.
func (a landingAgent) unclaimed(id string) bool {
	record, err := a.manager().Store.Read(id)
	return err == nil && record.Kind == launch.LandingKind && unclaimedLanding(record)
}

// resumePending keeps the reservation's inputs and settings; current lane
// admission and the supervisor's atomic claim still own permission to start.
func (a landingAgent) resumePending(root string, wake lane.Wake) (string, bool, error) {
	manager := a.manager()
	records, err := manager.List()
	if err != nil {
		return "", false, err
	}
	var pending *launch.Record
	for i := range records {
		record := &records[i]
		if record.Kind != launch.LandingKind || record.State.Terminal() {
			continue
		}
		if !unclaimedLanding(*record) || pending != nil {
			return "", true, errors.New("another landing reservation has not ended; no recovery starts")
		}
		pending = record
	}
	if pending == nil {
		return "", false, nil
	}
	hold := func(reason string) (string, bool, error) {
		return "", true, &lane.StartHeld{Reason: reason, Reservation: pending.ID, Permanent: true}
	}
	if pending.Reason == "cancel-requested" {
		return hold("cancellation is requested")
	}
	var binding landingPendingBinding
	if err := json.Unmarshal(pending.AdapterData["landingPendingBinding"], &binding); err != nil || binding.Registration.CustodyEpoch == 0 {
		return hold("its retained lane registration cannot be read")
	}
	registration, present, err := lane.Read(a.home)
	if err != nil || !present {
		return hold(fmt.Sprintf("the current lane registration cannot be read (%v)", err))
	}
	if registration != binding.Registration || pending.WorkingDirectory != root || binding.Batch != wake.BatchID {
		return hold("its lane registration or selected batch changed")
	}
	var brief, fence string
	if err := json.Unmarshal(pending.AdapterData["brief"], &brief); err != nil {
		return hold("its retained brief cannot be read")
	}
	if err := json.Unmarshal(pending.AdapterData["fenceRoot"], &fence); err != nil || fence != registration.Install {
		return hold("its retained process fence does not name the registered installation")
	}
	resumed, err := manager.ResumePending(pending.ID)
	return resumed.ID, true, err
}
