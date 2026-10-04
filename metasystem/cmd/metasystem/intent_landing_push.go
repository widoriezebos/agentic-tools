package main

// landing push (plain lane step 4): the one way the landing agent puts work
// on main. HEAD goes on main only when results.jsonl says green for exactly
// HEAD's tree and origin's main is an ancestor of HEAD, leased at that main.
// Nothing else: a hand-in main then contains reads landed, and its seat
// concludes its goal.

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/designgate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/refusal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/textui"
)

type landingPushOwners struct {
	contains func(string, string) func(string) (bool, error)
	facts    func(string) landing.DesignFacts
	record   func(string, plain.DesignCheck) error
	notify   func(plain.PushOutcome) error
}

func (inv *intentInvocation) laneDesignPair(pair [2]string) {
	page := textui.NewLegacy(inv.textEnv(inv.stderr))
	page.Legacy(pair[0], pair[1])
	_, _ = io.WriteString(inv.stderr, page.String())
}

func (o landingPushOwners) withDefaults(inv *intentInvocation, admitted laneAdmitted) landingPushOwners {
	if o.contains == nil {
		o.contains = plain.ContainedIn
	}
	if o.facts == nil {
		o.facts = func(id string) landing.DesignFacts {
			reader, err := landingDesignInvocation(admitted.installation, inv.stderr)
			if err != nil {
				return landing.DesignFacts{Facts: designgate.Facts{Goal: id, Error: err}}
			}
			return reader.landingDesignFacts(reader.stateRoot, id)
		}
	}
	if o.record == nil {
		o.record = plain.RecordDesignCheck
	}
	if o.notify == nil {
		o.notify = func(outcome plain.PushOutcome) error {
			return postLanded(admitted.installation, landedMessage(admitted.installation, string(admitted.layout.Checkout), outcome.Old, outcome.Commit), outcome.Commit, admitted.owners.now())
		}
	}
	return o
}

func (inv *intentInvocation) checkLaneDesigns(admitted laneAdmitted, owners landingPushOwners, old, head string) *intentResult {
	waiting, err := plain.Waiting(admitted.installation)
	checkout := string(admitted.layout.Checkout)
	if err != nil {
		inv.laneDesignPair([2]string{fmt.Sprintf("warning: the lane's design check could not run (%s); the push goes on", oneLine(err.Error())), "metasystem landing status --verbose"})
		return nil
	}
	inHead, inOld := owners.contains(checkout, head), owners.contains(checkout, old)
	var refused *intentResult
	for _, entry := range waiting {
		now, headErr := inHead(entry.SHA)
		before, oldErr := inOld(entry.SHA)
		var facts landing.DesignFacts
		if err := errors.Join(headErr, oldErr); err != nil {
			facts = landing.DesignFacts{Facts: designgate.Facts{Goal: entry.Goal, Error: err}}
		} else if !now || before {
			continue
		} else {
			facts = owners.facts(entry.Goal)
		}
		design := landing.ObserveDesign(facts, false)
		if design.RefusesAgent {
			design.Pair[0] = strings.TrimSuffix(design.Pair[0], "; nothing was landed") + "; nothing was pushed"
			design.Pair[1] = "metasystem landing return " + entry.Goal + " --reason TEXT"
			if refused == nil {
				detail := "refused because: LANDING_DESIGN_NOT_STANDING"
				if ruling := refusal.GovernedBy["LANDING_DESIGN_NOT_STANDING"]; ruling != "" {
					detail += " governed-by=" + ruling
				}
				refused = &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(admitted.record.Root), Data: design,
					Summary: design.Pair[0], next: inv.publicArgv("landing", "return", entry.Goal, "--reason", "TEXT"),
					nextReason: "gives the goal back to its seat to restore its accepted design",
					Details:    []string{detail}}
			} else {
				inv.laneDesignPair(design.Pair)
			}
		} else if design.Pair[0] != "" {
			inv.laneDesignPair(design.Pair)
		}
		check := plain.DesignCheck{Goal: entry.Goal, Commit: entry.SHA, Verdict: design.Verdict, Reason: design.Pair[0], At: admitted.owners.now().UTC().Format(time.RFC3339)}
		if err := owners.record(admitted.installation, check); err != nil {
			inv.laneDesignPair([2]string{fmt.Sprintf("warning: the lane's design check record could not be written (%s); the check still stands", oneLine(err.Error())), "nothing to do: the next push checks the design again"})
		}
	}
	return refused
}

