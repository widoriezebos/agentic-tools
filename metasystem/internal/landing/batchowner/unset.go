package batchowner

// landing unset's steps over the lane's layout (design r10 §1): settle,
// reconcile, return at a person's word, and confirm by reading back.
// internal/landing/lane owns the journal, the fence and the order; this file
// binds the steps to the batch store, origin and the ledger. The returns go
// as typed returns with disposition person (K8), each goal given back under
// the authority the ledger shows holding it.

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody/laneprobe"
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
	// Agent names the landing agent launch that runs on this computer, if
	// one does; StopAgent asks it to end. nil reads none.
	Agent     func() (id string, live bool, err error)
	StopAgent func(id string) error
	// Custody are the custody barrier's reads for a layout; nil reads the
	// host.
	Custody func(lane.Layout) custody.Probes
}

// ProductionUnsetLane is landing unset for the person by, on this host.
func ProductionUnsetLane(home, by string) UnsetLane {
	return UnsetLane{Home: home, By: by, Now: func() time.Time { return time.Now().UTC() }, Calls: BatchOwnerCalls}
}

// Seams are the unset's steps.
func (u UnsetLane) Seams() lane.UnsetSeams {
	return lane.UnsetSeams{Settle: u.settle, Override: u.override, Records: u.records, Reconcile: u.reconcile, Return: u.returnBatch, Confirm: u.confirm}
}

// home is the host home the unset works on, for the lane's claim identity.
func (u UnsetLane) home() (string, error) { return u.Home, nil }

func (u UnsetLane) store(layout lane.Layout) batch.Store {
	return batch.NewStore(string(layout.Checkout), identity.KernelProber{})
}

func (u UnsetLane) records(layout lane.Layout) ([]batch.Record, error) {
	return u.store(layout).Records()
}

// settle stops the landing agent, then reads the lane's one custody
// barrier (K9): every execution the kernel launched, the installation's
// proof leases, and the host's proving flock, which a running proof holds.
// A publication admitted before the fence finished under the host flock
// the fence took, and the fence holds every gated lane operation from here
// on (K2), so nothing new starts while it settles.
func (u UnsetLane) settle(layout lane.Layout) (lane.Settlement, error) {
	var settlement lane.Settlement
	if u.Agent != nil {
		id, live, err := u.Agent()
		switch {
		case err != nil:
			settlement.Unknown = append(settlement.Unknown, "whether the landing agent runs is unknown: "+err.Error())
		case live && u.StopAgent == nil:
			settlement.Live = append(settlement.Live, "the landing agent "+id+" runs")
		case live:
			if err := u.StopAgent(id); err != nil {
				settlement.Unknown = append(settlement.Unknown, "the landing agent "+id+" could not be asked to end: "+err.Error())
			} else if after, still, err := u.Agent(); err != nil {
				settlement.Unknown = append(settlement.Unknown, "whether the landing agent ended is unknown: "+err.Error())
			} else if still {
				settlement.Live = append(settlement.Live, "the landing agent "+after+" still runs after it was asked to end")
			}
		}
	}
	held, err := custody.Settle(u.Home, u.probes(layout))
	if err != nil {
		settlement.Unknown = append(settlement.Unknown, "whether landing work runs is unknown: "+err.Error())
	}
	settlement.Live = append(settlement.Live, held.Live...)
	settlement.Unknown = append(settlement.Unknown, held.Unknown...)
	return settlement, nil
}

// personCleanup is the authority a person's unset acts under on the lane's
// goals: the lane's claim identity, with its goal writes through the lane's
// publication boundary as a person's cleanup, which a paused lane admits.
func (u UnsetLane) personCleanup() (ownercall.Invocation, error) {
	claim, err := lane.Claim(u.Home)
	if err != nil {
		return ownercall.Invocation{}, err
	}
	invocation := LaneInvocation(claim)
	invocation.Ledger = u.cleanupBoundary()
	return invocation, nil
}

func (u UnsetLane) cleanupBoundary() func(goal.Endpoint) goal.Endpoint {
	return laneLedger(u.home, lane.OpReturn, lane.AuthorityPerson)
}

func (u UnsetLane) probes(layout lane.Layout) custody.Probes {
	if u.Custody != nil {
		return u.Custody(layout)
	}
	return laneprobe.Production(u.Home, string(layout.Install), true)
}

// override records the person's --force past unknown custody against each
// record it went past.
func (u UnsetLane) override(layout lane.Layout) error {
	_, err := custody.Override(u.Home, u.By, u.probes(layout))
	return err
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

// reconcile finalizes the members of a batch that are on main
// (landed-trailer recovery, design r10 §1 step 3), read from origin's main
// fetched now. A batch whose push reached main without the record saying
// so (the process that pushed died between the two) is recorded pushed when every joined
// member's trailer is on main, and finalized. When only some are, those
// are listed and the batch kept: returning one on main would undo a
// landing, and landing the rest is not the unset's to do.
func (u UnsetLane) reconcile(layout lane.Layout, record batch.Record) ([]lane.Unresolved, error) {
	if !open(record) {
		return nil, nil
	}
	if record.Landing == nil || record.Landing.Base == "" {
		// The batch never began its landing, so nothing of it can be on
		// main: its members are returned.
		return nil, nil
	}
	store := u.store(layout)
	// Only commits after the batch's base count: an earlier landing of the
	// same member, reverted since, is not this batch's push.
	seams := recoverySeamsAt(string(layout.Checkout), string(layout.Install), record.Landing.Base, store, record.BatchID, u.Now(), GitOutput, &u.Calls, u.personCleanup)
	if record.Landing != nil && record.Landing.PushComplete {
		return nil, batch.RecoverPushedSeries(store, record.BatchID, u.By, u.Now(), seams)
	}
	var joined []batch.Unit
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined {
			joined = append(joined, unit)
		}
	}
	if len(joined) == 0 {
		return nil, nil
	}
	if _, err := fetchLandingBaseTree(string(layout.Checkout)); err != nil {
		return nil, err
	}
	var onMain []lane.Unresolved
	tip := ""
	for _, unit := range joined {
		commit, found, err := originTrailer(seams, unit)
		if err != nil {
			return nil, err
		}
		if found {
			// Members replay in their order, so the last one's commit is
			// the pushed tip.
			tip = commit
			onMain = append(onMain, lane.Unresolved{Batch: record.BatchID, Member: unit.GoalID,
				Reason: "its commit " + commit + " is on main, but other members of its batch are not, so it was neither finalized nor returned"})
		}
	}
	switch {
	case len(onMain) == 0:
		return nil, nil
	case len(onMain) < len(joined):
		return onMain, nil
	}
	if err := store.Update(record.BatchID, func(next *batch.Record) error {
		if next.Landing == nil {
			next.Landing = &batch.LandingProgress{}
		}
		next.Landing.PushComplete, next.Landing.PushedTip = true, tip
		return nil
	}); err != nil {
		return nil, err
	}
	return nil, batch.RecoverPushedSeries(store, record.BatchID, u.By, u.Now(), seams)
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
			// A person's return (landing return --disposition person): the
			// person's word is its evidence (K8).
			if _, err := batch.RequestTypedReturn(store, record.BatchID, unit.GoalID, batch.DispositionPerson, u.By, reason, u.By, at); err != nil {
				return nil, err
			}
		}
	}
	tree, err := fetchLandingBaseTree(string(layout.Checkout))
	if err != nil {
		return nil, err
	}
	seams := returnSeamsAt(string(layout.Checkout), string(layout.Install), func() string { return tree }, &u.Calls, u.cleanupBoundary(), u.home)
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
