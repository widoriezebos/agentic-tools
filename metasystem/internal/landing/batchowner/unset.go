package batchowner

// landing unset's steps over the lane's layout (design r10 §1): settle,
// reconcile, return at a person's word, and confirm by reading back.
// internal/landing/lane owns the journal, the fence and the order; this file
// binds the steps to the batch store, origin and the ledger. The returns go
// through the existing return functions with a person's authority; K-d
// replaces Return's body with typed evidence per disposition.

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// UnsetLane is one person's landing unset: who, when, and the seams its
// steps reach beyond the lane's own records.
type UnsetLane struct {
	Home string
	By   string
	Now  func() time.Time
	// Calls publish the returns' goal changes.
	Calls BatchOwnerCallSet
	// Probe reads whether the lane's owner runs; End ends it by its
	// recorded identity.
	Probe func(root string) (lane.OwnerProbe, error)
	End   func(root string) (int64, error)
}

// ProductionUnsetLane is landing unset for the person by, on this host.
func ProductionUnsetLane(home, by string) UnsetLane {
	return UnsetLane{Home: home, By: by, Now: func() time.Time { return time.Now().UTC() }, Calls: BatchOwnerCalls,
		Probe: LandingLaneOwnerProbe, End: EndLaneOwner}
}

// Seams are the unset's steps.
func (u UnsetLane) Seams() lane.UnsetSeams {
	return lane.UnsetSeams{Settle: u.settle, Records: u.records, Reconcile: u.reconcile, Return: u.returnBatch, Confirm: u.confirm}
}

func (u UnsetLane) store(layout lane.Layout) batch.Store {
	return batch.NewStore(string(layout.Checkout), identity.KernelProber{})
}

func (u UnsetLane) records(layout lane.Layout) ([]batch.Record, error) {
	return u.store(layout).Records()
}

// settle lets a publication the owner is making finish, then ends the
// owner, and reads the host's proving flock, which a running proof holds.
// The pause the fence set keeps the keeper from starting another.
func (u UnsetLane) settle(layout lane.Layout) (lane.Settlement, error) {
	var settlement lane.Settlement
	checkout := string(layout.Checkout)
	probe, err := u.Probe(checkout)
	if err != nil {
		settlement.Unknown = append(settlement.Unknown, "whether the lane's owner runs is unknown: "+err.Error())
	}
	if probe.Alive {
		records, err := u.records(layout)
		if err != nil {
			return settlement, err
		}
		for _, record := range records {
			if record.State == batch.StateLanding {
				settlement.Live = append(settlement.Live, "batch "+record.BatchID+" is publishing to main")
			}
		}
		if len(settlement.Live) == 0 {
			if _, err := u.End(checkout); err != nil {
				settlement.Unknown = append(settlement.Unknown, "the lane's owner could not be ended: "+err.Error())
			}
		}
	}
	holder, busy, err := lane.ProbeProving(u.Home)
	switch {
	case err != nil:
		settlement.Unknown = append(settlement.Unknown, "whether a proof runs is unknown: "+err.Error())
	case busy:
		settlement.Live = append(settlement.Live, "a proof runs ("+holder+")")
	}
	return settlement, nil
}

// open reports whether the batch may still hold members to settle.
func open(record batch.Record) bool {
	return record.State != batch.StateLanded && record.State != batch.StateDissolved
}

// member reports whether the unit is still the lane's: joining, joined or
// on its way back.
func member(unit batch.Unit) bool {
	return unit.State == batch.UnitJoining || unit.State == batch.UnitJoined || unit.State == batch.UnitReturnPending
}

// reconcile finalizes the members of a batch whose push completed and are
// on main (landed-trailer recovery). A member of a batch with no completed
// push whose commit is on main anyway is listed and its batch kept: a
// return would undo a landing.
func (u UnsetLane) reconcile(layout lane.Layout, record batch.Record) ([]lane.Unresolved, error) {
	if !open(record) {
		return nil, nil
	}
	store := u.store(layout)
	seams := recoverySeamsAt(string(layout.Checkout), string(layout.Install), store, record.BatchID, u.Now(), GitOutput, &u.Calls)
	if record.Landing != nil && record.Landing.PushComplete {
		// The lane is going away: its checkout's engine is left as it is.
		seams.Rearm = func(string) error { return nil }
		return nil, batch.RecoverPushedSeries(store, record.BatchID, u.By, u.Now(), seams)
	}
	var unresolved []lane.Unresolved
	for _, unit := range record.Units {
		if unit.State != batch.UnitJoined {
			continue
		}
		commit, found, err := originTrailer(seams, unit)
		if err != nil {
			return nil, err
		}
		if found {
			unresolved = append(unresolved, lane.Unresolved{Batch: record.BatchID, Member: unit.GoalID,
				Reason: "its commit " + commit + " is on main, but its batch has no completed push to finalize it from"})
		}
	}
	return unresolved, nil
}

