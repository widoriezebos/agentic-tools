package dispatch

import (
	"bytes"
	"encoding/json"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"os"
	"reflect"
	"strconv"
	"strings"
)

// Decoded-JSON helpers for the dispatch verbs: reading numbers strictly
// out of the untyped documents job records are, and rendering values back
// in the canonical byte form those records are compared and hashed in.

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
//
// These are TRANSIENT hand-off files — manifests, patches, resolved
// envelopes — that the shell reads within the same operation and that no
// later run depends on. They are not contractually durable state, so they
// keep the error-only contract and pass no durable anchor
// (go-production-grade B5's inventory is durable STATE mutations; this is
// the recorded classification for this writer).
func writeCompactJSON(path string, value any) error {
	_, err := atomicfile.WriteText(path, jsonCompact(value)+"\n", "")
	return err
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
