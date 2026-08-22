// Package usage is the single owner of typed usage extraction — the
// correctness-sensitive rules that turn each runtime's raw reporting into
// the typed per-turn usage records the fences aggregate. Host and adapter
// must never carry their own copies: duplicated usage math drifts, and a
// fence must not reach runtime event parsing through an adapter import.
// A Devin metric change or a delta bugfix lands here, once. adapter and
// host front these functions behind their own CLI families; the
// implementation has one home.
//
// RootJobID lives here because usage attribution is per CHAIN: attributing
// a round's spend requires walking to its root. If a dedicated ancestry
// home ever exists, it may move there.
package usage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// providerUnit finds the first metric, in sorted key order, whose name mentions
// ACU and whose value is a number, returning it as a metered provider unit.
func providerUnit(metrics map[string]any) (string, any, bool) {
	for _, key := range sortedKeys(metrics) {
		if !strings.Contains(strings.ToLower(key), "acu") {
			continue
		}
		if _, ok := asFloat(metrics[key]); ok {
			return key, metrics[key], true
		}
	}
	return "", nil, false
}

// RootJobID walks a job's parentJob chain to the root of its lineage: the first
// job with no parent. A job whose parentJob is null or absent is its own root.
// A chain that ever revisits a job it already stepped through is cyclic and
// cannot have a root, so the walk refuses rather than looping forever.
func RootJobID(jobsDir, job string) (string, error) {
	seen := map[string]bool{}
	for {
		if seen[job] {
			return "", fmt.Errorf("cyclic job chain")
		}
		seen[job] = true

		record := loadObject(filepath.Join(jobsDir, job+".json"))
		if record == nil {
			return "", fmt.Errorf("cannot read job record %s", job)
		}
		parent, present := record["parentJob"]
		if !present || parent == nil {
			return job, nil
		}
		next, ok := parent.(string)
		if !ok {
			return "", fmt.Errorf("job %s has a non-string parentJob", job)
		}
		job = next
	}
}

// The helpers below are this leaf package's own copies of tiny pure
// utilities (the C-3 cross-boundary duplication doctrine): the package must
// stay a leaf, so it borrows nothing from adapter or host.

func loadObject(path string) map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return nil
	}
	object, _ := value.(map[string]any)
	return object
}

func jsonlObjects(path string) []map[string]any {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var objects []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 32*1024*1024)
	for scanner.Scan() {
		decoder := json.NewDecoder(bytes.NewReader(scanner.Bytes()))
		decoder.UseNumber()
		var value any
		if decoder.Decode(&value) != nil {
			continue
		}
		if object, ok := value.(map[string]any); ok {
			objects = append(objects, object)
		}
	}
	return objects
}

func firstPresent(object map[string]any, keys ...string) any {
	for _, key := range keys {
		if value, ok := object[key]; ok && value != nil {
			return value
		}
	}
	return nil
}

// atomicWriteJSON renders through wiredoc.RenderValue — the one home of
// the canon detour (adapter-host-registry-2) — so bytes cannot drift.
func atomicWriteJSON(path string, value any) error {
	rendered, err := wiredoc.RenderValue(value)
	if err != nil {
		return err
	}
	_, writeErr := atomicfile.WriteText(path, string(rendered), "")
	return writeErr
}

func asInt(value any) (int64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case float64:
		return int64(typed), typed == float64(int64(typed))
	case int64:
		return typed, true
	case int:
		return int64(typed), true
	}
	return 0, false
}

func asFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case float64:
		return typed, true
	case int64:
		return float64(typed), true
	case int:
		return float64(typed), true
	}
	return 0, false
}

func sortedKeys(object map[string]any) []string {
	keys := make([]string, 0, len(object))
	for key := range object {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// eventStreamUsageValue derives typed usage from the last usage block
// an events.jsonl stream reports — the RUNTIME-NEUTRAL parser both
// claude and codex recoverers share (the
// shared code lives in the neutral file; each runtime's seam file
// wraps or registers it under its own name). Field spellings vary
// across builds, so each takes the first present spelling.
func eventStreamUsageValue(eventsPath string) map[string]any {
	var last map[string]any
	for _, event := range jsonlObjects(eventsPath) {
		if value, ok := event["usage"].(map[string]any); ok {
			last = value
		}
	}
	return map[string]any{
		"availability":      "native",
		"inputTokens":       firstPresent(last, "input_tokens", "inputTokens"),
		"cachedInputTokens": firstPresent(last, "cached_input_tokens", "cachedInputTokens"),
		"outputTokens":      firstPresent(last, "output_tokens", "outputTokens"),
		"reasoningTokens":   firstPresent(last, "reasoning_output_tokens", "reasoning_tokens", "reasoningTokens"),
		"cost":              nil,
		"providerUnits":     nil,
	}
}
