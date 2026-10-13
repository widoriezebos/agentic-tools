// Package state owns the landing shadow and its event history.
package state

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type Since struct {
	At       time.Time `json:"at"`
	Observed bool      `json:"observed"`
	Source   string    `json:"source,omitempty"`
}
type Fact struct {
	Word  string `json:"word"`
	Since Since  `json:"since"`
}
type Lane struct {
	Fact   `json:",inline"`
	By     string `json:"by"`
	Reason string `json:"reason"`
}
type Agent struct {
	Session string `json:"session"`
	Word    string `json:"word"`
	Step    Step   `json:"step"`
	Since   Since  `json:"since"`
}
type Step struct {
	Verb      string   `json:"verb"`
	Args      []string `json:"args"`
	StartedAt string   `json:"started_at"`
}
type Observation struct {
	At       time.Time `json:"at"`
	Problems []string  `json:"problems"`
}
type Member struct {
	Goal string `json:"goal"`
	SHA  string `json:"sha"`
}
type Batch struct {
	SealedAt  time.Time `json:"sealed_at"`
	MainMove  string    `json:"main_move"`
	ID        string    `json:"id"`
	Execution Fact      `json:"execution"`
	Admission Fact      `json:"admission"`
	Members   []Member  `json:"members"`
	Base      string    `json:"base"`
}
type HandIn struct {
	Fact    `json:",inline"`
	Goal    string    `json:"goal"`
	SHA     string    `json:"sha"`
	Ordinal int       `json:"ordinal"`
	Reason  string    `json:"reason"`
	At      time.Time `json:"at"`
}
type Resolution struct {
	Reason      string    `json:"reason"`
	Commit      string    `json:"commit"`
	CheckResult string    `json:"check_result"`
	LaunchError string    `json:"launch_error"`
	Paths       []string  `json:"paths"`
	Units       []string  `json:"units"`
	Round       int       `json:"round"`
	LaunchAt    time.Time `json:"launch_at"`
	ReadPending bool      `json:"read_pending"`
	Fact        `json:",inline"`
	Goal        string `json:"goal"`
	Job         string `json:"job"`
}
type Proof struct {
	Fact        `json:",inline"`
	Attempt     string           `json:"attempt"`
	Scope       string           `json:"scope"`
	Trunk       bool             `json:"trunk"`
	Members     []Member         `json:"members"`
	LoadAtStart *hostload.Sample `json:"load_at_start"`
}
type Main struct {
	Reason         string `json:"reason"`
	Flakes         string `json:"flakes"`
	FlakePublished bool   `json:"flake_published"`
	Fact           `json:",inline"`
	Commit         string   `json:"commit"`
	OpenIncidents  []string `json:"open_incidents"`
}
type Question struct {
	ID      string `json:"id"`
	Premise string `json:"premise"`
	Since   Since  `json:"since"`
}
type Holds struct {
	Barren           int   `json:"barren"`
	SelectionPending bool  `json:"selection_pending"`
	Since            Since `json:"since"`
}
type EngineIncident struct {
	At    time.Time `json:"at"`
	Verb  string    `json:"verb"`
	Panic string    `json:"panic"`
}
type Engine struct {
	Incidents []EngineIncident `json:"incidents"`
	Stamp     string           `json:"stamp"`
}
type Record struct {
	Stop            *Stop        `json:"stop,omitempty"`
	RemedyCommand   string       `json:"remedy_command,omitempty"`
	Lane            Lane         `json:"lane"`
	Agent           Agent        `json:"agent"`
	Batch           Batch        `json:"batch"`
	HandIns         []HandIn     `json:"hand_ins"`
	Resolution      Resolution   `json:"resolution"`
	Proof           Proof        `json:"proof"`
	Main            Main         `json:"main"`
	Questions       []Question   `json:"questions"`
	Holds           Holds        `json:"holds"`
	Engine          Engine       `json:"engine"`
	Version         int          `json:"version"`
	LastObservation *Observation `json:"last_observation,omitempty"`
}
type Stop struct {
	Subject string   `json:"subject"`
	Kind    string   `json:"kind"`
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Tests   []string `json:"tests"`
}
type Event struct {
	RecordID string    `json:"record_id,omitempty"`
	At       time.Time `json:"at"`
	Kind     string    `json:"kind"`
	From     string    `json:"from"`
	To       string    `json:"to"`
	By       string    `json:"by"`
	Evidence string    `json:"evidence"`
	Rebuild  bool      `json:"rebuild"`
}
type Refused struct{ Kind, Actual, Expected string }

