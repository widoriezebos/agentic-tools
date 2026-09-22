package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// The blob id is the repository's own name for the bytes read: the literal is
// what `git hash-object` answers for a file holding "hello\n".
func TestReadNamesTheGitBlobOfTheBytesRead(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/hello.md", "hello\n")

	document, err := Read(roots, "docs/hello.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Expect(t, "the revision", document.Revision, "blob:ce013625030ba8dba906f756967f9e9ca394464a")
	testutil.Expect(t, "the kind", document.Kind, "document")
	testutil.Expect(t, "the id", document.ID, "docs/hello.md")
	testutil.Expect(t, "the state", document.State, stateReadable)
	testutil.Expect(t, "the path", document.Path, filepath.Join(roots.Checkout, "docs", "hello.md"))
	testutil.Expect(t, "read at", document.ReadAt, "2026-09-21T10:11:12Z")
	testutil.Expect(t, "the bytes", document.Bytes, int64(6))
}

// A title is the document's first heading, and the file's name when it has
// none, so a listing never shows an empty row.
func TestReadTitlesADocument(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/titled.md", "Preamble\n\n# The title\n\nBody.\n")
	plant(t, roots.Checkout, "docs/untitled.md", "Body with no heading.\n")

	titled, err := Read(roots, "docs/titled.md", readAt)
	testutil.Require(t, "read the titled document", err, nil)
	untitled, err := Read(roots, "docs/untitled.md", readAt)
	testutil.Require(t, "read the untitled document", err, nil)

	testutil.Expect(t, "the title", titled.Title, "The title")
	testutil.Expect(t, "the fallback title", untitled.Title, "untitled.md")
}

// Every refusal is the same answer. The id is judged as text before anything
// is opened, and what gets past that is judged on the descriptor.
func TestReadRefusesEveryInadmissibleID(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/a.md", "# A\n")
	plant(t, roots.Checkout, "notes.txt", "not markdown\n")
	plant(t, roots.Checkout, "metasystem/metasystem.conf.local", "secret = 1\n")
	makeDirectory(t, filepath.Join(roots.Checkout, "docs", "adirectory.md"))

	for _, id := range []string{
		"", ".", "/", "/etc/passwd", "../../etc/passwd", "..", "../a.md",
		"docs/../docs/a.md", "docs//a.md", "docs/./a.md", "docs/a.md/",
		"notes.txt", "docs/a.MD", "metasystem/metasystem.conf.local",
		"docs/adirectory.md", "docs/missing.md", "docs",
	} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			document, err := Read(roots, id, readAt)

			testutil.Expect(t, "the refusal", errors.Is(err, ErrNotFound), true)
			testutil.Expect(t, "nothing is answered", document, Document{})
		})
	}
}

// The four refused segments are compared case-insensitively, because APFS is
// case-insensitive by default: .GIT/x.md reaches .git/x.md, and os.Root folds
// nothing of its own. Each case records whether the mixed-case spelling really
// does reach the planted file on this filesystem, so a reader can tell whether
// this run is the one that proves the fold.
func TestReadRefusesSegmentsInAnyCase(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	for _, directory := range refusedSegments {
		plant(t, roots.Checkout, directory+"/x.md", "# Planted under "+directory+"\n")
	}

	for _, spelling := range []string{".GIT/x.md", ".Git/x.md", "NODE_MODULES/x.md", "Node_Modules/x.md", "Artifacts/x.md", "ARTIFACTS/x.md", "BIN/x.md", "Bin/x.md"} {
		t.Run(spelling, func(t *testing.T) {
			t.Parallel()

			_, statErr := os.Stat(filepath.Join(roots.Checkout, filepath.FromSlash(spelling)))
			reaches := statErr == nil
			t.Logf("os.Stat(%q) reaches the planted file: %v; this run proves the fold: %v", spelling, reaches, reaches)

			document, err := Read(roots, spelling, readAt)

			testutil.Expect(t, "the refusal", errors.Is(err, ErrNotFound), true)
			testutil.Expect(t, "nothing is answered", document.ID, "")
		})
	}
}

