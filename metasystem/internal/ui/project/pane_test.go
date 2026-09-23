package project

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The adapter's fixtures are whole small projects written into the two layouts
// the checkout fixtures already build: a ledger of three goals, a one-page
// intent book, a doctrine book whose reading order names a record and binds a
// document, one decision, three designs across the two homes, a register, and
// enough Markdown carrying no head to prove that the rest of the checkout is
// still listed by path.
//
// The record helper is the resolver's, written out again here rather than
// imported: a package's tests are not an API, and a fixture that drifted from
// the one it copied would be a fault this pane could not see.

func record(title, kind, id, status, goals string, extra ...string) string {
	head := []string{
		"- Kind: " + kind,
		"- Id: " + id,
		"- Status: " + status,
	}
	if goals != "" {
		head = append(head, "- Goals: "+goals)
	}
	head = append(head, extra...)
	return "# " + title + "\n\n" + strings.Join(head, "\n") + "\n"
}

// goalFile is one goal of the ledger as the ledger writes it: only the heading,
// the state and the intent are read from it here.
func goalFile(id, state, intent string) string {
	return "# " + id + "\n\n- State: " + state + "\n- Intent: " + intent + "\n"
}

// seed writes the project into one layout. state is the checkout-relative
// prefix of the state root, which is the nested installation in the
// self-hosted layout and nothing at all in an adopted one.
func seed(t *testing.T, roots Roots) string {
	t.Helper()
	state := ""
	if roots.StateRoot != roots.Checkout {
		state = relativeTo(t, roots.Checkout, roots.StateRoot) + "/"
	}

	plant(t, roots.Checkout, state+"plans/goals/backlog.md", "# backlog\n\n- SyncMode: local\n")
	plant(t, roots.Checkout, state+"plans/goals/ledger-sync.md",
		goalFile("ledger-sync", "queued", "The ledger syncs on every landing"))
	plant(t, roots.Checkout, state+"plans/goals/reading-pane.md",
		goalFile("reading-pane", "approved", "The pane reads a document as a chapter"))
	plant(t, roots.Checkout, state+"records/goals/two-homes.md",
		goalFile("two-homes", "done", "Designs live in two homes"))

	plant(t, roots.Checkout, state+"docs/architecture.md", "# The engine\n\nProse, and no head, so no kind is claimed.\n")
	plant(t, roots.Checkout, state+"plans/designs/summaries.md",
		record("A design that says what it is", "design", "design-summary", "draft", "ledger-sync")+
			"\n## Outcome\n\n"+
			"| a | b |\n| --- | --- |\n| 1 | 2 |\n\n"+
			"- a list, which is not prose\n\n"+
			"The **first** paragraph, with a [link](./ledger.md) and `code`, is the summary.\n"+
			"It runs to the second line.\n\n"+
			"A second paragraph nobody reads.\n")
	plant(t, roots.Checkout, state+"docs/intent/index.md",
		record("The project's intent", "intent", "intent-index", "accepted", "")+
			"\n## Chapters\n"+
			"- doc:"+state+"docs/architecture.md — The engine\n")
	plant(t, roots.Checkout, state+"docs/doctrine/index.md",
		record("The project's doctrine", "doctrine", "doctrine-index", "accepted", "")+
			"\n## Chapters\n"+
			"- doctrine-events — Events are the source of truth\n"+
			"- doctrine-budgets\n")
	// Neither chapter names a goal, and neither may: the doctrine is the
	// project's own, so a chapter of it is about the whole by definition.
	plant(t, roots.Checkout, state+"docs/doctrine/events.md",
		record("Events are the source of truth", "doctrine", "doctrine-events", "accepted", ""))
	plant(t, roots.Checkout, state+"docs/doctrine/chapters/budgets.md",
		record("Every run is budgeted", "doctrine", "doctrine-budgets", "draft", ""))
	plant(t, roots.Checkout, state+"docs/decisions/0001-one-binary.md",
		record("One binary", "decision", "decision-one-binary", "accepted", ""))
	plant(t, roots.Checkout, state+"plans/designs/ledger.md",
		record("The ledger", "design", "design-ledger", "done", "ledger-sync"))
	plant(t, roots.Checkout, state+"plans/designs/pane/reading.md",
		record("The reading pane", "design", "design-reading", "draft", "reading-pane"))
	plant(t, roots.Checkout, "plans/designs/interface.md",
		record("The interface", "design", "design-interface", "accepted", "ledger-sync two-homes"))
	plant(t, roots.Checkout, state+"memory/questions.md",
		"# Open questions\n\n"+
			"| id | opened | question | goals | status |\n"+
			"| --- | --- | --- | --- | --- |\n"+
			"| Q-1 | 2026-09-22 | Where does an adopted project's intent live? |  | open |\n"+
			"| Q-2 | 2026-09-22 | Who accepts a decision? | ledger-sync | answered: decision-one-binary |\n")
	return state
}

