package main

// landing begin and landing prove (lane runtime design r10 K4, K6): the
// landing agent's kernel verbs over one batch. begin records the canonical
// series before anything executes; prove runs one subject of it as the
// lane's test run. Both are declared through laneKernelCommand, so the
// engine check runs first, and both act under the pause (lane.Gate).

import (
	"errors"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/custody"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

func landingBeginCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "begin", audience: "both", summary: "record a batch's composed series on main as the one the lane proves and publishes",
		usage: []string{"metasystem landing begin --batch ID --members M,M --base B --head C",
			"metasystem landing begin --batch ID --record-conflict M --base B --onto C"},
		details: []string{"The series B..C holds one commit per member's change, build or chain, in the order --members names them, each its exact replay or marked Lane-Resolved, then at most one Lane-Integration commit.",
			"Every joined member of the batch is named. The lane's own edits (resolutions and integration) come to at most 40 lines; over that, or when a member does not apply on main, the refusal is recorded for the member's return.",
			"The series is written again with the landing trailers and recorded before any test runs; the same series again changes nothing.",
			"--record-conflict records that member M's work does not apply on C, a commit of the series on B, once the lane sees it fail."},
		flags: []intentFlag{{name: "batch", value: "ID", usage: "the batch"}, {name: "members", value: "M,M", usage: "the batch's members in series order"},
			{name: "base", value: "B", usage: "main's commit the series is composed on"}, {name: "head", value: "C", usage: "the tip of the composed series"},
			{name: "record-conflict", value: "M", usage: "record that member M does not apply on --onto"}, {name: "onto", value: "C", usage: "the series commit M failed to apply on"}},
		maxArgs:  0,
		examples: []string{"metasystem landing begin --batch 01j5x00000000000000000ba01 --members goal-a,change:0123456789ab --base origin/main --head HEAD"},
	}, runIntentLandingBegin)
}

func landingProveCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "prove", audience: "both", summary: "run the tests of a batch's recorded series, its base, or one member on the base",
		usage: []string{"metasystem landing prove --batch ID --subject batch|base|member:M"},
		details: []string{"batch runs the recorded candidate, the tree that is published; base runs main's tree; member:M runs main plus exactly M's own work, so a red can be placed on a member, on main, or on their combination.",
			"Each run is the lane's own, on this computer, one at a time; it is recorded on the batch with its subject and ends green, red (a test failed) or unavailable (it could not run), which is never red.",
			"Refused while the lane is stopped, and before landing begin has recorded the batch's series."},
		flags:    []intentFlag{{name: "batch", value: "ID", usage: "the batch"}, {name: "subject", value: "S", usage: "batch, base or member:M"}},
		maxArgs:  0,
		examples: []string{"metasystem landing prove --batch 01j5x00000000000000000ba01 --subject batch", "metasystem landing prove --batch 01j5x00000000000000000ba01 --subject member:goal-a"},
	}, runIntentLandingProve)
}

// laneClaim is the lane's stable claim identity (K7): its machine, the
// lineage every joined goal is handed over to (landing-lane, unit K-d) and
// the custody epoch of the host record.
func laneClaim(kernel laneKernel) (lane.ClaimIdentity, error) {
	machine, err := kernel.owners.machine(kernel.record.Root)
	if err != nil {
		return lane.ClaimIdentity{}, err
	}
	return lane.ClaimIdentity{Root: kernel.record.Root, Machine: machine, Lineage: lane.ClaimLineage, Epoch: kernel.record.CustodyEpoch}, nil
}

// laneClaimActor is the lane's claim identity as a ledger actor: the
// holder begin requires of every goal member.
func laneClaimActor(kernel laneKernel) (string, error) {
	claim, err := laneClaim(kernel)
	if err != nil {
		return "", err
	}
	return claim.Machine + "+" + claim.Lineage, nil
}

func runIntentLandingBegin(inv *intentInvocation, admitted laneKernel) int {
	targets := append(laneTargets(admitted.record.Root), intentTarget{Kind: "batch", ID: inv.input.text("batch")})
	id := strings.TrimSpace(inv.input.text("batch"))
	conflict := strings.TrimSpace(inv.input.text("record-conflict"))
	members := splitMembers(inv.input.text("members"))
	missing := id == "" || strings.TrimSpace(inv.input.text("base")) == "" ||
		conflict == "" && (len(members) == 0 || strings.TrimSpace(inv.input.text("head")) == "") ||
		conflict != "" && (strings.TrimSpace(inv.input.text("onto")) == "" || len(members) != 0)
	if missing {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing begin needs a batch, a base, and either its members and head or a member and the commit it failed on; nothing was recorded",
			next:    inv.publicArgv("landing", "begin", "--help"), nextReason: "shows both forms"})
	}
	layout, err := admitted.record.Layout()
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's record names no usable checkout, so nothing was recorded", err))
	}
	actor, err := laneClaimActor(admitted)
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's machine can't be named, so nothing was recorded", err))
	}
	outcome, err := admitted.owners.begin(kernel.BeginRequest{Home: admitted.home, Layout: layout, BatchID: id, Members: members,
		Base: inv.input.text("base"), Head: inv.input.text("head"), Actor: actor, Conflict: conflict, Onto: inv.input.text("onto")})
	if err != nil {
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	if outcome.Evidence != nil {
		summary := "recorded that " + conflict + " does not apply on " + shortLandingID(outcome.Evidence.Onto) + " in batch " + id
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: outcome.Evidence, Summary: summary,
			view: landingDone(summary, admitted.record.Root)})
	}
	opening := outcome.Opening
	if !outcome.Changed {
		summary := "batch " + id + "'s series is already recorded; its candidate is " + shortLandingID(opening.Candidate)
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: opening, Summary: summary, view: landingDone(summary, admitted.record.Root)})
	}
	summary := fmt.Sprintf("recorded batch %s's series of %d commits on main %s; its candidate is %s, with %d lines of the lane's own",
		id, len(opening.Series), shortLandingID(opening.Base), shortLandingID(opening.Candidate), opening.Deviation)
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: opening, Summary: summary, view: landingDone(summary, admitted.record.Root)})
}

