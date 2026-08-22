package usage

import (
	"errors"
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atif"
)

// Devin's usage math — a per-runtime seam file:
// adapter and host both consume it here, once. Devin's
// dead-round recovery is DECLARED UNSUPPORTED: the math needs
// transcript metrics and predecessor cumulative state a dead round's
// event stream does not carry, so recovery stays honestly unavailable.

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
func DevinUsage(usagePath, transcriptPath, snapshotPath, cumulativePath, previousPath string, expectPrevious bool) error {
	// The attempt snapshot: when a snapshot path is given, the
	// transcript is read through atif's bounded, materialize-once copy so
	// usage, settlement, and collection all decide over the SAME bytes.
	// An oversize transcript surfaces as its own error for the caller's
	// transcript-oversize terminal.
	metrics := map[string]any(nil)
	var transcript map[string]any
	if snapshotPath != "" {
		var err error
		transcript, err = atif.SnapshotObject(transcriptPath, snapshotPath)
		if err != nil && errors.Is(err, atif.ErrOversize) {
			return err
		}
	} else {
		transcript = loadObject(transcriptPath)
	}
	if transcript != nil {
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
