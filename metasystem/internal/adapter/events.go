package adapter

// The emulator event projection (acp-adapter-seam slice three): the
// CLI runtimes' native round artifacts projected into the seam's
// Event vocabulary, post hoc. Projection, never invention — kinds
// come from the fields the artifacts really carry, params are the
// line's raw bytes verbatim, and loss is visible (a counted
// lines-skipped event). The namespaces (<runtime>/<value>) keep
// post-hoc replay visibly distinct from the native driver's live
// update/ and driver/ streams.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// MaxEventArtifactBytes bounds one projected artifact (the ATIF
// transcript precedent). Over-ceiling is an ERROR, never a silent
// truncation — a suite must see that its evidence outgrew the law.
const MaxEventArtifactBytes = 8 << 20

// eventKindFields names, per runtime, the line fields consulted for
// the projected kind, in precedence order. A line carrying none of
// them projects as <runtime>/unlabeled.
var eventKindFields = map[string][]string{
	"codex":  {"type"},
	"claude": {"type"},
	"devin":  {"type"},
	"fake":   {"type", "event"},
}

// projectEventArtifact reads one JSONL artifact into seam events.
// Seq is the 1-based ordinal in the PROJECTED list (one law, stated
// once — the selected artifact is the order authority); malformed
// lines are counted and surfaced as the final
// <runtime>/lines-skipped event with params {"skipped":<count>}.
func projectEventArtifact(runtime, path string) ([]delegate.Event, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if info.Size() > MaxEventArtifactBytes {
		return nil, fmt.Errorf("%s event artifact %s is %d bytes; the projection ceiling is %d", runtime, filepath.Base(path), info.Size(), MaxEventArtifactBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	kindFields := eventKindFields[runtime]
	var events []delegate.Event
	skipped := 0
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// One more than the ceiling: the file-size gate above already
	// rejects over-ceiling artifacts, and a single line exactly AT
	// the ceiling must scan (bufio requires tokens strictly smaller
	// than the max; S3-C1-004).
	scanner.Buffer(make([]byte, 0, 64*1024), MaxEventArtifactBytes+1)
	for scanner.Scan() {
		line := scanner.Bytes()
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &object); err != nil || object == nil {
			// Unparseable lines, arrays, scalars, and a bare null
			// alike: not projectable, counted (a JSON null decodes
			// into a nil map without error; S3-C1-002).
			skipped++
			continue
		}
		kind := runtime + "/" + eventKindValue(runtime, object, kindFields)
		events = append(events, delegate.Event{
			Seq:  uint64(len(events) + 1),
			Kind: kind,
			// The line's raw bytes VERBATIM — padding included
			// (S3-C1-001); only the parse consulted the trim.
			Params: append([]byte(nil), line...),
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s event artifact %s: %v", runtime, filepath.Base(path), err)
	}
	if skipped > 0 {
		events = append(events, delegate.Event{
			Seq:    uint64(len(events) + 1),
			Kind:   runtime + "/lines-skipped",
			Params: []byte(fmt.Sprintf(`{"skipped":%d}`, skipped)),
		})
	}
	return events, nil
}

// eventKindValue resolves the projected kind value from the line's
// declared fields; claude's subtype refinement rides its type.
func eventKindValue(runtime string, object map[string]json.RawMessage, fields []string) string {
	value := ""
	for _, field := range fields {
		if s := rawString(object[field]); s != "" {
			value = s
			break
		}
	}
	if value == "" {
		return "unlabeled"
	}
	if runtime == "claude" {
		if sub := rawString(object["subtype"]); sub != "" {
			value = value + "." + sub
		}
	}
	return value
}

func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// RuntimeEvents is the Events port body for one runtime: artifact
// selection per the design — claude uses claude-stream.jsonl as the
// SOLE artifact when it exists (streamed rounds) and events.jsonl
// otherwise (legacy blocking rounds), never both; every other
// runtime projects the round's events.jsonl.
func RuntimeEvents(runtime string) func(roundDir string) ([]delegate.Event, error) {
	return func(roundDir string) ([]delegate.Event, error) {
		artifact := filepath.Join(roundDir, "events.jsonl")
		if runtime == "claude" {
			stream := filepath.Join(roundDir, "claude-stream.jsonl")
			if _, err := os.Stat(stream); err == nil {
				artifact = stream
			}
		}
		return projectEventArtifact(runtime, artifact)
	}
}
