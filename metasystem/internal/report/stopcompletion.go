package report

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopreport"
)

const StopCompletionObservationSchemaVersion = 1

// StopCompletionIdentity binds a read-only record observation to the one
// seat judgment that captured it. Runtime delivery identity is added later by
// the presentation input, outside the goal judgment.
type StopCompletionIdentity struct {
	Installation string `json:"installation"`
	Session      string `json:"session"`
	MainId       string `json:"mainId"`
}

// StopCompletionRecord is the presentation-only slice of a job or run record.
// It is deliberately separate from the judgment scan's live-record facts.
type StopCompletionRecord struct {
	Kind         string `json:"kind"`
	Id           string `json:"id"`
	GoalId       string `json:"goalId"`
	MainId       string `json:"mainId"`
	Machine      string `json:"machine"`
	Lineage      string `json:"lineage"`
	ClaimEpoch   int64  `json:"claimEpoch"`
	Status       string `json:"status"`
	OperationId  string `json:"operationId"`
	StartedAt    string `json:"startedAt"`
	EndedAt      string `json:"endedAt"`
	Generation   int    `json:"generation"`
	Nonce        string `json:"nonce"`
	TerminalSeq  int64  `json:"terminalSeq"`
	Title        string `json:"title"`
	Role         string `json:"role"`
	SourcePath   string `json:"sourcePath"`
	SourceDigest string `json:"sourceDigest"`
	Ownership    string `json:"ownership"`
}

// StopCompletionCapture is filled during the same record enumeration as the
// judgment scan and bound to the judgment identity afterward.
type StopCompletionCapture struct {
	Records     []StopCompletionRecord
	Unavailable []string
}

// Observation freezes every compared record, including non-success states.
type StopCompletionObservation struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Identity      StopCompletionIdentity `json:"identity"`
	CollectedAt   string                 `json:"collectedAt"`
	Records       []StopCompletionRecord `json:"records"`
	Unavailable   []string               `json:"unavailable"`
}

// StopCompletionEvent is one newly observed successful terminal transition.
type StopCompletionEvent struct {
	Kind         string `json:"kind"`
	Id           string `json:"id"`
	GoalId       string `json:"goalId"`
	MainId       string `json:"mainId"`
	Status       string `json:"status"`
	OperationId  string `json:"operationId"`
	StartedAt    string `json:"startedAt"`
	EndedAt      string `json:"endedAt"`
	Generation   int    `json:"generation"`
	Nonce        string `json:"nonce"`
	TerminalSeq  int64  `json:"terminalSeq"`
	Title        string `json:"title"`
	Role         string `json:"role"`
	SourcePath   string `json:"sourcePath"`
	SourceDigest string `json:"sourceDigest"`
}

// StopCompletion records the honest comparison used for line one.
type StopCompletion struct {
	State            string                `json:"state"`
	BaselineReportId string                `json:"baselineReportId"`
	IntervalStart    string                `json:"intervalStart"`
	ObservedAt       string                `json:"observedAt"`
	Selected         *StopCompletionEvent  `json:"selected"`
	Events           []StopCompletionEvent `json:"events"`
	Unavailable      []string              `json:"unavailable"`
}

// BindStopCompletion gives the record capture its frozen judgment identity.
func BindStopCompletion(capture StopCompletionCapture, installation, session, mainID string, collectedAt time.Time) StopCompletionObservation {
	records := append([]StopCompletionRecord(nil), capture.Records...)
	unavailable := append([]string(nil), capture.Unavailable...)
	for index := range records {
		switch {
		case records[index].MainId == "" || mainID == "":
			records[index].Ownership = "unknown"
		case records[index].MainId == mainID:
			records[index].Ownership = "owned"
		default:
			records[index].Ownership = "other"
		}
	}
	if records == nil {
		records = []StopCompletionRecord{}
	}
	if unavailable == nil {
		unavailable = []string{}
	}
	return StopCompletionObservation{
		SchemaVersion: StopCompletionObservationSchemaVersion,
		Identity:      StopCompletionIdentity{Installation: installation, Session: session, MainId: mainID},
		CollectedAt:   collectedAt.UTC().Format(time.RFC3339Nano),
		Records:       records,
		Unavailable:   unavailable,
	}
}

