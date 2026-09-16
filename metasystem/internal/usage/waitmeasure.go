package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const waitLimitNanos = int64(60 * time.Second)

// WaitMeasureOptions bounds a read-only projection of durable wait records.
type WaitMeasureOptions struct {
	Since   time.Time
	Until   time.Time
	Runtime string
}

// WaitMeasurement is the records-only lower-bound report for clause 5.
type WaitMeasurement struct {
	SchemaVersion int                      `json:"schemaVersion"`
	Verdict       string                   `json:"verdict"`
	Coverage      WaitMeasurementCoverage  `json:"coverage"`
	Runtimes      []WaitRuntimeMeasurement `json:"runtimes"`
}

type WaitMeasurementCoverage struct {
	RegistrationEvents int `json:"registrationEvents"`
	RowRegistrations   int `json:"rowRegistrations"`
}

type WaitRuntimeMeasurement struct {
	Runtime        string               `json:"runtime"`
	Counts         map[string]int       `json:"counts"`
	Defects        map[string]int       `json:"defects"`
	MaxRUpperNanos int64                `json:"maxRUpperNanos"`
	Samples        []WaitMeasureSample  `json:"samples"`
	Unavailable    []WaitMeasureProblem `json:"unavailable,omitempty"`
}

type WaitMeasureProblem struct {
	WaitID string `json:"waitId"`
	Nonce  string `json:"nonce"`
	Reason string `json:"reason"`
}

type WaitMeasureSample struct {
	WaitID             string `json:"waitId"`
	Nonce              string `json:"nonce"`
	Kind               string `json:"kind"`
	TargetID           string `json:"targetId"`
	Class              string `json:"class"`
	Verdict            string `json:"verdict,omitempty"`
	StampsFrom         string `json:"stampsFrom"`
	PublishedLower     int64  `json:"publishedLower,omitempty"`
	PublishedUpper     int64  `json:"publishedUpper,omitempty"`
	Returned           int64  `json:"returned"`
	RLowerNanos        int64  `json:"rLowerNanos,omitempty"`
	RUpperNanos        int64  `json:"rUpperNanos,omitempty"`
	UncertaintyNanos   int64  `json:"uncertaintyNanos,omitempty"`
	LowerEdge          string `json:"lowerEdge,omitempty"`
	UpperEdge          string `json:"upperEdge,omitempty"`
	LooseEdgeReason    string `json:"looseEdgeReason,omitempty"`
	UnavailableReason  string `json:"unavailableReason,omitempty"`
	RegisteredLagNanos int64  `json:"registeredLagNanos,omitempty"`
}

type waitMeasureReturn struct {
	WaitID, Nonce, Kind, TargetID, Runtime, Mode, State, SourceOutcome, SourceEvidence, LedgerTip string
	RegisteredAt, PrevObservedAt, ObservedAt, ReturnedAt                                          string
	RegisteredBootNanos, PrevObservedBootNanos, ObservedBootNanos, ReturnedBootNanos              int64
	RegisteredBootID, PrevObservedBootID, ObservedBootID, ReturnedBootID                          string
	AtEntry                                                                                       bool
	StampsFrom                                                                                    string
}

type waitMeasureHint struct {
	Kind, TargetID, PublicationID, BeganBootID, PublishedBootID string
	BeganBootNanos, PublishedBootNanos                          int64
}

type waitMeasureRow struct {
	WaitID, Nonce, Kind, TargetID, Runtime, Mode, State, RegisteredAt string
	RegisteredBootNanos                                               int64
	RegisteredBootID                                                  string
	Result                                                            *waitMeasureWireResult
	Renewals                                                          []struct{ Result waitMeasureWireResult }
}

type waitMeasureWireResult struct {
	WaitID, Nonce, Runtime, Mode, State, SourceOutcome, SourceEvidence, LedgerTip    string
	RegisteredAt, PrevObservedAt, ObservedAt, ReturnedAt                             string
	RegisteredBootNanos, PrevObservedBootNanos, ObservedBootNanos, ReturnedBootNanos int64
	RegisteredBootID, PrevObservedBootID, ObservedBootID, ReturnedBootID             string
	AtEntry                                                                          bool
	Selector                                                                         struct{ Kind, TargetID string }
}

