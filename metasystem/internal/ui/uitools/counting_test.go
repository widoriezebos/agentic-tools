package uitools_test

import (
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// The number of proposals one answer carries, and the sentence that says so.
//
// Both are here, in the package that holds every other bound a proposal is held
// to, because the sentence spells the number in words: one of them moving
// without the other is the Partner being told to do something other than what
// the interface will accept (R-130-ui).
func TestTheProposalCountIsBoundedAtFifty(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the bound is fifty", uitools.MostProposalsPerAnswer, 50)
	testutil.Expect(t, "an answer carrying forty-nine may carry one more",
		uitools.BeyondTheProposalCount(49), "")
	testutil.Expect(t, "one carrying fifty may not",
		uitools.BeyondTheProposalCount(50),
		"this answer already carries fifty proposals; say how many remain and "+
			"propose them in your next answer, after the human has applied these")
	testutil.Expect(t, "and nor may one carrying more",
		uitools.BeyondTheProposalCount(51), uitools.BeyondTheProposalCount(50))
}
