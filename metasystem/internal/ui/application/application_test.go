package application

import (
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/knownissues"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

// The clock every test below composes against, and the window "new" is decided
// against. Both are injected: nothing here reads a wall clock.
var (
	readAt = time.Date(2026, 9, 25, 11, 0, 0, 0, time.UTC)
	window = time.Date(2026, 9, 24, 11, 0, 0, 0, time.UTC)
)

func done(id, intent, concluded, doneAt string) backlog.Row {
	row := backlog.Row{
		Where: "archived", Lane: backlog.LaneDone, State: "done",
		Intent: intent, Concluded: concluded, DoneAt: doneAt,
	}
	row.ID = id
	return row
}

// The closed rows a board projection hands over: six concluded goals at
// different ages, one of them undated, and one abandoned goal, which is not
// something that concluded.
func closedRows() []backlog.Row {
	rows := []backlog.Row{
		done("g1-s40", "The queue row opens in place", "landed in 7b1c9de; the row opens on the board", "2026-09-25T09:00:00Z"),
		done("g1-s39", "The header counts what is asked", "landed in 2ad91f0", "2026-09-24T09:00:00Z"),
		done("g1-s38", "The label chips are drawn from the rows", "landed in 9cc2e31", "2026-09-18T09:00:00Z"),
		done("g1-s30", "The fleet page reads a seat's chain", "landed in 44de0a1", "2026-08-30T09:00:00Z"),
		done("g1-s29", "A second bundler beside the first", "Obsolete: self-declared duplicate holding no work", ""),
		done("g1-s31", "The stream reconnects by itself", "landed in 1f0aa54", "2026-09-25T09:00:00Z"),
	}
	dropped := backlog.Row{Where: "archived", Lane: backlog.LaneAbandoned, State: "abandoned",
		Intent: "A second bundler", Concluded: "overtaken by g1-s8", DoneAt: "2026-09-25T10:00:00Z"}
	dropped.ID = "g1-s7"
	rows = append(rows, dropped)
	// Labels and an arc, so the row carries what an open row shows.
	rows[0].Labels = []string{"browser-interface", "robustness"}
	rows[0].Arc = "covenant-harvest"
	return rows
}

func composed(over func(*Inputs)) Page {
	in := Inputs{
		Subject: "MetaSystem", Mode: workspace.ModeSelfHosted,
		Engine: &Engine{Build: "5b9d958", Generation: 4, PublishedAt: "2026-09-25T10:45:00Z"},
		Closed: closedRows(),
		// The self-hosted layout: the repository's own README at the checkout
		// root, and the MetaSystem's documents under the installation. Neither
		// root carries a concepts document, so the rule that only a listed
		// path is linked still bites.
		Documents: []project.File{
			{Path: "README.md", Title: "Agentic tools"},
			{Path: "metasystem/README.md", Title: "MetaSystem"},
			{Path: "metasystem/docs/glossary.md", Title: "Glossary"},
			{Path: "metasystem/docs/journey.md", Title: "The journey"},
		},
		Installation: "metasystem",
		RegisterPath: "metasystem/memory/known-issues.md",
		Since:        window,
	}
	if over != nil {
		over(&in)
	}
	return Compose(in, readAt)
}

func landedIDs(page Page) []string {
	ids := []string{}
	for _, row := range page.Landed {
		ids = append(ids, row.ID)
	}
	return ids
}

// Newest first, the undated last, and the abandoned goal is not here at all: a
// goal this project decided not to do is not something the application has.
func TestComposeOrdersTheConcludedGoalsNewestFirstWithTheUndatedLast(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the order", landedIDs(page),
		[]string{"g1-s31", "g1-s40", "g1-s39", "g1-s38", "g1-s30", "g1-s29"})
}

// Two conclusions written in the same second keep one order between reads, so
// a page that is read twice does not shuffle its own rows.
func TestComposeBreaksATieByIDSoTheOrderIsStable(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the two written at 09:00 on the 25th",
		[]string{page.Landed[0].ID, page.Landed[1].ID}, []string{"g1-s31", "g1-s40"})
}

func TestComposeCarriesTheLedgersOwnWordsOnARow(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the newest but one row", page.Landed[1], Landed{
		ID: "g1-s40", Intent: "The queue row opens in place",
		Concluded: "landed in 7b1c9de; the row opens on the board",
		DoneAt:    "2026-09-25T09:00:00Z",
		Labels:    []string{"browser-interface", "robustness"},
		Arc:       "covenant-harvest", New: true,
	})
}

// The header's line: the whole history, this month of it, and what landed
// since this human last read THIS page.
func TestComposeCountsTheHistoryTheMonthAndTheWindow(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	// Two are new: the window opened at 11:00 on the 24th, and the row
	// concluded at 09:00 that same day is on the window's own day but before
	// it, which is an instant compared as an instant rather than as a day.
	testutil.Expect(t, "the counts", page.Counts, Counts{Landed: 6, ThisMonth: 4, New: 2})
}

// A conclusion nothing dated is never new: the record does not say when it was
// written, and a page that guessed would be marking rows new on no evidence.
func TestComposeNeverMarksAnUndatedConclusionNew(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the undated row", page.Landed[5].New, false)
}

// And a page composed over no window at all has nothing new on it, rather than
// every row the ledger ever recorded.
func TestComposeMarksNothingNewWithoutAWindow(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) { in.Since = time.Time{} })
	testutil.Expect(t, "how many are new", page.Counts.New, 0)
	testutil.Expect(t, "the window it says it used", page.Visit, Visit{Since: "", First: false})
}

