package main

// landing push (plain lane step 4): the one way the landing agent puts work
// on main. HEAD goes on main only when results.jsonl says green for exactly
// HEAD's tree and origin's main is an ancestor of HEAD, leased at that main.
// Nothing else: a hand-in main then contains reads landed, and its seat
// concludes its goal.

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
)

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
	root := admitted.record.Root
	targets := laneTargets(root)
	if refused := inv.lanePaused(admitted, "pushed"); refused != nil {
		return inv.render(*refused)
	}
	checkout := string(admitted.layout.Checkout)
	outcome, err := admitted.owners.push(admitted.installation, checkout, admitted.owners.now())
	var told []string
	if outcome.Changed {
		// A landing on main is the one piece of news the channel carries
		// (Decision 7); a failed post is kept for a retry and fails nothing.
		if problem := postLanded(admitted.installation, landedMessage(admitted.installation, checkout, outcome.Old, outcome.Commit, admitted.owners.contained), outcome.Commit, admitted.owners.now()); problem != nil {
			told = []string{"the channel was not told of the landing; the next landing or tick retries once: " + problem.Error()}
		}
		told = append(told, inv.writeLandedCards(admitted, outcome)...)
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

func (inv *intentInvocation) boardHome() (string, error) {
	lookup := inv.owners.lookupEnv
	if lookup == nil {
		lookup = os.LookupEnv
	}
	return board.HomeWith(lookup)
}

// writeLandedCards follows every hand-in put on main, including superseded
// ones. Problems with the board are news about the card, never a failed push.
func (inv *intentInvocation) writeLandedCards(admitted laneAdmitted, outcome plain.PushOutcome) []string {
	entries, err := plain.Entries(admitted.installation)
	if err != nil {
		return []string{"the landed cards could not be read from the queue: " + err.Error()}
	}
	checkout := string(admitted.layout.Checkout)
	inHead := admitted.owners.contained(checkout, outcome.Commit)
	inOld := admitted.owners.contained(checkout, outcome.Old)
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
		problem := "no single live card holds the goal"
		if live {
			var count int
			count, err = admitted.owners.unitsOnMain(checkout, outcome.Commit, goal)
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
