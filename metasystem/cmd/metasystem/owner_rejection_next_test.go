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

// A rejection is a rule's refusal: the same command would be refused again,
// so it offers no retry and its sentence stands alone. A lost race, whose
// cause passes, offers the same command again.
func TestOwnerResultOffersARetryOnlyWhenTheCauseMayPass(t *testing.T) {
	t.Parallel()
	for _, detail := range []string{
		"the human approved this intent; unapprove the goal, edit it, then approve the new intent",
		"goal g1 is not open; set-priority changes live backlog goals only",
	} {
		result := ownerResult(&ownerReport{result: &goal.PublishResult{Outcome: goal.OutcomeRejected, Detail: detail}}, 1, intentResult{})
		if result.Outcome != intentRefused || result.Summary != detail || result.retry != "" || len(result.next) != 0 {
			t.Fatalf("rejection %q = %+v; want the sentence alone, no retry", detail, result)
		}
	}
	lost := ownerResult(&ownerReport{result: &goal.PublishResult{Outcome: goal.OutcomeLost, Detail: "winner: other-opid"}}, 1, intentResult{})
	if lost.Outcome != intentRefused || lost.retry != "the goals changed meanwhile; try again" {
		t.Fatalf("lost race = %+v; want the retry", lost)
	}
}
