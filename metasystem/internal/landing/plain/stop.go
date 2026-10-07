package plain

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// Stop records a lane loop's decision and the evidence for its handoff.
type Stop struct {
	Loop    string `json:"loop"`
	Subject string `json:"subject"`
	Attempt int    `json:"attempt"`
	Budget  int    `json:"budget"`
	Measure struct {
		Name     string   `json:"name"`
		Previous []string `json:"previous"`
		Now      []string `json:"now"`
	} `json:"measure"`
	Class    string   `json:"class"`
	Decision string   `json:"decision"`
	Handoff  string   `json:"handoff"`
	Cause    *Cause   `json:"cause"`
	Evidence string   `json:"evidence"`
	At       string   `json:"at"`
	Required []string `json:"required,omitempty"`
}

func stopsPath(install string) string { return filepath.Join(Dir(install), "stops.jsonl") }

// Command is the one act that resolves this stop.
func (s Stop) Command() string {
	if len(s.Required) > 0 {
		return strings.Join(s.Required, " ")
	}
	if strings.HasPrefix(s.Handoff, "hold ") {
		return "metasystem incident list"
	}
	if s.Loop == "lane-return" || s.Cause != nil && s.Cause.Kind == "environment" {
		return "metasystem landing run"
	}
	goal := "GOAL"
	if s.Cause != nil && s.Cause.Goal != "" {
		goal = s.Cause.Goal
	}
	return "metasystem landing return " + goal + " --cause own --reason TEXT"
}

func (s Stop) Words() string {
	why := s.Class
	if s.Loop == "lane-return" {
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

// NewestStop is the newest stop for which no later decision closed its loop.
func NewestStop(install string) (*Stop, error) {
	lines, err := readLines[Stop](stopsPath(install))
	closed := map[string]bool{}
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		if s.Decision == "close" {
			closed[stopKey(s)] = true
		}
		if s.Decision == "stop" && !closed[stopKey(s)] {
			return &s, err
		}
	}
	return nil, err
}

func stopKey(s Stop) string { return fmt.Sprintf("%s/%s/%d", s.Loop, s.Subject, s.Attempt) }

func closeMatchingStopsLocked(install, act string, now time.Time, matches func(Stop) bool) error {
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil {
		return err
	}
	closed := map[string]bool{}
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		key := stopKey(s)
		if closed[key] {
			continue
		}
		closed[key] = true
		if s.Decision != "stop" || !matches(s) {
			continue
		}
		s.Decision, s.Handoff, s.At = "close", act, now.UTC().Format(time.RFC3339Nano)
		if err := appendLine(stopsPath(install), s); err != nil {
			return err
		}
	}
	return nil
}

func closeSubjectStopsLocked(install, loop, subject, act string, now time.Time) error {
	return closeMatchingStopsLocked(install, act, now, func(s Stop) bool { return s.Loop == loop && s.Subject == subject })
}

func closeGoalStopsLocked(install, goal, act string, now time.Time) error {
	return closeMatchingStopsLocked(install, act, now, func(s Stop) bool {
		return !(s.Loop == "lane-return" && s.Subject == "lane") && slices.Contains(strings.Split(s.Subject, ", "), goal)
	})
}

// CloseIncidentStop ends only the proof stop whose main incident cleared.
func CloseIncidentStop(install, incident string, now time.Time) error {
	return withLock(install, func() error {
		lines, err := readLines[Stop](stopsPath(install))
		if err != nil {
			return err
		}
		for i := len(lines) - 1; i >= 0; i-- {
			s := lines[i]
			if s.Loop != "lane-proof" {
				continue
			}
			if s.Decision == "stop" && s.Cause != nil && s.Cause.Kind == "main" && s.Cause.Name == incident {
				return closeMatchingStopsLocked(install, "incident cleared "+incident, now, func(stop Stop) bool {
					return stop.Loop == "lane-proof" && stop.Cause != nil && stop.Cause.Kind == "main" && stop.Cause.Name == incident
				})
			}
			break
		}
		return nil
	})
}

