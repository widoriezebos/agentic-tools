// Package host carries the per-turn work a runtime host does around a single
// CLI invocation: writing the turn's result envelope, extracting a return
// object and typed usage from a runtime's output, and the fixtures the fake
// host stands up. Each function is a self-contained transformation over files
// so the thin shell hosts stay declarative.
package host

import (
	"bytes"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"os"
	"sort"
	"strconv"
)

// canonicalJSON renders a value the way every on-disk artifact in this system
// is rendered: two-space indent, map keys sorted, HTML left intact, and a
// trailing newline (the encoder appends it).
func canonicalJSON(value any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(value); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// atomicWriteJSON writes value to path so a reader sees either the old bytes or
// the new bytes and never a half-written file: render, write a temp file in the
// target directory, fsync it, rename it into place, then fsync the directory.
func atomicWriteJSON(path string, value any) error {
	encoded, err := canonicalJSON(value)
	if err != nil {
		return err
	}
	// Through the durable-write owner (go-production-grade B5); the
	// empty anchor preserves this writer's previous behavior exactly
	// until its caller is converted to the two-outcome contract.
	_, writeErr := atomicfile.WriteText(path, string(encoded), "")
	return writeErr
}

// decodeJSONNumber parses JSON with numbers preserved as json.Number so integer
// counts survive a round trip and are never rounded to a float.
func decodeJSONNumber(raw []byte) (any, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// loadValue reads and parses a JSON file, reporting whether it was readable and
// well-formed so callers can fall back leniently.
func loadValue(path string) (any, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	value, err := decodeJSONNumber(raw)
	if err != nil {
		return nil, false
	}
	return value, true
}

// loadObject reads a JSON file whose top level must be an object; a missing,
// malformed, or non-object file yields nil.
func loadObject(path string) map[string]any {
	value, ok := loadValue(path)
	if !ok {
		return nil
	}
	object, _ := value.(map[string]any)
	return object
}

// nullIfEmpty maps an empty string to a JSON null and any other string to
// itself, so an absent session or return path is recorded as null.
func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// sortedKeys returns an object's keys in ascending order for deterministic
// iteration.
func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// asInt reports a value that is a whole-number JSON integer, rejecting floats
// and non-numbers alike.
func asInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		i, err := strconv.ParseInt(typed.String(), 10, 64)
		return i, err == nil
	default:
		return 0, false
	}
}

// asFloat reports a value that is any JSON number, integer or fractional.
func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// isNumber reports whether a value is any JSON number.
func isNumber(value any) bool {
	_, ok := value.(json.Number)
	return ok
}
