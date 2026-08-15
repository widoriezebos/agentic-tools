package usage

// Codex's usage parsing — a per-runtime seam file (agnosticism audit
// class 6). The event-stream walk is shared with claude.go: both
// runtimes append a usage block to rounds/N/events.jsonl and the
// parser takes each field's first present spelling.

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

// The codex dead-round recoverer: the last usage block in the event
// stream (registered seam-locally; the fence consumes by provider).
func init() {
	RegisterRecoverer("codex", func(ctx RecoveryContext) RecoveryOutcome {
		return RecoveryOutcome{State: Recovered, Fields: CodexUsageValue(ctx.EventsPath), Source: ctx.EventsPath}
	})
}
