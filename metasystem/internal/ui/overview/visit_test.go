package overview

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The visit rule, which is the one piece of state this section keeps.

// noon is the instant the visits below are measured from.
var noon = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)

// A human this checkout has never served looks back a day, and the page is
// told so: there is no previous visit to name, and an instant nobody was
// there for is not one to put on the screen.
func TestAFirstVisitLooksBackADayAndSaysSo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	since, first, err := Visit(root, "Wido", noon)

	testutil.Require(t, "recording the visit", err, nil)
	testutil.Expect(t, "the window", since, noon.Add(-24*time.Hour))
	testutil.Expect(t, "that it is a first visit", first, true)
}

// A visit is a span of looking. Every read inside it compares against the same
// instant, so pressing Refresh never hides what a human has not read yet.
func TestReadingAgainInsideAVisitNeverNarrowsTheWindow(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	// The visit before this one: one read, then a gap, then another, which
	// ends the first visit at the instant of its own last read.
	_, _, err := Visit(root, "Wido", noon.Add(-4*time.Hour))
	testutil.Require(t, "the first visit", err, nil)
	ended := noon.Add(-3 * time.Hour)
	_, _, err = Visit(root, "Wido", ended)
	testutil.Require(t, "still the first visit", err, nil)

	opened, first, err := Visit(root, "Wido", noon)
	testutil.Require(t, "the second visit", err, nil)
	testutil.Expect(t, "it is not a first visit", first, false)
	testutil.Expect(t, "the window is the end of the previous visit", opened, ended)

	// Ten minutes later, still looking: the same window, to the second.
	again, stillNotFirst, err := Visit(root, "Wido", noon.Add(10*time.Minute))
	testutil.Require(t, "reading again", err, nil)
	testutil.Expect(t, "the window has not moved", again, opened)
	testutil.Expect(t, "and it is still not a first visit", stillNotFirst, false)
}

// A first visit does not narrow either: the day is measured from when the
// visit began, not from each read inside it.
func TestAFirstVisitsDayIsMeasuredFromWhenItBegan(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	opened, _, err := Visit(root, "Wido", noon)
	testutil.Require(t, "the first read", err, nil)
	again, first, err := Visit(root, "Wido", noon.Add(20*time.Minute))

	testutil.Require(t, "the second read", err, nil)
	testutil.Expect(t, "the window has not moved", again, opened)
	testutil.Expect(t, "and it is still the first visit", first, true)
}

// The gap is what ends a visit, and what it ends at is the last read rather
// than now: nobody was looking in between.
func TestAGapBeginsANewVisitEndingAtTheLastRead(t *testing.T) {
	t.Parallel()

	for _, one := range []struct {
		name     string
		gap      time.Duration
		newVisit bool
	}{
		{"just inside the gap", 29 * time.Minute, false},
		{"exactly the gap", 30 * time.Minute, false},
		{"past the gap", 31 * time.Minute, true},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			// Two reads a long way apart, so the visit in progress has a
			// previous visit to compare against and the window is an instant
			// rather than a day back.
			_, _, err := Visit(root, "Wido", noon.Add(-8*time.Hour))
			testutil.Require(t, "the first visit", err, nil)
			ended := noon.Add(-7 * time.Hour)
			_, _, err = Visit(root, "Wido", ended)
			testutil.Require(t, "and its last read", err, nil)
			opened, _, err := Visit(root, "Wido", noon)
			testutil.Require(t, "the visit under test", err, nil)
			testutil.Require(t, "its window", opened, ended)

			after, _, err := Visit(root, "Wido", noon.Add(one.gap))

			testutil.Require(t, "reading again", err, nil)
			if one.newVisit {
				testutil.Expect(t, "the window is the read that ended the visit", after, noon)
				return
			}
			testutil.Expect(t, "the window has not moved", after, opened)
		})
	}
}

// One row per human. Two people looking at one checkout each have their own
// window, and a seat that names nobody is the row under the empty handle.
func TestTheMarkerIsKeptPerHuman(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	_, _, err := Visit(root, "Wido", noon.Add(-4*time.Hour))
	testutil.Require(t, "Wido's first visit", err, nil)
	_, _, err = Visit(root, "Wido", noon.Add(-4*time.Hour+10*time.Minute))
	testutil.Require(t, "Wido's second read, inside the same visit", err, nil)

	_, first, err := Visit(root, "Ada", noon)
	testutil.Require(t, "Ada's first visit", err, nil)
	testutil.Expect(t, "Ada has not been here", first, true)

	_, seat, err := Visit(root, "", noon)
	testutil.Require(t, "the seat itself", err, nil)
	testutil.Expect(t, "nor has the seat", seat, true)

	held := readFile(t, root)
	testutil.Expect(t, "how many rows the file keeps", len(held.Humans), 3)
	testutil.Expect(t, "the schema it is written under", held.SchemaVersion, visitsSchema)
	testutil.Expect(t, "Wido's visit is still the one it was",
		held.Humans["Wido"].Began, stamp(noon.Add(-4*time.Hour)))
}

// The marker lives beside the interface's other lifecycle state.
func TestTheMarkerSitsBesideTheSessionsFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()

	testutil.Expect(t, "where the marker is written", VisitsPath(root),
		filepath.Join(root, "artifacts", "agents", "ui", "visits.json"))
}

// Losing the file changes the comparison window and nothing else, so a file
// this build cannot read is a first visit with the reason beside it rather
// than a page that will not render.
func TestALostMarkerIsAFirstVisitWithItsReason(t *testing.T) {
	t.Parallel()

	for _, one := range []struct{ name, written string }{
		{"not JSON at all", "{{{"},
		{"a schema this build does not read", `{"schemaVersion":99,"humans":{}}`},
	} {
		t.Run(one.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			testutil.Require(t, "making the directory",
				os.MkdirAll(filepath.Dir(VisitsPath(root)), 0o755), nil)
			testutil.Require(t, "planting the file",
				os.WriteFile(VisitsPath(root), []byte(one.written), 0o644), nil)

			since, first, err := Visit(root, "Wido", noon)

			testutil.Expect(t, "the window is a first visit's", since, noon.Add(-24*time.Hour))
			testutil.Expect(t, "and says so", first, true)
			testutil.Expect(t, "the reason is carried", err != nil, true)
			// And the unreadable file is replaced, so the next read is
			// ordinary rather than a first visit again.
			held := readFile(t, root)
			testutil.Expect(t, "the file was rewritten", held.SchemaVersion, visitsSchema)
		})
	}
}

// A row whose instants this build cannot read is a row that says nothing, and
// nothing is a first visit rather than a window computed from a stamp nobody
// can parse.
func TestARowWithAnUnreadableStampIsAFirstVisit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	testutil.Require(t, "making the directory",
		os.MkdirAll(filepath.Dir(VisitsPath(root)), 0o755), nil)
	testutil.Require(t, "planting the row", os.WriteFile(VisitsPath(root),
		[]byte(`{"schemaVersion":1,"humans":{"Wido":{"began":"yesterday","seen":"yesterday","previous":""}}}`),
		0o644), nil)

	since, first, err := Visit(root, "Wido", noon)

	testutil.Require(t, "recording the visit", err, nil)
	testutil.Expect(t, "the window", since, noon.Add(-24*time.Hour))
	testutil.Expect(t, "and that it is a first visit", first, true)
}

func readFile(t *testing.T, root string) Visits {
	t.Helper()
	data, err := os.ReadFile(VisitsPath(root))
	testutil.Require(t, "reading the marker", err, nil)
	var held Visits
	testutil.Require(t, "decoding the marker", json.Unmarshal(data, &held), nil)
	return held
}
