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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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

// claimLaneReader reads only when a held claim needs a lane answer. An
// unreadable lane supplies no entry, so the standing quota still applies.
func (inv *intentInvocation) claimLaneReader() func(string, string) (string, error) {
	return func(id, main string) (string, error) {
		root, configured, problem := inv.laneCheck(nil)
		if problem != nil || !configured {
			return "", nil
		}
		install, err := inv.laneInstallOf(root)
		if err != nil {
			return "", nil
		}
		entry, ok, err := inv.latestLaneGoalEntry(install, id, main)
		if err != nil || !ok {
			return "", nil
		}
		// Landed code decides first; otherwise a goal whose newest entry of
		// any kind was returned is returned (a returned records hand-in
		// carries the goal's code). Only the claim reads it this way: work
		// land's land-once check keeps reading latestLaneGoalEntry.
		if entry.State != plain.StateLanded && newestLaneEntryReturned(install, id) {
			return plain.StateReturned, nil
		}
		return entry.State, nil
	}
}

// newestLaneEntryReturned says whether the goal's newest queue entry, records
// hand-ins included, is a return.
func newestLaneEntryReturned(install, goalID string) bool {
	entries, err := plain.Entries(install)
	if err != nil {
		return false
	}
	for index := len(entries) - 1; index >= 0; index-- {
		if entries[index].Goal == goalID {
			return entries[index].State == plain.StateReturned
		}
	}
	return false
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
	if entry.State == plain.StateWaiting && inv.input.text("exception") == branch.LandTrunkRedCode {
		return nil
	}
	if entry.State == plain.StateWaiting {
		if refused := inv.landIncidentHold(goalID, sha); refused != nil {
			return refused
		}
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
		if sha != "" && inv.input.has("again") {
			return nil
		}
		if entry.Conflict != nil {
			return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
				Summary: fmt.Sprintf("%s at %s was returned: %s", subject, plain.Short(entry.SHA), entry.Reason),
				next:    inv.publicArgv("work", "rebase", goalID)}
		}
		return &intentResult{Targets: targets, Outcome: intentRefused, code: 1, Data: data,
			Summary: fmt.Sprintf("%s at %s was returned: %s", subject, plain.Short(entry.SHA), entry.Reason),
			next:    inv.sameCommand(), nextReason: "after the fix is pushed to " + entry.Branch + ", hands it in again; or metasystem work land " + goalID + " --again when the return no longer applies"}
	}
	if len(entry.After) > 0 {
		return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("%s at %s waits in the landing lane. It %s.", subject, plain.Short(entry.SHA), entry.Reason)}
	}
	if entry.Cause != nil {
		return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
			Summary: fmt.Sprintf("%s at %s waits in the landing lane: %s; cause: %s", subject, plain.Short(entry.SHA), entry.Reason, entry.Cause.Kind)}
	}
	return &intentResult{Targets: targets, Outcome: intentUnchanged, Data: data,
		Summary: fmt.Sprintf("%s at %s is waiting in the landing lane; its landing agent proves and pushes it", subject, plain.Short(entry.SHA))}
}

func (inv *intentInvocation) handInTrunkInputs() (lane.Record, plain.ProveSeams, []goal.TrunkRedEntry, error, *intentResult) {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return record, plain.ProveSeams{}, nil, nil, problem
	}
	seams := inv.laneBatchSeams(home, record, owners.plainProve)
	incidents, err := plain.TrunkInputs(record, seams)
	return record, seams, incidents, err, nil
}

