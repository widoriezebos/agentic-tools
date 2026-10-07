package plain

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/loopstop"
)

// Stop retains the shared loop decision beside the lane.
type Stop = loopstop.Stop

func stopsPath(install string) string { return filepath.Join(Dir(install), "stops.jsonl") }

// NewestStop is the newest stop for which no later decision closed its loop.
func NewestStop(install string) (*Stop, error) {
	lines, err := readLines[Stop](stopsPath(install))
	closed := map[string]bool{}
	for i := len(lines) - 1; i >= 0; i-- {
		s := lines[i]
		if s.Decision == "close" {
			closed[s.Loop+"\x00"+s.Subject] = true
		}
		if s.Decision == "stop" && !closed[s.Loop+"\x00"+s.Subject] {
			return &s, err
		}
	}
	return nil, err
}

func closeStopsLocked(install, loop, act string, now time.Time) error {
	lines, err := readLines[Stop](stopsPath(install))
	if err != nil {
		return err
	}
	latest := map[string]Stop{}
	for _, s := range lines {
		latest[s.Loop+"\x00"+s.Subject] = s
	}
	var keys []string
	for key := range latest {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		s := latest[key]
		if !slices.Contains([]string{"lane-proof", "lane-gate", "lane-return"}, s.Loop) || s.Decision == "close" || loop != "" && loop != s.Loop {
			continue
		}
		s.Decision, s.Handoff, s.At = "close", act, now.UTC().Format(time.RFC3339Nano)
		if err := appendLine(stopsPath(install), s); err != nil {
			return err
		}
	}
	return nil
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
				return closeStopsLocked(install, "lane-proof", "incident cleared "+incident, now)
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
	s = loopstop.Decide(loopstop.Input{Stop: s, Continue: s.Decision == "repeat"})
	if s.Decision == "continue" {
		s.Decision = "repeat"
	}
	return appendLine(stopsPath(install), s)
}

// RecordBarrenStop mirrors the keeper's existing hold without changing its count.
func RecordBarrenStop(install string, state lane.AgentState, evidence string, now time.Time) error {
	return withLock(install, func() error {
		if state.Barren == 0 {
			return closeStopsLocked(install, "lane-return", "lane changed", now)
		}
		s := Stop{Loop: "lane-return", Subject: "lane", Attempt: state.Barren, Budget: 2, Class: "lane unchanged", Decision: "stop", Handoff: "ask lane",
			Cause: &Cause{Kind: "unclassified"}, Evidence: evidence, At: now.UTC().Format(time.RFC3339Nano)}
		s.Measure.Name, s.Measure.Previous, s.Measure.Now = "lane fingerprint", []string{state.Fingerprint}, []string{state.BarrenFingerprint}
		last, err := NewestStop(install)
		if err != nil || last != nil && last.Loop == s.Loop && last.Evidence == s.Evidence {
			return err
		}
		return appendLine(stopsPath(install), s)
	})
}

// SyncStopQuestion withdraws only questions the lane itself asked for a stop.
// The channel's own question fields carry the command, stop line and evidence.
func SyncStopQuestion(install string, machine func(string) (string, error), now time.Time) error {
	return withLock(install, func() error {
		stop, err := NewestStop(install)
		if err != nil {
			return err
		}
		var facts []string
		if stop != nil && strings.HasPrefix(stop.Handoff, "ask ") {
			data, err := json.Marshal(stop)
			if err != nil {
				return err
			}
			facts = []string{stop.Command(), "lane stop: " + string(data), "evidence: " + stop.Evidence}
		}
		// The reference identifies our question even when its JSON is unreadable.
		// Other question files cannot hold the lane's synchronization.
		reference := filepath.Join(Dir(install), "stop-question")
		id, err := os.ReadFile(reference)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if len(facts) > 0 && len(id) > 0 {
			q, err := channel.ReadQuestion(install, string(id))
			if err != nil || q.ID != string(id) || channel.LaneStopCommand(q) == "" || q.OpenedAt.IsZero() || q.State == "" {
				return fmt.Errorf("the lane's stop question %s cannot be read: %v", id, err)
			}
		}
		questions, _ := channel.WalkQuestions(install)
		found := false
		name := ""
		if len(facts) > 0 || slices.ContainsFunc(questions, func(q channel.Question) bool { return channel.LaneStopCommand(q) != "" && q.State != "closed" }) {
			name, err = machine(install)
			if err != nil {
				return err
			}
		}
		for _, q := range questions {
			if channel.LaneStopCommand(q) == "" || q.Machine != name {
				continue
			}
			if len(facts) > 0 && slices.Equal(q.Facts, facts) {
				found = true
				if _, err := atomicfile.WriteFile(reference, []byte(q.ID), 0o600, ""); err != nil {
					return err
				}
			} else if q.State != "closed" {
				if _, err := channel.Withdraw(install, q.ID, "a later lane record ended the stop", nil, channel.DestinationConfig{}); err != nil {
					return err
				}
			}
		}
		if found || len(facts) == 0 {
			if len(facts) == 0 {
				if err := os.Remove(reference); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			return nil
		}
		q, err := channel.Ask(channel.AskRequest{RepoRoot: install, About: "lane", Kind: "other", Machine: name, Lineage: lane.AgentLineage,
			Facts: facts, Recommendation: "Run the command above; a later lane record closes this question.", Now: now})
		if err != nil {
			return err
		}
		_, err = atomicfile.WriteFile(reference, []byte(q.ID), 0o600, "")
		return err
	})
}
