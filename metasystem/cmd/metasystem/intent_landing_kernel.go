package main

// landing prove and landing push (simple lane): the landing agent's two
// deterministic rails over the lane checkout. prove runs the tests of the
// checkout's HEAD tree and records the result for that exact tree; push
// puts HEAD on main only when that tree was proven green, and only as a
// fast-forward. Both act under the pause (lane.Gate).

import (
	"errors"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/kernel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
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
		usage: []string{"metasystem landing prove [--tree T] [--wait]"},
		details: []string{"Runs the selected tests of the landing checkout's HEAD tree, the same ones a seat's own landing runs, on the lane's account, one run at a time on this computer.",
			"The proof belongs to the lane: it starts in the background and the command returns at once, so the proof outlives the session that asked for it. landing status shows it while it runs; the keeper wakes the landing agent when it ends. Asked again while that tree's proof runs, it starts nothing.",
			"The result is kept for that exact tree, which landing push reads, and recorded on every hand-off waiting in the lane. It ends green, red (a test failed) or unavailable (it could not run), which is never red.",
			"--tree proves another commit or tree of the checkout instead; --wait runs the proof in this command and says its result. Refused while the lane is stopped."},
		flags: []intentFlag{{name: "tree", value: "T", usage: "a commit or tree of the landing checkout (default: its HEAD)"},
			{name: "wait", usage: "run the proof here and wait for its result"},
			{name: "attempt", value: "ID", hidden: true, usage: "the attempt id a background start chose"}},
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
	request := kernel.ProveRequest{Home: admitted.home, Layout: admitted.layout, Tree: inv.input.text("tree"), Actor: actor, Attempt: inv.input.text("attempt")}
	if !inv.input.switched("wait") {
		return runIntentLandingProveDetached(inv, admitted, request)
	}
	proof, err := admitted.owners.prove(request)
	if err != nil {
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	tree := provenTreeWords(proof)
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

// provenTreeWords names a proof's subject: its commit and tree, or its tree.
func provenTreeWords(proof kernel.TreeProof) string {
	tree := "tree " + shortLandingID(proof.Tree)
	if proof.Commit != "" {
		tree = shortLandingID(proof.Commit) + " (" + tree + ")"
	}
	return tree
}

// runIntentLandingProveDetached starts the proof as the lane's own job and
// returns at once: the agent that asked may end its turn; the keeper wakes
// it when the proof ends. A repeat while that tree's proof runs is success.
func runIntentLandingProveDetached(inv *intentInvocation, admitted laneKernel, request kernel.ProveRequest) int {
	targets := laneTargets(admitted.record.Root)
	started, err := admitted.owners.startProof(request)
	if err != nil {
		return inv.render(landingKernelRefusal(inv, targets, err))
	}
	proof := started.Proof
	result := intentResult{Outcome: intentConfirmed, Targets: targets, Data: proof,
		Summary: "proving " + provenTreeWords(proof) + " as attempt " + proof.Attempt,
		next:    inv.publicArgv("landing", "status"), nextReason: "shows when it ends; the landing agent is woken then"}
	if started.Already {
		result.Outcome = intentUnchanged
		result.Summary = "already proving " + provenTreeWords(proof) + " as attempt " + proof.Attempt + ", since " + lane.LocalText(proof.StartedAt)
	}
	return inv.render(result)
}

// landingRunningProof is the lane's proof recorded running, as landing
// status shows it.
type landingRunningProof struct {
	Attempt string `json:"attempt"`
	Tree    string `json:"tree"`
	Commit  string `json:"commit,omitempty"`
	Since   string `json:"since"`
	// State is running while its process runs, died when it ended
	// without a result (the next prove runs it again).
	State string `json:"state"`
}

// readLandingRunningProof is the lane's running proof; nil when none is
// recorded running or it can't be read.
func readLandingRunningProof(layout lane.Layout) *landingRunningProof {
	proof, live, ok, err := kernel.ReadRunningProof(layout, identity.KernelProber{})
	if err != nil || !ok {
		return nil
	}
	state := "running"
	if live == identity.Dead {
		state = "died"
	}
	return &landingRunningProof{Attempt: proof.Attempt, Tree: proof.Tree, Commit: proof.Commit, Since: proof.StartedAt, State: state}
}

// laneProof is the lane's proof as the keeper's wake and landing status
// read it, at the lane checkout root.
func laneProof(root string) (lane.ProofFact, error) {
	layout, err := lane.NewLayout(root)
	if err != nil {
		return lane.ProofFact{}, err
	}
	return kernel.ReadProofFact(layout, identity.KernelProber{})
}

// withRunningProof adds the lane's running proof to landing status's page.
func withRunningProof(view func(*textui.Page), running *landingRunningProof) func(*textui.Page) {
	if running == nil {
		return view
	}
	return func(page *textui.Page) {
		view(page)
		since := lane.LocalText(running.Since)
		if at, err := time.Parse(time.RFC3339, running.Since); err == nil {
			since = page.Env().Since(at)
		}
		words := "proving tree " + shortLandingID(running.Tree) + " as attempt " + running.Attempt + ", " + since
		if running.State == "died" {
			words = "the test run of tree " + shortLandingID(running.Tree) + " (attempt " + running.Attempt + ") died without a result; the next landing prove runs it again"
		}
		page.Section("Tests", "").Text(words)
	}
}
