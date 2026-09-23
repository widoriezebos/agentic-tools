package uitools

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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

var (
	readAt      = time.Date(2026, 9, 23, 11, 30, 0, 0, time.UTC)
	observedAt  = time.Date(2026, 9, 23, 11, 29, 55, 0, time.UTC)
	observedTip = "c5d517f427e35e19c2944ab9aefee4d9c9cd9e6c"
)

// Every result names the reading it was of. A ledger tip cannot stamp a
// document and a document's revision cannot stamp a goal, so the two carry
// different sources and both say which.
func TestEveryResultNamesTheReadingItWasOf(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	board := readers.Answer(OpBoard, Args{})
	testutil.Expect(t, "the board reads the accepted tip",
		strings.Contains(board.Source, "the accepted tip "+observedTip+", observed 2026-09-23T11:29:55Z"), true)
	document := readers.Answer(OpDocument, Args{"id": "records/designs/d1.md"})
	testutil.Expect(t, "a document reads the file as it stands",
		strings.Contains(document.Source, "records/designs/d1.md as it stands, revision "), true)
	records := readers.Answer(OpRecords, Args{})
	testutil.Expect(t, "the records read the checkout",
		strings.HasPrefix(records.Source, "the checkout's records as they stand, read "), true)
}

// The board carries the rows the ledger holds, one line each, and says how
// many of how many it supplied.
func TestTheBoardIsEveryRowWithItsCount(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpBoard, Args{})
	testutil.Expect(t, "every goal", result.Total, 4)
	testutil.Expect(t, "all of them supplied", result.Supplied, 4)
	testutil.Expect(t, "nothing remains", result.Cursor, "")
	testutil.Expect(t, "a claimed goal names its seat",
		strings.Contains(result.Body, "running · Do running. · tier 3 · 2:6 · claimed · seat m1e"), true)
	testutil.Expect(t, "and the text says how much of the whole it is",
		strings.Contains(result.Text(), "Supplied: 4 of 4"), true)
}

// The board's filter narrows to the rows whose line carries the text, whatever
// part of the line that is.
func TestTheBoardsFilterNarrowsByAnyPartOfTheLine(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	testutil.Expect(t, "by seat", readers.Answer(OpBoard, Args{"filters": "m1e"}).Total, 1)
	testutil.Expect(t, "by lane", readers.Answer(OpBoard, Args{"filters": "waiting"}).Total, 2)
	testutil.Expect(t, "by tier", readers.Answer(OpBoard, Args{"filters": "tier 2"}).Total, 1)
	testutil.Expect(t, "and nothing matches nothing", readers.Answer(OpBoard, Args{"filters": "zzz"}).Total, 0)
}

// A listing longer than one result stops at a whole row, says how many it
// supplied, and hands back the cursor that reads the rest.
func TestAListingLongerThanTheBoundCarriesACursor(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	readers.Project = func() (project.Pane, error) {
		return project.Pane{ReadAt: stamp(readAt), Records: manyRecords(400)}, nil
	}

	first := readers.Answer(OpRecords, Args{})
	testutil.Expect(t, "the whole is known", first.Total, 400)
	testutil.Expect(t, "not all of it is supplied", first.Supplied < first.Total, true)
	testutil.Expect(t, "the body stays inside the bound", len(first.Body) <= MaxBody, true)
	testutil.Expect(t, "and a cursor says where the rest is", first.Cursor, strconv.Itoa(first.Supplied))
	testutil.Expect(t, "which the text says too",
		strings.Contains(first.Text(), "More remains: call this tool again with cursor"), true)

	second := readers.Answer(OpRecords, Args{"cursor": first.Cursor})
	testutil.Expect(t, "the second page starts where the first stopped",
		strings.Contains(second.Body, "Record "+strconv.Itoa(first.Supplied)+" ·"), true)
	testutil.Expect(t, "and counts from there", second.Supplied > first.Supplied, true)

	// Every page, read in turn, reaches the end and stops.
	cursor, pages := first.Cursor, 1
	for cursor != "" && pages < 20 {
		page := readers.Answer(OpRecords, Args{"cursor": cursor})
		cursor = page.Cursor
		pages++
		if cursor == "" {
			testutil.Expect(t, "the last page supplies the whole", page.Supplied, 400)
		}
	}
	testutil.Expect(t, "the walk ends", cursor, "")
}

