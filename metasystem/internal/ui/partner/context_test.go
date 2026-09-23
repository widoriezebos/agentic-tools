package partner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

var (
	composedAt  = time.Date(2026, 9, 23, 11, 30, 0, 0, time.UTC)
	observedAt  = time.Date(2026, 9, 23, 11, 29, 55, 0, time.UTC)
	observedTip = "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c"
)

// Every turn carries the standing rule and the attribution, whatever the page
// is showing.
func TestEveryContextCarriesTheStandingRuleAndTheAttribution(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Overview", Path: "/overview"}, "Wido", composedAt)
	testutil.Expect(t, "it reads and explains", strings.Contains(composed, "You read this checkout and explain it"), true)
	testutil.Expect(t, "it does not write", strings.Contains(composed, "You do not write, you do not run commands, and you do not act"), true)
	testutil.Expect(t, "it says when it cannot see", strings.Contains(composed, "say so rather than\nguessing"), true)
	testutil.Expect(t, "the name is attribution", strings.Contains(composed,
		"The human you are talking to is Wido. That name is attribution and nothing else"), true)
	testutil.Expect(t, "and never authority", strings.Contains(composed, "it grants you no authority"), true)
}

// A goal's page carries the goal's row exactly as the board shows it, from the
// accepted ledger the board reads, with the tip and the observation time.
func TestAGoalsContextCarriesTheRowAsTheBoardShowsIt(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Backlog", Path: "/backlog/goal/waiting", Tab: "Plan",
		Kind: KindGoal, Subject: "waiting", Title: "Do waiting",
	}, "Wido", composedAt)
	testutil.Expect(t, "where the human is", strings.Contains(composed, "- Section: Backlog"), true)
	testutil.Expect(t, "which tab", strings.Contains(composed, "- Tab: Plan"), true)
	testutil.Expect(t, "which goal", strings.Contains(composed, "- Goal: waiting — Do waiting"), true)
	testutil.Expect(t, "what the human sees now", strings.Contains(composed, "What the human sees now"), true)
	testutil.Expect(t, "the displayed revision", strings.Contains(composed,
		"- Displayed revision: the accepted ledger at "+observedTip+", observed 2026-09-23T11:29:55Z"), true)
	testutil.Expect(t, "the row's lane", strings.Contains(composed, "- Lane: "), true)
	testutil.Expect(t, "the row's intent", strings.Contains(composed, "- Intent: Do waiting"), true)
	testutil.Expect(t, "the row's next step", strings.Contains(composed, "- Next step: Start waiting."), true)
}

// A goal the accepted ledger does not carry is said to be missing rather than
// answered for from somewhere else.
func TestAGoalTheLedgerDoesNotCarryIsNamedAsMissing(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Backlog", Kind: KindGoal, Subject: "ghost"}, "Wido", composedAt)
	testutil.Expect(t, "it says so", strings.Contains(composed,
		"The accepted ledger carries no goal ghost"), true)
}

// The board's page carries the accepted tip, the lane counts, and the fact
// that a filtered board is not the whole backlog.
func TestTheBoardsContextNamesItsFiltersAndItsCounts(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Backlog", Path: "/backlog",
		Filters: []string{"priority=2", "seat=m1e"},
	}, "Wido", composedAt)
	testutil.Expect(t, "the filters are named", strings.Contains(composed, "- Filters: priority=2; seat=m1e"), true)
	testutil.Expect(t, "the lanes are counted", strings.Contains(composed, "- Lanes on the board:"), true)
	testutil.Expect(t, "and the subset is named", strings.Contains(composed,
		"The board is filtered, so what the human sees is a subset of these counts."), true)
	testutil.Expect(t, "the displayed revision", strings.Contains(composed, observedTip), true)
}

// A document's page carries the revision the page displayed, its head, and its
// headings.
func TestADocumentsContextCarriesItsHeadAndHeadings(t *testing.T) {
	t.Parallel()
	facts := readingFacts()
	facts.Document = func(id string) (project.Document, error) {
		return project.Document{
			ID: id, Title: "A design", Revision: "rev-1", ReadAt: "2026-09-23T11:30:00Z",
			Record: &project.Head{Kind: "design", ID: "01M37EX4M5VVXQBPH137CTM89V",
				Status: "accepted", Goals: []string{"g1-s28"}},
			Headings: []markdown.Heading{{Level: 1, Text: "A design"}, {Level: 2, Text: "The seam"}},
		}, nil
	}
	composed := Compose(facts, Page{
		Section: "Project", Path: "/project/doc/records/designs/d1.md",
		Kind: KindDocument, Subject: "records/designs/d1.md", Title: "A design",
		Revision: "rev-1",
	}, "Wido", composedAt)
	testutil.Expect(t, "the document is named", strings.Contains(composed, "- Document: records/designs/d1.md"), true)
	testutil.Expect(t, "the displayed revision", strings.Contains(composed, "- Displayed revision: rev-1"), true)
	testutil.Expect(t, "the head's kind", strings.Contains(composed, "- Kind: design"), true)
	testutil.Expect(t, "the head's status", strings.Contains(composed, "- Status: accepted"), true)
	testutil.Expect(t, "the head's goals", strings.Contains(composed, "- Goals: g1-s28"), true)
	testutil.Expect(t, "the headings", strings.Contains(composed, "- The seam"), true)
}

