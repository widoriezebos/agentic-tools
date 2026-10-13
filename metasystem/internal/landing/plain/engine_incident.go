package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type EngineIncident struct {
	Count  int       `json:"count"`
	LastAt time.Time `json:"last_at"`
	ID     string    `json:"id"`
	Verb   string    `json:"verb"`
	Args   []string  `json:"args"`
	At     time.Time `json:"at"`
	Panic  string    `json:"panic"`
	Stack  string    `json:"stack"`
	Engine string    `json:"engine_stamp"`
}

func RecordEngineIncident(install, verb string, args []string, at time.Time, panicText, stack string) (EngineIncident, error) {
	var incident EngineIncident
	err := withLock(install, func() error {
		incidents, err := EngineIncidents(install)
		if err != nil {
			return err
		}
		for _, open := range incidents {
			if open.Verb == verb && open.Panic == panicText {
				incident = open
				if incident.Count == 0 {
					incident.Count = 1
				}
				incident.Count++
				incident.LastAt = at.UTC()
				return writeAgentRecord(install, filepath.Join("incidents", incident.ID+".json"), incident)
			}
		}
		stamp, _ := shadowEngine(install)
		incident = EngineIncident{ID: at.UTC().Format("20060102T150405.000000000") + "-" + verb, Verb: verb, Args: args, At: at.UTC(), LastAt: at.UTC(), Count: 1, Panic: panicText, Stack: stack, Engine: stamp}
		// Different panic messages can arrive in the same clock tick.
		base := incident.ID
		for suffix := 1; ; suffix++ {
			_, err := os.Stat(filepath.Join(Dir(install), "incidents", incident.ID+".json"))
			if errors.Is(err, os.ErrNotExist) {
				break
			}
			if err != nil {
				return err
			}
			incident.ID = fmt.Sprintf("%s-%d", base, suffix)
		}
		return writeAgentRecord(install, filepath.Join("incidents", incident.ID+".json"), incident)
	})
	return incident, err
}
func EngineIncidents(install string) (incidents []EngineIncident, err error) {
	files, err := filepath.Glob(filepath.Join(Dir(install), "incidents", "*.json"))
	for _, path := range files {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil, readErr
		}
		var incident EngineIncident
		if err := json.Unmarshal(data, &incident); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		incidents = append(incidents, incident)
	}
	return incidents, err
}

// ClearEngineIncidents serializes clearing with recurrence recording.
func ClearEngineIncidents(install, id string, all bool) error {
	return withLock(install, func() error {
		ids := []string{id}
		if all {
			incidents, err := EngineIncidents(install)
			if err != nil {
				return err
			}
			ids = nil
			for _, incident := range incidents {
				ids = append(ids, incident.ID)
			}
		}
		for _, id := range ids {
			if filepath.Base(id) != id || id == "." || id == ".." || id == "" {
				return errors.New("invalid engine incident id")
			}
			err := os.Remove(filepath.Join(Dir(install), "incidents", id+".json"))
			if err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		return nil
	})
}
