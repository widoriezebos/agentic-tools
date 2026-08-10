// Package adapter holds the shared lifecycle plumbing every runtime adapter
// relies on: walking a job's parent chain to its root, materializing and
// bounding the effective-permissions file a launch is measured against,
// comparing that effective grant to what the job requested, writing the small
// compare-and-swap patch files the record lifecycle consumes, and writing a
// runtime's capability snapshot. The runtime command lines, event parsing, and
// identity stay in each adapter; only the reusable core lives here.
package adapter

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// now is the wall clock, overridable in tests so a snapshot's dated sequence
// and capture time are deterministic.
var now = time.Now

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

// encodeJSON renders a value the way every on-disk artifact in this system is
// rendered: 2-space indent, map keys sorted, HTML left unescaped, and a
// trailing newline. The encoder already appends the newline.
func encodeJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// atomicWriteJSON writes value to path so a reader sees the old bytes or the
// new bytes and never a half-written file: render, write a temp file in the
// target directory, fsync it, rename it into place, then fsync the directory.
func atomicWriteJSON(path string, value any) error {
	data, err := encodeJSON(value)
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(directory, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempName, path); err != nil {
		return err
	}
	syncDir(directory)
	return nil
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
