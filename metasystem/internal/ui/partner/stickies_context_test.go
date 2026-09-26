package partner

import (
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// What a turn is told about the human's own notepad.
//
// The notepad is outside every checkout precisely so that no seat reads it, so
// a Partner that is not told a sticky cannot find one. That is why the capture
// carries them, and why what it carries is marked for what it is: reminders a
// human wrote to themselves, on no seat's input and in no ledger.

// A note about no subject reaches the Partner when the panel is open, which is
// Astra's F2: "what did I want to remember about the Fleet page" is answered by
// opening the panel and asking.
func TestThePanelsOwnStickiesReachTheTurn(t *testing.T) {
	t.Parallel()

	block := See(Facts{}, Page{
		Section: "Fleet", Path: "/fleet", Sheet: "Stickies",
		StickiesOpen: 2,
		Stickies: []Sticky{
			{Text: "the Fleet page's wording is off"},
			{Text: "ask Sol about the retry", About: []string{"goal g1-s45"}},
		},
	}, composedAt).Block

	testutil.Expect(t, "that the notepad is marked for what it is",
		strings.Contains(block, "private reminders they wrote to themselves, not records"), true)
	testutil.Expect(t, "how many are open", strings.Contains(block, "2 open"), true)
	testutil.Expect(t, "the note about no subject",
		strings.Contains(block, "the Fleet page's wording is off"), true)
	testutil.Expect(t, "what the other one is about",
		strings.Contains(block, "ask Sol about the retry (about goal g1-s45)"), true)
}

// The count travels whether or not any sticky does, so "you have four open
// stickies, none about this page" is an answer the Partner can give.
func TestTheOpenCountTravelsEvenWhenNoStickyDoes(t *testing.T) {
	t.Parallel()

	block := See(Facts{}, Page{Section: "Fleet", Path: "/fleet", StickiesOpen: 4}, composedAt).Block

	testutil.Expect(t, "how many are open", strings.Contains(block, "4 open"), true)
	testutil.Expect(t, "that it says none is about this page",
		strings.Contains(block, "none of them is about what is on this page"), true)
}

// A human with no stickies at all has no line about them: a block that said
// "0 open" on every page would be a sentence nobody needs on every turn.
func TestAnEmptyNotepadSaysNothingAtAll(t *testing.T) {
	t.Parallel()

	block := See(Facts{}, Page{Section: "Fleet", Path: "/fleet"}, composedAt).Block

	testutil.Expect(t, "that the notepad is not mentioned",
		strings.Contains(block, "stickies"), false)
}

// The bound: a notepad holds five hundred, and a block carries what it carries
// and says what it left out.
func TestTheStickiesInOneTurnAreBoundedAndSayWhatIsMissing(t *testing.T) {
	t.Parallel()

	many := make([]Sticky, 0, maxStickiesCarried+5)
	for count := 0; count < maxStickiesCarried+5; count++ {
		many = append(many, Sticky{Text: "one of many"})
	}

	block := See(Facts{}, Page{Section: "Fleet", Path: "/fleet",
		StickiesOpen: len(many), Stickies: many}, composedAt).Block

	testutil.Expect(t, "how many lines were carried",
		strings.Count(block, "  - one of many"), maxStickiesCarried)
	testutil.Expect(t, "that it says what it left out",
		strings.Contains(block, "5 more the page was showing are not in this block"), true)
}

// The boundary's own bound: forty stickies on the wire are twenty-five in the
// capture, and the twenty-five are the first of them.
//
// The page caps what it sends, and this is the check that does not take the
// page's word for it. It matters more than the block's bound does: a capture
// is WRITTEN DOWN, in this checkout's state root, with the message it was
// asked from — so a capture that arrived whole would put a whole notepad of
// private reminders inside the checkout the notepad exists to stay out of.
func TestACaptureIsHeldToTheBoundBeforeAnythingKeepsIt(t *testing.T) {
	t.Parallel()

	many := make([]Sticky, 0, 40)
	for count := 0; count < 40; count++ {
		many = append(many, Sticky{Text: "one of forty"})
	}

	bounded := Page{Section: "Fleet", Path: "/fleet", StickiesOpen: 40, Stickies: many}.Bound()

	testutil.Expect(t, "how many the capture carries", len(bounded.Stickies), maxStickiesCarried)
	testutil.Expect(t, "how many it says it cut", bounded.StickiesCut, 40-maxStickiesCarried)
	testutil.Expect(t, "that the open count is untouched", bounded.StickiesOpen, 40)
	testutil.Expect(t, "that a capture inside the bound is left alone",
		Page{Stickies: many[:maxStickiesCarried]}.Bound().StickiesCut, 0)
}

// What the boundary cut is said, not swallowed: the block counts what it left
// out and what never reached it as one number, so a Partner reading a capped
// notepad knows it is capped.
func TestTheBlockSaysWhatTheBoundaryCutAsWellAsWhatItLeftOut(t *testing.T) {
	t.Parallel()

	many := make([]Sticky, 0, 40)
	for count := 0; count < 40; count++ {
		many = append(many, Sticky{Text: "one of forty"})
	}

	block := See(Facts{}, Page{Section: "Fleet", Path: "/fleet",
		StickiesOpen: 40, Stickies: many}.Bound(), composedAt).Block

	testutil.Expect(t, "how many lines were carried",
		strings.Count(block, "  - one of forty"), maxStickiesCarried)
	testutil.Expect(t, "that it says what is missing",
		strings.Contains(block, "15 more the page was showing are not in this block"), true)
}

// A done sticky says it is done, so a Partner reading a panel with the
// disclosure open does not report a struck-off reminder as outstanding.
func TestADoneStickySaysSo(t *testing.T) {
	t.Parallel()

	block := See(Facts{}, Page{Section: "Fleet", Path: "/fleet", Sheet: "Stickies",
		StickiesOpen: 0, Stickies: []Sticky{{Text: "check g1-s45 tomorrow", Done: true}}}, composedAt).Block

	testutil.Expect(t, "that it says so",
		strings.Contains(block, "check g1-s45 tomorrow — done"), true)
}
