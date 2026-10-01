package main

// The landing lane's kernel verbs and their engine identity (lane runtime
// design §2, K5). A kernel verb acts only on the lane's enrolled engine: it
// hashes its own running executable and refuses when that is not the
// enrollment of the registered lane's installation. landing engine advance
// is the one way that engine changes.

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/laneengine"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

// laneKernelActions are the landing actions the lane's kernel owns (design
// §2 and §3; begin and prove arrived with K-b; publish, return and validate
// arrive with units K-c, K-d and K-f). Each is declared through laneKernelCommand, which
// checks the engine before the verb runs; TestEveryLaneKernelVerbChecksItsEngineFirst
// holds every one of them to it. landing return checks it first on every
// path but a person's --disposition person, which a person runs on any
// engine (intent_landing_return.go).
var laneKernelActions = map[string]bool{"begin": true, "prove": true, "publish": true, "return": true, "validate": true, "engine": true}

// laneKernel is a kernel verb's admission: the registered lane, its
// installation, and the engine identity RequireSelf established.
type laneKernel struct {
	owners       laneVerbOwners
	home         string
	record       lane.Record
	installation string
	identity     laneengine.Identity
}

// laneKernelCommand declares a kernel verb: its run starts only after the
// running executable was admitted as the lane's enrolled engine.
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

// admitLaneKernel is the first thing every kernel verb does.
func (inv *intentInvocation) admitLaneKernel() (laneKernel, *intentResult) {
	owners, home, record, problem := inv.laneContext(true)
	if problem != nil {
		return laneKernel{}, problem
	}
	targets := laneTargets(record.Root)
	// The installation is the one landing set recorded (K-a), never
	// guessed from the checkout again.
	layout, err := record.Layout()
	installation := string(layout.Install)
	if err != nil {
		return laneKernel{}, &intentResult{Outcome: intentFailed, code: 1, Targets: targets,
			Summary: "the landing lane's installation can't be found, so nothing was done",
			next:    inv.publicArgv("landing", "status", "--verbose"), nextReason: "shows the lane's checkout",
			Details: []string{"the lane at " + record.Root + " has no metasystem installation: " + err.Error()}}
	}
	identity, err := owners.engine(record.Root, installation, inv.typedArgv()[1:])
	if err != nil {
		result := laneEngineResult(err, targets)
		return laneKernel{}, &result
	}
	return laneKernel{owners: owners, home: home, record: record, installation: installation, identity: identity}, nil
}

// laneEngineResult renders an engine refusal as its two lines, the code and
// digests only in the details.
func laneEngineResult(err error, targets []intentTarget) intentResult {
	var refusal *laneengine.Refusal
	if !errors.As(err, &refusal) {
		return intentResult{Outcome: intentFailed, code: 1, Targets: targets, Summary: "the landing lane's engine couldn't be changed: " + oneLine(err.Error()),
			retry: "tries again", Details: []string{err.Error()}}
	}
	details := []string{"refused because: " + refusal.Code + ": " + refusal.Message}
	if refusal.Detail != "" {
		details = append(details, refusal.Detail)
	}
	reason := laneEngineFixReason(refusal)
	return intentResult{Outcome: intentRefused, code: 1, Targets: targets, Summary: refusal.Message,
		next: refusal.Argv, nextReason: reason, Details: details, viewsRefusal: true,
		view: func(page *textui.Page) {
			// Paths in the command read as the person's (~/…), as every
			// page's paths do; --json keeps them whole.
			argv := make([]string, len(refusal.Argv))
			for index, word := range refusal.Argv {
				argv[index] = word
				if filepath.IsAbs(word) {
					argv[index] = page.Env().Path(word)
				}
			}
			page.Refusal(refusal.Message, textui.Hint{Argv: argv, Reason: reason})
		}}
}

func laneEngineFixReason(refusal *laneengine.Refusal) string {
	switch {
	case len(refusal.Argv) > 1 && refusal.Argv[0] == "git":
		return "then run metasystem landing engine advance again"
	case slices.Equal(refusal.Argv, []string{"metasystem", "landing", "status"}):
		return "shows what holds the lane; then run the command again"
	case refusal.Code == laneengine.CodeNotEnrolled && len(refusal.Argv) > 0 && refusal.Argv[0] != "metasystem":
		return "runs it on the lane's enrolled engine"
	case refusal.Code == laneengine.CodeAdvanceCustodyLive:
		return "tries again once the tests have ended"
	case strings.HasPrefix(refusal.Code, "LANDING_LANE_"):
		// The lane gate's refusal (K-a) carries the lane's own fix.
		if reason, _, found := strings.Cut(refusal.Detail, ": metasystem "); found {
			return reason
		}
		return "then run metasystem landing engine advance again"
	}
	return "a person at a terminal no agent started arms the lane checkout"
}

func landingEngineCommand() intentCommand {
	return laneKernelCommand(intentCommand{
		object: "landing", action: "engine", audience: "both", summary: "move the landing lane to the engine built from landed main, between batches",
		usage: []string{"metasystem landing engine advance"},
		details: []string{"Builds the lane checkout's landed origin/main and makes it the lane's enrolled engine; it is the only way the lane's engine changes.",
			"Refused while the lane is stopped, while a batch is underway or landing tests still run, and when the checkout is not on landed main.",
			"Every landing kernel verb runs only on the lane's enrolled engine; any other executable, a hand-rebuilt one included, is refused.",
			"An engine already built from landed main changes nothing."},
		maxArgs:  1,
		examples: []string{"metasystem landing engine advance"},
	}, runIntentLandingEngine)
}

func runIntentLandingEngine(inv *intentInvocation, kernel laneKernel) int {
	targets := laneTargets(kernel.record.Root)
	if len(inv.input.args) != 1 || inv.input.args[0] != "advance" {
		return inv.render(intentResult{Outcome: intentRefused, code: 2, Targets: targets,
			Summary: "landing engine takes one action, advance; nothing was done",
			next:    inv.publicArgv("landing", "engine", "advance"), nextReason: "moves the lane to landed main's engine"})
	}
	outcome, err := kernel.owners.advance(laneengine.AdvanceRequest{Home: kernel.home, Checkout: kernel.record.Root,
		Installation: kernel.installation, Identity: kernel.identity})
	if err != nil {
		return inv.render(laneEngineResult(err, targets))
	}
	short := shortLandingID(outcome.Commit)
	if !outcome.Changed {
		summary := "the landing lane's engine is already landed main's build " + short
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: outcome, Summary: summary,
			view: landingDone(summary, kernel.record.Root)})
	}
	summary := "moved the landing lane to the engine built from landed main " + short
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: outcome, Summary: summary,
		view: landingDone(summary, kernel.record.Root)})
}
