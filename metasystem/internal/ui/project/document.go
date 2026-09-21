package project

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// ErrNotFound is every refusal this route makes. A request that names a
// directory, a segment this route does not serve, a file that is not there, or
// a name that leaves the checkout gets the same answer, because telling them
// apart would answer questions about a filesystem the caller cannot see.
var ErrNotFound = errors.New("no document at that id")

// maxDocumentBytes is what this route reads. One byte more is read than the
// limit allows, so the limit is decided by what came back and never by a size
// the filesystem reported before the read.
const maxDocumentBytes = 1 << 20

// titleHead is how far into a document the title is looked for.
const titleHead = 4 << 10

// The three states a document is answered in. Only readable carries blocks.
const (
	stateReadable   = "readable"
	stateUnreadable = "unreadable"
	stateTooLarge   = "too-large"
)

// refusedSegments never name a directory on the way to a served document.
// They are compared case-insensitively: APFS is case-insensitive by default,
// so .GIT/x.md reaches .git/x.md, while os.Root folds nothing of its own.
var refusedSegments = []string{".git", "node_modules", "artifacts", "bin"}

// Document is one Markdown file, read once, as it was at readAt.
type Document struct {
	Kind       string             `json:"kind"`
	ID         string             `json:"id"`
	Title      string             `json:"title"`
	Revision   string             `json:"revision"`
	Owner      string             `json:"owner"`
	Path       string             `json:"path"`
	Bytes      int64              `json:"bytes"`
	ModifiedAt string             `json:"modifiedAt"`
	ReadAt     string             `json:"readAt"`
	State      string             `json:"state"`
	Reason     string             `json:"reason"`
	Headings   []markdown.Heading `json:"headings"`
	Blocks     []markdown.Block   `json:"blocks"`
}

// Read answers one document by its checkout-relative id.
//
// The order is the boundary, and it is the whole of it: the id is judged as
// text, one anchored open yields the only descriptor, that descriptor is
// fstat'd and read within a bound, and the bytes that came back are the only
// thing parsed and hashed. No pathname is resolved and reopened, so there is no
// window in which a name can be swapped for something the caller cannot reach.
func Read(roots Roots, id string, now time.Time) (Document, error) {
	return read(roots, id, now, nil)
}

// read is Read with the seam the race test needs. beforeOpen runs after every
// check on the id and immediately before the one call that yields a
// descriptor, so a test can swap the leaf or a parent in the window an
// implementation that resolved a pathname and reopened it would leave open.
func read(roots Roots, id string, now time.Time, beforeOpen func()) (Document, error) {
	if !admissibleID(id) {
		return Document{}, ErrNotFound
	}
	root, err := os.OpenRoot(roots.Checkout)
	if err != nil {
		return Document{}, fmt.Errorf("cannot open the checkout at %s: %w", roots.Checkout, err)
	}
	defer func() { _ = root.Close() }()

	if beforeOpen != nil {
		beforeOpen()
	}
	file, err := root.Open(id)
	if err != nil {
		// Every failure here is the same 404. A name that escapes the root is
		// not fs.ErrNotExist, and nothing tests for it: the caller learns that
		// this checkout serves no document at that id, and nothing else.
		return Document{}, ErrNotFound
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return Document{}, ErrNotFound
	}
	if !info.Mode().IsRegular() {
		// A directory opens through os.Root, and is refused here on the
		// descriptor rather than by a second look at the name.
		return Document{}, ErrNotFound
	}
	data, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("cannot read the document at %s: %w", id, err)
	}

	document := Document{
		Kind:       "document",
		ID:         id,
		Title:      titleOf(id, data),
		Owner:      unknownOwner,
		Path:       absolutePath(roots.Checkout, id),
		Bytes:      info.Size(),
		ModifiedAt: stamp(info.ModTime()),
		ReadAt:     stamp(now),
		State:      stateReadable,
		Headings:   []markdown.Heading{},
		Blocks:     []markdown.Block{},
	}
	owner, ownerReason := ownerOf(roots.Installation, document.Path)
	document.Owner = owner

	if len(data) > maxDocumentBytes {
		// Decided from the bytes that came back, never from the size above: a
		// file that grew or was replaced after the fstat cannot make this
		// allocate beyond the limit and one byte. Not every byte was read, so
		// there is no revision to name.
		document.State = stateTooLarge
		document.Reason = fmt.Sprintf("the file is larger than %s, so it is not read here", humanLimit())
		return document, nil
	}
	document.Revision = revisionOf(data)
	if !utf8.Valid(data) {
		document.State = stateUnreadable
		document.Reason = "the file is not valid UTF-8 text"
		return document, nil
	}
	// The reason belongs to the state when there is one to give; a readable
	// document has none, and lends the field to the ownership oracle so that an
	// "unknown" owner is never shown without saying why.
	document.Reason = ownerReason
	parsed := markdown.Parse(data)
	classify(parsed.Blocks, id)
	document.Headings = parsed.Headings
	document.Blocks = parsed.Blocks
	return document, nil
}

