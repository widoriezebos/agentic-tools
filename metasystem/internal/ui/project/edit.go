package project

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/markdown"
)

// Editing one document in place.
//
// This is the one write in this package that names a file, and it names it the
// way the read does: the same id, judged by the same admissibleID, opened
// through the same anchored root. A human editing a document in the browser is
// editing the document they are reading, so there is nothing else it could
// name — and because the id is the read's own, a name this route would write
// to is exactly a name the read route would serve.
//
// Three things hold beside the rules every other write keeps:
//
//   - The save is against the read. A caller sends back the revision it was
//     given, and a file whose bytes no longer hash to it is not written at
//     all: the two edits are not merged, and neither is silently lost.
//   - A record still has to be a record. Where the resolver has jurisdiction
//     over this path, what is about to be written goes through the resolver's
//     own parser and the check verb's own rules first, and a refusal carries
//     those problems and leaves the file exactly as it was.
//   - Nothing else about the bytes is touched. No trailing newline is added,
//     no line ending is changed, no head is reordered. What was typed is what
//     lands, and the file keeps the mode it had.
//
// The answer is the document read again from disk, so the page re-renders from
// what is there rather than from what this package believes it wrote.

// Preview is source a human is typing, rendered through the same parser the
// read route renders a file with.
//
// It carries blocks and nothing else: there is no file behind it, so there is
// no path, no revision, no ownership and no record head to answer with. A
// relative link in it stays text for the same reason — a link is resolved
// against the directory of the document it was written in, and a preview is
// not yet written anywhere.
type Preview struct {
	Blocks []markdown.Block `json:"blocks"`
}

// PreviewDocument renders source. It opens nothing and writes nothing.
func PreviewDocument(source string) (Preview, error) {
	if refusal := admissibleSource(source); refusal != nil {
		return Preview{}, refusal
	}
	parsed := markdown.Parse([]byte(source))
	return Preview{Blocks: parsed.Blocks}, nil
}

// EditDocument replaces one document's bytes with the source a human typed,
// and answers the document as the read route now reads it.
//
// revision is the one the caller was given when it opened the file. It is the
// whole of the concurrency story: the bytes on disk are hashed again here, and
// a hash that is not this one means the file changed underneath, which is said
// rather than resolved.
func EditDocument(roots Roots, id, source, revision string, now time.Time) (Document, error) {
	if !admissibleID(id) {
		// The same refusal, for the same reason, as the read: a caller learns
		// that this checkout serves no document at that id, and nothing else.
		return Document{}, ErrNotFound
	}
	// The id and the text are both judged as text, before anything is opened
	// on their account: a request that could not be written whatever is on
	// disk is refused without touching the filesystem at all.
	if refusal := admissibleSource(source); refusal != nil {
		return Document{}, refusal
	}
	root, err := os.OpenRoot(roots.Checkout)
	if err != nil {
		return Document{}, fmt.Errorf("cannot open the checkout at %s: %w", roots.Checkout, err)
	}
	defer func() { _ = root.Close() }()

	mode, refusal, err := held(root, id, revision)
	if err != nil || refusal != nil {
		return Document{}, join(err, refusal)
	}
	if refusal, err := refusedByTheProject(roots, id, source); err != nil || refusal != nil {
		return Document{}, join(err, refusal)
	}
	if err := replace(root, id, source, mode); err != nil {
		return Document{}, err
	}
	return read(roots, id, now, nil)
}

// held reads the file this save is against and reports the mode it carries, so
// the file that is written is the file that was hashed.
//
// Every way of not being there is the read route's own 404. A file that is
// larger than the limit was never read to its end, so it has no revision to
// have been given and cannot match one: it is answered as the changed file it
// would have to be for a caller to be holding a revision of it at all.
func held(root *os.Root, id, revision string) (os.FileMode, *Refusal, error) {
	file, err := root.Open(id)
	if err != nil {
		return 0, nil, ErrNotFound
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return 0, nil, ErrNotFound
	}
	data, err := io.ReadAll(io.LimitReader(file, maxDocumentBytes+1))
	if err != nil {
		return 0, nil, fmt.Errorf("cannot read the document at %s: %w", id, err)
	}
	if len(data) > maxDocumentBytes || revisionOf(data) != revision {
		return 0, refuse(RefusalStale, staleMessage), nil
	}
	return info.Mode(), nil, nil
}

// staleMessage is what a caller holding an old revision is told, in the words
// the interface shows: what happened, and that nothing was written.
const staleMessage = "the file changed since you opened it"

// admissibleSource judges the text before anything is opened on its account:
// more than the read route reads back, or bytes that are not text, and the
// file would be one this interface could no longer show.
func admissibleSource(source string) *Refusal {
	if len(source) > maxDocumentBytes {
		return refuse(RefusalTooLarge,
			"the text is larger than "+humanLimit()+", which is more than this interface reads back")
	}
	if !utf8.ValidString(source) {
		return refuse(RefusalBad, "the text is not valid UTF-8")
	}
	return nil
}

