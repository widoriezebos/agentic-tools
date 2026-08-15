package host

import (
	"fmt"
	"os"
)

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

// FinishTurn is the host turn's one outcome adjudication (review
// script-adapters-10, replacing three shell copies): it decides the
// outcome from the observed facts, writes the envelope, and returns the
// exit code of the host's taxonomy — 3 a failed turn, 6 a missing session
// (the adapter's own fault signal; a ROTATED session is reported in the
// envelope and judged once, at the runner's adjudication), 0 completed.
// requireReply is the runtime shape where exit 0 with no reply means
// "could not do it" (Devin): treating it as success would hand the runner
// an empty return and blame the wrong thing.
// acceptedPath, when non-empty, is the delivery walk's accepted
// snapshot (D64 phase 2): the require-reply judgment consults IT
// instead of raw stdout, so a file-delivered host result is a reply.
// The raw path stays in the envelope as evidence either way.
func FinishTurn(resultPath, session, usagePath, rawPath, returnPath, acceptedPath string, cliStatus int64, requireReply bool) (int, error) {
	if cliStatus != 0 {
		if err := ResultWrite(resultPath, session, "failed", usagePath, rawPath, ""); err != nil {
			return 1, err
		}
		return 3, nil
	}
	replyEvidence := rawPath
	if acceptedPath != "" {
		replyEvidence = acceptedPath
	}
	if requireReply {
		if info, err := os.Stat(replyEvidence); err != nil || info.Size() == 0 {
			if err := ResultWrite(resultPath, session, "failed", usagePath, rawPath, ""); err != nil {
				return 1, err
			}
			return 3, nil
		}
	}
	if session == "" {
		if err := ResultWrite(resultPath, session, "unresumable", usagePath, rawPath, returnPath); err != nil {
			return 1, err
		}
		return 6, nil
	}
	if err := ResultWrite(resultPath, session, "completed", usagePath, rawPath, returnPath); err != nil {
		return 1, err
	}
	return 0, nil
}
