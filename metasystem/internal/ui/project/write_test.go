package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The write surface, against the same fixtures the pane is read from: a real
// checkout with a real intent index, a real register, and the resolver reading
// back whatever was written.
//
// Every case asserts the file as well as the answer. A route that returned the
// right JSON over a file it had not written, or had written somewhere else,
// would pass an assertion on the answer alone.

var wroteAt = time.Date(2026, 9, 22, 14, 31, 0, 0, time.UTC)

func fileAt(t *testing.T, roots Roots, relative string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(roots.Checkout, filepath.FromSlash(relative)))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}
	return string(data)
}

func refusalOf(t *testing.T, err error) *Refusal {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("expected a refusal, got %v", err)
	}
	return refusal
}

// Each kind lands in its own home at the state root, with a fresh id, a draft
// status, the goals that were asked for and the kind's own empty sections —
// and the resolver reads every one of them back as the record it claims to be.
func TestCreateRecordWritesEachKindInItsHome(t *testing.T) {
	t.Parallel()

	for _, kind := range []struct {
		kind, home string
		sections   []string
	}{
		{"decision", "metasystem/docs/decisions", []string{"## Context", "## Decision", "## Consequences"}},
		{"design", "metasystem/plans/designs", []string{"## Outcome", "## Scope", "## What changes", "## Verification"}},
		{"intent", "metasystem/docs/intent", []string{"## Users", "## Outcomes", "## Constraints", "## Open questions"}},
	} {
		t.Run(kind.kind, func(t *testing.T) {
			t.Parallel()
			roots := selfHostedFixture(t)
			seed(t, roots)

			written, err := CreateRecord(roots, NewRecord{
				Kind: kind.kind, Title: "The Project Partner is a drawer", Goals: []string{"ledger-sync"},
				Affects: []string{"design-ledger"},
			}, wroteAt)

			testutil.Require(t, "create the "+kind.kind, err, nil)
			testutil.Expect(t, "the path", written.Path, kind.home+"/the-project-partner-is-a-drawer.md")
			testutil.Expect(t, "the absolute path",
				written.Absolute, filepath.Join(roots.Checkout, filepath.FromSlash(written.Path)))
			testutil.Expect(t, "the kind", written.Record.Kind, kind.kind)
			testutil.Expect(t, "the status", written.Record.Status, "draft")
			testutil.Expect(t, "the goals", written.Record.Goals, []string{"ledger-sync"})
			testutil.Expect(t, "the title", written.Record.Title, "The Project Partner is a drawer")
			testutil.Expect(t, "the home", written.Record.Home, kind.home)
			testutil.Expect(t, "an id was minted", written.Record.ID != "", true)

			on := fileAt(t, roots, written.Path)
			testutil.Expect(t, "the title line", strings.HasPrefix(on, "# The Project Partner is a drawer\n\n"), true)
			testutil.Expect(t, "the head names the id", strings.Contains(on, "- Id: "+written.Record.ID+"\n"), true)
			testutil.Expect(t, "the head is a draft", strings.Contains(on, "- Status: draft\n"), true)
			testutil.Expect(t, "the head names the goals", strings.Contains(on, "- Goals: ledger-sync\n"), true)
			testutil.Expect(t, "the head carries what it affects",
				strings.Contains(on, "- Affects: design-ledger\n"), true)
			for _, section := range kind.sections {
				testutil.Expect(t, "the body carries "+section, strings.Contains(on, "\n"+section+"\n"), true)
			}

			// The pane reads it back as one more record of its kind, which is
			// the only proof that the file is where the resolver looks.
			pane, err := ReadPane(roots, readAt)
			testutil.Require(t, "read the pane", err, nil)
			testutil.Expect(t, "the pane carries it",
				recordWithID(pane.Records, written.Record.ID).Path, written.Path)
		})
	}
}

