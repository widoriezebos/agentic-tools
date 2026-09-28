package applaunch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// RecordSchemaVersion is the run record's schema.
const RecordSchemaVersion = 1

// StandingKey names the run this project's contract describes without
// --at: the standing application, on the contract's own address, with the
// contract's own data.
const StandingKey = "standing"

// Dir is the launch state directory beneath the state root, beside the
// interface's and the supervision's. A run record is per seat and never
// committed.
func Dir(stateRoot string) string { return filepath.Join(stateRoot, "artifacts", "agents", "app") }

// RunDir is where one run's own things live: its worktree, its state root
// where the contract's prepare makes data, and its log.
func RunDir(stateRoot, key string) string { return filepath.Join(Dir(stateRoot), "runs", key) }

func RecordPath(stateRoot, key string) string {
	return filepath.Join(Dir(stateRoot), key+".json")
}

func lockPath(stateRoot, key string) string {
	return filepath.Join(Dir(stateRoot), key+".flock")
}

// DefaultLogPath is the engine's own capture for a run whose contract names
// no log file of the application's.
func DefaultLogPath(stateRoot, key string) string {
	return filepath.Join(Dir(stateRoot), key+".log")
}

var keySafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

// KeyFor names one run. One run per ref at a time, so the ref is the key: a
// second start for the same ref rejoins or replaces that run and never the
// standing one.
func KeyFor(ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return StandingKey
	}
	safe := strings.Trim(keySafe.ReplaceAllString(ref, "-"), "-")
	sum := sha256.Sum256([]byte(ref))
	short := hex.EncodeToString(sum[:4])
	if safe == "" {
		return "at-" + short
	}
	if len(safe) > 40 {
		safe = safe[:40]
	}
	return "at-" + safe + "-" + short
}

// Ended is how a run that finished by itself is remembered: a record, not a
// live process.
type Ended struct {
	At         string `json:"at"`
	ExitStatus string `json:"exitStatus"`
}

// Check is the last verdict the contract's testing group returned against
// this run's address.
type Check struct {
	Group   string `json:"group"`
	Verdict string `json:"verdict"`
	At      string `json:"at"`
	Address string `json:"address,omitempty"`
}

// Record is what the supervisor writes before it spawns anything, and what
// every other verb reads. Nothing in it is authority; it is one seat's
// observation of one run.
type Record struct {
	SchemaVersion  int    `json:"schemaVersion"`
	Name           string `json:"name,omitempty"`
	Key            string `json:"key"`
	Ref            string `json:"ref,omitempty"`
	Goal           string `json:"goal,omitempty"`
	Commit         string `json:"commit,omitempty"`
	ContractDigest string `json:"contractDigest,omitempty"`
	// Supervisor is the app serve process's native identity ref, written
	// before the application is spawned. Group is the process group it leads
	// and the application is spawned into, so that a tree can be ended
	// through a group whose living leader is provably ours.
	Supervisor string `json:"supervisor"`
	Group      int64  `json:"group"`
	// Child is the application's own native identity ref, written as the very
	// next act after the spawn. A record with a group and no child is a
	// supervisor killed in that one instant, and status says exactly that.
	Child     string `json:"child,omitempty"`
	Address   string `json:"address,omitempty"`
	Log       string `json:"log,omitempty"`
	StateRoot string `json:"stateRoot,omitempty"`
	Tree      string `json:"tree,omitempty"`
	Data      string `json:"data,omitempty"`
	ReadyForm string `json:"readyForm,omitempty"`
	StartedAt string `json:"startedAt,omitempty"`
	ReadyAt   string `json:"readyAt,omitempty"`
	Prepared  bool   `json:"prepared,omitempty"`
	Ended     *Ended `json:"ended,omitempty"`
	Check     *Check `json:"check,omitempty"`
}

// SupervisorRef and ChildRef parse the recorded identities. A ref that will
// not parse is never signalled; it is refused by name.
func (r Record) SupervisorRef() (identity.Ref, error) { return identity.ParseRef(r.Supervisor) }

func (r Record) ChildRef() (identity.Ref, bool, error) {
	if r.Child == "" {
		return identity.Ref{}, false, nil
	}
	ref, err := identity.ParseRef(r.Child)
	return ref, true, err
}

// DataSentence is the words a record and a status use for a run's data.
func (r Record) DataSentence() string {
	if r.Data == DataOwn {
		return "data: own, made by prepare"
	}
	return "data: shared with the standing run"
}

var writing sync.Mutex

// WriteRecord replaces one run's record atomically.
func WriteRecord(stateRoot string, record Record) error {
	writing.Lock()
	defer writing.Unlock()
	return writeRecord(stateRoot, record)
}

func writeRecord(stateRoot string, record Record) error {
	if record.Key == "" {
		return errors.New("a run record needs its key")
	}
	record.SchemaVersion = RecordSchemaVersion
	if err := os.MkdirAll(Dir(stateRoot), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(RecordPath(stateRoot, record.Key), string(data)+"\n", stateRoot)
	return err
}

// UpdateRecord rewrites a run's record in place. A record that is gone is a
// run that was stopped; that is not an error and nothing is recreated.
func UpdateRecord(stateRoot, key string, apply func(*Record)) error {
	writing.Lock()
	defer writing.Unlock()
	record, err := ReadRecord(stateRoot, key)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	apply(record)
	return writeRecord(stateRoot, *record)
}

// ReadRecord reads one run's record.
func ReadRecord(stateRoot, key string) (*Record, error) {
	data, err := os.ReadFile(RecordPath(stateRoot, key))
	if err != nil {
		return nil, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, err
	}
	if record.SchemaVersion != RecordSchemaVersion {
		return nil, fmt.Errorf("unsupported application run record schema %d", record.SchemaVersion)
	}
	if _, err := identity.ParseRef(record.Supervisor); err != nil {
		return nil, err
	}
	if record.Key == "" {
		record.Key = key
	}
	return &record, nil
}

// RemoveRecord takes one run's record away. Only stop, reset and the next
// start of that ref do this, and only after the evidence copy, because an
// ended record is the last thing that says what the run did.
func RemoveRecord(stateRoot, key string) error {
	err := os.Remove(RecordPath(stateRoot, key))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Keys lists every run this seat has a record for, the standing one first.
func Keys(stateRoot string) ([]string, error) {
	entries, err := os.ReadDir(Dir(stateRoot))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var keys []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".json") {
			continue
		}
		keys = append(keys, strings.TrimSuffix(name, ".json"))
	}
	for index, key := range keys {
		if key == StandingKey && index != 0 {
			keys[0], keys[index] = keys[index], keys[0]
		}
	}
	return keys, nil
}