// recordProofStop runs under the lane lock after attribution and before a repeat.
func recordProofStop(install string, result Result) error {
	if result.Trunk || result.Result != Red || result.Cause == nil {
		return nil
	}
	s := Stop{Loop: "lane-proof", Budget: 2, Cause: result.Cause, Class: strings.Join(result.Cause.Tests, ", "), Evidence: result.Cause.Evidence, At: result.At}
	if !result.CountedFull && len(result.Goals) == 1 && result.Cause.Kind == "unclassified" {
		cause := *result.Cause
		cause.Goal = result.Goals[0].Goal
		s.Cause = &cause
	}
	if s.Class == "" {
		s.Class = result.Reason
	}
	var names []string
	for _, g := range result.Goals {
		names = append(names, g.Goal)
	}
	s.Subject = strings.Join(names, ", ")
	if s.Subject == "" {
		s.Subject = "main"
	}
	s.Measure.Name, s.Measure.Now = "red set", result.Cause.Tests
	mode := ProveSeams{Gate: result.Scope == "gate"}
	if mode.Gate {
		s.Loop, s.Measure.Name = "lane-gate", "gate red set"
	}
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return err
	}
	attempts := map[string]bool{}
	for _, r := range results {
		if mode.Gate && r.Tree != result.Tree || r.Trunk && !r.LoopClosed {
			continue
		}
		if r.LoopClosed || len(result.Goals) > 0 && !subsetGoals(result.Goals, r.Goals) {
			attempts = map[string]bool{}
			s.Measure.Previous = nil
			continue
		}
		if r.CountedFull {
			attempts[r.Attempt] = true
		}
		if r.Result == Red && r.Attempt != result.Attempt {
			s.Measure.Previous = failingTests(r.Failed)
		}
	}
	if result.CountedFull {
		attempts[result.Attempt] = true
	}
	s.Attempt = len(attempts)
	s.Decision, s.Handoff = "stop", "ask lane"
	switch result.Cause.Kind {
	case "flake":
		s.Decision, s.Handoff = "repeat", "landing prove"
	case "own":
		s.Handoff = "return " + result.Cause.Goal + " own"
	case "main":
		s.Handoff = "hold " + result.Cause.Name
	default:
		if result.Repeat == "allowed" && s.Attempt < s.Budget {
			s.Decision, s.Handoff = "repeat", "landing prove"
		}
	}
	if mode.Gate && s.Decision == "repeat" {
		s.Handoff = "landing prove --gate"
	}
	return appendLine(stopsPath(install), s)
}

// RecordBarrenStop mirrors the keeper's existing hold without changing its count.
func RecordBarrenStop(install string, state lane.AgentState, evidence string, now time.Time, effects ...ProveSeams) error {
	return withLock(install, func() error {
		if state.Barren == 0 {
			return closeSubjectStopsLocked(install, "lane-return", "lane", "superseded by lane progress", now)
		}
		s := Stop{Loop: "lane-return", Subject: "lane", Attempt: state.Barren, Budget: 2, Class: "lane unchanged", Decision: "stop", Handoff: "ask lane",
			Cause: &Cause{Kind: "unclassified"}, Evidence: evidence, At: now.UTC().Format(time.RFC3339Nano)}
		seams := ProveSeams{}
		if len(effects) > 0 {
			seams = effects[0]
		}
		checkout := install
		if seams.Lane != nil {
			record, err := seams.Lane()
			if err != nil {
				return err
			}
			checkout = record.Root
		} else if batch, err := ReadBatch(install); err == nil && batch != nil {
			checkout = batch.Lane.Root
		}
		eligible, err := pending(install, checkout, seams)
		if err != nil {
			return err
		}
		s.Required = []string{"metasystem", "landing", "prove", "--trunk"}
		if len(eligible) > 0 {
			goals := []string{}
			for _, e := range eligible {
				goals = append(goals, e.Goal)
			}
			s.Required = []string{"metasystem", "landing", "run", "--goals", strings.Join(goals, ",")}
		}
		s.Measure.Name, s.Measure.Previous, s.Measure.Now = "lane fingerprint", []string{state.Fingerprint}, []string{state.BarrenFingerprint}
		last, err := NewestStop(install)
		if err != nil || last != nil && last.Loop == s.Loop && last.Evidence == s.Evidence && slices.Equal(last.Required, s.Required) {
			return err
		}
		return appendLine(stopsPath(install), s)
	})
}