// An intent chapter that no index names is a chapter nobody reads, so creating
// one appends it to the index's reading order and leaves every other line of
// the index alone.
func TestCreateIntentChapterListsItInTheIndex(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/docs/intent/index.md")

	written, err := CreateRecord(roots, NewRecord{
		Kind: "intent", Title: "Who this is for",
	}, wroteAt)

	testutil.Require(t, "write the chapter", err, nil)
	after := fileAt(t, roots, "metasystem/docs/intent/index.md")
	entry := "- " + written.Record.ID + " — Who this is for"
	testutil.Expect(t, "the index lists the chapter", strings.Contains(after, entry+"\n"), true)
	testutil.Expect(t, "the chapter joins the reading order",
		strings.Contains(after, "— The engine\n"+entry+"\n"), true)
	testutil.Expect(t, "nothing else in the index moved",
		strings.Replace(after, entry+"\n", "", 1), before)

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the book reads it as a chapter",
		pane.Intent.Chapters[len(pane.Intent.Chapters)-1].Title, "Who this is for")
	testutil.Expect(t, "the project refuses nothing", pane.Problems, []Problem{})
}

// A title yields one file name: lower case, hyphens for everything else, runs
// collapsed, the ends trimmed, and no more than sixty characters.
func TestSlugIsTheTitleAsAFileName(t *testing.T) {
	t.Parallel()

	testutil.Expect(t, "a plain title", slug("One binary"), "one-binary")
	testutil.Expect(t, "punctuation and runs",
		slug("  The engine's *one* binary — really!  "), "the-engine-s-one-binary-really")
	testutil.Expect(t, "digits stay", slug("g1-s22 Project, step 2"), "g1-s22-project-step-2")
	testutil.Expect(t, "sixty characters at most",
		slug("The Project Partner is a drawer along the bottom of the page and more"),
		"the-project-partner-is-a-drawer-along-the-bottom-of-the-page")
	testutil.Expect(t, "a title of punctuation names no file", slug("··· !!! ···"), "")
}

// Two records with the same title would be one file, and the second is refused
// rather than written over the first.
func TestCreateRecordRefusesADuplicateFileName(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	asked := NewRecord{Kind: "decision", Title: "One binary", Goals: []string{"ledger-sync"}}
	first, err := CreateRecord(roots, asked, wroteAt)
	testutil.Require(t, "create the first", err, nil)
	on := fileAt(t, roots, first.Path)

	_, err = CreateRecord(roots, asked, wroteAt)

	refusal := refusalOf(t, err)
	testutil.Expect(t, "the refusal", refusal.Kind, RefusalExists)
	testutil.Expect(t, "the reason names the file", refusal.Message,
		"a file already lives at metasystem/docs/decisions/one-binary.md")
	testutil.Expect(t, "the first record is untouched", fileAt(t, roots, first.Path), on)
}

// A goal the ledger does not have is refused, and nothing is written.
func TestCreateRecordRefusesAGoalTheLedgerDoesNotHave(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	_, err := CreateRecord(roots, NewRecord{
		Kind: "decision", Title: "Somewhere else", Goals: []string{"ledger-sync", "logistics"},
	}, wroteAt)

	refusal := refusalOf(t, err)
	testutil.Expect(t, "the refusal", refusal.Kind, RefusalBad)
	testutil.Expect(t, "the reason names the goal", refusal.Message,
		`the goal "logistics" is not in the ledger`)
	_, statErr := os.Stat(filepath.Join(roots.Checkout, "metasystem/docs/decisions/somewhere-else.md"))
	testutil.Expect(t, "nothing was written", os.IsNotExist(statErr), true)
}

// A record about the project as a whole names no goal, and carries no Goals
// line at all: an empty key would declare nothing, and the grammar says a head
// without one is about the whole.
func TestCreateRecordMayNameNoGoalAtAll(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	written, err := CreateRecord(roots, NewRecord{Kind: "decision", Title: "About the whole"}, wroteAt)

	testutil.Require(t, "create the decision", err, nil)
	testutil.Expect(t, "the goals it names", written.Record.Goals, []string{})
	on := fileAt(t, roots, written.Path)
	testutil.Expect(t, "the head carries no Goals line", strings.Contains(on, "- Goals:"), false)

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the project refuses nothing", pane.Problems, []Problem{})
}

