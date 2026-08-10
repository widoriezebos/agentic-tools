package host

import "fmt"

// ClaudeResult reads a Claude CLI result document and splits it into the turn's
// return object and its typed usage. The return is the structured output when
// present, else the result string parsed as JSON; it is written only when an
// object is found, leaving the runner to report an absent return. Usage always
// lands, reading native token counts and cost from the CLI's own usage block.
func ClaudeResult(providerPath, returnPath, usagePath string) error {
	value, ok := loadValue(providerPath)
	if !ok {
		value = map[string]any{}
	}
	document, _ := value.(map[string]any)

	if candidate := returnObject(document); candidate != nil {
		if err := atomicWriteJSON(returnPath, candidate); err != nil {
			return fmt.Errorf("write claude return: %w", err)
		}
	}

	native, _ := document["usage"].(map[string]any)
	usage := map[string]any{
		"availability":      "native",
		"inputTokens":       native["input_tokens"],
		"cachedInputTokens": native["cache_read_input_tokens"],
		"outputTokens":      native["output_tokens"],
		"reasoningTokens":   native["reasoning_tokens"],
		"cost":              nil,
		"providerUnits":     nil,
	}
	if cost := document["total_cost_usd"]; isNumber(cost) {
		usage["cost"] = map[string]any{"amount": cost, "currency": "USD"}
	}
	if err := atomicWriteJSON(usagePath, usage); err != nil {
		return fmt.Errorf("write claude usage: %w", err)
	}
	return nil
}

// returnObject picks the structured return from a Claude result document: the
// structured_output object when it is one, otherwise the result field parsed as
// a JSON object. It returns nil when neither yields an object.
func returnObject(document map[string]any) map[string]any {
	if structured, ok := document["structured_output"].(map[string]any); ok {
		return structured
	}
	if result, ok := document["result"].(string); ok {
		if parsed, err := decodeJSONNumber([]byte(result)); err == nil {
			if object, ok := parsed.(map[string]any); ok {
				return object
			}
		}
	}
	return nil
}
