package partner

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
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
	testutil.Expect(t, "the row's lane", strings.Contains(composed, "- Lane: "), true)
	testutil.Expect(t, "the row's intent", strings.Contains(composed, "- Intent: Do waiting."), true)
	testutil.Expect(t, "the row's next step", strings.Contains(composed, "- Next step: Start waiting."), true)
	testutil.Expect(t, "and the goals beside it in its lane",
		strings.Contains(composed, "- The other goals in waiting (1): waiting-two"), true)
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

// The board's page carries the rows the human is looking at, lane by lane, in
// the page's own order — because the goals are in the accepted ledger commit
// and in no file the Partner can read, so a context without them is a context
// that cannot answer "which goals are in Ready for Work?".
func TestTheBoardsContextCarriesTheRowsOnScreen(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{
		Section: "Backlog", Path: "/backlog",
		Filters: []string{"priority=2", "Done reaches back 1 day"},
		Lanes: []Lane{
			{ID: "ready", Title: "Ready for Work", Total: 1, Goals: []string{"ready-one"}},
			{ID: "in-progress", Title: "In Progress", Total: 1, Goals: []string{"running"}},
			{ID: "waiting", Title: "Waiting", Total: 2, Goals: []string{"waiting", "waiting-two"}},
		},
	}, "Wido", composedAt)
	testutil.Expect(t, "the filters are named", strings.Contains(composed,
		"- Filters: priority=2; Done reaches back 1 day"), true)
	testutil.Expect(t, "the lanes are named in the page's order",
		strings.Index(composed, "Ready for Work (1)") < strings.Index(composed, "In Progress (1)"), true)
	testutil.Expect(t, "an approved goal's row",
		strings.Contains(composed, "- ready-one · Do ready-one. · tier 2 · 1:3 · approved"), true)
	testutil.Expect(t, "a claimed goal names its seat",
		strings.Contains(composed, "- running · Do running. · tier 3 · 2:6 · claimed · seat m1e"), true)
	testutil.Expect(t, "a waiting goal names why",
		strings.Contains(composed, "waiting: The census format is being decided."), true)
	testutil.Expect(t, "nothing was left out", strings.Contains(composed, "more goals are on the board"), false)
}

// Only so many rows of one lane travel, and what did not is counted rather
// than dropped in silence.
func TestTheBoardsContextCapsALaneAndSaysWhatItLeftOut(t *testing.T) {
	t.Parallel()
	deep := make([]string, 0, 40)
	for at := 0; at < 40; at++ {
		deep = append(deep, "filler-"+strconv.Itoa(at))
	}
	composed := Compose(readingFacts(), Page{
		Section: "Backlog", Path: "/backlog",
		Lanes: []Lane{{ID: "to-do", Title: "To Do", Total: 400, Goals: deep}},
	}, "Wido", composedAt)
	testutil.Expect(t, "the lane's whole count is named", strings.Contains(composed, "To Do (400)"), true)
	testutil.Expect(t, "only the cap is named", strings.Count(composed, "- filler-"), maxLaneRows)
	testutil.Expect(t, "and the rest are counted",
		strings.Contains(composed, "- 375 more goals are on the board than are named here"), true)
}

// A page that named no rows is told so, rather than given the whole ledger's
// counts as though they were what is on the screen.
func TestTheBoardsContextSaysWhenThePageNamedNoRows(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Backlog", Path: "/backlog"}, "Wido", composedAt)
	testutil.Expect(t, "it counts the lanes instead",
		strings.Contains(composed, "- Goals per lane at the accepted tip:"), true)
	testutil.Expect(t, "and says which it is",
		strings.Contains(composed, "The page did not say which goals it is showing"), true)
}

// The standing rule says the one thing a Partner cannot find out for itself:
// the goals are not in the files.
func TestTheStandingRuleSaysTheLedgerIsNotInTheFiles(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Backlog"}, "Wido", composedAt)
	testutil.Expect(t, "it says so", strings.Contains(composed,
		"The ledger of goals is not in the files you can read; what you\nare told here about goals is the whole of what you know about them."), true)
}

// The block is marked with the revision it is a snapshot of, so what the
// Partner reads afterwards is distinguishable from what it was told.
func TestTheBlockIsMarkedWithTheTipItWasReadAt(t *testing.T) {
	t.Parallel()
	composed := Compose(readingFacts(), Page{Section: "Backlog"}, "Wido", composedAt)
	testutil.Expect(t, "the heading names both", strings.Contains(composed,
		"What the human sees now, from the accepted tip "+observedTip+" observed 2026-09-23T11:29:55Z"), true)
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
	tree.Live["waiting"] = waitingGoal("waiting")
	tree.Live["waiting-two"] = waitingGoal("waiting-two")
	tree.Live["running"] = claimedGoal()
	tree.Live["ready-one"] = approvedGoal()
	horizon := goal.NewApprovalHorizon(tree, observedAt)
	return snapshot.Observation{
		ObservedAt: observedAt, State: snapshot.StateRead, Tip: observedTip,
		Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Tree: tree, Horizon: horizon}),
	}
}

func waitingGoal(id string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: goal.StateParked, Intent: "Do " + id + ".", Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Parked: &goal.ParkRecord{
			By: "human:wido", At: "2026-09-20T09:00:00Z",
			Because: "The census format is being decided. It has been for a week.",
		},
	}
}

func claimedGoal() *goal.GoalFile {
	return &goal.GoalFile{
		Id: "running", State: goal.StateClaimed, Intent: "Do running. And then more.",
		Origin: "main", NextStep: "Keep going.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Tier: 3, Priority: 2, Sequence: 6,
		Claimed: &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-20T09:00:00Z"},
	}
}

func approvedGoal() *goal.GoalFile {
	return &goal.GoalFile{
		Id: "ready-one", State: goal.StateApproved, Intent: "Do ready-one. With care.",
		Origin: "main", NextStep: "Take it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Tier: 2, Priority: 1, Sequence: 3,
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
