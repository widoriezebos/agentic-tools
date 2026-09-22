// Package project answers the interface's Project section: the pane over the
// project's declared memory, and the anchored, bounded read of one document.
//
// It decides nothing about what a record is. The resolver in internal/project
// reads the homes, parses the heads, and collects every refusal; this package
// carries that answer to the browser in the shape the pane reads, adds the
// instant it was read at, and lists the rest of the checkout's Markdown by
// path, with no kind claimed, so a document that declares nothing is still
// reachable.
//
// Nothing here writes, and nothing infers a kind from a filename. A record
// with a problem is listed all the same, with the problem beside it, because a
// pane that hid it would be hiding the one thing a human has to act on.
package project

import (
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// Roots names the three directories this package reads from. It is declared
// here, rather than imported from the lifecycle slice, for the reason that
// slice's own comment gives: taking its types closes an import loop. Its
// fields are the resolver's, so the one conversion below is the whole of the
// handover.
type Roots struct{ Checkout, Installation, StateRoot string }

// SchemaVersion is the shape of the project resource the interface reads. It
// is 3: the first was the catalogue of canonical documents, which guessed a
// kind from a filename; the second carried what the records declare; this one
// carries, beside each of them, the record's own first words.
const SchemaVersion = 3

// Pane is the whole of Project, read once, as it was at readAt.
type Pane struct {
	SchemaVersion int        `json:"schemaVersion"`
	ReadAt        string     `json:"readAt"`
	Areas         []Area     `json:"areas"`
	Records       []Record   `json:"records"`
	Intent        Book       `json:"intent"`
	Doctrine      Book       `json:"doctrine"`
	Questions     []Question `json:"questions"`
	Problems      []Problem  `json:"problems"`
	Documents     []File     `json:"documents"`
}

// Area is one slug the intent index declares, with the name it reads by.
type Area struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// Record is one declared document: what it says it is, where it lives, and
// the first words of its own body. The summary is a convention rather than a
// key — a record whose body opens with a table has none — so it is carried as
// the empty string rather than as an absence the pane must explain.
type Record struct {
	Kind    string   `json:"kind"`
	ID      string   `json:"id"`
	Status  string   `json:"status"`
	Areas   []string `json:"areas"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Home    string   `json:"home"`
	Summary string   `json:"summary"`
}

// Book is one of the two indexes and the reading order it names. A home whose
// index.md is absent or declares no head is a book with no index, which the
// pane says rather than invents.
type Book struct {
	Index    *Record   `json:"index"`
	Chapters []Chapter `json:"chapters"`
}

// Chapter is one line of a book's reading order: a record named by id, or a
// document bound by its checkout-relative path and left where it is. Exactly
// one of the two is carried, and the title is the index's own word for it.
type Chapter struct {
	ID    string `json:"id,omitempty"`
	Path  string `json:"path,omitempty"`
	Title string `json:"title"`
	// Summary is the chapter's own first words: the record's, or, for a bound
	// document, the first paragraph after whatever opens it.
	Summary string `json:"summary"`
}

// Question is one row of the register.
type Question struct {
	ID       string   `json:"id"`
	Opened   string   `json:"opened"`
	Question string   `json:"question"`
	Areas    []string `json:"areas"`
	Status   string   `json:"status"`
}

// Problem is one refusal the check verb would print, anchored where a human
// can act on it.
type Problem struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Message string `json:"message"`
}

// File is one of the checkout's other Markdown documents: a path the reading
// route serves and the title its first heading gives it. No kind is claimed.
type File struct {
	Path  string `json:"path"`
	Title string `json:"title"`
}

// ReadPane reads the homes and the checkout once. It is called per request: a
// record written while the server runs is read without a restart, and "read
// at" says how old what the human is looking at is.
func ReadPane(roots Roots, now time.Time) (Pane, error) {
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Pane{}, err
	}
	pane := Pane{
		SchemaVersion: SchemaVersion,
		ReadAt:        stamp(now),
		Areas:         areasOf(read),
		Records:       recordsOf(read),
		Intent:        bookOf(roots, read, resolver.KindIntent),
		Doctrine:      bookOf(roots, read, resolver.KindDoctrine),
		Questions:     questionsOf(read),
		Problems:      problemsOf(read),
	}
	files, err := documents(roots.Checkout)
	if err != nil {
		return Pane{}, err
	}
	pane.Documents = files
	return pane, nil
}

// areasOf is the declared areas in declared order. A slug declared twice is
// one area, and the project as a whole is not one of them: the pane's own
// first row is everything, which is what naming it would mean.
func areasOf(read *resolver.Project) []Area {
	areas := []Area{}
	seen := map[string]bool{}
	for _, area := range read.Areas {
		if area.Slug == resolver.ProjectArea || seen[area.Slug] {
			continue
		}
		seen[area.Slug] = true
		areas = append(areas, Area{Slug: area.Slug, Name: area.Name})
	}
	return areas
}

// recordsOf is every record the resolver listed, in its order, whether or not
// it carries a refusal.
func recordsOf(read *resolver.Project) []Record {
	records := make([]Record, 0, len(read.Records))
	for _, record := range read.Records {
		records = append(records, describe(record))
	}
	return records
}

func describe(record resolver.Record) Record {
	return Record{
		Kind:    record.Kind,
		ID:      record.ID,
		Status:  record.Status,
		Areas:   list(record.Areas),
		Title:   record.Title,
		Path:    record.Path,
		Home:    record.Home,
		Summary: summaryOf(record.Body),
	}
}

// bookOf is one book's index and its reading order. A chapter the index left
// unnamed is titled by what it binds, so no row in the pane is blank.
func bookOf(roots Roots, read *resolver.Project, kind string) Book {
	book := Book{Chapters: []Chapter{}}
	for _, found := range read.Books {
		if found.Kind != kind || found.Index == nil {
			continue
		}
		index := describe(*found.Index)
		book.Index = &index
		for _, chapter := range found.Chapters {
			book.Chapters = append(book.Chapters, chapterOf(roots, read, chapter))
		}
		break
	}
	return book
}

// chapterOf carries a chapter's title and its own first words. A chapter that
// names a record borrows the summary already read from it; a chapter that
// binds a document is read for its first paragraph, once, within the same
// bound the title read takes.
func chapterOf(roots Roots, read *resolver.Project, chapter resolver.Chapter) Chapter {
	out := Chapter{ID: chapter.ID, Path: chapter.Doc, Title: chapter.Title}
	if chapter.Doc != "" {
		out.Summary = summaryOfFile(filepath.Join(roots.Checkout, filepath.FromSlash(chapter.Doc)))
		if out.Title == "" {
			out.Title = path.Base(chapter.Doc)
		}
		return out
	}
	if record := read.Record(chapter.ID); record != nil {
		out.Summary = summaryOf(record.Body)
		if out.Title == "" {
			out.Title = record.Title
		}
		return out
	}
	if out.Title == "" {
		out.Title = chapter.ID
	}
	return out
}

func questionsOf(read *resolver.Project) []Question {
	questions := make([]Question, 0, len(read.Questions))
	for _, question := range read.Questions {
		questions = append(questions, Question{
			ID:       question.ID,
			Opened:   question.Opened,
			Question: question.Text,
			Areas:    list(question.Areas),
			Status:   question.Status,
		})
	}
	return questions
}

// problemsOf is whatever the check verb would refuse, so the pane can show a
// broken record rather than hide it.
func problemsOf(read *resolver.Project) []Problem {
	problems := make([]Problem, 0, len(read.Problems))
	for _, problem := range read.Problems {
		problems = append(problems, Problem{Path: problem.Path, Line: problem.Line, Message: problem.Message})
	}
	return problems
}

// documents lists the rest of the checkout's Markdown, in path order: every
// file the reading route would serve, titled by its first heading or by its
// name. The route's own admissibility test decides what is listed, so the pane
// never offers a path the route would refuse.
//
// A directory whose name begins with a dot is not descended, which is the rule
// the resolver applies to the homes: what is hidden from a listing of the
// checkout is hidden here too, and one of those directories holds a working
// copy of the whole repository per agent.
func documents(checkout string) ([]File, error) {
	files := []File{}
	walk := func(full string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			// A directory this process cannot read is not a reason to answer
			// nothing: the rest of the checkout is still listed.
			if entry != nil && entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		name := entry.Name()
		if entry.IsDir() {
			if full == checkout {
				return nil
			}
			if strings.HasPrefix(name, ".") || refusedSegment(name) {
				return fs.SkipDir
			}
			return nil
		}
		// A symlink is not a regular file, and is left out rather than offered
		// as a document whose target the route may refuse.
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(checkout, full)
		if err != nil {
			return nil
		}
		id := filepath.ToSlash(relative)
		if !admissibleID(id) {
			return nil
		}
		files = append(files, File{Path: id, Title: documentTitle(full, id)})
		return nil
	}
	if err := filepath.WalkDir(checkout, walk); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return files, nil
}

// documentTitle is the head read the reading route makes, and no more: one
// open, the first few kilobytes, and the first heading in them. A file that
// cannot be opened or read is titled by its name rather than dropped.
func documentTitle(absolute, id string) string {
	file, err := os.Open(absolute)
	if err != nil {
		return path.Base(id)
	}
	defer func() { _ = file.Close() }()
	head, err := io.ReadAll(io.LimitReader(file, titleHead))
	if err != nil {
		return path.Base(id)
	}
	return titleOf(id, head)
}

// refusedSegment reports the directory names the reading route never serves
// through, compared the way that route compares them.
func refusedSegment(name string) bool {
	for _, refused := range refusedSegments {
		if strings.EqualFold(name, refused) {
			return true
		}
	}
	return false
}

// list is a list the browser reads as a list: an absent value is an empty
// array rather than a null the pane would have to test for.
func list(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
