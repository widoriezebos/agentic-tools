package processmeasure

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type Act struct {
	ID, Actor, Lineage, Grant, Change string
	Requester, Applier                humanauthority.Proof
}
type Observation struct {
	Revision string
	Hours    float64
	Tokens   Tokens
}
type Watermark map[string]Observation
type Projection struct {
	Acts      []Act
	Measures  *Measures
	Lines     []string
	Watermark Watermark
}

// Report projects observed work under consumed acts; it makes no savings claim.
func Report(m Measures, steps []Step, acts []Act, stops []string, boundary Watermark) Projection {
	p := Projection{Measures: &m, Acts: slices.Clone(acts), Lines: slices.Clone(stops), Watermark: Watermark{}}
	for id, observation := range boundary {
		p.Watermark[id] = observation
	}
	revision := func(value any) string { body, _ := json.Marshal(value); return fmt.Sprintf("%x", sha256.Sum256(body)) }
	hours, tokens := map[string]float64{}, map[string]Tokens{}
	applied, seen := map[string]bool{}, map[string]bool{}
	for _, act := range acts {
		applied[act.ID] = true
		p.Watermark["act:"+act.ID] = Observation{Revision: revision(act)}
	}
	for _, step := range steps {
		if step.ID == "" || seen[step.ID] {
			continue
		}
		seen[step.ID] = true
		if step.Act == "" || !applied[step.Act] {
			continue
		}
		start, _ := time.Parse(time.RFC3339Nano, step.Start)
		end, _ := time.Parse(time.RFC3339Nano, step.End)
		if step.End == "" && !step.Terminal {
			end = m.ObservedAt
		}
		value, valid := duration(Interval{start, end}, m.ObservedAt)
		if step.Kind == "build" || step.Kind == "correction" || (step.Kind == "attest" && step.Coverage == "") {
			value -= nestedHours(step.ID, Interval{start, end}, steps, m.ObservedAt)
		}
		if !valid {
			m.Unknown = append(slices.Clone(m.Unknown), "consumed work time unavailable: "+step.ID)
			continue
		}
		observation := Observation{Hours: value}
		if step.Usage != nil && (step.Coverage == "" || step.Coverage == "separate") {
			observation.Tokens = *step.Usage
		}
		previous := boundary[step.ID]
		if step.Usage == nil {
			observation.Tokens = previous.Tokens
		}
		observation.Revision = revision([]any{step, observation.Hours, observation.Tokens})
		if value < previous.Hours || (step.Usage != nil && (observation.Tokens.Input < previous.Tokens.Input || observation.Tokens.CacheRead < previous.Tokens.CacheRead || observation.Tokens.CacheCreation < previous.Tokens.CacheCreation || observation.Tokens.Output < previous.Tokens.Output)) {
			m.Unknown = append(slices.Clone(m.Unknown), "observation regressed for "+step.ID)
			continue
		}
		p.Watermark[step.ID] = observation
		hours[step.Act] += value - previous.Hours
		delta := tokens[step.Act]
		delta.Input += observation.Tokens.Input - previous.Tokens.Input
		delta.CacheRead += observation.Tokens.CacheRead - previous.Tokens.CacheRead
		delta.CacheCreation += observation.Tokens.CacheCreation - previous.Tokens.CacheCreation
		delta.Output += observation.Tokens.Output - previous.Tokens.Output
		tokens[step.Act] = delta
		if step.Usage == nil && step.Kind != "attest" {
			m.Unknown = append(slices.Clone(m.Unknown), "consumed work tokens unavailable: "+step.ID)
		}
	}
	for _, act := range acts {
		actor, actual := "own process cost", act.Actor
		if act.Grant != "" {
			actual = "agent"
		}
		if act.Actor == "direct-person" && act.Grant == "" {
			actor = "person-directed process cost"
		}
		cost := "observed cost unavailable"
		if value, ok := hours[act.ID]; ok {
			cost = fmt.Sprintf("observed %.6g minutes; reported input/cache-read/cache-creation/output tokens %+v", value*60, tokens[act.ID])
		}
		p.Lines = append(p.Lines, fmt.Sprintf("Process act %s: %s; actor %s; lineage %s; grant %s; %s; %s; counterfactual unknown", act.ID, actor, actual, act.Lineage, act.Grant, act.Change, cost))
	}
	for _, kind := range []string{"build", "attest", "read", "correction", "pending", "collection", "person"} {
		label, scale, unit := "Recorded work: ", 1.0, " hours"
		if slices.Contains([]string{"pending", "collection", "person"}, kind) {
			label, scale, unit = "External delay: ", 60.0, " minutes"
		}
		cost := "unavailable"
		if value := m.Hours[kind]; value != nil {
			cost = fmt.Sprintf("%.6g", *value*scale) + unit
		}
		p.Lines = append(p.Lines, label+kind+" "+cost)
	}
	for _, child := range m.Children {
		usage, outcome := "unavailable", child.Outcome
		if child.Usage != nil {
			usage = fmt.Sprint(*child.Usage)
		}
		if !child.Terminal {
			outcome = "unfinished"
		}
		p.Lines = append(p.Lines, fmt.Sprintf("Check execution %s: %s; reported child input/cache-read/cache-creation/output tokens %s; token inclusion %s", child.ID, outcome, usage, child.Coverage))
	}
	if boundary == nil {
		p.Lines = append(p.Lines, "Unknown: report boundary; all retained applied acts included")
	}
	p.Lines = append(p.Lines, "Observed at "+m.ObservedAt.In(time.Local).Format(time.RFC3339)+"; unmatched work is unclassified; remote seat coverage unavailable")
	for _, unknown := range m.Unknown {
		p.Lines = append(p.Lines, "Unknown: "+unknown)
	}
	return p
}
