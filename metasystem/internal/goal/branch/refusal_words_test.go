package branch

import (
	"fmt"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// A goal-branch refusal reads as its words; its code is data for --verbose,
// --json and records ("Messages a Person Reads").
func TestBranchRefusalsKeepTheirCodeOutOfTheirWords(t *testing.T) {
	t.Parallel()
	op := fmt.Errorf("goal branch commit: %w", operationRefusal(NotHolderCode, "goal %s is held by another session", "g1"))
	if op.Error() != "goal branch commit: goal g1 is held by another session" || goal.RefusalCode(op) != NotHolderCode ||
		goal.RecordText(op) != NotHolderCode+": goal branch commit: goal g1 is held by another session" {
		t.Fatalf("operation refusal: words %q code %q record %q", op.Error(), goal.RefusalCode(op), goal.RecordText(op))
	}
	rangeErr := rangeRefusal("g1", "abc123", "the commit touches two units")
	if rangeErr.Error() != "commit abc123: the commit touches two units\nrun: metasystem work status g1" || goal.RefusalCode(rangeErr) != RangeCode {
		t.Fatalf("range refusal: words %q code %q", rangeErr.Error(), goal.RefusalCode(rangeErr))
	}
}
