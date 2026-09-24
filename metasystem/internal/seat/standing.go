package seat

import (
	"fmt"
	"sort"
	"time"
)

// A Standing is what one reader says about one machine at one moment. It is
// derived at read time and never stored on the record.
type Standing string

const (
	Reachable   Standing = "reachable"
	Unreachable Standing = "unreachable"
	Unknown     Standing = "unknown"
)

// DefaultStaleMinutes is seat.presence-stale-min's default.
const DefaultStaleMinutes = 30

// Threshold is the one inequality's right-hand side: the reader's own window
// or three of the writer's own ticks, whichever is longer, so a machine that
// ticks every forty minutes is not read dead by a reader that expected ten.
func Threshold(window time.Duration, tickSeconds int) time.Duration {
	writer := time.Duration(tickSeconds) * 3 * time.Second
	if writer > window {
		return writer
	}
	return window
}

// Judge derives one machine's standing from its record and the reader's own
// clock. A record from the future by less than the threshold is reachable
// with the clock named; further ahead than that it is unknown, the house
// pattern for clock disorder.
func Judge(record Record, now time.Time, window time.Duration) (Standing, string) {
	threshold := Threshold(window, record.TickSeconds)
	age := now.UTC().Sub(record.At())
	if age < 0 {
		ahead := -age
		if ahead <= threshold {
			return Reachable, fmt.Sprintf("presence published %s ago; clock ahead by %s", roundedAge(0), roundedAge(ahead))
		}
		return Unknown, fmt.Sprintf("clock ahead by %s", roundedAge(ahead))
	}
	if age <= threshold {
		return Reachable, fmt.Sprintf("presence published %s ago", roundedAge(age))
	}
	return Unreachable, fmt.Sprintf("no presence for %s, past %s", roundedAge(age), roundedAge(threshold))
}

// roundedAge renders a duration the way every presence line says it: minutes
// under an hour, then hours, then days.
func roundedAge(age time.Duration) string {
	switch {
	case age < time.Minute:
		return fmt.Sprintf("%d sec", int(age/time.Second))
	case age < time.Hour:
		return fmt.Sprintf("%d min", int(age/time.Minute))
	case age < 24*time.Hour:
		return fmt.Sprintf("%d h", int(age/time.Hour))
	default:
		return fmt.Sprintf("%d d", int(age/(24*time.Hour)))
	}
}

// MachineStanding is one line of the fleet: what the reader read, what it
// concluded, and which claims the ledger joins to it.
type MachineStanding struct {
	Machine  string   `json:"machine"`
	Standing Standing `json:"standing"`
	Reason   string   `json:"reason"`
	// AgeSeconds is the reader's clock minus tickAt; negative when the
	// record is dated ahead of the reader. Absent without a record.
	AgeSeconds *int64  `json:"ageSeconds,omitempty"`
	Record     *Record `json:"record,omitempty"`
	// Holds are the goals whose Claimed: line names this machine at the
	// captured accepted tip.
	Holds []string `json:"holds,omitempty"`
	// Since is the reader's own first observation of this standing, frozen
	// in the standings file and never recomputed.
	Since string `json:"since,omitempty"`
	// Malformed names the refusal when the record could not be read.
	Malformed string `json:"malformed,omitempty"`
	// Flag is the words a claim's row carries when its holder has gone
	// silent. It is a flag, never an act.
	Flag string `json:"flag,omitempty"`
	// This marks the reader's own machine.
	This bool `json:"this,omitempty"`
}

// Copy is the reader's local namespace as it was read: one parsed or refused
// record per machine.
type Copy struct {
	Records   map[string]Record
	Malformed map[string]string
}

// Join folds the two remote namespaces a presence fetch carries into one
// view per machine, the newest tickAt winning, so a machine that moved rungs
// is read from its live rung and its stale ref on the other is ignored. A
// record that could not be read counts only where no readable one exists.
func Join(copies ...Copy) Copy {
	joined := Copy{Records: map[string]Record{}, Malformed: map[string]string{}}
	for _, one := range copies {
		for machine, record := range one.Records {
			existing, seen := joined.Records[machine]
			if !seen || record.At().After(existing.At()) {
				joined.Records[machine] = record
			}
		}
		for machine, reason := range one.Malformed {
			if _, seen := joined.Malformed[machine]; !seen {
				joined.Malformed[machine] = reason
			}
		}
	}
	for machine := range joined.Records {
		delete(joined.Malformed, machine)
	}
	return joined
}