// The kinds this surface does not create, and the requests it cannot make a
// record out of, are refused before anything is read or written.
func TestCreateRecordRefusesWhatItCannotWrite(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	for _, refused := range []struct {
		what   string
		asked  NewRecord
		reason string
	}{
		{"doctrine is not created here",
			NewRecord{Kind: "doctrine", Title: "Events", Goals: []string{"ledger-sync"}},
			`the kind "doctrine" is not one of intent, decision, design`},
		{"an unknown kind",
			NewRecord{Kind: "note", Title: "Events", Goals: []string{"ledger-sync"}},
			`the kind "note" is not one of intent, decision, design`},
		{"no title",
			NewRecord{Kind: "decision", Title: "   ", Goals: []string{"ledger-sync"}},
			"a record needs a title"},
		{"a title that is no file name",
			NewRecord{Kind: "decision", Title: "!!!", Goals: []string{"ledger-sync"}},
			`the title "!!!" yields no file name`},
	} {
		t.Run(refused.what, func(t *testing.T) {
			_, err := CreateRecord(roots, refused.asked, wroteAt)

			refusal := refusalOf(t, err)
			testutil.Expect(t, "the refusal", refusal.Kind, RefusalBad)
			testutil.Expect(t, "the reason", refusal.Message, refused.reason)
		})
	}
}

// A status change rewrites one line. Every other byte of the file, its body
// and the rest of its head included, is what it was.
func TestSetStatusRewritesOneLineAndNothingElse(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	path := "metasystem/plans/designs/pane/reading.md"
	before := fileAt(t, roots, path)

	written, err := SetStatus(roots, "design-reading", "accepted")

	testutil.Require(t, "change the status", err, nil)
	testutil.Expect(t, "the answer", written.Record.Status, "accepted")
	testutil.Expect(t, "the path", written.Path, path)
	after := fileAt(t, roots, path)
	testutil.Expect(t, "the status line changed",
		strings.Replace(after, "- Status: accepted\n", "- Status: draft\n", 1), before)
	testutil.Expect(t, "the line count is the same",
		strings.Count(after, "\n"), strings.Count(before, "\n"))
}

// A status the grammar does not carry, and an id no record declares, are
// refused with the status each deserves.
func TestSetStatusRefusals(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/plans/designs/pane/reading.md")

	_, err := SetStatus(roots, "design-reading", "shipped")
	refusal := refusalOf(t, err)
	testutil.Expect(t, "a status the grammar does not carry", refusal.Kind, RefusalBad)
	testutil.Expect(t, "the reason a status is refused", refusal.Message,
		`the status "shipped" is not one of draft, accepted, superseded, done`)

	_, err = SetStatus(roots, "design-nothing", "accepted")
	refusal = refusalOf(t, err)
	testutil.Expect(t, "an id no record declares", refusal.Kind, RefusalAbsent)
	testutil.Expect(t, "the reason an id is refused", refusal.Message, "no record declares the id design-nothing")

	testutil.Expect(t, "nothing was written",
		fileAt(t, roots, "metasystem/plans/designs/pane/reading.md"), before)
}

// A question is one row appended to the register, open, dated today, and read
// back by the resolver as the row it is.
func TestAskQuestionAppendsARow(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/memory/questions.md")

	asked, err := AskQuestion(roots, NewQuestion{
		Question: "Where does an adopted application's doctrine live?", Goals: []string{"ledger-sync", "reading-pane"},
	}, wroteAt)

	testutil.Require(t, "ask", err, nil)
	testutil.Expect(t, "the status", asked.Question.Status, "open")
	testutil.Expect(t, "the date", asked.Question.Opened, "2026-09-22")
	testutil.Expect(t, "the goals", asked.Question.Goals, []string{"ledger-sync", "reading-pane"})
	testutil.Expect(t, "the id is minted with the register's prefix",
		strings.HasPrefix(asked.Question.ID, "Q-"), true)

	after := fileAt(t, roots, "metasystem/memory/questions.md")
	row := "| " + asked.Question.ID + " | 2026-09-22 | Where does an adopted application's doctrine live? " +
		"| ledger-sync reading-pane | open |\n"
	testutil.Expect(t, "the row is the table's last", strings.HasSuffix(after, row), true)
	testutil.Expect(t, "every other row is untouched", strings.Replace(after, row, "", 1), before)
}

