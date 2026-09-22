package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// referencedByKeys are the keys that make one record a reference to another.
// Governs names goal ids and By names whoever accepted the record, so neither
// belongs in what references what. Answers is the register's own: the question
// a record answered names it there.
var referencedByKeys = []string{"Cites", "Affects", "Supersedes", "Answers"}

// ListOptions narrows a listing to one goal, one status, or both. An empty
// field narrows nothing.
type ListOptions struct{ Goal, Status string }

// questionRecords are the register's rows in the shape every query answers in:
// a question is a record of the question kind, titled by what it asks, standing
// under the word its status carries, and living in the register rather than in
// a page of its own.
func (p *Project) questionRecords() []Record {
	register := p.registerRel()
	records := make([]Record, 0, len(p.Questions))
	for _, question := range p.Questions {
		records = append(records, Record{
			Kind:     KindQuestion,
			ID:       question.ID,
			Status:   question.State(),
			Goals:    question.Goals,
			Title:    question.Text,
			Path:     register,
			Home:     register,
			Answers:  question.Answers(),
			HeadLine: question.Line,
		})
	}
	return records
}

// queried is everything a query answers about: the pages the homes declare,
// then the register's rows.
func (p *Project) queried() []Record {
	return append(append([]Record(nil), p.Records...), p.questionRecords()...)
}

// List is every record of one kind, in path order, narrowed as asked. The
// questions list in the order the register holds them.
func (p *Project) List(kind string, options ListOptions) []Record {
	var listed []Record
	for _, record := range p.queried() {
		if record.Kind != kind {
			continue
		}
		if options.Status != "" && record.Status != options.Status {
			continue
		}
		if options.Goal != "" && !contains(record.Goals, options.Goal) {
			continue
		}
		listed = append(listed, record)
	}
	return listed
}

// Reference is one record naming another, under the key it named it by.
type Reference struct {
	ID   string
	Path string
	Key  string
}

// ReferencedBy is every record whose Cites, Affects or Supersedes names this
// id, and every question answered by it — the half of a reference the
// referenced record cannot declare itself.
func (p *Project) ReferencedBy(id string) []Reference {
	var references []Reference
	for _, record := range p.queried() {
		if record.ID == id {
			continue
		}
		for _, key := range referencedByKeys {
			if contains(record.References(key), id) {
				references = append(references, Reference{ID: record.ID, Path: record.Path, Key: key})
			}
		}
	}
	sort.SliceStable(references, func(i, j int) bool {
		if references[i].Path != references[j].Path {
			return references[i].Path < references[j].Path
		}
		return references[i].Key < references[j].Key
	})
	return references
}

// Counts is one bucket's tally, by kind and by status. Every kind and every
// status is present, zero included, so two buckets read the same way.
type Counts struct {
	Kind   map[string]int
	Status map[string]int
	Total  int
}

func newCounts() Counts {
	counts := Counts{Kind: map[string]int{}, Status: map[string]int{}}
	for _, kind := range QueryKinds {
		counts.Kind[kind] = 0
	}
	for _, status := range QueryStatuses {
		counts.Status[status] = 0
	}
	return counts
}

func (c *Counts) add(record Record) {
	c.Total++
	if _, known := c.Kind[record.Kind]; known {
		c.Kind[record.Kind]++
	}
	if _, known := c.Status[record.Status]; known {
		c.Status[record.Status]++
	}
}

// GoalCounts is one goal of the ledger with what the project holds about it.
type GoalCounts struct {
	Goal   Goal
	Counts Counts
}

// Tree is the ledger's goals — the live ones first, each in id order — and the
// project-wide bucket last: what belongs to the whole rather than to one goal.
// A record naming two goals is counted under both, because it is about both,
// and a question is counted beside the records, because a question is one of
// the things a goal holds.
//
// A record naming a goal the ledger does not have is counted under no goal and
// is not counted as project-wide either: the check refuses that record by name,
// and counting it somewhere would be inventing a place for it.
func (p *Project) Tree() ([]GoalCounts, Counts) {
	tree := make([]GoalCounts, 0, len(p.Goals))
	at := map[string]int{}
	for _, one := range p.Goals {
		if _, seen := at[one.ID]; seen {
			continue
		}
		at[one.ID] = len(tree)
		tree = append(tree, GoalCounts{Goal: one, Counts: newCounts()})
	}
	whole := newCounts()
	for _, record := range p.queried() {
		if len(record.Goals) == 0 {
			whole.add(record)
			continue
		}
		for _, id := range record.Goals {
			if index, known := at[id]; known {
				tree[index].Counts.add(record)
			}
		}
	}
	return tree, whole
}

