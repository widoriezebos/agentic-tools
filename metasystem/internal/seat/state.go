package seat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

const stateSchema = 1

// Outcome states the component records. The component evidence record stores
// a digest, not this; health's own counters count dead observations, not
// attempts, so the component keeps its own publication state.
const (
	OutcomePublished = "published"
	OutcomeSkipped   = "skipped"
	OutcomeFailed    = "failed"
)

// Skip reasons, named because health reads them.
const (
	SkipNoNickname = "no machine nickname"
	SkipUnarmed    = "an unarmed tick publishes no presence"
	SkipManualTick = "manual tick publishes no presence"
	SkipConflict   = "SEAT_PRESENCE_CONFLICT"
)

// PublicationState is the component's own record of its publishing, local to
// this machine and never published.
type PublicationState struct {
	Schema              int    `json:"schema"`
	Machine             string `json:"machine,omitempty"`
	LastAttemptAt       string `json:"lastAttemptAt,omitempty"`
	LastSuccessAt       string `json:"lastSuccessAt,omitempty"`
	LastOutcome         string `json:"lastOutcome,omitempty"`
	ConsecutiveFailures int    `json:"consecutiveFailures,omitempty"`
	Detail              string `json:"detail,omitempty"`
	// Rung is the ladder rung that last carried a record, remembered so the
	// next tick starts where the last one succeeded.
	Rung int `json:"rung,omitempty"`
	// TickSeconds is this machine's own cadence, so health judges its own
	// publishing against the same threshold every reader uses.
	TickSeconds int `json:"tickSeconds,omitempty"`
}

// PublicationStatePath is where the component keeps its publication state.
func PublicationStatePath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "seat-presence.json")
}

// StandingsPath is where the component keeps the standings it last read, so
// that a change of standing can be noticed exactly once.
func StandingsPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "seat-fleet.json")
}

// LoadPublicationState reads the publication state. An absent file is no
// publication yet, which is not an error; a torn one is, because health must
// say unknown rather than guess.
func LoadPublicationState(repoRoot string) (PublicationState, bool, error) {
	data, err := os.ReadFile(PublicationStatePath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return PublicationState{Schema: stateSchema}, false, nil
		}
		return PublicationState{}, false, err
	}
	var state PublicationState
	if err := json.Unmarshal(data, &state); err != nil {
		return PublicationState{}, true, fmt.Errorf("the seat-presence publication state is malformed: %w", err)
	}
	if state.Schema != stateSchema {
		return PublicationState{}, true, fmt.Errorf("the seat-presence publication state has schema %d, want %d", state.Schema, stateSchema)
	}
	return state, true, nil
}

// SavePublicationState publishes the state durably.
func SavePublicationState(repoRoot string, state PublicationState) error {
	state.Schema = stateSchema
	return writeJSON(PublicationStatePath(repoRoot), repoRoot, state)
}

// StandingsState is the standings the component last read, with each
// standing's frozen first observation.
type StandingsState struct {
	Schema   int                    `json:"schema"`
	ReadAt   string                 `json:"readAt,omitempty"`
	Machines map[string]Observation `json:"machines"`
}

// LoadStandings reads the standings file; an absent one is the baseline.
func LoadStandings(repoRoot string) (StandingsState, bool, error) {
	data, err := os.ReadFile(StandingsPath(repoRoot))
	if err != nil {
		if os.IsNotExist(err) {
			return StandingsState{Schema: stateSchema, Machines: map[string]Observation{}}, false, nil
		}
		return StandingsState{}, false, err
	}
	var state StandingsState
	if err := json.Unmarshal(data, &state); err != nil {
		return StandingsState{}, true, fmt.Errorf("the seat-fleet standings are malformed: %w", err)
	}
	if state.Schema != stateSchema {
		return StandingsState{}, true, fmt.Errorf("the seat-fleet standings have schema %d, want %d", state.Schema, stateSchema)
	}
	if state.Machines == nil {
		state.Machines = map[string]Observation{}
	}
	return state, true, nil
}

// SaveStandings publishes the standings durably.
func SaveStandings(repoRoot string, state StandingsState) error {
	state.Schema = stateSchema
	if state.Machines == nil {
		state.Machines = map[string]Observation{}
	}
	return writeJSON(StandingsPath(repoRoot), repoRoot, state)
}

func writeJSON(path, anchor string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), anchor)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("%s was published with durability unknown", filepath.Base(path))
	}
	return nil
}

// RecordPublished folds one successful publish into the state.
func RecordPublished(state PublicationState, machine string, rung Rung, tickSeconds int, detail string, at time.Time) PublicationState {
	state.Machine = machine
	state.LastAttemptAt = FormatTime(at)
	state.LastSuccessAt = FormatTime(at)
	state.LastOutcome = OutcomePublished
	state.ConsecutiveFailures = 0
	state.Detail = detail
	state.Rung = int(rung)
	state.TickSeconds = tickSeconds
	return state
}

// RecordSkipped folds one refusal to publish into the state. A skip is not a
// failure: it never advances the failure count.
func RecordSkipped(state PublicationState, reason string, at time.Time) PublicationState {
	state.LastAttemptAt = FormatTime(at)
	state.LastOutcome = OutcomeSkipped + ": " + reason
	state.Detail = reason
	return state
}

// RecordFailed folds one failed publish into the state, leaving the previous
// success where it was.
func RecordFailed(state PublicationState, detail string, at time.Time) PublicationState {
	state.LastAttemptAt = FormatTime(at)
	state.LastOutcome = OutcomeFailed + ": " + detail
	state.Detail = detail
	state.ConsecutiveFailures++
	return state
}

// SkippedFor reports the reason of a skipped outcome, or "".
func SkippedFor(state PublicationState) string {
	if !strings.HasPrefix(state.LastOutcome, OutcomeSkipped+": ") {
		return ""
	}
	return strings.TrimPrefix(state.LastOutcome, OutcomeSkipped+": ")
}
