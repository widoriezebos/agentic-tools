// Package atif owns bounded access to exported ATIF transcripts:
// one ceiling, one snapshot per attempt,
// and step iteration — a LEAF, imported by adapter and usage, importing
// neither. Transcripts are delegate-controlled input; every consumer that
// reads one unbounded is a wedge surface, and consumers that read the
// live export separately can be shown different bytes. The snapshot is
// the fix for both: the first consumer of an attempt copies the export
// once, bounded, and everyone reads the copy.
package atif

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// MaxTranscriptBytes is the read ceiling. Far above every observed
// export (two-turn exports reach ~350 KB); crossing it is a loud,
// named failure, never a silent truncation.
const MaxTranscriptBytes = 8 << 20

// ErrOversize marks a transcript beyond the ceiling. Callers map it to
// the transcript-oversize terminal, never to identity disagreement and
// never to an empty reply.
var ErrOversize = fmt.Errorf("transcript exceeds the %d-byte ceiling", MaxTranscriptBytes)

// ToolCall is one recorded tool invocation with its raw arguments.
type ToolCall struct {
	ToolCallID   string          `json:"tool_call_id"`
	FunctionName string          `json:"function_name"`
	Arguments    json.RawMessage `json:"arguments"`
}

// Step is one transcript step; only the fields delivery decisions read.
type Step struct {
	StepID    json.Number `json:"step_id"`
	ToolCalls []ToolCall  `json:"tool_calls"`
}

// Transcript is the bounded, decoded view of one export.
type Transcript struct {
	SessionID string         `json:"session_id"`
	Agent     map[string]any `json:"agent"`
	Steps     []Step         `json:"steps"`
	raw       []byte
}

// Raw returns the exact bytes the transcript was decoded from.
func (t *Transcript) Raw() []byte { return t.raw }

// ReadBounded reads and decodes a transcript under the ceiling. An
// over-ceiling file returns ErrOversize with no partial content — a
// truncated transcript must never masquerade as a complete one.
func ReadBounded(path string) (*Transcript, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxTranscriptBytes {
		return nil, ErrOversize
	}
	data, err := io.ReadAll(io.LimitReader(file, MaxTranscriptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxTranscriptBytes {
		return nil, ErrOversize
	}
	var transcript Transcript
	if err := json.Unmarshal(data, &transcript); err != nil {
		return nil, fmt.Errorf("transcript is not parseable: %w", err)
	}
	transcript.raw = data
	return &transcript, nil
}

// Snapshot materializes the attempt's immutable copy on first use and
// returns the decoded snapshot. When the snapshot already exists it is
// read INSTEAD of the live export — usage extraction, settlement, and
// collection all see the same bytes by construction. The write is
// exclusive (O_EXCL): two racing first consumers cannot interleave, the
// loser reads the winner's copy.
func Snapshot(exportPath, snapshotPath string) (*Transcript, error) {
	if transcript, err := ReadBounded(snapshotPath); err == nil {
		return transcript, nil
	} else if err == ErrOversize {
		return nil, err
	}
	transcript, err := ReadBounded(exportPath)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(snapshotPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return ReadBounded(snapshotPath)
		}
		return nil, err
	}
	_, writeErr := file.Write(transcript.Raw())
	closeErr := file.Close()
	if writeErr != nil {
		return nil, writeErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	return transcript, nil
}

// ReadBoundedObject is the generic bounded decode for consumers that
// need fields the typed view does not carry (usage metrics, agent
// metadata): the same ceiling, a UseNumber object decode.
func ReadBoundedObject(path string) (map[string]any, error) {
	transcript, err := ReadBounded(path)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(transcript.Raw()))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("transcript is not parseable: %w", err)
	}
	object, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("transcript is not a JSON object")
	}
	return object, nil
}

// SnapshotObject materializes (or reuses) the attempt snapshot and
// returns its generic decode — the object-shaped twin of Snapshot.
func SnapshotObject(exportPath, snapshotPath string) (map[string]any, error) {
	if _, err := Snapshot(exportPath, snapshotPath); err != nil {
		return nil, err
	}
	return ReadBoundedObject(snapshotPath)
}
