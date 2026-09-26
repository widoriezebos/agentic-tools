package uihome

import (
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