// A register that is not there yet is created with the header the grammar
// names, and the resolver reads the one row back.
func TestAskQuestionCreatesTheRegister(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	testutil.Require(t, "remove the register",
		os.Remove(filepath.Join(roots.Checkout, "metasystem/memory/questions.md")), nil)

	asked, err := AskQuestion(roots, NewQuestion{Question: "Who accepts?", Goals: []string{"ledger-sync"}}, wroteAt)

	testutil.Require(t, "ask", err, nil)
	on := fileAt(t, roots, "metasystem/memory/questions.md")
	testutil.Expect(t, "the header", strings.Contains(on, "| id | opened | question | goals | status |\n"), true)
	testutil.Expect(t, "the rule", strings.Contains(on, "| --- | --- | --- | --- | --- |\n"), true)
	testutil.Expect(t, "the row", strings.HasSuffix(on,
		"| "+asked.Question.ID+" | 2026-09-22 | Who accepts? | ledger-sync | open |\n"), true)

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the register carries one row", len(pane.Questions), 1)
	testutil.Expect(t, "the project refuses nothing", pane.Problems, []Problem{})
}

// A pipe or a line break in a question would make the row something other than
// a row, so a question carrying either is refused and nothing is appended.
func TestAskQuestionRefusesWhatWouldBreakTheRow(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/memory/questions.md")

	for _, refused := range []struct{ what, question, reason string }{
		{"a pipe", "Is it a | or a cell?", "a question carries no | and no line break"},
		{"a line break", "Two\nlines", "a question carries no | and no line break"},
		{"no words", "   ", "a question needs words"},
	} {
		t.Run(refused.what, func(t *testing.T) {
			_, err := AskQuestion(roots, NewQuestion{Question: refused.question, Goals: []string{"ledger-sync"}}, wroteAt)

			refusal := refusalOf(t, err)
			testutil.Expect(t, "the refusal", refusal.Kind, RefusalBad)
			testutil.Expect(t, "the reason", refusal.Message, refused.reason)
		})
	}
	testutil.Expect(t, "nothing was appended", fileAt(t, roots, "metasystem/memory/questions.md"), before)
}

// Answering rewrites one cell of one row, and the register is otherwise the
// file it was.
func TestSetQuestionStatusRewritesOneCell(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/memory/questions.md")

	asked, err := SetQuestionStatus(roots, "Q-1", "answered: decision-one-binary")

	testutil.Require(t, "answer", err, nil)
	testutil.Expect(t, "the status", asked.Question.Status, "answered: decision-one-binary")
	testutil.Expect(t, "the question is what it was", asked.Question.Question,
		"Where does an adopted project's intent live?")
	after := fileAt(t, roots, "metasystem/memory/questions.md")
	testutil.Expect(t, "only the cell changed",
		strings.Replace(after, "| answered: decision-one-binary |\n| Q-2", "| open |\n| Q-2", 1), before)
}

