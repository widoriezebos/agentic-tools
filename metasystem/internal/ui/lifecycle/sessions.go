package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
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

// ReadSessions reads what earlier runs left.
//
// A file that is not there is a checkout that has signed nobody in: the floor
// starts at zero and the sheet asks for a handle once. A file that is there
// and cannot be read, or that is written under a schema this build does not
// know, is an error and NOT a floor of zero — treating it as zero would make
// a corrupt or newer file into permission to spend every code again.
func ReadSessions(stateRoot string) (SessionFloor, error) {
	path := sessionsPath(stateRoot)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return SessionFloor{SchemaVersion: 1}, nil
	}
	if err != nil {
		return SessionFloor{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var floor SessionFloor
	if err := json.Unmarshal(data, &floor); err != nil {
		return SessionFloor{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	if floor.SchemaVersion != 1 {
		return SessionFloor{}, fmt.Errorf("%s is schema %d, which this build cannot read", path, floor.SchemaVersion)
	}
	return floor, nil
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
	current, err := ReadSessions(stateRoot)
	if err != nil {
		return err
	}
	if floor.LastStep < current.LastStep {
		return errors.New("the one-time-code floor cannot move backwards")
	}
	floor.SchemaVersion = 1
	data, marshalErr := json.Marshal(floor)
	if marshalErr != nil {
		return marshalErr
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
