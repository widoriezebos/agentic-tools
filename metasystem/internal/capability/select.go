// Package capability selects and validates the capability snapshot for one
// dispatch. It matches the current configuration identity to a captured
// snapshot, checks the snapshot is fresh and its declared capabilities meet
// the role's requirements, and refuses when the runtime cannot enforce a
// restrictive permission the job requests without a role waiver.
package capability

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// now is the time source, overridable in tests.
var now = time.Now

var (
	identityTokenRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	keyHashRe       = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Result is what a successful selection emits.
type Result struct {
	Path                         string           `json:"path"`
	Fallbacks                    []map[string]any `json:"fallbacks"`
	SessionEstablishedSignal     bool             `json:"sessionEstablishedSignal"`
	SessionEstablishedTimeoutSec int              `json:"sessionEstablishedTimeoutSec"`
	Resume                       bool             `json:"resume"`
}

type snapshotEntry struct {
	captured time.Time
	path     string
	value    map[string]any
}

// Select performs the whole selection and writes the result JSON to outputPath.
func Select(root, runtime, role, identityJSON string, maxAge int, envelopePath, outputPath string) error {
	if maxAge < 0 {
		return fmt.Errorf("capability snapshot maximum age must be non-negative")
	}
	root = resolve(root)
	version, configHash, currentHashes, err := parseIdentity(identityJSON, runtime)
	if err != nil {
		return err
	}

	dir := filepath.Join(root, "artifacts", "agents", "capabilities")
	var all []snapshotEntry
	paths, _ := filepath.Glob(filepath.Join(dir, fmt.Sprintf("%s-%s-*.json", runtime, version)))
	for _, path := range paths {
		if entry, ok := loadSnapshot(path, runtime, version); ok {
			all = append(all, entry)
		}
	}

	var candidates []snapshotEntry
	for _, entry := range all {
		if h, _ := entry.value["configHash"].(string); h == configHash {
			candidates = append(candidates, entry)
		}
	}
	if len(candidates) == 0 {
		suffix := changedKeySuffix(currentHashes, all)
		return fmt.Errorf("no capability snapshot matches %s %s %s%s; run %s adapter probe",
			runtime, version, configHash, suffix, runtime)
	}

	best := newest(candidates)
	ageDays := now().UTC().Sub(best.captured.UTC()).Seconds() / 86400
	if ageDays > float64(maxAge) {
		return fmt.Errorf("capability snapshot is stale (%.1f days); re-run %s adapter probe", ageDays, runtime)
	}

	requirementsPath := filepath.Join(root, "scripts", "agents", "roles", role+".requirements.json")
	requirements, err := readObject(requirementsPath)
	if err != nil {
		return fmt.Errorf("cannot evaluate capabilities: %w", err)
	}
	envelope, err := readObject(envelopePath)
	if err != nil {
		return fmt.Errorf("cannot evaluate capabilities: %w", err)
	}

	snapshot := best.value
	caps, _ := snapshot["capabilities"].(map[string]any)
	if caps == nil {
		caps = map[string]any{}
	}
	enforcement, ok := snapshot["envelopeEnforcement"].(map[string]any)
	if !ok || !validEnforcement(enforcement) {
		return fmt.Errorf("capability snapshot has no valid envelope enforcement declaration; re-run adapter probe")
	}
	timeout, ok := intCapability(caps["sessionEstablishedTimeoutSec"], 2)
	if !ok || timeout < 1 || timeout > 60 {
		return fmt.Errorf("capability snapshot has an invalid session-established timeout")
	}

	var missing []string
	for _, name := range stringList(requirements["required"]) {
		if caps[name] != true {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("required runtime capabilities are absent: %s", strings.Join(missing, ", "))
	}

	fallbacks := []map[string]any{}
	if optional, ok := requirements["optional"].(map[string]any); ok {
		for _, name := range sortedMapKeys(optional) {
			if caps[name] != true {
				var fallback any
				if decl, ok := optional[name].(map[string]any); ok {
					fallback = decl["fallback"]
				}
				fallbacks = append(fallbacks, map[string]any{"capability": name, "fallback": fallback})
			}
		}
	}

	waivers, _ := requirements["waivers"].(map[string]any)
	unverified := permissionsUnverified(snapshot)
	for _, field := range unverified {
		if isRestrictive(field, envelope[field], enforcement) && !waived(waivers, field, runtime) {
			return fmt.Errorf("runtime %s cannot enforce restrictive permission field %s (requested %v); "+
				"record a role waiver for %s in %s or choose another runtime",
				runtime, field, envelope[field], runtime, filepath.Base(requirementsPath))
		}
	}

	rel, err := filepath.Rel(root, best.path)
	if err != nil {
		rel = best.path
	}
	result := Result{
		Path:                         rel,
		Fallbacks:                    fallbacks,
		SessionEstablishedSignal:     caps["sessionEstablishedSignal"] == true,
		SessionEstablishedTimeoutSec: timeout,
		Resume:                       caps["resume"] == true,
	}
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, append(encoded, '\n'), 0o644)
}

func parseIdentity(raw, runtime string) (version, configHash string, keyHashes map[string]string, err error) {
	var value map[string]any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return "", "", nil, fmt.Errorf("%s adapter returned a malformed configuration identity", runtime)
	}
	if r, _ := value["runtime"].(string); r != runtime {
		return "", "", nil, fmt.Errorf("%s adapter returned a malformed configuration identity", runtime)
	}
	version, _ = value["cliVersion"].(string)
	configHash, _ = value["configHash"].(string)
	hashes, ok := validKeyHashes(value["configKeyHashes"])
	if !identityTokenRe.MatchString(version) || !identityTokenRe.MatchString(configHash) || !ok {
		return "", "", nil, fmt.Errorf("%s adapter returned a malformed configuration identity", runtime)
	}
	return version, configHash, hashes, nil
}

