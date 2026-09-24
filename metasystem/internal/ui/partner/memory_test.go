package partner

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The index is a map with a date on it, and it says so before it says anything
// else. Everything below rests on that sentence: a reader that takes the index
// for evidence will answer "there are no designs about X" from a block that was
// trimmed, or "that question is open" from a block that is an hour old.
func TestTheIndexSaysItIsAMapAndNotEvidence(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)

	testutil.Expect(t, "it is dated", index.At, composedAt)
	testutil.Expect(t, "the heading names the moment",
		strings.Contains(index.Block, "The project's memory, indexed at 2026-09-23T11:30:00Z"), true)
	testutil.Expect(t, "and it says what it is",
		strings.Contains(index.Block, "This index is a map from 2026-09-23T11:30:00Z; "+
			"it is not evidence that something exists, is absent, or has the status shown. "+
			"For any of those, use the tools."), true)
}

// The groups come in the order the project is read in: the books, then the
// project's own records, then the goal-scoped ones. Each row says how to fetch
// the thing it names.
func TestTheIndexListsEachGroupInReadingOrder(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)
	block := index.Block

	headings := []string{
		"Intent, in reading order",
		"Doctrine, in reading order",
		"Decisions about the project as a whole, newest first",
		"Designs about the project as a whole, newest first",
		"Open questions about the project as a whole, newest first",
		"Under the goal g1-s9 · The document reader, newest first",
		"Under the goal g1-s13, newest first",
	}
	for at, heading := range headings {
		testutil.Expect(t, "heading "+strconv.Itoa(at)+" is there", strings.Contains(block, heading), true)
	}
	// In that order, all the way down: the books, the project's own, then one
	// group per goal in the ledger's own order.
	for at := 1; at < len(headings); at++ {
		testutil.Expect(t, "heading "+strconv.Itoa(at)+" follows the one before it",
			strings.Index(block, headings[at-1]) < strings.Index(block, headings[at]), true)
	}

	// A record by its id, a bound chapter by its path, a question by its row
	// id. Fetching them is three different tools, and a reference that lied
	// about which would send a reader at the wrong one.
	testutil.Expect(t, "a record is named by its id",
		strings.Contains(block, "- 01INTENT · The intent book · accepted"), true)
	testutil.Expect(t, "a bound chapter is named by its path",
		strings.Contains(block, "- doc:docs/doctrine/concepts.md · The metasystem in concepts"), true)
	testutil.Expect(t, "a question is named by its row id",
		strings.Contains(block, "- Q-4 · Which runtime answers?"), true)
	testutil.Expect(t, "and a record's own first words are carried",
		strings.Contains(block, "— Why this workspace exists."), true)
}

// The project's own records are the ones whose head names no goal, and they
// come first because they are the small set that shapes everything: a Partner
// asked what this project has decided is asked about these.
func TestTheIndexPutsTheProjectsOwnRecordsFirst(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)
	block := index.Block

	own := strings.Index(block, "Decisions about the project as a whole, newest first")
	under := strings.Index(block, "Under the goal g1-s9")
	testutil.Expect(t, "the project's own decisions come before any goal's", own < under, true)

	// The one decision that names no goal is in the project's own group, and
	// the one that names a goal is not.
	group := block[own:under]
	testutil.Expect(t, "the project-wide decision is in it", strings.Contains(group, "01DECISION"), true)
	testutil.Expect(t, "and the goal-scoped one is not", strings.Contains(group, "01DECISION-SCOPED"), false)
	testutil.Expect(t, "which is under its own goal",
		strings.Contains(block[under:], "01DECISION-SCOPED"), true)
}

// A record naming two goals stands under each of them. That is the point of
// the grouping: a Partner asked about one goal is asked about every record
// that says it is about that goal, whatever else those records also say.
func TestTheIndexPutsARecordUnderEachGoalItNames(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)
	testutil.Expect(t, "it is listed twice",
		strings.Count(index.Block, "01DESIGN-BOTH"), 2)

	// A goal nothing is about has no group at all: an empty one would cost a
	// line to say nothing.
	testutil.Expect(t, "a goal nothing is about has no group",
		strings.Contains(index.Block, "Under the goal g1-s99"), false)

	// A goal named by a record that the ledger does not carry keeps its place
	// at the end: dropping it would drop the record that names it.
	testutil.Expect(t, "a goal the ledger does not carry still has its group",
		strings.Contains(index.Block, "Under the goal no-such-goal, newest first"), true)
	testutil.Expect(t, "after the ones the ledger does",
		strings.Index(index.Block, "Under the goal g1-s13") <
			strings.Index(index.Block, "Under the goal no-such-goal"), true)
}

