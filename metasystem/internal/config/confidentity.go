package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// BuildConfigIdentity produces one runtime adapter's canonical configuration
// identity. Sources are merged in order (a later source overrides an earlier
// key), missing sources are skipped, and an unparsable existing source is a
// hard error.
func BuildConfigIdentity(runtime, version string, sourcePaths []string) (map[string]any, error) {
	flattened := map[string]any{}
	for _, raw := range sourcePaths {
		path := expandPath(raw)
		info, err := os.Stat(path)
		if err != nil || info.IsDir() {
			continue
		}
		source, err := loadConfigSource(path)
		if err != nil {
			return nil, fmt.Errorf("cannot canonicalize configuration source %s: %w", path, err)
		}
		for key, value := range source {
			flattened[key] = value
		}
	}

	encoded, err := canonicalJSON(flattened)
	if err != nil {
		return nil, err
	}
	keyHashes := map[string]any{}
	for key, value := range flattened {
		vEncoded, err := canonicalJSON(value)
		if err != nil {
			return nil, err
		}
		keyHashes[key] = sha256Hex(vEncoded)
	}
	return map[string]any{
		"runtime":         runtime,
		"cliVersion":      version,
		"configHash":      sha256Hex(encoded)[:24],
		"configKeyHashes": keyHashes,
	}, nil
}

// CanonicalConfigJSON renders a value as compact, key-sorted JSON without HTML
// escaping — the canonical form the identity hashes over.
func CanonicalConfigJSON(value any) (string, error) {
	return canonicalJSON(value)
}

func canonicalJSON(value any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(value); err != nil {
		return "", err
	}
	return strings.TrimRight(buf.String(), "\n"), nil
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") || path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// loadConfigSource parses a JSON or TOML source and flattens it to dotted keys.
func loadConfigSource(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var value any
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber() // keep 5 and 5.0 distinct in the canonical form
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
	case ".toml":
		if err := toml.Unmarshal(data, &value); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("unsupported configuration source type: %s", path)
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("configuration source must contain an object or table: %s", path)
	}
	return flattenConfig(table)
}

// flattenConfig turns a nested table into dotted leaf keys. A non-empty table
// recurses; a scalar, list, or empty table is a leaf. A key that collides after
// flattening is ambiguous and refused.
func flattenConfig(table map[string]any) (map[string]any, error) {
	result := map[string]any{}
	var visit func(item any, prefix string) error
	visit = func(item any, prefix string) error {
		if child, ok := item.(map[string]any); ok && len(child) > 0 {
			for _, key := range sortedKeys(child) {
				next := key
				if prefix != "" {
					next = prefix + "." + key
				}
				if err := visit(child[key], next); err != nil {
					return err
				}
			}
			return nil
		}
		if _, exists := result[prefix]; exists {
			return fmt.Errorf("configuration key is ambiguous after flattening: %s", prefix)
		}
		result[prefix] = normalizeConfigValue(item)
		return nil
	}
	for _, key := range sortedKeys(table) {
		if err := visit(table[key], key); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// normalizeConfigValue renders TOML date/time values as their ISO strings so
// the canonical form is text, and leaves everything else untouched.
func normalizeConfigValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for k, item := range v {
			out[k] = normalizeConfigValue(item)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = normalizeConfigValue(item)
		}
		return out
	case json.Number:
		return v // keep the numeric literal, not its String() form
	case time.Time:
		return v.Format(time.RFC3339)
	case fmt.Stringer:
		return v.String() // toml.LocalDate / LocalTime / LocalDateTime
	default:
		return value
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
