package host

import (
	"fmt"
	"path/filepath"
)

// FakeReturn builds the fake host's return object for a behavior marker,
// attesting the session the prompt declared and leaving the update arrays empty
// except for the one the behavior exercises. The dispatch-terminal behavior
// also writes a completed job record so the runner sees a terminal dispatch.
func FakeReturn(turnPath, statePath, outputPath, behavior, root string) error {
	turn := loadObject(turnPath)
	if turn == nil {
		return fmt.Errorf("fake return: turn record %s is unreadable", turnPath)
	}
	state := loadObject(statePath)
	if state == nil {
		return fmt.Errorf("fake return: mission state %s is unreadable", statePath)
	}
	streams, ok := state["streams"].(map[string]any)
	if !ok {
		return fmt.Errorf("fake return: mission state has no streams")
	}
	active, err := activeStream(streams)
	if err != nil {
		return err
	}

	turnID, err := requireField(turn, "turnId")
	if err != nil {
		return err
	}
	missionID, err := requireField(turn, "missionId")
	if err != nil {
		return err
	}
	cycle, err := requireField(turn, "cycle")
	if err != nil {
		return err
	}
	model, err := requireField(turn, "model")
	if err != nil {
		return err
	}

	value := map[string]any{
		"turnId":                 turnID,
		"missionId":              missionID,
		"cycle":                  cycle,
		"dispatched":             []any{},
		"certified":              []any{},
		"streamUpdatesRequested": []any{},
		"askCandidates":          []any{},
		"factsForLedger":         []any{},
		"gaps":                   []any{},
		// The return attests what the prompt declared (the host session, null on
		// a first or unresumable turn), never a session the adapter minted.
		"identity": map[string]any{
			"runtime":   "fake",
			"model":     model,
			"sessionId": turn["hostSession"],
		},
	}

	switch behavior {
	case "dispatch-ghost":
		value["dispatched"] = []any{map[string]any{
			"jobId":  fmt.Sprintf("ghost-%v", cycle),
			"role":   "implementer",
			"stream": active,
		}}
	case "dispatch-terminal":
		jobID := fmt.Sprintf("verifier-%v", missionID)
		startedAt, err := requireField(turn, "startedAt")
		if err != nil {
			return err
		}
		record := map[string]any{
			"jobId":              jobID,
			"role":               "verifier",
			"mission":            missionID,
			"turnId":             turnID,
			"runtime":            "fake",
			"round":              1,
			"parentJob":          nil,
			"status":             "completed",
			"endedAt":            startedAt,
			"capabilitySnapshot": "artifacts/agents/capabilities/fake.json",
			"usage":              nil,
			"mirror":             nil,
			"chainClosed":        false,
			"runnerClosed":       false,
		}
		recordPath := filepath.Join(root, "artifacts", "agents", "jobs", jobID+".json")
		if err := atomicWriteJSON(recordPath, record); err != nil {
			return fmt.Errorf("write fake job record: %w", err)
		}
		value["dispatched"] = []any{map[string]any{
			"jobId":  jobID,
			"role":   "verifier",
			"stream": active,
		}}
	case "close-stream":
		value["streamUpdatesRequested"] = []any{map[string]any{
			"streamId":       active,
			"requestedState": "done",
			"reason":         "done",
		}}
	case "park-request":
		value["streamUpdatesRequested"] = []any{map[string]any{
			"streamId":       active,
			"requestedState": "parked-reserved",
			"reason":         "fake-host-request",
		}}
	}

	if err := atomicWriteJSON(outputPath, value); err != nil {
		return fmt.Errorf("write fake return: %w", err)
	}
	return nil
}

// activeStream picks the first active stream in sorted name order, falling back
// to the first stream by name when none is active.
func activeStream(streams map[string]any) (string, error) {
	keys := sortedKeys(streams)
	if len(keys) == 0 {
		return "", fmt.Errorf("fake return: mission state has no streams")
	}
	for _, key := range keys {
		if stream, ok := streams[key].(map[string]any); ok {
			if s, _ := stream["state"].(string); s == "active" {
				return key, nil
			}
		}
	}
	return keys[0], nil
}

// requireField reads a field the fake return cannot be built without.
func requireField(object map[string]any, field string) (any, error) {
	value, ok := object[field]
	if !ok {
		return nil, fmt.Errorf("fake return: turn record is missing %q", field)
	}
	return value, nil
}
