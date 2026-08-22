package usage

import (
	"encoding/json"
	"fmt"
	"os"
)

// The ACP transport's usage owner: PromptResponse.usage arrives
// WITH the completion signal and its observed wire
// semantics are PER-TURN (each turn reports its own context read),
// so no predecessor differencing
// exists on this path. An absent or unreadable usage member
// publishes UNAVAILABLE — never a fabricated number, never a
// legacy-transcript fallback: wire and transcript figures are
// alternatives, and this owner is the wire branch.

type acpUsageOutcome struct {
	JournalError string `json:"journalError"`
	Usage        *struct {
		InputTokens      *int64 `json:"inputTokens"`
		OutputTokens     *int64 `json:"outputTokens"`
		CachedReadTokens *int64 `json:"cachedReadTokens"`
	} `json:"usage"`
}

// ACPUsage derives the turn's typed usage record from an acp turn
// outcome file.
func ACPUsage(usagePath, outcomePath string) error {
	unavailable := map[string]any{
		"availability":      "unavailable",
		"inputTokens":       nil,
		"cachedInputTokens": nil,
		"outputTokens":      nil,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	body, err := os.ReadFile(outcomePath)
	if err != nil {
		return fmt.Errorf("acp outcome unreadable: %w", err)
	}
	var outcome acpUsageOutcome
	if err := json.Unmarshal(body, &outcome); err != nil {
		return fmt.Errorf("acp outcome not JSON: %w", err)
	}
	usage := outcome.Usage
	if outcome.JournalError != "" || usage == nil || usage.InputTokens == nil || usage.OutputTokens == nil {
		// A thinned journal disqualifies native publication the same
		// way an absent member does: the raw evidence is incomplete,
		// so the number is not provably complete either.
		if err := atomicWriteJSON(usagePath, unavailable); err != nil {
			return fmt.Errorf("write acp usage: %w", err)
		}
		return nil
	}
	var cached any
	if usage.CachedReadTokens != nil {
		cached = *usage.CachedReadTokens
	}
	value := map[string]any{
		"availability":      "native",
		"inputTokens":       *usage.InputTokens,
		"cachedInputTokens": cached,
		"outputTokens":      *usage.OutputTokens,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     nil,
	}
	if err := atomicWriteJSON(usagePath, value); err != nil {
		return fmt.Errorf("write acp usage: %w", err)
	}
	return nil
}
