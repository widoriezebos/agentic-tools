// Package registry implements the supervision registry contract
// (plans/supervision-registry.md): the single machine-wide custody view
// for supervision owners. Each file in this package names the REG clause
// it implements; the contract document is the authority and this package
// is its executable form.
package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"time"
)

// TornEvent is the framing repair marker (REG-1). It carries no claim
// state; its checkoutPath is empty by definition, because the repairer
// cannot know which checkout the destroyed fragment spoke for.
const TornEvent = "torn"

// AppendFrame appends one record to the registry file under REG-1's
// framing: one JSON object per line, trailing newline, tail inspected
// and repaired first.
//
// Invariant (REG-1, SLC-R7-004): after AppendFrame returns, the payload
// is the file's final line, newline-terminated, and no valid record in
// the file follows a non-JSON line without an intervening torn marker.
// The repair is two-part: a final line that is valid JSON but lost its
// newline is completed with the newline alone (the record was fully
// written); a final line that is not valid JSON is newline-terminated
// and fenced with a torn marker before the payload, because testing
// only for the newline would let this append concatenate two objects
// into fatal mid-file corruption.
//
// The caller must hold the registry lock (REG-4); framing does not lock.
// Writes are issued in repair order (newline, marker, payload) so a
// crash at any byte leaves exactly one of the states the repair rule
// already recovers (SLC-R6-009: the repair is itself crash-recoverable,
// and a re-run of it is idempotent — at worst one more marker).
func AppendFrame(path string, payload []byte) error {
	if !json.Valid(payload) {
		return fmt.Errorf("registry append refused: payload is not valid JSON")
	}
	if bytes.ContainsRune(payload, '\n') {
		return fmt.Errorf("registry append refused: payload spans multiple lines")
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return fmt.Errorf("registry open: %w", err)
	}
	defer file.Close()

	terminated, finalLine, err := inspectTail(file)
	if err != nil {
		return err
	}

	var writes [][]byte
	switch {
	case finalLine == nil:
		// Empty file: nothing to repair.
	case json.Valid(finalLine):
		if !terminated {
			// The record was fully written and only its newline was
			// lost: complete it, nothing else (REG-1 first part).
			writes = append(writes, []byte("\n"))
		}
	default:
		// The final line is not valid JSON: newline-terminate the
		// fragment and fence it with a torn marker (REG-1 second part).
		if !terminated {
			writes = append(writes, []byte("\n"))
		}
		marker, err := tornMarker()
		if err != nil {
			return err
		}
		writes = append(writes, append(marker, '\n'))
	}
	writes = append(writes, append(append([]byte(nil), payload...), '\n'))

	for _, chunk := range writes {
		if _, err := file.Write(chunk); err != nil {
			return fmt.Errorf("registry append: %w", err)
		}
	}
	// Durability is claimed only after the sync succeeds
	// (go-production-grade B6). A failure here is the append families' third
	// outcome — VISIBLE BUT UNCOMMITTED: bytes may already be in the file,
	// but the append is not committed, so the caller returns a plain error
	// and emits no success. The torn tail is the READER's contract, which
	// this package already keeps: the next append inspects the tail and
	// fences an invalid final line with a torn marker, and Reduce surfaces
	// CorruptionError rather than trusting a fragment.
	if err := syncFile(file); err != nil {
		return fmt.Errorf("registry append: not durably written: %w", err)
	}
	return nil
}

// syncFile is the durability barrier, injectable so fault tests can fail it.
var syncFile = func(file *os.File) error { return file.Sync() }

// inspectTail reports whether the file ends with a newline byte and
// returns the final line: the trailing partial line when unterminated,
// otherwise the last complete line. A nil final line means the file is
// empty. The whole file is read; registry compaction (REG-3) bounds its
// size, so tail inspection does not need to seek.
func inspectTail(file *os.File) (terminated bool, finalLine []byte, err error) {
	content, err := os.ReadFile(file.Name())
	if err != nil {
		return false, nil, fmt.Errorf("registry tail inspection: %w", err)
	}
	if len(content) == 0 {
		return true, nil, nil
	}
	terminated = content[len(content)-1] == '\n'
	trimmed := bytes.TrimRight(content, "\n")
	if len(trimmed) == 0 {
		return terminated, nil, nil
	}
	if cut := bytes.LastIndexByte(trimmed, '\n'); cut >= 0 {
		return terminated, trimmed[cut+1:], nil
	}
	return terminated, trimmed, nil
}

func tornMarker() ([]byte, error) {
	marker, err := json.Marshal(map[string]any{
		"schemaVersion": 1,
		"event":         TornEvent,
		"checkoutPath":  "",
		"at":            time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		return nil, fmt.Errorf("torn marker: %w", err)
	}
	return marker, nil
}

// Frame is one physical line of the registry file as the reader saw it.
type Frame struct {
	// Line is 1-based within the file.
	Line int
	// Record holds the parsed object for a valid JSON line, nil for a
	// tolerated fragment.
	Record map[string]any
	// Raw is the line's bytes, kept for reporting.
	Raw []byte
}

// CorruptionError reports the one shape REG-1 does not tolerate:
// garbage followed by a valid record with no intervening torn marker.
// Both safety-critical readers fail closed on it (REG-5).
type CorruptionError struct {
	GarbageLine int
	RecordLine  int
}

func (e *CorruptionError) Error() string {
	return fmt.Sprintf(
		"registry corrupt: non-JSON line %d is followed by a valid record on line %d with no torn marker between them; arming and reaping must fail closed (REG-5)",
		e.GarbageLine, e.RecordLine)
}

// ReadFrames parses the registry file under REG-1's run-tolerance rule:
// a non-JSON line is tolerated iff every valid record after it is
// separated from it by a torn marker, which makes all trailing garbage
// tolerated however an interrupted repair left it. A final valid line
// that lost only its newline is a record (the next writer completes it).
// Fragments are returned as frames with a nil Record so callers can
// report them (a torn tail is "tolerated and reported", REG-5).
//
// A missing file is an empty registry, not an error: the registry is
// created by its first append.
func ReadFrames(path string) ([]Frame, error) {
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("registry read: %w", err)
	}
	var frames []Frame
	garbageLine := 0 // last garbage line not yet fenced by a torn marker
	for i, line := range bytes.Split(content, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		number := i + 1
		if !json.Valid(line) {
			frames = append(frames, Frame{Line: number, Raw: line})
			garbageLine = number
			continue
		}
		var record map[string]any
		if err := json.Unmarshal(line, &record); err != nil {
			frames = append(frames, Frame{Line: number, Raw: line})
			garbageLine = number
			continue
		}
		if garbageLine != 0 && record["event"] != TornEvent {
			return nil, &CorruptionError{GarbageLine: garbageLine, RecordLine: number}
		}
		if record["event"] == TornEvent {
			garbageLine = 0
		}
		frames = append(frames, Frame{Line: number, Record: record, Raw: line})
	}
	return frames, nil
}