// landedMessage is the message of a push from old to head: the plain
// sentences of what the waiting hand-ins it put on main (head contains
// them, old did not) delivered, one per line in queue order. Hand-ins
// with no sentence add nothing; none at all is an empty message.
func landedMessage(install, checkout, old, head string) string {
	waiting, _ := plain.Waiting(install)
	inHead, inOld := plain.ContainedIn(checkout, head), plain.ContainedIn(checkout, old)
	var sentences []string
	for _, entry := range waiting {
		now, _ := inHead(entry.SHA)
		before, _ := inOld(entry.SHA)
		if now && !before && strings.TrimSpace(entry.Delivered) != "" {
			sentences = append(sentences, strings.TrimSpace(entry.Delivered))
		}
	}
	return strings.Join(sentences, "\n")
}

func landingPushCommand() intentCommand {
	return laneCommand(intentCommand{
		object: "landing", action: "push", audience: "agent", summary: "put the landing checkout's proven HEAD on main",
		usage: []string{"metasystem landing push"},
		details: []string{"Pushes the landing checkout's HEAD to main only when landing prove recorded green for HEAD's exact tree, and only when HEAD contains origin's main: main is never rewritten.",
			"Each hand-in whose commit main then contains reads landed; its seat sees it in work land and concludes the goal.",
			"Refused while the lane is stopped. A HEAD already on main pushes nothing."},
		maxArgs:  0,
		examples: []string{"metasystem landing push"},
	}, runIntentLandingPush)
}

func runIntentLandingPush(inv *intentInvocation, admitted laneAdmitted) int {
	return runIntentLandingPushWithOwners(inv, admitted, landingPushOwners{})
}

func runIntentLandingPushWithOwners(inv *intentInvocation, admitted laneAdmitted, owners landingPushOwners) int {
	root := admitted.record.Root
	targets := laneTargets(root)
	if refused := inv.lanePaused(admitted, "pushed"); refused != nil {
		return inv.render(*refused)
	}
	owners = owners.withDefaults(inv, admitted)
	checkout := string(admitted.layout.Checkout)
	var designRefusal *intentResult
	outcome, err := admitted.owners.push(admitted.installation, checkout, admitted.owners.now(), func(old, head string) error {
		designRefusal = inv.checkLaneDesigns(admitted, owners, old, head)
		if designRefusal != nil {
			return errors.New(designRefusal.Summary)
		}
		return nil
	})
	if designRefusal != nil {
		return inv.render(*designRefusal)
	}
	var told []string
	if outcome.Changed {
		// A landing on main is the one piece of news the channel carries
		// (Decision 7); a failed post is kept for a retry and fails nothing.
		if problem := owners.notify(outcome); problem != nil {
			told = []string{"the channel was not told of the landing; the next landing or tick retries once: " + problem.Error()}
		}
	}
	// The project's deploy follows a push that changed main; a failed
	// deploy is told here and by deploy status, never on the channel.
	if err == nil || outcome.Changed {
		told = append(told, inv.deployAfterPush(admitted.installation, checkout, outcome.Changed)...)
	}
	var refusal *plain.Refusal
	switch {
	case errors.As(err, &refusal):
		result := intentResult{Outcome: intentRefused, code: 1, Targets: targets, Data: outcome, Summary: refusal.Reason,
			Details: []string{"refused because: " + refusal.Code}}
		switch refusal.Code {
		case plain.CodeUnproven:
			result.next, result.nextReason = inv.publicArgv("landing", "prove"), "proves HEAD's tree; then push again"
		case plain.CodeRed:
			result.next, result.nextReason = inv.publicArgv("landing", "return", "GOAL", "--reason", "TEXT"), "gives the goal that broke it back to its seat"
		default:
			result.next, result.nextReason = []string{"git", "-C", root, "merge", "origin/main"}, "then prove and push again"
		}
		return inv.render(result)
	case err != nil && outcome.Changed:
		return inv.render(intentResult{Outcome: intentPartial, code: 1, Targets: targets, Data: outcome,
			Summary: "pushed " + shortLandingID(outcome.Commit) + " to main, but the push could not be recorded for landing status: " + oneLine(err.Error()),
			Details: append([]string{err.Error()}, told...)})
	case err != nil:
		return inv.render(intentResult{Outcome: intentFailed, code: 1, Targets: targets, Data: outcome,
			Summary: "the push could not be made: " + oneLine(err.Error()), retry: "tries again", Details: []string{err.Error()}})
	}
	if !outcome.Changed {
		summary := "main already is " + shortLandingID(outcome.Commit) + "; nothing was pushed"
		return inv.render(intentResult{Outcome: intentUnchanged, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root), Details: told})
	}
	summary := "pushed " + shortLandingID(outcome.Commit) + " to main (from " + shortLandingID(outcome.Old) + ")"
	return inv.render(intentResult{Outcome: intentConfirmed, Targets: targets, Data: outcome, Summary: summary, view: landingDone(summary, root), Details: told})
}
