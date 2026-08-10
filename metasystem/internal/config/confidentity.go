package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// BuildConfigIdentity is the Go port of config-identity.py: it produces one
// runtime adapter's versioned, canonical configuration identity — a stable
// fingerprint of the configuration that actually affects behavior. Volatile
// keys named by a version-gated filter are excluded so a cosmetic change does
// not read as a capability change. Sources are merged in order (a later source
// overrides an earlier key), missing sources are skipped, and an unparsable
// existing source is a hard error.
func BuildConfigIdentity(runtime, version, filterPath string, sourcePaths []string) (map[string]any, error) {
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

	filteredPaths, warning := loadConfigFilter(filterPath, version)
	if warning != "" {
		fmt.Fprintf(os.Stderr, "warning: %s configuration filter %s %s; hashing all canonical configuration keys\n",
			runtime, filterPath, warning)
	}

	identity := map[string]any{}
	for key, value := range flattened {
		if !excludedKey(key, filteredPaths) {
			identity[key] = value
		}
	}

	encoded, err := canonicalJSON(identity)
	if err != nil {
		return nil, err
	}
	keyHashes := map[string]any{}
	for key, value := range identity {
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

// excludedKey reports whether a flattened key is covered by a filtered path —
// either the exact key or a table prefix naming its leaves.
func excludedKey(key string, filteredPaths []string) bool {
	for _, path := range filteredPaths {
		if key == path || strings.HasPrefix(key, path+".") {
			return true
		}
	}
	return false
}

// loadConfigFilter reads the version-gated exclusion filter, returning the
// excluded key paths, or a warning (and no exclusions) when the filter is
// malformed or the version falls outside its declared range.
func loadConfigFilter(path, version string) (paths []string, warning string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Sprintf("is malformed or unparsable: %v", err)
	}
	var filter struct {
		CLIVersionRange *struct {
			Min *string `json:"min"`
			Max *string `json:"max"`
		} `json:"cliVersionRange"`
		Keys *[]struct {
			Path   string `json:"path"`
			Reason string `json:"reason"`
			Source string `json:"source"`
		} `json:"keys"`
	}
	// Strict: exactly cliVersionRange and keys.
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Sprintf("is malformed or unparsable: %v", err)
	}
	if len(top) != 2 || top["cliVersionRange"] == nil || top["keys"] == nil {
		return nil, "is malformed or unparsable: top level must contain exactly cliVersionRange and keys"
	}
	if err := json.Unmarshal(data, &filter); err != nil {
		return nil, fmt.Sprintf("is malformed or unparsable: %v", err)
	}
	if filter.CLIVersionRange == nil || filter.CLIVersionRange.Min == nil || filter.CLIVersionRange.Max == nil {
		return nil, "is malformed or unparsable: cliVersionRange must contain string min and max values"
	}
	if filter.Keys == nil {
		return nil, "is malformed or unparsable: keys must be an array"
	}
	for _, entry := range *filter.Keys {
		if entry.Path == "" || entry.Reason == "" || entry.Source == "" {
			return nil, "is malformed or unparsable: each key must contain non-empty path, reason, and source strings"
		}
		paths = append(paths, entry.Path)
	}
	minimum, maximum := *filter.CLIVersionRange.Min, *filter.CLIVersionRange.Max
	if !versionInRange(version, minimum, maximum) {
		return nil, fmt.Sprintf("CLI version %s is outside filter range %s through %s", version, minimum, maximum)
	}
	return paths, ""
}

var numericVersionPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)

func numericVersion(value string) []int {
	if !numericVersionPattern.MatchString(value) {
		return nil
	}
	parts := strings.Split(value, ".")
	out := make([]int, len(parts))
	for i, part := range parts {
		out[i], _ = strconv.Atoi(part)
	}
	return out
}

// versionInRange reports whether version is within [minimum, maximum], where a
// maximum ending in ".x" is a prefix wildcard (e.g. "5.x" matches any 5.*).
func versionInRange(version, minimum, maximum string) bool {
	actual := numericVersion(version)
	lower := numericVersion(minimum)
	wildcard := strings.HasSuffix(maximum, ".x")
	upperText := maximum
	if wildcard {
		upperText = maximum[:len(maximum)-2]
	}
	upper := numericVersion(upperText)
	if actual == nil || lower == nil || upper == nil {
		return false
	}
	width := max(len(actual), max(len(lower), len(upper)))
	if compareVersions(pad(actual, width), pad(lower, width)) < 0 {
		return false
	}
	if wildcard {
		if len(actual) < len(upper) {
			return false
		}
		return compareVersions(actual[:len(upper)], upper) == 0
	}
	return compareVersions(pad(actual, width), pad(upper, width)) <= 0
}

func pad(v []int, width int) []int {
	out := make([]int, width)
	copy(out, v)
	return out
}

func compareVersions(a, b []int) int {
	for i := range a {
		if a[i] != b[i] {
			if a[i] < b[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
