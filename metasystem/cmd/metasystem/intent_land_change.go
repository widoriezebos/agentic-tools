package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/landpath"
)

// seatGit runs git in the seat's installation and returns its trimmed output.
func seatGit(root string, args ...string) (string, int) {
	result := landingPathGit(landpath.GitCall{Dir: root, Args: args})
	output := strings.TrimSpace(string(result.Stdout))
	if result.Code != 0 {
		output = strings.TrimSpace(string(result.Stdout) + " " + string(result.Stderr))
	}
	return output, result.Code
}

func (owners *intentDeliveryOwners) runLandPath(path landpath.Owners, request landpath.LandRequest, stdout, stderr io.Writer) int {
	if owners.landPath != nil {
		return owners.landPath(path, request, stdout, stderr)
	}
	return landpath.Land(path, request, stdout, stderr)
}

func (owners *intentDeliveryOwners) heldChange(root, base, commit, branch string) (string, int) {
	if owners.changeHeld != nil {
		return owners.changeHeld(root, base, commit, branch)
	}
	var output bytes.Buffer
	status := landingHeldTo(&output, &output, cleanOwnerRoot(root), base, commit, "origin", "refs/heads/"+branch)
	return strings.TrimSpace(output.String()), status
}

func (owners *intentDeliveryOwners) joinChange(request batchowner.ChangeJoinRequest) (batch.Record, error) {
	if owners.changeJoin != nil {
		return owners.changeJoin(request)
	}
	return batchowner.ExecuteChangeJoin(request, batchowner.ProductionChangeJoinDependencies())
}

func (owners *intentDeliveryOwners) lookupChange(landingRoot, id string) (batch.Record, batch.Unit, bool, error) {
	if owners.changeUnit != nil {
		return owners.changeUnit(landingRoot, id)
	}
	return productionChangeUnit(landingRoot, id)
}

func (owners *intentDeliveryOwners) advanceSeat(root, branch string) error {
	if owners.changeAdvance != nil {
		return owners.changeAdvance(root, branch)
	}
	if output, code := seatGit(root, "fetch", "--quiet", "origin", "+refs/heads/"+branch+":refs/remotes/origin/"+branch); code != 0 {
		return fmt.Errorf("fetch origin: %s", output)
	}
	var output bytes.Buffer
	if err := landing.Advance(root, "refs/remotes/origin/"+branch, &output, &output); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(output.String()))
	}
	return nil
}

// productionChangeUnit finds a change in the lane checkout's batches: its
// live membership, else its newest settled one. A record that cannot be read
// is an error, never "not a member", so a change never joins twice.
func productionChangeUnit(landingRoot, id string) (batch.Record, batch.Unit, bool, error) {
	records, err := batch.NewStore(landingRoot, identity.KernelProber{}).Records()
	if err != nil {
		return batch.Record{}, batch.Unit{}, false, err
	}
	var found batch.Record
	var unit batch.Unit
	member := false
	for _, record := range records {
		for _, candidate := range record.Units {
			if candidate.GoalID != id {
				continue
			}
			switch candidate.State {
			case batch.UnitJoining, batch.UnitJoined, batch.UnitReturnPending:
				return record, candidate, true, nil
			}
			found, unit, member = record, candidate, true
		}
	}
	return found, unit, member, nil
}

// landChange lands a hand-made change through the host's landing lane
// (U11b): the landing path's seat steps up to the commit, held over that
// commit, the commit pinned, and the change joined to the lane. The same
// command with nothing new to land, on a seat whose HEAD is a pinned change,
// is the continuation: it reads the change's membership and never joins it
// twice.
func (inv *intentInvocation) landChange(request landpath.LandRequest, path landpath.Owners, landingRoot string, targets []intentTarget, run *landingRun) intentResult {
	root := request.Root
	owners := inv.delivery()
	if head, code := seatGit(root, "rev-parse", "HEAD"); code == 0 && changePinned(root, head) && nothingNewToLand(root, request) {
		return inv.continueChange(request, landingRoot, targets, head)
	}
	request.CommitOnly = true
	if status := owners.runLandPath(path, request, &run.details, &run.told); status != 0 {
		return run.stopped(intentResult{Outcome: intentRefused, code: status, Targets: targets, Data: map[string]any{"route": "lane", "exitCode": status}},
			status, inv.typedArgv())
	}
	head, code := seatGit(root, "rev-parse", "HEAD")
	branch, branchCode := seatGit(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if code != 0 || branchCode != 0 {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: map[string]any{"route": "lane"},
			Summary: "the change was committed, but this checkout's commit or branch can't be read now",
			next:    []string{"git", "status"}, nextReason: "then repeat this command; it joins the committed change",
			Details: append(run.detailLines(), "git: "+head+" "+branch)}
	}
	id := batch.ChangeID(head)
	if text, status := owners.heldChange(root, head+"^", head, branch); status != 0 {
		back := giveChangeBack(root, head, request)
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary:  "the change's goal may not land from this checkout, so it didn't join the landing lane: " + oneLine(heldReason(text)),
			Decision: back, Details: append(run.detailLines(), "held refused change "+id+": "+text)}
	}
	if output, pinCode := seatGit(root, "update-ref", batchowner.ChangePinRef(head), head); pinCode != 0 {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: "the change was committed but couldn't be marked for the landing lane",
			next:    inv.sameCommand(), nextReason: "tries again; it joins the committed change",
			Details: append(run.detailLines(), "change "+id+" pin failed: "+output)}
	}
	result := inv.joinChangeToLane(request, landingRoot, targets, head)
	result.Details = append(run.detailLines(), result.Details...)
	return result
}

