package seat

import (
	"fmt"
	"strings"
	"time"
)

// Notification is one human message the component queues when a peer's
// standing changes. Names only, which is all a presence record has.
type Notification struct {
	Nonce   string
	Message string
}

// Transitions compares the standings just read with the standings recorded
// at the previous tick and returns one notification per change, deduplicated
// by (machine, standing, since). The first read after arming has no previous
// standings and notifies nothing: it is the baseline.
//
// The caller queues every notification BEFORE persisting the new standings,
// which is ledger attention's order: a crash between the two repeats a
// notification, which delivery already permits, and never loses one.
func Transitions(previous map[string]Observation, standings []MachineStanding, baseline bool) []Notification {
	if baseline {
		return nil
	}
	var queued []Notification
	for _, line := range standings {
		if line.This {
			// This machine's own publishing is the health role's business,
			// not a peer notification.
			continue
		}
		was, seen := previous[line.Machine]
		if seen && was.Standing == line.Standing {
			continue
		}
		if !seen && line.Standing == Reachable {
			// A machine first seen already reachable is news of nothing.
			continue
		}
		queued = append(queued, Notification{
			Nonce:   NotificationNonce(line.Machine, line.Standing, line.Since),
			Message: transitionMessage(line),
		})
	}
	return queued
}

// NotificationNonce is the pending notification's file name, so it must be a
// plain one: the nickname charset of ValidateMachineName and a unix second
// make it so.
func NotificationNonce(machine string, standing Standing, since string) string {
	seconds := int64(0)
	if at, err := parsePresenceTime(since); err == nil {
		seconds = at.Unix()
	}
	return fmt.Sprintf("seat-presence-%s-%s-%d", machine, standing, seconds)
}

func transitionMessage(line MachineStanding) string {
	if line.Standing == Reachable {
		return fmt.Sprintf("%s is reachable again", line.Machine)
	}
	when := line.Since
	if at, err := parsePresenceTime(line.Since); err == nil {
		when = at.Format("15:04")
	}
	held := "holds no goals"
	if len(line.Holds) == 1 {
		held = "holds 1 goal: " + line.Holds[0]
	} else if len(line.Holds) > 1 {
		held = fmt.Sprintf("holds %d goals: %s", len(line.Holds), strings.Join(line.Holds, ", "))
	}
	return fmt.Sprintf("%s has been %s since %s and %s", line.Machine, line.Standing, when, held)
}

// NextStandings renders the standings file the component persists after its
// notifications are queued.
func NextStandings(standings []MachineStanding, now time.Time) StandingsState {
	return StandingsState{Schema: stateSchema, ReadAt: FormatTime(now), Machines: Observations(standings)}
}