func originTrailer(seams batch.RecoverySeams, unit batch.Unit) (string, bool, error) {
	switch {
	case unit.IsChange():
		return seams.OriginChange(unit)
	case len(unit.CommitIDs) != 0:
		return seams.OriginSource(unit, unit.CommitIDs[len(unit.CommitIDs)-1])
	}
	return seams.OriginCommit(unit)
}

// returnBatch returns the batch's remaining members at the person's word:
// joined members are withdrawn with reason, custody already returning keeps
// the outcome it records, and a goal member whose return settled while the
// ledger on main still shows the lane holding it is released again. It is
// admitted by the lane's gate as a person's cleanup, while paused and
// fenced.
func (u UnsetLane) returnBatch(layout lane.Layout, record batch.Record, reason string) ([]lane.Unresolved, error) {
	if !open(record) && !slices.ContainsFunc(record.Units, member) {
		return nil, nil
	}
	if err := lane.Gate(u.Home, lane.OpReturn, lane.AuthorityPerson, nil); err != nil {
		return nil, err
	}
	store, at := u.store(layout), u.Now()
	// Goal members an earlier unset already returned: when main still
	// shows the lane holding one, its release is issued again.
	var returnedBefore []string
	for _, unit := range record.Units {
		if !unit.IsChange() && !member(unit) && unit.State != batch.UnitLanded {
			returnedBefore = append(returnedBefore, unit.GoalID)
		}
	}
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoining || unit.State == batch.UnitJoined {
			if err := batch.RequestReturn(store, record.BatchID, unit.GoalID, batch.UnitWithdrawn, reason, u.By, at); err != nil {
				return nil, err
			}
		}
	}
	tree, err := fetchLandingBaseTree(string(layout.Checkout))
	if err != nil {
		return nil, err
	}
	seams := returnSeamsAt(string(layout.Checkout), string(layout.Install), func() string { return tree }, &u.Calls)
	failures, returnErr := batch.ReturnUnits(store, record.BatchID, tree, u.By, at, seams)
	var unresolved []lane.Unresolved
	for _, failure := range failures {
		unresolved = append(unresolved, lane.Unresolved{Batch: record.BatchID, Member: failure.GoalID, Reason: failure.Reason})
	}
	if returnErr != nil && len(failures) == 0 {
		return unresolved, returnErr
	}
	for _, unit := range record.Units {
		if !slices.Contains(returnedBefore, unit.GoalID) {
			continue
		}
		ledger, present, err := batch.ReadLedgerGoalAt(string(layout.Install), tree, unit.GoalID)
		if err != nil || !present || !batch.LaneHolds(ledger, record.BatchID, unit) {
			continue
		}
		next := unit.Outcome + ": " + unit.Failure
		if err := seams.Release(unit.GoalID, next); err != nil {
			unresolved = append(unresolved, lane.Unresolved{Batch: record.BatchID, Member: unit.GoalID, Reason: "its release was issued again and failed: " + err.Error()})
		}
	}
	return unresolved, nil
}

// confirm reads every member back after the returns: a change by its
// durable disposition, a goal by the ledger on origin's main, fetched now.
// A landed member counts once it is finalized. A batch that landed or
// dissolved was settled member by member before; only its members still in
// it are read.
func (u UnsetLane) confirm(layout lane.Layout, records []batch.Record) ([]lane.Unresolved, error) {
	var unresolved []lane.Unresolved
	tree := ""
	for _, record := range records {
		for _, unit := range record.Units {
			if !open(record) && !member(unit) {
				continue
			}
			list := func(reason string) {
				unresolved = append(unresolved, lane.Unresolved{Batch: record.BatchID, Member: unit.GoalID, Reason: reason})
			}
			switch {
			case member(unit):
				list(fmt.Sprintf("it is still in the batch (%s)", unit.State))
				continue
			case unit.State == batch.UnitLanded:
				if !unit.P6Done {
					list("it landed but is not finalized yet")
				}
				continue
			case unit.IsChange():
				if unit.ReturnDisposition == "" {
					list("its return has no recorded disposition")
				}
				continue
			}
			if tree == "" {
				fetched, err := fetchLandingBaseTree(string(layout.Checkout))
				if err != nil {
					return nil, err
				}
				tree = fetched
			}
			ledger, present, err := batch.ReadLedgerGoalAt(string(layout.Install), tree, unit.GoalID)
			switch {
			case err != nil:
				list("its goal's ledger entry cannot be read: " + strings.TrimSpace(err.Error()))
			case present && batch.LaneHolds(ledger, record.BatchID, unit):
				list(fmt.Sprintf("the ledger on main still shows %s+%s holding it for this batch", ledger.Machine, ledger.Lineage))
			}
		}
	}
	return unresolved, nil
}
