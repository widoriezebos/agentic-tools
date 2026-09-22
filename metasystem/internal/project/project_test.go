package project

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// fixture is a temporary checkout carrying a whole small project, in one of the
// two layouts the engine installs into.
type fixture struct {
	t        *testing.T
	checkout string
	// state is the checkout-relative prefix of the state root: the nested
	// installation in the self-hosted layout, nothing in an adopted one.
	state string
	roots Roots
}

// newFixture initialises a real Git checkout, because only a Git top is a
// checkout, and installs a metasystem in the named layout. The self-hosted
// layout is the nested metasystem/ directory beside
// development/metasystem-design.md, which is the whole of stateroot's template
// test; anything else is adopted, and its state root is the Git top.
func newFixture(t *testing.T, selfHosted bool) *fixture {
	t.Helper()

	checkout := filepath.Join(t.TempDir(), "checkout")
	mustNot(t, os.MkdirAll(checkout, 0o755), "create the checkout")
	output, err := exec.Command("git", "init", "-q", "-b", "main", checkout).CombinedOutput()
	mustNot(t, err, "git init: "+string(output))
	canonical, err := filepath.EvalSymlinks(checkout)
	mustNot(t, err, "canonicalize the checkout")

	installation, state := "vendor/metasystem", ""
	if selfHosted {
		installation, state = "metasystem", "metasystem/"
	}
	f := &fixture{t: t, checkout: canonical, state: state}
	f.write(installation+"/metasystem.conf", "metasystem.runtimes=claude\n")
	mustNot(t, os.MkdirAll(filepath.Join(canonical, installation, "scripts", "agents"), 0o755),
		"create the installation's scripts")
	if selfHosted {
		f.write("development/metasystem-design.md", "# The metasystem's design\n")
	} else {
		// A design beneath an adopted installation is not a home: the state
		// root is the application's repository, and only the self-hosted layout
		// reads a second one.
		f.write(installation+"/plans/designs/decoy.md", record("Never read", "design", "design-decoy", "draft", "project"))
	}

	roots, err := ResolveRoots(filepath.Join(canonical, installation))
	mustNot(t, err, "resolve the roots")
	f.roots = roots
	return f
}

// mustNot stops the test on a fixture's own failure. The fixture builds the
// same files many times over, so its failures are reported here rather than
// through a labelled expectation, whose labels are unique per test by design.
func mustNot(t *testing.T, err error, what string) {
	t.Helper()
	if err != nil {
		t.Fatalf("fixture could not %s: %v", what, err)
	}
}

func (f *fixture) write(rel, content string) {
	f.t.Helper()
	path := filepath.Join(f.checkout, filepath.FromSlash(rel))
	mustNot(f.t, os.MkdirAll(filepath.Dir(path), 0o755), "create the directory for "+rel)
	mustNot(f.t, os.WriteFile(path, []byte(content), 0o644), "write "+rel)
}

func (f *fixture) read() *Project {
	f.t.Helper()
	read, err := Read(f.roots)
	mustNot(f.t, err, "read the project")
	return read
}

// lineOf is where a fixture file carries a piece of text, so an expected
// refusal is anchored by what it names rather than by a counted offset.
func (f *fixture) lineOf(rel, substring string) int {
	f.t.Helper()
	data, err := os.ReadFile(filepath.Join(f.checkout, filepath.FromSlash(rel)))
	mustNot(f.t, err, "read "+rel+" for its line numbers")
	for index, line := range strings.Split(string(data), "\n") {
		if strings.Contains(line, substring) {
			return index + 1
		}
	}
	f.t.Fatalf("%s carries no line containing %q", rel, substring)
	return 0
}

// record is a whole small record: a title, a head, and nothing else.
func record(title, kind, id, status, areas string, extra ...string) string {
	head := []string{
		"- Kind: " + kind,
		"- Id: " + id,
		"- Status: " + status,
		"- Areas: " + areas,
	}
	head = append(head, extra...)
	return "# " + title + "\n\n" + strings.Join(head, "\n") + "\n"
}

