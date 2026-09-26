package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// What Settings says about the private store (g1-s54 D3): where it is, what it
// holds per workspace, and the bounds in words.
//
// The sizes are a real walk of a real store, because a human who asked "I do not
// want this to grow and grow until it fills up the file system" is answered by a
// number they can watch, not by a sentence about a design.
func TestTheStoreSaysItsPathItsSizePerWorkspaceAndItsBoundsInWords(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "repository")
	plant(t, home, "partner", key(t, checkout), "wido.jsonl", 3000)
	plant(t, home, "stickies", key(t, checkout), "stickies.json", 1000)
	plant(t, home, "partner", "elsewhere-abcdef", "wido.jsonl", 2<<20)

	store := DescribeStore(home, "", checkout, StoreBounds{WireMB: 8, ConversationMB: 2, ConversationDays: 90})

	testutil.Expect(t, "the store's own path", store.Path, filepath.Join(home, "ui"))
	testutil.Expect(t, "nothing went wrong", store.Problem, "")
	testutil.Require(t, "one line per workspace", len(store.Workspaces), 2)
	testutil.Expect(t, "the largest first", store.Workspaces[0].Name, "elsewhere-abcdef")
	testutil.Expect(t, "in megabytes once it is one", store.Workspaces[0].Size, "2.0 MB")
	testutil.Expect(t, "and it is not this one", store.Workspaces[0].This, false)
	testutil.Expect(t, "this workspace is named too", store.Workspaces[1].Name, key(t, checkout))
	testutil.Expect(t, "with both owners counted in one figure", store.Workspaces[1].Size, "4 KB")
	testutil.Expect(t, "and marked as the one in front of the human", store.Workspaces[1].This, true)
	testutil.Require(t, "three sentences of bounds", len(store.Bounds), 3)
	testutil.Expect(t, "the journal's bound", store.Bounds[0],
		"The Partner's wire journal is rotated at 8 MB, keeping one previous.")
	testutil.Expect(t, "the conversation's two", store.Bounds[1],
		"A conversation is trimmed from its oldest end when it passes 2 MB or its oldest message is more than 90 days old, and the last 200 messages are always kept.")
	testutil.Expect(t, "and what is never removed", store.Bounds[2],
		"Nothing else here is ever removed, and no record of your project is: the records are the memory that survives.")
}

// A workspace the store holds nothing for yet is still named, at nothing, so the
// card reads as a store with nothing in it rather than as a store that could not
// be read.
func TestAWorkspaceTheStoreHoldsNothingForIsStillNamed(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "fresh")

	store := DescribeStore(home, "", checkout, StoreBounds{WireMB: 8, ConversationMB: 2, ConversationDays: 90})

	testutil.Expect(t, "nothing went wrong", store.Problem, "")
	testutil.Require(t, "this workspace is named", len(store.Workspaces), 1)
	testutil.Expect(t, "as this one", store.Workspaces[0].This, true)
	testutil.Expect(t, "holding nothing", store.Workspaces[0].Size, "0 bytes")
}

// A seat whose account has no registry home keeps no private store at all, and
// says so where the size would be.
func TestASeatWithNoHomeSaysItKeepsNoStore(t *testing.T) {
	t.Parallel()
	store := DescribeStore("", "this account has no registry home the interface can read", "/work/repository",
		StoreBounds{WireMB: 8})

	testutil.Expect(t, "the reason is the reader's own",
		store.Problem, "this account has no registry home the interface can read")
	testutil.Expect(t, "and nothing is claimed about a path", store.Path, "")
	testutil.Expect(t, "or about sizes", len(store.Workspaces), 0)
}

// A bound of zero is a bound that is off, and the words say which.
func TestBoundsThatAreOffAreSaidToBeOff(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	checkout := filepath.Join(t.TempDir(), "repository")

	off := DescribeStore(home, "", checkout, StoreBounds{})
	testutil.Expect(t, "the journal's bound is off", off.Bounds[0],
		"The Partner's wire journal is not bounded on this seat.")
	testutil.Expect(t, "and the conversation's", off.Bounds[1],
		"Conversations are not bounded on this seat.")

	size := DescribeStore(home, "", checkout, StoreBounds{ConversationMB: 2})
	testutil.Expect(t, "size alone says age is off",
		strings.Contains(size.Bounds[1], "Nothing is trimmed for age on this seat."), true)

	days := DescribeStore(home, "", checkout, StoreBounds{ConversationDays: 90})
	testutil.Expect(t, "age alone says size is off",
		strings.Contains(days.Bounds[1], "Nothing is trimmed for size on this seat."), true)
}

// A size a human reads: bytes, then whole kilobytes, then megabytes with one
// decimal, which is the unit the bounds themselves are named in.
func TestASizeIsSaidInTheUnitTheBoundsAreNamedIn(t *testing.T) {
	t.Parallel()
	testutil.Expect(t, "nothing", sizeWords(0), "0 bytes")
	testutil.Expect(t, "under a kilobyte", sizeWords(1023), "1023 bytes")
	testutil.Expect(t, "a kilobyte", sizeWords(1024), "1 KB")
	testutil.Expect(t, "rounded to the nearest", sizeWords(49*1024+600), "50 KB")
	testutil.Expect(t, "a megabyte", sizeWords(1<<20), "1.0 MB")
	testutil.Expect(t, "and past it", sizeWords(3*(1<<20)+(1<<19)), "3.5 MB")
}

// plant writes one file of n bytes where one owner keeps one workspace's files.
func plant(t *testing.T, home, owner, workspace, name string, size int) {
	t.Helper()
	directory := filepath.Join(home, "ui", owner, workspace)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		t.Fatalf("cannot make %s: %v", directory, err)
	}
	if err := os.WriteFile(filepath.Join(directory, name), make([]byte, size), 0o600); err != nil {
		t.Fatalf("cannot write %s: %v", name, err)
	}
}

// key is the directory segment one checkout is kept under, as the store's own
// owner derives it.
func key(t *testing.T, checkout string) string {
	t.Helper()
	return filepath.Base(DescribeStore(t.TempDir(), "", checkout, StoreBounds{}).Workspaces[0].Name)
}
