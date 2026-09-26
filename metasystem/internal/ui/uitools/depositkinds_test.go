package uitools

import (
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The two kinds step 2 added, and the whole of what this server decides about
// them (g1-s55 D1, D2).
//
// Neither is an entry of one of the four piles, and each one's form says so. A
// case is the one kind with two clauses, because it is the one kind offered as a
// choice between two entries — the decision it becomes and the open question it
// becomes — and each of those needs its own. An outcome is a whole section, so
// its bound is a section's and it takes no clause at all.

// A case carries both of the clauses its two presses need, in the framing's own
// labelled lines, with the case itself after the separator.
func TestACaseCarriesTheClauseItWouldBecomeAndTheConsequenceOfLeavingItOpen(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{
		"kind":        "case",
		"text":        "a person reads a long page for an hour without touching anything.",
		"clause":      "reading without input does not keep a session alive",
		"consequence": "long readers are signed out mid-sentence until somebody rules on it",
	})
	testutil.Expect(t, "it is prepared", result.Failed(), false)
	lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
	testutil.Require(t, "six lines", len(lines), 6)
	testutil.Expect(t, "the header names the kind", lines[1], DepositHeader+DepositCase)
	testutil.Expect(t, "the clause it would become is labelled", lines[2],
		DepositClause+"reading without input does not keep a session alive")
	testutil.Expect(t, "and so is the consequence of leaving it open", lines[3],
		DepositConsequence+"long readers are signed out mid-sentence until somebody rules on it")
	testutil.Expect(t, "the framing ends at the separator", lines[4], DepositSeparator)
	testutil.Expect(t, "and the case is the whole of the rest", lines[5],
		"a person reads a long page for an hour without touching anything.")
}

// A case with no clause is prepared all the same: the Decide sheet opens with
// the case itself, and the human writes the clause. The tool does not require of
// the Partner what the human is there to supply.
func TestACaseWithNoClauseIsPreparedWithoutOne(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{"kind": "case", "text": "a laptop sleeps with the page open."})
	testutil.Expect(t, "it is prepared", result.Failed(), false)
	testutil.Expect(t, "with no clause line at all",
		strings.Contains(result.Text(), DepositClause), false)
	testutil.Expect(t, "and no consequence either",
		strings.Contains(result.Text(), DepositConsequence), false)
}

// An outcome is the closing draft: a whole section, carried whole, with no
// clause beside it and every one of its own newlines kept.
func TestAnOutcomeIsAWholeSectionAndCarriesNoClause(t *testing.T) {
	t.Parallel()
	said := "The limit counts from last activity.\n\nConstraints: the mobile client is not changed here.\n\n" +
		"Open questions: what the twelve hours protected."
	result := fixture(t).Answer(OpDeposit, Args{
		"kind": "outcome", "text": said,
		// The clauses of the other kinds are dropped: this card has no place for
		// one, and a labelled line it cannot show would be framing nobody reads.
		"anchor": "sessions.md:14", "reason": "because", "consequence": "so", "clause": "the clause",
	})
	testutil.Expect(t, "it is prepared", result.Failed(), false)
	testutil.Expect(t, "the header names the kind",
		strings.Contains(result.Text(), DepositHeader+DepositOutcome+"\n"), true)
	for _, label := range []string{DepositAnchor, DepositReason, DepositConsequence, DepositClause} {
		testutil.Expect(t, "no "+strings.TrimSpace(label)+" line",
			strings.Contains(result.Text(), "\n"+label), false)
	}
	after := strings.SplitN(result.Text(), DepositSeparator+"\n", 2)
	testutil.Require(t, "the framing ends once", len(after), 2)
	testutil.Expect(t, "and the draft is whole, paragraphs and all",
		strings.TrimRight(after[1], "\n"), said)
}

// The bounds: an entry's for the four that are entries, and a section's for the
// one that is a section. Each is refused in words that name both numbers.
func TestTheTwoNewKindsAreBoundedAsWhatTheyAre(t *testing.T) {
	t.Parallel()
	over := fixture(t).Answer(OpDeposit, Args{"kind": "case", "text": strings.Repeat("c", maxDeposit+1)})
	testutil.Expect(t, "a case past an entry's bound is refused", over.Failed(), true)
	testutil.Expect(t, "naming the case and its bound",
		strings.Contains(over.Problem, "the case carries at most "+strconv.Itoa(maxDeposit)), true)

	long := fixture(t).Answer(OpDeposit, Args{"kind": "outcome", "text": strings.Repeat("o", maxDeposit+1)})
	testutil.Expect(t, "an outcome of that length is prepared, because it is a section", long.Failed(), false)

	huge := fixture(t).Answer(OpDeposit, Args{"kind": "outcome", "text": strings.Repeat("o", maxOutcome+1)})
	testutil.Expect(t, "an outcome past a section's bound is refused", huge.Failed(), true)
	testutil.Expect(t, "naming the outcome and its bound",
		strings.Contains(huge.Problem, "the outcome carries at most "+strconv.Itoa(maxOutcome)), true)

	clause := fixture(t).Answer(OpDeposit, Args{
		"kind": "case", "text": "words.", "clause": strings.Repeat("c", maxClause+1),
	})
	testutil.Expect(t, "a long clause is refused", clause.Failed(), true)
	testutil.Expect(t, "naming the clause",
		strings.Contains(clause.Problem, "the clause on a deposit"), true)
}

// The catalogue offers the five and names them, so a Partner reading the schema
// is told which kinds there are rather than guessing.
func TestTheCatalogueOffersTheFiveKinds(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the five kinds", DepositKinds,
		[]string{DepositFact, DepositDecision, DepositQuestion, DepositCase, DepositOutcome})
	for _, tool := range Catalogue() {
		if tool.Name != OpDeposit {
			continue
		}
		schema, ok := tool.InputSchema["properties"].(map[string]any)
		testutil.Require(t, "the schema has properties", ok, true)
		for _, field := range []string{"kind", "text", "anchor", "reason", "consequence", "clause"} {
			_, named := schema[field]
			testutil.Expect(t, "the schema offers "+field, named, true)
		}
		kind, ok := schema["kind"].(map[string]any)
		testutil.Require(t, "the kind is described", ok, true)
		testutil.Expect(t, "and enumerated from the one list", kind["enum"], DepositKinds)
		testutil.Expect(t, "the description names the case",
			strings.Contains(tool.Description, "case at the edge"), true)
		testutil.Expect(t, "and says when the outcome is offered",
			strings.Contains(tool.Description, "close the sitting"), true)
	}
}

// A proposal is still not one of them, and the refusal says what the closing
// deposit is instead, so a Partner that reaches for one is told where to go.
func TestAProposalIsStillRefusedAndSaysWhatTheClosingDepositIs(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{"kind": "proposal", "text": "words."})
	testutil.Expect(t, "it is refused", result.Failed(), true)
	testutil.Expect(t, "and the refusal names the closing outcome",
		strings.Contains(result.Problem, "the closing outcome"), true)
}
