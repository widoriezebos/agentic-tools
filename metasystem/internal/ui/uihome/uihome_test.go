package uihome

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// Where the interface keeps an account's own material, and the two things about
// it that must never drift.
//
// The seam itself — the registry's own environment variable, and the refusal of a
// relative home that would resolve inside the checkout this server was started in
// — is proved against a store that uses it, in internal/ui/stickies/grants_test.go,
// because a relative home is only harmful in relation to a checkout. What is
// proved here is the arithmetic every store depends on: one workspace is one
// directory however its path is spelled, two workspaces named alike are two, and
// two owners under one home never reach into each other.

// Nothing here writes, and nothing here resolves the account's real home: this
// package's arithmetic is arithmetic, and a test that read a human's home to
// check a path would be reading a home for no reason.

// The home is the registry's own, and it is absolute or it is nothing.
//
// It is read here rather than set here: the shared test boundary pins a
// run-scoped registry home for every test process, which is the same seam a
// fixture uses, so this asks the question without touching the account's own
// home and without mutating an environment variable of its own.
func TestTheHomeIsTheRegistrysOwnAndAlwaysAbsolute(t *testing.T) {
	t.Parallel()
	home, err := Home()

	testutil.Require(t, "resolving the account's registry home", err, nil)
	testutil.Expect(t, "it is absolute, because a relative one would resolve in the checkout",
		filepath.IsAbs(home), true)
	testutil.Expect(t, "and it is the registry's own directory", filepath.Base(home), ".metasystem")
	testutil.Expect(t, "which is the directory every owner hangs under",
		strings.HasPrefix(Under(home, "partner", "/tmp/workspaces/example"),
			filepath.Join(home, "ui", "partner")+string(filepath.Separator)), true)
}

// One checkout is one directory however it is spelled, and two checkouts that
// share a last name are still two.
func TestOneWorkspaceIsOneDirectoryHoweverItIsSpelled(t *testing.T) {
	t.Parallel()
	home := "/home/someone/.metasystem"
	one := Under(home, "partner", "/tmp/workspaces/example")

	testutil.Expect(t, "a path spelled the long way round is the same directory",
		Under(home, "partner", "/tmp/workspaces/../workspaces/example/"), one)
	testutil.Expect(t, "a trailing separator changes nothing",
		Under(home, "partner", "/tmp/workspaces/example/"), one)
	if two := Under(home, "partner", "/tmp/elsewhere/example"); two == one {
		t.Fatalf("two checkouts named example share one directory at %q", one)
	}
	testutil.Expect(t, "the workspace is recognisable in the path",
		strings.Contains(one, "example-"), true)
	testutil.Expect(t, "it lies under the home it was given",
		strings.HasPrefix(one, home+string(filepath.Separator)), true)
}

// Two owners under one home are two directories. It is what keeps a human's
// notepad out of the way of their transcript: neither can be read by asking for
// the other, and neither is the parent of the other.
func TestTwoOwnersUnderOneHomeAreTwoDirectories(t *testing.T) {
	t.Parallel()
	home := "/home/someone/.metasystem"
	transcripts := Under(home, "partner", "/tmp/workspaces/example")
	notes := Under(home, "stickies", "/tmp/workspaces/example")

	if transcripts == notes {
		t.Fatalf("the transcript and the notepad share one directory at %q", transcripts)
	}
	testutil.Expect(t, "neither contains the other",
		strings.HasPrefix(transcripts, notes) || strings.HasPrefix(notes, transcripts), false)
	testutil.Expect(t, "each is named after its owner",
		strings.Contains(transcripts, filepath.Join("ui", "partner")+string(filepath.Separator)), true)
	testutil.Expect(t, "and both are under the one home",
		strings.HasPrefix(notes, filepath.Join(home, "ui")+string(filepath.Separator)), true)
}

// A path that is not a name at all is still one directory, and never a path out
// of the home.
//
// A relative path is resolved against the working directory first, so a dot and a
// parent name the real directories they stand for and are keyed by their own last
// name. What is left with no name to be keyed by is the filesystem root, and that
// one is keyed by its digest alone — twelve hexadecimal characters and nothing
// else. Either way the key is one segment that escapes nothing.
func TestAPathThatIsNotANameIsStillOneDirectoryUnderTheHome(t *testing.T) {
	t.Parallel()
	home := "/home/someone/.metasystem"
	owned := filepath.Join(home, "ui", "partner")
	for _, probe := range []struct {
		what     string
		checkout string
	}{
		{"the filesystem root", string(filepath.Separator)},
		{"a bare dot", "."},
		{"a parent", ".."},
	} {
		under := Under(home, "partner", probe.checkout)
		key := filepath.Base(under)
		testutil.Expect(t, probe.what+" is one directory below the owner's",
			filepath.Dir(under), owned)
		testutil.Expect(t, probe.what+" escapes nothing", strings.Contains(key, ".."), false)
		testutil.Expect(t, probe.what+" is keyed by something",
			len(key) >= 12, true)
	}
	testutil.Expect(t, "the root, which has no name, is keyed by its digest alone",
		len(filepath.Base(Under(home, "partner", string(filepath.Separator)))), 12)
}

