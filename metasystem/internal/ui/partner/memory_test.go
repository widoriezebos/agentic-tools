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

// The kinds come in the order the project is read in, and each row says how to
// fetch the thing it names.
func TestTheIndexListsEachKindInReadingOrder(t *testing.T) {
	t.Parallel()
	index := Memory(Facts{Project: func() (project.Pane, error) { return indexedPane(), nil }}, composedAt)
	block := index.Block

	for at, heading := range []string{
		"Intent, in reading order",
		"Doctrine, in reading order",
		"Decisions, newest first",
		"Designs, newest first",
		"Open questions, newest first",
	} {
		testutil.Expect(t, "heading "+strconv.Itoa(at)+" is there", strings.Contains(block, heading), true)
	}
	testutil.Expect(t, "in that order",
		strings.Index(block, "Intent, in reading order") < strings.Index(block, "Doctrine, in reading order"), true)
	testutil.Expect(t, "and the records follow the books",
		strings.Index(block, "Doctrine, in reading order") < strings.Index(block, "Decisions, newest first"), true)

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
func TestTheIndexDropsWholeKindsAndNamesThem(t *testing.T) {
	t.Parallel()
	pane := indexedPane()
	for at := 0; at < 400; at++ {
		pane.Records = append(pane.Records, project.Record{
			Kind: "design", ID: "01BULK" + strconv.Itoa(at), Title: "A design",
			Status: "accepted", ChangedAt: "2026-09-01T00:00:00Z",
			Summary: strings.Repeat("This design says something about the interface. ", 4),
		})
	}
	index := Memory(Facts{Project: func() (project.Pane, error) { return pane, nil }}, composedAt)

	testutil.Expect(t, "within the character bound", len(index.Block) <= maxIndexCharacters, true)
	testutil.Expect(t, "and within the line bound",
		len(strings.Split(strings.TrimRight(index.Block, "\n"), "\n")) <= maxIndexLines, true)
	testutil.Expect(t, "the kinds that were left out are named",
		strings.Contains(index.Block, "whole kinds were left out of it rather than cut: Designs, newest first (402)"), true)
	testutil.Expect(t, "and so are the ones behind them",
		strings.Contains(index.Block, "Open questions, newest first (2)"), true)
	testutil.Expect(t, "with the tool that lists them",
		strings.Contains(index.Block, "The records tool lists them."), true)
	// The kinds that did fit are still whole: trimming from the end is what
	// keeps a kept kind trustworthy.
	testutil.Expect(t, "the decisions are still there",
		strings.Contains(index.Block, "Decisions, newest first"), true)
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

// indexedPane is a project with one of each kind, two designs of different
// ages, and a doctrine chapter that is a bound document rather than a record.
func indexedPane() project.Pane {
	return project.Pane{
		ReadAt: "2026-09-23T11:30:00Z",
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
			{Kind: "design", ID: "01DESIGN-OLD", Status: "done", Title: "An older design",
				Path: "plans/designs/old.md", ChangedAt: "2026-09-01T00:00:00Z"},
			{Kind: "design", ID: "01DESIGN-NEW", Status: "accepted", Title: "A newer design",
				Path: "plans/designs/new.md", ChangedAt: "2026-09-22T00:00:00Z"},
		},
		Questions: []project.Question{
			{ID: "Q-4", Opened: "2026-09-22", Question: "Which runtime answers?", Status: "open"},
			{ID: "Q-1", Opened: "2026-08-01", Question: "Where do rulings live?", Status: "answered"},
		},
	}
}
