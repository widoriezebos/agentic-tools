package batchowner

// After landing push (simple lane, rail 1): every queued member whose head
// the pushed commit contains has landed. It goes the way a pushed batch's
// members always went: the landed-trailer recovery records it landed and
// writes its goal's Next (the goal's conclusion), and the return
// settlement hands the goal back to its seat, or releases it when the seat
// is gone. Members main does not contain stay queued.

import (
	"errors"
	"os/exec"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/ownercall"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// PushedSettlement is one settlement after a push of Commit from the lane
// checkout Checkout, whose installation (ledger) is Install.
type PushedSettlement struct {
	Home, Checkout, Install string
	Commit, Actor           string
	Now                     time.Time
	// Calls are the goal-ledger calls; nil is the production LaneCalls.
	Calls *LaneCallSet
}

// SettlePushed records every queued member whose head request.Commit
// contains as landed and concludes it; it names them. A member an earlier
// settlement recorded landed but could not conclude is concluded now. A
// repeat finds them landed and changes nothing.
func SettlePushed(request PushedSettlement) ([]string, error) {
	store := batch.NewStore(request.Checkout, identity.KernelProber{})
	records, err := store.Records()
	if err != nil {
		return nil, err
	}
	calls := request.Calls
	if calls == nil {
		calls = &LaneCalls
	}
	home := func() (string, error) { return request.Home, nil }
	boundary := laneLedger(home, lane.OpPublish, lane.AuthorityAgent)
	invoke := func() (ownercall.Invocation, error) {
		claim, err := lane.Claim(request.Home)
		if err != nil {
			return ownercall.Invocation{}, err
		}
		invocation := LaneInvocation(claim)
		invocation.Ledger = boundary
		return invocation, nil
	}
	var landed []string
	var problems []error
	for _, record := range records {
		var contained []string
		pushed := record.Landing != nil && record.Landing.PushComplete
		for _, unit := range record.Units {
			// A member an earlier push recorded landed but did not settle
			// (its finalization or return failed) is settled again.
			if pushed && unit.State == batch.UnitReturnPending && unit.Outcome == batch.UnitLanded {
				contained = append(contained, unit.GoalID)
				continue
			}
			if unit.State != batch.UnitJoined {
				continue
			}
			inside, err := containsCommit(request.Checkout, unit.Head(), request.Commit)
			if err != nil {
				return landed, err
			}
			if inside {
				contained = append(contained, unit.GoalID)
			}
		}
		if len(contained) == 0 {
			continue
		}
		if err := store.Update(record.BatchID, func(next *batch.Record) error {
			if next.Landing == nil {
				next.Landing = &batch.LandingProgress{}
			}
			next.Landing.PushComplete, next.Landing.PushedTip = true, request.Commit
			return nil
		}); err != nil {
			return landed, err
		}
		seams := RecoverySeams(request.Checkout, request.Install, "", store, record.BatchID, request.Now, GitOutput, calls, invoke)
		onMain := func(unit batch.Unit) (string, bool, error) {
			return request.Commit, slices.Contains(contained, unit.GoalID), nil
		}
		seams.OriginCommit, seams.OriginChange = onMain, onMain
		seams.OriginSource = func(unit batch.Unit, _ string) (string, bool, error) { return onMain(unit) }
		// The lane checkout is the agent's own: nothing of it is cleaned up.
		seams.Cleanup = func() error { return nil }
		if err := batch.RecoverPushedSeries(store, record.BatchID, request.Actor, request.Now, seams); err != nil {
			problems = append(problems, err)
			continue
		}
		tree, err := fetchLandingBaseTree(request.Checkout)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		returns := returnSeamsAt(request.Checkout, request.Install, func() string { return tree }, calls, boundary, home)
		if _, err := batch.ReturnUnits(store, record.BatchID, tree, request.Actor, request.Now, returns); err != nil {
			problems = append(problems, err)
			continue
		}
		landed = append(landed, contained...)
	}
	return landed, errors.Join(problems...)
}

// containsCommit reports whether commit contains head.
func containsCommit(root, head, commit string) (bool, error) {
	if head == "" {
		return false, nil
	}
	command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", head, commit)
	command.Env = gittree.ScrubbedEnviron()
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	// A head the checkout does not hold is not on main.
	if _, missing := GitOutput(root, "cat-file", "-e", head+"^{commit}"); missing != nil {
		return false, nil
	}
	return false, err
}
