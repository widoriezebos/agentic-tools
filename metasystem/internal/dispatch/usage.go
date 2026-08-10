package dispatch

import (
	"bytes"
	"path/filepath"
)

// ChainUsage aggregates the typed usage of every record in a chain into the
// {chainUsage: {tokens, cost, providerUnits}} patch the root record carries:
// token counts per runtime, cost per currency, provider units per runtime.
// When the aggregate already matches what the root record holds, no patch is
// written and unchanged is true — the caller skips the metadata CAS entirely.
func ChainUsage(jobsDir, root, output string) (unchanged bool, err error) {
	members, err := chainMembers(jobsDir, root)
	if err != nil {
		return false, err
	}

	tokenNames := []string{"inputTokens", "cachedInputTokens", "outputTokens", "reasoningTokens"}
	tokens := map[string]any{}
	costs := map[string]any{}
	units := map[string]any{}
	for _, member := range members {
		// Comparisons and sums work on plain numbers, so re-read each record
		// with float decoding rather than literal-preserving numbers.
		record, err := readPlainObject(member.path)
		if err != nil {
			continue
		}
		usage, ok := record["usage"].(map[string]any)
		if !ok {
			continue
		}
		runtime := "unknown"
		if name, ok := record["runtime"].(string); ok {
			runtime = name
		}
		target, ok := tokens[runtime].(map[string]any)
		if !ok {
			target = map[string]any{}
			for _, name := range tokenNames {
				target[name] = nil
			}
			tokens[runtime] = target
		}
		for _, name := range tokenNames {
			value, ok := usage[name].(float64)
			if !ok {
				continue
			}
			current, _ := target[name].(float64)
			target[name] = current + value
		}
		if cost, ok := usage["cost"].(map[string]any); ok {
			amount, amountOK := cost["amount"].(float64)
			currency, currencyOK := cost["currency"].(string)
			if amountOK && currencyOK {
				current, _ := costs[currency].(float64)
				costs[currency] = current + amount
			}
		}
		if unit, ok := usage["providerUnits"].(map[string]any); ok {
			name, nameOK := unit["name"].(string)
			value, valueOK := unit["value"].(float64)
			if nameOK && valueOK {
				runtimeUnits, ok := units[runtime].(map[string]any)
				if !ok {
					runtimeUnits = map[string]any{}
					units[runtime] = runtimeUnits
				}
				current, _ := runtimeUnits[name].(float64)
				runtimeUnits[name] = current + value
			}
		}
	}

	aggregate := map[string]any{"tokens": tokens, "cost": costs, "providerUnits": units}
	if current, err := readPlainObject(filepath.Join(jobsDir, root+".json")); err == nil {
		if existing, present := current["chainUsage"]; present {
			// Compare canonical renderings, so 5 and 5.0 agree the way the
			// values themselves do.
			if bytes.Equal([]byte(jsonCompact(existing)), []byte(jsonCompact(aggregate))) {
				return true, nil
			}
		}
	}
	return false, writeCompactJSON(output, map[string]any{"chainUsage": aggregate})
}
