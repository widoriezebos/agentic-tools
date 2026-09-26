package uitools

import (
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The second operation that reads nothing, and the whole of what it decides.
//
// It cannot judge whether there is a sitting to deposit into — this process has
// no conversation — so what it can be held to is its arguments and the form it
// answers in. Both are here, because the form is what the interface reads a
// deposit out of and a form that drifted would be a card that stopped appearing.

// The result is the fixed form: the one line the model reads, the header naming
// the kind, the one labelled clause the kind takes, the separator, and the text.
func TestAPreparedDepositAnswersInTheFixedForm(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{
		"kind":   "fact",
		"text":   "the mobile client renews the session differently from the web one.",
		"anchor": "internal/session/session.go:212",
	})
	testutil.Expect(t, "it is not a failure", result.Failed(), false)
	lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
	testutil.Require(t, "five lines", len(lines), 5)
	// It says "prepared" and then says what preparing is not, for the reason the
	// suggestion's own line does: a Partner that read "recorded" would tell a
	// human their decision is in the record when nothing has been written.
	testutil.Expect(t, "the model is told that preparing is not recording", lines[0],
		"prepared; preparing does not record it: the human presses Record it, "+
			"and it enters the record then and not before")
	testutil.Expect(t, "which is the line the rest of the build reads", lines[0], DepositedLine)
	testutil.Expect(t, "the header names the kind", lines[1], DepositHeader+"fact")
	testutil.Expect(t, "the anchor is a labelled line of the framing", lines[2],
		DepositAnchor+"internal/session/session.go:212")
	testutil.Expect(t, "the framing ends at the separator", lines[3], DepositSeparator)
	testutil.Expect(t, "and the text is the whole of the rest", lines[4],
		"the mobile client renews the session differently from the web one.")
}

// Each kind carries the one clause it has a place for, and no other. A reason on
// a fact and an anchor on a decision are the Partner filling in a field the card
// cannot show, so they are dropped rather than carried into it.
func TestEachKindCarriesTheOneClauseItHasAPlaceFor(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		kind  string
		kept  string
		lines []string
	}{
		{"fact", "the anchor", []string{DepositAnchor + "session.go:212"}},
		{"decision", "the reason", []string{DepositReason + "it counts from last activity"}},
		{"question", "the consequence", []string{DepositConsequence + "old sessions keep the old limit"}},
	} {
		result := fixture(t).Answer(OpDeposit, Args{
			"kind": probe.kind, "text": "words.",
			"anchor":      "session.go:212",
			"reason":      "it counts from last activity",
			"consequence": "old sessions keep the old limit",
		})
		lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
		testutil.Require(t, probe.kind+" answers five lines", len(lines), 5)
		testutil.Expect(t, probe.kind+" keeps "+probe.kept, lines[2], probe.lines[0])
	}
}

// A fact with no anchor and a decision with no reason are prepared all the same.
// The requirement belongs to the card, after the human's own edit: they may know
// the anchor the Partner could not find, and a tool that refused here would lose
// the words instead of asking for the one thing missing (g1-s53 D4).
func TestADepositWithNoClauseIsPreparedAndLeavesTheRequirementToTheCard(t *testing.T) {
	t.Parallel()
	for _, kind := range DepositKinds {
		result := fixture(t).Answer(OpDeposit, Args{"kind": kind, "text": "words."})
		testutil.Expect(t, kind+" with no clause is prepared", result.Failed(), false)
		lines := strings.Split(strings.TrimRight(result.Text(), "\n"), "\n")
		testutil.Require(t, kind+" answers four lines", len(lines), 4)
		testutil.Expect(t, kind+" names its kind", lines[1], DepositHeader+kind)
		testutil.Expect(t, kind+" goes straight to the separator", lines[2], DepositSeparator)
	}
}

// It read nothing, so it is stamped with nothing: a source would name a reading
// that never happened and a count would count rows there were none of.
func TestAPreparedDepositIsStampedWithNoReading(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{"kind": "question", "text": "what does the current limit protect?"})
	testutil.Expect(t, "no source", result.Source, "")
	testutil.Expect(t, "no rows supplied", result.Supplied, 0)
	testutil.Expect(t, "no cursor", result.Cursor, "")
	testutil.Expect(t, "and the text says neither", strings.Contains(result.Text(), "Supplied:"), false)
}

