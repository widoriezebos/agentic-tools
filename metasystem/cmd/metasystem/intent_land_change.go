package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
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
func (inv *intentInvocation) landChange(request landpath.LandRequest, path landpath.Owners, landingRoot string, targets []intentTarget, stdout, stderr io.Writer) intentResult {
	root := request.Root
	owners := inv.delivery()
	if head, code := seatGit(root, "rev-parse", "HEAD"); code == 0 && changePinned(root, head) && nothingNewToLand(root, request) {
		return inv.continueChange(request, landingRoot, targets, head)
	}
	request.CommitOnly = true
	if status := owners.runLandPath(path, request, stdout, stderr); status != 0 {
		return intentResult{Outcome: intentRefused, code: status, Targets: targets, Data: map[string]any{"route": "lane", "exitCode": status},
			Summary: fmt.Sprintf("the landing stopped with exit %d before the change could join the landing lane; the lines above name the cause", status)}
	}
	head, code := seatGit(root, "rev-parse", "HEAD")
	branch, branchCode := seatGit(root, "symbolic-ref", "--quiet", "--short", "HEAD")
	if code != 0 || branchCode != 0 {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: map[string]any{"route": "lane"},
			Summary: "the change was committed but its commit or branch cannot be read: " + head + " " + branch}
	}
	id := batch.ChangeID(head)
	if text, status := owners.heldChange(root, head+"^", head, branch); status != 0 {
		back := giveChangeBack(root, head, request)
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: fmt.Sprintf("held refused change %s, so it cannot join the landing lane: %s; %s", id, text, back)}
	}
	if output, pinCode := seatGit(root, "update-ref", batchowner.ChangePinRef(head), head); pinCode != 0 {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: fmt.Sprintf("change %s was committed but could not be pinned for the landing lane: %s; run the same command again", id, output)}
	}
	return inv.joinChangeToLane(request, landingRoot, targets, head)
}

// continueChange answers the repeat for a pinned change from the lane.
func (inv *intentInvocation) continueChange(request landpath.LandRequest, landingRoot string, targets []intentTarget, head string) intentResult {
	root, id := request.Root, batch.ChangeID(head)
	record, unit, member, err := inv.delivery().lookupChange(landingRoot, id)
	if err != nil {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: fmt.Sprintf("the landing lane's batches are unreadable, so work land cannot tell whether change %s joined: %v; nothing was joined; run the same command again once the lane reads", id, err)}
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
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: data,
			Summary: fmt.Sprintf("change %s left batch %s as %s: %s; %s", id, record.BatchID, unit.State, unit.Failure, back)}
	case batch.UnitReturnPending:
		return intentResult{Outcome: intentInProgress, Targets: targets, Data: data,
			Summary: fmt.Sprintf("change %s is leaving batch %s as %s: %s", id, record.BatchID, unit.Outcome, unit.Failure),
			next:    inv.sameCommand(), nextReason: "reads the change's membership again once the batch owner settles it"}
	}
	return intentResult{Outcome: intentInProgress, Targets: targets, Data: data,
		Summary: fmt.Sprintf("change %s is %s in landing batch %s; the batch owner proves and pushes it", id, unit.State, record.BatchID),
		next:    inv.sameCommand(), nextReason: "reads the same change's membership until the lane records its landing; it never joins twice"}
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
		return intentResult{Outcome: intentInProgress, Targets: targets,
			Data:    map[string]any{"route": "lane", "change": id, "batchId": record.BatchID, "batchState": record.State, "unitState": batch.UnitJoined, "joinedNow": true},
			Summary: fmt.Sprintf("change %s joined landing batch %s; the lane owner could not be started: %v — metasystem landing start", id, record.BatchID, ownerless.Cause),
			next:    inv.publicArgv("landing", "start"), nextReason: "starts the lane's owner, which proves and pushes the batch"}
	}
	var stacked *batch.StackedChangeRefusal
	if errors.As(err, &stacked) {
		// The parent lands first; the seat keeps both commits and the pin.
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: stacked.Reason}
	}
	if err != nil {
		// A refused join gives the commit back, as an ejection does: the
		// same commit would be refused again (N-2).
		back := giveChangeBack(request.Root, head, request)
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: map[string]any{"route": "lane", "change": id},
			Summary: fmt.Sprintf("change %s did not join the landing lane: %v; %s", id, err, back)}
	}
	targets = append(targets, intentTarget{Kind: "batch", ID: record.BatchID})
	return intentResult{Outcome: intentInProgress, Targets: targets,
		Data:    map[string]any{"route": "lane", "change": id, "batchId": record.BatchID, "batchState": record.State, "unitState": batch.UnitJoined, "joinedNow": true},
		Summary: fmt.Sprintf("change %s joined landing batch %s; the batch owner proves and pushes it", id, record.BatchID),
		next:    inv.sameCommand(), nextReason: "reads the same change's membership until the lane records its landing; it never joins twice"}
}

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
		return "the commit is no longer this branch's head, so it was left where it is; fold it into a new change and land that"
	}
	mode := "--mixed"
	where := "in the working tree"
	if request.StagedOnly {
		mode, where = "--soft", "staged"
	}
	if output, code := seatGit(root, "reset", "-q", mode, head+"^"); code != 0 {
		return "the commit could not be undone (" + output + "); undo it with git reset " + mode + " HEAD^ and land again"
	}
	seatGit(root, "update-ref", "-d", batchowner.ChangePinRef(head))
	return "the commit is undone and its changes are " + where + " again: fix them and run the same command"
}
