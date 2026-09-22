package lifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// The two facts about browser sign-ins that must outlive one run of the
// server, and the file they live in.
//
// They are not in server.json, deliberately. That record is this run's — it is
// removed when the server stops and removed again when a stale one is found —
// and a replay floor that vanished with it would make a restart a way to spend
// a one-time code twice. So the floor has a file of its own beside it, written
// whenever it moves and read when a server starts.
//
// Neither fact is a credential. The floor is a step number, which authorizes
// nothing on its own; the handle is a name, which authorizes nothing either.
// No session and no proof is written here: a proof is an observation of this
// process, and a process that has stopped has none to leave behind.

// SessionFloor is what one run hands the next.
type SessionFloor struct {
	SchemaVersion int `json:"schemaVersion"`
	// LastStep is the highest one-time-code step any run of this checkout's
	// interface has accepted. A code at or below it is spent.
	LastStep int64 `json:"lastStep,omitempty"`
	// Human is the handle a browser named on a seat that configures none.
	Human string `json:"human,omitempty"`
}

func sessionsPath(stateRoot string) string { return filepath.Join(Dir(stateRoot), "sessions.json") }

// writingSessions serializes this process's writes of the floor, as Update
// serializes its writes of the record and for the same reason.
var writingSessions sync.Mutex

// ReadSessions reads what earlier runs left. A file that is not there, or that
// cannot be read as this schema, is a checkout that has signed nobody in:
// the floor starts at zero and the sheet asks for a handle once.
func ReadSessions(stateRoot string) SessionFloor {
	data, err := os.ReadFile(sessionsPath(stateRoot))
	if err != nil {
		return SessionFloor{SchemaVersion: 1}
	}
	var floor SessionFloor
	if err := json.Unmarshal(data, &floor); err != nil || floor.SchemaVersion != 1 {
		return SessionFloor{SchemaVersion: 1}
	}
	return floor
}

// WriteSessions records the floor durably. A floor that would go backwards is
// refused rather than written: the rule it exists for is that a spent step
// stays spent, and only a rule that cannot be walked back is one.
func WriteSessions(stateRoot string, floor SessionFloor) error {
	writingSessions.Lock()
	defer writingSessions.Unlock()
	if err := os.MkdirAll(Dir(stateRoot), 0o755); err != nil {
		return err
	}
	current := ReadSessions(stateRoot)
	if floor.LastStep < current.LastStep {
		return errors.New("the one-time-code floor cannot move backwards")
	}
	floor.SchemaVersion = 1
	data, err := json.Marshal(floor)
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(sessionsPath(stateRoot), string(data)+"\n", stateRoot)
	if err != nil {
		return err
	}
	if !durable {
		return errors.New("the one-time-code floor was written but its durability is unknown")
	}
	return nil
}