// A checkout whose last name carries a separator, a space or a colon becomes one
// segment, so the key is a name on every filesystem this runs on and never a path
// out of the home.
func TestAWorkspaceNameBecomesOneSafeSegment(t *testing.T) {
	t.Parallel()
	home := "/home/someone/.metasystem"
	under := Under(home, "partner", "/tmp/work spaces/a:b c")
	key := filepath.Base(under)

	testutil.Expect(t, "the segment carries no separator", strings.ContainsRune(key, filepath.Separator), false)
	testutil.Expect(t, "no space", strings.Contains(key, " "), false)
	testutil.Expect(t, "no colon", strings.Contains(key, ":"), false)
	testutil.Expect(t, "and the directory is exactly one below the owner's",
		filepath.Dir(under), filepath.Join(home, "ui", "partner"))
}

// The anchor an atomic write may make durable up to is the directory ABOVE the
// home, because the home itself and everything under it is something a store may
// create for itself on a machine that has never run this before.
func TestTheAnchorIsTheDirectoryAboveTheHome(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "the anchor pre-exists the home",
		Anchor("/home/someone/.metasystem"), "/home/someone")
}

// What the store holds, per workspace, across every owner in it — and nothing
// that is not in it.
//
// D2 of g1-s54 is that housekeeping and everything reporting on it work only on
// what is under the account's home. This is the measurement's half of that: the
// walk starts at <home>/ui and reaches neither the rest of the home nor anything
// beside it.
func TestMeasureSumsEveryOwnerPerWorkspaceAndReadsNothingElse(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	one := "/work/one"
	two := "/work/two"
	plantMeasured(t, filepath.Join(home, "ui", "partner", Key(one)), "wido.jsonl", 2000)
	plantMeasured(t, filepath.Join(home, "ui", "partner", Key(one)), "wire.jsonl", 1000)
	plantMeasured(t, filepath.Join(home, "ui", "stickies", Key(one)), "stickies.json", 500)
	plantMeasured(t, filepath.Join(home, "ui", "partner", Key(two)), "wido.jsonl", 10)
	// Under the home but outside the store, and outside the home entirely.
	plantMeasured(t, home, "armed-checkouts.jsonl", 9999)
	plantMeasured(t, filepath.Dir(home), "beside-the-home", 9999)

	measured, err := Measure(home)

	testutil.Require(t, "measuring", err, nil)
	testutil.Require(t, "one entry per workspace", len(measured), 2)
	testutil.Expect(t, "the largest first", measured[0].Key, Key(one))
	testutil.Expect(t, "with every owner's files in the one figure", measured[0].Bytes, int64(3500))
	testutil.Expect(t, "then the smaller", measured[1].Key, Key(two))
	testutil.Expect(t, "at its own size", measured[1].Bytes, int64(10))
}

// A store that is not there yet is no entries and no failure: a seat whose human
// has written nothing has a store of nothing, and that is the true answer.
func TestMeasureAnswersNothingForAStoreThatIsNotThereYet(t *testing.T) {
	t.Parallel()
	measured, err := Measure(t.TempDir())
	testutil.Require(t, "measuring", err, nil)
	testutil.Expect(t, "no entries", len(measured), 0)
}

func plantMeasured(t *testing.T, directory, name string, size int) {
	t.Helper()
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("cannot make %s: %v", directory, err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), make([]byte, size), 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", name, err)
	}
}

// What is not a directory under the store is not a workspace, and skipping it
// costs nothing: a stray file beside the owners is not a checkout's share.
func TestMeasureSkipsWhatIsNotADirectory(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	one := "/work/one"
	plantMeasured(t, filepath.Join(home, "ui", "partner", Key(one)), "wido.jsonl", 40)
	plantMeasured(t, filepath.Join(home, "ui"), "a-stray-file", 9999)
	plantMeasured(t, filepath.Join(home, "ui", "partner"), "another-stray-file", 9999)

	measured, err := Measure(home)

	testutil.Require(t, "measuring", err, nil)
	testutil.Require(t, "only the workspace is counted", len(measured), 1)
	testutil.Expect(t, "at its own size", measured[0].Bytes, int64(40))
}

