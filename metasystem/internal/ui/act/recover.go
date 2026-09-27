package act

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// Reconcile classifies this clone's transaction journal, through the engine's
// own recovery rule — the same call `bin/metasystem goal recover` makes, with
// the policy this hand can carry.
//
// It exists because a push can LAND and fail its confirmation. The entry then
// stays at pushed with no outcome, and the engine's exclusion rule is that this
// clone mutates nothing until somebody classifies it. The browser had no way to
// reach that classification: every later act met the engine's refusal, and a
// Refresh that advanced the accepted ref answered a current board without
// touching the journal at all — a successful ledger read is not journal
// recovery. So the two presses a human already has run it: this one, before an
// act publishes, and the board's Refresh before it observes.
//
// Nothing runs unless an entry is actually pushed, which is one local directory
// read: the happy path costs no capture and no remote. What recovery may not
// touch it leaves — a live owner's entry is never taken from it — and the
// refusal then names the entry that stands and the engine's own word for why.
func Reconcile(root string, endpoint goal.Endpoint) error {
	return reads{}.reconcile(root, endpoint)
}

func (r reads) reconcile(root string, endpoint goal.Endpoint) error {
	blocking, blocked, err := goal.PushedBlocking(root)
	if err != nil {
		return refuse(KindFailed, "journal-unreadable",
			"this clone's transaction journal could not be read ("+err.Error()+
				"); check the goal before acting again")
	}
	if !blocked {
		return nil
	}
	reports, recoverErr := goal.RecoverWithPolicy(endpoint, recovering{root: root, reads: r})
	if recoverErr != nil {
		return refuse(KindFailed, "pushed-unknown",
			"journal entry "+blocking.Opid+" is pushed with its outcome unknown and could not be classified: "+
				recoverErr.Error())
	}
	left, stands, err := goal.PushedBlocking(root)
	if err != nil {
		return refuse(KindFailed, "journal-unreadable",
			"this clone's transaction journal could not be read after recovery ("+err.Error()+
				"); check the goal before acting again")
	}
	if !stands {
		return nil
	}
	return refuse(KindFailed, "pushed-unknown",
		"journal entry "+left.Opid+" is pushed with its outcome unknown and recovery left it: "+
			recoveryDetail(reports, left.Opid)+"; this clone mutates nothing until it is classified")
}

// recoveryDetail is the engine's own word about one entry, taken from the
// report recovery just wrote rather than restated here.
func recoveryDetail(reports []goal.RecoveryReport, opid string) string {
	for _, report := range reports {
		if report.Opid == opid {
			if report.Detail == "" {
				return string(report.Action)
			}
			return report.Detail
		}
	}
	return "recovery reported nothing about it"
}

// recovering is the policy a browser session carries into the engine's
// recovery.
//
// The park branch check is the one live precondition journal text cannot
// reconstruct, and it is the reading this hand already carries on every request,
// so a recovered park is judged exactly as a live one is. A breach-stop's
// precondition is not one this hand has: it needs the live budget projection
// under the goal-revision lock, which is the dispatch edge's, so the policy says
// so and names the terminal verb that has it.
type recovering struct {
	root  string
	reads reads
}

func (p recovering) ParkBranchCheck(endpoint goal.Endpoint) func(goalID, next string) (string, error) {
	return p.reads.parkBranchCheck(p.root, endpoint)
}

func (p recovering) BreachStop(goal.Endpoint, goal.Entry) (goal.PublishRequest, func(), error) {
	return goal.PublishRequest{}, nil, fmt.Errorf(
		"a breach-stop is not recovered from the interface: it needs the live budget projection under the goal-revision lock; run bin/metasystem goal recover from your own terminal")
}
