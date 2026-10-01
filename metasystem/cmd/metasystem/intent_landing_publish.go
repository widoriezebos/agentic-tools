package main

// landing publish (lane runtime design r10 §2, K3): the kernel verb that
// puts one proven batch on main. It publishes exactly the series landing
// begin recorded, with its Landing-Proof trailers, only when the batch's
// latest proof is green for that series and was judged by the lane's
// enrolled engine; it pushes through the lane's one publication boundary
// (lane.Publish: gate, tuple token, the lane checkout's pre-push hook, an
// explicit lease) and returns a moved base to the agent instead of
// republishing.

import (
	"errors"
	"fmt"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	landingkernel "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
)

// The refusal codes of landing publish (the refusal register names their
// sites).
const (
	codeLanePublishUnproven = "LANE_PUBLISH_UNPROVEN"
	codeLanePublishNotHeld  = "LANE_PUBLISH_NOT_HELD"
)

func landingPublishCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "publish", audience: "agent", summary: "put one proven batch on main, exactly as it was proven",
		usage: []string{"metasystem landing publish --batch ID"},
		details: []string{"Publishes the series landing begin recorded for the batch, each commit with a Landing-Proof trailer and its tree unchanged, when the batch's latest proof is green for that series and was judged by the lane's enrolled engine.",
			"The push leaves only from the landing checkout, through its pre-push hook, with a lease on the base the series was composed on; nothing else can push from there.",
			"When main moved since the series was composed, nothing is published: recompose on the new main and begin again. A batch already on main changes nothing."},
		flags:    []intentFlag{{name: "batch", value: "ID", usage: "the batch to publish"}},
		maxArgs:  0,
		examples: []string{"metasystem landing publish --batch 4gr18nm8t3nyev9sssda9jgtsq"},
	}, runIntentLandingPublish)
}

// landingPublication is what landing publish put on main.
type landingPublication struct {
	Batch   string `json:"batch"`
	Attempt string `json:"attempt"`
	Base    string `json:"base"`
	Commit  string `json:"commit"`
	Tree    string `json:"tree"`
}

