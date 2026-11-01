package humanauthority

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Decision binds an act in a person's name to its outcome and published effect.
type Decision struct {
	ID       string          `json:"id"`
	At       time.Time       `json:"at"`
	Actor    string          `json:"actor"`
	Lineage  string          `json:"lineage"`
	Machine  string          `json:"machine"`
	Grant    string          `json:"grant"`
	By       string          `json:"by"`
	Class    string          `json:"class"`
	Text     string          `json:"text"`
	Rejected string          `json:"rejected"`
	Context  string          `json:"context,omitempty"`
	Undo     string          `json:"undo"`
	Argv     []string        `json:"argv"`
	Outcome  string          `json:"outcome"`
	Words    string          `json:"words"`
	Effect   json.RawMessage `json:"effect"`
}

// DecisionLogPath names the host's append-only decision log.
func DecisionLogPath(home string) string {
	return filepath.Join(home, "artifacts", "agents", "authority", "decisions.jsonl")
}

// AppendDecision makes one decision durable before returning.
func AppendDecision(home string, d Decision) error {
	d.At = d.At.UTC()
	line, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return appendAuthorityLine(DecisionLogPath(home), string(line))
}

// ReadDecisions keeps readable lines and counts lines it cannot decode.
func ReadDecisions(home string) ([]Decision, int, error) {
	data, err := os.ReadFile(DecisionLogPath(home))
	if os.IsNotExist(err) {
		return nil, 0, nil
	}
	if err != nil || len(data) == 0 {
		return nil, 0, err
	}
	var decisions []Decision
	bad := 0
	for _, line := range bytes.Split(bytes.TrimSuffix(data, []byte("\n")), []byte("\n")) {
		var d Decision
		if err := json.Unmarshal(line, &d); err != nil {
			bad++
			continue
		}
		decisions = append(decisions, d)
	}
	return decisions, bad, nil
}
