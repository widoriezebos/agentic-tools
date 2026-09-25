package stickies

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The notepad: what it writes, what order it answers in, what it refuses, and
// what it keeps apart.
//
// Every test here owns its own home under t.TempDir(). The real account's home
// is never written, and never read either: the store resolves its home through
// the registry's own fixture seam, which these tests redirect.

// noon is the instant the stickies below are written at.
var noon = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

// at is a clock that answers a chosen instant, and moves only when a test
// moves it. No test here waits for a real second to pass.
type at struct{ now time.Time }

func (c *at) read() time.Time { return c.now }

// fixture is a notepad under a home of this test's own, with a clock the test
// holds and ids that say which sticky they are.
func fixture(t *testing.T) (*Store, *at) {
	t.Helper()
	clock := &at{now: noon}
	store := New(filepath.Join(t.TempDir(), ".metasystem"), "/tmp/workspaces/example", clock.read)
	minted := 0
	store.mint = func() (string, error) {
		minted++
		return "STICKY" + string(rune('0'+minted)), nil
	}
	return store, clock
}

func texts(list List) []string {
	said := make([]string, 0, len(list.Stickies))
	for _, row := range list.Stickies {
		said = append(said, row.Text)
	}
	return said
}

// A notepad nobody has written in is a notepad with nothing in it, not a
// failure: the file is the whole truth, and there is no file yet.
func TestAMissingFileIsNoStickies(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)

	list, err := store.List("Wido")

	testutil.Require(t, "reading a notepad nobody has written in", err, nil)
	testutil.Expect(t, "the stickies", len(list.Stickies), 0)
	testutil.Expect(t, "the counts", list.Counts, Counts{})
	testutil.Expect(t, "who it is for", list.Human, "Wido")
	testutil.Expect(t, "the schema", list.SchemaVersion, SchemaVersion)
	_, err = os.Stat(store.File())
	testutil.Expect(t, "that reading wrote nothing", os.IsNotExist(err), true)
}

// Writing one answers the whole list, because every act changes what the panel
// shows: the sticky, its instants, its about, and the counts the badge reads.
func TestWritingOneAnswersTheWholeList(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)

	list, err := store.Add("Wido", "  ask Sol about the retry  ",
		[]About{{Kind: KindGoal, ID: "g1-s45"}})

	testutil.Require(t, "writing a sticky", err, nil)
	testutil.Expect(t, "how many", len(list.Stickies), 1)
	testutil.Expect(t, "the text, trimmed", list.Stickies[0].Text, "ask Sol about the retry")
	testutil.Expect(t, "its id", list.Stickies[0].ID, "STICKY1")
	testutil.Expect(t, "what it is about", list.Stickies[0].About, []About{{Kind: KindGoal, ID: "g1-s45"}})
	testutil.Expect(t, "when it was written", list.Stickies[0].CreatedAt, "2026-09-25T12:00:00Z")
	testutil.Expect(t, "when it was touched", list.Stickies[0].UpdatedAt, "2026-09-25T12:00:00Z")
	testutil.Expect(t, "that it is open", list.Stickies[0].DoneAt, "")
	testutil.Expect(t, "the counts", list.Counts, Counts{Open: 1})
}

// A sticky survives the process that wrote it, which is the whole of J5: the
// file is read back by a second store over the same home.
func TestAStickySurvivesTheStoreThatWroteIt(t *testing.T) {
	t.Parallel()

	home := filepath.Join(t.TempDir(), ".metasystem")
	clock := &at{now: noon}
	first := New(home, "/tmp/workspaces/example", clock.read)
	_, err := first.Add("Wido", "check g1-s45 tomorrow", nil)
	testutil.Require(t, "writing a sticky", err, nil)

	list, err := New(home, "/tmp/workspaces/example", clock.read).List("Wido")

	testutil.Require(t, "reading it back", err, nil)
	testutil.Expect(t, "what was kept", texts(list), []string{"check g1-s45 tomorrow"})
}

