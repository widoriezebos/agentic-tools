package host

import "fmt"

// ResultWrite writes the turn's result envelope, the one artifact the mission
// runner reads back: exactly sessionId, outcome, usage, rawPath, and
// returnPath. An unreadable usage file records usage as unavailable rather than
// failing the turn, and an empty session or return path is recorded as null.
func ResultWrite(resultPath, session, outcome, usagePath, rawPath, returnPath string) error {
	usage, ok := loadValue(usagePath)
	if !ok {
		usage = map[string]any{"availability": "unavailable"}
	}
	envelope := map[string]any{
		"sessionId":  nullIfEmpty(session),
		"outcome":    outcome,
		"usage":      usage,
		"rawPath":    rawPath,
		"returnPath": nullIfEmpty(returnPath),
	}
	if err := atomicWriteJSON(resultPath, envelope); err != nil {
		return fmt.Errorf("write result envelope: %w", err)
	}
	return nil
}

// FakeResult writes the fake host's result envelope with the fixed typed usage
// the test double reports. A completed turn carries its return path; a failed
// turn carries none.
func FakeResult(resultPath, session, rawPath, returnPath, outcome string) error {
	var usage map[string]any
	switch outcome {
	case "completed":
		usage = fakeUsage(11, 2, 7)
	case "failed":
		usage = fakeUsage(1, 0, 0)
	default:
		return fmt.Errorf("fake result: unknown outcome %q", outcome)
	}
	envelope := map[string]any{
		"sessionId":  nullIfEmpty(session),
		"outcome":    outcome,
		"usage":      usage,
		"rawPath":    rawPath,
		"returnPath": nullIfEmpty(returnPath),
	}
	if err := atomicWriteJSON(resultPath, envelope); err != nil {
		return fmt.Errorf("write fake result envelope: %w", err)
	}
	return nil
}

// fakeUsage builds the fake host's native usage shape with the given token
// counts and a single fake-host-turn provider unit.
func fakeUsage(input, cached, output int) map[string]any {
	return map[string]any{
		"availability":      "native",
		"inputTokens":       input,
		"cachedInputTokens": cached,
		"outputTokens":      output,
		"reasoningTokens":   nil,
		"cost":              nil,
		"providerUnits":     map[string]any{"name": "fake-host-turn", "value": 1},
	}
}
