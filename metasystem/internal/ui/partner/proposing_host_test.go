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