// The policy, stated plainly and tested as stated: a symbolic link that stays
// in the checkout is served, at the leaf or in the middle of the path.
func TestReadServesLinksThatStayInTheCheckout(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/target.md", "# The target\n")
	plant(t, roots.Checkout, "docs/real/inner.md", "# Inside a real directory\n")
	link(t, "target.md", filepath.Join(roots.Checkout, "docs", "leaf.md"))
	link(t, "real", filepath.Join(roots.Checkout, "docs", "linked"))

	target, err := Read(roots, "docs/target.md", readAt)
	testutil.Require(t, "read the target", err, nil)
	leaf, err := Read(roots, "docs/leaf.md", readAt)
	testutil.Require(t, "read through the leaf link", err, nil)
	inner, err := Read(roots, "docs/real/inner.md", readAt)
	testutil.Require(t, "read the inner document", err, nil)
	linked, err := Read(roots, "docs/linked/inner.md", readAt)
	testutil.Require(t, "read through the linked directory", err, nil)

	testutil.Expect(t, "the leaf link's revision", leaf.Revision, target.Revision)
	testutil.Expect(t, "the leaf link's title", leaf.Title, "The target")
	testutil.Expect(t, "the linked directory's revision", linked.Revision, inner.Revision)
	testutil.Expect(t, "the linked directory's title", linked.Title, "Inside a real directory")
}

// A link that leaves the checkout, at the leaf or in the middle, and an
// absolute link even to a file inside it, are the same 404.
func TestReadRefusesLinksThatLeaveTheCheckout(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	elsewhere := outside(t)
	plant(t, elsewhere, "secret.md", "# Outside\n")
	plant(t, roots.Checkout, "docs/inside.md", "# Inside\n")
	makeDirectory(t, filepath.Join(roots.Checkout, "docs"))
	link(t, filepath.Join(elsewhere, "secret.md"), filepath.Join(roots.Checkout, "docs", "escape.md"))
	link(t, elsewhere, filepath.Join(roots.Checkout, "docs", "away"))
	link(t, filepath.Join(roots.Checkout, "docs", "inside.md"), filepath.Join(roots.Checkout, "docs", "absolute.md"))

	for _, id := range []string{"docs/escape.md", "docs/away/secret.md", "docs/absolute.md"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()

			document, err := Read(roots, id, readAt)

			testutil.Expect(t, "the refusal", errors.Is(err, ErrNotFound), true)
			testutil.Expect(t, "nothing is answered", document, Document{})
		})
	}
}