// Newest first is what the records themselves say: when each file was last
// written, which is the only age they carry.
func TestTheIndexPutsTheNewestRecordsFirst(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)
	newer := strings.Index(index.Block, "01DESIGN-NEW")
	older := strings.Index(index.Block, "01DESIGN-OLD")
	testutil.Expect(t, "both are listed", newer > 0 && older > 0, true)
	testutil.Expect(t, "the newer one first", newer < older, true)
}

// A reader handed a list silently cut in the middle cannot tell a short
// project from a trimmed index. So what does not fit is dropped a whole kind
// at a time, and the kinds dropped are named with their counts.
func TestTheIndexDropsWholeGroupsAndNamesThem(t *testing.T) {
	t.Parallel()
	pane := indexedPane()
	// Two goals with far too much under them, so what is dropped is dropped a
	// goal at a time and the goals left out are the ones named.
	for at := 0; at < 200; at++ {
		pane.Records = append(pane.Records, project.Record{
			Kind: "design", ID: "01BULK-A" + strconv.Itoa(at), Title: "A design", Goals: []string{"g1-s9"},
			Status: "accepted", ChangedAt: "2026-09-01T00:00:00Z",
			Summary: strings.Repeat("This design says something about the interface. ", 4),
		})
		pane.Records = append(pane.Records, project.Record{
			Kind: "design", ID: "01BULK-B" + strconv.Itoa(at), Title: "A design", Goals: []string{"g1-s13"},
			Status: "accepted", ChangedAt: "2026-09-01T00:00:00Z",
			Summary: strings.Repeat("This design says something about the interface. ", 4),
		})
	}
	index := Memory(Facts{Project: func() (project.Pane, error) { return pane, nil }}, composedAt)

	testutil.Expect(t, "within the character bound", len(index.Block) <= maxIndexCharacters, true)
	testutil.Expect(t, "and within the line bound",
		len(strings.Split(strings.TrimRight(index.Block, "\n"), "\n")) <= maxIndexLines, true)
	testutil.Expect(t, "the goals that were left out are named, with their counts",
		strings.Contains(index.Block, "whole groups were left out of it rather than cut: "+
			"Under the goal g1-s9 · The document reader, newest first (203)"), true)
	testutil.Expect(t, "and so are the ones behind them",
		strings.Contains(index.Block, "Under the goal g1-s13, newest first (201)"), true)
	testutil.Expect(t, "down to the last of them",
		strings.Contains(index.Block, "Under the goal no-such-goal, newest first (1)"), true)
	testutil.Expect(t, "with the tool that lists them",
		strings.Contains(index.Block, "The records tool lists them."), true)
	// The groups that did fit are still whole, and the project's own records
	// are never the ones dropped: trimming from the end is what keeps a kept
	// group trustworthy, and the order is what keeps the important half.
	testutil.Expect(t, "the project's own decisions are still there",
		strings.Contains(index.Block, "Decisions about the project as a whole, newest first"), true)
	testutil.Expect(t, "and so are the project's own open questions",
		strings.Contains(index.Block, "Open questions about the project as a whole, newest first"), true)
}

// A read that failed is said, not skipped. An index that quietly listed
// nothing would be read as a project with nothing in it.
func TestTheIndexSaysWhenItCouldNotRead(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) {
		return project.Pane{}, errors.New("the questions register is malformed")
	}}, composedAt)
	testutil.Expect(t, "it says the read failed",
		strings.Contains(index.Block, "The project's records could not be read, so this index is empty rather than short: "+
			"the questions register is malformed"), true)

	absent := Memory(Facts{}, composedAt)
	testutil.Expect(t, "and says when this build has no reader at all",
		strings.Contains(absent.Block, "This build has no reader for the project's records"), true)
}

// A kind the project happens to have none of says so, rather than being left
// out and read as "the index did not reach it".
func TestTheIndexSaysWhenAKindIsEmpty(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return project.Pane{}, nil }}, composedAt)
	// The five that always stand — the two books and the project's own three
	// — say so when they are empty. A goal's group is not one of the five: a
	// project with no goals has no groups, and an empty group would cost a
	// line to say nothing.
	testutil.Expect(t, "each empty kind says so",
		strings.Count(index.Block, "This reading found none."), 5)
}

