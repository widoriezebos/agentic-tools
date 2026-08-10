package dispatch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// --- shared JSON, number, path, and digest helpers for the dispatch verbs ---

// numFloat reads a numeric value as a float, refusing booleans and strings.
// Works for json.Number and float64 decodings as well as native integers.
func numFloat(v any) (float64, bool) {
	switch typed := v.(type) {
	case json.Number:
		f, err := typed.Float64()
		return f, err == nil
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	}
	return 0, false
}

// numInt reads a decoded JSON number as an exact integer: a json.Number must
// be integer-literal (no fraction or exponent), a float64 must be whole. This
// is the strict "an integer, not merely a number" check several attestations
// demand.
func numInt(v any) (int64, bool) {
	switch typed := v.(type) {
	case json.Number:
		i, err := typed.Int64()
		return i, err == nil
	case float64:
		if typed == float64(int64(typed)) {
			return int64(typed), true
		}
	}
	return 0, false
}

// numString renders a decoded JSON number in its literal form, for building
// path segments like rounds/<round>.
func numString(v any) (string, bool) {
	switch typed := v.(type) {
	case json.Number:
		return typed.String(), true
	case float64:
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10), true
		}
	}
	return "", false
}

// looseEqual compares two decoded JSON scalars the way attestation checks
// need: numbers compare by value regardless of representation, everything
// else compares structurally.
func looseEqual(a, b any) bool {
	if fa, ok := numFloat(a); ok {
		fb, okb := numFloat(b)
		return okb && fa == fb
	}
	if _, okb := numFloat(b); okb {
		return false
	}
	return reflect.DeepEqual(a, b)
}

// jsonCompact renders a value as compact JSON without HTML escaping. Map keys
// serialize sorted, so equal values render identically.
func jsonCompact(value any) string {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return ""
	}
	return strings.TrimSuffix(buf.String(), "\n")
}

// writeCompactJSON writes a value as one compact JSON line (plus newline),
// atomically.
func writeCompactJSON(path string, value any) error {
	return atomicWriteText(path, []byte(jsonCompact(value)+"\n"))
}

// decodeJSONValue parses a JSON document from bytes, keeping numbers in their
// literal form.
func decodeJSONValue(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

// readPlainObject parses a JSON object with default number decoding
// (float64), for readers that do arithmetic rather than byte-stable rewrites.
func readPlainObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return value, nil
}

// resolvePath returns the absolute, symlink-free form of a path. A path that
// does not (yet) exist resolves through its deepest existing ancestor, so a
// destination that will be created still compares against the real directory
// it will land in.
func resolvePath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	suffix := ""
	current := path
	for {
		if real, err := filepath.EvalSymlinks(current); err == nil {
			return filepath.Join(real, suffix)
		}
		parent := filepath.Dir(current)
		if parent == current {
			return filepath.Clean(path)
		}
		suffix = filepath.Join(filepath.Base(current), suffix)
		current = parent
	}
}

// pathWithin reports whether path sits at or below root, comparing whole
// path segments so /a/bc never counts as inside /a/b.
func pathWithin(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, "../"))
}

// parseRecordTime parses the timezone-qualified timestamps job records carry
// (whole-second or fractional, Z or offset).
func parseRecordTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339, value)
}

// sha256File streams a file through SHA-256 and returns the hex digest.
func sha256File(path string) (string, error) {
	handle, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer handle.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, handle); err != nil {
		return "", err
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// gitOutput runs a git subcommand in a directory and returns its trimmed
// stdout.
func gitOutput(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Stderr = io.Discard
	out, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