// Two workspaces of one size are ordered by name, so the card's lines do not
// swap places between two reads of the same store.
func TestMeasureOrdersWorkspacesOfOneSizeByName(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	plantMeasured(t, filepath.Join(home, "ui", "partner", "beta-222222"), "wido.jsonl", 100)
	plantMeasured(t, filepath.Join(home, "ui", "partner", "alpha-111111"), "wido.jsonl", 100)

	measured, err := Measure(home)

	testutil.Require(t, "measuring", err, nil)
	testutil.Require(t, "both are counted", len(measured), 2)
	testutil.Expect(t, "the first by name", measured[0].Key, "alpha-111111")
	testutil.Expect(t, "then the second", measured[1].Key, "beta-222222")
}

// A store that cannot be read is a refusal naming the path, never a size of
// zero. A number a human is watching to see it stop growing must not be smaller
// because a directory could not be opened.
func TestMeasureRefusesAStoreItCannotRead(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	// The store's own directory is a file, which is not the same thing as a
	// store that is not there yet.
	plantMeasured(t, home, "ui", 10)

	_, err := Measure(home)

	testutil.Require(t, "it refused", err != nil, true)
	testutil.Expect(t, "and named the store", strings.Contains(err.Error(), filepath.Join(home, "ui")), true)
	testutil.Expect(t, "and said it could not be read",
		strings.Contains(err.Error(), "could not be read"), true)
}

// An owner directory this account cannot open, and a workspace directory it
// cannot walk, are both refusals for the same reason.
func TestMeasureRefusesWhatItCannotOpen(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	owner := filepath.Join(home, "ui", "partner")
	plantMeasured(t, filepath.Join(owner, "one-111111"), "wido.jsonl", 10)
	closed(t, owner)

	_, err := Measure(home)

	testutil.Require(t, "an owner it cannot open is refused", err != nil, true)
	testutil.Expect(t, "in the store's own words",
		strings.Contains(err.Error(), "could not be read"), true)

	elsewhere := t.TempDir()
	workspace := filepath.Join(elsewhere, "ui", "partner", "two-222222")
	plantMeasured(t, workspace, "wido.jsonl", 10)
	closed(t, workspace)

	_, err = Measure(elsewhere)

	testutil.Require(t, "a workspace it cannot walk is refused too", err != nil, true)
	testutil.Expect(t, "naming the directory it could not measure",
		strings.Contains(err.Error(), "could not be measured"), true)
}

// closed takes a directory's own permissions away for the length of one test,
// and gives them back so the fixture can be cleaned up.
func closed(t *testing.T, directory string) {
	t.Helper()
	if err := os.Chmod(directory, 0); err != nil {
		t.Fatalf("cannot close %s: %v", directory, err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
}

// A file this walk listed and cannot stat is a refusal naming the directory,
// not a file counted as nothing. The size is a number a human watches to see it
// stop growing, and a count that is quietly smaller because one file could not
// be read is worse than a refusal: nothing on the page would say so.
func TestMeasuringAFileItCannotStatIsARefusal(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	workspace := filepath.Join(home, "ui", "partner", "one-111111")
	plantMeasured(t, workspace, "wido.jsonl", 10)
	// Readable, so the walk still lists the file, and not searchable, so the
	// file's own metadata cannot be read. That is the state this rule is about:
	// the walk itself did not fail, one entry of it did.
	unsearchable(t, workspace)

	_, err := sizeOf(workspace)

	testutil.Require(t, "a file it cannot stat is refused", err != nil, true)

	_, err = Measure(home)

	testutil.Require(t, "and the store's measurement carries it", err != nil, true)
	testutil.Expect(t, "naming the directory it could not measure",
		strings.Contains(err.Error(), "could not be measured"), true)
}

// unsearchable leaves a directory readable and takes away the permission to
// read what is inside it, for the length of one test.
func unsearchable(t *testing.T, directory string) {
	t.Helper()
	if err := os.Chmod(directory, 0o400); err != nil {
		t.Fatalf("cannot close %s: %v", directory, err)
	}
	t.Cleanup(func() { _ = os.Chmod(directory, 0o700) })
}

// A workspace whose files went while the sweep walked is nothing to count and
// nothing to refuse. The store is written under this account while the server
// serves it, so a walk that refused on a name that had moved on would report a
// failure where a human's own conversation had merely been trimmed.
func TestMeasuringADirectoryThatWentIsNothingRatherThanARefusal(t *testing.T) {
	t.Parallel()
	total, err := sizeOf(filepath.Join(t.TempDir(), "went", "while", "walking"))
	testutil.Require(t, "measuring what is not there", err, nil)
	testutil.Expect(t, "it holds nothing", total, int64(0))
}
