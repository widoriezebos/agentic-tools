package adapter

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The per-runtime adapters share a handful of small transformations over a
// runtime's own output: parsing a version token out of --version chatter,
// reading JSONL event streams, and the number/string coercions that keep an
// on-disk usage shape exact. They live here so each runtime file carries only
// what is specific to that runtime.

var versionPattern = regexp.MustCompile(`[0-9]+(?:\.[0-9A-Za-z_-]+)+`)

// ParseCLIVersion extracts the first dotted version token from a CLI's
// --version output, so a runtime's exact build can seed its configuration
// identity. It fails when the output carries no such token.
func ParseCLIVersion(r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	match := versionPattern.Find(data)
	if match == nil {
		return "", fmt.Errorf("could not parse CLI version")
	}
	return string(match), nil
}

// jsonlObjects reads a JSONL file and returns each line that parses as a JSON
// object, numbers preserved. An unreadable file yields no objects and a
// malformed or non-object line is skipped, so a partially written event stream
// is read as far as it is valid.
func jsonlObjects(path string) []map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var objects []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		value, err := decodeJSONBytes(scanner.Bytes())
		if err != nil {
			continue
		}
		if object, ok := value.(map[string]any); ok {
			objects = append(objects, object)
		}
	}
	return objects
}

// loadAny reads a JSON document, yielding nil when it is absent or malformed so
// callers can fall back leniently.
func loadAny(path string) any {
	value, err := decodeJSON(path)
	if err != nil {
		return nil
	}
	return value
}

// loadObjectOrEmpty reads a JSON object, yielding an empty mutable object when
// the file is absent, unreadable, or not an object.
func loadObjectOrEmpty(path string) map[string]any {
	if object, err := readObject(path); err == nil && object != nil {
		return object
	}
	return map[string]any{}
}

// appendJSONLine appends value as one compact JSON line to a flight-recorder
// events file, creating the file and its parent when absent.
func appendJSONLine(path string, value any) error {
	line, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

// firstString returns the value of the first key that holds a non-empty string.
func firstString(object map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := object[key].(string); ok && value != "" {
			return value
		}
	}
	return ""
}

func sortedStringKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// stringList collects the string members of a JSON array, ignoring non-strings.
func stringList(value any) []string {
	list, _ := value.([]any)
	result := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// stringSlice widens a []string into the []any a JSON array member needs,
// staying an empty array (never a null) when there is nothing to list.
func stringSlice(items []string) []any {
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func isNumber(value any) bool {
	_, ok := value.(json.Number)
	return ok
}

// scalarString renders a scalar the way a shell caller expects to read it: a
// string bare, a number as its token, and a boolean lowercase.
func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		encoded, _ := json.Marshal(typed)
		return string(encoded)
	}
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellQuote renders a string so a shell reads it as a single word: a string of
// only safe characters is left bare, anything else is single-quoted with
// embedded quotes escaped.
func shellQuote(value string) string {
	if value != "" && shellSafe.MatchString(value) {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}
