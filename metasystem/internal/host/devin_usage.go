package host

import (
	"fmt"
	"strings"
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
	usage := map[string]any{
		"availability":      "native",
		"inputTokens":       delta("total_prompt_tokens"),
		"cachedInputTokens": delta("total_cached_tokens"),
		"outputTokens":      delta("total_completion_tokens"),
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     map[string]any{"name": "devin-steps", "value": delta("total_steps")},
	}
	if err := atomicWriteJSON(usagePath, usage); err != nil {
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
