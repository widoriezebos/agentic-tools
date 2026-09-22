package project

import (
	"os"
	"path/filepath"
	"sort"
)

// referencedByKeys are the keys that make one record a reference to another.
// Governs names goal ids and By names whoever accepted the record, so neither
// belongs in what references what.
var referencedByKeys = []string{"Cites", "Affects", "Supersedes"}

// ListOptions narrows a listing to one area, one status, or both. An empty
// field narrows nothing.
type ListOptions struct{ Area, Status string }

// List is every record of one kind, in path order, narrowed as asked.
func (p *Project) List(kind string, options ListOptions) []Record {
	var listed []Record
	for _, record := range p.Records {
		if record.Kind != kind {
			continue
		}
		if options.Status != "" && record.Status != options.Status {
			continue
		}
		if options.Area != "" && !contains(record.Areas, options.Area) {
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
// id — the half of a reference the referenced record cannot declare itself.
func (p *Project) ReferencedBy(id string) []Reference {
	var references []Reference
	for _, record := range p.Records {
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
	for _, kind := range Kinds {
		counts.Kind[kind] = 0
	}
	for _, status := range Statuses {
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

// AreaCounts is one declared area with what it holds.
type AreaCounts struct {
	Area   Area
	Counts Counts
}

// Tree is the declared areas in the order the intent index declares them, and
// the project bucket last: what belongs to the whole rather than to a part. A
// record naming two areas is counted under both, because it is in both.
func (p *Project) Tree() ([]AreaCounts, Counts) {
	tree := make([]AreaCounts, 0, len(p.Areas))
	seen := map[string]bool{}
	for _, area := range p.Areas {
		if area.Slug == ProjectArea || seen[area.Slug] {
			continue
		}
		seen[area.Slug] = true
		tree = append(tree, AreaCounts{Area: area, Counts: newCounts()})
	}
	whole := newCounts()
	for _, record := range p.Records {
		for index := range tree {
			if contains(record.Areas, tree[index].Area.Slug) {
				tree[index].Counts.add(record)
			}
		}
		if contains(record.Areas, ProjectArea) {
			whole.add(record)
		}
	}
	return tree, whole
}

// check collects every refusal. It runs once, over what was read, so the same
// answer serves the check verb and the pane.
func (p *Project) check() {
	declared := p.DeclaredAreas()
	firstByID := map[string]Record{}
	for _, record := range p.Records {
		if record.ID != "" {
			if first, duplicate := firstByID[record.ID]; duplicate {
				p.problem(record.Path, record.line("Id"),
					"the id "+record.ID+" is already declared by "+first.Path)
			} else {
				firstByID[record.ID] = record
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
		for _, area := range record.Areas {
			if !declared[area] {
				p.problem(record.Path, record.line("Areas"),
					"the area "+area+" is declared by no intent index")
			}
		}
	}
	for _, book := range p.Books {
		p.checkChapters(book)
	}
	p.checkQuestions(declared)
}

// checkChapters reads a book's reading order: every id names a record of the
// book's own kind, and every binding names a file that is there.
func (p *Project) checkChapters(book Book) {
	index := book.Index.Path
	for _, chapter := range book.Chapters {
		if chapter.Doc != "" {
			if _, err := os.Stat(filepath.Join(p.Roots.Checkout, filepath.FromSlash(chapter.Doc))); err != nil {
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

// checkQuestions reads the register: a row with no id names nothing, and a row
// whose status is none of the three says nothing about where the question
// stands.
func (p *Project) checkQuestions(declared map[string]bool) {
	register := ""
	for _, home := range p.Homes {
		if home.Register {
			register = home.Rel
		}
	}
	for _, question := range p.Questions {
		if question.ID == "" {
			p.problem(register, question.Line, "the row declares no id")
		}
		if !questionStatusValid(question.Status) {
			p.problem(register, question.Line,
				"the status "+quoteEmpty(question.Status)+" is not open, answered: <reference>, or withdrawn")
		}
		for _, area := range question.Areas {
			if !declared[area] {
				p.problem(register, question.Line, "the area "+area+" is declared by no intent index")
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