func deriveStopCompletion(input StopPresentationInput, reportDir string) StopCompletion {
	observedAt := input.Identity.ObservedAt
	completion := StopCompletion{State: "unknown", ObservedAt: observedAt, Events: []StopCompletionEvent{}, Unavailable: []string{}}
	current := input.CompletionObservation
	if current == nil {
		completion.Unavailable = append(completion.Unavailable, "completion observation was unavailable")
		return completion
	}
	completion.ObservedAt = current.CollectedAt
	if reasons := completionObservationProblems(*current, input); len(reasons) > 0 {
		completion.Unavailable = append(completion.Unavailable, reasons...)
		return completion
	}
	baseline, baselineID, state := newestCompletionBaseline(input, reportDir)
	if state != "ok" {
		completion.Unavailable = append(completion.Unavailable, state)
		return completion
	}
	completion.BaselineReportId = baselineID
	completion.IntervalStart = baseline.CollectedAt
	if reasons := completionObservationProblems(baseline, input); len(reasons) > 0 {
		completion.Unavailable = append(completion.Unavailable, "previous completion observation is invalid: "+strings.Join(reasons, "; "))
		return completion
	}
	baselineTime, _ := time.Parse(time.RFC3339Nano, baseline.CollectedAt)
	currentTime, _ := time.Parse(time.RFC3339Nano, current.CollectedAt)
	if baselineTime.After(currentTime) {
		completion.Unavailable = append(completion.Unavailable, "previous completion observation is in the future")
		return completion
	}
	previous := map[string]StopCompletionRecord{}
	for _, record := range baseline.Records {
		if key, ok := completionRecordIdentity(record); ok {
			previous[key] = record
		}
	}
	for _, record := range current.Records {
		if record.Ownership != "owned" {
			continue
		}
		key, ok := completionRecordIdentity(record)
		if !ok {
			completion.Unavailable = append(completion.Unavailable, "owned "+record.Kind+" record "+record.Id+" has incomplete terminal identity")
			continue
		}
		prior, existed := previous[key]
		switch record.Kind {
		case "job":
			if record.Status != "completed" {
				continue
			}
			started, startErr := time.Parse(time.RFC3339Nano, record.StartedAt)
			ended, endErr := time.Parse(time.RFC3339Nano, record.EndedAt)
			if startErr != nil || endErr != nil || ended.Before(started) {
				completion.Unavailable = append(completion.Unavailable, "owned completed job "+record.Id+" has invalid timestamps")
				continue
			}
			if existed && prior.Status == "completed" {
				continue
			}
			if !existed && ended.Before(baselineTime) {
				continue
			}
			completion.Events = append(completion.Events, completionEvent(record))
		case "run":
			if record.Status != "green" || (existed && prior.Status == "green") {
				continue
			}
			completion.Events = append(completion.Events, completionEvent(record))
		}
	}
	if len(completion.Unavailable) > 0 {
		return completion
	}
	completion.State = "none"
	jobs := make([]StopCompletionEvent, 0, len(completion.Events))
	runs := make([]StopCompletionEvent, 0, len(completion.Events))
	for _, event := range completion.Events {
		if event.Kind == "job" {
			jobs = append(jobs, event)
		} else {
			runs = append(runs, event)
		}
	}
	if len(jobs) > 0 {
		sort.Slice(jobs, func(i, j int) bool {
			left, _ := time.Parse(time.RFC3339Nano, jobs[i].EndedAt)
			right, _ := time.Parse(time.RFC3339Nano, jobs[j].EndedAt)
			if left.Equal(right) {
				return jobs[i].Id < jobs[j].Id
			}
			return left.After(right)
		})
		selected := jobs[0]
		completion.Selected = &selected
		completion.State = "observed"
	} else if len(runs) > 0 {
		sort.Slice(runs, func(i, j int) bool {
			if runs[i].TerminalSeq == runs[j].TerminalSeq {
				return runs[i].Id < runs[j].Id
			}
			return runs[i].TerminalSeq > runs[j].TerminalSeq
		})
		selected := runs[0]
		completion.Selected = &selected
		completion.State = "observed"
	}
	return completion
}

