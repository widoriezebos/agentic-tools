package launch

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"golang.org/x/sys/unix"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type State string

const (
	Starting  State = "starting"
	Running   State = "running"
	Completed State = "completed"
	Failed    State = "failed"
	Cancelled State = "cancelled"
)

func (s State) Terminal() bool { return s == Completed || s == Failed || s == Cancelled }

type Input struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}
type Output struct {
	Path  string `json:"path"`
	Bytes int64  `json:"bytes"`
}
type Measurement struct {
	Calls         int    `json:"calls"`
	Compactions   int    `json:"compactions"`
	PeakContext   int64  `json:"peakContext"`
	CallsAbove200 int    `json:"callsAbove200K"`
	Turns         int    `json:"turns"`
	ResultLines   int    `json:"resultLines"`
	ResultWords   int    `json:"resultWords"`
	ResultTail    string `json:"resultTail"`
	PageLines     int    `json:"pageLines"`
	PageWords     int    `json:"pageWords"`
	PageMissing   bool   `json:"pageMissing"`
	MaterialCount int    `json:"materialCount"`
	Verdict       string `json:"verdict"`
}
type Record struct {
	ID               string                     `json:"id"`
	Kind             string                     `json:"kind"`
	Adapter          string                     `json:"adapter"`
	Goal             string                     `json:"goal"`
	Tag              string                     `json:"tag"`
	WorkingDirectory string                     `json:"workingDirectory"`
	Inputs           []Input                    `json:"inputs"`
	State            State                      `json:"state"`
	Supervisor       *identity.Ref              `json:"supervisor"`
	Child            *identity.Ref              `json:"child"`
	ProcessGroup     *identity.Ref              `json:"processGroup"`
	StartedAt        string                     `json:"startedAt"`
	FinishedAt       string                     `json:"finishedAt"`
	ExitCode         *int                       `json:"exitCode"`
	Reason           string                     `json:"reason"`
	Measurement      Measurement                `json:"measurement"`
	Outputs          []Output                   `json:"outputs"`
	AdapterData      map[string]json.RawMessage `json:"adapterData"`
	DeclaredLines    int64                      `json:"declaredLines,omitempty"`
	ReadMode         string                     `json:"readMode,omitempty"`
	ReadPackage      string                     `json:"readPackage,omitempty"`
	ChangedLines     int64                      `json:"changedLines,omitempty"`
	VerdictCounts    *bool                      `json:"verdictCounts,omitempty"`
}

type Refusal struct {
	Time    string           `json:"time"`
	Code    string           `json:"code"`
	Kind    string           `json:"kind"`
	Goal    string           `json:"goal"`
	Tag     string           `json:"tag"`
	Numbers map[string]int64 `json:"numbers"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Store struct{ Root string }

func DefaultRoot() (string, error) {
	path, err := registry.DefaultPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "launch"), nil
}
func (s Store) root() (string, error) {
	if s.Root != "" {
		if !filepath.IsAbs(s.Root) {
			return "", fmt.Errorf("launch state root must be absolute")
		}
		return filepath.Clean(s.Root), nil
	}
	return DefaultRoot()
}
func (s Store) StateDir(id string) (string, error) {
	if !idPattern.MatchString(id) {
		return "", fmt.Errorf("invalid launch id %q", id)
	}
	root, err := s.root()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, id), nil
}
func (s Store) Create(record Record) error {
	dir, err := s.StateDir(record.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return s.withLock(record.ID, func() error {
		if _, err := os.Stat(filepath.Join(dir, "record.json")); err == nil {
			return fmt.Errorf("launch %s already exists", record.ID)
		} else if !os.IsNotExist(err) {
			return err
		}
		return s.write(dir, record)
	})
}
func (s Store) Read(id string) (Record, error) {
	dir, err := s.StateDir(id)
	if err != nil {
		return Record{}, err
	}
	data, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("launch %s record: %w", id, err)
	}
	if record.ID != id {
		return Record{}, fmt.Errorf("launch %s record names %q", id, record.ID)
	}
	return record, nil
}
func (s Store) Update(id string, change func(*Record) error) (Record, error) {
	var result Record
	err := s.withLock(id, func() error {
		record, err := s.Read(id)
		if err != nil {
			return err
		}
		before := record.State
		if err := change(&record); err != nil {
			return err
		}
		if before.Terminal() && record.State != before {
			return fmt.Errorf("terminal launch %s remains %s", id, before)
		}
		dir, _ := s.StateDir(id)
		if err := s.write(dir, record); err != nil {
			return err
		}
		result = record
		return nil
	})
	return result, err
}
func (s Store) List() ([]Record, error) {
	root, err := s.root()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var records []Record
	for _, entry := range entries {
		if !entry.IsDir() || !idPattern.MatchString(entry.Name()) {
			continue
		}
		record, err := s.Read(entry.Name())
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt < records[j].StartedAt })
	return records, nil
}
func (s Store) AppendRefusal(refusal Refusal) error {
	root, err := s.root()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(root, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	path := filepath.Join(root, "refusals.jsonl")
	content, readErr := os.ReadFile(path)
	if readErr != nil && !os.IsNotExist(readErr) {
		return readErr
	}
	row, err := json.Marshal(refusal)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(content)+string(row)+"\n", filepath.Dir(root))
	return err
}
func (s Store) Refusals() ([]Refusal, error) {
	root, err := s.root()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(root, "refusals.jsonl"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []Refusal
	for number, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row Refusal
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, fmt.Errorf("refusal row %d: %w", number+1, err)
		}
		result = append(result, row)
	}
	return result, nil
}
func (s Store) write(dir string, record Record) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(filepath.Join(dir, "record.json"), string(data)+"\n", filepath.Dir(dir))
	return err
}
func (s Store) withLock(id string, fn func() error) error {
	dir, err := s.StateDir(id)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		return err
	}
	defer unix.Flock(int(lock.Fd()), unix.LOCK_UN)
	return fn()
}
func newID(now time.Time) (string, error) {
	var random [5]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", err
	}
	return now.UTC().Format("20060102t150405") + "-" + hex.EncodeToString(random[:]), nil
}