// refusedByTheProject puts what is about to be written through the resolver's
// own parser and the check verb's own rules, where this path is one the
// resolver reads records from.
//
// The jurisdiction is the resolver's, not this package's: a file beneath none
// of the homes is never read as a record, so nothing here may refuse it over a
// head it does not have and would not be judged by. Beneath a home, the check
// runs when the file is a record today or would be one after the save — so a
// record whose head has been deleted is refused as the record it stops being,
// and a plain document that grows a head is refused if that head is bad.
//
// In a home of untyped history the jurisdiction is narrower still, by the same
// rule the resolver reads it under: a file that declares no Kind is prose the
// typing pass has not reached, so the legacy bullets it opens with are not a
// head and are not judged as one.
func refusedByTheProject(roots Roots, id, source string) (*Refusal, error) {
	if !beneathAHome(roots, id) {
		return nil, nil
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return nil, err
	}
	written, _, declares := resolver.ParseRecord(id, source)
	if declares && historicalHome(roots, id) && !written.Declares("Kind") {
		return nil, nil
	}
	if !declares && !isRecord(read, id) {
		return nil, nil
	}
	if problems := wouldRefuse(read, id, source); len(problems) > 0 {
		return &Refusal{
			Kind:     RefusalProject,
			Message:  "the record this would write is one the project refuses",
			Problems: problems,
		}, nil
	}
	return nil, nil
}

func isRecord(read *resolver.Project, id string) bool {
	for index := range read.Records {
		if read.Records[index].Path == id {
			return true
		}
	}
	return false
}

// replace writes source over the document at id atomically, keeping its mode.
//
// A temp file is filled beside the target and renamed over it, so a reader
// sees the old bytes or the new ones and never a half-written file. Both go
// through the same anchored root the read went through, so the publication
// cannot land outside the checkout either.
//
// The mode is set on the descriptor rather than on the name: a fresh file is
// created 0600 and masked by the umask besides, and a document that was group
// readable has to stay group readable. Setting it through the open file means
// there is no name in between for anything to be swapped at.
func replace(root *os.Root, id, source string, mode os.FileMode) error {
	temporary, file, err := temporaryBeside(root, id)
	if err != nil {
		return fmt.Errorf("cannot write %s: %w", id, err)
	}
	written := func() error {
		if _, err := io.WriteString(file, source); err != nil {
			return err
		}
		if err := file.Chmod(mode.Perm()); err != nil {
			return err
		}
		if err := file.Sync(); err != nil {
			return err
		}
		return file.Close()
	}()
	if written != nil {
		_ = file.Close()
		_ = root.Remove(temporary)
		return fmt.Errorf("cannot write %s: %w", id, written)
	}
	if err := root.Rename(temporary, id); err != nil {
		_ = root.Remove(temporary)
		return fmt.Errorf("cannot write %s: %w", id, err)
	}
	// Published: the new content IS the file from here. The directory sync
	// that makes the rename survive a crash is attempted and its failure is
	// not reported, because a committed write reported as a failure is the one
	// divergence this model exists to prevent.
	if directory, err := root.Open(pathOf(id)); err == nil {
		_ = directory.Sync()
		_ = directory.Close()
	}
	return nil
}

// temporaryBeside creates a file next to the target under a name nothing else
// holds, and answers the name and the open file.
//
// The name is hidden and does not end in .md, so a temp file that outlives a
// crash is not walked as a record and is not served as a document. O_EXCL and
// a fresh random suffix are what make it nobody else's file.
func temporaryBeside(root *os.Root, id string) (string, *os.File, error) {
	directory, base := pathOf(id), path.Base(id)
	for attempt := 0; attempt < 8; attempt++ {
		var suffix [8]byte
		if _, err := rand.Read(suffix[:]); err != nil {
			return "", nil, err
		}
		name := path.Join(directory, "."+base+"."+hex.EncodeToString(suffix[:])+".tmp")
		file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if !os.IsExist(err) {
			return "", nil, err
		}
	}
	return "", nil, fmt.Errorf("cannot make a temporary file beside %s", id)
}

// pathOf is the directory an id lies in, as os.Root reads a name: "." for a
// file at the root of the checkout, which path.Join then drops again.
func pathOf(id string) string {
	directory := path.Dir(id)
	if directory == "" || strings.HasPrefix(directory, "/") {
		return "."
	}
	return directory
}

// join answers whichever of the two is there, so a caller that has both kinds
// of not-writing to report has one return to make.
func join(err error, refusal *Refusal) error {
	if err != nil {
		return err
	}
	if refusal == nil {
		return nil
	}
	return refusal
}
