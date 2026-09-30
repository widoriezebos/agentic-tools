package launch

import (
	"errors"
	"fmt"
	"testing"
)

// A launch refusal reads as a plain reason by default; its code and facts are
// details that ErrorCode and ErrorDetail recover, through wrapping too.
func TestCodedErrorKeepsTheCodeOutOfTheDefaultText(t *testing.T) {
	t.Parallel()
	err := fmt.Errorf("start: %w", coded("UNIT_RUN_BUSY", "unit=u goal=g run=r", errors.New("another command is advancing unit u")))
	if got := err.Error(); got != "start: another command is advancing unit u" {
		t.Fatalf("default text %q", got)
	}
	if got := ErrorCode(err); got != "UNIT_RUN_BUSY" {
		t.Fatalf("code %q", got)
	}
	if got := ErrorDetail(err); got != "UNIT_RUN_BUSY unit=u goal=g run=r: another command is advancing unit u" {
		t.Fatalf("detail %q", got)
	}
	// A code-looking first word is words, never a code: the code is data
	// beside the words (goal error-checks-use-typed-errors).
	if got := ErrorCode(errors.New("LAUNCH_BUILD_OVERSIZE size=9 cap=5")); got != "" {
		t.Fatalf("a code read from the words %q", got)
	}
	if got := ErrorCode(errors.New("plain words")); got != "" {
		t.Fatalf("plain error code %q", got)
	}
	if !errors.Is(coded("DESIGN_ATTEMPT_STALE", "", fmt.Errorf("%w (you named 1)", ErrDesignStale)), ErrDesignStale) {
		t.Fatal("a coded refusal no longer matches its sentinel")
	}
}
