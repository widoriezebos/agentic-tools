package knownissues

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The kit's own column set, and one row of every shape the reader has to tell
// apart: an open row, one of each concluded status word, a status that opens
// with a word that only looks like one of them, a cell holding an escaped
// pipe, and two rows with the wrong number of cells.
//
// Every test below reads one of the two registers here, so a rule that answers
// one shape by breaking another fails in this file.
const kitHeader = "| Id | Date | Symptom and evidence | Cost when it bites | Fix direction or lever | Status |"

var kitRows = []string{
	"| KI-1 | 2026-08-04 | The census costs 475ms on a busy laptop | A scan can outrun its own interval | Batch the cwd resolution | FIXED 2026-08-06: one lsof for every candidate |",
	"| KI-2 | 2026-08-05 | The suite's wall time grew from 2m14s to 4m38s | CI cost, and a suite nobody runs locally | Profile the fixtures and split fast from slow | ACCEPTED 2026-08-06 with a measured trigger |",
	"| KI-3 | 2026-08-06 | A fixture timed out once under load | A flaky gate erodes trust in the suite | Make the arming wait proportional to the census | RETIRED 2026-08-06: the load-immunity rework addressed the cause |",
	"| KI-4 | 2026-08-07 | The register reader skipped rows with an escaped pipe | A page claims fewer defects than the project has | Split on unescaped pipes only | RESOLVED 2026-08-08 |",
	"| KI-5 | 2026-08-08 | The watcher logged every tick at info level | Log noise nobody reads | Move it to debug | CLOSED 2026-08-09 |",
	"| KI-6 | 2026-08-09 | Evidence mirroring fails silently: every call site swallows it with `\\|\\| true` | The only copy of paid evidence can vanish | Surface a failed mirror as a watcher condition | OPEN |",
	"| KI-7 | 2026-08-10 | The fingerprint covers the CLI's own self-written state | Dispatch demands a fresh probe for nothing | Hash a filtered view | FIX SHIPPED, PROOF INCOMPLETE 2026-08-11 |",
	"| KI-8 | The follow-up rounds review a stale tree | OPEN |",
	"| KI-9 | 2026-08-12 | Two mains in one working tree | OPEN |",
}

// The column set an adoption ships, whose fifth column is a reopen condition
// rather than a fix direction.
const adoptedHeader = "| Id | Date | Issue | Consequence | Reopen when | Status |"

var adoptedRows = []string{
	"| A-1 | 2026-09-01 | The importer takes one file at a time | A bulk import is a morning of clicking | A customer asks for more than fifty | OPEN |",
	"| A-2 | 2026-09-02 | The nightly export runs in the wrong zone | A day's rows land on the wrong date | Never: the exporter was rewritten | FIXED 2026-09-03 |",
}

