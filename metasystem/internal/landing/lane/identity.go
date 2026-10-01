package lane

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The lane's lineages (design r10 K7). ClaimLineage is the stable identity
// a lane holds goal claims under: it belongs to no session, so joins,
// custody and queuing work while no agent runs. AgentLineage is the
// landing agent's launch lineage: descent from it is required only for the
// kernel operations the agent issues.
const (
	ClaimLineage = goal.LaneClaimLineage
	AgentLineage = "landing-agent"
)

// OldOwnerLineage is the lineage the deleted batch owner held claims under
// (design r10 §5, migration step 0, Astra R9-01). Nothing acts under it any
// more: it is named only so the cutover refuses while the ledger still
// shows a goal claimed under it, and a person releases each such goal.
const OldOwnerLineage = "landing-m1l"

// CodeClaimUnnamed is a lane whose checkout has no machine nickname, so no
// claim can name it.
const CodeClaimUnnamed = "LANDING_LANE_UNNAMED"

// ClaimIdentity is the lane's claim identity: {lane machine, lineage
// landing-lane, custody epoch of the host record}. It is independent of any
// live session.
type ClaimIdentity struct {
	// Root is the lane checkout the record registers.
	Root    string
	Machine string
	Lineage string
	Epoch   uint64
}

// Actor is the lane's actor on batch records and on what it lands, lane:<id>;
// a session id is audit only.
func (identity ClaimIdentity) Actor() string { return AccountID(identity.Root) }

// Claim reads the host's lane record and names the lane's claim identity.
// No record, an unreadable or older record, or a checkout without a machine
// nickname is refused.
func Claim(home string) (ClaimIdentity, error) {
	record, ok, err := Read(home)
	if err != nil {
		return ClaimIdentity{}, err
	}
	if !ok {
		return ClaimIdentity{}, &Refusal{Code: CodeNotRegistered, Message: "no landing lane is registered on this computer, so it holds no claims",
			Fix: "a person registers the landing checkout: metasystem landing set PATH"}
	}
	return ClaimOf(record)
}

// ClaimOf is the claim identity of a lane record already read.
func ClaimOf(record Record) (ClaimIdentity, error) {
	machine, err := goal.ResolveMachine(record.Root)
	if err != nil {
		return ClaimIdentity{}, &Refusal{Code: CodeClaimUnnamed,
			Message: fmt.Sprintf("the landing lane %s has no machine nickname, so it can hold no claim; nothing was done", record.Root),
			Fix:     "name it once: git -C " + record.Root + " config metasystem.goal.machine NAME",
			Argv:    []string{"git", "-C", record.Root, "config", "metasystem.goal.machine", "NAME"}}
	}
	return ClaimIdentity{Root: record.Root, Machine: machine, Lineage: ClaimLineage, Epoch: record.CustodyEpoch}, nil
}
