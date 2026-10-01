package batchowner

// landing return MEMBER --disposition (lane design r10 K8): one member
// leaves its batch with a typed disposition whose evidence the batch
// records, goes back to its seat under the authority that holds it, and is
// read back before the return counts as done.

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// MemberReturn is one typed return.
type MemberReturn struct {
	Home, Checkout, Install string
	// BatchID is the member's batch; empty finds the one it waits in.
	BatchID, Member, Disposition string
	// Person is the person proven at an enrolled terminal, empty when the
	// landing agent asks.
	Person, Reason, Actor string
	Now                   time.Time
	Calls                 *BatchOwnerCallSet
	// Fetch fetches origin's main into the lane checkout and names its tree.
	Fetch func(checkout string) (string, error)
}

// MemberReturnReport is what the return did and what was read back.
type MemberReturnReport struct {
	Batch       string `json:"batch"`
	Member      string `json:"member"`
	Disposition string `json:"disposition"`
	Evidence    string `json:"evidence"`
	// Settled is the recorded return (handed-back, released,
	// already-returned, recorded); empty while it is not settled.
	Settled   string `json:"settled,omitempty"`
	Confirmed bool   `json:"confirmed"`
	// Unresolved says why the return is not confirmed yet; the same
	// command continues it.
	Unresolved string `json:"unresolved,omitempty"`
	// Repeat says the return was already settled before this call: its
	// effect holds and nothing was written again.
	Repeat bool `json:"repeat,omitempty"`
}

// ReturnMember runs one typed return. The request, with its evidence read
// under the batch lock, is the gated step (K2): the landing agent's return
// is held by a pause, a person's is cleanup and admitted. Settling custody
// already asked for is never refused.
func ReturnMember(request MemberReturn) (MemberReturnReport, error) {
	report := MemberReturnReport{Member: request.Member, Disposition: request.Disposition}
	store := batch.NewStore(request.Checkout, identity.KernelProber{})
	batchID := request.BatchID
	if batchID == "" {
		found, err := memberBatch(store, request.Member, request.Disposition)
		if err != nil {
			return report, err
		}
		batchID = found
	}
	report.Batch = batchID
	fetch := request.Fetch
	if fetch == nil {
		fetch = fetchLandingBaseTree
	}
	if record, err := store.Load(batchID); err == nil {
		for _, unit := range record.Units {
			if unit.GoalID == request.Member && unit.Disposition == request.Disposition && unit.ReturnDisposition != "" {
				report.Repeat, report.Evidence = true, unit.Evidence
			}
		}
	}
	if report.Repeat {
		// The return already settled: a repeat only reads it back, so a
		// pause, which holds new lane work, does not refuse it.
		return confirmMemberReturn(store, request, report, fetch)
	}
	authority := lane.AuthorityAgent
	if request.Person != "" {
		authority = lane.AuthorityPerson
	}
	if err := lane.Gate(request.Home, lane.OpReturn, authority, func(lane.Record) error {
		evidence, err := batch.RequestTypedReturn(store, batchID, request.Member, request.Disposition, request.Person, request.Reason, request.Actor, request.Now)
		report.Evidence = evidence
		return err
	}); err != nil {
		return report, err
	}
	tree, err := fetch(request.Checkout)
	if err != nil {
		report.Unresolved = "origin's main could not be read to give it back: " + err.Error()
		return report, nil
	}
	home := func() (string, error) { return request.Home, nil }
	// The return's goal writes are the lane's and go through its
	// publication boundary (K3) as this return: a person's, which a paused
	// lane admits, or the agent's.
	invoke := func() ownercall.Invocation {
		invocation := ownercall.FromThisProcess(LandingOwnerLineage)
		invocation.Ledger = laneLedger(home, lane.OpReturn, authority)
		return invocation
	}
	seams := returnSeamsAt(request.Checkout, request.Install, func() string { return tree }, request.Calls, invoke, home)
	failures, returnErr := batch.ReturnUnits(store, batchID, tree, request.Actor, request.Now, seams)
	for _, failure := range failures {
		if failure.GoalID == request.Member {
			report.Unresolved = failure.Reason
		}
	}
	if returnErr != nil && report.Unresolved == "" && len(failures) == 0 {
		return report, returnErr
	}
	return confirmMemberReturn(store, request, report, fetch)
}

// memberBatch is the batch member waits in, or is on its way back from;
// else the one it already left with disposition, so a repeat finds it.
func memberBatch(store batch.Store, member, disposition string) (string, error) {
	records, err := store.Records()
	if err != nil {
		return "", err
	}
	var waiting, left []string
	for _, record := range records {
		for _, unit := range record.Units {
			switch {
			case unit.GoalID != member:
			case unit.State == batch.UnitJoining || unit.State == batch.UnitJoined || unit.State == batch.UnitReturnPending:
				waiting = append(waiting, record.BatchID)
			case unit.Disposition == disposition && unit.ReturnDisposition != "":
				left = append(left, record.BatchID)
			}
		}
	}
	if len(waiting) == 0 && len(left) == 1 {
		waiting = left
	}
	switch len(waiting) {
	case 1:
		return waiting[0], nil
	case 0:
		return "", &batch.ReturnRefusal{Code: batch.CodeReturnMemberAbsent, Member: member,
			Message: member + " waits in no landing batch, so nothing was returned"}
	}
	return "", &batch.ReturnRefusal{Code: batch.CodeReturnMemberAbsent, Member: member,
		Message: fmt.Sprintf("%s waits in %d batches (%s), so which one it leaves is not known; nothing was returned", member, len(waiting), strings.Join(waiting, ", "))}
}

// confirmMemberReturn reads the return back: a change by its durable
// disposition, a goal by the ledger on origin's main, fetched again.
func confirmMemberReturn(store batch.Store, request MemberReturn, report MemberReturnReport, fetch func(string) (string, error)) (MemberReturnReport, error) {
	record, err := store.Load(report.Batch)
	if err != nil {
		return report, err
	}
	var unit batch.Unit
	for _, candidate := range record.Units {
		if candidate.GoalID == request.Member && candidate.Disposition == request.Disposition {
			unit = candidate
		}
	}
	report.Settled = unit.ReturnDisposition
	if unit.State == batch.UnitReturnPending || unit.ReturnDisposition == "" {
		if report.Unresolved == "" {
			report.Unresolved = "its return is asked for but not settled yet"
		}
		return report, nil
	}
	if unit.IsChange() {
		report.Confirmed = true
		return report, nil
	}
	tree, err := fetch(request.Checkout)
	if err != nil {
		report.Unresolved = "origin's main could not be read to confirm the return: " + err.Error()
		return report, nil
	}
	ledger, present, err := batch.ReadLedgerGoalAt(request.Install, tree, request.Member)
	switch {
	case err != nil:
		report.Unresolved = "its goal's ledger entry cannot be read: " + strings.TrimSpace(err.Error())
	case present && batch.LaneHolds(ledger, report.Batch, unit):
		report.Unresolved = fmt.Sprintf("the ledger on main still shows %s+%s holding it for this batch", ledger.Machine, ledger.Lineage)
	default:
		report.Confirmed, report.Unresolved = true, ""
	}
	return report, nil
}