func fieldString(object map[string]any, name string) string {
	value, _ := object[name].(string)
	return value
}
func fieldInt(object map[string]any, name string) (int64, error) {
	value, present := object[name]
	if !present || value == nil {
		return 0, nil
	}
	if wire, ok := value.(string); ok {
		parsed, err := strconv.ParseInt(wire, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("event field %s is not an integer: %q", name, wire)
		}
		return parsed, nil
	}
	parsed, ok := asInt(value)
	if !ok {
		return 0, fmt.Errorf("event field %s is not an integer", name)
	}
	return parsed, nil
}
func fieldBool(object map[string]any, name string) (bool, error) {
	value, present := object[name]
	if !present || value == nil {
		return false, nil
	}
	if wire, ok := value.(string); ok {
		parsed, err := strconv.ParseBool(wire)
		if err != nil {
			return false, fmt.Errorf("event field %s is not a boolean: %q", name, wire)
		}
		return parsed, nil
	}
	parsed, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("event field %s is not a boolean", name)
	}
	return parsed, nil
}
func waitMeasureKey(waitID, nonce string) string { return waitID + "\x00" + nonce }

func returnFromEvent(event map[string]any) (waitMeasureReturn, error) {
	// An absent field is represented by zero so the sample's comparability
	// checks can name the missing stamp. A present zero is a valid decoded
	// value with the same result. A malformed present value is a record defect
	// and must stop the projection instead of being mistaken for either case.
	registered, err := fieldInt(event, "registeredBootNanos")
	if err != nil {
		return waitMeasureReturn{}, err
	}
	previous, err := fieldInt(event, "prevObservedBootNanos")
	if err != nil {
		return waitMeasureReturn{}, err
	}
	observed, err := fieldInt(event, "observedBootNanos")
	if err != nil {
		return waitMeasureReturn{}, err
	}
	returned, err := fieldInt(event, "returnedBootNanos")
	if err != nil {
		return waitMeasureReturn{}, err
	}
	atEntry, err := fieldBool(event, "atEntry")
	if err != nil {
		return waitMeasureReturn{}, err
	}
	return waitMeasureReturn{
		WaitID: fieldString(event, "waitId"), Nonce: fieldString(event, "nonce"), Kind: fieldString(event, "kind"), TargetID: fieldString(event, "targetId"),
		Runtime: fieldString(event, "runtime"), Mode: fieldString(event, "mode"), State: fieldString(event, "state"), AtEntry: atEntry,
		SourceOutcome: fieldString(event, "sourceOutcome"), SourceEvidence: fieldString(event, "sourceEvidence"), LedgerTip: fieldString(event, "ledgerTip"),
		RegisteredAt: fieldString(event, "registeredAt"), PrevObservedAt: fieldString(event, "prevObservedAt"), ObservedAt: fieldString(event, "observedAt"), ReturnedAt: fieldString(event, "returnedAt"),
		RegisteredBootNanos: registered, PrevObservedBootNanos: previous, ObservedBootNanos: observed, ReturnedBootNanos: returned,
		RegisteredBootID: fieldString(event, "registeredBootId"), PrevObservedBootID: fieldString(event, "prevObservedBootId"), ObservedBootID: fieldString(event, "observedBootId"), ReturnedBootID: fieldString(event, "returnedBootId"), StampsFrom: "event",
	}, nil
}

func hintFromEvent(event map[string]any) (waitMeasureHint, error) {
	began, err := fieldInt(event, "beganBootNanos")
	if err != nil {
		return waitMeasureHint{}, err
	}
	published, err := fieldInt(event, "publishedBootNanos")
	if err != nil {
		return waitMeasureHint{}, err
	}
	return waitMeasureHint{Kind: fieldString(event, "kind"), TargetID: fieldString(event, "targetId"), PublicationID: fieldString(event, "publicationId"), BeganBootNanos: began, BeganBootID: fieldString(event, "beganBootId"), PublishedBootNanos: published, PublishedBootID: fieldString(event, "publishedBootId")}, nil
}