// heldReason is the held check's refusal detail without its code and
// commit ("held refused: CODE: COMMIT: detail").
func heldReason(text string) string {
	for _, line := range nonEmptyLines(text) {
		if rest, found := strings.CutPrefix(line, "held refused: "); found {
			if parts := strings.SplitN(rest, ": ", 3); len(parts) == 3 {
				return parts[2]
			}
			return rest
		}
	}
	return text
}

// oneLine is text's first line; the rest is the details'.
func oneLine(text string) string {
	text = strings.TrimSpace(text)
	if cut := strings.IndexByte(text, '\n'); cut >= 0 {
		text = strings.TrimSpace(text[:cut])
	}
	return text
}

// continueChange answers the repeat for a pinned change from the lane.
func (inv *intentInvocation) continueChange(request landpath.LandRequest, landingRoot string, targets []intentTarget, head string) intentResult {
	root, id := request.Root, batch.ChangeID(head)
	record, unit, member, err := inv.delivery().lookupChange(landingRoot, id)
	if err != nil {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: "the landing lane's records can't be read, so the change wasn't joined again",
			next:    inv.publicArgv("landing", "status"), nextReason: "shows the lane; then repeat this command",
			Details: []string{fmt.Sprintf("change %s: %v", id, err)}}
	}
	if !member {
		// Pinned but in no batch: the join did not complete.
		return inv.joinChangeToLane(request, landingRoot, targets, head)
	}
	targets = append(targets, intentTarget{Kind: "batch", ID: record.BatchID})
	data := map[string]any{"route": "lane", "change": id, "batchId": record.BatchID, "batchState": record.State, "unitState": unit.State}
	switch unit.State {
	case batch.UnitLanded:
		summary := fmt.Sprintf("change %s already landed as %s through batch %s", id, shortCommit(unit.LandedCommit), record.BatchID)
		branch, _ := seatGit(root, "symbolic-ref", "--quiet", "--short", "HEAD")
		if err := inv.delivery().advanceSeat(root, branch); err != nil {
			summary += "; this branch still carries the local commit, since it could not move onto origin: " + err.Error()
		} else {
			seatGit(root, "update-ref", "-d", batchowner.ChangePinRef(head))
			summary += "; this branch is on origin again"
		}
		return intentResult{Outcome: intentUnchanged, Targets: targets, Data: data, Summary: summary}
	case batch.UnitEjected, batch.UnitWithdrawn, batch.UnitWithdrawnBudget:
		back := giveChangeBack(root, head, request)
		why, details := personLaneText(unit.Failure)
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data,
			Summary:  fmt.Sprintf("the landing lane sent the change back: %s", oneLine(why)),
			Decision: back, Details: append(details, fmt.Sprintf("change %s left batch %s as %s", id, record.BatchID, unit.State))}
	case batch.UnitReturnPending:
		why, details := personLaneText(unit.Failure)
		return intentResult{Outcome: intentInProgress, Targets: targets, Data: data,
			Summary: fmt.Sprintf("the landing lane is sending the change back: %s", oneLine(why)),
			next:    inv.sameCommand(), nextReason: "shows the outcome once the lane has settled it",
			Details: append(details, fmt.Sprintf("change %s is leaving batch %s as %s", id, record.BatchID, unit.Outcome))}
	}
	return intentResult{Outcome: intentInProgress, Targets: targets, Data: data,
		Summary: fmt.Sprintf("change %s is %s in landing batch %s; the lane tests and pushes it", id, unit.State, record.BatchID),
		next:    inv.sameCommand(), nextReason: "shows its progress until it lands; it never joins twice"}
}