func (inv *intentInvocation) landIncidentHold(goalID string, tips ...string) *intentResult {
	if inv.laneException != nil {
		return nil
	}
	_, configured, problem := inv.laneCheck(inv.targets(goalID))
	if problem != nil {
		return problem
	}
	if !configured {
		projection, _, problem := inv.projection()
		if problem != nil {
			return problem
		}
		if red, held := goal.LandingIncident(projection.Tree.TrunkRed, goalID); held {
			return &intentResult{Targets: inv.targets(goalID), Outcome: intentRefused, code: 1, Summary: plain.IncidentReason(red) + ". Nothing was handed in.", next: inv.publicArgv("incident", "list")}
		}
		return nil
	}
	record, seams, incidents, readErr, problem := inv.handInTrunkInputs()
	if problem != nil {
		return problem
	}
	value := plain.PolicyValue{Value: "auto"}
	var policyErr error
	if seams.Policy != nil {
		value, policyErr = seams.Policy("landing.trunk-red")
	}
	entry := plain.Entry{Goal: goalID}
	if current, known, err := plain.Latest(record.Install, goalID); err == nil && known && current.State == plain.StateWaiting {
		entry = current
		if len(tips) > 0 && tips[0] != "" && tips[0] != entry.SHA {
			tip := tips[0]
			entry.SHA, entry.Exception = tip, nil
		}
	}
	err := readErr
	if err == nil {
		err = plain.TrunkDecision(entry, incidents, value, policyErr, record)
	}
	if err == nil {
		return nil
	}
	syncErr := plain.SyncPolicyQuestion(record.Install, inv.landing().machine, inv.delivery().now(), seams)
	details := []string{}
	if syncErr != nil {
		details = append(details, "question sync pending: "+syncErr.Error())
	}
	return &intentResult{Targets: inv.targets(goalID), Outcome: intentRefused, code: 1, Data: map[string]any{"code": branch.LandTrunkRedCode},
		Summary: err.Error() + ". Nothing was handed in.", next: inv.publicArgv(plain.ExceptionCommand(goalID, "", inv.personName(""))[1:]...), Details: details}
}