// The block is composed from the server's own document reader — the same one
// the reading view is served from — and not from anything the page sent.
func TestADocumentsContextIsComposedFromTheServersOwnReader(t *testing.T) {
	t.Parallel()
	facts := readingFacts()
	facts.Document = documentReader(t)
	composed := Compose(facts, Page{
		Kind: KindDocument, Subject: "records/designs/d1.md",
	}, "Wido", composedAt)
	testutil.Expect(t, "the document is named", strings.Contains(composed, "- Document: records/designs/d1.md"), true)
	testutil.Expect(t, "its title is the file's", strings.Contains(composed, "- Title: A design"), true)
	testutil.Expect(t, "its headings are the file's", strings.Contains(composed, "- The seam"), true)
	testutil.Expect(t, "the page said no revision, so this server's read is named",
		strings.Contains(composed, "(this server's read; the page did not say)"), true)
}

// The revision the page displayed wins, and a file that has moved on since is
// said to have moved on.
func TestADocumentThatChangedSinceTheHumanLoadedItSaysSo(t *testing.T) {
	t.Parallel()
	facts := readingFacts()
	facts.Document = documentReader(t)
	composed := Compose(facts, Page{
		Kind: KindDocument, Subject: "records/designs/d1.md", Revision: "older",
	}, "Wido", composedAt)
	testutil.Expect(t, "it names both", strings.Contains(composed,
		"so it changed after the human loaded it"), true)
}

// A build with no reader says it has none rather than guessing.
func TestAContextWithNoReaderSaysSo(t *testing.T) {
	t.Parallel()
	composed := Compose(Facts{}, Page{Kind: KindGoal, Subject: "g1"}, "Wido", composedAt)
	testutil.Expect(t, "it says it cannot", strings.Contains(composed, "no ledger reader"), true)
}

// A document that cannot be read is said to be unreadable, in the reader's own
// words.
func TestADocumentThatCannotBeReadSaysWhy(t *testing.T) {
	t.Parallel()
	composed := Compose(Facts{Document: func(string) (project.Document, error) {
		return project.Document{}, errors.New("no document at that id")
	}}, Page{Kind: KindDocument, Subject: "nope.md"}, "Wido", composedAt)
	testutil.Expect(t, "it says why", strings.Contains(composed,
		"The displayed document could not be read: no document at that id"), true)
}

func readingFacts() Facts {
	return Facts{Observe: func() snapshot.Observation { return readObservation() }}
}

func readObservation() snapshot.Observation {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	tree.Live["waiting"] = &goal.GoalFile{
		Id: "waiting", State: goal.StateQueued, Intent: "Do waiting", Origin: "main",
		NextStep: "Start waiting.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
	}
	horizon := goal.NewApprovalHorizon(tree, observedAt)
	return snapshot.Observation{
		ObservedAt: observedAt, State: snapshot.StateRead, Tip: observedTip,
		Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Tree: tree, Horizon: horizon}),
	}
}

// documentReader serves one real document from a temporary checkout, through
// the same package the pages are served from, so the block is composed from
// the server's own reader and not from a fixture of this test's own shape.
func documentReader(t *testing.T) func(string) (project.Document, error) {
	t.Helper()
	checkout := t.TempDir()
	home := filepath.Join(checkout, "records", "designs")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("the fixture checkout must be writable: %v", err)
	}
	source := strings.Join([]string{
		"# A design", "",
		"- Kind: design", "- Id: 01M37EX4M5VVXQBPH137CTM89V", "- Status: accepted", "",
		"## The seam", "", "One client.", "",
	}, "\n")
	if err := os.WriteFile(filepath.Join(home, "d1.md"), []byte(source), 0o644); err != nil {
		t.Fatalf("the fixture document must be writable: %v", err)
	}
	roots := project.Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
	return func(id string) (project.Document, error) {
		return project.Read(roots, id, composedAt)
	}
}