// joinChangeToLane joins the pinned change to the lane.
func (inv *intentInvocation) joinChangeToLane(request landpath.LandRequest, landingRoot string, targets []intentTarget, head string) intentResult {
	id := batch.ChangeID(head)
	gateTip := ""
	if request.GoalSet {
		gateTip = inv.intentBranchTip(request.Goal)
	}
	owners := inv.delivery()
	record, err := owners.joinChange(batchowner.ChangeJoinRequest{SeatRoot: request.Root, LandingRoot: landingRoot, Commit: head, GateTip: gateTip, At: owners.now()})
	var ownerless *batchowner.ChangeOwnerStartError
	if errors.As(err, &ownerless) {
		// Joined; only the lane's owner did not start: nothing is given back.
		record = ownerless.Record
		targets = append(targets, intentTarget{Kind: "batch", ID: record.BatchID})
		why, details := personLaneText(ownerless.Cause.Error())
		return intentResult{Outcome: intentInProgress, Targets: targets,
			Data:    map[string]any{"route": "lane", "change": id, "batchId": record.BatchID, "batchState": record.State, "unitState": batch.UnitJoined, "joinedNow": true},
			Summary: fmt.Sprintf("change %s joined landing batch %s, but the lane couldn't be started: %s", id, record.BatchID, oneLine(why)),
			next:    inv.publicArgv("landing", "start"), nextReason: "starts the lane, which tests and pushes the batch",
			Details: details}
	}
	var stacked *batch.StackedChangeRefusal
	if errors.As(err, &stacked) {
		// The parent lands first; the seat keeps both commits and the pin.
		why, details := personLaneText(stacked.Reason)
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: "the change builds on another change still in the landing lane: " + oneLine(why),
			next:    inv.sameCommand(), nextReason: "once that change has landed", Details: details}
	}
	if err != nil {
		// A refused join gives the commit back, as an ejection does: the
		// same commit would be refused again (N-2).
		back := giveChangeBack(request.Root, head, request)
		why, details := personLaneText(err.Error())
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary:  "the change couldn't join the landing lane: " + oneLine(why),
			Decision: back, Details: append(details, "change "+id)}
	}
	targets = append(targets, intentTarget{Kind: "batch", ID: record.BatchID})
	return intentResult{Outcome: intentInProgress, Targets: targets,
		Data:    map[string]any{"route": "lane", "change": id, "batchId": record.BatchID, "batchState": record.State, "unitState": batch.UnitJoined, "joinedNow": true},
		Summary: fmt.Sprintf("change %s joined landing batch %s; the lane tests and pushes it", id, record.BatchID),
		next:    inv.sameCommand(), nextReason: "shows its progress until it lands; it never joins twice"}
}

// personLaneText is a landing lane's reason as a person reads it: its
// leading refusal code (BATCH_JOIN_CONFLICT: ...) moves to the details,
// which keep the reason whole.
func personLaneText(reason string) (string, []string) {
	reason = strings.TrimSpace(reason)
	plain := reason
	for {
		code, rest, found := strings.Cut(plain, ": ")
		if !found || !laneCode.MatchString(code) {
			break
		}
		plain = rest
	}
	if plain == reason {
		return plain, nil
	}
	return plain, []string{"refused because: " + reason}
}

// laneCode is a refusal code a lane reason leads with.
var laneCode = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$|^EJECTED from landing batch [^ ]+$`)

// changePinned reports whether head is pinned as a change bound for the lane.
func changePinned(root, head string) bool {
	pinned, code := seatGit(root, "rev-parse", "--verify", "--quiet", batchowner.ChangePinRef(head)+"^{commit}")
	return code == 0 && pinned == head
}

// nothingNewToLand reports whether the request names nothing the seat has not
// already committed: an empty index for --staged, clean named paths otherwise.
func nothingNewToLand(root string, request landpath.LandRequest) bool {
	if request.StagedOnly {
		_, code := seatGit(root, "diff", "--cached", "--quiet", "--")
		return code == 0
	}
	status, code := seatGit(root, append([]string{"status", "--porcelain", "--"}, request.Pathspecs...)...)
	return code == 0 && status == ""
}

// giveChangeBack undoes a change's commit on the seat when it is still the
// branch head, keeping its bytes (staged for --staged, in the working tree
// for named paths), and drops its pin, so fixing it and running the same
// command lands it again. It says what it did.
func giveChangeBack(root, head string, request landpath.LandRequest) string {
	current, _ := seatGit(root, "rev-parse", "HEAD")
	if current != head {
		return "the commit is no longer this branch's last, so it was left where it is; fold it into a new change and land that"
	}
	mode := "--mixed"
	where := "in the working tree"
	if request.StagedOnly {
		mode, where = "--soft", "staged"
	}
	if output, code := seatGit(root, "reset", "-q", mode, head+"^"); code != 0 {
		return "undo the commit with git reset " + mode + " HEAD^ (it couldn't be undone: " + output + "), fix the change, then land it again"
	}
	seatGit(root, "update-ref", "-d", batchowner.ChangePinRef(head))
	return "the commit is undone and its changes are " + where + " again: fix them, then repeat this command"
}
