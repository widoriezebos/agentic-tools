package batchowner

// The landing lane's claim authority (lane design r10 K7, K8): the lane
// holds goals under its stable claim identity {lane machine, landing-lane,
// custody epoch of the host record}, which belongs to no session. Its own
// ledger acts carry the custody epoch the kernel reads from the host
// record, and a claim held at an older epoch is renewed to the current one
// before the lane acts on it.

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// LaneInvocation is an owner call made as the lane's claim identity at its
// custody epoch, from this process.
func LaneInvocation(claim lane.ClaimIdentity) ownercall.Invocation {
	invocation := ownercall.FromThisProcess(claim.Lineage)
	invocation.LaneEpoch = int64(claim.Epoch)
	return invocation
}

// claimAuthority is the authority a lane act on goalID runs under: the
// lane's claim identity, renewed to the host record's custody epoch first
// when the ledger holds an older one. The lane acts on no other claim: a
// goal the ledger shows held by anyone else, the deleted batch owner's
// lineage included (design r10 §5, R9-01), is refused with a person's
// release as the way forward. boundary is the publication boundary (K3)
// the act's goal writes go through.
func claimAuthority(controlRoot, goalID string, ledger batch.ReturnLedgerGoal, calls *LaneCallSet, home func() (string, error), boundary func(goal.Endpoint) goal.Endpoint) (ownercall.Invocation, error) {
	if ledger.Lineage != lane.ClaimLineage {
		return ownercall.Invocation{}, notLaneHeld(goalID, ledger)
	}
	dir, err := home()
	if err != nil {
		return ownercall.Invocation{}, fmt.Errorf("this computer's landing lane record can't be found, so goal %s was not given back: %w", goalID, err)
	}
	claim, err := lane.Claim(dir)
	if err != nil {
		return ownercall.Invocation{}, err
	}
	if err := renewLaneClaim(controlRoot, goalID, ledger, claim, dir, calls, boundary); err != nil {
		return ownercall.Invocation{}, err
	}
	invocation := LaneInvocation(claim)
	invocation.Ledger = boundary
	return invocation, nil
}

// notLaneHeld refuses a lane act on a goal the lane does not hold, in two
// lines: the situation, then the release a person runs.
func notLaneHeld(goalID string, ledger batch.ReturnLedgerGoal) error {
	holder := ledger.Machine + "+" + ledger.Lineage
	if ledger.Lineage == lane.OldOwnerLineage {
		holder = "the old landing owner (" + holder + ")"
	}
	return fmt.Errorf("goal %s is held by %s, not by this landing lane, so it was not given back\nrun: metasystem goal release %s --reason TEXT", goalID, holder, goalID)
}

// renewLaneClaim moves a lane-held claim to the lane's current custody
// epoch; a claim already there is left as it is. A claim held on another
// machine, or at a later epoch than the host record's, is not this lane's.
// The renewal is a lane goal write and goes through boundary (K3).
func renewLaneClaim(controlRoot, goalID string, ledger batch.ReturnLedgerGoal, claim lane.ClaimIdentity, home string, calls *LaneCallSet, boundary func(goal.Endpoint) goal.Endpoint) error {
	switch {
	case ledger.Machine != claim.Machine:
		return fmt.Errorf("goal %s is held by the landing lane on %s, not by this computer's lane on %s", goalID, ledger.Machine, claim.Machine)
	case ledger.ClaimEpoch > claim.Epoch:
		return fmt.Errorf("goal %s is held by a later registration of the lane (%d > %d), not by this one", goalID, ledger.ClaimEpoch, claim.Epoch)
	case ledger.ClaimEpoch == claim.Epoch:
		return nil
	}
	invocation := LaneInvocation(claim)
	invocation.Ledger = boundary
	return calls.Handover(invocation, ownercall.HandoverRequest{Root: controlRoot, GoalID: goalID,
		TargetMachine: claim.Machine, TargetLineage: claim.Lineage, TargetEpoch: int64(claim.Epoch), Batch: ledger.Batch, LaneHome: home})
}

// RenewLaneClaims renews every goal member of the batch that the ledger at
// tree shows the lane holding at an older custody epoch to the host
// record's (lane design r10 K7): landing begin runs it before it records
// the series, so every claim the batch lands under is the lane's current
// one. A change member holds no claim.
func RenewLaneClaims(home, checkout, install, batchID, tree string, calls *LaneCallSet) error {
	claim, err := lane.Claim(home)
	if err != nil {
		return err
	}
	// The renewal is the lane's own goal write, made at begin (K3).
	boundary := laneLedger(func() (string, error) { return home, nil }, lane.OpBegin, lane.AuthorityAgent)
	record, err := batch.NewStore(checkout, nil).Load(batchID)
	if err != nil {
		return err
	}
	for _, unit := range record.Units {
		if unit.State != batch.UnitJoined || unit.IsChange() {
			continue
		}
		ledger, err := BatchReturnLedgerGoal(install, tree, unit.GoalID)
		if err != nil {
			return fmt.Errorf("goal %s's ledger entry can't be read before its claim is renewed: %w", unit.GoalID, err)
		}
		if !ledger.Claimed || ledger.Lineage != lane.ClaimLineage || ledger.Batch != batchID {
			return fmt.Errorf("goal %s is not held by the landing lane for batch %s (the ledger shows %s+%s)", unit.GoalID, batchID, ledger.Machine, ledger.Lineage)
		}
		if err := renewLaneClaim(install, unit.GoalID, ledger, claim, home, calls, boundary); err != nil {
			return err
		}
	}
	return nil
}