func returnFromWire(result waitMeasureWireResult) waitMeasureReturn {
	return waitMeasureReturn{WaitID: result.WaitID, Nonce: result.Nonce, Kind: result.Selector.Kind, TargetID: result.Selector.TargetID,
		Runtime: result.Runtime, Mode: result.Mode, State: result.State, AtEntry: result.AtEntry, SourceOutcome: result.SourceOutcome, SourceEvidence: result.SourceEvidence, LedgerTip: result.LedgerTip,
		RegisteredAt: result.RegisteredAt, PrevObservedAt: result.PrevObservedAt, ObservedAt: result.ObservedAt, ReturnedAt: result.ReturnedAt,
		RegisteredBootNanos: result.RegisteredBootNanos, PrevObservedBootNanos: result.PrevObservedBootNanos, ObservedBootNanos: result.ObservedBootNanos, ReturnedBootNanos: result.ReturnedBootNanos,
		RegisteredBootID: result.RegisteredBootID, PrevObservedBootID: result.PrevObservedBootID, ObservedBootID: result.ObservedBootID, ReturnedBootID: result.ReturnedBootID, StampsFrom: "row"}
}

func parseMeasureTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}
func insideMeasureWindow(value string, options WaitMeasureOptions) bool {
	stamp := parseMeasureTime(value)
	return !stamp.IsZero() && (options.Since.IsZero() || !stamp.Before(options.Since)) && (options.Until.IsZero() || !stamp.After(options.Until))
}

// MeasureWaits reads rows and the event stream without appending either.
func MeasureWaits(root string, options WaitMeasureOptions) (WaitMeasurement, error) {
	returns, registrations, currentRows := map[string]waitMeasureReturn{}, map[string]waitMeasureReturn{}, map[string]bool{}
	var hints []waitMeasureHint
	coverage := WaitMeasurementCoverage{}
	for _, event := range jsonlObjects(filepath.Join(root, "artifacts", "agents", "events.jsonl")) {
		if !options.Until.IsZero() && parseMeasureTime(fieldString(event, "ts")).After(options.Until) {
			continue
		}
		switch fieldString(event, "event") {
		case "wait-registered":
			item, parseErr := returnFromEvent(event)
			if parseErr != nil {
				return WaitMeasurement{}, parseErr
			}
			registrations[waitMeasureKey(item.WaitID, item.Nonce)] = item
			coverage.RegistrationEvents++
		case "wait-returned":
			item, parseErr := returnFromEvent(event)
			if parseErr != nil {
				return WaitMeasurement{}, parseErr
			}
			key := waitMeasureKey(item.WaitID, item.Nonce)
			if original, found := returns[key]; found && original.Mode != "replay" && item.Mode == "replay" {
				continue
			}
			returns[key], registrations[key] = item, item
		case "wait-published":
			hint, parseErr := hintFromEvent(event)
			if parseErr != nil {
				return WaitMeasurement{}, parseErr
			}
			hints = append(hints, hint)
		}
	}
	paths, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "waiters", "*.json"))
	if err != nil {
		return WaitMeasurement{}, err
	}
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			continue
		}
		var row waitMeasureRow
		if json.Unmarshal(data, &row) != nil || row.WaitID == "" || row.Nonce == "" {
			continue
		}
		base := waitMeasureReturn{WaitID: row.WaitID, Nonce: row.Nonce, Kind: row.Kind, TargetID: row.TargetID, Runtime: row.Runtime, Mode: row.Mode, State: row.State, RegisteredAt: row.RegisteredAt, RegisteredBootNanos: row.RegisteredBootNanos, RegisteredBootID: row.RegisteredBootID}
		key := waitMeasureKey(row.WaitID, row.Nonce)
		registrations[key], currentRows[key] = base, true
		coverage.RowRegistrations++
		if row.Result != nil {
			item := returnFromWire(*row.Result)
			if _, found := returns[key]; !found {
				returns[key] = item
			}
		}
		for _, renewal := range row.Renewals {
			item := returnFromWire(renewal.Result)
			oldKey := waitMeasureKey(item.WaitID, item.Nonce)
			if _, found := returns[oldKey]; !found {
				returns[oldKey] = item
			}
			registrations[oldKey] = item
		}
	}
	byRuntime := map[string]*WaitRuntimeMeasurement{}
	for key, registration := range registrations {
		if !insideMeasureWindow(registration.RegisteredAt, options) || (options.Runtime != "" && registration.Runtime != options.Runtime) {
			continue
		}
		report := byRuntime[registration.Runtime]
		if report == nil {
			report = &WaitRuntimeMeasurement{Runtime: registration.Runtime, Counts: map[string]int{}, Defects: map[string]int{}}
			byRuntime[registration.Runtime] = report
		}
		report.Counts["registrations"]++
		returned, found := returns[key]
		returnedAfterCut := found && !options.Until.IsZero() && parseMeasureTime(returned.ReturnedAt).After(options.Until)
		if !found || returnedAfterCut {
			if currentRows[key] && (returnedAfterCut || registration.State == "registering" || registration.State == "pending") {
				report.Counts["pending-at-cut"]++
			} else {
				report.Counts["unavailable"]++
				report.Unavailable = append(report.Unavailable, WaitMeasureProblem{WaitID: registration.WaitID, Nonce: registration.Nonce, Reason: "return-unrecorded"})
			}
			continue
		}
		report.Counts["returns"]++
		if returned.State != "ready" || returned.Mode == "replay" {
			report.Counts["not-sampled"]++
			continue
		}
		sample := evaluateWaitSample(returned, hints, report)
		report.Samples = append(report.Samples, sample)
		if sample.UnavailableReason != "" {
			report.Counts["unavailable"]++
			report.Unavailable = append(report.Unavailable, WaitMeasureProblem{WaitID: sample.WaitID, Nonce: sample.Nonce, Reason: sample.UnavailableReason})
		} else {
			report.Counts[sample.Verdict]++
			if sample.RUpperNanos > report.MaxRUpperNanos {
				report.MaxRUpperNanos = sample.RUpperNanos
			}
		}
	}
	measurement := WaitMeasurement{SchemaVersion: 1, Verdict: "unproven", Coverage: coverage}
	for _, report := range byRuntime {
		sort.Slice(report.Samples, func(i, j int) bool {
			return report.Samples[i].WaitID+report.Samples[i].Nonce < report.Samples[j].WaitID+report.Samples[j].Nonce
		})
		measurement.Runtimes = append(measurement.Runtimes, *report)
		if report.Counts["refuted"] > 0 {
			measurement.Verdict = "refuted"
		}
	}
	sort.Slice(measurement.Runtimes, func(i, j int) bool { return measurement.Runtimes[i].Runtime < measurement.Runtimes[j].Runtime })
	return measurement, nil
}