func TestComposeSaysWhichWindowItDecidedNewAgainst(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) { in.First = true })
	testutil.Expect(t, "the window", page.Visit, Visit{Since: "2026-09-24T11:00:00Z", First: true})
}

func TestComposeNamesTheSubjectTheModeAndTheEngineItWasGiven(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the subject", page.Subject, "MetaSystem")
	testutil.Expect(t, "the mode", page.Mode, "self-hosted")
	testutil.Expect(t, "the engine", page.Engine,
		&Engine{Build: "5b9d958", Generation: 4, PublishedAt: "2026-09-25T10:45:00Z"})
	testutil.Expect(t, "when it was read", page.ReadAt, "2026-09-25T11:00:00Z")
	testutil.Expect(t, "the schema", page.SchemaVersion, SchemaVersion)
}

// A seat that has published no presence has no build to name, and the page
// carries the absence rather than an empty build a human would read as one.
func TestComposeCarriesNoEngineWhereThisSeatHasPublishedNone(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) { in.Engine = nil })
	if page.Engine != nil {
		t.Fatalf("a seat with no presence record has no engine build: %+v", page.Engine)
	}
}

func TestComposeCarriesTheRegisterAsItWasRead(t *testing.T) {
	t.Parallel()
	open := knownissues.Row{ID: "KI-19", Status: "OPEN", Open: true}
	closed := knownissues.Row{ID: "KI-1", Status: "FIXED 2026-08-06"}
	page := composed(func(in *Inputs) {
		in.Register = knownissues.Register{
			Columns:   []string{"Id", "Date", "Issue", "Consequence", "Reopen when", "Status"},
			Open:      []knownissues.Row{open},
			Concluded: []knownissues.Row{closed},
			Unread:    5,
			Defects:   []string{"row=19: wrong column count: got 3, want 6"},
		}
	})
	testutil.Expect(t, "the problems", page.Problems, Problems{
		Columns:   []string{"Id", "Date", "Issue", "Consequence", "Reopen when", "Status"},
		Open:      []knownissues.Row{open},
		Concluded: []knownissues.Row{closed},
		Unread:    5,
		Defects:   []string{"row=19: wrong column count: got 3, want 6"},
		Register:  "metasystem/memory/known-issues.md",
	})
}

// Every list is written whether the register had anything to say or not, so a
// reader never has to tell an absent list from an empty one.
func TestComposeWritesEveryListOfAnEmptyRegister(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) { in.RegisterPath = "" })
	testutil.Expect(t, "the problems", page.Problems, Problems{
		Columns: []string{}, Open: []knownissues.Row{}, Concluded: []knownissues.Row{},
		Unread: 0, Defects: []string{}, Register: "memory/known-issues.md",
	})
}

// The documents this page links, in its own order, and only the ones the
// Project reader lists: an application with no concepts document gets no link
// to one.
//
// Self-hosted, the subject is the MetaSystem, and its README, concepts and
// glossary are the installation's. The checkout's own root README is a
// document about the repository that carries the installation, and it is not
// linked here.
func TestComposeLinksTheInstallationsDocumentsWhereTheWorkspaceIsSelfHosted(t *testing.T) {
	t.Parallel()
	page := composed(nil)
	testutil.Expect(t, "the links", page.Docs, []Document{
		{Title: "README", Path: "metasystem/README.md"},
		{Title: "Glossary", Path: "metasystem/docs/glossary.md"},
	})
}

// Adopted, the subject is the application the checkout holds, so its documents
// are the checkout root's. The kit's own glossary under the installation is
// the machinery's, and this page is not about the machinery.
func TestComposeLinksTheApplicationRootsDocumentsWhereTheWorkspaceIsAdopted(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) {
		in.Mode = workspace.ModeAdopted
		in.Documents = []project.File{
			{Path: "README.md", Title: "The application"},
			{Path: "docs/concepts.md", Title: "Concepts"},
			{Path: "metasystem/docs/glossary.md", Title: "Glossary"},
		}
	})
	testutil.Expect(t, "the links", page.Docs, []Document{
		{Title: "README", Path: "README.md"},
		{Title: "Concepts", Path: "docs/concepts.md"},
	})
}

// A self-hosted layout whose checkout and installation are one directory — the
// kit's own, and the walkthrough's — links the root paths, because that is
// where the installation is.
func TestComposeLinksTheRootPathsWhereTheInstallationIsTheCheckout(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) {
		in.Installation = "."
		in.Documents = []project.File{{Path: "README.md", Title: "MetaSystem"}}
	})
	testutil.Expect(t, "the links", page.Docs, []Document{{Title: "README", Path: "README.md"}})
}

func TestComposeLinksNothingWhereTheCheckoutHasNoneOfThem(t *testing.T) {
	t.Parallel()
	page := composed(func(in *Inputs) { in.Documents = nil })
	testutil.Expect(t, "the links", page.Docs, []Document{})
}

// The month is the observing clock's own UTC month, because the dates are UTC
// instants: a boundary read in another zone would move rows between two
// answers about the same ledger.
func TestComposeCountsTheMonthAgainstTheObservingClock(t *testing.T) {
	t.Parallel()
	in := Inputs{Closed: closedRows(), Since: window}
	october := Compose(in, time.Date(2026, 10, 1, 0, 30, 0, 0, time.UTC))
	testutil.Expect(t, "how many concluded in October", october.Counts.ThisMonth, 0)
}