// A status the register's grammar does not carry, and an id no row declares,
// are refused and change nothing.
func TestSetQuestionStatusRefusals(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := fileAt(t, roots, "metasystem/memory/questions.md")

	for _, refused := range []struct {
		what, id, status string
		kind             RefusalKind
		reason           string
	}{
		{"an answer naming nothing", "Q-1", "answered:", RefusalBad,
			`the status "answered:" is not open, answered: <reference>, or withdrawn`},
		{"a status the grammar does not carry", "Q-1", "closed", RefusalBad,
			`the status "closed" is not open, answered: <reference>, or withdrawn`},
		{"a status that would break the row", "Q-1", "answered: a | b", RefusalBad,
			`the status "answered: a | b" is not open, answered: <reference>, or withdrawn`},
		{"an id no row declares", "Q-9", "withdrawn", RefusalAbsent,
			"no row of the register declares the id Q-9"},
	} {
		t.Run(refused.what, func(t *testing.T) {
			_, err := SetQuestionStatus(roots, refused.id, refused.status)

			refusal := refusalOf(t, err)
			testutil.Expect(t, "the refusal", refusal.Kind, refused.kind)
			testutil.Expect(t, "the reason", refusal.Message, refused.reason)
		})
	}
	testutil.Expect(t, "nothing was rewritten", fileAt(t, roots, "metasystem/memory/questions.md"), before)
}

// The page, byte for byte.
//
// It is pinned here because the sheet composes the same bytes in the browser
// before it asks a human to confirm them — src/project/writing.test.ts pins
// the same page, with the id line standing in for the one the server mints.
// Two sides that drifted would be a sheet promising a file nobody wrote.
func TestThePageIsWrittenByteForByte(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	written, err := CreateRecord(roots, NewRecord{
		Kind: "decision", Title: "One binary", Goals: []string{"ledger-sync"},
		Cites: []string{"design-ledger"}, Affects: []string{"design-reading"},
	}, wroteAt)

	testutil.Require(t, "create", err, nil)
	testutil.Expect(t, "the page", fileAt(t, roots, written.Path), strings.Join([]string{
		"# One binary",
		"",
		"- Kind: decision",
		"- Id: " + written.Record.ID,
		"- Status: draft",
		"- Goals: ledger-sync",
		"- Cites: design-ledger",
		"- Affects: design-reading",
		"",
		"## Context",
		"",
		"## Decision",
		"",
		"## Consequences",
		"",
	}, "\n"))
}

// The state root's home is the one a record is written in, never the
// checkout's second design home, which holds the kit's own designs beside the
// installation that reads them.
func TestDesignsAreWrittenInTheStateRootsHome(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	written, err := CreateRecord(roots, NewRecord{
		Kind: "design", Title: "A new design", Goals: []string{"ledger-sync"},
	}, wroteAt)

	testutil.Require(t, "create the design", err, nil)
	testutil.Expect(t, "the home", written.Record.Home, "metasystem/plans/designs")
	_, statErr := os.Stat(filepath.Join(roots.Checkout, "plans/designs/a-new-design.md"))
	testutil.Expect(t, "the checkout's second home is untouched", os.IsNotExist(statErr), true)
}

// A project declares the homes it uses, so a home that is not there yet is
// made rather than refused: the first decision a project records is the one
// that creates docs/decisions.
func TestAHomeThatIsNotThereYetIsMade(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	seed(t, roots)
	testutil.Require(t, "remove the decisions home",
		os.RemoveAll(filepath.Join(roots.Checkout, "docs", "decisions")), nil)

	written, err := CreateRecord(roots, NewRecord{
		Kind: "decision", Title: "The first decision", Goals: []string{"ledger-sync"},
	}, wroteAt)

	testutil.Require(t, "create the first decision", err, nil)
	testutil.Expect(t, "the path", written.Path, "docs/decisions/the-first-decision.md")
	testutil.Expect(t, "the head is a draft",
		strings.Contains(fileAt(t, roots, written.Path), "- Status: draft\n"), true)
}

// An adopted application has one design home, and it is its own.
func TestAnAdoptedProjectWritesInItsOwnHomes(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	seed(t, roots)

	written, err := CreateRecord(roots, NewRecord{
		Kind: "decision", Title: "One binary, please", Goals: []string{"reading-pane"},
	}, wroteAt)

	testutil.Require(t, "create the decision", err, nil)
	testutil.Expect(t, "the path", written.Path, "docs/decisions/one-binary-please.md")
	testutil.Expect(t, "the head names the goal",
		strings.Contains(fileAt(t, roots, written.Path), "- Goals: reading-pane\n"), true)
}