func publicationJoinKey(result waitMeasureReturn) (string, bool) {
	switch result.Kind {
	case "job":
		return result.SourceEvidence + ":" + result.SourceOutcome, result.SourceEvidence != "" && result.SourceOutcome != ""
	case "attempt":
		return result.SourceEvidence, result.SourceEvidence != ""
	case "goal", "landing":
		return result.LedgerTip, result.LedgerTip != ""
	}
	return "", true
}

func evaluateWaitSample(result waitMeasureReturn, hints []waitMeasureHint, report *WaitRuntimeMeasurement) WaitMeasureSample {
	class := "sample"
	if result.AtEntry {
		class = "at-entry"
		report.Counts["at-entry"]++
	}
	sample := WaitMeasureSample{WaitID: result.WaitID, Nonce: result.Nonce, Kind: result.Kind, TargetID: result.TargetID, Class: class, StampsFrom: result.StampsFrom, Returned: result.ReturnedBootNanos}
	joinKey, joinable := publicationJoinKey(result)
	var matches []waitMeasureHint
	if joinable && joinKey != "" {
		for _, hint := range hints {
			if hint.Kind == result.Kind && hint.TargetID == result.TargetID && hint.PublicationID == joinKey && hint.PublishedBootID == result.ReturnedBootID {
				matches = append(matches, hint)
			}
		}
	}
	nonZero := 0
	var selected waitMeasureHint
	for _, hint := range matches {
		if hint.BeganBootNanos != 0 {
			nonZero++
			selected = hint
		}
	}
	useHints := joinable && nonZero <= 1
	switch {
	case !joinable:
		sample.LooseEdgeReason = "join-key-missing"
		report.Defects["join-key-missing"]++
		useHints = false
	case nonZero > 1:
		sample.LooseEdgeReason = "publication-ambiguous"
		report.Defects["publication-ambiguous"]++
		useHints = false
	case len(matches) == 0:
		sample.LooseEdgeReason = "no-matching-publication"
	case nonZero == 0:
		sample.LooseEdgeReason = "lower-edge-prev-observation"
	}
	lower, upper := result.PrevObservedBootNanos, result.ObservedBootNanos
	sample.LowerEdge, sample.UpperEdge = "prev-observation", "observation"
	if useHints {
		for _, hint := range matches {
			if hint.PublishedBootNanos != 0 && hint.PublishedBootNanos < upper {
				upper, sample.UpperEdge = hint.PublishedBootNanos, "publication"
			}
		}
		if nonZero == 1 {
			lower, sample.LowerEdge = selected.BeganBootNanos, "publication-began"
		}
	}
	if result.AtEntry && sample.LowerEdge != "publication-began" {
		sample.UnavailableReason = "no-lower-edge"
		return sample
	}
	baseStamps := []int64{result.RegisteredBootNanos, result.PrevObservedBootNanos, result.ObservedBootNanos, result.ReturnedBootNanos}
	baseIDs := []string{result.RegisteredBootID, result.PrevObservedBootID, result.ObservedBootID, result.ReturnedBootID}
	for index, stamp := range baseStamps {
		if stamp == 0 || baseIDs[index] == "" || baseIDs[index] != result.ReturnedBootID {
			sample.UnavailableReason = "clock-not-comparable"
			return sample
		}
	}
	if sample.LowerEdge == "publication-began" && (selected.BeganBootID != result.ReturnedBootID || selected.PublishedBootID != result.ReturnedBootID) {
		sample.UnavailableReason = "clock-not-comparable"
		return sample
	}
	if !(result.RegisteredBootNanos <= result.PrevObservedBootNanos && result.PrevObservedBootNanos <= result.ObservedBootNanos && result.ObservedBootNanos <= result.ReturnedBootNanos) || lower > upper {
		sample.UnavailableReason = "stamp-order"
		return sample
	}
	sample.PublishedLower, sample.PublishedUpper = lower, upper
	sample.RUpperNanos = result.ReturnedBootNanos - lower
	sample.RLowerNanos = result.ReturnedBootNanos - upper
	if sample.RLowerNanos < 0 {
		sample.RLowerNanos = 0
	}
	sample.UncertaintyNanos = sample.RUpperNanos - sample.RLowerNanos
	if sample.LowerEdge == "publication-began" {
		sample.RegisteredLagNanos = result.RegisteredBootNanos - lower
	}
	switch {
	case sample.RLowerNanos >= waitLimitNanos:
		sample.Verdict = "refuted"
	case sample.RUpperNanos >= waitLimitNanos:
		sample.Verdict = "suspect"
	default:
		sample.Verdict = "returned-within-60"
	}
	return sample
}

