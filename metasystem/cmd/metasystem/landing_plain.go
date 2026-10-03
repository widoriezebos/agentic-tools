package main

// The plain landing lane (plain-lane design r3), the seat's side: work
// land's hand-in and what the seat reads back. The records and rails are
// internal/landing/plain's. A hand-in is landed when main contains its sha,
// derived here from the seat's own view of main; the seat concludes its
// goal itself.

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

// laneInstallOf is the installation of the lane checkout at root, where
// its records live.
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
// its reason, or landed when main (the seat's endpoint tip) contains it.
// nil when it has none, or one at another sha, so the hand-in goes on. A
// waiting or landed goal the ledger shows unmarked is marked, as its hand-in
// would have marked it, so its wait stays off its clock; a landed goal whose
// mark would be refused reads landed without one.
func (inv *intentInvocation) laneQueueState(targets []intentTarget, install, goalID, sha, main string) *intentResult {
	entry, ok, err := plain.Latest(install, goalID)
	if err == nil && ok && main != "" {
		contained := inv.delivery().containedIn
		if contained == nil {
			contained = plain.ContainedIn
		}
		var derived []plain.Entry
		derived, err = plain.Landed([]plain.Entry{entry}, contained(inv.layout.InstallationRoot, main))
		entry = derived[0]
	}
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
		landed := intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("goal %s at %s landed on main through the landing lane; the goal stays open until done", goalID, plain.Short(entry.SHA)),
			next:    inv.publicArgv("goal", "done", goalID, "--reason", "TEXT"), nextReason: "concludes it"}
		if inv.markRefusal(targets, goalID) == nil {
			landed = inv.markHandedIn(landed, goalID)
		}
		return &landed
	case plain.StateReturned:
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("goal %s at %s was returned: %s", goalID, plain.Short(entry.SHA), entry.Reason),
			next:    inv.sameCommand(), nextReason: "after the fix is pushed to " + entry.Branch + ", hands it in again"}
	}
	waiting := inv.markHandedIn(intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
		Summary: fmt.Sprintf("goal %s at %s is waiting in the landing lane; its landing agent proves and pushes it", goalID, plain.Short(entry.SHA))}, goalID)
	return &waiting
}

// markRefusal refuses a hand-in whose goal land-ready would not mark: the
// goal is not this session's claim, or a stop fence stands. It reads the
// goal's file in the accepted ledger the landing gate has just fetched.
func (inv *intentInvocation) markRefusal(targets []intentTarget, goalID string) *intentResult {
	projection, _, problem := inv.projection()
	if problem != nil {
		problem.Targets = targets
		return problem
	}
	session, err := inv.sessionActor()
	switch file := projection.Tree.Live[goalID]; {
	case err != nil:
	case file == nil:
		err = fmt.Errorf("goal %s is not live", goalID)
	default:
		err = goal.LandReadyRefusal(file, goalID, session)
	}
	if err == nil {
		return nil
	}
	return &intentResult{Targets: targets, Outcome: intentRefused, code: 1,
		Summary: fmt.Sprintf("goal %s can't be marked as waiting to land, so nothing was handed in: %v", goalID, err),
		next:    inv.publicArgv("status", goalID), nextReason: "shows who holds the goal and whether it is stopped"}
}

// markHandedIn marks the goal of a standing queue line as waiting to land,
// so its elapsed clock stops while the line waits and stays stopped once it
// lands, unless the ledger shows it marked already. A mark that fails leaves
// the line: the next work land at the same commit reads it, waiting or
// landed, and marks the goal.
func (inv *intentInvocation) markHandedIn(result intentResult, goalID string) intentResult {
	if projection, _, problem := inv.projection(); problem == nil {
		if file := projection.Tree.Live[goalID]; file == nil || file.Landing != nil {
			return result
		}
	}
	mark := inv.delivery().landMark
	if mark == nil {
		mark = (*intentInvocation).landReady
	}
	if marked := mark(inv, goalID); marked.Outcome != intentConfirmed && marked.Outcome != intentUnchanged {
		result.Outcome, result.code = intentPartial, 1
		result.Summary += "; it is not marked as waiting to land, so its clock still runs: " + marked.Summary
		result.next, result.nextReason = inv.sameCommand(), "marks it"
	}
	return result
}

// handIn appends the goal's branch at sha to the lane's queue, then marks
// the goal as waiting to land. The seat's gates passed before it, and the
// mark's own conditions are read before the line, so a goal that could not
// be marked hands nothing in. The branch is read at origin, so it is already
// pushed. A repeat at the same sha appends nothing.
func (inv *intentInvocation) handIn(targets []intentTarget, install, goalID, sha, main string) intentResult {
	if refused := inv.markRefusal(targets, goalID); refused != nil {
		return *refused
	}
	now, seat := inv.delivery().now(), inv.delivery().laneSeat
	if seat == nil {
		seat = batchowner.LandingLaneRegistrant
	}
	line := plain.Line{Goal: goalID, Branch: "goal/" + goalID, SHA: sha, Seat: seat(inv.layout.InstallationRoot), At: now.UTC().Format(time.RFC3339),
		Delivered: strings.TrimSpace(inv.input.text("delivered"))}
	_, added, err := plain.HandIn(install, line)
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing lane's queue can't be written, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !added {
		if result := inv.laneQueueState(targets, install, goalID, sha, main); result != nil {
			return *result
		}
	}
	entry, _, _ := plain.Latest(install, goalID)
	return inv.markHandedIn(intentResult{Targets: targets, Outcome: intentConfirmed, Data: map[string]any{"route": "lane", "queue": entry},
		Summary: fmt.Sprintf("goal %s at %s handed to the lane; its landing agent proves and pushes it", goalID, plain.Short(sha)),
		next:    inv.sameCommand(), nextReason: "shows whether it waits, landed or was returned"}, goalID)
}
