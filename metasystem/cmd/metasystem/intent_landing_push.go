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
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
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
		o.contains = admitted.owners.contained
	}
	if o.contains == nil {
		o.contains = plain.ContainedIn
	}
	if o.facts == nil {
		o.facts = func(id string) landing.DesignFacts {
			reader, err := landingDesignInvocation(admitted.installation, inv.stderr)
			if err != nil {
				return landing.DesignFacts{Facts: designgate.Facts{Goal: id, Error: err}}
			}
			return reader.landingDesignFacts(reader.layout.InstallationRoot.Path(), id)
		}
	}
	if o.record == nil {
		o.record = plain.RecordDesignCheck
	}
	if o.notify == nil {
		contains := o.contains
		o.notify = func(outcome plain.PushOutcome) error {
			return postLanded(admitted.installation, landedMessage(admitted.installation, string(admitted.layout.Checkout), outcome.Old, outcome.Commit, contains), outcome.Commit, admitted.owners.now())
		}
	}
	return o
}

func (inv *intentInvocation) checkLaneDesigns(admitted laneAdmitted, owners landingPushOwners, old, head string) *intentResult {
	entries, err := plain.Entries(admitted.installation)
	checkout := string(admitted.layout.Checkout)
	if err != nil {
		inv.laneDesignPair([2]string{fmt.Sprintf("warning: the lane's design check could not run (%s); the push goes on", oneLine(err.Error())), "metasystem landing status --verbose"})
		return nil
	}
	inHead, inOld := owners.contains(checkout, head), owners.contains(checkout, old)
	rebuild := []string{"git", "-C", checkout, "checkout", "--detach", "origin/main"}
	rebuildReason := "rebuild the batch: git merge --no-ff SHA for each waiting sha, then metasystem landing prove"
	for _, entry := range entries {
		if entry.State != plain.StateReturned {
			continue
		}
		now, headErr := inHead(entry.SHA)
		before, oldErr := inOld(entry.SHA)
		if err := errors.Join(headErr, oldErr); err != nil {
			failed := landingLaneFailure(laneTargets(admitted.record.Root), "nothing was pushed: whether HEAD contains returned goal "+entry.Goal+" could not be read: "+oneLine(err.Error()), err)
			return &failed
		}
		if now && !before {
			return &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(admitted.record.Root),
				Summary: "HEAD still contains returned goal " + entry.Goal + "; nothing was pushed",
				next:    rebuild, nextReason: rebuildReason}
		}
	}
	var refused *intentResult
	for _, entry := range entries {
		if entry.State != plain.StateWaiting {
			continue
		}
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
			check := plain.DesignCheck{Goal: entry.Goal, Commit: entry.SHA, Verdict: design.Verdict, Reason: design.Pair[0]}
			if _, _, err := plain.ReturnDesignRefused(admitted.installation, check, admitted.owners.now()); err != nil {
				failed := landingLaneFailure(laneTargets(admitted.record.Root), "nothing was pushed: the design refusal of "+entry.Goal+" could not be returned: "+oneLine(err.Error()), err)
				return &failed
			}
			design.Pair[1] = textui.Command(rebuild) + " (" + rebuildReason + ")"
			if refused == nil {
				detail := "refused because: LANDING_DESIGN_NOT_STANDING"
				if ruling := refusal.GovernedBy["LANDING_DESIGN_NOT_STANDING"]; ruling != "" {
					detail += " governed-by=" + ruling
				}
				refused = &intentResult{Outcome: intentRefused, code: 1, Targets: laneTargets(admitted.record.Root), Data: design,
					Summary: strings.TrimSuffix(design.Pair[0], "; nothing was pushed") + "; goal " + entry.Goal + " was returned to its seat; nothing was pushed", next: rebuild,
					nextReason: rebuildReason,
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
func landedMessage(install, checkout, old, head string, owners ...func(string, string) func(string) (bool, error)) string {
	waiting, _ := plain.Waiting(install)
	contained := plain.ContainedIn
	if len(owners) > 0 {
		contained = owners[0]
	}
	inHead, inOld := contained(checkout, head), contained(checkout, old)
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
		told = append(told, inv.writeLandedCards(admitted, owners.contains, outcome)...)
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
			result.next, result.nextReason = inv.publicArgv("landing", "status"), "shows the cause and the waiting goals"
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

func (inv *intentInvocation) boardHome() (string, error) {
	lookup := inv.owners.lookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return board.HomeWith(lookup)
}

// writeLandedCards follows every hand-in put on main, including superseded
// ones. Problems with the board are news about the card, never a failed push.
func (inv *intentInvocation) writeLandedCards(admitted laneAdmitted, contains func(string, string) func(string) (bool, error), outcome plain.PushOutcome) []string {
	entries, err := plain.Entries(admitted.installation)
	if err != nil {
		return []string{"the landed cards could not be read from the queue: " + err.Error()}
	}
	checkout := string(admitted.layout.Checkout)
	inHead := contains(checkout, outcome.Commit)
	inOld := contains(checkout, outcome.Old)
	unitsOnMain := admitted.owners.unitsOnMain
	if unitsOnMain == nil {
		unitsOnMain = plain.UnitsOnMain
	}
	goals, pending, problems := []string{}, map[string]bool{}, map[string]string{}
	selected := map[string]bool{}
	for _, entry := range entries {
		if entry.State == plain.StateReturned {
			continue
		}
		now, err := inHead(entry.SHA)
		if err != nil {
			problems[entry.Goal] = err.Error()
			continue
		}
		if !now {
			if entry.State == plain.StateWaiting {
				pending[entry.Goal] = true
			}
			continue
		}
		before, err := inOld(entry.SHA)
		if err != nil {
			problems[entry.Goal] = err.Error()
		} else if !before && !selected[entry.Goal] {
			selected[entry.Goal] = true
			goals = append(goals, entry.Goal)
		}
	}
	var told []string
	for goal, problem := range problems {
		told = append(told, fmt.Sprintf("the landed card for %s was not written: %s", goal, problem))
	}
	if len(goals) == 0 {
		return told
	}
	home, err := inv.boardHome()
	if err != nil {
		return append(told, "the landed cards were not written: "+err.Error())
	}
	for _, goal := range goals {
		if problems[goal] != "" {
			continue
		}
		card, live := board.LiveCard(home, goal)
		problem := ""
		if live {
			var count int
			count, err = unitsOnMain(checkout, outcome.Commit, goal)
			if err == nil {
				problem = "the card ended or disappeared before the update"
				err = board.Update(home, card.Seat, goal, func(current board.Card) (board.Card, bool) {
					if current.Goal == "" || current.Stage.Terminal() {
						return current, false
					}
					current.Landed = count
					current.Writer.At = admitted.owners.now()
					if !current.Stage.ProcessBound() {
						current.Stage = board.StageClaimedIdle
						if pending[goal] {
							current.Stage = board.StageJoined
						}
						current.Owner, current.Job, current.Proof = nil, nil, nil
						current.Since = time.Time{}
						current.Writer.Component = "landing-push"
					}
					problem = ""
					return current, true
				})
			}
			if err != nil {
				problem = err.Error()
			}
		}
		if problem != "" {
			told = append(told, fmt.Sprintf("the landed card for %s was not written: %s", goal, problem))
		}
	}
	return told
}
