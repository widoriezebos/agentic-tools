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
// is 7: the first was the catalogue of canonical documents, which guessed a
// kind from a filename; the second carried what the records declare; the third
// carried, beside each of them, the record's own first words; the fourth
// carried the ledger's goals where the third carried the intent index's areas,
// which are gone; the fifth carried what a goal's slice plan is read out of —
// the goal's own slicing boundary, and the slices each design lists; the sixth
// carried when each record's file was last written, which is what a reader
// asking "what changed since I last looked" compares against; this one carries
// the name of the home a record was read from, because the kit's own history
// has a name its path does not give.
const SchemaVersion = 7

// Pane is the whole of Project, read once, as it was at readAt.
type Pane struct {
	SchemaVersion int        `json:"schemaVersion"`
	ReadAt        string     `json:"readAt"`
	Goals         []Goal     `json:"goals"`
	Records       []Record   `json:"records"`
	Intent        Book       `json:"intent"`
	Doctrine      Book       `json:"doctrine"`
	Questions     []Question `json:"questions"`
	Problems      []Problem  `json:"problems"`
	Documents     []File     `json:"documents"`
}

// Goal is one goal of the ledger: the project's one subdivision, named by its
// ledger id, with where it stands and why it is open.
type Goal struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	State  string `json:"state"`
	Intent string `json:"intent"`
	// Sliced is when slicing started on this goal and which seat started it,
	// where the record carries the boundary at all. It is absent rather than
	// zeroed, because "slicing has not started" and "slicing started at the
	// zero instant" are not the same statement.
	Sliced *Sliced `json:"sliced,omitempty"`
}

// Sliced is the goal's own `- Sliced:` line, as a reader of the plan needs it.
type Sliced struct {
	At      string `json:"at"`
	Machine string `json:"machine"`
	Lineage string `json:"lineage"`
}

