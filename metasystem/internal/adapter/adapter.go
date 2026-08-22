// Package adapter holds the shared lifecycle plumbing every runtime adapter
// relies on: walking a job's parent chain to its root, materializing and
// bounding the effective-permissions file a launch is measured against,
// comparing that effective grant to what the job requested, writing the small
// compare-and-swap patch files the record lifecycle consumes, and writing a
// runtime's capability snapshot. Since the port it also carries the
// runtime-specific decision helpers the shell adapters call back into —
// command construction, event-stream reads, result-field derivation, and
// session correlation (claude.go, codex.go, devin.go); what stays in each
// scripts/agents/adapters/*.sh is launching and OS plumbing, not decisions.
package adapter

import (
	"bytes"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
	"os"
	"path/filepath"
	"time"
)

// now is the wall clock, overridable in tests so a snapshot's dated sequence
// and capture time are deterministic.
var now = time.Now

// timestampUTC renders a time the way every dated artifact in this system is
// stamped: whole seconds, UTC, trailing Z.
func timestampUTC(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05") + "Z"
}

// readObject parses a JSON object from a file, keeping numbers in their exact
// on-disk form so a value that is read and rewritten stays byte-stable.
func readObject(path string) (map[string]any, error) {
	value, err := decodeJSON(path)
	if err != nil {
		return nil, err
	}
	object, _ := value.(map[string]any)
	return object, nil
}

// decodeJSON reads and parses a JSON document, keeping numbers as json.Number.
func decodeJSON(path string) (any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeJSONBytes(data)
}

func decodeJSONBytes(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// encodeJSON renders a value the way every on-disk artifact in this
// system is rendered — 2-space indent, sorted keys, HTML unescaped,
// trailing newline — through the wire-document owner; byte equivalence
// is pinned by the package's own bytecheck test.
func encodeJSON(value any) ([]byte, error) {
	return wiredoc.RenderValue(value)
}

// atomicWriteJSON writes value to path so a reader sees the old bytes or the
// new bytes and never a half-written file: render, write a temp file in the
// target directory, fsync it, rename it into place, then fsync the directory.
func atomicWriteJSON(path string, value any) error {
	encoded, err := encodeJSON(value)
	if err != nil {
		return err
	}
	// Through the durable-write owner; the empty anchor syncs only the
	// target's own directory, and the durable outcome is dropped,
	// because this writer's callers have not adopted the two-outcome
	// contract.
	_, writeErr := atomicfile.WriteText(path, string(encoded), "")
	return writeErr
}

func syncDir(directory string) {
	if dir, err := os.Open(directory); err == nil {
		_ = dir.Sync()
		_ = dir.Close()
	}
}

// resolve returns the absolute, symlink-free form of a path, matching the
// canonical form a launch is measured against.
func resolve(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	if real, err := filepath.EvalSymlinks(path); err == nil {
		return real
	}
	return path
}