// seed writes the whole fixture project: a one-page intent book declaring two
// areas, a doctrine book whose reading order is two chapters and one bound
// document, two decisions with one in a subdirectory, three designs, and a
// register of three questions.
func (f *fixture) seed() {
	f.t.Helper()

	f.write(f.state+"docs/architecture.md", "# The engine\n\nProse, and no head, so no kind is claimed.\n")

	f.write(f.state+"docs/intent/index.md",
		record("The project's intent", "intent", "intent-index", "accepted", "project")+
			"\n## Areas\n"+
			"- billing — Billing and invoicing\n"+
			"- security — Security and identity\n")

	f.write(f.state+"docs/doctrine/index.md",
		record("The project's doctrine", "doctrine", "doctrine-index", "accepted", "project")+
			"\n## Chapters\n"+
			"- doctrine-events — Events are the source of truth\n"+
			"- doctrine-budgets — Every run is budgeted\n"+
			fmt.Sprintf("- doc:%sdocs/architecture.md — The engine\n", f.state))
	f.write(f.state+"docs/doctrine/events.md",
		record("Events are the source of truth", "doctrine", "doctrine-events", "accepted", "billing",
			"- Cites: intent-index"))
	f.write(f.state+"docs/doctrine/chapters/budgets.md",
		record("Every run is budgeted", "doctrine", "doctrine-budgets", "draft", "security project"))

	f.write(f.state+"docs/decisions/README.md", "# Decisions\n\nProse, and no head.\n")
	f.write(f.state+"docs/decisions/0001-one-binary.md",
		record("One binary", "decision", "decision-one-binary", "accepted", "project",
			"- Cites: doctrine-events", "- By: wido"))
	f.write(f.state+"docs/decisions/archive/0002-two-homes.md",
		record("Two design homes", "decision", "decision-two-homes", "superseded", "billing",
			"- Supersedes: decision-one-binary"))

	f.write(f.state+"plans/designs/ledger.md",
		record("The ledger", "design", "design-ledger", "done", "billing",
			"- Affects: decision-one-binary", "- Governs: goal-17"))
	f.write(f.state+"plans/designs/pane/reading.md",
		record("The reading pane", "design", "design-reading", "draft", "security"))
	// The checkout's own plans/designs: a second home in the self-hosted layout,
	// and the one design home in an adopted one.
	f.write("plans/designs/interface.md",
		record("The interface", "design", "design-interface", "accepted", "project billing",
			"- Cites: design-ledger"))

	f.write(f.state+"memory/questions.md",
		"# Open questions\n\n"+
			"| id | opened | question | areas | status |\n"+
			"| --- | --- | --- | --- | --- |\n"+
			"| Q-1 | 2026-09-22 | Where does an adopted application's intent live? | project | open |\n"+
			"| Q-2 | 2026-09-22 | Who accepts a decision? | billing | answered: decision-one-binary |\n"+
			"| Q-3 | 2026-09-22 | Do designs move when they conclude? | security | withdrawn |\n")
}

func ids(records []Record) []string {
	collected := make([]string, 0, len(records))
	for _, record := range records {
		collected = append(collected, record.ID)
	}
	return collected
}

func sorted(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}

func homeRels(homes []Home) []string {
	collected := make([]string, 0, len(homes))
	for _, home := range homes {
		collected = append(collected, home.Rel)
	}
	return collected
}

func problemLines(problems []Problem) []string {
	collected := make([]string, 0, len(problems))
	for _, problem := range problems {
		collected = append(collected, problem.String())
	}
	return collected
}

// The homes follow the layout: the self-hosted checkout reads its own
// plans/designs as a second design home, and an adopted one reads only the
// application's repository — never the installation vendored inside it.
func TestHomesFollowTheLayout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		selfHosted bool
		homes      []string
	}{
		{
			name:       "self-hosted",
			selfHosted: true,
			homes: []string{
				"metasystem/docs/intent", "metasystem/docs/doctrine", "metasystem/docs/decisions",
				"metasystem/plans/designs", "plans/designs", "metasystem/memory/questions.md",
			},
		},
		{
			name: "adopted vendored beneath the application",
			homes: []string{
				"docs/intent", "docs/doctrine", "docs/decisions",
				"plans/designs", "memory/questions.md",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			f := newFixture(t, test.selfHosted)
			f.seed()
			read := f.read()

			testutil.Expect(t, "the homes", homeRels(read.Homes), test.homes)
			testutil.Expect(t, "every record the homes declare", sorted(ids(read.Records)), []string{
				"decision-one-binary", "decision-two-homes",
				"design-interface", "design-ledger", "design-reading",
				"doctrine-budgets", "doctrine-events", "doctrine-index", "intent-index",
			})
			testutil.Expect(t, "the refusals a seeded project carries", problemLines(read.Problems), []string{})
			testutil.Expect(t, "the declared areas", []string{read.Areas[0].Slug, read.Areas[1].Slug},
				[]string{"billing", "security"})
			testutil.Expect(t, "the area names", []string{read.Areas[0].Name, read.Areas[1].Name},
				[]string{"Billing and invoicing", "Security and identity"})
			testutil.Expect(t, "the questions", len(read.Questions), 3)
		})
	}
}

