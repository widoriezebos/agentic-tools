package stickies

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Editing, and the edges the acts share: what a sticky is about, what an edit
// is refused by, and what a notepad that cannot be written says.

// What a sticky is about can be changed without touching what it says, which
// is the other half of "a field nobody sent is a field nobody changed".
func TestWhatAStickyIsAboutCanBeChangedOnItsOwn(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	_, err := store.Add("Wido", "ask Sol about the retry", []About{{Kind: KindGoal, ID: "g1-s45"}})
	testutil.Require(t, "writing a sticky", err, nil)

	named := []About{{Kind: KindGoal, ID: "g1-s45"}, {Kind: KindRecord, ID: "plans/designs/launch.md"}}
	list, err := store.Edit("Wido", "STICKY1", nil, &named, nil)

	testutil.Require(t, "changing what it is about", err, nil)
	testutil.Expect(t, "what it is now about", list.Stickies[0].About, named)
	testutil.Expect(t, "that the text is untouched", list.Stickies[0].Text, "ask Sol about the retry")

	// And a sticky can be made about nothing at all, which is the common case.
	none := []About{}
	list, err = store.Edit("Wido", "STICKY1", nil, &none, nil)
	testutil.Require(t, "taking every chip off", err, nil)
	testutil.Expect(t, "what it is about once the chips are off", len(list.Stickies[0].About), 0)
}

// An edit is refused by the same rules a write is, and nothing is written.
func TestAnEditIsRefusedByTheSameRulesAsAWrite(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	_, err := store.Add("Wido", "ask Sol about the retry", nil)
	testutil.Require(t, "writing a sticky", err, nil)

	long := strings.Repeat("a", MaxText+1)
	_, err = store.Edit("Wido", "STICKY1", &long, nil, nil)
	testutil.Expect(t, "editing past the text bound", refusalOf(t, "too much text", err).Kind, RefusalBounds)

	empty := "   "
	_, err = store.Edit("Wido", "STICKY1", &empty, nil, nil)
	testutil.Expect(t, "editing to nothing at all", refusalOf(t, "an empty edit", err).Kind, RefusalBad)

	unknown := []About{{Kind: "machine", ID: "m1a"}}
	_, err = store.Edit("Wido", "STICKY1", nil, &unknown, nil)
	testutil.Expect(t, "editing to an unknown kind", refusalOf(t, "an unknown about", err).Kind, RefusalBad)

	list, err := store.List("Wido")
	testutil.Require(t, "reading the notepad", err, nil)
	testutil.Expect(t, "that nothing was written", texts(list), []string{"ask Sol about the retry"})
}

// A refusal reads as a sentence, because the sentence is the whole of what a
// human is shown.
func TestARefusalReadsAsItsOwnSentence(t *testing.T) {
	t.Parallel()

	refusal := refuse(RefusalBad, "a sticky says something; this one is empty")

	testutil.Expect(t, "what it says", refusal.Error(), "a sticky says something; this one is empty")
}

// A store given no clock keeps the wall clock, which is what every run uses.
func TestAStoreGivenNoClockKeepsTheWallOne(t *testing.T) {
	t.Parallel()

	store := New(filepath.Join(t.TempDir(), ".metasystem"), "/tmp/workspaces/example", nil)

	list, err := store.Add("Wido", "written now", nil)

	testutil.Require(t, "writing a sticky", err, nil)
	testutil.Expect(t, "that it is stamped", list.Stickies[0].CreatedAt != "", true)
}

// A checkout whose last name is not a name on every filesystem still keys one
// directory, and a checkout that has no last name at all is its digest alone.
func TestAWorkspaceKeyIsAlwaysOneSegment(t *testing.T) {
	t.Parallel()

	home := "/home/someone/.metasystem"

	odd := Path(home, "/tmp/a workspace: v2/")

	testutil.Expect(t, "that the key is one segment",
		strings.Count(strings.TrimPrefix(odd, home), string(filepath.Separator)), 4)
	testutil.Expect(t, "that what is not a plain name is replaced",
		strings.Contains(odd, "a-workspace--v2-"), true)
	testutil.Expect(t, "that a checkout with no last name is still one directory",
		strings.Count(strings.TrimPrefix(Path(home, "/"), home), string(filepath.Separator)), 4)
}

// A notepad this build cannot write is an error and not a silent loss.
func TestANotepadThatCannotBeWrittenSaysSo(t *testing.T) {
	t.Parallel()

	blocked := filepath.Join(t.TempDir(), "in-the-way")
	testutil.Require(t, "planting a file where the home goes",
		os.WriteFile(blocked, []byte("not a directory"), 0o600), nil)
	store := New(filepath.Join(blocked, ".metasystem"), "/tmp/workspaces/example", func() time.Time { return noon })

	_, err := store.Add("Wido", "ask Sol about the retry", nil)

	if err == nil {
		t.Fatalf("a notepad that could not be written answered as though it had been")
	}
}
