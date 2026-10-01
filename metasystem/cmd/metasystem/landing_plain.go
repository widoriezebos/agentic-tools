package main

// The plain landing lane (plain-lane design r2), the command side: work
// land's hand-in, and the in-process conclusion of a goal landing push
// settles. The records and rails are internal/landing/plain's. The lane
// path calls none of the older lane's batch records, join planning,
// receipt worktrees, custody or lane-side admission.

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// laneInstallOf is the installation of the lane checkout at root.
func (inv *intentInvocation) laneInstallOf(root string) (string, error) {
	if read := inv.delivery().laneInstall; read != nil {
		return read(root)
	}
	layout, err := lane.NewLayout(root)
	if err != nil {
		return "", err
	}
	return string(layout.Install), nil
}

// laneQueueState answers work land G from the lane's queue when the goal's
// newest hand-in is at sha (or its branch is gone): waiting, returned with
// its reason, or landed. nil when it has none, or one at another sha, so
// the hand-in goes on.
func (inv *intentInvocation) laneQueueState(targets []intentTarget, install, goalID, sha string) *intentResult {
	entry, ok, err := plain.Latest(install, goalID)
	if err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing lane's queue can't be read, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !ok || (sha != "" && entry.SHA != sha) {
		return nil
	}
	data := map[string]any{"route": "lane", "queue": entry}
	switch entry.State {
	case plain.StateLanded:
		return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("goal %s landed on main through the landing lane (%s)", goalID, plain.Short(entry.Main))}
	case plain.StateReturned:
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("goal %s at %s was returned: %s", goalID, plain.Short(entry.SHA), entry.Reason),
			next:    inv.sameCommand(), nextReason: "after the fix is pushed to " + entry.Branch + ", hands it in again"}
	}
	return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
		Summary: fmt.Sprintf("goal %s at %s is waiting in the landing lane; its landing agent proves and pushes it", goalID, plain.Short(entry.SHA))}
}

// handIn appends the goal's branch at sha to the lane's queue: the seat's
// gates passed before it. The branch is read at origin, so it is already
// pushed. A repeat at the same sha appends nothing.
func (inv *intentInvocation) handIn(targets []intentTarget, install, goalID, sha string) intentResult {
	now := inv.delivery().now()
	line := plain.Line{Goal: goalID, Branch: "goal/" + goalID, SHA: sha, Seat: batchowner.LandingLaneRegistrant(inv.layout.InstallationRoot), At: now.UTC().Format(time.RFC3339)}
	entry, added, err := plain.HandIn(install, line)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing lane's queue can't be written, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !added {
		return *inv.laneQueueState(targets, install, goalID, sha)
	}
	return intentResult{Targets: targets, Outcome: intentConfirmed, Data: map[string]any{"route": "lane", "queue": entry},
		Summary: fmt.Sprintf("goal %s at %s handed to the lane; its landing agent proves and pushes it", goalID, plain.Short(sha)),
		next:    inv.sameCommand(), nextReason: "shows whether it waits, landed or was returned"}
}

// plainLaneDone concludes a goal landing push settled, in this process, on
// the lane installation's ledger: goal done as the pair that holds the
// claim (the claim stays with its seat), else as the lane's machine for a
// goal nobody holds. When the ledger refuses a done that needs a person
// (open read items, review obligations), the holder's landed line is
// recorded instead, as a seat's own landing records it, and the goal waits
// for its person's goal done.
func plainLaneDone(install string) func(plain.Entry, string) (string, error) {
	return func(entry plain.Entry, main string) (string, error) {
		endpoint, err := goal.ResolveEndpoint(install)
		if err != nil {
			return "", err
		}
		now := time.Now().UTC()
		projection, err := goal.Project(endpoint, true, now)
		if err != nil {
			return "", err
		}
		if file, archived := projection.Tree.Archived(entry.Goal); archived && file.State == goal.StateDone {
			return "done", nil
		}
		file := projection.Tree.Live[entry.Goal]
		if file == nil {
			return "", fmt.Errorf("goal %s is neither live nor done on the ledger", entry.Goal)
		}
		actor := goal.Actor{Lineage: lane.ClaimLineage}
		if actor.Machine, err = goal.ResolveMachine(install); err != nil {
			return "", err
		}
		if file.State == goal.StateClaimed && file.Claimed != nil {
			actor = goal.Actor{Machine: file.Claimed.Machine, Lineage: file.Claimed.Lineage}
		}
		request := func() (goal.VerbRequest, error) {
			ulid, err := goal.NewOperationULID()
			return goal.VerbRequest{Endpoint: endpoint, Actor: actor, Ulid: ulid, Now: now}, err
		}
		done, err := request()
		if err != nil {
			return "", err
		}
		result, doneErr := goal.Done(done, entry.Goal, "landed on main as "+plain.Short(main)+" through the landing lane")
		if doneErr == nil && (result.Outcome == goal.OutcomeConfirmed || result.Outcome == goal.OutcomeConfirmedLate || result.Unchanged) {
			return "done", nil
		}
		if doneErr == nil {
			doneErr = errors.New(result.Detail)
		}
		landed, err := request()
		if err != nil {
			return "", err
		}
		result, landedErr := goal.RecordLanded(landed, entry.Goal, "the landing lane, main "+plain.Short(main))
		if landedErr == nil && (result.Outcome == goal.OutcomeConfirmed || result.Unchanged) {
			return "landed; goal done waits for a person: " + oneLine(doneErr.Error()), nil
		}
		if landedErr == nil {
			landedErr = errors.New(result.Detail)
		}
		return "", fmt.Errorf("goal %s could not be done (%v) nor recorded landed (%v)", entry.Goal, doneErr, strings.TrimSpace(landedErr.Error()))
	}
}