// validKeyHashes checks a configKeyHashes map is string->64hex and returns it.
func validKeyHashes(value any) (map[string]string, bool) {
	obj, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	out := make(map[string]string, len(obj))
	for key, raw := range obj {
		digest, ok := raw.(string)
		if !ok || !keyHashRe.MatchString(digest) {
			return nil, false
		}
		out[key] = digest
	}
	return out, true
}

func loadSnapshot(path, runtime, version string) (snapshotEntry, bool) {
	value, err := readObject(path)
	if err != nil {
		return snapshotEntry{}, false
	}
	capturedRaw, _ := value["capturedAt"].(string)
	captured, err := parseTimestamp(capturedRaw)
	if err != nil {
		return snapshotEntry{}, false
	}
	if r, _ := value["runtime"].(string); r != runtime {
		return snapshotEntry{}, false
	}
	if v, _ := value["cliVersion"].(string); v != version {
		return snapshotEntry{}, false
	}
	return snapshotEntry{captured: captured, path: path, value: value}, true
}

func parseTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable timestamp %q", s)
}

// newest returns the entry with the greatest (capturedAt, filename).
func newest(entries []snapshotEntry) snapshotEntry {
	best := entries[0]
	for _, entry := range entries[1:] {
		if entry.captured.After(best.captured) ||
			(entry.captured.Equal(best.captured) && filepath.Base(entry.path) > filepath.Base(best.path)) {
			best = entry
		}
	}
	return best
}

func changedKeySuffix(current map[string]string, snapshots []snapshotEntry) string {
	if len(snapshots) == 0 {
		return ""
	}
	previous := newest(snapshots)
	previousHashes, ok := validKeyHashes(previous.value["configKeyHashes"])
	if !ok {
		return ""
	}
	seen := map[string]bool{}
	var changed []string
	for key := range current {
		if current[key] != previousHashes[key] {
			changed = append(changed, key)
			seen[key] = true
		}
	}
	for key := range previousHashes {
		if !seen[key] && current[key] != previousHashes[key] {
			changed = append(changed, key)
		}
	}
	if len(changed) == 0 {
		return ""
	}
	sort.Strings(changed)
	return "; changed configuration keys: " + strings.Join(changed, ", ")
}

func validEnforcement(enforcement map[string]any) bool {
	if len(enforcement) != 3 {
		return false
	}
	for _, field := range []string{"writeRoots", "readRoots", "network"} {
		v, ok := enforcement[field].(string)
		if !ok || (v != "mapped" && v != "notEnforced") {
			return false
		}
	}
	return true
}

// isRestrictive reports whether an envelope field names a restriction the
// runtime would have to enforce. It is per field: a non-empty root array is a
// bounded grant; an empty writeRoots is a restriction only when the runtime's
// write boundary is notEnforced (it can still write through a shell); and
// network/approvals/tools are restrictive whenever they are not their most
// permissive value.
func isRestrictive(field string, value any, enforcement map[string]any) bool {
	switch field {
	case "writeRoots":
		if list, ok := value.([]any); ok && len(list) > 0 {
			return true
		}
		return enforcement["writeRoots"] == "notEnforced"
	case "readRoots":
		list, ok := value.([]any)
		return ok && len(list) > 0
	case "network":
		return value != "allow"
	case "approvals":
		return value != "allow"
	case "tools":
		return value != "runtime-default"
	default:
		return false
	}
}

func waived(waivers map[string]any, field, runtime string) bool {
	list, ok := waivers[field].([]any)
	if !ok {
		return false
	}
	for _, r := range list {
		if s, ok := r.(string); ok && s == runtime {
			return true
		}
	}
	return false
}

func permissionsUnverified(snapshot map[string]any) []string {
	permissions, ok := snapshot["permissions"].(map[string]any)
	if !ok {
		return nil
	}
	return stringList(permissions["unverified"])
}

func intCapability(value any, def int) (int, bool) {
	if value == nil {
		return def, true
	}
	if _, isBool := value.(bool); isBool {
		return 0, false
	}
	if f, ok := value.(float64); ok && f == math.Trunc(f) {
		return int(f), true
	}
	return 0, false
}

func stringList(value any) []string {
	list, ok := value.([]any)
	if !ok {
		return nil
	}
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func readObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func resolve(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		return resolved
	}
	return root
}
