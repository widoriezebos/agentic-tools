package act

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
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
//
// It runs under this clone's one publication lock. A live owner is another
// PROCESS, and that test says nothing about a request of this one: an act
// whose push has landed and whose confirmation has not yet returned is, to
// recovery, an entry to classify, and classifying it made the request that
// landed the act answer refused and record no authority proof (Astra D-01).
// So a Refresh that arrives mid-publication waits for it, and then finds the
// entry terminal and touches nothing. The act routes do not come through here:
// they hold the lock for their whole request and run the same rule inside it.
func Reconcile(root string, endpoint goal.Endpoint) error {
	held := ownerOf(root)
	held.publications.Lock()
	defer held.publications.Unlock()
	return reads{}.reconcile(root, endpoint)
}

func (r reads) reconcile(root string, endpoint goal.Endpoint) error {
	blocking, blocked, err := goal.PushedBlocking(root)
	if err != nil {
		return refuse(KindFailed, "journal-unreadable",
			"this clone's record of pending acts could not be read ("+err.Error()+
				"); check the goal before acting again")
	}
	if !blocked {
		return nil
	}
	if endpoint.SplitConfirmed == nil {
		endpoint.SplitConfirmed = func(e goal.Endpoint, tip, parent string, now time.Time) {
			projection, err := goal.ProjectAtEndpoint(e, tip, now)
			if err == nil {
				split := projection.Tree.Live[parent].Split
				err = phase.NotifySplitApproval(context.Background(), e, tip, parent, split.Transaction, split.Children, now)
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
	}
	reports, recoverErr := goal.RecoverWithPolicy(endpoint, recovering{root: root, reads: r})
	if recoverErr != nil {
		return refuse(KindFailed, "pushed-unknown",
			"act "+blocking.Opid+" was sent with its outcome unknown and could not be settled: "+
				recoverErr.Error())
	}
	left, stands, err := goal.PushedBlocking(root)
	if err != nil {
		return refuse(KindFailed, "journal-unreadable",
			"the pending acts could not be read after recovery ("+err.Error()+
				"); check the goal before acting again")
	}
	if !stands {
		return nil
	}
	return refuse(KindFailed, "pushed-unknown",
		"act "+left.Opid+" was sent, its outcome unknown, and recovery left it: "+
			recoveryDetail(reports, left.Opid)+"; nothing changes until it is settled")
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
		"a budget stop is not recovered from the browser; run metasystem goal sync --recover in your terminal")
}
