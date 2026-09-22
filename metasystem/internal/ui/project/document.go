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

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
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

// Head is what a record declares about itself, carried as data rather than as
// the bullet list at the top of its text. Every list is a list the browser can
// read as one, so an absent key is an empty array and not a null.
type Head struct {
	Kind       string   `json:"kind"`
	ID         string   `json:"id"`
	Status     string   `json:"status"`
	Areas      []string `json:"areas"`
	Cites      []string `json:"cites"`
	Affects    []string `json:"affects"`
	Governs    []string `json:"governs"`
	Supersedes []string `json:"supersedes"`
	By         []string `json:"by"`
}

// Link is one record on the other end of a relationship, named the way a
// reader needs it: enough to show it and to open it.
type Link struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Kind  string `json:"kind"`
}

// Document is one Markdown file, read once, as it was at readAt.
//
// A file that is a record carries its head as Record, and its head lines are
// not among the blocks: the reading view shows them as facts, and showing them
// twice — once as facts and once as a bullet list — would be showing the same
// four lines in two voices. A file that declares no head carries none of this
// and is answered exactly as it was before.
type Document struct {
	Kind       string `json:"kind"`
	ID         string `json:"id"`
	Title      string `json:"title"`
	Revision   string `json:"revision"`
	Owner      string `json:"owner"`
	Path       string `json:"path"`
	Bytes      int64  `json:"bytes"`
	ModifiedAt string `json:"modifiedAt"`
	ReadAt     string `json:"readAt"`
	State      string `json:"state"`
	Reason     string `json:"reason"`
	Record     *Head  `json:"record"`
	// ReferencedBy is every record whose Cites, Affects or Supersedes names
	// this one — the half of a reference a record cannot declare itself.
	ReferencedBy []Link `json:"referencedBy"`
	// SupersededBy is the part of ReferencedBy that replaced this record. A
	// record appears in both, because being superseded is one way of being
	// referenced and the reader says each of them in its own words.
	SupersededBy []Link             `json:"supersededBy"`
	Headings     []markdown.Heading `json:"headings"`
	Blocks       []markdown.Block   `json:"blocks"`
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
		Kind:         "document",
		ID:           id,
		Title:        titleOf(id, data),
		Owner:        unknownOwner,
		Path:         absolutePath(roots.Checkout, id),
		Bytes:        info.Size(),
		ModifiedAt:   stamp(info.ModTime()),
		ReadAt:       stamp(now),
		State:        stateReadable,
		ReferencedBy: []Link{},
		SupersededBy: []Link{},
		Headings:     []markdown.Heading{},
		Blocks:       []markdown.Block{},
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
	declared(&document, roots, id)
	return document, nil
}

// declared attaches what the project's own resolver knows about this file: the
// head it declares, and the records that name it.
//
// It runs after the document has been read, on the bytes that came back, and
// it opens nothing on the caller's behalf: the id is only compared, as text,
// against the paths the resolver found in the homes it reads for the pane. A
// file that lies beneath none of those homes cannot be a record, so the walk
// is not made at all and the answer is the one the route gave before.
func declared(document *Document, roots Roots, id string) {
	if !beneathAHome(roots, id) {
		return
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return
	}
	var found *resolver.Record
	for index := range read.Records {
		if read.Records[index].Path == id {
			found = &read.Records[index]
			break
		}
	}
	if found == nil {
		return
	}
	document.Record = &Head{
		Kind:       found.Kind,
		ID:         found.ID,
		Status:     found.Status,
		Areas:      list(found.Areas),
		Cites:      list(found.Cites),
		Affects:    list(found.Affects),
		Governs:    list(found.Governs),
		Supersedes: list(found.Supersedes),
		By:         list(found.By),
	}
	document.ReferencedBy, document.SupersededBy = references(read, found.ID)
	document.Blocks = withoutTheHead(document.Blocks, len(found.Head))
}

// references are the records that name this one, each listed once, and the
// part of them that superseded it.
func references(read *resolver.Project, id string) (referenced []Link, superseded []Link) {
	referenced, superseded = []Link{}, []Link{}
	if id == "" {
		return referenced, superseded
	}
	seen, replaced := map[string]bool{}, map[string]bool{}
	for _, reference := range read.ReferencedBy(id) {
		record := read.Record(reference.ID)
		if record == nil {
			continue
		}
		link := Link{ID: record.ID, Title: record.Title, Path: record.Path, Kind: record.Kind}
		if !seen[reference.ID] {
			seen[reference.ID] = true
			referenced = append(referenced, link)
		}
		if reference.Key == "Supersedes" && !replaced[reference.ID] {
			replaced[reference.ID] = true
			superseded = append(superseded, link)
		}
	}
	return referenced, superseded
}

// beneathAHome reports whether this id could name a record at all: it lies
// inside one of the directories the resolver reads. The register is not one of
// them — it is a file of rows, not a record — and a home that is the checkout
// root itself would admit everything, so only a proper prefix counts.
func beneathAHome(roots Roots, id string) bool {
	for _, home := range resolver.Homes(resolver.Roots(roots)) {
		if home.Register || home.Rel == "" || home.Rel == "." {
			continue
		}
		if strings.HasPrefix(id, home.Rel+"/") {
			return true
		}
	}
	return false
}

// withoutTheHead drops the record's head lines from the blocks. The head is
// the bullet list that follows the title, so it is the second block of a file
// whose first is that title, and lines is how many of its items the head
// declared.
//
// The count matters because a blank line between two bullet lists does not end
// a list: a record whose body opens with a list of its own has one list block
// holding both, and dropping the block would drop the body with the head. So
// the head's own items are taken off the front and whatever is left is kept,
// in the shape the parser gave it. A head line the parser refused is not among
// the count and stays visible, which is where a human can see what is wrong
// with it.
func withoutTheHead(blocks []markdown.Block, lines int) []markdown.Block {
	if len(blocks) < 2 || lines <= 0 {
		return blocks
	}
	if blocks[0].Type != "heading" || blocks[0].Level != 1 {
		return blocks
	}
	head := blocks[1]
	if head.Type != "list" || head.Ordered {
		return blocks
	}
	kept := make([]markdown.Block, 0, len(blocks))
	kept = append(kept, blocks[0])
	if len(head.Items) > lines {
		rest := head
		rest.Items = head.Items[lines:]
		kept = append(kept, rest)
	}
	return append(kept, blocks[2:]...)
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
