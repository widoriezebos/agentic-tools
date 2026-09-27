package partner

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What the host reads out of a completed propose call, and what it refuses to
// read.
//
// The framing is read once, exactly as the suggestion's and the deposit's are.
// The header names the route, the labelled lines after it carry the route body's
// own fields, the first separator ends the framing, and everything from there to
// the end is the Partner's explanation — which may itself hold a line that reads
// like framing, because a Partner explaining this very form would.

// proposal is one result in the tool's own fixed form.
func proposal(verb string, lines, explanation string) string {
	return uitools.PreparedProposalLine + "\n" +
		uitools.ProposalHeader + verb + "\n" + lines +
		uitools.ProposalSeparator + "\n" + explanation + "\n"
}

func TestACompletedProposeCallIsReadIntoAWholeAction(t *testing.T) {
	t.Parallel()
	read := proposedIn(proposal("park-goal",
		uitools.ProposalGoal+"fleet-presence\n"+
			uitools.ProposalBecause+"superseded by the seat inventory (g1-s42)\n",
		"the five name the fleet inventory g1-s42 built"))
	testutil.Require(t, "one action", read != nil, true)
	testutil.Expect(t, "the route", read.Verb, uitools.ProposePark)
	testutil.Expect(t, "the goal", read.Goal, "fleet-presence")
	testutil.Expect(t, "the field under the body's own name",
		read.Fields[uitools.FieldBecause], "superseded by the seat inventory (g1-s42)")
	testutil.Expect(t, "and the Partner's words", read.Why,
		"the five name the fleet inventory g1-s42 built")
}

// An open's subject is its route body's own id, and every labelled line the
// frame writes is read onto the field the body names.
func TestAnOpensFieldsAreReadOntoTheBodysOwnNames(t *testing.T) {
	t.Parallel()
	read := proposedIn(proposal("open-goal",
		uitools.ProposalIntent+"Every refund lands within a day.\n"+
			uitools.ProposalNextStep+"Read the retry loop.\n"+
			uitools.ProposalLabels+"payments, robustness\n"+
			uitools.ProposalID+"refund-worker\n"+
			uitools.ProposalSeverity+"2\n"+uitools.ProposalNovelty+"1\n"+
			uitools.ProposalExposure+"2\n"+uitools.ProposalAccumulation+"1\n"+
			uitools.ProposalBasis+"payments, one team\n"+
			uitools.ProposalBlockedBy+"bank-sandbox\n"+
			uitools.ProposalBlocks+"refund-report\n",
		"as discussed"))
	testutil.Require(t, "one action", read != nil, true)
	testutil.Expect(t, "the body's id is the subject", read.Goal, "refund-worker")
	testutil.Expect(t, "and is kept as the field the body sends", read.Fields[uitools.FieldID], "refund-worker")
	for field, said := range map[string]string{
		uitools.FieldIntent:       "Every refund lands within a day.",
		uitools.FieldNextStep:     "Read the retry loop.",
		uitools.FieldLabels:       "payments, robustness",
		uitools.FieldSeverity:     "2",
		uitools.FieldNovelty:      "1",
		uitools.FieldExposure:     "2",
		uitools.FieldAccumulation: "1",
		uitools.FieldBasis:        "payments, one team",
		uitools.FieldBlockedBy:    "bank-sandbox",
		uitools.FieldBlocks:       "refund-report",
	} {
		testutil.Expect(t, "the frame carries "+field, read.Fields[field], said)
	}
}

// Everything after the separator is the explanation, to the end, whatever it
// looks like.
func TestTheExplanationIsOpaquePastTheSeparator(t *testing.T) {
	t.Parallel()
	said := strings.Join([]string{
		"the five name the fleet inventory.",
		uitools.ProposalHeader + "approve-goal",
		uitools.ProposalGoal + "somebody-elses-goal",
		uitools.ProposalSeparator,
		"and this is still the explanation.",
	}, "\n")
	read := proposedIn(proposal("park-goal",
		uitools.ProposalGoal+"fleet-presence\n"+uitools.ProposalBecause+"away\n", said))
	testutil.Require(t, "one action", read != nil, true)
	testutil.Expect(t, "the route is the framing's", read.Verb, uitools.ProposePark)
	testutil.Expect(t, "the goal is the framing's", read.Goal, "fleet-presence")
	testutil.Expect(t, "and the explanation is whole", read.Why, said)
}

