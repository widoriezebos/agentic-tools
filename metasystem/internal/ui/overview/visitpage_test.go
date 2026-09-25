package overview

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A page that is not the landing page keeps its own entry in the same file.
//
// This is the whole reason the entry is its own. A human who reads Decisions
// every morning and Overview once a week has two boundaries; one marker for
// both would mean that this morning's Decisions read told Overview it had been
// seen, and the week they missed there would never be shown to them.

func TestAPagesVisitAdvancesItsOwnEntryAndNotTheLandingPages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// The landing page is read twice, an hour apart, which establishes its
	// own boundary at the first of the two reads.
	_, _, err := Visit(root, "Wido", noon.Add(-2*time.Hour))
	testutil.Require(t, "the first landing read", err, nil)
	_, first, err := Visit(root, "Wido", noon.Add(-time.Hour))
	testutil.Require(t, "the second landing read", err, nil)
	testutil.Expect(t, "the landing page is on its second visit", first, false)

	// Then Decisions is read, twice, an hour apart: its own first visit, and
	// then its second.
	since, first, err := VisitPage(root, PageDecisions, "Wido", noon.Add(-time.Hour))
	testutil.Require(t, "the first decisions read", err, nil)
	testutil.Expect(t, "the page's first visit looks back a day", since, noon.Add(-25*time.Hour))
	testutil.Expect(t, "and says so", first, true)

	since, first, err = VisitPage(root, PageDecisions, "Wido", noon)
	testutil.Require(t, "the second decisions read", err, nil)
	testutil.Expect(t, "the page compares against its own last read", since, noon.Add(-time.Hour))
	testutil.Expect(t, "which is no longer a first visit", first, false)

	// And the landing page's row is untouched by all of it: it still ends
	// where its own last read ended, and its boundary is still its own.
	held := readFile(t, root)
	testutil.Expect(t, "the landing page's last read", held.Humans["Wido"].Seen,
		noon.Add(-time.Hour).UTC().Format(time.RFC3339))
	testutil.Expect(t, "the landing page's boundary", held.Humans["Wido"].Previous,
		noon.Add(-2*time.Hour).UTC().Format(time.RFC3339))
	testutil.Expect(t, "the page's own entry is beside it", held.Pages[PageDecisions]["Wido"].Seen,
		noon.UTC().Format(time.RFC3339))
}

// The key is the page AND the human, so neither a second human on one page nor
// a second page under one human reads the other's window.
func TestEachPageAndHumanKeepsItsOwnEntry(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, _, err := VisitPage(root, PageDecisions, "Wido", noon.Add(-2*time.Hour))
	testutil.Require(t, "Wido's first read", err, nil)
	_, _, err = VisitPage(root, PageDecisions, "Wido", noon)
	testutil.Require(t, "Wido's second read", err, nil)

	// A second human has never been here, whatever Wido has read.
	since, first, err := VisitPage(root, PageDecisions, "Ada", noon)
	testutil.Require(t, "Ada's first read", err, nil)
	testutil.Expect(t, "Ada's window is a first visit's", since, noon.Add(-24*time.Hour))
	testutil.Expect(t, "and says so", first, true)

	// And so has Wido on a page nobody has opened.
	since, first, err = VisitPage(root, "somewhere-else", "Wido", noon)
	testutil.Require(t, "the other page's first read", err, nil)
	testutil.Expect(t, "its window is a first visit's", since, noon.Add(-24*time.Hour))
	testutil.Expect(t, "and the other page says so too", first, true)

	held := readFile(t, root)
	testutil.Expect(t, "Wido's decisions boundary is still their own",
		held.Pages[PageDecisions]["Wido"].Previous, noon.Add(-2*time.Hour).UTC().Format(time.RFC3339))
}

// A file written before pages kept entries reads back as a page nobody has
// visited, rather than as a file this build refuses: the addition did not
// change the schema, and it must not change what an older file means.
func TestAFileWrittenBeforePagesKeptEntriesIsAFirstVisitForThePage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.Require(t, "making the directory",
		os.MkdirAll(filepath.Dir(VisitsPath(root)), 0o755), nil)
	testutil.Require(t, "planting the row", os.WriteFile(VisitsPath(root),
		[]byte(`{"schemaVersion":1,"humans":{"Wido":{"began":"2026-09-23T10:00:00Z",`+
			`"seen":"2026-09-23T10:00:00Z","previous":"2026-09-22T10:00:00Z"}}}`),
		0o644), nil)

	since, first, err := VisitPage(root, PageDecisions, "Wido", noon)

	testutil.Require(t, "recording the visit", err, nil)
	testutil.Expect(t, "the page's window is a first visit's", since, noon.Add(-24*time.Hour))
	testutil.Expect(t, "and says so", first, true)
	// The landing page's row survived the rewrite untouched.
	held := readFile(t, root)
	testutil.Expect(t, "the landing page's boundary", held.Humans["Wido"].Previous, "2026-09-22T10:00:00Z")
}