// A document is paged by its own characters, so a design longer than one
// result can be read whole in order.
func TestADocumentLongerThanTheBoundIsReadInOrder(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	first := readers.Answer(OpDocument, Args{"id": "records/designs/long.md"})
	testutil.Expect(t, "it starts at the beginning",
		strings.Contains(first.Body, "# A long design"), true)
	testutil.Expect(t, "it stops inside the bound", len(first.Body) <= MaxBody+len(recordHeadOf(t)), true)
	testutil.Require(t, "and says where the rest is", first.Cursor != "", true)

	second := readers.Answer(OpDocument, Args{"id": "records/designs/long.md", "cursor": first.Cursor})
	testutil.Expect(t, "the second call continues rather than repeating",
		strings.Contains(second.Body, "# A long design"), false)
	testutil.Expect(t, "and starts where the first stopped", second.Supplied > first.Supplied, true)

	// Read on until the document ends: the last page carries the last line and
	// hands back nothing, and the whole has been supplied.
	page, pages := second, 2
	for page.Cursor != "" && pages < 20 {
		page = readers.Answer(OpDocument, Args{"id": "records/designs/long.md", "cursor": page.Cursor})
		pages++
	}
	testutil.Expect(t, "the last page reaches the last line",
		strings.Contains(page.Body, "The last line."), true)
	testutil.Expect(t, "with nothing left", page.Cursor, "")
	testutil.Expect(t, "and the whole supplied", page.Supplied, page.Total)
}

// A goal names itself field by field and the goals beside it in its lane.
func TestAGoalIsItsRowAndItsNeighbours(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpGoal, Args{"id": "waiting"})
	testutil.Expect(t, "the goal", strings.Contains(result.Body, "- Goal: waiting"), true)
	testutil.Expect(t, "its next step", strings.Contains(result.Body, "- Next step: Start waiting."), true)
	testutil.Expect(t, "and its neighbours",
		strings.Contains(result.Body, "The other goals in waiting (1): waiting-two"), true)
}

// A read that did not happen is a failure with the reader's own words, never
// an empty answer that reads like an absence.
func TestAReadThatFailedSaysSoRatherThanAnsweringEmpty(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	missing := readers.Answer(OpGoal, Args{"id": "ghost"})
	testutil.Expect(t, "the goal is missing", missing.Failed(), true)
	testutil.Expect(t, "in the tip's own terms",
		strings.Contains(missing.Problem, "the accepted tip carries no goal ghost"), true)
	testutil.Expect(t, "and the text says the read failed",
		strings.Contains(missing.Text(), "Outcome: this read failed"), true)

	readers.Project = func() (project.Pane, error) { return project.Pane{}, errors.New("the records could not be read") }
	broken := readers.Answer(OpRecords, Args{})
	testutil.Expect(t, "a reader that refused", broken.Problem, "the records could not be read")

	none := Readers{Now: func() time.Time { return readAt }}
	testutil.Expect(t, "and a build with no reader at all",
		none.Answer(OpBoard, Args{}).Problem, "this build has no ledger reader")
}

// An operation this server does not have is refused by name rather than
// answered with something else.
func TestAnUnknownOperationIsRefusedByName(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer("write", Args{})
	testutil.Expect(t, "it names what it answers",
		strings.Contains(result.Problem, "this server answers board, goal, document"), true)
	testutil.Expect(t, "and what was asked for", strings.HasSuffix(result.Problem, ", not write"), true)
}

