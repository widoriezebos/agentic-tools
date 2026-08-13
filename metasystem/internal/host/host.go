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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
	"os"
	"sort"
)

// canonicalJSON renders a value in this family's wire dialect — the
// unescaped canon — through the wire-document owner (Phase 5.3); the corpus
// equivalence test proves the bytes identical to the encoder this replaces.
func canonicalJSON(value any) ([]byte, error) {
	return wiredoc.RenderValue(value)
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

func isNumber(value any) bool {
	_, ok := value.(json.Number)
	return ok
}
