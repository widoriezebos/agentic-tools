package seat

import (
	"fmt"
	"strings"
	"time"
)

// Verdict is one presence health judgment. The check decides only alive,
// dead or unknown and its reason; the consecutive-failure counting and the
// escalation belong to the health evaluator, which owns them for every role.
type Verdict struct {
	Status string
	Reason string
	Remedy string
}

// Health status words, the engine's own vocabulary.
const (
	StatusAlive   = "alive"
	StatusDead    = "dead"
	StatusUnknown = "unknown"
)

// RemedyRemote is what a human does about a presence that stopped reaching
// the remote; RemedyOneCheckout is what they do about two checkouts sharing
// one nickname.
const (
	RemedyRemote      = "check the ledger remote; the next tick republishes"
	RemedyOneCheckout = "one checkout per nickname"
)

// Health renders the verdict for the seat-presence role from the component's
// own publication state, in the precedence of the design's section 5:
// unreadable state is unknown; no attempt since arming is alive; a skip for
// no nickname is alive, because that is a configuration fact and not a
// fault; a conflict is dead; a last success older than the threshold is
// dead; otherwise alive, with the newest failure's detail appended while an
// earlier success is still fresh.
func Health(state PublicationState, readable bool, readErr error, now time.Time, window time.Duration) Verdict {
	if readErr != nil {
		return Verdict{Status: StatusUnknown, Reason: "the seat-presence publication state is unreadable: " + readErr.Error(), Remedy: RemedyRemote}
	}
	if !readable || state.LastAttemptAt == "" {
		return Verdict{Status: StatusAlive, Reason: "first publish pending"}
	}
	if reason := SkippedFor(state); reason != "" {
		if strings.HasPrefix(reason, SkipConflict) {
			return Verdict{Status: StatusDead, Reason: reason, Remedy: RemedyOneCheckout}
		}
		if reason == SkipNoNickname {
			return Verdict{Status: StatusAlive, Reason: "publishes no presence: " + SkipNoNickname}
		}
	}
	threshold := Threshold(window, state.TickSeconds)
	success, err := parsePresenceTime(state.LastSuccessAt)
	if state.LastSuccessAt == "" || err != nil {
		return Verdict{Status: StatusDead,
			Reason: "presence not published since arming: " + detailOrOutcome(state),
			Remedy: RemedyRemote}
	}
	if now.UTC().Sub(success) > threshold {
		return Verdict{Status: StatusDead,
			Reason: fmt.Sprintf("presence not published since %s: %s", state.LastSuccessAt, detailOrOutcome(state)),
			Remedy: RemedyRemote}
	}
	reason := fmt.Sprintf("presence published %s ago on rung %d, %s",
		roundedAge(now.UTC().Sub(success)), state.Rung, Rung(state.Rung).Description())
	if strings.HasPrefix(state.LastOutcome, OutcomeFailed+": ") {
		reason += "; newest attempt failed: " + state.Detail
	}
	return Verdict{Status: StatusAlive, Reason: reason}
}

func detailOrOutcome(state PublicationState) string {
	if state.Detail != "" {
		return state.Detail
	}
	if state.LastOutcome != "" {
		return state.LastOutcome
	}
	return "no publish has been attempted"
}
