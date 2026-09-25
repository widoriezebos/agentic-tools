// Package launch is one machine joining this fleet on this host: the record
// of a launch, and the sequencer that runs the join step by step.
//
// A machine is a clone beside this checkout with its own git configuration,
// its own engine and its own armed steward. The steps are the fleet-join
// design's (plans/fleet-join-bootstrap-design.md, sections 1 to 4), and every
// one of them is an existing owner's verb run as a subprocess: git, the kit's
// one build script, the engine's own config, goal, steward and up verbs.
// Nothing here encodes what those owners know; it orders them, records each
// outcome, and stops at the first refusal with the owner's own words.
//
// Two things make that recording load-bearing rather than decorative. A
// launch is long — a clone, a build and an arming — so the interface starts
// it detached and reads the record afterwards; and a launch can die, so the
// record carries the process identity that says whether the launch that wrote
// it is still alive.
//
// The design is plans/designs/user-interface/g1-s43-launch-a-machine-on-this-host.md.
package launch

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// SchemaVersion is the shape a reader parses. A reader accepts this version
// or a later one and ignores what it does not know.
const SchemaVersion = 1

// The four outcomes a launch ends in.
//
// Armed is its own outcome and not a failure: the machine is enrolled and its
// runner is up, and only the confirmation that it has published presence is
// missing. A page that called that failed would send a human to delete a
// working machine.
// Starting is the record as the interface writes it, before the verb it
// spawned has said which process it is. Nothing reconciles a starting record:
// there is no process identity in it to judge, and a reader that treated the
// absence of one as a dead process would call every launch dead in the
// moments between the act's answer and the verb's first write.
const (
	OutcomeStarting = "starting"
	OutcomeRunning  = "running"
	OutcomeDone     = "done"
	OutcomeArmed    = "armed"
	OutcomeFailed   = "failed"
)

// The outcomes one step takes. Skipped is a precondition that already held,
// which is what makes a second run of the verb idempotent.
const (
	StepPending = "pending"
	StepDone    = "done"
	StepSkipped = "skipped"
	StepFailed  = "failed"
	StepArmed   = "armed"
)

// The nine steps, by the names the record and the card use.
const (
	StepClone         = "clone"
	StepTracking      = "tracking"
	StepEngine        = "engine"
	StepConfiguration = "configuration"
	StepNickname      = "nickname"
	StepLedger        = "ledger"
	StepEnrollment    = "enrollment"
	StepSupervision   = "supervision"
	StepPresence      = "presence"
)

// Steps is the order the sequencer runs them in, which is also the order the
// card lists them in.
var Steps = []string{
	StepClone, StepTracking, StepEngine, StepConfiguration, StepNickname,
	StepLedger, StepEnrollment, StepSupervision, StepPresence,
}

// Process is the identity of the verb that is running this launch: the pid
// and the kernel's own start time for it, in whole seconds.
//
// The pair is what makes a dead launch recognisable. A pid alone is reused by
// the operating system, so a record naming a pid that some other program now
// carries would read as a launch still running.
type Process struct {
	PID       int   `json:"pid"`
	StartedAt int64 `json:"startedAt"`
}

// Created says what this launch made, and so what a resume of it may reuse.
// Everything else the preflight refuses to touch a second time.
type Created struct {
	Destination  bool `json:"destination"`
	Nickname     bool `json:"nickname"`
	EvidenceRoot bool `json:"evidenceRoot"`
}

// Step is one entry of the record: what was attempted, how it ended, when,
// and the owner's own words where it refused.
type Step struct {
	Step    string `json:"step"`
	Outcome string `json:"outcome"`
	At      string `json:"at"`
	Words   string `json:"words"`
}

// Next is the two commands a human has for a machine that has joined: start a
// session in it by hand, or stop it. Neither is an act this interface makes.
type Next struct {
	Session string `json:"session"`
	Stop    string `json:"stop"`
}

// Record is one launch, rewritten atomically after every step.
//
// The human's authorization is not in it and never travels into it: the word
// reaches the arming verb's argument list and nothing else.
type Record struct {
	SchemaVersion int     `json:"schemaVersion"`
	Launch        string  `json:"launch"`
	Machine       string  `json:"machine"`
	Destination   string  `json:"destination"`
	ClonedCommit  string  `json:"clonedCommit"`
	BuiltStamp    string  `json:"builtStamp"`
	Process       Process `json:"process"`
	StartedAt     string  `json:"startedAt"`
	EndedAt       *string `json:"endedAt"`
	Outcome       string  `json:"outcome"`
	ReviewBy      string  `json:"reviewBy"`
	Created       Created `json:"created"`
	Steps         []Step  `json:"steps"`
	Orientation   string  `json:"orientation"`
	Next          Next    `json:"next"`
}

// Dir is where this checkout keeps its launch records, one file per launch.
func Dir(checkout string) string {
	return filepath.Join(checkout, "artifacts", "agents", "ui", "launches")
}

// Path is one record's file. The launch id is a ULID, which is one path
// segment by construction; a caller that hands in anything else is refused
// rather than reaching the filesystem with it.
func Path(checkout, id string) (string, error) {
	if !ValidLaunchID(id) {
		return "", refuse(CodeIDInvalid, "%q is not a launch id", id)
	}
	return filepath.Join(Dir(checkout), id+".json"), nil
}

