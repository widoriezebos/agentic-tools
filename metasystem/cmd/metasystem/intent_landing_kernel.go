package main

// landing prove and landing push (simple lane): the landing agent's two
// deterministic rails over the lane checkout. prove runs the tests of the
// checkout's HEAD tree and records the result for that exact tree; push
// puts HEAD on main only when that tree was proven green, and only as a
// fast-forward. Both act under the pause (lane.Gate).

import (
	"errors"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// laneKernel is a lane verb's admission: the registered lane and its
// installation.
type laneKernel struct {
	owners       laneVerbOwners
	home         string
	record       lane.Record
	layout       lane.Layout
	installation string
}

// laneKernelCommand declares a lane verb whose run starts only once the
// registered lane was read.
func laneKernelCommand(command intentCommand, run func(*intentInvocation, laneKernel) int) intentCommand {
	command.run = func(inv *intentInvocation) int {
		kernel, problem := inv.admitLaneKernel()
		if problem != nil {
			return inv.render(*problem)
		}
		return run(inv, kernel)
	}
	return command
}

// admitLaneKernel reads the registered lane and the installation landing
// set recorded for it, never guessed from the checkout again.
func (inv *intentInvocation) admitLaneKernel() (laneKernel, *intentResult) {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return laneKernel{}, problem
	}
	layout, err := record.Layout()
	if err != nil {
		return laneKernel{}, &intentResult{Outcome: intentFailed, code: 1, Targets: laneTargets(record.Root),
			Summary: "the landing lane's installation can't be found, so nothing was done",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's checkout",
			Details: []string{"the lane at " + record.Root + " has no metasystem installation: " + err.Error()}}
	}
	return laneKernel{owners: owners, home: home, record: record, layout: layout, installation: string(layout.Install)}, nil
}

func landingProveCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "prove", audience: "both", summary: "run the tests of the landing checkout's HEAD, the tree landing push may put on main",
		usage: []string{"metasystem landing prove [--tree T]"},
		details: []string{"Runs the selected tests of the landing checkout's HEAD tree, the same ones a seat's own landing runs, on the lane's account, one run at a time on this computer.",
			"The result is kept for that exact tree, which landing push reads, and recorded on every hand-off waiting in the lane. It ends green, red (a test failed) or unavailable (it could not run), which is never red.",
			"--tree proves another commit or tree of the checkout instead. Refused while the lane is stopped."},
		flags:    []intentFlag{{name: "tree", value: "T", usage: "a commit or tree of the landing checkout (default: its HEAD)"}},
		maxArgs:  0,
		examples: []string{"metasystem landing prove"},
	}, runIntentLandingProve)
}

// laneClaimActor is the lane's claim identity as a ledger actor: its
// machine and the lineage every joined goal is handed over to.
func laneClaimActor(kernel laneKernel) (string, error) {
	machine, err := kernel.owners.machine(kernel.record.Root)
	if err != nil {
		return "", err
	}
	return machine + "+" + lane.ClaimLineage, nil
}

func runIntentLandingProve(inv *intentInvocation, admitted laneKernel) int {
	targets := laneTargets(admitted.record.Root)
	actor, err := laneClaimActor(admitted)
	if err != nil {
		return inv.render(landingKernelFailure(targets, "the landing lane's machine can't be named, so nothing was run", err))
	}
	proof, err := admitted.owners.prove(kernel.ProveRequest{Home: admitted.home, Layout: admitted.layout, Tree: inv.input.text("tree"), Actor: actor})
	if err != nil {
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	tree := "tree " + shortLandingID(proof.Tree)
	if proof.Commit != "" {
		tree = shortLandingID(proof.Commit) + " (" + tree + ")"
	}
	switch proof.Status {
	case batch.AttemptGreen:
		summary := "the tests of " + tree + " passed; landing push may put it on main"
		return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: proof, Summary: summary, view: landingDone(summary, admitted.record.Root)})
	case batch.AttemptRed:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: proof,
			Summary: "tests of " + tree + " failed: " + strings.Join(proof.RedGroups, ", "),
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the waiting members; return the one that broke it"})
	}
	return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: proof,
		Summary: "the tests of " + tree + " could not run, which says nothing about the work: " + oneLine(proof.Reason),
		retry:   "runs them again once what stopped them is fixed", Details: []string{"attempt " + proof.Attempt + " is unavailable: " + proof.Reason}})
}

func landingKernelFailure(targets []intentTarget, summary string, err error) intentResult {
	return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: summary, retry: "tries again", Details: []string{err.Error()}}
}

// landingKernelRefusal renders a lane verb's refusal: the pause and the
// lane's own refusals name their command; prove's and push's say what to do
// and run the same command again.
func landingKernelRefusal(inv *intentInvocation, targets []intentTarget, err error) intentResult {
	var laneRefusal *lane.Refusal
	if errors.As(err, &laneRefusal) {
		return *laneRefusalResult(targets, laneRefusal)
	}
	var refusal *kernel.Refusal
	if !errors.As(err, &refusal) {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane could not do it: " + oneLine(err.Error()),
			retry: "tries again", Details: []string{err.Error()}}
	}
	result := intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: refusal.Reason,
		retry: refusal.Next, Details: []string{"refused because: " + refusal.Code}}
	if refusal.Code == kernel.CodePushUnproven {
		result.retry, result.next, result.nextReason = "", inv.publicArgv("landing", "prove"), "proves HEAD's tree; then run landing push again"
	}
	return result
}