// Open stickies are newest first and done ones are ordered by when they were
// struck off, so the panel's order is decided once, here, and not in three
// places in the browser.
func TestOpenAreNewestFirstAndDoneAreOrderedByWhenTheyWereStruckOff(t *testing.T) {
	t.Parallel()

	store, clock := fixture(t)
	for _, said := range []string{"first", "second", "third"} {
		clock.now = clock.now.Add(time.Minute)
		_, err := store.Add("Wido", said, nil)
		testutil.Require(t, "writing "+said, err, nil)
	}
	// The oldest is struck off last, so its place among the done ones is not
	// the place it had among the open ones.
	clock.now = clock.now.Add(time.Minute)
	done := true
	_, err := store.Edit("Wido", "STICKY2", nil, nil, &done)
	testutil.Require(t, "striking off the second", err, nil)
	clock.now = clock.now.Add(time.Minute)
	_, err = store.Edit("Wido", "STICKY1", nil, nil, &done)
	testutil.Require(t, "striking off the first", err, nil)

	list, err := store.List("Wido")

	testutil.Require(t, "reading the notepad", err, nil)
	// The one struck off last leads the done ones, which is not the place it
	// had among the open ones.
	testutil.Expect(t, "the order", texts(list), []string{"third", "first", "second"})
	testutil.Expect(t, "the counts", list.Counts, Counts{Open: 1, Done: 2})
}

// Editing changes only what the request carried. A field nobody sent is a
// field nobody changed, which is why the three are pointers.
func TestEditingChangesOnlyWhatWasSent(t *testing.T) {
	t.Parallel()

	store, clock := fixture(t)
	_, err := store.Add("Wido", "the launch sheet's wording is off",
		[]About{{Kind: KindRecord, ID: "plans/designs/launch.md"}})
	testutil.Require(t, "writing a sticky", err, nil)
	clock.now = clock.now.Add(time.Hour)

	text := "the launch sheet's wording is still off"
	list, err := store.Edit("Wido", "STICKY1", &text, nil, nil)

	testutil.Require(t, "editing the sticky", err, nil)
	testutil.Expect(t, "the text", list.Stickies[0].Text, text)
	testutil.Expect(t, "what it is still about", list.Stickies[0].About,
		[]About{{Kind: KindRecord, ID: "plans/designs/launch.md"}})
	testutil.Expect(t, "when it was written", list.Stickies[0].CreatedAt, "2026-09-25T12:00:00Z")
	testutil.Expect(t, "when it was touched", list.Stickies[0].UpdatedAt, "2026-09-25T13:00:00Z")
}

// Done is a state a human can take back, and taking it back puts the sticky
// among the open ones again. Marking one done twice leaves the instant it was
// struck off where it was, so the done list does not reorder under them.
func TestDoneCanBeTakenBackAndIsNotReDatedByASecondMarking(t *testing.T) {
	t.Parallel()

	store, clock := fixture(t)
	_, err := store.Add("Wido", "ask Sol about the retry", nil)
	testutil.Require(t, "writing a sticky", err, nil)
	clock.now = clock.now.Add(time.Hour)
	done, open := true, false
	list, err := store.Edit("Wido", "STICKY1", nil, nil, &done)
	testutil.Require(t, "striking it off", err, nil)
	testutil.Expect(t, "when it was struck off", list.Stickies[0].DoneAt, "2026-09-25T13:00:00Z")
	testutil.Expect(t, "the counts once it is done", list.Counts, Counts{Done: 1})

	clock.now = clock.now.Add(time.Hour)
	list, err = store.Edit("Wido", "STICKY1", nil, nil, &done)
	testutil.Require(t, "striking it off again", err, nil)
	testutil.Expect(t, "that the instant did not move", list.Stickies[0].DoneAt, "2026-09-25T13:00:00Z")

	list, err = store.Edit("Wido", "STICKY1", nil, nil, &open)
	testutil.Require(t, "putting it back", err, nil)
	testutil.Expect(t, "that it is open again", list.Stickies[0].DoneAt, "")
	testutil.Expect(t, "the counts once it is open again", list.Counts, Counts{Open: 1})
}

// Removing takes one sticky and leaves the rest.
func TestRemovingTakesOneAndLeavesTheRest(t *testing.T) {
	t.Parallel()

	store, clock := fixture(t)
	for _, said := range []string{"first", "second"} {
		clock.now = clock.now.Add(time.Minute)
		_, err := store.Add("Wido", said, nil)
		testutil.Require(t, "writing "+said, err, nil)
	}

	list, err := store.Remove("Wido", "STICKY2")

	testutil.Require(t, "removing a sticky", err, nil)
	testutil.Expect(t, "what is left", texts(list), []string{"first"})
}

