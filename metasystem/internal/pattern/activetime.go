package pattern

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// state is the pattern state the lane steward keeps between cycles
// (artifacts/agents/steward/patterns.json, written under the alerts lock).
type state struct {
	Schema  int                   `json:"schema"`
	Batches map[string]batchClock `json:"batches"`
}

// batchClock is one unfinished batch's observed active time (§1 Active
// time): the counted seconds, and what the last cycle observed.
type batchClock struct {
	ActiveSeconds  float64   `json:"activeSeconds"`
	LastObservedAt time.Time `json:"lastObservedAt"`
	LastHeld       bool      `json:"lastHeld"`
	// LastReadable says the batch and its hold readers were readable at the
	// last observation.
	LastReadable bool `json:"lastReadable"`
}

func decodeState(raw []byte) (state, error) {
	current := state{Schema: 1, Batches: map[string]batchClock{}}
	if len(raw) == 0 {
		return current, nil
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return state{}, fmt.Errorf("the pattern state is unreadable: %w", err)
	}
	if current.Schema != 1 {
		return state{}, fmt.Errorf("the pattern state has schema %d, not 1", current.Schema)
	}
	if current.Batches == nil {
		current.Batches = map[string]batchClock{}
	}
	return current, nil
}

func (s state) encode() ([]byte, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// account adds this cycle's interval to each unfinished batch's active time
// when, and only when, both of its ends were observed not held with every
// hold reader readable, the gap is at most maxGap, and the batch's own
// history confirms no hold inside it. Any other interval (a restart, an
// unreadable hold, a batch first seen mid-life) is not counted: being
// conservative can only delay a report, never invent one. It fills each
// batch's Active.
func (s *state) account(batches []BatchSignal, now time.Time, maxGap time.Duration) {
	present := map[string]bool{}
	for index := range batches {
		b := &batches[index]
		present[b.ID] = true
		if b.finished() {
			delete(s.Batches, b.ID)
			continue
		}
		previous, known := s.Batches[b.ID]
		readable := !b.Unreadable && !b.HoldUnreadable
		gap := now.Sub(previous.LastObservedAt)
		if known && previous.LastReadable && !previous.LastHeld && readable && !b.Held &&
			gap > 0 && gap <= maxGap && !holdConfirmed(b.Steps, previous.LastObservedAt, now) {
			previous.ActiveSeconds += gap.Seconds()
		}
		s.Batches[b.ID] = batchClock{ActiveSeconds: previous.ActiveSeconds, LastObservedAt: now.UTC(), LastHeld: b.Held, LastReadable: readable}
		b.Active = time.Duration(previous.ActiveSeconds * float64(time.Second))
	}
	for id := range s.Batches {
		if !present[id] {
			delete(s.Batches, id)
		}
	}
}

// unobserved is a cycle whose batch store could not be listed: every clock
// keeps its time, and the interval on either side of this cycle is not
// counted.
func (s *state) unobserved(now time.Time) {
	for id, clock := range s.Batches {
		clock.LastObservedAt, clock.LastReadable = now.UTC(), false
		s.Batches[id] = clock
	}
}

// holdConfirmed reports a typed history entry inside (from, to] that entered
// or left a held state: the batch history only confirms a hold, and nothing
// from it is ever added to active time.
func holdConfirmed(steps []Step, from, to time.Time) bool {
	for _, step := range steps {
		if step.At.After(from) && !step.At.After(to) && (strings.HasPrefix(step.To, "held-") || strings.HasPrefix(step.From, "held-")) {
			return true
		}
	}
	return false
}