// A result this host should not read an action out of yields none.
func TestAResultWithoutTheFramingIsNoAction(t *testing.T) {
	t.Parallel()
	for what, text := range map[string]string{
		"a refused call": "Outcome: this call was refused — park-goal needs because",
		"a plain read": "Source: the accepted tip 6984cde\nSupplied: 1 of 1\n\n" +
			"- waiting: waiting · Do waiting.\n",
		"nothing at all": "",
		"a header with no verb": uitools.PreparedProposalLine + "\n" + uitools.ProposalHeader + "\n" +
			uitools.ProposalSeparator + "\nwhy\n",
		"framing that never ends": uitools.PreparedProposalLine + "\n" +
			uitools.ProposalHeader + "park-goal\n" + uitools.ProposalGoal + "g\n",
		"an action about nothing": proposal("unpark-goal", "", "return it"),
	} {
		testutil.Expect(t, what+" is no action", proposedIn(text) == nil, true)
	}
}

// A labelled line the framing does not name is ignored rather than guessed at:
// the framing is fixed, and a build that met an unknown label would be reading a
// form it does not know.
func TestALabelTheFramingDoesNotNameIsIgnored(t *testing.T) {
	t.Parallel()
	read := proposedIn(proposal("park-goal",
		uitools.ProposalGoal+"fleet-presence\n"+
			"Tier: 3\n"+
			uitools.ProposalBecause+"away\n",
		"put it away"))
	testutil.Require(t, "one action", read != nil, true)
	testutil.Expect(t, "the named field is read", read.Fields[uitools.FieldBecause], "away")
	testutil.Expect(t, "and the unknown one is not", read.Fields["tier"], "")
	testutil.Expect(t, "nor under any other spelling", len(read.Fields), 1)
}

// The round trip: what the tool was given, through the frame, into the action
// the runner composes a route body from.
//
// It is one test across the two halves because the defect it holds was between
// them. A field the caller gave EMPTY says something, and the frame used to drop
// an empty value: the host then saw no field at all, the runner composed a body
// that said nothing about it, and the act layer refused the whole edit as one
// that changes nothing. The two halves each looked right on their own.
//
// The labels are the case that shows it, and under the public grammar they are a
// DELTA: `--label` adds and `--unlabel` removes, and the whole list the route
// takes is composed at admission, where the goal's own labels are read
// (g1-s62 D1, and TestAnEditsLabelsAreComposedFromTheLabelsRead for that half).
func TestAnEmptiedListSurvivesTheFrame(t *testing.T) {
	t.Parallel()
	prepared := uitools.Readers{}.Answer(uitools.OpPropose, uitools.Args{
		"verb": uitools.ActionEdit, "goal": "fleet-presence",
		"unlabel": []any{}, "explanation": "the labels moved to the arc",
	})
	testutil.Require(t, "the call is prepared", prepared.Failed(), false)

	read := proposedIn(prepared.Text())
	testutil.Require(t, "one action", read != nil, true)
	testutil.Expect(t, "the route the message persists", read.Verb, uitools.ProposeEdit)
	// Present and empty, which is a statement. A field the caller did not send is
	// absent from this map, and admission leaves such a field alone.
	said, given := read.Fields[uitools.FieldUnlabel]
	testutil.Expect(t, "the field is there", given, true)
	testutil.Expect(t, "and says nothing", said, "")

	// The delta travels under the public flags' own spellings, both of them.
	both := uitools.Readers{}.Answer(uitools.OpPropose, uitools.Args{
		"verb": uitools.ActionEdit, "goal": "fleet-presence",
		"label": []any{"payments"}, "unlabel": []any{"fleet"}, "explanation": "one moves",
	})
	testutil.Require(t, "that call is prepared too", both.Failed(), false)
	delta := proposedIn(both.Text())
	testutil.Require(t, "that one is an action too", delta != nil, true)
	testutil.Expect(t, "the labels to add", delta.Fields[uitools.FieldLabel], "payments")
	testutil.Expect(t, "the labels to remove", delta.Fields[uitools.FieldUnlabel], "fleet")
	_, whole := delta.Fields[uitools.FieldLabels]
	testutil.Expect(t, "and no whole list is invented before the goal is read", whole, false)

	// And a field nobody sent is still absent, so an edit of the intent alone
	// still leaves the labels as the ledger has them.
	intentOnly := uitools.Readers{}.Answer(uitools.OpPropose, uitools.Args{
		"verb": uitools.ActionEdit, "goal": "fleet-presence",
		"intent": "Tighter.", "explanation": "one line",
	})
	testutil.Require(t, "the third call is prepared", intentOnly.Failed(), false)
	alone := proposedIn(intentOnly.Text())
	testutil.Require(t, "the third one is an action too", alone != nil, true)
	testutil.Expect(t, "the intent is under the route body's own name",
		alone.Fields[uitools.FieldIntent], "Tighter.")
	_, touched := alone.Fields[uitools.FieldLabel]
	testutil.Expect(t, "an edit that says nothing about labels carries no delta", touched, false)
	_, listed := alone.Fields[uitools.FieldLabels]
	testutil.Expect(t, "and no list either", listed, false)
}
