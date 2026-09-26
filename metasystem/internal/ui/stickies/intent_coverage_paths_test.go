package stickies

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// The notepad's failure edges: a file that cannot be read as a notepad, a
// notepad file with no humans in it, an id that cannot be minted, and a
// directory that refuses the rewrite. Each says so, and none loses what was
// already written.

// A notepad file that is not JSON is an error naming the file, and a write
// over it is refused rather than replacing what a human wrote with one row.
func TestANotepadThatIsNotJSONIsRefusedAndNotOverwritten(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	testutil.Require(t, "making the notepad's directory",
		os.MkdirAll(filepath.Dir(store.File()), 0o700), nil)
	testutil.Require(t, "planting a damaged notepad",
		os.WriteFile(store.File(), []byte("{not json"), 0o600), nil)

	_, err := store.List("Wido")
	if err == nil || !strings.Contains(err.Error(), store.File()) {
		t.Fatalf("reading a damaged notepad answered %v, want an error naming %s", err, store.File())
	}
	_, err = store.Add("Wido", "ask Sol about the retry", nil)
	if err == nil {
		t.Fatalf("writing over a damaged notepad was admitted")
	}
	kept, readErr := os.ReadFile(store.File())
	testutil.Require(t, "reading the damaged notepad back", readErr, nil)
	testutil.Expect(t, "what the damaged notepad still holds", string(kept), "{not json")
}

// A notepad file under this schema that names no humans at all is an empty
// notepad for everyone, and the first write into it keeps the schema.
func TestANotepadWithNoHumansReadsAsEmpty(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	testutil.Require(t, "making the notepad's directory",
		os.MkdirAll(filepath.Dir(store.File()), 0o700), nil)
	testutil.Require(t, "planting a notepad without humans",
		os.WriteFile(store.File(), []byte(`{"schemaVersion":1}`), 0o600), nil)

	list, err := store.List("Wido")
	testutil.Require(t, "reading a notepad without humans", err, nil)
	testutil.Expect(t, "how many stickies it holds", len(list.Stickies), 0)
	testutil.Expect(t, "that the answer is a list and not nothing", list.Stickies != nil, true)

	list, err = store.Add("Wido", "check g1-s45 tomorrow", nil)
	testutil.Require(t, "writing into it", err, nil)
	testutil.Expect(t, "what it now holds", texts(list), []string{"check g1-s45 tomorrow"})
}

// An id that cannot be minted is the add's error, and nothing is written.
func TestAStickyWhoseIDCannotBeMintedIsNotWritten(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	exhausted := errors.New("no entropy")
	store.mint = func() (string, error) { return "", exhausted }

	_, err := store.Add("Wido", "ask Sol about the retry", nil)

	testutil.Expect(t, "the add's error", errors.Is(err, exhausted), true)
	if _, statErr := os.Stat(store.File()); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("a sticky with no id left a notepad behind: %v", statErr)
	}
}

// A rewrite that cannot be persisted fails the act, and the notepad still
// answers what it held before the act. The mint hook runs after the act has
// read the notepad, so it is where the notepad's directory is moved aside and
// a regular file put in its place: the rewrite then has no directory to land
// in on any host and under any user, and the original notepad is untouched
// in the moved directory until it is put back.
func TestARewriteThatCannotBePersistedFailsTheAct(t *testing.T) {
	t.Parallel()

	store, _ := fixture(t)
	_, err := store.Add("Wido", "ask Sol about the retry", nil)
	testutil.Require(t, "writing the first sticky", err, nil)
	before, err := os.ReadFile(store.File())
	testutil.Require(t, "reading the notepad before the act", err, nil)

	dir := filepath.Dir(store.File())
	aside := dir + ".aside"
	restore := func() error {
		if _, err := os.Stat(aside); errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err := os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return os.Rename(aside, dir)
	}
	t.Cleanup(func() {
		if err := restore(); err != nil {
			t.Errorf("putting the notepad's directory back: %v", err)
		}
	})
	store.mint = func() (string, error) {
		if err := os.Rename(dir, aside); err != nil {
			return "", err
		}
		if err := os.WriteFile(dir, []byte("not a directory"), 0o600); err != nil {
			return "", err
		}
		return "STICKY2", nil
	}

	_, err = store.Add("Wido", "check g1-s45 tomorrow", nil)
	if err == nil {
		t.Fatalf("a rewrite with no directory to land in answered as though it had been written")
	}
	testutil.Require(t, "putting the notepad's directory back", restore(), nil)

	after, err := os.ReadFile(store.File())
	testutil.Require(t, "reading the notepad after the failed act", err, nil)
	testutil.Expect(t, "the notepad's bytes after the failed act", string(after), string(before))
	list, err := store.List("Wido")
	testutil.Require(t, "listing the notepad after the failed act", err, nil)
	testutil.Expect(t, "what it still holds", texts(list), []string{"ask Sol about the retry"})
}