// Search reaches across the ledger and the checkout at once, and says it read
// both.
func TestSearchReadsTheLedgerAndTheCheckout(t *testing.T) {
	t.Parallel()
	result := fixture(t).Answer(OpSearch, Args{"text": "waiting"})
	testutil.Expect(t, "goals are found", strings.Contains(result.Body, "- goal waiting: waiting ·"), true)
	testutil.Expect(t, "and both readings are named",
		strings.Contains(result.Source, "the accepted tip ") && strings.Contains(result.Source, "the checkout's records"), true)
	testutil.Expect(t, "a search for nothing is refused",
		fixture(t).Answer(OpSearch, Args{}).Failed(), true)
}

// The journal is read newest first, and a full page hands back the id the next
// page continues from — the journal's own cursor, which is not an offset.
func TestTheJournalsCursorIsItsOwnOldestEntry(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	readers.Notices = func(limit int, before string) ([]notifications.Notice, error) {
		notices := []notifications.Notice{}
		for at := 0; at < limit; at++ {
			notices = append(notices, notifications.Notice{
				ID: "n" + strconv.Itoa(at), At: stamp(readAt), Source: "steward",
				Message: "something happened", Delivered: true,
			})
		}
		return notices, nil
	}
	result := readers.Answer(OpNotifications, Args{"limit": 3})
	testutil.Expect(t, "three entries", result.Supplied, 3)
	testutil.Expect(t, "and the cursor is the oldest id", result.Cursor, "n2")
}

// The questions and the landing page answer from the readers the pages use.
func TestQuestionsAndOverviewAnswerFromThePagesOwnReaders(t *testing.T) {
	t.Parallel()
	readers := fixture(t)
	questions := readers.Answer(OpQuestions, Args{})
	testutil.Expect(t, "the open question", strings.Contains(questions.Body, "Who decides? · open · q1"), true)
	overview := readers.Answer(OpOverview, Args{})
	testutil.Expect(t, "the landing page's own numbers",
		strings.Contains(overview.Body, "- Needs you (2 in all):"), true)
	testutil.Expect(t, "and the window it compares against",
		strings.Contains(overview.Body, "The window this page compares against opens"), true)
}

/* ------------------------------------------------------------ the fixture -- */

func fixture(t *testing.T) Readers {
	t.Helper()
	checkout := t.TempDir()
	home := filepath.Join(checkout, "records", "designs")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("the fixture checkout must be writable: %v", err)
	}
	write(t, filepath.Join(home, "d1.md"), strings.Join([]string{
		"# A design", "",
		"- Kind: design", "- Id: 01M37EX4M5VVXQBPH137CTM89V", "- Status: accepted", "",
		"## The seam", "", "One client, and nothing else.", "",
	}, "\n"))
	long := []string{"# A long design", "", "- Kind: design", "- Status: accepted", ""}
	for at := 0; at < 400; at++ {
		long = append(long, "Paragraph "+strconv.Itoa(at)+" of a design nobody shortened.", "")
	}
	long = append(long, "The last line.", "")
	write(t, filepath.Join(home, "long.md"), strings.Join(long, "\n"))
	roots := project.Roots{Checkout: checkout, Installation: checkout, StateRoot: checkout}
	return Readers{
		Now:      func() time.Time { return readAt },
		Observe:  func() snapshot.Observation { return readObservation() },
		Document: func(id string) (project.Document, error) { return project.Read(roots, id, readAt) },
		Project:  func() (project.Pane, error) { return readPane(), nil },
		Overview: func() (overview.Page, error) { return readOverview(), nil },
		Notices:  func(int, string) ([]notifications.Notice, error) { return nil, nil },
	}
}

func write(t *testing.T, at, body string) {
	t.Helper()
	if err := os.WriteFile(at, []byte(body), 0o644); err != nil {
		t.Fatalf("the fixture file must be writable: %v", err)
	}
}

