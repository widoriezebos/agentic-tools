package main

import (
	"slices"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// An owner rejection whose second line names the command that clears it
// renders that command as its next step, not a bare retry: done on a
// breach-stopped goal says to resume it, then repeat.
func TestOwnerRejectionCarriesItsRunLineAsTheNextStep(t *testing.T) {
	t.Parallel()
	report := &ownerReport{result: &goal.PublishResult{Outcome: goal.OutcomeRejected,
		Detail: "goal g1 is breach-stopped by stop-g1-r3-f1; only goal resume may clear its launch fence\nrun: metasystem goal resume g1  (then repeat this command)"}}
	result := ownerResult(report, 1, intentResult{})
	if result.Summary != "goal g1 is breach-stopped by stop-g1-r3-f1; only goal resume may clear its launch fence" ||
		!slices.Equal(result.next, []string{"metasystem", "goal", "resume", "g1"}) || result.nextReason != "then repeat this command" || result.retry != "" {
		t.Fatalf("rejection = %+v", result)
	}
}
