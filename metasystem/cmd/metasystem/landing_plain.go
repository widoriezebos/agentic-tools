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
// nil when it has none, or one at another sha, so the hand-in goes on.
func (inv *intentInvocation) laneQueueState(targets []intentTarget, install, goalID, sha, main string) *intentResult {
	entry, ok, err := inv.latestLaneEntry(install, goalID, main)
	if err != nil {
		return &intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing lane's queue can't be read, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !ok || (sha != "" && entry.SHA != sha) {
		return nil
	}
	data := map[string]any{"route": "lane", "queue": entry}
	subject := "goal " + goalID
	if entry.Records {
		subject += "'s records"
	}
	switch entry.State {
	case plain.StateLanded:
		return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("%s at %s landed on main through the landing lane; the goal stays open until done", subject, plain.Short(entry.SHA)),
			next:    inv.publicArgv("goal", "done", goalID, "--reason", "TEXT"), nextReason: "concludes it"}
	case plain.StateReturned:
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("%s at %s was returned: %s", subject, plain.Short(entry.SHA), entry.Reason),
			next:    inv.sameCommand(), nextReason: "after the fix is pushed to " + entry.Branch + ", hands it in again"}
	}
	return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
		Summary: fmt.Sprintf("%s at %s is waiting in the landing lane; its landing agent proves and pushes it", subject, plain.Short(entry.SHA))}
}

// handIn appends the goal's branch at sha to the lane's queue: the seat's
// gates passed before it. The branch is read at origin, so it is already
// pushed. A repeat at the same sha appends nothing.
func (inv *intentInvocation) handIn(targets []intentTarget, install, goalID, sha, main string) intentResult {
	now := inv.delivery().now()
	line := plain.Line{Goal: goalID, Branch: "goal/" + goalID, SHA: sha, Seat: batchowner.LandingLaneRegistrant(inv.layout.InstallationRoot.Path()), At: now.UTC().Format(time.RFC3339),
		Records: inv.input.has("records"), Delivered: strings.TrimSpace(inv.input.text("delivered"))}
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
	subject := "goal " + goalID
	if inv.input.has("records") {
		subject += "'s records"
	}
	return intentResult{Targets: targets, Outcome: intentConfirmed, Data: map[string]any{"route": "lane", "queue": entry},
		Summary: fmt.Sprintf("%s at %s handed to the lane; its landing agent proves and pushes it", subject, plain.Short(sha)),
		next:    inv.sameCommand(), nextReason: "shows whether it waits, landed or was returned"}
}

// latestLaneEntry reads the same derived landing state for hand-in and rebase.
func (inv *intentInvocation) latestLaneEntry(install, goalID, main string) (plain.Entry, bool, error) {
	if read := inv.delivery().laneLatest; read != nil {
		return read(install, goalID, main)
	}
	entry, ok, err := plain.Latest(install, goalID)
	if err == nil && ok && main != "" {
		var derived []plain.Entry
		derived, err = plain.Landed([]plain.Entry{entry}, plain.ContainedIn(inv.layout.InstallationRoot.Path(), main))
		entry = derived[0]
	}
	return entry, ok, err
}
