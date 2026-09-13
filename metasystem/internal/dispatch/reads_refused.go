package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/readsubject"
)

var writeReadRefusals = atomicfile.WriteText

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

func appendReadRefusalLocked(path, anchor string, event ReadRefusal) (durable bool, err error) {
	if err := validateReadRefusal(event); err != nil {
		return false, err
	}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("read refusals from %s: %w", path, err)
	}
	equalID := false
	for index, line := range bytes.Split(data, []byte{'\n'}) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		var existing ReadRefusal
		if err := json.Unmarshal(line, &existing); err != nil {
			return false, fmt.Errorf("%s line %d: %w", path, index+1, err)
		}
		if err := validateReadRefusal(existing); err != nil {
			return false, fmt.Errorf("%s line %d: %w", path, index+1, err)
		}
		if existing.ID != event.ID {
			continue
		}
		if reflect.DeepEqual(existing, event) {
			equalID = true
			continue
		}
		return false, fmt.Errorf("read refusal id %s already names different event content", event.ID)
	}
	if equalID {
		return false, nil
	}
	encoded, err := json.Marshal(event)
	if err != nil {
		return false, err
	}
	text := string(data)
	if len(data) > 0 && data[len(data)-1] != '\n' {
		text += "\n"
	}
	text += string(encoded) + "\n"
	return writeReadRefusals(path, text, anchor)
}

func validateReadRefusal(event ReadRefusal) error {
	if !validJobID.MatchString(event.ID) {
		return fmt.Errorf("read refusal id %q is invalid", event.ID)
	}
	if event.Reason != redundantReadRefusal {
		return fmt.Errorf("read refusal %s has unsupported reason %q", event.ID, event.Reason)
	}
	if !validJobID.MatchString(event.CriticRoot) || event.Round < 1 {
		return fmt.Errorf("read refusal %s has invalid critic coordinates", event.ID)
	}
	if err := validateReadSubjectForRole(event.Role, event.Subject); err != nil {
		return fmt.Errorf("read refusal %s: %w", event.ID, err)
	}
	refusedAt, err := time.Parse(time.RFC3339Nano, event.RefusedAt)
	if err != nil {
		return fmt.Errorf("read refusal %s has invalid refusal time", event.ID)
	}
	_, offset := refusedAt.Zone()
	if offset != 0 {
		return fmt.Errorf("read refusal %s refusal time is not UTC", event.ID)
	}
	return nil
}
