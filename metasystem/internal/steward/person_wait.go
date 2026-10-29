package steward

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

func heldGoal(root, machine, command string, ev Evidence) (string, error) {
	if strings.HasPrefix(command, "metasystem goal approve ") {
		words := strings.Fields(command)
		return words[len(words)-1], nil
	}
	if ev.CurrentContinuation != "" {
		intent, err := ConsumedIntent(root, ev.CurrentContinuation)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if intent.Goal != "" {
			return intent.Goal, nil
		}
	}
	records, err := readSeatRecords(root)
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if record.LaunchID == ev.CurrentSeat && record.Goal != "" {
			return record.Goal, nil
		}
	}
	if len(records) > 0 && records[len(records)-1].Goal != "" {
		return records[len(records)-1].Goal, nil
	}
	return "seat:" + machine, nil
}

func askPersonWaits(root, machine string, d Decision, ev Evidence, work goal.ClaimableBudgetedWork, now time.Time) error {
	questions, unreadable := channel.WalkQuestions(root)
	if len(unreadable) > 0 {
		return fmt.Errorf("question store: %s", strings.Join(unreadable, "; "))
	}
	asked := map[channel.Decision]bool{}
	first, _, _ := strings.Cut(d.Reason, "\n")
	if cause, command, marked := strings.Cut(first, personRemedyMark); d.Action == ActNotify && marked {
		held, err := heldGoal(root, machine, command, ev)
		if err != nil {
			return err
		}
		words := strings.ReplaceAll(held, "-", " ")
		if strings.HasPrefix(held, "seat:") {
			words = "this seat"
		}
		key := channel.Decision{Kind: "steward-hold", Subject: held, State: cause}
		request := channel.AskRequest{RepoRoot: root, About: "machine", Kind: "other", Machine: machine, Contract: true, Now: now, Facts: []string{cause}, Decision: &key,
			Recommendation: "Revive it", Plain: &channel.Plain{Decision: "The seat on " + machine + " is held: " + cause + ". Nothing starts for " + words + " until it is revived.", IfNothing: "it stays down; this question stays open while the hold stands."},
			Options: []channel.Option{{Label: "Revive it", Act: []string{"metasystem", "machine", "revive", machine}, Consequence: "I clear the stale record and the seat starts again where the goal stands; no work is lost. Undo: metasystem work stop on the seat it starts."},
				{Label: "Leave it", Consequence: "the seat stays down and the goal waits; this question stays open."}}}
		if strings.HasPrefix(command, "metasystem goal approve ") {
			key.State = "restart cap"
			request.Recommendation = "Approve it again"
			request.Plain = &channel.Plain{Decision: words + ": " + cause + ", so I stopped starting seats for it.", IfNothing: "no seat starts for it; this question stays open."}
			request.Options[0] = channel.Option{Label: "Approve it again", Act: []string{"metasystem", "goal", "approve", held}, Consequence: "seats start again under a fresh approval; its claim and its box stay. Undo: metasystem goal pause " + held + " --reason TEXT parks it."}
		}
		if _, _, err := channel.AskOrFind(request); err != nil {
			return err
		}
		asked[key] = true
	}
	var retire []string
	for _, q := range questions {
		if q.Machine != machine || q.Decision == nil || !q.RetiredAt.IsZero() || asked[*q.Decision] || q.Decision.Kind != "steward-hold" || d.Verdict == VerdictDegraded || d.Verdict == VerdictUnknown {
			continue
		}
		cleared := false
		if q.Decision.State == "restart cap" {
			file, owned := work.OwnedClaim(q.Decision.Subject)
			cleared = !owned
			if owned {
				capped, err := SeatCapped(root, file.Id, approvalOpid(file))
				if err != nil {
					return err
				}
				cleared = !capped
			}
		} else {
			_, reason, err := abnormalRestartState(root, ev, now, nil)
			if err != nil {
				return err
			}
			cleared = reason == ""
		}
		if cleared {
			retire = append(retire, q.ID)
		}
	}
	for _, id := range retire {
		if err := channel.Retire(root, id, "the condition cleared", now); err != nil {
			return err
		}
	}
	return nil
}
