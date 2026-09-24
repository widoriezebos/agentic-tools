package seat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Report is one reading of the fleet: what `metasystem seat fleet` prints and
// what its --json hands the interface.
type Report struct {
	Machines []MachineStanding `json:"machines"`
	// This is the reader's own nickname; NoNickname says the checkout has
	// none and therefore publishes no presence.
	This       string `json:"this,omitempty"`
	NoNickname bool   `json:"noNickname,omitempty"`
	// ClaimsUnavailable explains an accepted tip that could not be read.
	ClaimsUnavailable string `json:"claimsUnavailable,omitempty"`
	// Publication is this machine's own publishing state, absent in a
	// checkout that never published.
	Publication *PublicationState `json:"publication,omitempty"`
	// CopyReadAt and CopySource say how old the presence copy is and who
	// fetched it.
	CopyReadAt string `json:"copyReadAt,omitempty"`
	CopySource string `json:"copySource,omitempty"`
	// CopyProblem names a fetch that failed; the previously fetched refs
	// still stand and are still reported.
	CopyProblem   string `json:"copyProblem,omitempty"`
	Now           string `json:"now"`
	WindowSeconds int    `json:"windowSeconds,omitempty"`

	// now is the injected clock every age in this report is measured
	// against; it is set by SetNow and never serialized.
	now time.Time
}

// SetNow records the reader's injected clock, so every age in the report is
// testable without the wall.
func (r *Report) SetNow(now time.Time) {
	r.now = now.UTC()
	r.Now = FormatTime(now)
}

// JSON renders the report for the interface.
func (r Report) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Text renders one line per machine, this machine first, then the reader's
// own publishing and the age of its presence copy.
func (r Report) Text() string {
	var out strings.Builder
	if r.NoNickname {
		out.WriteString("this checkout has no machine nickname and publishes no presence\n")
	}
	for _, line := range r.Machines {
		out.WriteString(r.machineLine(line))
		out.WriteString("\n")
	}
	if r.ClaimsUnavailable != "" {
		fmt.Fprintf(&out, "claims unavailable: %s\n", r.ClaimsUnavailable)
	}
	if r.Publication != nil {
		fmt.Fprintf(&out, "presence of this machine: %s\n", r.publicationLine())
	}
	if r.CopyReadAt != "" {
		fmt.Fprintf(&out, "presence copy fetched %s ago by %s\n", roundedAge(r.ageOf(r.CopyReadAt)), r.CopySource)
	}
	if r.CopyProblem != "" {
		fmt.Fprintf(&out, "presence copy problem: %s\n", r.CopyProblem)
	}
	return out.String()
}

func (r Report) machineLine(line MachineStanding) string {
	fields := []string{line.Machine, string(line.Standing)}
	if line.Record == nil {
		reason := line.Reason
		if len(line.Holds) > 0 {
			reason += "; named by the claim on " + strings.Join(line.Holds, ", ")
		}
		if line.Malformed != "" {
			reason += " (" + line.Malformed + ")"
		}
		return strings.Join(append(fields, reason), "  ")
	}
	record := *line.Record
	age := roundedAge(r.ageOf(record.TickAt)) + " ago"
	if r.ageOf(record.TickAt) < 0 {
		age = "dated ahead"
	}
	fields = append(fields, age,
		fmt.Sprintf("generation %d", record.Generation),
		"engine "+shortEngine(record.Engine),
		chainWords(record.Chain))
	if len(line.Holds) > 0 {
		holds := "holds " + strings.Join(line.Holds, ", ")
		if line.Flag != "" {
			holds += " (" + line.Flag + ")"
		}
		fields = append(fields, holds)
	}
	if line.Standing != Reachable {
		fields = append(fields, line.Reason)
	}
	return strings.Join(fields, "  ")
}

func (r Report) publicationLine() string {
	state := *r.Publication
	if state.LastOutcome == "" {
		return "no publish has been attempted"
	}
	if state.LastOutcome == OutcomePublished && state.LastSuccessAt != "" {
		return fmt.Sprintf("published %s ago on rung %d, %s",
			roundedAge(r.ageOf(state.LastSuccessAt)), state.Rung, Rung(state.Rung).Description())
	}
	return state.LastOutcome
}

func (r Report) ageOf(stamp string) time.Duration {
	at, err := parsePresenceTime(stamp)
	if err != nil {
		return 0
	}
	return r.now.Sub(at)
}

func chainWords(chain *Chain) string {
	if chain == nil {
		return "idle"
	}
	words := "running " + chain.Role
	if chain.Round > 0 {
		words += fmt.Sprintf(" round %d", chain.Round)
	}
	if chain.Goal != "" {
		words += " on " + chain.Goal
	}
	if chain.StartedAt == nil {
		words += " (not started)"
	}
	return words
}

func shortEngine(engine string) string {
	if len(engine) > 7 {
		return engine[:7]
	}
	return engine
}