// ValidLaunchID reports whether an id is the one path segment a record is
// named by: the twenty-six upper-case characters and digits a ULID is made
// of, and nothing else.
func ValidLaunchID(id string) bool {
	if len(id) != 26 {
		return false
	}
	for _, char := range id {
		if (char < '0' || char > '9') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

// StepOf reports the entry for one step, and whether the record carries one.
func (r Record) StepOf(name string) (Step, bool) {
	for _, step := range r.Steps {
		if step.Step == name {
			return step, true
		}
	}
	return Step{}, false
}

// SetStep records one step's outcome, replacing the entry where the record
// already carries one so that a resume rewrites rather than appends.
func (r *Record) SetStep(step Step) {
	for index, held := range r.Steps {
		if held.Step == step.Step {
			r.Steps[index] = step
			return
		}
	}
	r.Steps = append(r.Steps, step)
}

// LastStep is the name of the step the record got furthest with, or "" for a
// record that has recorded none.
func (r Record) LastStep() string {
	if len(r.Steps) == 0 {
		return ""
	}
	return r.Steps[len(r.Steps)-1].Step
}

// Save writes one record atomically, beside the steward's other agent files.
func Save(checkout string, record Record) error {
	path, err := Path(checkout, record.Launch)
	if err != nil {
		return err
	}
	return SaveAt(path, record, checkout)
}

// SaveAt writes one record to a named file.
//
// It exists because the interface writes the record BEFORE the verb runs and
// then hands the verb the file it wrote: one launch has one record, and a
// verb that minted a second id would leave the page watching a launch nobody
// is running.
func SaveAt(path string, record Record, anchor string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	record.SchemaVersion = SchemaVersion
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(append(data, '\n')), anchor)
	return err
}

// LoadAt reads one record from a named file.
func LoadAt(path string) (Record, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Record{}, err
	}
	var record Record
	if err := json.Unmarshal(data, &record); err != nil {
		return Record{}, fmt.Errorf("the launch record at %s is malformed: %w", path, err)
	}
	return record, nil
}

// Load reads one record by its launch id.
func Load(checkout, id string) (Record, error) {
	path, err := Path(checkout, id)
	if err != nil {
		return Record{}, err
	}
	return LoadAt(path)
}

// List is every record on this host, newest first, so a page opened later
// still sees a running or a failed launch.
//
// A file that cannot be read or parsed is left out rather than failing the
// listing: one unreadable record must not cost the page every other one. An
// absent directory is a host that has launched nothing, which is not an
// error.
func List(checkout string) ([]Record, error) {
	entries, err := os.ReadDir(Dir(checkout))
	if err != nil {
		if os.IsNotExist(err) {
			return []Record{}, nil
		}
		return nil, err
	}
	records := []Record{}
	for _, entry := range entries {
		name, isRecord := strings.CutSuffix(entry.Name(), ".json")
		if entry.IsDir() || !isRecord || !ValidLaunchID(name) {
			continue
		}
		record, err := Load(checkout, name)
		if err != nil {
			continue
		}
		records = append(records, record)
	}
	// A launch id is a ULID, whose lexical order is its order in time, so
	// newest first is the reverse of the ids.
	sort.Slice(records, func(i, j int) bool { return records[i].Launch > records[j].Launch })
	return records, nil
}

// Alive reports whether the process a record names is still the process that
// started this launch. It is a function so that a test drives it.
type Alive func(process Process) bool

// Reconcile marks every running record whose launch process is dead as
// failed, and answers the list as it now stands, newest first.
//
// It runs wherever the records are read — the fleet resource, and the server
// at start — because a launch has no other way to say that it died: the verb
// rewrites the record after every step, and a verb that was killed between
// two of them leaves a record that says running forever. A record it rewrites
// is written back to disk, so the page and the next reader agree.
//
// It judges only records that carry a process identity. A record still says
// `starting` until the verb writes its own pid into it, and a reader that
// read the absence of an identity as a dead process would mark every launch
// failed in the moments between the act answering and the child's first
// write. A spawn that never happened is the act's own to record, and it
// records it as failed with the reason.
func Reconcile(checkout string, alive Alive, now time.Time) ([]Record, error) {
	records, err := List(checkout)
	if err != nil {
		return nil, err
	}
	for index, record := range records {
		if record.Outcome != OutcomeRunning || record.Process.PID <= 0 || alive(record.Process) {
			continue
		}
		ended := now.UTC().Format(time.RFC3339)
		record.Outcome = OutcomeFailed
		record.EndedAt = &ended
		words := "the launch process died"
		if last := record.LastStep(); last != "" {
			words = "the launch process died after " + last
		}
		record.SetStep(Step{Step: dyingStep(record), Outcome: StepFailed, At: ended, Words: words})
		// A record that cannot be written back is still reported as failed to
		// this reader: the process is gone either way, and a page that showed
		// it as running because a write failed would be the worse answer.
		_ = Save(checkout, record)
		records[index] = record
	}
	return records, nil
}

// dyingStep is the step a dead launch's failure is recorded against: the one
// it was in the middle of, which is the last one it wrote, or the first step
// for a launch that died before it wrote any.
func dyingStep(record Record) string {
	if last := record.LastStep(); last != "" {
		return last
	}
	return StepClone
}
