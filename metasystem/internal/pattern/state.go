package pattern

import (
	"encoding/json"
	"fmt"
)

// state is the pattern state the lane steward keeps between cycles
// (artifacts/agents/steward/patterns.json, written under the alerts lock).
type state struct {
	Schema int         `json:"schema"`
	Trunk  *trunkState `json:"trunk,omitempty"`
}

func decodeState(raw []byte) (state, error) {
	current := state{Schema: 1}
	if len(raw) == 0 {
		return current, nil
	}
	if err := json.Unmarshal(raw, &current); err != nil {
		return state{}, fmt.Errorf("the pattern state is unreadable: %w", err)
	}
	if current.Schema != 1 {
		return state{}, fmt.Errorf("the pattern state has schema %d, not 1", current.Schema)
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