func runIntentLandingPublish(inv *intentInvocation, kernel laneKernel) int {
	root := kernel.record.Root
	targets := laneTargets(root)
	batchID := strings.TrimSpace(inv.input.text("batch"))
	if batchID == "" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing publish needs the batch to publish; nothing was published",
			next:    inv.publicArgv("landing", "status"), nextReason: "names the batch the lane works on"})
	}
	evidence := kernel.owners.evidence
	if evidence == nil {
		// The lane's own records: the series landing begin kept and the
		// proofs landing prove kept, re-verified by test verify.
		layout, err := kernel.record.Layout()
		if err != nil {
			evidence = lane.NoEvidence{}
		} else {
			evidence = landingkernel.PublishEvidence{Layout: layout, Verifier: verifyRetainedTesting}
		}
	}
	status := inv.publicArgv("landing", "status", "--verbose")
	// The proof to run again is landing prove's; line 2 is the lane's
	// status, which names the batch to prove.
	reprove := status
	unproven := func(summary string, next []string, reason string, details ...string) int {
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: summary,
			next: next, nextReason: reason, Details: append([]string{"refused because: " + codeLanePublishUnproven}, details...)})
	}
	begin, err := evidence.Begin(batchID)
	if err != nil {
		return unproven("batch "+batchID+" has no readable begin record, so nothing was published", status, "shows the batch and what it waits for", err.Error())
	}
	proof, err := evidence.Proof(batchID)
	if err != nil {
		return unproven("batch "+batchID+" has no readable proof, so nothing was published", reprove, "shows the batch; prove it again on the enrolled engine", err.Error())
	}
	if problem := proofMismatch(batchID, begin, proof); problem != "" {
		return unproven("batch "+batchID+"'s latest proof is not a green proof of the series it began ("+problem+"), so nothing was published",
			reprove, "shows the batch; prove the series as it began", fmt.Sprintf("begin %+v; proof %+v", begin, proof))
	}
	if err := laneengine.RequirePolicyEngine(kernel.identity.Enrolled, laneengine.ProofEngines{Policy: proof.PolicyEngineDigest, Candidate: proof.CandidateEngineDigest}, reprove); err != nil {
		return inv.render(laneEngineResult(err, targets))
	}
	if err := evidence.Verify(proof); err != nil {
		return unproven("batch "+batchID+"'s retained proof no longer verifies for its tree, so nothing was published", reprove, "shows the batch; prove the series again", err.Error())
	}
	held, err := landing.HeldLane(kernel.installation, begin.Base, begin.Head, "origin", lane.MainRef, begin.LaneCommits)
	if err != nil || held.ExitCode != 0 {
		detail := fmt.Sprintf("held %+v: %v", held, err)
		if held.Refusal != nil {
			detail = held.Refusal.Code + " at " + held.Refusal.Commit + ": " + held.Refusal.Detail
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets,
			Summary: "a commit of batch " + batchID + " is not held by its goal, so nothing was published",
			next:    status, nextReason: "shows the batch's members; return the one that no longer holds its goal",
			Details: []string{"refused because: " + codeLanePublishNotHeld, detail}})
	}
	commit, err := lane.TrailedSeries(root, begin.Base, begin.Head, lane.ProofValue(batchID, proof.Attempt))
	if err != nil {
		return unproven("batch "+batchID+"'s series can't be prepared for main ("+oneLine(err.Error())+"), so nothing was published", status, "shows the batch", err.Error())
	}
	remote, err := kernel.owners.remoteURL(root)
	if err != nil {
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing checkout's origin can't be read, so nothing was published",
			next:    []string{"git", "-C", root, "remote", "-v"}, nextReason: "shows the checkout's remotes", Details: []string{err.Error()}})
	}
	published := landingPublication{Batch: batchID, Attempt: proof.Attempt, Base: begin.Base, Commit: commit, Tree: begin.Tree}
	if current, err := lane.RemoteMain(root, remote); err == nil && current == commit {
		// The same tuple is already on main (a repeat, or a push whose
		// answer was lost): success, and nothing is pushed.
		summary := "batch " + batchID + " is already on main as " + shortLandingID(commit)
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: published, Summary: summary, view: landingDone(summary, root)})
	}
	tuple := lane.Tuple{Repo: root, RemoteURL: remote, Ref: lane.MainRef, Old: begin.Base, New: commit, Tree: begin.Tree,
		Kind: lane.KindLanding, Op: batchID, ProofAttempt: proof.Attempt, Generation: kernel.identity.Enrolled.Generation}
	err = kernel.owners.publish(kernel.home, tuple)
	var refused *lane.PublishError
	switch {
	case err == nil:
		summary := "published batch " + batchID + " to main as " + shortLandingID(commit)
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: published, Summary: summary, view: landingDone(summary, root)})
	case errors.As(err, &refused) && refused.Code == lane.CodeBaseMoved:
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: published,
			Summary: fmt.Sprintf("main moved to %s since batch %s was composed on %s, so nothing was published", shortLandingID(refused.Current), batchID, shortLandingID(begin.Base)),
			next:    []string{"git", "-C", root, "fetch", "origin"}, nextReason: "then recompose the series on the new main and begin the batch again",
			Details: []string{"refused because: " + lane.CodeBaseMoved, refused.Error()}})
	case errors.As(err, &refused):
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: published,
			Summary: strings.TrimSuffix(refused.Message, ", so nothing was published") + ", so batch " + batchID + " was not published",
			next:    status, nextReason: "shows the lane and the batch",
			Details: []string{"refused because: " + refused.Code, refused.Error()}})
	}
	var gate *lane.Refusal
	if errors.As(err, &gate) {
		next := gate.Argv
		if len(next) == 0 {
			next = status
		}
		return inv.render(intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: gate.Message,
			next: next, nextReason: gate.Fix, Details: []string{"refused because: " + gate.Code}})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets,
		Summary: "batch " + batchID + " could not be published: " + oneLine(err.Error()),
		next:    status, nextReason: "shows the lane and the batch", Details: []string{err.Error()}})
}

// proofMismatch is why proof is not a green proof of begin's series; empty
// when it is.
func proofMismatch(batchID string, begin lane.BatchBegin, proof lane.ProofAttempt) string {
	switch {
	case begin.Batch != batchID || begin.Base == "" || begin.Head == "" || begin.Tree == "":
		return "the begin record is not this batch's series"
	case proof.Batch != batchID || proof.Attempt == "":
		return "the proof is another batch's"
	case proof.Subject != lane.SubjectBatch:
		return "it proved " + proof.Subject + ", not the whole batch"
	case proof.Outcome != lane.OutcomeGreen:
		return "it is " + proof.Outcome
	case proof.Base != begin.Base || proof.Commit != begin.Head:
		return "it proved another series"
	case proof.Tree != begin.Tree:
		return "it proved another tree"
	}
	return ""
}
