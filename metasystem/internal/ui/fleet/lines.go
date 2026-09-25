package fleet

// The page said in words, for the Partner's `fleet` tool and for the Fleet
// capture's own block. The page draws the same payload; these are the lines a
// reader that has no screen gets, in the same order the table is in: this
// seat first, then the machines, with the needs-you lines at the head because
// they are the one thing a human is asked to do something about.

import (
	"strconv"
	"strings"
	"time"
)

// Lines is the whole reading, one row per line.
func (p Page) Lines(now time.Time) []string {
	lines := []string{}
	for _, held := range p.NeedsYou {
		lines = append(lines, "- Needs you: "+held.Goal+" is "+held.Flag+"; a human steals or resumes it at a terminal")
	}
	lines = append(lines, "- This seat: "+p.thisLine())
	for _, machine := range p.Machines {
		lines = append(lines, "- "+p.machineLine(machine, now))
	}
	return lines
}

// Source is what a reading of this page is stamped with: where the presence
// copy came from, and which accepted tip the holders were read at. Both,
// because they are two readings and a row joins them.
func (p Page) Source() string {
	presence := "presence from " + p.Copy.Source
	switch {
	case p.Copy.SucceededAt != "":
		presence += ", fetched " + p.Copy.SucceededAt
	case p.Copy.AttemptedAt == "":
		presence += ", never fetched by this server"
	}
	if p.Copy.Problem != "" {
		presence += " (" + p.Copy.Problem + ")"
	}
	claims := "the accepted tip " + p.Claims.Tip
	if p.Claims.Tip == "" {
		claims = "no accepted tip"
	}
	if p.Claims.Unavailable != "" {
		claims = "claims unavailable: " + p.Claims.Unavailable
	}
	return presence + "; claims from " + claims
}

func (p Page) thisLine() string {
	seat := p.This
	if seat.NoNickname {
		return "this checkout has no machine nickname and publishes no presence"
	}
	parts := []string{seat.Machine, seat.Armed}
	if seat.Health != nil && seat.Health.Problem != "" {
		parts = append(parts, seat.Health.Problem)
	} else if seat.Health != nil {
		parts = append(parts, "health "+seat.Health.State+" last recorded "+seat.Health.ObservedAt)
	}
	switch {
	case seat.PublicationProblem != "":
		parts = append(parts, "publication state unreadable: "+seat.PublicationProblem)
	case seat.Publication == nil:
		parts = append(parts, "no publish has been attempted")
	case seat.Publication.LastOutcome != "":
		published := seat.Publication.LastOutcome
		if seat.Publication.LastSuccessAt != "" {
			published += " (last success " + seat.Publication.LastSuccessAt +
				" on rung " + strconv.Itoa(seat.Publication.Rung) + ")"
		}
		parts = append(parts, published)
	}
	parts = append(parts, runningWords(seat.Running, seat.RunningProblem))
	return strings.Join(parts, "; ")
}

func (p Page) machineLine(machine Machine, now time.Time) string {
	parts := []string{machine.Machine, machine.Standing}
	if machine.Seen != "" {
		parts = append(parts, "seen "+clockWords(machine.Seen, now))
	}
	if machine.Reason != "" {
		parts = append(parts, machine.Reason)
	}
	if machine.Engine != "" {
		parts = append(parts, "engine "+shortEngine(machine.Engine)+
			", generation "+strconv.Itoa(machine.Generation))
	}
	parts = append(parts, runningWords(machine.Running, ""))
	if len(machine.Holds) > 0 {
		goals := make([]string, 0, len(machine.Holds))
		for _, held := range machine.Holds {
			goals = append(goals, held.Goal)
		}
		holds := "holds " + strings.Join(goals, ", ")
		if flag := machine.Holds[0].Flag; flag != "" {
			holds += " (" + flag + ")"
		}
		parts = append(parts, holds)
	}
	return strings.Join(parts, "; ")
}

// runningWords says what one machine is running, keeping the pending
// distinction: a reservation that has not started says so rather than reading
// as work in flight.
func runningWords(running *Running, problem string) string {
	if problem != "" {
		return problem
	}
	if running == nil {
		return "idle"
	}
	words := "running " + running.Role
	if running.Round > 0 {
		words += " round " + strconv.Itoa(running.Round)
	}
	if running.Goal != "" {
		words += " on " + running.Goal
	}
	if running.StartedAt == nil {
		words += " (not started)"
	}
	return words
}

// shortEngine is the build stamp a human compares by eye, as every other
// reader of one writes it.
func shortEngine(engine string) string {
	if len(engine) > 7 {
		return engine[:7]
	}
	return engine
}
