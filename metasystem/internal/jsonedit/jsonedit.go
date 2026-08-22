// Package jsonedit owns the shell-facing JSON verb decisions (review
// architecture-2): how `json get` renders a value for the dozens of shell
// call sites that string-compare its output, how `json set` classifies its
// edits, and how `json object` spells a constructed object. cmd parses
// flags, reads files, and prints; the contract lives here, under a
// coverage floor.
package jsonedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrUsage marks a caller mistake (malformed KEY=VALUE, non-integer --int):
// the verb exits 2 with a prefixed message instead of 1. Match with
// errors.Is; the message itself carries no sentinel text.
var ErrUsage = errors.New("usage")

type usageError struct{ msg string }

func (e usageError) Error() string      { return e.msg }
func (usageError) Is(target error) bool { return target == ErrUsage }
func usagef(format string, args ...any) error {
	return usageError{msg: fmt.Sprintf(format, args...)}
}

// SetFields applies top-level edits to a decoded JSON object: --field pairs
// set strings, --int pairs set parsed int64s. The returned object is the
// caller's to write atomically.
func SetFields(data []byte, stringFields, intFields []string) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	for _, pair := range stringFields {
		key, value, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, usagef("--field %q is not KEY=VALUE", pair)
		}
		object[key] = value
	}
	for _, pair := range intFields {
		key, raw, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, usagef("--int %q is not KEY=VALUE", pair)
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, usagef("--int %q is not an integer", pair)
		}
		object[key] = value
	}
	return object, nil
}

// Object builds the one-line compact JSON object from key=value arguments:
// string values split on the first '=', arguments without '=' skipped,
// keys sorted, HTML left unescaped.
func Object(pairs []string) (string, error) {
	object := map[string]any{}
	for _, arg := range pairs {
		if i := strings.IndexByte(arg, '='); i >= 0 {
			object[arg[:i]] = arg[i+1:]
		}
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(object); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

// Get resolves a dotted field in JSON content and renders it the way shell
// callers compare it: strings bare, integral floats without a decimal
// point, bools as true/false, null as "null", composites as compact JSON.
// def, when non-nil, prints for a missing/null field (and for traversal
// through a non-object). ok=false means unparsable content or an
// unresolvable path with no default — the verb exits 1 printing nothing.
func Get(content []byte, field string, def *string) (string, bool) {
	var current any
	if err := json.Unmarshal(content, &current); err != nil {
		return "", false
	}
	for _, key := range strings.Split(field, ".") {
		object, isObject := current.(map[string]any)
		if !isObject {
			if def != nil {
				return *def, true
			}
			return "", false
		}
		value, present := object[key]
		if !present {
			// A missing field prints the default when one was given, matching
			// the lenient readers that treat absent and null the same.
			if def != nil {
				return *def, true
			}
			return "", false
		}
		current = value
	}
	switch typed := current.(type) {
	case string:
		return typed, true
	case float64:
		// Integers print without a decimal point, so whole numbers are
		// emitted without a trailing ".0".
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed)), true
		}
		return fmt.Sprintf("%v", typed), true
	case bool:
		return fmt.Sprintf("%v", typed), true
	case nil:
		if def != nil {
			return *def, true
		}
		return "null", true
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return "", false
		}
		return string(encoded), true
	}
}

// StripKeys removes named top-level keys from a decoded JSON object — the
// structural replacement for producing settings.json by sed-deleting the
// "_comment" line. Stripping an absent key is a no-op:
// adoption is re-runnable and an already-clean file must stay a success.
func StripKeys(data []byte, keys []string) (map[string]any, error) {
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return nil, err
	}
	for _, key := range keys {
		delete(object, key)
	}
	return object, nil
}
