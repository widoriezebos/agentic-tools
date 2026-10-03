package partner

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What the host reads out of a completed deposit call, and what it refuses to
// read.
//
// The framing is read once, exactly as the suggestion's is. That is the whole
// safety of a textual channel: the header names the kind, the labelled lines
// after it carry the clause, the first separator ends the framing, and
// everything from there to the end is the Partner's words — which may themselves
// hold a line that reads like framing, because a fact about this interface would.

// deposited is one result in the tool's own fixed form.
func deposited(kind, clause, text string) string {
	return uitools.DepositedLine + "\n" +
		uitools.DepositHeader + kind + "\n" + clause +
		uitools.DepositSeparator + "\n" + text + "\n"
}

func TestACompletedDepositCallIsReadIntoAWholeDeposit(t *testing.T) {
	t.Parallel()
	said := "the mobile client renews the session differently from the web one."
	read := depositedIn(deposited("fact", uitools.DepositAnchor+"internal/session/session.go:212\n", said))
	testutil.Require(t, "one deposit", read != nil, true)
	testutil.Expect(t, "the kind", read.Kind, uitools.DepositFact)
	testutil.Expect(t, "the anchor", read.Anchor, "internal/session/session.go:212")
	testutil.Expect(t, "the text, whole", read.Text, said)
	testutil.Expect(t, "no reason on a fact", read.Reason, "")
	// The subject is the service's to stamp: the model is never told which record
	// the human is sitting on, so a deposit that arrived carrying one would have
	// guessed it.
	testutil.Expect(t, "and no subject yet", read.Subject.ID, "")
	testutil.Expect(t, "nor is it offered here", read.Offered, false)
}

// A decision's reason and a question's consequence are read onto their own
// fields, so the card can show each where it belongs.
func TestEachKindsClauseIsReadOntoItsOwnField(t *testing.T) {
	t.Parallel()
	decision := depositedIn(deposited("decision",
		uitools.DepositReason+"it counts from last activity\n", "the limit counts from last activity."))
	testutil.Require(t, "one decision", decision != nil, true)
	testutil.Expect(t, "with its reason", decision.Reason, "it counts from last activity")

	question := depositedIn(deposited("question",
		uitools.DepositConsequence+"sessions made before the rule keep the old limit\n",
		"what does the current limit protect?"))
	testutil.Require(t, "one question", question != nil, true)
	testutil.Expect(t, "with its consequence", question.Consequence,
		"sessions made before the rule keep the old limit")

	bare := depositedIn(deposited("fact", "", "nothing recorded says what the limit protects."))
	testutil.Require(t, "one fact", bare != nil, true)
	testutil.Expect(t, "and a deposit with no clause is still a deposit", bare.Anchor, "")
}

// A finding's plain layers are read onto fields of their own
// (review-findings-read-as-decisions §4): how much it matters, the problem in a
// person's words, why it matters, and the decision the reviewer recommends with
// its reason, which rides the Reason line a decision's reason does.
func TestAFindingsPlainLayersAreReadOntoTheirOwnFields(t *testing.T) {
	t.Parallel()
	read := depositedIn(deposited("finding",
		uitools.DepositAnchor+"internal/owner.go:60\n"+
			uitools.DepositConsequence+"the lock stays held\n"+
			uitools.DepositSeverity+"blocks\n"+
			uitools.DepositTitle+"A press that dies halfway leaves the ledger locked.\n"+
			uitools.DepositWhy+"The next press waits forever.\n"+
			uitools.DepositRecommends+"must-fix\n"+
			uitools.DepositReason+"Release the lock on every way out.\n",
		"nothing covers a press that dies at owner.go:60"))
	testutil.Require(t, "one finding", read != nil, true)
	testutil.Expect(t, "its severity", read.Severity, "blocks")
	testutil.Expect(t, "its title", read.Title, "A press that dies halfway leaves the ledger locked.")
	testutil.Expect(t, "why it matters", read.Why, "The next press waits forever.")
	testutil.Expect(t, "the recommendation", read.Recommend, "must-fix")
	testutil.Expect(t, "and its reason", read.Reason, "Release the lock on every way out.")
	testutil.Expect(t, "the evidence is still its words", read.Text, "nothing covers a press that dies at owner.go:60")
	testutil.Expect(t, "and its anchor", read.Anchor, "internal/owner.go:60")
}