func (r *Refused) Error() string {
	return fmt.Sprintf("%s is %q, expected %q", r.Kind, r.Actual, r.Expected)
}

func Read(dir string) (Record, bool, error) {
	var record Record
	data, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return record, false, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &record)
	}
	return record, err == nil, err
}

// ReadEvents keeps complete lines after a torn append; blank lines carry no event.
func ReadEvents(dir string) (events []Event, skipped int, err error) {
	file, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, 0, nil
	}
	if err != nil {
		return nil, 0, err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	reader := bufio.NewReader(file)
	for {
		line, readErr := reader.ReadBytes('\n')
		if len(strings.TrimSpace(string(line))) > 0 {
			var event Event
			if json.Unmarshal(line, &event) != nil {
				skipped++
			} else {
				events = append(events, event)
			}
		}
		if errors.Is(readErr, io.EOF) {
			return events, skipped, nil
		}
		if readErr != nil {
			return events, skipped, readErr
		}
	}
}

func SkippedEvidence(count int) string {
	if count == 1 {
		return "events.jsonl: 1 undecodable line skipped"
	}
	return fmt.Sprintf("events.jsonl: %d undecodable lines skipped", count)
}

// Diff includes payloads and clocks, even when the state words agree.
func Diff(a, b Record) (changed []string) {
	var walk func(string, reflect.Value, reflect.Value)
	walk = func(path string, x, y reflect.Value) {
		if reflect.DeepEqual(x.Interface(), y.Interface()) {
			return
		}
		if (path == "hand_ins" || path == "questions") && x.Len() == y.Len() {
			for i := 0; i < x.Len(); i++ {
				walk(fmt.Sprintf("%s[%d]", path, i), x.Index(i), y.Index(i))
			}
			return
		}
		if x.Kind() == reflect.Struct && x.Type() != reflect.TypeOf(time.Time{}) {
			for i := 0; i < x.NumField(); i++ {
				field := x.Type().Field(i)
				if path == "" && (field.Name == "Version" || field.Name == "LastObservation") {
					continue
				}
				name := strings.Split(field.Tag.Get("json"), ",")[0]
				if field.Anonymous {
					walk(path, x.Field(i), y.Field(i))
					continue
				}
				if path != "" {
					name = path + "." + name
				}
				walk(name, x.Field(i), y.Field(i))
			}
			return
		}
		changed = append(changed, fmt.Sprintf("%s: %s → %s", path, diffValue(x), diffValue(y)))
	}
	walk("", reflect.ValueOf(a), reflect.ValueOf(b))
	return changed
}

func diffValue(value reflect.Value) string {
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "<nil>"
		}
		return diffValue(value.Elem())
	}
	return fmt.Sprint(value.Interface())
}