// handIn appends the goal's branch at sha to the lane's queue: the seat's
// gates passed before it. The branch is read at origin, so it is already
// pushed. Again re-queues a returned tip; a waiting repeat keeps new metadata.
func (inv *intentInvocation) handIn(targets []intentTarget, install, goalID, sha string, state intentBranchState) intentResult {
	now := inv.delivery().now()
	registrant := inv.delivery().laneRegistrant
	if registrant == nil {
		registrant = batchowner.LandingLaneRegistrant
	}
	line := plain.Line{Goal: goalID, Branch: "goal/" + goalID, SHA: sha, Seat: registrant(inv.layout.InstallationRoot.Path()), At: now.UTC().Format(time.RFC3339),
		Records: inv.input.has("records"), Delivered: strings.TrimSpace(inv.input.text("delivered")), Again: inv.input.has("again")}
	previous, seen, err := plain.Latest(install, goalID)
	if err != nil {
		return intentResult{Outcome: intentFailed, code: 1, Summary: "the landing queue cannot be read: " + err.Error(), next: inv.sameCommand(), nextReason: "retry after the queue can be read"}
	}
	var exceptionSeams plain.ProveSeams
	exceptionWarnings := []string{}
	if inv.laneException != nil {
		record, seams, incidents, readErr, problem := inv.handInTrunkInputs()
		if problem != nil {
			return *problem
		}
		if record.Install != install {
			return intentResult{Outcome: intentRefused, code: 1, Summary: "the registered lane changed; repeat this exception for the current lane", next: inv.sameCommand()}
		}
		person := inv.laneException
		var err error
		line.Exception, err = plain.BindException(plain.GoalSHA{Goal: goalID, SHA: sha}, record, person.Proof, person.Root, person.Name, inv.input.text("reason"), person.At, incidents, readErr)
		if err != nil {
			return intentResult{Outcome: intentRefused, code: 1, Summary: err.Error(), next: inv.sameCommand()}
		}
		exceptionSeams = seams
		if readErr != nil {
			exceptionWarnings = append(exceptionWarnings, "incident binding unknown: "+readErr.Error())
		}
		if _, err := seams.Policy("landing.trunk-red"); err != nil {
			exceptionWarnings = append(exceptionWarnings, "advisory trunk-red policy unreadable: "+err.Error())
		}
	}
	projection, _, problem := inv.projection()
	if problem != nil {
		return *problem
	}
	if file := projection.Tree.Live[goalID]; file != nil && file.Claimed != nil {
		line.AreaSnapshot = file.Claimed.AreaSnapshot
		if line.Source == "" && !line.Known && len(line.Warnings) == 0 {
			line.AreaSnapshot = inv.claimAreaReaders().Design(goalID, projection.Tip)
			if !line.Known && line.Source == "" {
				line.Warnings = nil
			}
		}
	} else {
		readers := inv.claimAreaReaders()
		line.AreaSnapshot = readers.Design(goalID, projection.Tip)
		if !line.Records {
			// Restoring code queue ownership after release needs area admission.
			endpoint, err := inv.owners.dependencies.endpoint(projection.Root)
			if err != nil {
				return intentResult{Outcome: intentFailed, code: 1, Summary: err.Error(), next: inv.sameCommand(), nextReason: "try again once the goal ledger can be read"}
			}
			warnings, err := goal.ClaimAreas(projection.Tree, endpoint, projection.Tip, goalID, line.AreaSnapshot, readers, goal.Actor{})
			if err != nil {
				message := strings.SplitN(err.Error(), "\nrun: ", 2)[0]
				_, proof, problem := inv.actingAs("work land", goalID, actorEither)
				if goal.RefusalCode(err) != goal.ClaimAreasCode || problem != nil || proof == nil || proof.Helm != nil || !proof.EnrolledTerminalFor(inv.stateRoot) {
					return intentResult{Outcome: intentRefused, code: 1, Summary: message, next: inv.sameCommand(), nextReason: "after the blocking goal lands or is dropped, hands this branch in again"}
				}
				warnings = append(warnings, message)
			}
			line.Warnings = warnings
		}
	}
	if line.Records && !line.Known && line.Source == "" {
		line.Warnings = nil
	}
	for _, incident := range projection.Tree.TrunkRed {
		if incident.Closed == nil && incident.EntryClass() == goal.TrunkRedClassTrunkRed && incident.FixGoal == goalID {
			line.Fix = incident.Identity
			break
		}
	}
	if inv.input.switched("whole") {
		line.WholeBy = strings.TrimPrefix(inv.input.text("by"), "human:")
	}
	work, problem := inv.goalWork(goalID)
	if problem != nil {
		return *problem
	}
	for _, one := range work {
		unit := handInUnitRounds(one)
		if state.ReadsWaived {
			unit.Read = "waived"
		}
		for index, committed := range state.Status.Units {
			if committed.Unit == one.Unit && index < len(state.Sources) {
				switch state.Sources[index] {
				case "unit-read":
					unit.Read = "promoted"
				case "critic-root", "reader-record":
					unit.Read = "critic"
				}
			}
		}
		line.Units = append(line.Units, unit)
	}
	added := false
	if err == nil {
		if inv.laneException != nil {
			if seen {
				line.Previous = &previous
			} else {
				line.ExpectEmpty = true
			}
			_, added, err = plain.HandIn(install, line, exceptionSeams)
		} else {
			_, added, err = plain.HandIn(install, line)
		}
	}
	if err != nil {
		return intentResult{Targets: targets, Outcome: intentFailed, code: 1,
			Summary: "the landing lane's queue can't be written, so nothing was handed in",
			next:    inv.sameCommand(), nextReason: "try again; --verbose shows the cause", Details: []string{err.Error()}}
	}
	if !added && inv.laneException == nil {
		if result := inv.laneQueueState(targets, install, goalID, sha, state.EndpointTip); result != nil {
			return *result
		}
	}
	var details []string
	if added || inv.laneException != nil {
		details = inv.writeJoinedCard(goalID)
	}
	if inv.laneException != nil {
		if err := plain.SyncPolicyQuestion(install, inv.landing().machine, now, exceptionSeams); err != nil {
			details = append(details, "exception recorded; question sync pending: "+err.Error())
		}
	}
	entry, _, _ := plain.Latest(install, goalID)
	subject := "goal " + goalID
	if inv.input.has("records") {
		subject += "'s records"
	}
	summary := fmt.Sprintf("%s at %s handed to the lane; its landing agent proves and pushes it", subject, plain.Short(sha))
	if line.Exception != nil {
		summary += "; exception bound to " + goalID + " at " + sha
		if line.Exception.BindingUnknown {
			summary += "; incident binding unknown"
		} else {
			identities := []string{}
			for _, incident := range line.Exception.Incidents.Open {
				identities = append(identities, incident.Identity)
			}
			summary += " for incidents " + strings.Join(identities, ", ")
		}
	}
	if line.Again && seen && previous.SHA == sha && previous.State == plain.StateReturned {
		summary += "; re-queued after a return that needed no change"
	}
	if warnings := append(line.Warnings, exceptionWarnings...); len(warnings) > 0 {
		summary += "; warning: " + strings.Join(warnings, "; ")
	}
	outcome := intentConfirmed
	if !added {
		outcome = intentUnchanged
	}
	return intentResult{Targets: targets, Outcome: outcome, Data: map[string]any{"route": "lane", "queue": entry},
		Details: details,
		Summary: summary,
		next:    inv.sameCommand(), nextReason: "shows whether it waits, landed or was returned"}
}