func relativeTo(t *testing.T, base, target string) string {
	t.Helper()
	relative, err := filepath.Rel(base, target)
	if err != nil {
		t.Fatalf("relate %s to %s: %v", target, base, err)
	}
	return filepath.ToSlash(relative)
}

// What the records declare is what the pane carries: the ledger's goals, live
// ones first, every record with its home and the goals it is about, and the two
// books in reading order.
func TestPaneCarriesWhatTheRecordsDeclare(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the schema version", pane.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "read at", pane.ReadAt, "2026-09-21T10:11:12Z")
	testutil.Expect(t, "the ledger's goals, live ones first", pane.Goals, []Goal{
		{ID: "ledger-sync", Title: "ledger-sync", State: "queued",
			Intent: "The ledger syncs on every landing"},
		{ID: "reading-pane", Title: "reading-pane", State: "approved",
			Intent: "The pane reads a document as a chapter"},
		{ID: "two-homes", Title: "two-homes", State: "done", Intent: "Designs live in two homes"},
	})
	testutil.Expect(t, "every record, in path order", recordIDs(pane.Records), []string{
		"decision-one-binary", "doctrine-budgets", "doctrine-events", "doctrine-index",
		"intent-index", "design-ledger", "design-reading", "design-summary", "design-interface",
	})
	testutil.Expect(t, "one design's row", recordWithID(pane.Records, "design-reading"), Record{
		Kind: "design", ID: "design-reading", Status: "draft", Goals: []string{"reading-pane"},
		Title: "The reading pane", Path: "metasystem/plans/designs/pane/reading.md",
		Home: "metasystem/plans/designs", Slices: []string{},
	})
	testutil.Expect(t, "a record about the project as a whole names no goal",
		recordWithID(pane.Records, "decision-one-binary").Goals, []string{})
	// A record's own first words travel with it: the first prose paragraph
	// after the head, as plain text, whatever the body opens with.
	testutil.Expect(t, "a record's summary", recordWithID(pane.Records, "design-summary").Summary,
		"The first paragraph, with a link and code, is the summary. It runs to the second line.")
	testutil.Expect(t, "a record whose body carries no prose has none",
		recordWithID(pane.Records, "design-reading").Summary, "")
	testutil.Expect(t, "the second design home", recordWithID(pane.Records, "design-interface").Home, "plans/designs")
	testutil.Expect(t, "nothing is refused", pane.Problems, []Problem{})
}

// A book is its index and its reading order: a chapter names a record by id or
// binds a document by path, and a chapter the index left unnamed is titled by
// what it names rather than left blank.
func TestPaneCarriesEachBookInReadingOrder(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Require(t, "intent has an index", pane.Intent.Index != nil, true)
	testutil.Expect(t, "the intent index", pane.Intent.Index.ID, "intent-index")
	// A chapter carries its own first words too: a bound document is read for
	// the paragraph after its title, and a record chapter borrows the summary
	// already read from the record.
	testutil.Expect(t, "the intent chapters", pane.Intent.Chapters, []Chapter{
		{Path: "metasystem/docs/architecture.md", Title: "The engine",
			Summary: "Prose, and no head, so no kind is claimed."},
	})
	testutil.Require(t, "doctrine has an index", pane.Doctrine.Index != nil, true)
	testutil.Expect(t, "the doctrine index", pane.Doctrine.Index.Title, "The project's doctrine")
	testutil.Expect(t, "the doctrine chapters", pane.Doctrine.Chapters, []Chapter{
		{ID: "doctrine-events", Title: "Events are the source of truth"},
		{ID: "doctrine-budgets", Title: "Every run is budgeted"},
	})
	testutil.Expect(t, "the schema the slice plan arrived in", pane.SchemaVersion, 5)
	testutil.Expect(t, "the register", pane.Questions, []Question{
		{ID: "Q-1", Opened: "2026-09-22", Question: "Where does an adopted project's intent live?",
			Goals: []string{}, Status: "open"},
		{ID: "Q-2", Opened: "2026-09-22", Question: "Who accepts a decision?",
			Goals: []string{"ledger-sync"}, Status: "answered: decision-one-binary"},
	})
}

// A broken record is shown, not hidden: it keeps its row, and the refusal the
// check verb would print is carried beside it, anchored at its line.
func TestPaneShowsARecordTheCheckRefuses(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	state := seed(t, roots)
	plant(t, roots.Checkout, state+"plans/designs/broken.md",
		record("A design about a goal the ledger does not have", "design", "design-broken", "accepted", "nowhere"))

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "the broken record is listed", recordWithID(pane.Records, "design-broken").Title,
		"A design about a goal the ledger does not have")
	testutil.Expect(t, "the refusal", pane.Problems, []Problem{{
		Path:    "metasystem/plans/designs/broken.md",
		Line:    6,
		Message: "the goal nowhere is not in the ledger",
	}})
}

