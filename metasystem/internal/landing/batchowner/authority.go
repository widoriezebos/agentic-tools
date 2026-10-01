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

// claimAuthority is the authority a lane act on goalID runs under, chosen by
// who the ledger shows holding it: the lane's claim identity (renewed to the
// host record's custody epoch first when the ledger holds an older one), or
// for a claim the old owner lineage holds, that owner's own authority: its
// claims are settled through the old authority (design r10 §5, R9-01).
// invoke is the act's own context: the old owner's authority as it is, and
// for the lane's claim identity the publication boundary (K3) its goal
// writes go through.
func claimAuthority(controlRoot, goalID string, ledger batch.ReturnLedgerGoal, calls *BatchOwnerCallSet, home func() (string, error), invoke func() ownercall.Invocation) (ownercall.Invocation, error) {
	act := invoke()
	if ledger.Lineage != lane.ClaimLineage {
		return act, nil
	}
	dir, err := home()
	if err != nil {
		return ownercall.Invocation{}, fmt.Errorf("this computer's landing lane record can't be found, so goal %s was not given back: %w", goalID, err)
	}
	claim, err := lane.Claim(dir)
	if err != nil {
		return ownercall.Invocation{}, err
	}
	if err := renewLaneClaim(controlRoot, goalID, ledger, claim, dir, calls, act.Ledger); err != nil {
		return ownercall.Invocation{}, err
	}
	invocation := LaneInvocation(claim)
	invocation.Ledger = act.Ledger
	return invocation, nil
}

// renewLaneClaim moves a lane-held claim to the lane's current custody
// epoch; a claim already there is left as it is. A claim held on another
// machine, or at a later epoch than the host record's, is not this lane's.
// The renewal is a lane goal write and goes through boundary (K3).
func renewLaneClaim(controlRoot, goalID string, ledger batch.ReturnLedgerGoal, claim lane.ClaimIdentity, home string, calls *BatchOwnerCallSet, boundary func(goal.Endpoint) goal.Endpoint) error {
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

// RenewLaneClaims renews every goal member of the batch that the ledger on
// main (tree reads it, fetched once and only when the batch has a goal
// member) shows the lane holding at an older custody epoch to the host
// record's (lane design r10 K7), and names the members it renewed. landing
// begin runs it before it records the series: a renewal is a lane goal
// write on main (K3), so the series is composed again on the new main. A
// change member holds no claim.
func RenewLaneClaims(home, checkout, install, batchID string, tree func() (string, error), calls *BatchOwnerCallSet) ([]string, error) {
	record, err := batch.NewStore(checkout, nil).Load(batchID)
	if err != nil {
		return nil, err
	}
	var goals []batch.Unit
	for _, unit := range record.Units {
		if unit.State == batch.UnitJoined && !unit.IsChange() {
			goals = append(goals, unit)
		}
	}
	if len(goals) == 0 {
		return nil, nil
	}
	claim, err := lane.Claim(home)
	if err != nil {
		return nil, err
	}
	at, err := tree()
	if err != nil {
		return nil, fmt.Errorf("the ledger on main can't be read before the lane's claims are renewed: %w", err)
	}
	// The renewal is the lane's own goal write, made at begin (K3).
	boundary := laneLedger(func() (string, error) { return home, nil }, lane.OpBegin, lane.AuthorityAgent)
	var renewed []string
	for _, unit := range goals {
		ledger, err := BatchReturnLedgerGoal(install, at, unit.GoalID)
		if err != nil {
			return renewed, fmt.Errorf("goal %s's ledger entry can't be read before its claim is renewed: %w", unit.GoalID, err)
		}
		if !ledger.Claimed || ledger.Lineage != lane.ClaimLineage || ledger.Batch != batchID {
			return renewed, fmt.Errorf("goal %s is not held by the landing lane for batch %s (the ledger shows %s+%s)", unit.GoalID, batchID, ledger.Machine, ledger.Lineage)
		}
		if ledger.Machine == claim.Machine && ledger.ClaimEpoch == claim.Epoch {
			continue
		}
		if err := renewLaneClaim(install, unit.GoalID, ledger, claim, home, calls, boundary); err != nil {
			return renewed, err
		}
		renewed = append(renewed, unit.GoalID)
	}
	return renewed, nil
}

// FetchLaneMainTree fetches origin's main into the lane checkout and names
// its tree: where the lane reads the ledger it renews against.
func FetchLaneMainTree(checkout string) (string, error) { return fetchLandingBaseTree(checkout) }
