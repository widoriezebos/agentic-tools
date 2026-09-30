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

// A malformed launch id is ErrInvalidID by type: a caller that treats it as
// no such launch decides with errors.Is, never on the words.
func TestReadOfAMalformedIDIsErrInvalidID(t *testing.T) {
	t.Parallel()
	if _, err := (Store{Root: t.TempDir()}).Read("../not an id"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("read of a malformed id = %v, want ErrInvalidID", err)
	}
}

// A review refused because the run is still running says so by type; the
// other not-ready refusals do not.
func TestReviewStillRunningIsTyped(t *testing.T) {
	t.Parallel()
	running := reviewStillRunning("r", "running")
	if !IsCode(running, "UNIT_REVIEW_NOT_READY") || !errors.Is(running, ErrRunStillRunning) || running.Error() != "run r is still running, so there is no result to review yet" {
		t.Fatalf("still running = %v", running)
	}
	if other := coded("UNIT_REVIEW_NOT_READY", "", errors.New("attempt 1 is still running green")); errors.Is(other, ErrRunStillRunning) {
		t.Fatal("words that say still running made a still-running refusal")
	}
}