// The rest of the checkout's Markdown is listed by path, with no kind claimed:
// every file the reading route would serve, titled by its first heading or by
// its name, and nothing beneath a segment that route refuses.
func TestPaneListsTheCheckoutsOtherMarkdownByPath(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.Checkout, "notes/untitled.md", "Prose with no heading at all.\n")
	plant(t, roots.Checkout, "notes/README.txt", "Not Markdown.\n")
	plant(t, roots.Checkout, "node_modules/left/out.md", "# Left out\n")
	plant(t, roots.Checkout, "artifacts/left-out.md", "# Left out\n")
	plant(t, roots.Checkout, "bin/left-out.md", "# Left out\n")
	plant(t, roots.Checkout, ".hidden/left-out.md", "# Left out\n")
	makeDirectory(t, filepath.Join(roots.Checkout, "notes", "elsewhere"))
	link(t, filepath.Join(roots.Checkout, "notes", "untitled.md"),
		filepath.Join(roots.Checkout, "notes", "elsewhere", "linked.md"))

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "a document titled by its heading",
		fileWithPath(pane.Documents, "development/metasystem-design.md").Title, "The design")
	testutil.Expect(t, "a document with no heading",
		fileWithPath(pane.Documents, "notes/untitled.md").Title, "untitled.md")
	testutil.Expect(t, "a record is a document by path too",
		fileWithPath(pane.Documents, "plans/designs/interface.md").Title, "The interface")
	testutil.Expect(t, "what the reading route refuses is not listed", refusedPaths(pane.Documents), []string{})
	testutil.Expect(t, "nothing but Markdown", notMarkdown(pane.Documents), []string{})
	testutil.Expect(t, "a link is not offered", fileWithPath(pane.Documents, "notes/elsewhere/linked.md"), File{})

	// Every path listed is a path the route answers, which is the whole of
	// what "offer them by path" promises.
	for _, file := range pane.Documents {
		document, readErr := Read(roots, file.Path, readAt)
		if readErr != nil {
			t.Fatalf("the route refuses a listed document %q: %v", file.Path, readErr)
		}
		if document.Title != file.Title {
			t.Fatalf("the listing and the route disagree about %q: %q and %q", file.Path, file.Title, document.Title)
		}
	}
}

// An adopted workspace has one design home, its own, and the pane says so
// rather than inventing a second.
func TestPaneReadsTheAdoptedLayoutsOneDesignHome(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	seed(t, roots)

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "every design is in the one home", homesOf(pane.Records, "design"), []string{"plans/designs"})
	testutil.Expect(t, "the designs", recordTitles(pane.Records, "design"),
		[]string{"The interface", "The ledger", "The reading pane", "A design that says what it is"})
	testutil.Expect(t, "nothing is refused", pane.Problems, []Problem{})
}

// Every goal a record can name is in the pane, concluded ones included, each
// with the state its ledger file declares.
//
// The pane is where a reader finds out whether the work a design names has
// landed, and a goal that landed is in records/goals rather than plans/goals.
// A payload that carried only the live ones would show a shipped design as a
// design naming a goal nobody has heard of.
func TestPaneCarriesConcludedGoalsWithTheirState(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	state := relativeTo(t, roots.Checkout, roots.StateRoot) + "/"
	plant(t, roots.Checkout, state+"records/goals/dropped.md",
		goalFile("dropped", "abandoned", "An idea nobody pursued"))

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "a concluded goal is carried", goalWithID(pane.Goals, "two-homes"),
		Goal{ID: "two-homes", Title: "two-homes", State: "done", Intent: "Designs live in two homes"})
	testutil.Expect(t, "a goal concluded another way carries that state",
		goalWithID(pane.Goals, "dropped").State, "abandoned")
	testutil.Expect(t, "the live goals come first, then the concluded ones in id order",
		goalIDs(pane.Goals), []string{"ledger-sync", "reading-pane", "dropped", "two-homes"})
	// The design that names the concluded goal is the pane's other half of
	// this: a reader works out that its work landed from these two together.
	testutil.Expect(t, "the design that names it", recordWithID(pane.Records, "design-interface").Goals,
		[]string{"ledger-sync", "two-homes"})
	testutil.Expect(t, "a Goals line naming a concluded goal is not refused", pane.Problems, []Problem{})
}