// The text is opaque to the end. A deposit may itself hold a line that reads
// like this server's own framing — a fact about this very format, say — and what
// it carries is the entry, not a second header.
func TestAPreparedDepositCarriesItsTextWhole(t *testing.T) {
	t.Parallel()
	said := "the tool answers " + DepositHeader + "fact\n" +
		DepositSeparator + "\nand the interface reads the framing once."
	result := fixture(t).Answer(OpDeposit, Args{"kind": "fact", "text": said, "anchor": "deposit.go:60"})
	_, after, split := strings.Cut(result.Text(), DepositSeparator+"\n")
	testutil.Require(t, "the framing ends once", split, true)
	testutil.Expect(t, "and everything after it is the deposit", strings.TrimRight(after, "\n"), said)
}

// A kind this build does not record is refused in words, and a proposal is
// refused in words that say why: proposals are the fourth kind of working
// material a sitting keeps, and the closing deposit that makes one worth having
// is not in this build (g1-s53 §4).
func TestADepositOfAKindThisBuildDoesNotRecordIsRefused(t *testing.T) {
	t.Parallel()
	for _, probe := range []struct {
		what string
		kind string
		says string
	}{
		{"a proposal", "proposal", "a proposal is not one of them yet"},
		{"something else entirely", "wish", `"wish" is none of them`},
		{"no kind at all", "  ", "a deposit with no kind at all"},
	} {
		result := fixture(t).Answer(OpDeposit, Args{"kind": probe.kind, "text": "words."})
		testutil.Expect(t, probe.what+" is refused", result.Failed(), true)
		testutil.Expect(t, probe.what+" says why", strings.Contains(result.Problem, probe.says), true)
		testutil.Expect(t, probe.what+" carries no framing",
			strings.Contains(result.Text(), DepositSeparator), false)
		testutil.Expect(t, probe.what+" tells the model it was refused",
			strings.HasPrefix(result.Text(), "Outcome: this call was refused"), true)
	}
}

// The kind is read in any case, because a model that writes "Fact" means the
// same thing.
func TestTheKindIsReadInAnyCase(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpDeposit, Args{"kind": "Decision", "text": "the limit counts from last activity."})
	testutil.Expect(t, "it is prepared", result.Failed(), false)
	testutil.Expect(t, "under the kind this build knows",
		strings.Contains(result.Text(), DepositHeader+"decision\n"), true)
}

// The two bounds, refused in words rather than cut: half an entry offered as a
// whole one is an entry a human cannot read as what the Partner meant.
func TestADepositPastItsBoundsIsRefusedInWords(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("e", maxDeposit+1)
	result := fixture(t).Answer(OpDeposit, Args{"kind": "fact", "text": long})
	testutil.Expect(t, "too long is refused", result.Failed(), true)
	testutil.Expect(t, "with both numbers",
		strings.Contains(result.Problem, strconv.Itoa(maxDeposit)) &&
			strings.Contains(result.Problem, strconv.Itoa(maxDeposit+1)), true)

	for _, probe := range []struct {
		clause string
		kind   string
	}{{"anchor", "fact"}, {"reason", "decision"}, {"consequence", "question"}} {
		over := fixture(t).Answer(OpDeposit, Args{
			"kind": probe.kind, "text": "words.", probe.clause: strings.Repeat("c", maxClause+1),
		})
		testutil.Expect(t, "a long "+probe.clause+" is refused", over.Failed(), true)
		testutil.Expect(t, "naming the "+probe.clause,
			strings.Contains(over.Problem, "the "+probe.clause+" on a deposit"), true)
	}

	empty := fixture(t).Answer(OpDeposit, Args{"kind": "fact", "text": "   \n  "})
	testutil.Expect(t, "nothing to deposit is refused", empty.Failed(), true)
	testutil.Expect(t, "and says a deposit is one entry",
		strings.Contains(empty.Problem, "one entry of the record"), true)
}

// The catalogue offers it, the dispatch answers it, and the permission rule
// admits it, because all three read one list.
func TestTheServerAnswersDepositAndSaysSo(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "it is an operation this server answers", Names(OpDeposit), true)
	named := false
	for _, tool := range Catalogue() {
		if tool.Name == OpDeposit {
			named = true
			testutil.Expect(t, "the catalogue says what it is for",
				strings.Contains(tool.Description, "Record it"), true)
			testutil.Expect(t, "and that it weighs nothing",
				strings.Contains(tool.Description, "Weigh nothing"), true)
		}
	}
	testutil.Expect(t, "the catalogue offers it", named, true)
	testutil.Expect(t, "and the server's instructions name it",
		strings.Contains(Instructions, "deposit offers one entry"), true)
}