// A later prompt of the same session is given the moment, not the map.
func TestALaterPromptNamesTheIndexsOwnMoment(t *testing.T) {
	t.Parallel()
	line := Returning(composedAt)
	testutil.Expect(t, "it names when the index was read",
		strings.Contains(line, "read at 2026-09-23T11:30:00Z"), true)
	testutil.Expect(t, "and says it has not been read again",
		strings.Contains(line, "It has not been read again since."), true)
	testutil.Expect(t, "and sends anything current to the tools",
		strings.Contains(line, "is a tool call, not a line of that index"), true)
}

/* ---------------------------------------------------------------- reading -- */

// indexedPane is a project with records on both sides of the line: one of each
// kind about the project as a whole, two designs of different ages under one
// goal, a design under two goals, a decision under a goal, a doctrine chapter
// that is a bound document rather than a record, a goal nothing is about, and
// a design naming a goal the ledger does not carry.
func indexedPane() project.Pane {
	return project.Pane{
		ReadAt: "2026-09-23T11:30:00Z",
		Goals: []project.Goal{
			{ID: "g1-s9", Title: "The document reader", State: "claimed"},
			{ID: "g1-s13", Title: "g1-s13", State: "queued"},
			{ID: "g1-s99", Title: "A goal nothing is about", State: "queued"},
		},
		Intent: project.Book{
			Index: &project.Record{
				Kind: "intent", ID: "01INTENT", Status: "accepted", Title: "The intent book",
				Path: "docs/intent/index.md", Summary: "Why this workspace exists.",
			},
			Chapters: []project.Chapter{
				{ID: "01INTENT-WHY", Title: "Why we build this", Summary: "The standing answer."},
			},
		},
		Doctrine: project.Book{
			Index: &project.Record{
				Kind: "doctrine", ID: "01DOCTRINE", Status: "accepted", Title: "The doctrine book",
				Path: "docs/doctrine/index.md", Summary: "The rules every design is held to.",
			},
			Chapters: []project.Chapter{
				{Path: "docs/doctrine/concepts.md", Title: "The metasystem in concepts",
					Summary: "How the parts compose."},
			},
		},
		Records: []project.Record{
			{Kind: "decision", ID: "01DECISION", Status: "accepted", Title: "One ACP client",
				Path: "docs/decisions/acp.md", ChangedAt: "2026-09-20T00:00:00Z",
				Summary: "The seam is the Agent Client Protocol."},
			{Kind: "decision", ID: "01DECISION-SCOPED", Status: "draft", Title: "The reader's rails",
				Path: "docs/decisions/rails.md", ChangedAt: "2026-09-21T00:00:00Z",
				Goals: []string{"g1-s9"}},
			{Kind: "design", ID: "01DESIGN-OLD", Status: "done", Title: "An older design",
				Path: "plans/designs/old.md", ChangedAt: "2026-09-01T00:00:00Z"},
			{Kind: "design", ID: "01DESIGN-NEW", Status: "accepted", Title: "A newer design",
				Path: "plans/designs/new.md", ChangedAt: "2026-09-22T00:00:00Z"},
			{Kind: "design", ID: "01DESIGN-BOTH", Status: "accepted", Title: "A design about two goals",
				Path: "plans/designs/both.md", ChangedAt: "2026-09-19T00:00:00Z",
				Goals: []string{"g1-s9", "g1-s13"}},
			{Kind: "design", ID: "01DESIGN-NOWHERE", Status: "draft", Title: "A design naming a goal nobody planted",
				Path: "plans/designs/nowhere.md", ChangedAt: "2026-09-18T00:00:00Z",
				Goals: []string{"no-such-goal"}},
		},
		Questions: []project.Question{
			{ID: "Q-4", Opened: "2026-09-22", Question: "Which runtime answers?", Status: "open"},
			{ID: "Q-1", Opened: "2026-08-01", Question: "Where do rulings live?", Status: "answered"},
			{ID: "Q-7", Opened: "2026-09-23", Question: "How wide is the rail?", Status: "open",
				Goals: []string{"g1-s9"}},
		},
	}
}
