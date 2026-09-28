package main

import "testing"

// The protocol checkers reserve exit 2 for usage, distinct from exit 1 for a
// violation (ported from the retired validate-metasystem.sh agent-protocol
// section).
func TestProtocolCheckersRefuseMissingArgumentsAsUsage(t *testing.T) {
	t.Parallel()
	for name, run := range map[string]func() int{
		"turn-prompt without a file or turn": func() int { return runValidateTurnPrompt(nil) },
		"critique-closed without its inputs": func() int { return runValidateCritiqueClosed(nil) },
	} {
		if code := run(); code != 2 {
			t.Errorf("%s exited %d, want 2 for usage", name, code)
		}
	}
}
