package loopstop

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type Cause struct {
	Kind     string   `json:"kind"`
	Goal     string   `json:"goal,omitempty"`
	SHA      string   `json:"sha,omitempty"`
	Name     string   `json:"name,omitempty"`
	Tests    []string `json:"tests,omitempty"`
	Evidence string   `json:"evidence,omitempty"`
}

func (c Cause) Valid() bool {
	return slices.Contains([]string{"own", "main", "other", "flake", "environment", "unclassified"}, c.Kind)
}

type Stop struct {
	ProofAttempt string   `json:"proof-attempt,omitempty"`
	Tree         string   `json:"tree,omitempty"`
	BatchID      string   `json:"batch-id,omitempty"`
	Scope        string   `json:"scope,omitempty"`
	Trunk        bool     `json:"trunk,omitempty"`
	StoppedAt    *string  `json:"stopped-at,omitempty"`
	Required     []string `json:"required,omitempty"`
	Loop         string   `json:"loop"`
	Subject      string   `json:"subject"`
	Attempt      int      `json:"attempt"`
	Budget       int      `json:"budget"`
	Measure      struct {
		Name     string   `json:"name"`
		Previous []string `json:"previous"`
		Now      []string `json:"now"`
	} `json:"measure"`
	Class    string `json:"class"`
	Decision string `json:"decision"`
	Handoff  string `json:"handoff"`
	Cause    *Cause `json:"cause"`
	Evidence string `json:"evidence"`
	At       string `json:"at"`
}

type Input struct {
	Stop      Stop
	Prior     []readsubject.Read
	Fixed     []readsubject.Finding
	Inherited []readsubject.Finding
	Read      *readsubject.Read
	Policy    string
	Unknown   string
	// Lane continuation is already attributed by the lane's proof owner.
	Continue bool
}

// Decide contains no storage or command rendering. Each loop's owner supplies
// the budget and handoff and records this answer before its next effect.
func Decide(in Input) Stop {
	s := in.Stop
	if s.Decision == "repeat" {
		s.Decision = "continue"
	}
	if s.Loop != "unit-round" && s.Loop != "design-round" {
		if in.Continue {
			s.Decision = "continue"
		} else if s.Decision == "" {
			s.Decision = "stop"
		}
		return s
	}
	s.Decision = "stop"
	if in.Unknown != "" || in.Read == nil {
		s.Class = "unknown read inputs"
		if in.Unknown != "" {
			s.Handoff = "stopped " + in.Unknown
		}
		return s
	}
	s.Measure.Name = "material findings"
	s.Measure.Now = []string{strconv.Itoa(in.Read.Material)}
	known := map[string]readsubject.Finding{}
	for _, r := range in.Prior {
		for _, f := range r.Findings {
			known[f.ID] = f
		}
	}
	for _, f := range in.Inherited {
		known[f.ID] = f
	}
	for _, f := range in.Read.Findings {
		if f.Resolves != "" {
			prior, ok := known[f.Resolves]
			if !ok || prior.Class != f.Class || prior.Where != f.Where {
				s.Class = "invalid prior finding reference"
				s.Handoff = "stopped invalid-resolves"
				return s
			}
		}
	}
	if in.Read.Material == 0 {
		s.Decision, s.Handoff = "close", ""
		return s
	}
	if in.Policy == "person" {
		s.Class = "person holds review decisions"
		return s
	}
	for _, f := range in.Read.Findings {
		if !f.Material {
			continue
		}
		recurrence := in.Prior
		if s.Loop == "design-round" {
			recurrence = []readsubject.Read{{Findings: in.Fixed}}
		}
		for _, r := range recurrence {
			for _, p := range r.Findings {
				if !p.Material || p.Class != f.Class || s.Loop == "design-round" && p.Where != f.Where {
					continue
				}
				if f.Class == "other" && p.Relation != f.Relation {
					continue
				}
				s.Class = "repeated " + f.Class + " at " + f.Where
				return s
			}
		}
	}
	if len(in.Prior) > 0 {
		previous := in.Prior[len(in.Prior)-1].Material
		s.Measure.Previous = []string{strconv.Itoa(previous)}
		if in.Read.Material >= previous {
			s.Class = "material findings did not fall"
			return s
		}
	}
	if s.Attempt >= s.Budget {
		s.Class = "correction allowance spent"
		return s
	}
	s.Decision, s.Handoff, s.Class = "continue", "", "new material findings"
	return s
}

func (s Stop) Command() string {
	if len(s.Required) > 0 {
		return strings.Join(s.Required, " ")
	}
	if strings.HasPrefix(s.Handoff, "hold ") {
		return "metasystem incident list"
	}
	if (s.Loop == "lane-proof" || s.Loop == "lane-gate") && s.Cause != nil && s.Cause.Kind == "environment" {
		if s.Scope == "regeneration" {
			return "metasystem landing run"
		}
		command := "metasystem landing prove"
		if s.Loop == "lane-gate" {
			command += " --gate"
		} else if s.Trunk {
			command += " --trunk"
		}
		return command
	}
	if s.Loop == "lane-return" && s.Subject == "lane" {
		return "metasystem landing run"
	}
	goal := "GOAL"
	if s.Cause != nil && s.Cause.Goal != "" {
		goal = s.Cause.Goal
	}
	kind := "unclassified"
	if s.Cause != nil && s.Cause.Valid() {
		kind = s.Cause.Kind
	}
	return "metasystem landing return " + goal + " --cause " + kind + " --reason TEXT"
}

func (s Stop) Words() string {
	why := s.Class
	if s.Loop == "lane-return" && s.Subject == "lane" {
		why = fmt.Sprintf("%d launches left the lane unchanged", s.Attempt)
	} else if s.Cause != nil && s.Cause.Kind == "environment" {
		why = "the check could not complete twice"
	} else if s.Attempt >= s.Budget && s.Measure.Name == "red set" {
		why = fmt.Sprintf("%d full proofs went red; the latest failures are %s", s.Attempt, s.Class)
		if !slices.ContainsFunc(s.Measure.Previous, func(test string) bool { return !slices.Contains(s.Measure.Now, test) }) {
			why += "; the second showed no smaller red set"
		}
	}
	if s.Cause != nil {
		why += "; cause: " + s.Cause.Kind
		if s.Cause.Kind == "main" && s.Cause.Name != "" {
			why += "; incident: " + s.Cause.Name
		}
	}
	return "the lane stopped: " + why + ". Subject: " + s.Subject
}