func completionObservationProblems(observation StopCompletionObservation, input StopPresentationInput) []string {
	var problems []string
	if observation.SchemaVersion != StopCompletionObservationSchemaVersion || observation.Records == nil || observation.Unavailable == nil {
		problems = append(problems, "completion observation schema or arrays are invalid")
	}
	if observation.Identity.Installation != input.Identity.Installation || observation.Identity.Session != input.Identity.Session || observation.Identity.MainId != input.Identity.MainId {
		problems = append(problems, "completion observation identity does not match the Stop identity")
	}
	if input.Identity.MainId == "" || input.Identity.Machine == "" || input.Identity.Lineage == "" || input.Identity.ClaimEpoch <= 0 {
		problems = append(problems, "seat ownership identity is incomplete")
	}
	collected, collectedErr := time.Parse(time.RFC3339Nano, observation.CollectedAt)
	if collectedErr != nil || collected.Location() != time.UTC {
		problems = append(problems, "completion observation time is invalid")
	} else if stopObserved, err := time.Parse(time.RFC3339Nano, input.Identity.ObservedAt); err == nil && collected.After(stopObserved) {
		problems = append(problems, "completion observation is in the future")
	}
	problems = append(problems, observation.Unavailable...)
	for _, record := range observation.Records {
		if record.Kind != "job" && record.Kind != "run" {
			problems = append(problems, "completion observation contains an unknown record kind")
			continue
		}
		if problem := completionRecordProblem(record); problem != "" {
			problems = append(problems, problem)
		}
		if record.MainId == "" || (record.Ownership != "owned" && record.Ownership != "other") {
			problems = append(problems, "completion observation contains a record with unknown ownership")
		}
		if record.Ownership == "other" && record.MainId == input.Identity.MainId {
			problems = append(problems, "completion observation contains a record with inconsistent ownership")
		}
		if record.Ownership == "owned" {
			if record.MainId != input.Identity.MainId || (record.Machine != "" && record.Machine != input.Identity.Machine) ||
				(record.Lineage != "" && record.Lineage != input.Identity.Lineage) || (record.ClaimEpoch > 0 && record.ClaimEpoch != input.Identity.ClaimEpoch) {
				problems = append(problems, "owned completion record conflicts with the seat holder identity")
			}
		}
	}
	return problems
}

func completionRecordProblem(record StopCompletionRecord) string {
	if record.Id == "" || record.SourcePath == "" || len(record.SourceDigest) != 64 {
		return "completion observation contains a record with incomplete source identity"
	}
	if !lowerHex(record.SourceDigest) {
		return "completion observation contains a record with invalid source digest"
	}
	if record.Kind == "job" {
		switch record.Status {
		case "pending-setup", "pending", "observing", "running", "completed", "failed", "cancelled", "timeout":
		default:
			return "completion observation contains a job with an invalid status"
		}
		if record.OperationId == "" && record.StartedAt == "" {
			return "completion observation contains a job with incomplete reservation identity"
		}
		return ""
	}
	if record.Generation <= 0 || len(record.Nonce) != 32 || !lowerHex(record.Nonce) {
		return "completion observation contains a run with incomplete launch identity"
	}
	switch record.Status {
	case "launching", "running", "draining":
		if record.TerminalSeq != 0 {
			return "completion observation contains a nonterminal run with a terminal sequence"
		}
	case "green", "red", "ended-unknown", "launch-failed":
		if record.TerminalSeq <= 0 {
			return "completion observation contains a terminal run without a terminal sequence"
		}
	default:
		return "completion observation contains a run with an invalid status"
	}
	return ""
}

func lowerHex(value string) bool {
	for _, char := range value {
		if !((char >= '0' && char <= '9') || (char >= 'a' && char <= 'f')) {
			return false
		}
	}
	return value != ""
}

