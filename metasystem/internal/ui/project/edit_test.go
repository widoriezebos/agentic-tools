package project

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// Editing one document in place, against the same fixtures the pane and the
// read route are read from.
//
// Every refusal asserts the file as well as the answer: a route that refused
// with the right sentence after writing the bytes anyway would pass an
// assertion on the answer alone, and the whole promise of this surface is that
// a refused save leaves the checkout exactly as it was.

// The two documents these tests edit: one plain, beneath no home, and one
// record the resolver has jurisdiction over.
const (
	plainDocument  = "metasystem/docs/architecture.md"
	recordDocument = "metasystem/plans/designs/ledger.md"
)

// checkout is every file beneath a root, by checkout-relative path, so a test
// can say that nothing at all changed rather than that one file did not.
func checkout(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || !entry.Type().IsRegular() {
			// A symlink is a name and not content: what it points at is not
			// this checkout's, and following it would read what these tests
			// planted outside on purpose.
			return err
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(name)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

// revisionAt is the revision the read route would hand a caller for one
// document, which is the revision a save of it has to carry back.
func revisionAt(t *testing.T, roots Roots, id string) string {
	t.Helper()
	document, err := Read(roots, id, readAt)
	if err != nil {
		t.Fatalf("read %s: %v", id, err)
	}
	return document.Revision
}

// The read route hands over the bytes it read, beside the revision it took
// over them, because an editor opens the source and saves against the pair.
func TestReadCarriesTheSourceItHashed(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	plant(t, roots.Checkout, "docs/notes.md", "# Notes\n\nOne line.\n")

	document, err := Read(roots, "docs/notes.md", readAt)

	testutil.Require(t, "read the document", err, nil)
	testutil.Expect(t, "the source", document.Source, "# Notes\n\nOne line.\n")
	testutil.Expect(t, "the revision is over those bytes",
		document.Revision, revisionOf([]byte(document.Source)))
}

// A file that was not read to its end, or that is not text, carries no source:
// there is nothing to open an editor on, and half a file would be worse.
func TestReadCarriesNoSourceForWhatItCannotShow(t *testing.T) {
	t.Parallel()

	roots := adoptedFixture(t)
	plantBytes(t, roots.Checkout, "docs/binary.md", []byte{0xff, 0xfe, 0xfd})
	plantBytes(t, roots.Checkout, "docs/huge.md", make([]byte, maxDocumentBytes+1))

	for _, id := range []string{"docs/binary.md", "docs/huge.md"} {
		document, err := Read(roots, id, readAt)
		testutil.Require(t, "read "+id, err, nil)
		testutil.Expect(t, id+" carries no source", document.Source, "")
	}
}

// A plain document goes round: the text lands byte for byte, the answer is the
// document read back from disk, and the revision is the one a next save has to
// carry.
func TestEditRoundTripsAPlainDocument(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	source := "# The engine\n\nProse, rewritten in the browser.\n\n## A second heading\n"

	saved, err := EditDocument(roots, plainDocument, source, revisionAt(t, roots, plainDocument), readAt)

	testutil.Require(t, "save the document", err, nil)
	testutil.Expect(t, "the file", fileAt(t, roots, plainDocument), source)
	testutil.Expect(t, "the answer's source", saved.Source, source)
	testutil.Expect(t, "the answer's revision", saved.Revision, revisionOf([]byte(source)))
	testutil.Expect(t, "the answer's title", saved.Title, "The engine")
	testutil.Expect(t, "the answer declares no head", saved.Record, (*Head)(nil))
	testutil.Expect(t, "the new heading is among the blocks", headingTexts(saved),
		[]string{"The engine", "A second heading"})
	// The answer is the read, so a second save of it is accepted with what it
	// carried: the pair the page now holds is the pair on disk.
	_, again := EditDocument(roots, plainDocument, source+"\nAnd more.\n", saved.Revision, readAt)
	testutil.Expect(t, "the answer's revision is the one the next save needs", again, nil)
}

// A record saves, and the read that answers it reads the new head and the new
// blocks: the status the browser shows is the one now on disk.
func TestEditSavesARecordAndReadsItBack(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	source := record("The ledger", "design", "design-ledger", "accepted", "ledger-sync") +
		"\n## Outcome\n\nThe ledger syncs on every landing.\n"

	saved, err := EditDocument(roots, recordDocument, source, revisionAt(t, roots, recordDocument), readAt)

	testutil.Require(t, "save the record", err, nil)
	testutil.Expect(t, "the file", fileAt(t, roots, recordDocument), source)
	testutil.Require(t, "the answer declares a head", saved.Record != nil, true)
	testutil.Expect(t, "the status it now carries", saved.Record.Status, "accepted")
	testutil.Expect(t, "the goals it now carries", saved.Record.Goals, []string{"ledger-sync"})
	testutil.Expect(t, "the new heading is among the blocks", headingTexts(saved),
		[]string{"The ledger", "Outcome"})
	// The head is data, not text, so it is not among the blocks a reader is
	// shown — which is the read route's own rule, holding over what was saved.
	testutil.Expect(t, "the head is not shown twice", strings.Contains(blockText(saved), "- Kind:"), false)
}

// A head the project refuses is a refusal carrying the check verb's own
// problems, and the file is not touched. One case per rule the check makes.
func TestEditRefusesAHeadTheProjectRefuses(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct{ what, source, says string }{
		{
			"a status the grammar does not carry",
			record("The ledger", "design", "design-ledger", "invented", "ledger-sync"),
			"the status invented is not one of",
		},
		{
			"a kind the grammar does not carry",
			record("The ledger", "invented", "design-ledger", "draft", "ledger-sync"),
			"the kind invented is not one of",
		},
		{
			"a goal the ledger does not carry",
			record("The ledger", "design", "design-ledger", "draft", "logistics"),
			"the goal logistics is not in the ledger",
		},
		{
			"a key declared twice",
			record("The ledger", "design", "design-ledger", "draft", "ledger-sync", "- Kind: decision"),
			"the head declares Kind twice",
		},
		{
			"a head line that is not a declaration",
			record("The ledger", "design", "design-ledger", "draft", "ledger-sync", "- not a declaration"),
			"the head line is not a `- Key: value` declaration",
		},
		{
			"an id another record already declares",
			record("The ledger", "design", "design-reading", "draft", "ledger-sync"),
			"the id design-reading is already declared by",
		},
		{
			"no head at all, where a record declared one",
			"# The ledger\n\nProse where the head was.\n",
			"this text does not declare itself a record",
		},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			roots := selfHostedFixture(t)
			seed(t, roots)
			before := checkout(t, roots.Checkout)

			_, err := EditDocument(roots, recordDocument, refused.source, revisionAt(t, roots, recordDocument), readAt)

			refusal := refusalOf(t, err)
			testutil.Expect(t, "the kind of refusal", refusal.Kind, RefusalProject)
			testutil.Require(t, "it carries problems", len(refusal.Problems) > 0, true)
			testutil.Expect(t, "what the problem says",
				strings.Contains(problemMessages(refusal), refused.says), true)
			testutil.Expect(t, "the checkout is untouched", checkout(t, roots.Checkout), before)
		})
	}
}