// An id this human has no sticky under is absent, whichever act names it, and
// the refusal says which id it was.
func TestAnIdThisHumanHasNoStickyUnderIsAbsent(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	_, err := store.Add("Wido", "mine", nil)
	testutil.Require(t, "writing a sticky", err, nil)
	// One human's id is another human's nothing, which is what keyed by human
	// means for the acts as well as for the reads.
	for _, act := range []struct {
		what string
		run  func() (List, error)
	}{
		{"editing", func() (List, error) { return store.Edit("Sol", "STICKY1", nil, nil, nil) }},
		{"removing", func() (List, error) { return store.Remove("Sol", "STICKY1") }},
		{"editing one nobody wrote", func() (List, error) { return store.Edit("Wido", "STICKY9", nil, nil, nil) }},
	} {
		_, err := act.run()
		refusal := refusalOf(t, act.what, err)
		testutil.Expect(t, act.what+" is refused as absent", refusal.Kind, RefusalAbsent)
		testutil.Expect(t, act.what+" names the id", strings.Contains(refusal.Message, "STICKY"), true)
	}
}

// The text bound, refused with the words a human reads rather than a code.
func TestTextPastTheBoundIsRefusedWithWords(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)

	_, err := store.Add("Wido", strings.Repeat("a", MaxText+1), nil)

	refusal := refusalOf(t, "writing too much", err)
	testutil.Expect(t, "the shape", refusal.Kind, RefusalBounds)
	testutil.Expect(t, "that it says the bound", strings.Contains(refusal.Message, "2000 characters"), true)
	testutil.Expect(t, "that it says what was sent", strings.Contains(refusal.Message, "2001"), true)
	// What is exactly the bound is admitted: a bound refuses what is past it.
	_, err = store.Add("Wido", strings.Repeat("a", MaxText), nil)
	testutil.Require(t, "writing exactly the bound", err, nil)
	// And the bound is on characters, not bytes, because that is what a human
	// was told: a sticky of 2000 letters that are three bytes each is one
	// sticky of 2000 characters.
	_, err = store.Add("Wido", strings.Repeat("é", MaxText), nil)
	testutil.Require(t, "writing the bound in letters that are not one byte", err, nil)
}

// The count bound, refused with the words a human reads. It is a bound on the
// notepad and not on the open ones: five hundred struck-off stickies are still
// five hundred stickies.
func TestMoreStickiesThanTheNotepadKeepsIsRefusedWithWords(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	store.mint = mintCounted()
	for count := 0; count < MaxStickies; count++ {
		if _, err := store.Add("Wido", "one of many", nil); err != nil {
			t.Fatalf("writing sticky %d: %v", count, err)
		}
	}

	_, err := store.Add("Wido", "one too many", nil)

	refusal := refusalOf(t, "writing past the bound", err)
	testutil.Expect(t, "the shape", refusal.Kind, RefusalBounds)
	testutil.Expect(t, "that it says the bound", strings.Contains(refusal.Message, "500 stickies"), true)
	// Another human's notepad is not full, because the bound is per human.
	_, err = store.Add("Sol", "mine", nil)
	testutil.Require(t, "another human writing their first", err, nil)
}

// An empty sticky and an about this build cannot render are both refused, with
// the sentence saying which it was.
func TestAnEmptyStickyAndAnUnknownAboutAreRefused(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	for _, refused := range []struct {
		what  string
		text  string
		about []About
		says  string
	}{
		{"nothing at all", "   ", nil, "is empty"},
		{"an unknown kind", "something", []About{{Kind: "machine", ID: "m1a"}}, `not a "machine"`},
		{"no kind at all", "something", []About{{ID: "g1-s45"}}, "thing with no kind at all"},
		{"a kind naming nothing", "something", []About{{Kind: KindGoal, ID: "  "}}, "says which one"},
	} {
		_, err := store.Add("Wido", refused.text, refused.about)
		refusal := refusalOf(t, refused.what, err)
		testutil.Expect(t, refused.what+" is a bad request", refusal.Kind, RefusalBad)
		testutil.Expect(t, refused.what+" says why",
			strings.Contains(refusal.Message, refused.says), true)
	}
}

