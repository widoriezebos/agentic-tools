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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
)

// Lines is the whole reading, one row per line.
//
// Every instant is printed as it stands, in RFC 3339. This reader has no
// viewer whose clock it could render into — it answers a tool call, and the
// answer may be quoted anywhere — so an unambiguous instant is the honest
// form, and the page renders the same instants in the browser's own zone.
func (p Page) Lines(now time.Time) []string {
	lines := []string{}
	for _, held := range p.NeedsYou {
		lines = append(lines, "- Needs you: "+held.Goal+" is "+FlagWithInstant(held.Flag, held.Since)+
			"; a human steals or resumes it at a terminal")
	}
	lines = append(lines, "- This seat: "+p.thisLine())
	for _, machine := range p.Machines {
		lines = append(lines, "- "+p.machineLine(machine, now))
	}
	// The launches come after the machines because that is what they are:
	// a machine that is joining is not a seat of this fleet yet, and a launch
	// that failed is a directory on this host rather than a machine. Only the
	// two a human can still act on are printed — a launch that finished is
	// the row above it.
	for _, record := range p.Launches {
		if record.Outcome != launch.OutcomeRunning && record.Outcome != launch.OutcomeFailed {
			continue
		}
		lines = append(lines, "- Launch: "+launchLine(record))
	}
	return lines
}

// launchLine is one launch in words: what it is making, how far it got, and
// the owner's words where it stopped.
func launchLine(record launch.Record) string {
	parts := []string{record.Machine, record.Outcome, "into " + record.Destination}
	for _, step := range record.Steps {
		if step.Outcome == launch.StepFailed {
			parts = append(parts, "stopped at "+step.Step+": "+step.Words)
			break
		}
	}
	if record.Outcome == launch.OutcomeRunning && len(record.Steps) > 0 {
		parts = append(parts, "at "+record.LastStep())
	}
	return strings.Join(parts, "; ")
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

// MachineLines is one machine's disclosure, opened, as lines: the goal, the
// job in hand, the goal's box and the chain. It is what a reader with no
// screen gets when it asks about one machine rather than about the fleet.
//
// A machine elsewhere shows what its presence carried and is stamped with
// when that was published, because it is a reading of a reading. This seat's
// own block is read from the records here and needs no such stamp.
func (p Page) MachineLines(machine string, now time.Time) []string {
	for _, row := range p.Machines {
		if row.Machine != machine {
			continue
		}
		if row.WorkingProblem != "" {
			return []string{"- " + row.Machine + ": " + row.WorkingProblem}
		}
		if len(row.Working) == 0 {
			return []string{"- " + row.Machine + ": idle"}
		}
		lines := []string{}
		for _, working := range row.Working {
			lines = append(lines, workingLines(row, working, now)...)
		}
		return lines
	}
	return []string{"- this fleet has no machine called " + machine}
}

// workingLines is one thing a machine is doing, in the order the disclosure
// shows it.
func workingLines(row Machine, working seat.Working, now time.Time) []string {
	job := "- This job: " + seat.PhaseWords(&working, now)
	if cap := seat.CapWords(working.Job, now); cap != "" {
		job += "; " + cap
	}
	job += "; status " + working.Job.Status
	if working.Job.StartedAt != nil {
		job += ", started " + *working.Job.StartedAt
	}
	// A goal-free critique is lawful work, and a line naming an empty goal
	// would read as a goal nobody could find. The title comes off the holds
	// this row was composed with, so a goal the machine does not hold gets
	// its id and no title rather than an invented one.
	goal := "- Goal: " + working.Goal
	if working.Goal == "" {
		goal = "- Goal: this work names no goal"
	}
	for _, held := range row.Holds {
		if held.Goal == working.Goal && held.Title != "" {
			goal += " · " + held.Title
		}
	}
	lines := []string{goal, job, "- Box: " + seat.BoxWords(working.Box)}
	if working.Box != nil && working.Box.Problem == "" {
		lines = append(lines, "  - "+seat.ReservedMeaning)
	}
	lines = append(lines, "- Chain:")
	for _, member := range working.Chain {
		lines = append(lines, "  - "+member.Job+": "+seat.ChainWords(member))
	}
	if !row.This && row.Seen != "" {
		lines = append(lines, "- As published "+row.Seen)
	}
	return lines
}

func (p Page) machineLine(machine Machine, now time.Time) string {
	parts := []string{machine.Machine, machine.Standing}
	if machine.Seen != "" {
		parts = append(parts, "seen "+machine.Seen)
	}
	if machine.Reason != "" {
		parts = append(parts, machine.Reason)
	}
	if machine.Engine != "" {
		parts = append(parts, "engine "+shortEngine(machine.Engine)+
			", generation "+strconv.Itoa(machine.Generation))
	}
	parts = append(parts, phaseWords(machine, now))
	if len(machine.Holds) > 0 {
		goals := make([]string, 0, len(machine.Holds))
		for _, held := range machine.Holds {
			goals = append(goals, held.Goal)
		}
		holds := "holds " + strings.Join(goals, ", ")
		if flag := FlagWithInstant(machine.Holds[0].Flag, machine.Holds[0].Since); flag != "" {
			holds += " (" + flag + ")"
		}
		parts = append(parts, holds)
	}
	return strings.Join(parts, "; ")
}

// phaseWords is the one sentence a machine's row says about its work: the
// phase, how long the job in hand has run and the cap it reserved.
//
// The chain's older words stay as the second branch, because a fleet is not
// one build: a machine still publishing the chain alone is answered from the
// chain rather than reported idle. This seat's row can carry several things
// in flight, and the row names the newest and counts the rest.
func phaseWords(machine Machine, now time.Time) string {
	if machine.WorkingProblem != "" {
		return machine.WorkingProblem
	}
	if len(machine.Working) == 0 {
		return runningWords(machine.Running, "")
	}
	words := seat.PhaseWords(&machine.Working[0], now)
	if len(machine.Working) > 1 {
		words += " (and " + strconv.Itoa(len(machine.Working)-1) + " more in flight)"
	}
	return words
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