// The resolver's jurisdiction is the whole of the rule: a document beneath
// none of the homes is never read as a record, so nothing about a head is
// checked over it and text that looks like one is saved as the text it is.
func TestEditChecksNoHeadOverADocumentTheResolverDoesNotRead(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	source := record("The engine", "invented", "design-ledger", "invented", "logistics")

	saved, err := EditDocument(roots, plainDocument, source, revisionAt(t, roots, plainDocument), readAt)

	testutil.Require(t, "save the document", err, nil)
	testutil.Expect(t, "the file", fileAt(t, roots, plainDocument), source)
	testutil.Expect(t, "it is still not a record", saved.Record, (*Head)(nil))
}

// A revision that is not what is on disk is the file having changed underneath,
// and the save is refused rather than resolved. Nothing is written.
func TestEditRefusesAStaleRevision(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	held := revisionAt(t, roots, plainDocument)
	plant(t, roots.Checkout, plainDocument, "# The engine\n\nSomebody else wrote this.\n")
	before := checkout(t, roots.Checkout)

	_, err := EditDocument(roots, plainDocument, "# The engine\n\nAnd I wrote this.\n", held, readAt)

	refusal := refusalOf(t, err)
	testutil.Expect(t, "the kind of refusal", refusal.Kind, RefusalStale)
	testutil.Expect(t, "what it says", refusal.Message, staleMessage)
	testutil.Expect(t, "the checkout is untouched", checkout(t, roots.Checkout), before)
}

// Text past what the read route reads back would write a file this interface
// could no longer open, and text that is not text is not a document at all.
func TestEditRefusesWhatItCouldNotReadBack(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what   string
		source string
		kind   RefusalKind
	}{
		{"more than the limit", strings.Repeat("x", maxDocumentBytes+1), RefusalTooLarge},
		{"bytes that are not text", "# A title\n\n" + string([]byte{0xff, 0xfe}) + "\n", RefusalBad},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()
			roots := selfHostedFixture(t)
			seed(t, roots)
			before := checkout(t, roots.Checkout)

			_, err := EditDocument(roots, plainDocument, refused.source, revisionAt(t, roots, plainDocument), readAt)

			testutil.Expect(t, "the kind of refusal", refusalOf(t, err).Kind, refused.kind)
			testutil.Expect(t, "the checkout is untouched", checkout(t, roots.Checkout), before)
		})
	}
}

