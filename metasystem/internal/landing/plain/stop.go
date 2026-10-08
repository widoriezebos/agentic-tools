package plain

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// Stop retains the shared loop decision beside the lane.
type Stop = loopstop.Stop

func stopsPath(install string) string { return filepath.Join(Dir(install), "stops.jsonl") }

// NewestStop is the newest stop for which no later decision closed its loop.
func NewestStop(install string) (*Stop, error) {
	lines, err := readLines[Stop](stopsPath(install))
	closed := stopSet{}
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		if s.Decision == "close" {
			closed.add(s)
		}
		if s.Decision == "stop" && !closed.contains(s) {
			return &s, err
		}
	}
	return nil, err
}

// OpenStops are every stop no later decision closed, newest first.
func OpenStops(install string) ([]Stop, error) {
	lines, err := readLines[Stop](stopsPath(install))
	closed := stopSet{}
	var open []Stop
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		if s.Decision == "close" {
			closed.add(s)
		}
		if s.Decision == "stop" && !closed.contains(s) {
			open = append(open, s)
		}
	}
	return open, err
}

func stopKey(s Stop) string {
	if s.Decision == "close" && s.StoppedAt != nil {
		s.At = *s.StoppedAt
	}
	return fmt.Sprintf("%s/%s/%d/%s/%t/%s/%s", s.Loop, s.Subject, s.Attempt, s.Tree, s.Trunk, s.At, s.ProofAttempt)
}

// stopSet reads both timestamp-bound closures and the original loop identity.
// An absent opening timestamp retains the original loop/subject/attempt match.
type stopSet map[string]bool

func legacyStopKey(s Stop) string {
	return fmt.Sprintf("legacy/%s/%s/%d", s.Loop, s.Subject, s.Attempt)
}

func (seen stopSet) contains(s Stop) bool {
	return seen[stopKey(s)] || seen[legacyStopKey(s)]
}

func (seen stopSet) add(s Stop) {
	if s.Decision == "close" && s.StoppedAt == nil {
		seen[legacyStopKey(s)] = true
	} else {
		seen[stopKey(s)] = true
	}
}

func closeMatchingStopsLocked(install, act string, now time.Time, matches func(Stop) bool) error {
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil {
		return err
	}
	closed := stopSet{}
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		if closed.contains(s) {
			continue
		}
		closed.add(s)
		if s.Decision != "stop" || !matches(s) {
			continue
		}
		openedAt := s.At
		s.StoppedAt = &openedAt
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

func closeGoalStopsLocked(install, goal, act string, now time.Time, sha ...string) error {
	var proofs []Result
	if len(sha) > 0 {
		for _, path := range []string{resultsPath(install), gatesPath(install)} {
			results, err := readLines[Result](path)
			if err != nil {
				return err
			}
			proofs = append(proofs, results...)
		}
	}
	return closeMatchingStopsLocked(install, act, now, func(s Stop) bool {
		if s.Loop == "lane-return" {
			return s.Subject == goal && (len(sha) == 0 || s.Tree == "" || s.Tree == sha[0])
		}
		if s.Loop != "lane-proof" && s.Loop != "lane-gate" || s.Trunk || !slices.Contains(strings.Split(s.Subject, ", "), goal) {
			return false
		}
		command := strings.Fields(s.Command())
		if len(command) < 4 || strings.Join(command[:3], " ") != "metasystem landing return" || command[3] != goal && command[3] != "GOAL" {
			return false
		}
		if len(sha) == 0 {
			return true
		}
		// A proof tree identifies the batch, while its goal/commit pairs
		// identify the waiting hand-in that the return completes.
		for _, proof := range proofs {
			if proof.Tree == s.Tree && (s.ProofAttempt == "" || proof.Attempt == s.ProofAttempt) && slices.Contains(proof.Goals, GoalSHA{Goal: goal, SHA: sha[0]}) {
				return true
			}
		}
		// Regeneration and legacy stops may carry only the goal's commit.
		return s.Tree == "" && (s.Cause == nil || s.Cause.SHA == "" || s.Cause.SHA == sha[0])
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
	if result.ClassificationPending || result.Result != Red || result.Cause == nil || result.Trunk && result.Cause.Kind != "environment" {
		return nil
	}
	s := Stop{ProofAttempt: result.Attempt, Loop: "lane-proof", Tree: result.Tree, BatchID: result.BatchID, Scope: result.Scope, Trunk: result.Trunk, Budget: 2, Cause: result.Cause, Class: strings.Join(result.Cause.Tests, ", "), Evidence: result.Cause.Evidence, At: result.At}
	if len(result.Goals) == 1 && result.Cause.Kind == "unclassified" {
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
	if result.Repeat == "started" && len(result.FlakeRepeats) > 0 {
		s.Required = strings.Fields(proofCommand(mode.Gate, result.Trunk))
	}
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
	s = loopstop.Decide(loopstop.Input{Stop: s, Continue: s.Decision == "repeat"})
	if s.Decision == "continue" {
		s.Decision = "repeat"
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
		if err := closeSubjectStopsLocked(install, s.Loop, s.Subject, "superseded by current lane remedy", now); err != nil {
			return err
		}
		return appendLine(stopsPath(install), s)
	})
}