// A hard link is an ordinary directory entry in the checkout, whatever other
// names the same file has, so it is served. The boundary is one of reachable
// names, not of inodes, and this test says so out loud.
func TestReadServesAHardLinkToAFileOutsideTheCheckout(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	elsewhere := outside(t)
	target := plant(t, elsewhere, "shared.md", "# Shared through a hard link\n")
	makeDirectory(t, filepath.Join(roots.Checkout, "docs"))
	hardLink(t, target, filepath.Join(roots.Checkout, "docs", "shared.md"))

	document, err := Read(roots, "docs/shared.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Expect(t, "the title", document.Title, "Shared through a hard link")
	testutil.Expect(t, "the state", document.State, stateReadable)
}

// too-large is decided from the bytes that came back, never from the size the
// filesystem reported before the read.
func TestReadDecidesTooLargeFromTheBytesRead(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plantBytes(t, roots.Checkout, "docs/exact.md", bytes.Repeat([]byte("a"), maxDocumentBytes))
	plantBytes(t, roots.Checkout, "docs/over.md", bytes.Repeat([]byte("a"), maxDocumentBytes+1))

	exact, err := Read(roots, "docs/exact.md", readAt)
	testutil.Require(t, "read the exact mebibyte", err, nil)
	over, err := Read(roots, "docs/over.md", readAt)
	testutil.Require(t, "read the mebibyte and one", err, nil)

	testutil.Expect(t, "a mebibyte is read", exact.State, stateReadable)
	testutil.Expect(t, "a mebibyte has a revision", strings.HasPrefix(exact.Revision, "blob:"), true)
	testutil.Expect(t, "a mebibyte and one is too large", over.State, stateTooLarge)
	testutil.Expect(t, "too large carries no revision", over.Revision, "")
	testutil.Expect(t, "too large carries no blocks", over.Blocks, []markdown.Block{})
	testutil.Expect(t, "too large carries its size", over.Bytes, int64(maxDocumentBytes+1))
	testutil.Expect(t, "too large says why", strings.Contains(over.Reason, "1 MiB"), true)
}

// A file whose bytes are not text is answered as unreadable, with the blob id
// of what was read, because every byte of it was.
func TestReadAnswersUnreadableForBytesThatAreNotText(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plantBytes(t, roots.Checkout, "docs/binary.md", []byte{0xff, 0xfe, 0x00, 0x01})

	document, err := Read(roots, "docs/binary.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Expect(t, "the state", document.State, stateUnreadable)
	testutil.Expect(t, "the reason", document.Reason, "the file is not valid UTF-8 text")
	testutil.Expect(t, "the revision is named", strings.HasPrefix(document.Revision, "blob:"), true)
	testutil.Expect(t, "no blocks", document.Blocks, []markdown.Block{})
}

// Ownership is the engine's answer, carried as the oracle's own string.
func TestReadCarriesTheOwnershipAnswer(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/app.md", "# Application\n")
	plant(t, roots.Checkout, "metasystem/docs/kit.md", "# The kit\n")

	app, err := Read(roots, "docs/app.md", readAt)
	testutil.Require(t, "read the application's document", err, nil)
	kit, err := Read(roots, "metasystem/docs/kit.md", readAt)
	testutil.Require(t, "read the kit's document", err, nil)

	testutil.Expect(t, "the application's ownership", app.Owner, "app-owned")
	testutil.Expect(t, "the kit's ownership", kit.Owner, "metasystem-generic")
}

// The race, through the seam.
//
// read calls beforeOpen after every check on the id and immediately before the
// one call that yields a descriptor, which is the window an implementation
// that resolved a pathname and reopened it by name would leave open. Each
// subtest swaps inside that window, on its own fixture, with no timer.
//
// An implementation that resolved and reopened would follow the swapped link
// in the first two cases and serve the outside file; this passes only because
// the descriptor comes from the anchored open.
func TestReadSwappedBeforeOpen(t *testing.T) {
	t.Parallel()

	type swapCase struct {
		name string
		// swap runs inside the hook, on a fixture planted with docs/a.md.
		swap func(t *testing.T, roots Roots, elsewhere string)
		// wantTitle is empty when the read must refuse.
		wantTitle string
	}
	for _, tc := range []swapCase{
		{
			name: "the leaf becomes a link to a file outside the checkout",
			swap: func(t *testing.T, roots Roots, elsewhere string) {
				leaf := filepath.Join(roots.Checkout, "docs", "a.md")
				if err := os.Remove(leaf); err != nil {
					t.Fatalf("remove the leaf: %v", err)
				}
				link(t, filepath.Join(elsewhere, "secret.md"), leaf)
			},
		},
		{
			name: "the parent becomes a link to a directory outside the checkout",
			swap: func(t *testing.T, roots Roots, elsewhere string) {
				parent := filepath.Join(roots.Checkout, "docs")
				rename(t, parent, filepath.Join(roots.Checkout, "docs-was"))
				link(t, elsewhere, parent)
			},
		},
		{
			name: "the leaf becomes a hard link to a file outside the checkout",
			swap: func(t *testing.T, roots Roots, elsewhere string) {
				leaf := filepath.Join(roots.Checkout, "docs", "a.md")
				if err := os.Remove(leaf); err != nil {
					t.Fatalf("remove the leaf: %v", err)
				}
				hardLink(t, filepath.Join(elsewhere, "secret.md"), leaf)
			},
			wantTitle: "Outside",
		},
		{
			name: "the leaf becomes a link to a sibling in the checkout",
			swap: func(t *testing.T, roots Roots, elsewhere string) {
				leaf := filepath.Join(roots.Checkout, "docs", "a.md")
				if err := os.Remove(leaf); err != nil {
					t.Fatalf("remove the leaf: %v", err)
				}
				link(t, "sibling.md", leaf)
			},
			wantTitle: "The sibling",
		},
		{
			name: "the leaf becomes a directory",
			swap: func(t *testing.T, roots Roots, elsewhere string) {
				leaf := filepath.Join(roots.Checkout, "docs", "a.md")
				if err := os.Remove(leaf); err != nil {
					t.Fatalf("remove the leaf: %v", err)
				}
				makeDirectory(t, leaf)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			roots := selfHostedFixture(t)
			elsewhere := outside(t)
			secret := plant(t, elsewhere, "secret.md", "# Outside\n")
			// The outside directory holds the same leaf name, so swapping the
			// parent for a link to it is discriminating: an implementation that
			// reopened the pathname would find a file there and serve it.
			plant(t, elsewhere, "a.md", "# Outside\n")
			plant(t, roots.Checkout, "docs/a.md", "# The original\n")
			plant(t, roots.Checkout, "docs/sibling.md", "# The sibling\n")
			swaps := 0

			document, err := read(roots, "docs/a.md", readAt, func() {
				swaps++
				tc.swap(t, roots, elsewhere)
			})

			testutil.Expect(t, "the hook ran once", swaps, 1)
			if tc.wantTitle == "" {
				testutil.Expect(t, "the refusal", errors.Is(err, ErrNotFound), true)
				testutil.Expect(t, "nothing is answered", document.Title, "")
				return
			}
			testutil.Require(t, "read", err, nil)
			testutil.Expect(t, "the title", document.Title, tc.wantTitle)
			if tc.wantTitle == "Outside" {
				content, readErr := os.ReadFile(secret)
				testutil.Require(t, "read the outside file", readErr, nil)
				testutil.Expect(t, "the revision", document.Revision, revisionOf(content))
			}
		})
	}
}

// A record is read as facts, not as a bullet list: the head comes back as
// data, the head lines are gone from the blocks, and the title above them
// stays where it was so the outline still lands on it.
func TestReadCarriesARecordsHeadAsData(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.Checkout, "metasystem/plans/designs/headed.md",
		record("A headed design", "design", "design-headed", "accepted", "ledger-sync two-homes",
			"- Cites: design-ledger",
			"- Affects: decision-one-binary",
			"- Governs: g1-s22",
			"- Supersedes: design-reading",
			"- By: wido")+
			"\nThe body opens with prose.\n\n- and then a list, which stays\n")

	document, err := Read(roots, "metasystem/plans/designs/headed.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Require(t, "the head is data", document.Record != nil, true)
	testutil.Expect(t, "the head", *document.Record, Head{
		Kind: "design", ID: "design-headed", Status: "accepted",
		Goals:      []string{"ledger-sync", "two-homes"},
		Cites:      []string{"design-ledger"},
		Affects:    []string{"decision-one-binary"},
		Governs:    []string{"g1-s22"},
		Supersedes: []string{"design-reading"},
		By:         []string{"wido"},
	})
	testutil.Expect(t, "the blocks after the title", blockTypes(document.Blocks),
		[]string{"heading", "paragraph", "list"})
	testutil.Expect(t, "the title is still a heading with its id", document.Headings,
		[]markdown.Heading{{Level: 1, ID: "a-headed-design", Text: "A headed design"}})
}

// A document that declares no head is answered exactly as it was before: no
// record, no relationships, and every block it has.
func TestReadLeavesADocumentThatIsNotARecordAlone(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.Checkout, "metasystem/plans/designs/notes.md", "# Notes\n\n- one\n- two\n")
	plant(t, roots.Checkout, "docs/elsewhere.md", "# Elsewhere\n\n- one\n- two\n")

	for _, id := range []string{"metasystem/plans/designs/notes.md", "docs/elsewhere.md"} {
		document, err := Read(roots, id, readAt)

		testutil.Require(t, "read "+id, err, nil)
		testutil.Expect(t, "no record at "+id, document.Record == nil, true)
		testutil.Expect(t, "nothing references "+id, document.ReferencedBy, []Link{})
		testutil.Expect(t, "nothing supersedes "+id, document.SupersededBy, []Link{})
		testutil.Expect(t, "every block at "+id, blockTypes(document.Blocks), []string{"heading", "list"})
	}
}

// The half of a reference a record cannot declare itself: who names it, once
// each, and which of them replaced it.
func TestReadCarriesWhatReferencesARecord(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.Checkout, "metasystem/plans/designs/successor.md",
		record("The successor", "design", "design-successor", "accepted", "billing",
			"- Supersedes: design-ledger",
			"- Cites: design-ledger")+"\nIt replaces the ledger design.\n")
	plant(t, roots.Checkout, "metasystem/plans/designs/reader.md",
		record("A reader", "design", "design-reader", "draft", "billing",
			"- Cites: design-ledger")+"\nIt rests on the ledger design.\n")

	document, err := Read(roots, "metasystem/plans/designs/ledger.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Expect(t, "referenced by, once each, in path order", document.ReferencedBy, []Link{
		{ID: "design-reader", Title: "A reader", Path: "metasystem/plans/designs/reader.md", Kind: "design"},
		{ID: "design-successor", Title: "The successor", Path: "metasystem/plans/designs/successor.md", Kind: "design"},
	})
	testutil.Expect(t, "superseded by the one that replaced it", document.SupersededBy, []Link{
		{ID: "design-successor", Title: "The successor", Path: "metasystem/plans/designs/successor.md", Kind: "design"},
	})
}

