// Package usage is the single owner of typed usage extraction — the
// correctness-sensitive rules that turn each runtime's raw reporting into
// the typed per-turn usage records the fences aggregate (review
// architecture-1: DevinUsage existed verbatim in host AND adapter, and
// mission/fence.go pulled runtime event parsing through an adapter import).
// A Devin metric change or a delta bugfix lands here, once. adapter and
// host front these functions behind their own CLI families per the
// GSC-R1-003 verb-surface ruling; the implementation has one home.
//
// RootJobID lives here because usage attribution is per CHAIN: attributing
// a round's spend requires walking to its root. If W2's chain-walker
// consolidation (dispatch-supervise-7) builds a dedicated ancestry home,
// it may move there.
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

// devinUsageFields are the cumulative counters Devin reports for a session.
var devinUsageFields = []string{
	"total_prompt_tokens",
	"total_completion_tokens",
	"total_cached_tokens",
	"total_steps",
}

// DevinUsage derives this turn's typed usage from Devin's cumulative session
// metrics. Devin reports the SESSION total on every turn, so each turn
// publishes the delta against its predecessor's stored totals and records its
// own cumulative for the next turn to subtract. A resumed turn that cannot find
// its predecessor's totals publishes unavailable rather than a number that
// would double-count every earlier turn. An enterprise account reports ACU and
// no tokens; ACU rides in providerUnits, never as a token or a cost.
func DevinUsage(usagePath, transcriptPath, cumulativePath, previousPath string, expectPrevious bool) error {
	metrics := map[string]any(nil)
	if transcript := loadObject(transcriptPath); transcript != nil {
		metrics, _ = transcript["final_metrics"].(map[string]any)
	}

	totals := map[string]int64{}
	for _, field := range devinUsageFields {
		if value, ok := asInt(metrics[field]); ok {
			totals[field] = value
		}
	}

	acuKey, acuValue, hasACU := providerUnit(metrics)

	var previous map[string]any
	if previousPath != "" {
		previous = loadObject(previousPath)
	}
	predecessorMissing := expectPrevious && previous == nil

	unavailable := map[string]any{
		"availability":      "unavailable",
		"inputTokens":       nil,
		"cachedInputTokens": nil,
		"outputTokens":      nil,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	if hasACU {
		unavailable["providerUnits"] = map[string]any{"name": "acu", "value": acuValue}
	}

	if len(totals) != len(devinUsageFields) {
		if hasACU {
			if err := atomicWriteJSON(cumulativePath, map[string]any{acuKey: acuValue}); err != nil {
				return fmt.Errorf("write devin cumulative usage: %w", err)
			}
			if predecessorMissing {
				unavailable["providerUnits"] = nil
			} else if earlier, ok := asFloat(previous[acuKey]); ok {
				current, _ := asFloat(acuValue)
				unavailable["providerUnits"] = map[string]any{"name": "acu", "value": current - earlier}
			}
		}
		if err := atomicWriteJSON(usagePath, unavailable); err != nil {
			return fmt.Errorf("write devin usage: %w", err)
		}
		return nil
	}

	cumulative := map[string]any{}
	for field, value := range totals {
		cumulative[field] = value
	}
	if err := atomicWriteJSON(cumulativePath, cumulative); err != nil {
		return fmt.Errorf("write devin cumulative usage: %w", err)
	}
	if predecessorMissing {
		if err := atomicWriteJSON(usagePath, unavailable); err != nil {
			return fmt.Errorf("write devin usage: %w", err)
		}
		return nil
	}

	delta := func(field string) int64 {
		if earlier, ok := asInt(previous[field]); ok {
			return totals[field] - earlier
		}
		return totals[field]
	}
	value := map[string]any{
		"availability":      "native",
		"inputTokens":       delta("total_prompt_tokens"),
		"cachedInputTokens": delta("total_cached_tokens"),
		"outputTokens":      delta("total_completion_tokens"),
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     map[string]any{"name": "devin-steps", "value": delta("total_steps")},
	}
	if err := atomicWriteJSON(usagePath, value); err != nil {
		return fmt.Errorf("write devin usage: %w", err)
	}
	return nil
}

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

// CodexUsageValue derives the typed usage for a Codex turn in memory, from
// the last usage block its event stream reports. Codex spells the same
// counter more than one way across builds, so each field takes the first
// present spelling. Codex reports no cost or provider units, so both stay
// null. Callers that must never write — the mission aggregator recovering a
// killed round's spend from its dead event stream — read this value directly.
func CodexUsageValue(eventsPath string) map[string]any {
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

// atomicWriteJSON renders through the SAME wiredoc canon both former
// copies used (host's canonicalJSON and adapter's encodeJSON were
// byte-identical bodies), so the consolidation changes no on-disk bytes.
func atomicWriteJSON(path string, value any) error {
	seed, err := json.Marshal(value)
	if err != nil {
		return err
	}
	doc, err := wiredoc.Decode(seed)
	if err != nil {
		return err
	}
	rendered, err := doc.Render()
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