// FleetInput is everything one reading of the fleet needs.
type FleetInput struct {
	// This is the reader's own machine nickname, empty in a checkout with
	// no nickname.
	This string
	// Copy is the joined presence the reader read.
	Copy Copy
	// Claims maps machine to the goals whose Claimed: line names it at one
	// captured accepted tip.
	Claims map[string][]string
	// ClaimsUnavailable explains an accepted tip the reader could not read;
	// the reader then reports standings from the refs alone and raises no
	// flag.
	ClaimsUnavailable string
	// Previous is the standings file: the frozen since of each standing.
	Previous map[string]Observation
	Now      time.Time
	Window   time.Duration
}

// Observation is one frozen standing in the local standings file.
type Observation struct {
	Standing Standing `json:"standing"`
	Since    string   `json:"since"`
}

// Fleet derives every machine's standing. The machine set is the union of
// the presence refs the reader read and the machines the ledger names in a
// Claimed: line, so a claiming machine that never published is visible as
// unknown rather than invisible.
func Fleet(input FleetInput) []MachineStanding {
	now := input.Now.UTC()
	universe := map[string]bool{}
	for machine := range input.Copy.Records {
		universe[machine] = true
	}
	for machine := range input.Copy.Malformed {
		universe[machine] = true
	}
	for machine := range input.Claims {
		universe[machine] = true
	}
	if input.This != "" {
		universe[input.This] = true
	}
	names := make([]string, 0, len(universe))
	for machine := range universe {
		names = append(names, machine)
	}
	sort.Strings(names)
	standings := make([]MachineStanding, 0, len(names))
	for _, machine := range names {
		line := MachineStanding{Machine: machine, This: machine == input.This, Holds: input.Claims[machine]}
		switch {
		case input.Copy.Malformed[machine] != "":
			line.Standing, line.Reason = Unknown, "malformed presence"
			line.Malformed = input.Copy.Malformed[machine]
		default:
			record, published := input.Copy.Records[machine]
			if !published {
				line.Standing, line.Reason = Unknown, "no presence"
				break
			}
			held := record
			line.Record = &held
			age := int64(now.Sub(record.At()) / time.Second)
			line.AgeSeconds = &age
			line.Standing, line.Reason = Judge(record, now, input.Window)
		}
		line.Since = freezeSince(input.Previous, machine, line.Standing, now)
		if input.ClaimsUnavailable == "" {
			line.Flag = SilentHolder(line)
		}
		standings = append(standings, line)
	}
	sort.SliceStable(standings, func(i, j int) bool {
		if standings[i].This != standings[j].This {
			return standings[i].This
		}
		return standings[i].Machine < standings[j].Machine
	})
	return standings
}

// freezeSince keeps the reader's first observation of a standing. An unknown
// machine with no record ever observed has no since.
func freezeSince(previous map[string]Observation, machine string, standing Standing, now time.Time) string {
	if was, seen := previous[machine]; seen && was.Standing == standing && was.Since != "" {
		return was.Since
	}
	return FormatTime(now)
}

// Observations renders the standings for the local standings file.
func Observations(standings []MachineStanding) map[string]Observation {
	observed := make(map[string]Observation, len(standings))
	for _, line := range standings {
		observed[line.Machine] = Observation{Standing: line.Standing, Since: line.Since}
	}
	return observed
}

// SilentHolder is the flag a reader raises on a claim whose holder has gone
// silent. It is a flag, never an act: the goal stays claimed.
func SilentHolder(line MachineStanding) string {
	if line.Standing == Reachable || len(line.Holds) == 0 {
		return ""
	}
	return fmt.Sprintf("held by a machine %s since %s", line.Standing, line.Since)
}