// A project with no book at all says so: no index, no chapters, and no
// refusal, because a project declares the homes it uses.
func TestPaneCarriesNoBookWhereNoneIsDeclared(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)

	pane, err := ReadPane(roots, readAt)

	testutil.Require(t, "read the pane", err, nil)
	testutil.Expect(t, "no intent index", pane.Intent, Book{Chapters: []Chapter{}})
	testutil.Expect(t, "no doctrine index", pane.Doctrine, Book{Chapters: []Chapter{}})
	testutil.Expect(t, "no goals", pane.Goals, []Goal{})
	testutil.Expect(t, "no records", pane.Records, []Record{})
	testutil.Expect(t, "no questions", pane.Questions, []Question{})
	testutil.Expect(t, "the checkout's Markdown is still listed",
		fileWithPath(pane.Documents, "development/metasystem-design.md").Title, "The design")
}

func recordIDs(records []Record) []string {
	collected := []string{}
	for _, record := range records {
		collected = append(collected, record.ID)
	}
	return collected
}

func recordWithID(records []Record, id string) Record {
	for _, record := range records {
		if record.ID == id {
			return record
		}
	}
	return Record{}
}

func recordTitles(records []Record, kind string) []string {
	collected := []string{}
	for _, record := range records {
		if record.Kind == kind {
			collected = append(collected, record.Title)
		}
	}
	return collected
}

func homesOf(records []Record, kind string) []string {
	collected := []string{}
	for _, record := range records {
		if record.Kind == kind && !contains(collected, record.Home) {
			collected = append(collected, record.Home)
		}
	}
	return collected
}

func fileWithPath(files []File, path string) File {
	for _, file := range files {
		if file.Path == path {
			return file
		}
	}
	return File{}
}

// refusedPaths are the listed paths the reading route would refuse: a segment
// it never serves through, or a directory hidden from a listing.
func refusedPaths(files []File) []string {
	collected := []string{}
	for _, file := range files {
		for _, segment := range strings.Split(file.Path, "/") {
			if refusedSegment(segment) || strings.HasPrefix(segment, ".") {
				collected = append(collected, file.Path)
				break
			}
		}
	}
	return collected
}

// The slice plan is read out of two records that already exist: the goal's own
// slicing boundary, and the list a governing design writes under a Slices
// heading. Neither is a structure the engine owns — there is no slice-plan
// owner yet — so what is carried is what was written, and nothing else.
func TestPaneReadsTheSlicePlanOutOfTheRecordsThatHaveOne(t *testing.T) {
	t.Parallel()
	roots := selfHostedFixture(t)
	seed(t, roots)
	state := relativeTo(t, roots.Checkout, roots.StateRoot) + "/"

	plant(t, roots.Checkout, state+"plans/goals/sliced-goal.md",
		"# sliced-goal\n\n- State: claimed\n- Intent: The board reads a slice plan\n"+
			"- Sliced: machine=m1e lineage=coordinator revision=4 at=2026-09-18T08:00:00Z\n")
	plant(t, roots.Checkout, state+"plans/designs/sliced.md",
		record("A design that plans its slices", "design", "design-sliced", "accepted", "sliced-goal")+
			"\nProse that is not a slice.\n\n"+
			"## Slices\n\n"+
			"- The payload carries the boundary\n"+
			"* The tab reads it\n"+
			"1. The design's list is shown as written\n"+
			"  - an indented item is still an item\n\n"+
			"## Verification\n\n"+
			"- not a slice, because the section ended\n")

	pane, err := ReadPane(roots, readAt)
	testutil.Require(t, "read the pane", err, nil)

	testutil.Expect(t, "the goal's slicing boundary", goalWithID(pane.Goals, "sliced-goal").Sliced,
		&Sliced{At: "2026-09-18T08:00:00Z", Machine: "m1e", Lineage: "coordinator"})
	testutil.Expect(t, "a goal nobody sliced carries none",
		goalWithID(pane.Goals, "ledger-sync").Sliced == nil, true)
	testutil.Expect(t, "the design's slices, as written",
		recordWithID(pane.Records, "design-sliced").Slices, []string{
			"The payload carries the boundary",
			"The tab reads it",
			"The design's list is shown as written",
			"an indented item is still an item",
		})
	testutil.Expect(t, "a design with no such section lists none",
		recordWithID(pane.Records, "design-summary").Slices, []string{})
}

func goalIDs(goals []Goal) []string {
	collected := []string{}
	for _, one := range goals {
		collected = append(collected, one.ID)
	}
	return collected
}

func goalWithID(goals []Goal, id string) Goal {
	for _, one := range goals {
		if one.ID == id {
			return one
		}
	}
	return Goal{}
}

func notMarkdown(files []File) []string {
	collected := []string{}
	for _, file := range files {
		if !strings.HasSuffix(file.Path, ".md") {
			collected = append(collected, file.Path)
		}
	}
	return collected
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