// A book's index carries the reading order, and a chapter is a record anywhere
// beneath the book or a document bound where it already lives.
func TestBooksCarryTheirReadingOrder(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()

	testutil.Require(t, "the books read", len(read.Books), 2)
	intent, doctrine := read.Books[0], read.Books[1]
	testutil.Expect(t, "the intent book's kind", intent.Kind, KindIntent)
	testutil.Expect(t, "a one-page book has no chapters", intent.Chapters, []Chapter(nil))
	testutil.Expect(t, "the intent index's id", intent.Index.ID, "intent-index")
	testutil.Expect(t, "the doctrine book's kind", doctrine.Kind, KindDoctrine)
	testutil.Expect(t, "the doctrine book declares no areas", doctrine.Areas, []Area(nil))
	testutil.Expect(t, "the reading order", []string{
		doctrine.Chapters[0].ID, doctrine.Chapters[1].ID, doctrine.Chapters[2].Doc,
	}, []string{"doctrine-events", "doctrine-budgets", "metasystem/docs/architecture.md"})
	testutil.Expect(t, "the bound document's title", doctrine.Chapters[2].Title, "The engine")
	testutil.Expect(t, "a chapter in a subdirectory",
		read.Record("doctrine-budgets").Path, "metasystem/docs/doctrine/chapters/budgets.md")
}

// A document with no head is not a record: the rest of the checkout's Markdown
// stays browsable by path, with no kind claimed and no refusal raised.
func TestADocumentWithNoHeadIsNotARecord(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	// A bullet list that is prose, not a head: the key is not a word and a colon.
	f.write(f.state+"docs/decisions/notes.md", "# Notes\n\n- `Id` is any string\n- and this is prose\n")
	read := f.read()

	testutil.Expect(t, "the headless documents are not records",
		contains(pathsOf(read.Records), "metasystem/docs/decisions/README.md") ||
			contains(pathsOf(read.Records), "metasystem/docs/decisions/notes.md"), false)
	testutil.Expect(t, "the records that are there", len(read.Records), 9)
	testutil.Expect(t, "the refusals a headless document raises", problemLines(read.Problems), []string{})
}

// list answers by kind, and narrows by area and by status.
func TestListNarrowsByAreaAndStatus(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()

	testutil.Expect(t, "every design, in path order", ids(read.List(KindDesign, ListOptions{})),
		[]string{"design-ledger", "design-reading", "design-interface"})
	testutil.Expect(t, "the designs of one area", ids(read.List(KindDesign, ListOptions{Area: "billing"})),
		[]string{"design-ledger", "design-interface"})
	testutil.Expect(t, "the designs of one status", ids(read.List(KindDesign, ListOptions{Status: StatusDraft})),
		[]string{"design-reading"})
	testutil.Expect(t, "an area and a status together",
		ids(read.List(KindDesign, ListOptions{Area: "billing", Status: StatusAccepted})),
		[]string{"design-interface"})
	testutil.Expect(t, "the doctrine", ids(read.List(KindDoctrine, ListOptions{})),
		[]string{"doctrine-budgets", "doctrine-events", "doctrine-index"})
	testutil.Expect(t, "a narrowing nothing answers",
		ids(read.List(KindIntent, ListOptions{Status: StatusDone})), []string{})
	testutil.Expect(t, "the designs' paths", pathsOf(read.List(KindDesign, ListOptions{})), []string{
		"metasystem/plans/designs/ledger.md",
		"metasystem/plans/designs/pane/reading.md",
		"plans/designs/interface.md",
	})
}

// show's other half: a record cannot declare what names it, so the reader walks
// Cites, Affects and Supersedes to find out. Governs names goal ids and By names
// whoever accepted the record, so neither makes one record reference another.
func TestReferencedByReadsTheOtherHalf(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()

	testutil.Expect(t, "what references the first decision", read.ReferencedBy("decision-one-binary"),
		[]Reference{
			{ID: "decision-two-homes", Path: "metasystem/docs/decisions/archive/0002-two-homes.md", Key: "Supersedes"},
			{ID: "design-ledger", Path: "metasystem/plans/designs/ledger.md", Key: "Affects"},
		})
	testutil.Expect(t, "what references the ledger design", read.ReferencedBy("design-ledger"),
		[]Reference{{ID: "design-interface", Path: "plans/designs/interface.md", Key: "Cites"}})
	testutil.Expect(t, "a goal id is not a record reference", read.ReferencedBy("goal-17"), []Reference(nil))
	testutil.Expect(t, "a person is not a record reference", read.ReferencedBy("wido"), []Reference(nil))
	testutil.Expect(t, "the references a record declares",
		read.Record("design-ledger").References("Governs"), []string{"goal-17"})
}