// WaitMeasurementExit maps the report to the wait command's stable exits.
func WaitMeasurementExit(measurement WaitMeasurement) int {
	if measurement.Verdict == "refuted" {
		return 1
	}
	for _, runtime := range measurement.Runtimes {
		if runtime.Counts["suspect"] > 0 || runtime.Counts["unavailable"] > 0 {
			return 2
		}
	}
	return 0
}

// FormatWaitMeasurement renders the same deterministic fields as the JSON view.
func FormatWaitMeasurement(measurement WaitMeasurement) string {
	var lines []string
	lines = append(lines, fmt.Sprintf("wait measurement verdict=%s registration-events=%d row-registrations=%d", measurement.Verdict, measurement.Coverage.RegistrationEvents, measurement.Coverage.RowRegistrations))
	for _, runtime := range measurement.Runtimes {
		lines = append(lines, fmt.Sprintf("runtime=%s registrations=%d returns=%d refuted=%d suspect=%d returned-within-60=%d unavailable=%d pending-at-cut=%d max-r-upper-nanos=%d", runtime.Runtime, runtime.Counts["registrations"], runtime.Counts["returns"], runtime.Counts["refuted"], runtime.Counts["suspect"], runtime.Counts["returned-within-60"], runtime.Counts["unavailable"], runtime.Counts["pending-at-cut"], runtime.MaxRUpperNanos))
		for _, sample := range runtime.Samples {
			encoded, _ := json.Marshal(sample)
			lines = append(lines, "sample="+string(encoded))
		}
		for _, name := range []string{"publication-ambiguous", "join-key-missing"} {
			if runtime.Defects[name] > 0 {
				lines = append(lines, "defect runtime="+runtime.Runtime+" reason="+name+" count="+strconv.Itoa(runtime.Defects[name]))
			}
		}
	}
	return strings.Join(lines, "\n") + "\n"
}