func newestCompletionBaseline(input StopPresentationInput, reportDir string) (StopCompletionObservation, string, string) {
	paths, err := filepath.Glob(filepath.Join(reportDir, input.Identity.SessionKey+"-*.md"))
	if err != nil || len(paths) == 0 {
		return StopCompletionObservation{}, "", "no usable previous completion observation"
	}
	type candidate struct {
		id         string
		observed   time.Time
		data       []byte
		compatible bool
	}
	var candidates []candidate
	var unreadable bool
	var invalidTime bool
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".md")
		data, identity, _, readErr := stopreport.Read(input.Identity.Installation, id)
		if readErr != nil {
			// A file in this session-key namespace cannot be ordered or
			// attributed safely when its immutable identity is unreadable.
			// Do not silently step over it to an older observation.
			unreadable = true
			continue
		}
		if identity.Installation != input.Identity.Installation || identity.Runtime != input.Identity.Runtime || identity.Session != input.Identity.Session ||
			identity.SessionKey != input.Identity.SessionKey || identity.MainId != input.Identity.MainId {
			continue
		}
		observed, parseErr := time.Parse(time.RFC3339Nano, identity.ObservedAt)
		if parseErr != nil {
			invalidTime = true
			continue
		}
		compatible := identity.Machine == input.Identity.Machine && identity.Lineage == input.Identity.Lineage && identity.ClaimEpoch == input.Identity.ClaimEpoch
		candidates = append(candidates, candidate{id: id, observed: observed, data: data, compatible: compatible})
	}
	if unreadable {
		return StopCompletionObservation{}, "", "a previous report in this session is unreadable"
	}
	if invalidTime {
		return StopCompletionObservation{}, "", "a previous report in this session has an invalid observation time"
	}
	if len(candidates) == 0 {
		return StopCompletionObservation{}, "", "no usable previous completion observation"
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].observed.Equal(candidates[j].observed) {
			return candidates[i].id > candidates[j].id
		}
		return candidates[i].observed.After(candidates[j].observed)
	})
	currentObserved, _ := time.Parse(time.RFC3339Nano, input.Identity.ObservedAt)
	if candidates[0].observed.After(currentObserved) {
		return StopCompletionObservation{}, candidates[0].id, "newest previous report is in the future"
	}
	if !candidates[0].compatible {
		return StopCompletionObservation{}, candidates[0].id, "newest previous report conflicts with the current seat holder identity"
	}
	var observation StopCompletionObservation
	if err := decodeStopReportSection(candidates[0].data, "Completion observation", &observation); err != nil {
		return StopCompletionObservation{}, candidates[0].id, "newest previous report has no usable completion observation"
	}
	return observation, candidates[0].id, "ok"
}

func decodeStopReportSection(data []byte, name string, destination any) error {
	startMarker := []byte("## " + name + "\n\n```json\n")
	start := bytes.Index(data, startMarker)
	if start < 0 {
		return fmt.Errorf("section %s is missing", name)
	}
	start += len(startMarker)
	end := bytes.Index(data[start:], []byte("\n```\n"))
	if end < 0 {
		return fmt.Errorf("section %s is incomplete", name)
	}
	decoder := json.NewDecoder(bytes.NewReader(data[start : start+end]))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return fmt.Errorf("section %s has trailing JSON", name)
	}
	return nil
}

func completionRecordIdentity(record StopCompletionRecord) (string, bool) {
	switch record.Kind {
	case "job":
		identity := record.OperationId
		if identity == "" && record.Id != "" && record.StartedAt != "" {
			identity = record.Id + "@" + record.StartedAt
		}
		return "job:" + record.MainId + ":" + identity + ":" + record.EndedAt, identity != "" && record.MainId != ""
	case "run":
		if record.Id == "" || record.MainId == "" || record.Generation <= 0 || record.Nonce == "" || record.TerminalSeq <= 0 {
			return "", false
		}
		return fmt.Sprintf("run:%s:%s:%d:%s:%d", record.MainId, record.Id, record.Generation, record.Nonce, record.TerminalSeq), true
	default:
		return "", false
	}
}

func completionEvent(record StopCompletionRecord) StopCompletionEvent {
	return StopCompletionEvent{
		Kind: record.Kind, Id: record.Id, GoalId: record.GoalId, MainId: record.MainId,
		Status: record.Status, OperationId: record.OperationId, StartedAt: record.StartedAt, EndedAt: record.EndedAt,
		Generation: record.Generation, Nonce: record.Nonce, TerminalSeq: record.TerminalSeq,
		Title: record.Title, Role: record.Role, SourcePath: record.SourcePath, SourceDigest: record.SourceDigest,
	}
}

func completionLine(completion StopCompletion) string {
	text := "unknown for this turn"
	switch completion.State {
	case "none":
		text = "none recorded this turn"
	case "observed":
		name := "task name unavailable"
		if completion.Selected != nil {
			if candidate, ok := goalIDTaskName(completion.Selected.GoalId); ok {
				name = candidate
			} else if completion.Selected.Kind == "job" {
				if candidate, ok := shortTaskLabel(completion.Selected.Role, true); ok {
					name = candidate
				}
			} else if candidate, ok := shortTaskLabel(completion.Selected.Title, false); ok {
				name = candidate
			}
			if completion.Selected.Kind == "job" {
				text = name + " (delegate returned)"
			} else {
				text = name + " (run passed)"
			}
		}
	}
	return "Just completed: " + text + "."
}

func completionObservationFile(path string) (StopCompletionObservation, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return StopCompletionObservation{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var observation StopCompletionObservation
	if err := decoder.Decode(&observation); err != nil {
		return StopCompletionObservation{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return StopCompletionObservation{}, fmt.Errorf("trailing JSON")
	}
	return observation, nil
}