func (inv *intentInvocation) writeJoinedCard(goal string) []string {
	home, err := inv.boardHome()
	if err == nil {
		card, live := board.LiveOrReturnedCard(home, goal)
		if !live {
			return nil
		}
		err = board.Update(home, card.Seat, goal, func(current board.Card) (board.Card, bool) {
			if current.Goal == "" || (current.Stage.Terminal() && current.Stage != board.StageReturned) || current.Stage.ProcessBound() {
				return current, false
			}
			current.Stage, current.Owner, current.Job, current.Proof, current.Batch = board.StageJoined, nil, nil, nil, ""
			current.Since = time.Time{}
			current.Writer = board.Writer{Component: "hand-in", At: inv.delivery().now()}
			return current, true
		})
	}
	if err != nil {
		return []string{"the hand-in card for " + goal + " was not written: " + err.Error()}
	}
	return nil
}

// latestLaneEntry derives the hand-in state from the seat's view of main.
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

// latestLaneGoalEntry keeps records hand-ins from hiding the goal's landing.
func (inv *intentInvocation) latestLaneGoalEntry(install, goalID, main string) (plain.Entry, bool, error) {
	entries, err := plain.Entries(install)
	if err != nil {
		return plain.Entry{}, false, err
	}
	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		if entry.Goal != goalID {
			continue
		}
		if entry.Records {
			continue
		}
		// A records hand-in can supersede this entry in the queue, but
		// main still determines whether the goal's work landed.
		if entry.State == plain.StateSuperseded {
			entry.State = plain.StateWaiting
		}
		if main != "" {
			contains := plain.ContainedIn(inv.layout.InstallationRoot.Path(), main)
			if read := inv.delivery().laneContains; read != nil {
				contains = func(sha string) (bool, error) { return read(sha, main) }
			}
			derived, err := plain.Landed([]plain.Entry{entry}, contains)
			return derived[0], true, err
		}
		return entry, true, nil
	}
	return plain.Entry{}, false, nil
}

func handInUnitRounds(work launch.NamedWork) plain.UnitRounds {
	unit := plain.UnitRounds{Unit: work.Unit, Machinery: map[string]int{}, Proof: []string{"check"}, Read: "none"}
	if work.Record == nil {
		return unit
	}
	for _, round := range work.Record.Rounds {
		if round.Cause == "" {
			unit.Counted++
		} else {
			unit.Machinery[round.Cause]++
		}
	}
	if rounds := work.Record.Rounds; len(rounds) > 0 {
		var proof []string
		for _, step := range rounds[len(rounds)-1].Steps {
			if name, ok := strings.CutPrefix(step.Name, "proof:"); ok {
				proof = append(proof, name)
			}
		}
		if len(proof) > 0 {
			unit.Proof = proof
		}
	}
	return unit
}

func (inv *intentInvocation) writeReturnedCard(goal string) []string {
	home, err := inv.boardHome()
	if err == nil {
		card, live := board.LiveCard(home, goal)
		if !live {
			return nil
		}
		err = board.Update(home, card.Seat, goal, func(current board.Card) (board.Card, bool) {
			if current.Goal == "" || current.Stage.Terminal() {
				return current, false
			}
			current.Stage, current.Owner, current.Job, current.Proof, current.Batch = board.StageReturned, nil, nil, nil, ""
			current.Since = time.Time{}
			current.Writer = board.Writer{Component: "landing-return", At: inv.delivery().now()}
			return current, true
		})
	}
	if err != nil {
		return []string{"the returned card for " + goal + " was not written: " + err.Error()}
	}
	return nil
}