// Record is one declared document: what it says it is, where it lives, and
// the first words of its own body. The summary is a convention rather than a
// key — a record whose body opens with a table has none — so it is carried as
// the empty string rather than as an absence the pane must explain.
type Record struct {
	Kind   string   `json:"kind"`
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Goals  []string `json:"goals"`
	Title  string   `json:"title"`
	Path   string   `json:"path"`
	Home   string   `json:"home"`
	// HomeName is what the home is called where its path does not say it. The
	// kit's own history is "designs (historical)"; every other home is named
	// by its path, and carries nothing here rather than a second spelling of
	// the path the row already shows.
	HomeName string `json:"homeName,omitempty"`
	Summary  string `json:"summary"`
	// ChangedAt is when this record's file was last written, from the
	// filesystem, in RFC3339. It is the file's own modification time and
	// nothing the record declares: a record carries no revision date, and a
	// reader asking what changed since their last visit has nothing else to
	// compare. A file this process cannot stat carries the empty string
	// rather than an instant nobody observed.
	ChangedAt string `json:"changedAt"`
	// Slices are the list items this record writes under a Slices heading, as
	// written, and nothing is taken from them. The repository has slice
	// admission and a first-slicing marker but no editable slice-plan owner,
	// so a design's own list is the whole of what is recorded; carrying it is
	// a read, never a claim that this is a plan the engine knows about.
	Slices []string `json:"slices"`
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
	Goals    []string `json:"goals"`
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
		Goals:         goalsOf(read),
		Records:       recordsOf(roots, read),
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

// goalsOf is the ledger's goals in the order the tree places them: the live
// ones first, each in id order, then the concluded ones. The counts the tree
// carries are the verb's; the pane counts what it shows, from the records it
// was given.
func goalsOf(read *resolver.Project) []Goal {
	tree, _ := read.Tree()
	goals := make([]Goal, 0, len(tree))
	for _, one := range tree {
		carried := Goal{
			ID:     one.Goal.ID,
			Title:  one.Goal.Title,
			State:  one.Goal.State,
			Intent: one.Goal.Intent,
		}
		if sliced := one.Goal.Sliced; sliced != nil {
			carried.Sliced = &Sliced{At: sliced.At, Machine: sliced.Machine, Lineage: sliced.Lineage}
		}
		goals = append(goals, carried)
	}
	return goals
}

// recordsOf is every record the resolver listed, in its order, whether or not
// it carries a refusal.
func recordsOf(roots Roots, read *resolver.Project) []Record {
	records := make([]Record, 0, len(read.Records))
	for _, record := range read.Records {
		records = append(records, describe(roots, record))
	}
	return records
}

func describe(roots Roots, record resolver.Record) Record {
	return Record{
		Kind:      record.Kind,
		ID:        record.ID,
		Status:    record.Status,
		Goals:     list(record.Goals),
		Title:     record.Title,
		Path:      record.Path,
		Home:      record.Home,
		HomeName:  record.HomeName,
		Summary:   summaryOf(record.Body),
		Slices:    slicesIn(record.Body),
		ChangedAt: changedAt(roots, record.Path),
	}
}

// changedAt is when a record's file was last written, as the filesystem says.
//
// It is one stat per record, on a read that already opened and parsed every
// one of them, so it costs nothing measurable. A file that cannot be stat'd —
// removed between the resolver's read and this one, or on a filesystem that
// refuses — carries no instant rather than a zero one: "this record changed at
// the beginning of the epoch" is a sentence no reader should be shown.
func changedAt(roots Roots, relative string) string {
	if relative == "" {
		return ""
	}
	info, err := os.Stat(filepath.Join(roots.Checkout, filepath.FromSlash(relative)))
	if err != nil {
		return ""
	}
	return stamp(info.ModTime().UTC())
}

// sliceHeading is the heading a design writes its slice plan under.
const sliceHeading = "slices"

// slicesIn reads the list items under a Slices heading of a record's body.
//
// It is a read and nothing more: each item's text, as written, with no meaning
// taken from it. The repository has slice admission and a first-slicing marker
// and no editable slice-plan owner, so what a design wrote is the whole of
// what is recorded, and showing it as the design wrote it is the only honest
// reading available until that owner exists.
//
// The heading matches at any level, because a design that nests its plan under
// a section is still recording one, and the section ends at the next heading of
// any level — so a plan whose items live under sub-headings is read as far as
// the first of them and no further, which is the same boundary the resolver's
// own chapter reader draws. Indentation is stripped before an item is
// recognized, so a nested item is an item.
func slicesIn(body []string) []string {
	slices := []string{}
	inside := false
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			inside = strings.EqualFold(strings.TrimSpace(strings.TrimLeft(trimmed, "#")), sliceHeading)
			continue
		}
		if !inside {
			continue
		}
		if item, is := listItem(trimmed); is {
			slices = append(slices, item)
		}
	}
	return slices
}

// listItem reports the text of a Markdown list item, bulleted or numbered.
func listItem(line string) (string, bool) {
	for _, marker := range []string{"- ", "* ", "+ "} {
		if rest, found := strings.CutPrefix(line, marker); found {
			return strings.TrimSpace(rest), true
		}
	}
	digits := 0
	for digits < len(line) && line[digits] >= '0' && line[digits] <= '9' {
		digits++
	}
	if digits == 0 || digits+1 >= len(line) {
		return "", false
	}
	if (line[digits] == '.' || line[digits] == ')') && line[digits+1] == ' ' {
		return strings.TrimSpace(line[digits+2:]), true
	}
	return "", false
}

// bookOf is one book's index and its reading order. A chapter the index left
// unnamed is titled by what it binds, so no row in the pane is blank.
func bookOf(roots Roots, read *resolver.Project, kind string) Book {
	book := Book{Chapters: []Chapter{}}
	for _, found := range read.Books {
		if found.Kind != kind || found.Index == nil {
			continue
		}
		index := describe(roots, *found.Index)
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
			Goals:    list(question.Goals),
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