// The tree counts a record under every area it names, and keeps the project
// bucket for what belongs to the whole rather than to a part.
func TestTreeCountsByKindAndStatus(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()
	areas, whole := read.Tree()

	testutil.Require(t, "the declared areas counted", len(areas), 2)
	testutil.Expect(t, "the first area", areas[0].Area.Slug, "billing")
	testutil.Expect(t, "billing by kind", areas[0].Counts.Kind,
		map[string]int{KindIntent: 0, KindDoctrine: 1, KindDecision: 1, KindDesign: 2})
	testutil.Expect(t, "billing by status", areas[0].Counts.Status,
		map[string]int{StatusDraft: 0, StatusAccepted: 2, StatusSuperseded: 1, StatusDone: 1})
	testutil.Expect(t, "billing in all", areas[0].Counts.Total, 4)
	testutil.Expect(t, "the second area", areas[1].Area.Slug, "security")
	testutil.Expect(t, "security by kind", areas[1].Counts.Kind,
		map[string]int{KindIntent: 0, KindDoctrine: 1, KindDecision: 0, KindDesign: 1})
	testutil.Expect(t, "security by status", areas[1].Counts.Status,
		map[string]int{StatusDraft: 2, StatusAccepted: 0, StatusSuperseded: 0, StatusDone: 0})
	testutil.Expect(t, "the project bucket by kind", whole.Kind,
		map[string]int{KindIntent: 1, KindDoctrine: 2, KindDecision: 1, KindDesign: 1})
	testutil.Expect(t, "the project bucket by status", whole.Status,
		map[string]int{StatusDraft: 1, StatusAccepted: 4, StatusSuperseded: 0, StatusDone: 0})
	testutil.Expect(t, "the project bucket in all", whole.Total, 5)
}

// The register is read as it is written: three rows, three statuses, and an
// answer that names what answered it.
func TestRegisterReadsItsThreeStatuses(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	read := f.read()

	testutil.Require(t, "the rows read", len(read.Questions), 3)
	testutil.Expect(t, "the ids", []string{read.Questions[0].ID, read.Questions[1].ID, read.Questions[2].ID},
		[]string{"Q-1", "Q-2", "Q-3"})
	testutil.Expect(t, "the statuses",
		[]string{read.Questions[0].Status, read.Questions[1].Status, read.Questions[2].Status},
		[]string{"open", "answered: decision-one-binary", "withdrawn"})
	testutil.Expect(t, "the areas a row names", read.Questions[1].Areas, []string{"billing"})
	testutil.Expect(t, "the question itself", read.Questions[0].Text,
		"Where does an adopted application's intent live?")
	testutil.Expect(t, "the line a row sits on", read.Questions[0].Line, 5)
}

// A head that is not there and a head that is: the unknown keys are kept, and
// the declared ones are read.
func TestHeadKeepsUnknownKeysAndReadsTheRest(t *testing.T) {
	t.Parallel()

	f := newFixture(t, true)
	f.seed()
	f.write(f.state+"docs/decisions/0003-revised.md",
		record("A revised decision", "decision", "decision-revised", "draft", "project",
			"- Revision: 3, because the first two were wrong", "- Cites: intent-index decision-one-binary"))
	read := f.read()

	revised := read.Record("decision-revised")
	testutil.Require(t, "the revised decision read", revised != nil, true)
	testutil.Expect(t, "the unknown key is kept", revised.Head[4],
		Field{Key: "Revision", Value: "3, because the first two were wrong", Line: 7})
	testutil.Expect(t, "two references on one line", revised.Cites,
		[]string{"intent-index", "decision-one-binary"})
	testutil.Expect(t, "an unknown key is no refusal", problemLines(read.Problems), []string{})
	testutil.Expect(t, "the home a record was found in", revised.Home, "metasystem/docs/decisions")
	testutil.Expect(t, "the title", revised.Title, "A revised decision")
}

// An id is any non-empty string, and minting one is the engine's own ULID.
func TestNewIDMintsTheEnginesULIDShape(t *testing.T) {
	t.Parallel()

	first, err := NewID()
	testutil.Require(t, "mint an id", err, nil)
	second, err := NewID()
	testutil.Require(t, "mint a second id", err, nil)

	testutil.Expect(t, "the length of a minted id", len(first), 26)
	testutil.Expect(t, "two minted ids differ", first == second, false)
	testutil.Expect(t, "a minted id is Crockford base32 throughout",
		strings.Trim(first, "0123456789ABCDEFGHJKMNPQRSTVWXYZ"), "")
}

func pathsOf(records []Record) []string {
	collected := make([]string, 0, len(records))
	for _, record := range records {
		collected = append(collected, record.Path)
	}
	return collected
}