// Every id the read route refuses, the save refuses the same way and with the
// same error, so there is one boundary and not two.
func TestEditRefusesEveryIDTheReadRefuses(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	away := outside(t)
	plant(t, away, "secrets.md", "# Outside\n")
	link(t, filepath.Join(away, "secrets.md"), filepath.Join(roots.Checkout, "escape.md"))
	before := checkout(t, roots.Checkout)

	for _, id := range []string{
		"../secrets.md",
		"/etc/passwd",
		"metasystem/../../secrets.md",
		"metasystem/docs/../docs/architecture.md",
		"metasystem/docs",
		"metasystem/docs/architecture.txt",
		".git/config.md",
		"metasystem/node_modules/a.md",
		"nothing/here.md",
		"escape.md",
		"",
	} {
		_, err := EditDocument(roots, id, "# Anything\n", "blob:whatever", readAt)
		testutil.Expect(t, "save "+id+" is the read's own refusal", errors.Is(err, ErrNotFound), true)
	}
	testutil.Expect(t, "the checkout is untouched", checkout(t, roots.Checkout), before)
}

// The file keeps the mode it had. A save that published a fresh temp file
// under its own permissions would quietly narrow a document every time it was
// edited.
func TestEditKeepsTheFilesMode(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	absolute := filepath.Join(roots.Checkout, filepath.FromSlash(plainDocument))
	if err := os.Chmod(absolute, 0o640); err != nil {
		t.Fatalf("chmod %s: %v", plainDocument, err)
	}

	_, err := EditDocument(roots, plainDocument, "# The engine\n\nEdited.\n", revisionAt(t, roots, plainDocument), readAt)

	testutil.Require(t, "save the document", err, nil)
	info, err := os.Stat(absolute)
	testutil.Require(t, "stat the document", err, nil)
	testutil.Expect(t, "the mode", info.Mode().Perm(), os.FileMode(0o640))
}

// A save leaves no temporary file behind: the one it published under is the
// only name it made, and the rename consumed it.
func TestEditLeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)

	_, err := EditDocument(roots, plainDocument, "# The engine\n\nEdited.\n", revisionAt(t, roots, plainDocument), readAt)

	testutil.Require(t, "save the document", err, nil)
	for name := range checkout(t, roots.Checkout) {
		testutil.Expect(t, name+" is not a leftover", strings.HasSuffix(name, ".tmp"), false)
	}
}

// The preview renders what is being typed, through the same parser the read
// route renders a file with, and touches nothing.
func TestPreviewRendersAndWritesNothing(t *testing.T) {
	t.Parallel()

	roots := selfHostedFixture(t)
	seed(t, roots)
	before := checkout(t, roots.Checkout)

	preview, err := PreviewDocument("# A title\n\nA paragraph.\n\n- an item\n")

	testutil.Require(t, "render the preview", err, nil)
	testutil.Expect(t, "the blocks", blockTypes(preview.Blocks), []string{"heading", "paragraph", "list"})
	testutil.Expect(t, "the checkout is untouched", checkout(t, roots.Checkout), before)
}

// The preview reads back what the reader reads back, so the same two things
// are refused: more text than the limit, and bytes that are not text.
func TestPreviewRefusesWhatTheReaderCouldNotShow(t *testing.T) {
	t.Parallel()

	for _, refused := range []struct {
		what   string
		source string
		kind   RefusalKind
	}{
		{"more than the limit", strings.Repeat("x", maxDocumentBytes+1), RefusalTooLarge},
		{"bytes that are not text", string([]byte{0xff}), RefusalBad},
	} {
		t.Run(refused.what, func(t *testing.T) {
			t.Parallel()

			_, err := PreviewDocument(refused.source)

			testutil.Expect(t, "the kind of refusal", refusalOf(t, err).Kind, refused.kind)
		})
	}
}

/* -------------------------------------------------------------- the words -- */

func headingTexts(document Document) []string {
	texts := []string{}
	for _, heading := range document.Headings {
		texts = append(texts, heading.Text)
	}
	return texts
}

// blockText is every word of a document's blocks, joined, which is enough to
// say that something is or is not shown.
func blockText(document Document) string {
	var out strings.Builder
	var inlines func(nodes []markdown.Inline)
	inlines = func(nodes []markdown.Inline) {
		for _, node := range nodes {
			out.WriteString(node.Text)
			inlines(node.Inlines)
		}
	}
	var walk func(blocks []markdown.Block)
	walk = func(blocks []markdown.Block) {
		for _, block := range blocks {
			out.WriteString(block.Text)
			inlines(block.Inlines)
			for _, item := range block.Items {
				walk(item.Blocks)
			}
			walk(block.Blocks)
		}
	}
	walk(document.Blocks)
	return out.String()
}

func problemMessages(refusal *Refusal) string {
	messages := []string{}
	for _, problem := range refusal.Problems {
		messages = append(messages, problem.Message)
	}
	return strings.Join(messages, "\n")
}