// recordHeadOf is how much a document's result carries beyond its source: the
// record head, which the first page prefixes.
func recordHeadOf(t *testing.T) string {
	t.Helper()
	return "- Kind: design\n- Status: accepted\n\n"
}

func readPane() project.Pane {
	return project.Pane{
		ReadAt: stamp(readAt),
		Records: []project.Record{
			{Kind: "design", ID: "d1", Title: "A design", Status: "accepted", Path: "records/designs/d1.md"},
			{Kind: "decision", ID: "k1", Title: "A decision", Status: "accepted", Path: "records/decisions/k1.md"},
		},
		Questions: []project.Question{{ID: "q1", Question: "Who decides?", Status: "open", Opened: stamp(readAt)}},
		Documents: []project.File{{Path: "docs/reading.md", Title: "Reading"}},
	}
}

func manyRecords(count int) []project.Record {
	records := make([]project.Record, 0, count)
	for at := 0; at < count; at++ {
		records = append(records, project.Record{
			Kind: "design", ID: "r" + strconv.Itoa(at), Title: "Record " + strconv.Itoa(at),
			Status: "accepted", Path: "records/designs/r" + strconv.Itoa(at) + ".md",
		})
	}
	return records
}

func stamp(at time.Time) string { return at.UTC().Format(time.RFC3339) }

func readOverview() overview.Page {
	return overview.Page{
		ReadAt: stamp(readAt),
		Since:  stamp(readAt.Add(-24 * time.Hour)),
		First:  true,
		NeedsYou: overview.NeedsYou{
			Approvals: overview.Group{Count: 1},
			Questions: overview.Group{Count: 1},
			Total:     2,
		},
		Work: overview.Work{Lanes: []overview.Lane{{ID: "ready", Count: 1}}},
	}
}

func readObservation() snapshot.Observation {
	tree := &goal.TreeGoals{
		Root:      &goal.RootRecord{Identity: "01ARZ3NDEKTSV4RRFFQ69G5FAV", FormatVersion: "2", SyncMode: goal.SyncLocal, Revision: 1},
		Live:      map[string]*goal.GoalFile{},
		Done:      map[string]*goal.GoalFile{},
		Abandoned: map[string]*goal.GoalFile{},
	}
	tree.Live["waiting"] = parked("waiting")
	tree.Live["waiting-two"] = parked("waiting-two")
	tree.Live["running"] = &goal.GoalFile{
		Id: "running", State: goal.StateClaimed, Intent: "Do running. And then more.",
		Origin: "main", NextStep: "Keep going.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Tier: 3, Priority: 2, Sequence: 6,
		Claimed: &goal.ClaimRecord{Machine: "m1e", Lineage: "coordinator", At: "2026-09-20T09:00:00Z"},
	}
	tree.Live["ready-one"] = &goal.GoalFile{
		Id: "ready-one", State: goal.StateApproved, Intent: "Do ready-one. With care.",
		Origin: "main", NextStep: "Take it.", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Tier: 2, Priority: 1, Sequence: 3,
	}
	horizon := goal.NewApprovalHorizon(tree, observedAt)
	return snapshot.Observation{
		ObservedAt: observedAt, State: snapshot.StateRead, Tip: observedTip,
		Tree: tree, Horizon: horizon, SyncMode: goal.SyncLocal,
		Admission: backlog.Admit(goal.Projection{Tree: tree, Horizon: horizon}),
	}
}

func parked(id string) *goal.GoalFile {
	return &goal.GoalFile{
		Id: id, State: goal.StateParked, Intent: "Do " + id + ".", Origin: "main",
		NextStep: "Start " + id + ".", OpenedAt: "2026-08-23T00:00:00Z", Revision: 9,
		Parked: &goal.ParkRecord{
			By: "human:wido", At: "2026-09-20T09:00:00Z",
			Because: "The census format is being decided.",
		},
	}
}