// check collects every refusal. It runs once, over what was read, so the same
// answer serves the check verb and the pane.
//
// One index holds every id the project declares, pages and register rows
// alike: an id is unique across the project, so the second declaration of one
// is refused at its own line whichever of the two it is.
func (p *Project) check() {
	declaredAt := map[string]string{}
	for _, record := range p.Records {
		if record.ID != "" {
			if first, duplicate := declaredAt[record.ID]; duplicate {
				p.problem(record.Path, record.line("Id"),
					"the id "+record.ID+" is already declared by "+first)
			} else {
				declaredAt[record.ID] = record.Path
			}
		}
		if record.Kind != "" && !contains(Kinds, record.Kind) {
			p.problem(record.Path, record.line("Kind"),
				"the kind "+record.Kind+" is not one of intent, doctrine, decision, design")
		}
		if record.Status != "" && !contains(Statuses, record.Status) {
			p.problem(record.Path, record.line("Status"),
				"the status "+record.Status+" is not one of draft, accepted, superseded, done")
		}
		for _, id := range record.Goals {
			if !p.HasGoal(id) {
				p.problem(record.Path, record.line(goalsKey), "the goal "+id+" is not in the ledger")
			}
		}
	}
	for _, book := range p.Books {
		p.checkChapters(book)
	}
	p.checkQuestions(declaredAt)
}

// checkChapters reads a book's reading order: every id names a record of the
// book's own kind, and every binding names a file that is there.
func (p *Project) checkChapters(book Book) {
	index := book.Index.Path
	for _, chapter := range book.Chapters {
		if chapter.Doc != "" {
			if !p.holds(chapter.Doc) {
				p.problem(index, chapter.Line, "the chapter binds "+chapter.Doc+", which is not in this checkout")
			}
			continue
		}
		record := p.Record(chapter.ID)
		if record == nil {
			p.problem(index, chapter.Line, "the chapter names "+chapter.ID+", which no record declares")
			continue
		}
		if record.Kind != book.Kind {
			p.problem(index, chapter.Line,
				"the chapter names "+chapter.ID+", a "+record.Kind+" record, in the "+book.Kind+" book")
		}
	}
}

// holds reports whether a bound chapter names a document in this checkout: a
// regular file, reached from the checkout root, by a path that is not absolute
// and steps out of nothing.
//
// The path is judged in both separators' spellings before it is resolved, and
// resolved inside the checkout afterwards, so neither `../outside.md` nor a
// symlink pointing out of the checkout binds a byte the project does not have.
func (p *Project) holds(doc string) bool {
	if doc == "" || filepath.IsAbs(doc) || strings.HasPrefix(doc, "/") || strings.HasPrefix(doc, `\`) {
		return false
	}
	for _, segment := range strings.FieldsFunc(doc, isSeparator) {
		if segment == ".." {
			return false
		}
	}
	root, err := os.OpenRoot(p.Roots.Checkout)
	if err != nil {
		return false
	}
	defer func() { _ = root.Close() }()
	info, err := root.Stat(filepath.FromSlash(doc))
	return err == nil && info.Mode().IsRegular()
}

// isSeparator reads a path in both spellings: a checkout written on Windows
// separates with a backslash, and a parent step is a parent step either way.
func isSeparator(r rune) bool { return r == '/' || r == '\\' }

// checkQuestions reads the register: a row with no id names nothing, a row
// taking an id the project already declares names something else's identity,
// and a row whose status is none of the three says nothing about where the
// question stands.
func (p *Project) checkQuestions(declaredAt map[string]string) {
	register := p.registerRel()
	for _, question := range p.Questions {
		if question.ID == "" {
			p.problem(register, question.Line, "the row declares no id")
		} else if first, duplicate := declaredAt[question.ID]; duplicate {
			p.problem(register, question.Line,
				"the id "+question.ID+" is already declared by "+first)
		} else {
			declaredAt[question.ID] = register
		}
		if !questionStatusValid(question.Status) {
			p.problem(register, question.Line,
				"the status "+quoteEmpty(question.Status)+" is not open, answered: <reference>, or withdrawn")
		}
		for _, id := range question.Goals {
			if !p.HasGoal(id) {
				p.problem(register, question.Line, "the goal "+id+" is not in the ledger")
			}
		}
	}
}

func quoteEmpty(value string) string {
	if value == "" {
		return `""`
	}
	return value
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
