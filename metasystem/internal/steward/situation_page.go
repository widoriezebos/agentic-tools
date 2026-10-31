package steward

// The situation page is one composed read shared by every reader of this
// checkout: the tick writes it each pass, and a reader within its freshness
// prints it instead of reading again.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// SituationFresh is how long a saved page serves a reader.
const SituationFresh = 2 * time.Minute

// SituationRecord is the saved page: the composition and the facts it was
// composed from.
type SituationRecord struct {
	Composition Composition
	Situation   Situation
}

func situationPagePath(root string) string { return filepath.Join(runnerDir(root), "situation.json") }

// SituationPage returns this machine's saved page when it is at most fresh
// old; else it reads once, composes and saves. An unreadable page is none; a
// failed save is returned beside the page.
func SituationPage(root string, read func(time.Time) Situation, now time.Time, fresh time.Duration) (SituationRecord, error) {
	var saved SituationRecord
	data, err := os.ReadFile(situationPagePath(root))
	if err == nil {
		err = json.Unmarshal(data, &saved)
	}
	if err != nil {
		saved = SituationRecord{}
	} else if age := now.Sub(saved.Composition.ReadAt); age >= 0 && age <= fresh {
		if machine, err := goal.ResolveMachine(root); err == nil && machine == saved.Composition.Machine {
			return saved, nil
		}
	}
	if read == nil {
		return SituationRecord{}, errors.New("no situation reader")
	}
	s := read(now)
	record := SituationRecord{Composition: Compose(s), Situation: s}
	if record.Composition.Digest == saved.Composition.Digest && record.Composition.ReadAt.Equal(saved.Composition.ReadAt) {
		return record, nil
	}
	return record, writeJSONAtomic(situationPagePath(root), record)
}