// admissibleID judges the requested id as text, before anything is opened. It
// is an allow test: a name has to look exactly like a checkout-relative
// Markdown path to get past it, which is stricter than the filesystem and safe
// in that direction.
func admissibleID(id string) bool {
	if id == "" || id == "." || strings.HasPrefix(id, "/") {
		return false
	}
	// Clean refuses a trailing slash, an empty or "." segment, and a ".." that
	// collapses; a leading ".." survives Clean, so the segments are read too.
	if path.Clean(id) != id {
		return false
	}
	if !strings.HasSuffix(id, ".md") {
		return false
	}
	for _, segment := range strings.Split(id, "/") {
		if segment == ".." {
			return false
		}
		for _, refused := range refusedSegments {
			if strings.EqualFold(segment, refused) {
				return false
			}
		}
	}
	return true
}

// revisionOf is the Git blob object id of the bytes read: the repository's own
// name for this content, equal to `git hash-object <file>`, which a later
// typed reference can pin.
func revisionOf(data []byte) string {
	digest := sha1.New()
	_, _ = io.WriteString(digest, "blob "+strconv.Itoa(len(data))+"\x00")
	_, _ = digest.Write(data)
	return "blob:" + hex.EncodeToString(digest.Sum(nil))
}

// titleOf is the first line beginning "# " within the head of the file, and
// the file's name when there is none or when what is there is not text.
func titleOf(id string, data []byte) string {
	head := data
	if len(head) > titleHead {
		head = head[:titleHead]
	}
	for _, line := range strings.Split(string(head), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if !strings.HasPrefix(line, "# ") {
			continue
		}
		title := strings.TrimSpace(strings.TrimPrefix(line, "# "))
		if title != "" && utf8.ValidString(title) {
			return title
		}
		break
	}
	return path.Base(id)
}

// unknownOwner is the answer when the ownership oracle cannot give one. It is
// never a guess: the interface says the ownership is unknown and why.
const unknownOwner = "unknown"

// ownerOf asks the engine's ownership oracle about a path. This is metadata
// only: the oracle resolves path ancestors of its own, and nothing it answers
// selects, reopens, hashes, or parses a single byte of a document.
func ownerOf(installation, absolute string) (owner string, reason string) {
	ownership, _, err := stateroot.OwnerForInstallation(installation, absolute)
	if err != nil {
		return unknownOwner, "the ownership of this path is unknown: " + err.Error()
	}
	return string(ownership), ""
}

func absolutePath(checkout, id string) string {
	return filepath.Join(checkout, filepath.FromSlash(id))
}

func stamp(at time.Time) string {
	return at.Format(time.RFC3339)
}

func humanLimit() string {
	return strconv.Itoa(maxDocumentBytes>>20) + " MiB"
}
