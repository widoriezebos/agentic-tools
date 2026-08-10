// Package events is the flight-recorder emitter (plans/flight-recorder.md).
// The stream is a witness, not an authority, so emitting an event must NEVER
// fail the caller: every error — bad input, missing registry, unwritable
// stream, full disk — is swallowed. Framing is one append of "\n" + compact
// JSON with no trailing newline, capped hard at 4096 bytes, so a torn short
// write can never corrupt the next writer's line.
package events

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"
)

const capBytes = 4096

var caps = map[string]int{"component": 16, "event": 40, "id": 160, "level": 8, "payload": 256}

var idFields = []string{"missionId", "jobId", "turnId", "cohortId", "executionId"}

var requiredEnvelope = map[string]bool{
	"schemaVersion": true, "ts": true, "component": true, "event": true,
	"level": true, "summary": true, "pid": true, "pidStartedAt": true, "seq": true,
}

// Emitter carries the identity of one component's event stream. Its sequence
// counter is monotonic per process so events from one emitter can be ordered.
type Emitter struct {
	Component    string
	Pid          int64
	PidStartedAt int64
	seq          int64
}

// clock is the time source, overridable in tests.
var clock = time.Now

// Emit appends one event. It is best-effort and never panics: any failure is
// silently dropped, because the recorder must never take down its caller.
func (e *Emitter) Emit(root, event, summary string, fields map[string]string) {
	defer func() { _ = recover() }()
	if root == "" {
		root = os.Getenv("METASYSTEM_HARNESS_ROOT")
	}
	if root == "" {
		return
	}
	seq := atomic.AddInt64(&e.seq, 1)

	args := map[string]string{}
	for k, v := range fields {
		args[k] = v
	}
	if exec := os.Getenv("METASYSTEM_EXECUTION_ID"); exec != "" {
		if _, ok := args["executionId"]; !ok {
			args["executionId"] = exec
		}
	}

	if !registryAllows(root, e.Component, event) {
		return
	}

	now := clock().UTC()
	record := map[string]any{
		"schemaVersion": 1,
		"ts":            now.Format("2006-01-02T15:04:05.000") + "Z",
		"component":     clip(e.Component, caps["component"]),
		"event":         clip(event, caps["event"]),
		"level":         clip(stringOr(args, "level", "info"), caps["level"]),
		"pid":           e.Pid,
		"pidStartedAt":  e.PidStartedAt,
		"seq":           seq,
	}
	delete(args, "level")
	for _, name := range idFields {
		if v := args[name]; v != "" {
			record[name] = clip(v, caps["id"])
			delete(args, name)
		}
	}
	if v := args["ref"]; v != "" {
		record["ref"] = clip(v, caps["id"])
		delete(args, "ref")
	}
	for name, value := range args {
		record[name] = clip(value, caps["payload"])
	}
	record["summary"] = summary

	line := frame(record)
	if overshoot := len(line) - capBytes; overshoot > 0 {
		record["summary"] = clip(summary, maxInt(0, len(summary)-overshoot-8))
		line = frame(record)
	}
	if len(line) > capBytes {
		line = shrink(record)
	}
	if len(line) > capBytes {
		return // pathological: dropped whole, never written torn
	}

	path := filepath.Join(root, "artifacts", "agents", "events.jsonl")
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	fd, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	_, _ = fd.Write([]byte(line))
	_ = fd.Close()
}

// frame renders the record as the on-wire line: a leading newline then
// compact, key-sorted JSON (Go sorts map keys), no trailing newline.
func frame(record map[string]any) string {
	data, err := json.Marshal(record)
	if err != nil {
		return ""
	}
	return "\n" + string(data)
}

// shrink drops optional payload fields, largest first, until the line fits —
// never dropping the required envelope or id fields.
func shrink(record map[string]any) string {
	var optional []string
	for k := range record {
		if requiredEnvelope[k] || isIDField(k) {
			continue
		}
		optional = append(optional, k)
	}
	sort.Slice(optional, func(i, j int) bool {
		return len(toString(record[optional[i]])) > len(toString(record[optional[j]]))
	})
	line := frame(record)
	for _, k := range optional {
		if len(line) <= capBytes {
			break
		}
		delete(record, k)
		line = frame(record)
	}
	return line
}

func registryAllows(root, component, event string) bool {
	data, err := os.ReadFile(filepath.Join(root, "scripts", "agents", "event-registry.json"))
	if err != nil {
		return true // a broken/absent registry must not silence the witness
	}
	var registry struct {
		Events map[string]struct {
			Emitters []string `json:"emitters"`
		} `json:"events"`
	}
	if json.Unmarshal(data, &registry) != nil {
		return true
	}
	entry, ok := registry.Events[event]
	if !ok {
		return false // unregistered event: dropped
	}
	for _, e := range entry.Emitters {
		if e == component {
			return true
		}
	}
	return false // this component may not emit this event
}

// clip truncates value to cap bytes on a UTF-8 boundary, marking a cut with ~.
func clip(value string, cap int) string {
	if len(value) <= cap {
		return value
	}
	cut := []byte(value)[:maxInt(0, cap-1)]
	for len(cut) > 0 && cut[len(cut)-1]&0xC0 == 0x80 {
		cut = cut[:len(cut)-1]
	}
	return string(cut) + "~"
}

func stringOr(m map[string]string, key, fallback string) string {
	if v, ok := m[key]; ok && v != "" {
		return v
	}
	return fallback
}

func isIDField(k string) bool {
	for _, f := range idFields {
		if f == k {
			return true
		}
	}
	return false
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	data, _ := json.Marshal(v)
	return string(data)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