func locked(dir string, change func() error) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(dir, "state.lock"), 0600, lock.Exclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	return change()
}
func appendEvent(dir string, event Event) error {
	file, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	// Separate a torn final line from the complete event about to be appended.
	info, statErr := file.Stat()
	if statErr == nil && info.Size() > 0 {
		var last [1]byte
		_, statErr = file.ReadAt(last[:], info.Size()-1)
		if statErr == nil && last[0] != '\n' {
			_, statErr = file.WriteString("\n")
		}
	}
	if statErr != nil {
		return errors.Join(statErr, file.Close())
	}
	return errors.Join(json.NewEncoder(file).Encode(event), file.Close())
}
func write(dir string, record Record) error {
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(filepath.Join(dir, "state.json"), append(data, '\n'), 0600, "")
	return err
}
func Transition(dir string, now time.Time, kind, from, to, by, evidence string, apply func(*Record), raw ...func() error) (event Event, err error) {
	event = Event{At: now.UTC(), Kind: kind, From: from, To: to, By: by, Evidence: evidence}
	err = locked(dir, func() error {
		record, _, err := Read(dir)
		if err != nil {
			return err
		}
		actual := "present"
		recordKind, _, retirement := strings.Cut(kind, ":")
		if retirement {
			event.Kind = recordKind
			event.RecordID = kind
		}
		if retirement {
			events, _, readErr := ReadEvents(dir)
			if readErr != nil {
				return readErr
			}
			for _, previous := range events {
				if previous.RecordID == kind && previous.Kind == recordKind {
					actual = previous.To
				}
			}
		} else {
			field := reflect.ValueOf(record)
			for _, part := range strings.Split(kind, ".") {
				if field.Kind() != reflect.Struct {
					return fmt.Errorf("unknown state field %q", kind)
				}
				field = field.FieldByNameFunc(func(name string) bool { return strings.EqualFold(name, part) })
				if !field.IsValid() {
					return fmt.Errorf("unknown state field %q", kind)
				}
			}
			if field.Kind() != reflect.Struct {
				return fmt.Errorf("state field %q has no word", kind)
			}
			word := field.FieldByName("Word")
			if !word.IsValid() {
				return fmt.Errorf("state field %q has no word", kind)
			}
			actual = word.String()
		}
		if actual != from {
			event.Kind, event.From = "defect", actual
			return errors.Join(&Refused{Kind: kind, Actual: actual, Expected: from}, appendEvent(dir, event))
		}
		version := record.Version
		for _, change := range raw {
			if err := change(); err != nil {
				return err
			}
		}
		if apply != nil {
			apply(&record)
		}
		record.Version = version + 1
		if err := appendEvent(dir, event); err != nil {
			return err
		}
		return write(dir, record)
	})
	return event, err
}
func Replace(dir string, now time.Time, next Record, by string, expectVersion int, expectFingerprint Fingerprint) (changed []string, err error) {
	err = locked(dir, func() error {
		previous, present, readErr := Read(dir)
		if readErr != nil {
			previous = Record{}
		}
		if previous.Version != expectVersion || expectFingerprint.Current == nil || expectFingerprint.Value != expectFingerprint.Current() {
			return &Stale{}
		}
		changed = Diff(previous, next)
		events, skipped, eventsErr := ReadEvents(dir)
		if eventsErr != nil {
			return eventsErr
		}
		recovered := previous.LastObservation != nil && next.LastObservation == nil
		damageRecorded := skipped == 0
		for i := len(events) - 1; i >= 0 && !damageRecorded; i-- {
			if events[i].Rebuild {
				damageRecorded = strings.Contains(events[i].Evidence, SkippedEvidence(skipped))
				break
			}
		}
		if len(changed) == 0 && readErr == nil && !recovered && damageRecorded {
			return nil
		}
		if readErr != nil {
			changed = append([]string{"previous record unreadable: " + readErr.Error()}, changed...)
		}
		if len(changed) > 0 || !damageRecorded {
			if present {
				if err := appendDrift(dir, now, changed, by, events); err != nil {
					return err
				}
			}
			evidence := append([]string(nil), changed...)
			if skipped > 0 {
				evidence = append(evidence, SkippedEvidence(skipped))
			}
			if err := appendEvent(dir, Event{At: now.UTC(), Kind: "rebuild", By: by, Evidence: strings.Join(evidence, "\n"), Rebuild: true}); err != nil {
				return err
			}
		}
		if recovered {
			if err := appendEvent(dir, Event{At: now.UTC(), Kind: "observation-recovered", By: by}); err != nil {
				return err
			}
		}
		next.Version = previous.Version + 1
		return write(dir, next)
	})
	return changed, err
}