// The text is opaque, and a line inside it that looks like framing is text.
func TestTheDepositsFramingIsReadOnceAndTheTextIsOpaque(t *testing.T) {
	t.Parallel()
	said := uitools.DepositReason + "a line the Partner quoted\n" +
		uitools.DepositHeader + "decision\n" +
		uitools.DepositSeparator + "\nstill the same fact."
	read := depositedIn(deposited("fact", uitools.DepositAnchor+"session.go:212\n", said))
	testutil.Require(t, "one deposit", read != nil, true)
	testutil.Expect(t, "named by the framing that came first", read.Kind, uitools.DepositFact)
	testutil.Expect(t, "carrying the clause the framing gave it", read.Anchor, "session.go:212")
	testutil.Expect(t, "with no reason read out of its own words", read.Reason, "")
	testutil.Expect(t, "and the text is every line of it", read.Text, said)
}

// A result without the framing is no deposit: a refused call, a failed one, or a
// build that does not write this form.
func TestAResultWithoutTheDepositFramingIsNoDeposit(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		what string
		text string
	}{
		{"a refusal", "Outcome: this call was refused — a proposal is not one of them yet\n"},
		{"an ordinary reading", "Source: plans/designs/d1.md as it stands\nSupplied: 1 of 1\n\n# A design\n"},
		{"a header with no separator", uitools.DepositHeader + "fact\nthe limit is twelve hours.\n"},
		{"a header naming no kind", uitools.DepositHeader + "\n" + uitools.DepositSeparator + "\nwords.\n"},
		{"a separator with nothing after it", deposited("fact", "", "")},
		{"nothing at all", ""},
	} {
		testutil.Expect(t, probe.what+" yields none", depositedIn(probe.text) == nil, true)
	}
}

// A completed call carries both: the look that accounts for it, and the deposit
// whole. The look's excerpt is shortened as every look's is; the deposit is not,
// because the words are what the human is offered.
func TestAPreparedDepositCallIsALookAndADepositBeside(t *testing.T) {
	t.Parallel()
	long := strings.TrimRight(strings.Repeat("a fact stated at length. ", 200), " ")
	body := toolCall{
		ToolCallID: "call-1", Title: "deposit(fact)", Status: "completed",
		Name: "mcp__metasystem__deposit",
	}
	body.Content = append(body.Content, contentBlock(deposited("fact", uitools.DepositAnchor+"a.go:1\n", long)))
	read := depositedIn(resultText(body))
	testutil.Require(t, "the deposit is there", read != nil, true)
	testutil.Expect(t, "and is not shortened", read.Text, long)
	look := lookAt(body.Title, body, true)
	testutil.Expect(t, "the look is named after the call", look.What, "deposit(fact)")
	testutil.Expect(t, "the look is shortened", strings.HasSuffix(look.Excerpt, "…"), true)
	testutil.Expect(t, "and it read nothing, so it is stamped with nothing", look.Source, "")
}

// RF-05: one deposit tool call carrying a finding's five plain fields
// becomes one card carrying them: the tool's own encoder writes them, the host
// reads them back off the same result text, and the card the page renders is
// that deposit, field for field.
func TestOneDepositCallCarriesAFindingsLayersToItsCard(t *testing.T) {
	t.Parallel()
	result := uitools.Readers{}.Answer(uitools.OpDeposit, uitools.Args{
		"kind": "finding", "text": "nothing tries a press that dies at owner.go:21-24",
		"anchor": "internal/owner/owner.go:21-24", "consequence": "the lock stays held",
		"severity": "blocks", "title": "A press that dies halfway leaves the ledger locked.",
		"why": "The next press waits forever.", "recommend": "must-fix", "reason": "Release the lock on every way out.",
	})
	testutil.Require(t, "the tool prepared it", result.Failed(), false)
	body := toolCall{ToolCallID: "call-1", Title: "deposit(finding)", Status: "completed", Name: "mcp__metasystem__deposit"}
	body.Content = append(body.Content, contentBlock(result.Text()))
	read := depositedIn(resultText(body))
	testutil.Require(t, "the host read one finding", read != nil, true)
	card, err := json.Marshal(read)
	testutil.Require(t, "the card's payload", err, nil)
	testutil.Expect(t, "carrying every layer", string(card),
		`{"kind":"finding","text":"nothing tries a press that dies at owner.go:21-24",`+
			`"anchor":"internal/owner/owner.go:21-24","reason":"Release the lock on every way out.",`+
			`"consequence":"the lock stays held","severity":"blocks",`+
			`"title":"A press that dies halfway leaves the ledger locked.","why":"The next press waits forever.",`+
			`"recommend":"must-fix","offered":false}`)
}
