package refusal

import (
	"errors"
	"fmt"
	"testing"
)

// CodeOf reads the code a typed error carries, through wrapping, and never
// the words: a message that starts with a code-looking word has no code
// (goal error-checks-use-typed-errors).
func TestCodeOfReadsTheTypeNeverTheWords(t *testing.T) {
	t.Parallel()
	coded := fmt.Errorf("start: %w", New("UNIT_RUN_BUSY", "run=r", errors.New("another command is advancing this work")))
	if got := CodeOf(coded); got != "UNIT_RUN_BUSY" {
		t.Fatalf("CodeOf(coded) = %q", got)
	}
	for _, words := range []string{"LAUNCH_BUILD_OVERSIZE size=9 cap=5", "READ_BUSY: another read", "plain words", ""} {
		if got := CodeOf(errors.New(words)); got != "" {
			t.Fatalf("CodeOf(%q) = %q, want no code", words, got)
		}
	}
	if got := CodeOf(nil); got != "" {
		t.Fatalf("CodeOf(nil) = %q", got)
	}
}