// appendDrift accounts for transitions since the preceding rebuild under state.lock.
func appendDrift(dir string, now time.Time, changed []string, by string, events []Event) error {
	var transitions []string
	for _, event := range events {
		if event.Rebuild {
			transitions = nil
		} else if event.Kind != "defect" && event.Kind != "drift" && !strings.HasPrefix(event.Kind, "observation-") {
			transitions = append(transitions, event.Kind)
		}
	}
	var drift []string
	for _, difference := range changed {
		field, _, _ := strings.Cut(difference, ": ")
		explained := false
		for _, kind := range transitions {
			if field == kind || strings.HasPrefix(field, kind+".") {
				explained = true
				break
			}
		}
		if !explained {
			drift = append(drift, difference)
		}
	}
	if len(drift) == 0 {
		return nil
	}
	return appendEvent(dir, Event{At: now.UTC(), Kind: "drift", By: by, Evidence: strings.Join(drift, "\n")})
}

// Fingerprint validates publication while the state lock is held. The reader
// supplies the fingerprint so storage never depends on the lane's readers.
type Fingerprint struct {
	Value   string
	Current func() string
}
type Stale struct{}

func (*Stale) Error() string { return "the lane changed before the shadow could be published" }

func NoteObservation(dir string, now time.Time, problems []string) error {
	return locked(dir, func() error {
		record, _, err := Read(dir)
		if err != nil {
			return err
		}
		record.LastObservation = &Observation{At: now.UTC(), Problems: append([]string(nil), problems...)}
		record.Version++
		if err := appendEvent(dir, Event{At: now.UTC(), Kind: "observation-failed", Evidence: strings.Join(problems, "; ")}); err != nil {
			return err
		}
		return write(dir, record)
	})
}
func NoteUnstable(dir string, now time.Time) error {
	return locked(dir, func() error { return appendEvent(dir, Event{At: now.UTC(), Kind: "observation-unstable"}) })
}

// Carry retains the first observation of an unchanged identity and word.
func Carry(previous *Record, next Record) Record {
	if previous == nil {
		return next
	}
	p := previous
	keep := func(old Since, candidate *Since, same bool) {
		if same && candidate.Observed && !old.At.IsZero() {
			*candidate = old
		}
	}
	keep(p.Lane.Since, &next.Lane.Since, p.Lane.Word == next.Lane.Word)
	keep(p.Agent.Since, &next.Agent.Since, p.Agent.Session == next.Agent.Session && p.Agent.Word == next.Agent.Word && reflect.DeepEqual(p.Agent.Step, next.Agent.Step))
	keep(p.Batch.Execution.Since, &next.Batch.Execution.Since, p.Batch.ID == next.Batch.ID && p.Batch.Execution.Word == next.Batch.Execution.Word)
	keep(p.Batch.Admission.Since, &next.Batch.Admission.Since, p.Batch.ID == next.Batch.ID && p.Batch.Admission.Word == next.Batch.Admission.Word)
	next.HandIns = append([]HandIn(nil), next.HandIns...)
	for i := range next.HandIns {
		for _, old := range p.HandIns {
			n := &next.HandIns[i]
			keep(old.Since, &n.Since, old.Goal == n.Goal && old.SHA == n.SHA && old.Ordinal == n.Ordinal && old.Word == n.Word)
		}
	}
	keep(p.Resolution.Since, &next.Resolution.Since, p.Resolution.Goal == next.Resolution.Goal && p.Resolution.Job == next.Resolution.Job && p.Resolution.Word == next.Resolution.Word)
	keep(p.Proof.Since, &next.Proof.Since, p.Proof.Attempt == next.Proof.Attempt && p.Proof.Word == next.Proof.Word)
	keep(p.Main.Since, &next.Main.Since, p.Main.Commit == next.Main.Commit && p.Main.Word == next.Main.Word)
	next.Questions = append([]Question(nil), next.Questions...)
	for i := range next.Questions {
		for _, old := range p.Questions {
			n := &next.Questions[i]
			keep(old.Since, &n.Since, old.ID == n.ID && old.Premise == n.Premise)
		}
	}
	keep(p.Holds.Since, &next.Holds.Since, p.Holds.Barren == next.Holds.Barren && p.Holds.SelectionPending == next.Holds.SelectionPending)
	return next
}
