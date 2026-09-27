package uitools_test

import (
	"strconv"
	"strings"
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

// The tool refuses the fifty-first proposal of one answer, at the call.
//
// A bound only at admission is one the Partner learns in its NEXT prompt, after
// the answer has ended, while every call goes on answering that the action was
// prepared — so the human collects rejected lines the Partner was never told
// about (Astra F-06). The answer's own boundary reaches this server through the
// mark the seat writes: the count is per mark, and a new mark starts a new
// answer.
func TestTheFiftyFirstProposalOfOneAnswerIsRefusedAtTheCall(t *testing.T) {
	t.Parallel()
	answer := "turn-1"
	readers := uitools.Readers{Proposals: uitools.NewProposalCount(func() string { return answer })}
	pausing := func(at int) uitools.Args {
		return uitools.Args{"verb": "pause", "goal": "g1-s" + strconv.Itoa(at),
			"reason": "the review has not come back", "explanation": "one of a long list"}
	}
	for at := 1; at <= uitools.MostProposalsPerAnswer; at++ {
		result := readers.Answer(uitools.OpPropose, pausing(at))
		testutil.Require(t, "call "+strconv.Itoa(at)+" is prepared", result.Failed(), false)
	}

	refused := readers.Answer(uitools.OpPropose, pausing(51))
	testutil.Expect(t, "the fifty-first is refused at the call", refused.Failed(), true)
	testutil.Expect(t, "in the words the bound is refused in",
		refused.Problem, uitools.BeyondTheProposalCount(uitools.MostProposalsPerAnswer))
	testutil.Expect(t, "and the model reads it as a refused call",
		strings.Contains(refused.Text(), "this call was refused — "+refused.Problem), true)

	// The next answer is a new count: the mark the seat writes has changed.
	answer = "turn-2"
	next := readers.Answer(uitools.OpPropose, pausing(52))
	testutil.Expect(t, "the next answer's first call is prepared", next.Failed(), false)

	// A call this server would have refused anyway is not one of the fifty: the
	// count stands for the lines an answer carries.
	shapeless := readers.Answer(uitools.OpPropose, uitools.Args{"verb": "pause", "goal": "g1-s53"})
	testutil.Require(t, "a call without its reason is refused", shapeless.Failed(), true)
	for at := 2; at <= uitools.MostProposalsPerAnswer; at++ {
		result := readers.Answer(uitools.OpPropose, pausing(100+at))
		testutil.Require(t, "the second answer's call "+strconv.Itoa(at)+" is prepared",
			result.Failed(), false)
	}
	testutil.Expect(t, "so the fiftieth of that answer is still prepared, and the fifty-first is not",
		readers.Answer(uitools.OpPropose, pausing(200)).Failed(), true)
}

// A server that was told no answer file counts nothing: the bound at admission
// still holds, and a tool that refused every call over a file it could not read
// would take the tool away.
func TestAServerWithNoAnswerMarkCountsNothing(t *testing.T) {
	t.Parallel()
	for what, readers := range map[string]uitools.Readers{
		"told no file":       {},
		"told an empty mark": {Proposals: uitools.NewProposalCount(func() string { return "" })},
	} {
		t.Run(what, func(t *testing.T) {
			t.Parallel()
			for at := 1; at <= uitools.MostProposalsPerAnswer+2; at++ {
				result := readers.Answer(uitools.OpPropose, uitools.Args{
					"verb": "resume", "goal": "g1-s" + strconv.Itoa(at), "explanation": "one of many"})
				testutil.Require(t, "call "+strconv.Itoa(at)+" is prepared", result.Failed(), false)
			}
		})
	}
}