// A blank line between two bullet lists does not end a list: a record whose
// body opens with one has a single list block holding the head and the body
// together. The head's own lines come off the front of it; the body's stay.
func TestReadKeepsABodyThatOpensWithAList(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	plant(t, roots.Checkout, "metasystem/plans/designs/listed.md",
		record("A design whose body is a list", "design", "design-listed", "draft", "billing")+
			"\n- the first thing the body says\n- the second\n")

	document, err := Read(roots, "metasystem/plans/designs/listed.md", readAt)

	testutil.Require(t, "read", err, nil)
	testutil.Require(t, "it is a record", document.Record != nil, true)
	testutil.Expect(t, "the title and what is left of the list", blockTypes(document.Blocks),
		[]string{"heading", "list"})
	testutil.Expect(t, "the head's four lines are gone", itemTexts(document.Blocks[1]),
		[]string{"the first thing the body says", "the second"})
}

// itemTexts is the first line of each item of a list block, flattened enough
// for a test to read what survived.
func itemTexts(block markdown.Block) []string {
	texts := []string{}
	for _, item := range block.Items {
		line := ""
		for _, inner := range item.Blocks {
			for _, inline := range inner.Inlines {
				line += inline.Text
			}
		}
		texts = append(texts, line)
	}
	return texts
}

func blockTypes(blocks []markdown.Block) []string {
	types := []string{}
	for _, block := range blocks {
		types = append(types, block.Type)
	}
	return types
}

// The seam is the only difference between read and Read: without a hook they
// answer the same document.
func TestReadWithoutAHookIsRead(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	plant(t, roots.Checkout, "docs/a.md", "# A\n")

	withHook, err := read(roots, "docs/a.md", readAt, func() {})
	testutil.Require(t, "read through the seam", err, nil)
	plain, err := Read(roots, "docs/a.md", readAt)
	testutil.Require(t, "read", err, nil)

	testutil.Expect(t, "the same document", withHook, plain)
}
