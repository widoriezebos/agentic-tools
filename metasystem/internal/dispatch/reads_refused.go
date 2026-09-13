package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

type ReadRefusal struct {
	ID         string      `json:"id"`
	Reason     string      `json:"reason"`
	Role       string      `json:"role"`
	CriticRoot string      `json:"criticRoot"`
	Round      int64       `json:"round"`
	Subject    ReadSubject `json:"subject"`
	RefusedAt  string      `json:"refusedAt"`
}

func LoadReadRefusals(paths ...string) ([]ReadRefusal, error) {
	refusals := []ReadRefusal{}
	seen := map[string]bool{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read refusals from %s: %w", path, err)
		}
		for index, line := range bytes.Split(data, []byte{'\n'}) {
			line = bytes.TrimSpace(line)
			if len(line) == 0 {
				continue
			}
			var raw any
			if err := json.Unmarshal(line, &raw); err != nil {
				return nil, fmt.Errorf("%s line %d: %w", path, index+1, err)
			}
			object, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("%s line %d: read refusal must be an object", path, index+1)
			}
			var refusal ReadRefusal
			if err := json.Unmarshal(line, &refusal); err != nil {
				return nil, fmt.Errorf("%s line %d: %w", path, index+1, err)
			}
			if rawSubject, present := object["subject"]; present {
				subject, _, err := readsubject.DecodeReadSubject(rawSubject)
				if err != nil {
					return nil, fmt.Errorf("%s line %d: %w", path, index+1, err)
				}
				refusal.Subject = subject
			}
			if seen[refusal.ID] {
				continue
			}
			seen[refusal.ID] = true
			refusals = append(refusals, refusal)
		}
	}
	return refusals, nil
}