func write(t *testing.T, header string, rows []string) string {
	t.Helper()
	root := t.TempDir()
	path := Path(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# Known Issues\n\nA standing register of defects and limitations.\n\n" +
		header + "\n| --- | --- | --- | --- | --- | --- |\n" +
		strings.Join(rows, "\n") + "\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func read(t *testing.T, header string, rows []string) Register {
	t.Helper()
	register, err := Read(write(t, header, rows))
	if err != nil {
		t.Fatal(err)
	}
	return register
}

func ids(rows []Row) []string {
	found := []string{}
	for _, row := range rows {
		found = append(found, row.ID)
	}
	return found
}

func TestReadTakesTheHeaderRowAsTheNamesOfItsColumns(t *testing.T) {
	t.Parallel()
	register := read(t, kitHeader, kitRows)
	testutil.Expect(t, "the kit's own column names", register.Columns,
		[]string{"Id", "Date", "Symptom and evidence", "Cost when it bites", "Fix direction or lever", "Status"})
}

// The fifth column's title is the one that differs between the two sets, and
// it is shown as supplied rather than renamed to the kit's word.
func TestReadKeepsTheAdoptedColumnNamesAsSupplied(t *testing.T) {
	t.Parallel()
	register := read(t, adoptedHeader, adoptedRows)
	testutil.Expect(t, "the adopted column names", register.Columns,
		[]string{"Id", "Date", "Issue", "Consequence", "Reopen when", "Status"})
}

// Either set is read BY POSITION: the fifth cell is the fifth cell, whatever
// the header called it.
func TestReadReadsTheAdoptedColumnSetByPosition(t *testing.T) {
	t.Parallel()
	register := read(t, adoptedHeader, adoptedRows)
	testutil.Expect(t, "the open row", register.Open, []Row{{
		ID: "A-1", Date: "2026-09-01",
		What:        "The importer takes one file at a time",
		Consequence: "A bulk import is a morning of clicking",
		Lever:       "A customer asks for more than fifty",
		Status:      "OPEN", Open: true,
	}})
	testutil.Expect(t, "the concluded row's lever", register.Concluded[0].Lever,
		"Never: the exporter was rewritten")
}

func TestReadConcludesEveryStatusWordAndLeavesTheRestOpen(t *testing.T) {
	t.Parallel()
	register := read(t, kitHeader, kitRows)
	// FIXED, ACCEPTED, RETIRED, RESOLVED and CLOSED conclude. An accepted
	// limitation still exists, which is why its row is concluded rather than
	// gone and why the word stays on it.
	testutil.Expect(t, "the concluded rows", ids(register.Concluded),
		[]string{"KI-1", "KI-2", "KI-3", "KI-4", "KI-5"})
	// KI-6 is OPEN. KI-7 opens with "FIX SHIPPED", which is not FIXED, and a
	// reader that matched a prefix without a word boundary would conclude a
	// row the project says is unproven.
	testutil.Expect(t, "the open rows", ids(register.Open), []string{"KI-6", "KI-7"})
}

func TestReadKeepsTheStatusWordVisibleOnAConcludedRow(t *testing.T) {
	t.Parallel()
	register := read(t, kitHeader, kitRows)
	testutil.Expect(t, "the accepted row's status", register.Concluded[1].Status,
		"ACCEPTED 2026-08-06 with a measured trigger")
}

// The one cell in this register that holds a pipe. A reader that split on
// every pipe would read this row as eight columns and refuse it.
func TestReadSplitsOnUnescapedPipesOnly(t *testing.T) {
	t.Parallel()
	register := read(t, kitHeader, kitRows)
	testutil.Expect(t, "the symptom with the escaped pipe", register.Open[0].What,
		"Evidence mirroring fails silently: every call site swallows it with `|| true`")
}

// The five rows this register carries today with three or four cells are the
// reason this line exists: a block that dropped them would say the project
// knows about fewer problems than it does.
func TestReadCountsAndNamesTheRowsItCouldNotRead(t *testing.T) {
	t.Parallel()
	register := read(t, kitHeader, kitRows)
	testutil.Expect(t, "how many could not be read", register.Unread, 2)
	testutil.Expect(t, "what they were", register.Defects, []string{
		"row=8: wrong column count: got 3, want 6",
		"row=9: wrong column count: got 4, want 6",
	})
}

func TestReadAnswersAnEmptyRegisterForACheckoutWithNone(t *testing.T) {
	t.Parallel()
	register, err := Read(t.TempDir())
	if err != nil {
		t.Fatalf("a checkout with no register is not a failure: %v", err)
	}
	testutil.Expect(t, "the register", register,
		Register{Columns: []string{}, Open: []Row{}, Concluded: []Row{}, Defects: []string{}})
}

// A header of another width is a file this reader cannot place columns in, and
// it says so rather than reading five cells as six.
func TestReadRefusesToPlaceColumnsUnderAHeaderOfAnotherWidth(t *testing.T) {
	t.Parallel()
	register := read(t, "| Id | Date | Issue | Status |", []string{"| X-1 | 2026-09-01 | Something | OPEN |"})
	testutil.Expect(t, "the rows it read", register.Open, []Row{})
	testutil.Expect(t, "what it said", register.Defects,
		[]string{"header: wrong column count: got 4, want 6"})
}

func TestConcludedJudgesTheOpeningWordAndNothingElse(t *testing.T) {
	t.Parallel()
	for _, one := range []struct {
		status string
		want   bool
	}{
		{"FIXED", true},
		{"FIXED 2026-08-06: the reader now walks the tree", true},
		{"RESOLVED", true},
		{"RETIRED 2026-08-06", true},
		{"CLOSED, superseded by KI-40", true},
		{"ACCEPTED 2026-08-06 with a measured trigger", true},
		{"  FIXED 2026-08-06", true},
		{"OPEN", false},
		{"2026-08-06: still being read", false},
		{"FIX SHIPPED, PROOF INCOMPLETE", false},
		{"FIXEDLY nothing", false},
		{"", false},
	} {
		if got := Concluded(one.status); got != one.want {
			t.Errorf("Concluded(%q) = %v, want %v", one.status, got, one.want)
		}
	}
}

func TestCellsSplitsARowTheWayTheRegisterWritesOne(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "a plain row", Cells("| a | b | c |"), []string{"a", "b", "c"})
	testutil.Expect(t, "a row holding an escaped pipe", Cells(`| a | b \| c | d |`),
		[]string{"a", "b | c", "d"})
	testutil.Expect(t, "a row that forgot its closing pipe", Cells("| a | b"), []string{"a", "b"})
	testutil.Expect(t, "a backslash that escapes nothing", Cells(`| a\b | c |`), []string{`a\b`, "c"})
	testutil.Expect(t, "a line that is not a row", Cells("not a row"), []string(nil))
}