func runIntentLandingProve(inv *intentInvocation, admitted laneKernel) int {
	id := strings.TrimSpace(inv.input.text("batch"))
	subject := strings.TrimSpace(inv.input.text("subject"))
	targets := append(laneTargets(admitted.record.Root), intentTarget{Kind: "batch", ID: id})
	if id == "" || subject == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing prove needs a batch and a subject; nothing was run",
			next:    inv.publicArgv("landing", "prove", "--help"), nextReason: "shows the subjects"})
	}
	layout, err := admitted.record.Layout()
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's record names no usable checkout, so nothing was run", err))
	}
	actor, err := laneClaimActor(admitted)
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's machine can't be named, so nothing was run", err))
	}
	attempt, err := admitted.owners.prove(kernel.ProveRequest{Home: admitted.home, Layout: layout, BatchID: id, Subject: subject, Actor: actor})
	if err != nil {
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	subjectWords := "batch " + id + "'s candidate"
	switch attempt.Subject {
	case batch.SubjectBase:
		subjectWords = "the base of batch " + id
	case batch.SubjectMember:
		subjectWords = attempt.Member + " on the base of batch " + id
	}
	switch attempt.Status {
	case batch.AttemptGreen:
		summary := "the tests of " + subjectWords + " passed"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: attempt, Summary: summary, view: landingDone(summary, admitted.record.Root)})
	case batch.AttemptRed:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: attempt,
			Summary: "tests of " + subjectWords + " failed: " + strings.Join(attempt.RedGroups, ", "),
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the batch and its recorded runs"})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: attempt,
		Summary: "the tests of " + subjectWords + " could not run, which says nothing about the work: " + oneLine(attempt.Reason),
		retry:   "runs them again once what stopped them is fixed", Details: []string{"attempt " + attempt.ID + " is unavailable: " + attempt.Reason}})
}

func splitMembers(value string) []string {
	var members []string
	for _, member := range strings.Split(value, ",") {
		if member = strings.TrimSpace(member); member != "" {
			members = append(members, member)
		}
	}
	return members
}

func landingKernelFailure(targets []intentTarget, summary string, err error) intentResult {
	return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: summary, retry: "tries again", Details: []string{err.Error()}}
}

// landingKernelRefusal renders a kernel verb's refusal: the pause and the
// lane's own refusals name their command; begin's and prove's say what to
// change and run the same command again.
func landingKernelRefusal(inv *intentInvocation, targets []intentTarget, err error) intentResult {
	var laneRefusal *lane.Refusal
	if errors.As(err, &laneRefusal) {
		return *laneRefusalResult(targets, laneRefusal)
	}
	// The custody barrier (K9): live landing work is waited for; work whose
	// state can't be read holds the lane until a person has checked it.
	var held *custody.Held
	if errors.As(err, &held) {
		status := inv.publicArgv("landing", "status", "--verbose")
		if len(held.Live) > 0 {
			return intentResult{Outcome: intentInProgress, code: 1, Targets: targets, Summary: "other landing work still runs, so nothing was started",
				next: status, nextReason: "shows what runs; run the same command again once it has ended", Details: held.Live}
		}
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: "whether other landing work still runs can't be read, so nothing was started",
			next: status, nextReason: "shows what can't be read, for a person to check", Details: held.Unknown}
	}
	var composition *batch.CompositionRefusal
	if errors.As(err, &composition) {
		return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: composition.Evidence,
			Summary: composition.Evidence.Detail + ", so the batch was not begun; the refusal is recorded for its return",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the batch and what its members return on",
			Details: []string{"refused because: " + composition.Code}}
	}
	var code, reason, fix string
	var begin *batch.BeginRefusal
	var refusal *kernel.Refusal
	switch {
	case errors.As(err, &begin):
		code, reason, fix = begin.Code, begin.Reason, begin.Next
	case errors.As(err, &refusal):
		code, reason, fix = refusal.Code, refusal.Reason, refusal.Next
	default:
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane could not do it: " + oneLine(err.Error()),
			retry: "tries again", Details: []string{err.Error()}}
	}
	return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: reason + "; nothing was recorded",
		retry: fix, Details: []string{"refused because: " + code}}
}