// The same thing named twice is one chip: a human who pressed the page's chip
// and then picked the same goal meant one of them.
func TestTheSameThingNamedTwiceIsOneChip(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)

	list, err := store.Add("Wido", "one goal, named twice", []About{
		{Kind: KindGoal, ID: "g1-s45"},
		{Kind: KindGoal, ID: " g1-s45 "},
		{Kind: KindRecord, ID: "plans/designs/launch.md"},
	})

	testutil.Require(t, "writing the sticky", err, nil)
	testutil.Expect(t, "what it is about", list.Stickies[0].About, []About{
		{Kind: KindGoal, ID: "g1-s45"},
		{Kind: KindRecord, ID: "plans/designs/launch.md"},
	})
}

// Two humans keep two notepads in one file, and neither one sees the other's.
func TestTwoHumansKeepTwoNotepads(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	_, err := store.Add("Wido", "mine", nil)
	testutil.Require(t, "Wido writing", err, nil)
	_, err = store.Add("Sol", "theirs", nil)
	testutil.Require(t, "Sol writing", err, nil)

	wido, err := store.List("Wido")
	testutil.Require(t, "reading Wido's", err, nil)
	sol, err := store.List("Sol")
	testutil.Require(t, "reading Sol's", err, nil)

	testutil.Expect(t, "Wido's notepad", texts(wido), []string{"mine"})
	testutil.Expect(t, "Sol's notepad", texts(sol), []string{"theirs"})
	// A seat that names nobody is the row under the empty handle — the seat
	// itself — and is nobody else's notepad either.
	_, err = store.Add("", "the seat's own", nil)
	testutil.Require(t, "the seat writing", err, nil)
	wido, err = store.List("Wido")
	testutil.Require(t, "reading Wido's again", err, nil)
	testutil.Expect(t, "that the seat's note is not Wido's", texts(wido), []string{"mine"})
}

// The write is atomic: the file is replaced whole, so a reader never sees half
// a notepad. What is asserted is the property the atomic writer gives — no
// temporary file is left beside the target, and what is on disk parses whole.
func TestTheWriteIsAtomicAndLeavesNothingBesideTheFile(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	for count := 0; count < 3; count++ {
		_, err := store.Add("Wido", "one of three", nil)
		testutil.Require(t, "writing sticky "+strconv.Itoa(count), err, nil)
	}

	entries, err := os.ReadDir(filepath.Dir(store.File()))
	testutil.Require(t, "reading the directory", err, nil)
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	testutil.Expect(t, "what is in the directory", names, []string{"stickies.json"})

	data, err := os.ReadFile(store.File())
	testutil.Require(t, "reading the file", err, nil)
	var held stored
	testutil.Require(t, "parsing the file", json.Unmarshal(data, &held), nil)
	testutil.Expect(t, "the schema on disk", held.SchemaVersion, SchemaVersion)
	testutil.Expect(t, "the rows on disk", len(held.Humans["Wido"]), 3)
	testutil.Expect(t, "that it ends in a newline", strings.HasSuffix(string(data), "\n"), true)
}

// A file this build cannot read is an error and not an empty notepad: these
// are a human's own notes, and answering "you have none" would lose them
// silently.
func TestAFileThisBuildCannotReadIsSaidRatherThanTreatedAsEmpty(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	testutil.Require(t, "making the directory",
		os.MkdirAll(filepath.Dir(store.File()), 0o700), nil)
	testutil.Require(t, "planting a file this build cannot read",
		os.WriteFile(store.File(), []byte(`{"schemaVersion":99,"humans":{}}`), 0o600), nil)

	_, err := store.List("Wido")

	if err == nil {
		t.Fatalf("a notepad written under a schema this build does not read answered as though it were empty")
	}
	testutil.Expect(t, "that it says which file",
		strings.Contains(err.Error(), "stickies.json"), true)
}

func refusalOf(t *testing.T, what string, err error) *Refusal {
	t.Helper()
	var refusal *Refusal
	if err == nil {
		t.Fatalf("%s was admitted, and should have been refused", what)
	}
	if !errors.As(err, &refusal) {
		t.Fatalf("%s was refused with %v, which is not a refusal", what, err)
	}
	return refusal
}

// mintCounted is a fresh id per call, for the tests that write more stickies
// than a single letter can number.
func mintCounted() func() (string, error) {
	minted := 0
	return func() (string, error) {
		minted++
		return "STICKY" + strconv.Itoa(minted), nil
	}
}
