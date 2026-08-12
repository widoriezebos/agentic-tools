// Package missionrunner is the mission runner: the engine that launches and
// drives a mission's unattended host turns, and the decisions that engine
// rests on — adjudicating an orchestrator's turn return against the mission
// state, projecting fence counters into that state, proposing the state after
// a completed, failed, or parked turn, and selecting the mission jobs the
// runner must reap or close. The decision surfaces stay pure and compute
// proposals; the Engine is the single writer that applies them (writes asks,
// advances the hash-chained state, signals processes) so every artifact keeps
// one owner.
package missionrunner

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// terminalJobStatuses are the job statuses that count as finished for fence
// projection, job draining, and chain closing. A job whose record is missing
// or unreadable is treated as still active: losing sight of a job must never
// count as finishing it.
var terminalJobStatuses = map[string]bool{
	"completed": true,
	"failed":    true,
	"timeout":   true,
	"cancelled": true,
}

// legalStreamTransitions are the stream transitions an ORCHESTRATOR may
// request in its turn return. This is deliberately narrower than the mission
// state's own transition table: a parked stream returns to active only
// through an answered ask applied by the runner, never by the orchestrator
// declaring it so. Requesting a stream's current state is always legal (a
// no-op refresh of its reason).
var legalStreamTransitions = map[string]map[string]bool{
	"active":           {"active": true, "parked-reserved": true, "parked-stop-loss": true, "done": true},
	"parked-reserved":  {"parked-reserved": true},
	"parked-stop-loss": {"parked-stop-loss": true},
	"done":             {"done": true},
}

// ScaledSeconds scales a base duration in seconds by the fixture cap scale
// (METASYSTEM_FIXTURE_CAP_SCALE_MILLI, permille, default 1000), rounding up
// and never below one second. Fixtures shrink real-time caps with it.
func ScaledSeconds(base int) (int, error) {
	raw := os.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI")
	if raw == "" {
		raw = "1000"
	}
	scale, err := strconv.Atoi(raw)
	if err != nil || scale < 1 {
		return 0, fmt.Errorf("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer")
	}
	seconds := int(math.Ceil(float64(base) * float64(scale) / 1000))
	if seconds < 1 {
		seconds = 1
	}
	return seconds, nil
}

// Interval reads a poll interval from the named environment variable
// (milliseconds, defaulting to defaultMS when unset) and returns it as a
// duration. A non-positive or non-integer value is a configuration error.
func Interval(name string, defaultMS int) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		raw = strconv.Itoa(defaultMS)
	}
	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 1 {
		return 0, fmt.Errorf("%s must be a positive integer in milliseconds", name)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

// MissionLineage derives the ownership lineage every process of one mission
// shares. Derived, not concatenated: a mission id has no length bound, so
// "mission-runner-<id>" would overflow the 128-character lineage bound for a
// long id and the mission could not arm at all. Truncating instead would let
// two missions sharing a prefix share a lineage, which would misread a
// foreign takeover as a renewal and suppress the epoch bump and sweep. A hash
// is fixed-length for any id and stays recomputable from the mission id.
func MissionLineage(mission string) string {
	sum := sha256.Sum256([]byte(mission))
	return "mission-" + hex.EncodeToString(sum[:])[:32]
}

// missionDirPath is the mission's artifact directory under a checkout root.
func missionDirPath(root, mission string) string {
	return filepath.Join(root, "artifacts", "agents", "missions", mission)
}

// jobsDirPath is the shared job-record directory under a checkout root.
func jobsDirPath(root string) string {
	return filepath.Join(root, "artifacts", "agents", "jobs")
}

// asksDirPath is the mission's ask directory.
func asksDirPath(root, mission string) string {
	return filepath.Join(missionDirPath(root, mission), "asks")
}

// ReadDoc reads a JSON object from a file, preserving number literals so
// values round-trip through the runner unchanged.
func ReadDoc(path string) (map[string]any, error) {
	return readJSONDoc(path)
}

// readJSONDoc reads a JSON object from a file, preserving number literals so
// values round-trip through the runner unchanged.
func readJSONDoc(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeJSONDoc(data)
}

// errNotJSONObject marks a document that parsed but is not an object, so
// callers can word that refusal differently from unreadable bytes.
var errNotJSONObject = errors.New("not a JSON object")

func decodeJSONDoc(data []byte) (map[string]any, error) {
	// Through the wire-document owner (Phase 5.2): one implementation of
	// the frozen grammar. The not-an-object distinction survives — the
	// owner returns its own wording for that case, so it is re-mapped to
	// this package's named error, which readDocLabeled words differently
	// from unreadable bytes (E1).
	doc, err := wiredoc.Decode(data)
	if err != nil {
		if err.Error() == "not a JSON object" {
			return nil, errNotJSONObject
		}
		return nil, err
	}
	return doc.Raw(), nil
}

// deepCopyDoc deep-copies a JSON document, preserving numbers as json.Number.
func deepCopyDoc(doc map[string]any) map[string]any {
	data, _ := json.Marshal(doc)
	copied, _ := decodeJSONDoc(data)
	return copied
}

// sortedKeys returns a map's keys in lexicographic order, the order every
// deterministic choice over streams and chains uses.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// numericEqual reports whether two JSON values are equal, comparing numbers
// by value so an integer written one way matches the same integer written
// another.
func numericEqual(a, b any) bool {
	an, aok := toFloat(a)
	bn, bok := toFloat(b)
	if aok && bok {
		return an == bn
	}
	if aok != bok {
		return false
	}
	if !comparableScalar(a) || !comparableScalar(b) {
		return false
	}
	return a == b
}

// comparableScalar guards numericEqual against composite JSON values, which
// never match a scalar identity field and would panic under ==.
func comparableScalar(v any) bool {
	switch v.(type) {
	case nil, string, bool:
		return true
	}
	return false
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}
