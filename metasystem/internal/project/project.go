// Package project reads the project's declared memory: its intent, its
// doctrine, its decisions, its designs, and its open questions, each in the one
// home the memory-system design gives it, each declaring what it is in a short
// head at the top of the file.
//
// Nothing here writes, and nothing here infers a kind from a filename: a
// document that carries no head is not a record and is not listed. The homes
// are resolved from the state root, so the same reader serves a self-hosted
// checkout and an adopted application; in the self-hosted layout the checkout's
// own plans/designs is a second home, because the kit's designs live beside the
// installation that reads them rather than beneath it.
//
// Read answers once and carries both: what parsed, and every refusal the check
// verb prints. A project with problems still lists, shows and counts.
package project

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots names the three directories this package reads from. It is declared
// here, rather than imported from the interface's lifecycle slice, for the
// reason that slice's own comment gives: taking its types closes an import
// loop.
type Roots struct{ Checkout, Installation, StateRoot string }

// The four kinds a record declares itself to be.
const (
	KindIntent   = "intent"
	KindDoctrine = "doctrine"
	KindDecision = "decision"
	KindDesign   = "design"
)

// The four statuses a record carries. Status is maintained by hand, as every
// decision log does it; done is a design whose work shipped.
const (
	StatusDraft      = "draft"
	StatusAccepted   = "accepted"
	StatusSuperseded = "superseded"
	StatusDone       = "done"
)

// ProjectArea is the one area no index declares: everything that belongs to the
// project as a whole rather than to a part of it.
const ProjectArea = "project"

// Kinds is reading order rather than alphabetical order: intent, then the
// doctrine that serves it, then the decisions, then the designs.
var Kinds = []string{KindIntent, KindDoctrine, KindDecision, KindDesign}

// Statuses is the life of a record, in the order it is lived.
var Statuses = []string{StatusDraft, StatusAccepted, StatusSuperseded, StatusDone}

// Home is one place the resolver looks: a directory holding records of one
// kind, or the single file the questions register lives in.
type Home struct {
	Kind     string // the kind the home holds; empty for the register
	Path     string // absolute
	Rel      string // checkout-relative, the way every answer names it
	Book     bool   // a directory whose index.md is itself a record of the kind
	Register bool   // the one questions table
}

// Area is one slug the intent index declares, with the name it reads by.
type Area struct {
	Slug string
	Name string
	Line int // in the intent index
}

// Chapter is one line of a book's reading order: a record of the book's kind
// named by id, or an existing document named by its checkout-relative path and
// left unchanged where it is.
type Chapter struct {
	ID    string // empty for a bound document
	Doc   string // checkout-relative path; empty for a record
	Title string
	Line  int
}

// Book is a home whose index.md is a record of the kind, carrying the areas
// (the intent index only) and the reading order.
type Book struct {
	Kind     string
	Home     string // checkout-relative
	Index    *Record
	Areas    []Area
	Chapters []Chapter
}

// Question is one row of the register: opened by hand, answered by reference,
// or withdrawn.
type Question struct {
	ID     string
	Opened string
	Text   string
	Areas  []string
	Status string
	Line   int
}

// Problem is one refusal, anchored where a human can act on it.
type Problem struct {
	Path    string // checkout-relative
	Line    int
	Message string
}

// String is the one line check prints.
func (p Problem) String() string {
	return p.Path + ":" + itoa(p.Line) + ": " + p.Message
}

// Project is everything the homes declare, read at one instant.
type Project struct {
	Roots     Roots
	Homes     []Home
	Records   []Record
	Books     []Book
	Questions []Question
	Areas     []Area
	Problems  []Problem
}

// ResolveRoots derives the three roots from a path at or below a metasystem
// installation: the installation itself, the Git checkout that carries it, and
// the state root the engine resolves — the installation in the self-hosted
// layout, the application's repository for an adopted one. It is the one place
// the homes' base is derived, so no verb can disagree with another about where
// the project's memory lives.
func ResolveRoots(installation string) (Roots, error) {
	layout, err := stateroot.ResolveLayout(installation)
	if err != nil {
		return Roots{}, err
	}
	stateRoot, err := stateroot.RootForInstallation(layout.InstallationRoot)
	if err != nil {
		return Roots{}, err
	}
	return Roots{
		Checkout:     canonical(layout.GitRoot),
		Installation: canonical(layout.InstallationRoot),
		StateRoot:    canonical(stateRoot),
	}, nil
}

// Homes are the homes in reading order. The second design home is the
// self-hosted layout's alone: there the state root is the installation, and the
// kit's own designs sit at the checkout root beside it.
func Homes(roots Roots) []Home {
	homes := []Home{
		{Kind: KindIntent, Path: filepath.Join(roots.StateRoot, "docs", "intent"), Book: true},
		{Kind: KindDoctrine, Path: filepath.Join(roots.StateRoot, "docs", "doctrine"), Book: true},
		{Kind: KindDecision, Path: filepath.Join(roots.StateRoot, "docs", "decisions")},
		{Kind: KindDesign, Path: filepath.Join(roots.StateRoot, "plans", "designs")},
	}
	if SelfHosted(roots) {
		homes = append(homes, Home{Kind: KindDesign, Path: filepath.Join(roots.Checkout, "plans", "designs")})
	}
	homes = append(homes, Home{Path: filepath.Join(roots.StateRoot, "memory", "questions.md"), Register: true})
	for index := range homes {
		homes[index].Rel = relative(roots.Checkout, homes[index].Path)
	}
	return homes
}

// SelfHosted is the template layout and nothing else: there, and only there,
// the state root stateroot resolves is the installation rather than the Git
// checkout that carries it.
func SelfHosted(roots Roots) bool { return roots.StateRoot != roots.Checkout }

// NewID mints a fresh identity for a record, in the engine's one ULID shape.
// An id is any non-empty string the project keeps unique; this is for those who
// would rather not invent one.
func NewID() (string, error) { return goal.NewOperationULID() }

// Read reads every home once. An unreadable file is a problem anchored at that
// file rather than an error, so one bad page never hides the rest of the
// memory; only a walk that cannot proceed at all is returned as an error.
func Read(roots Roots) (*Project, error) {
	read := &Project{Roots: roots, Homes: Homes(roots)}
	for _, home := range read.Homes {
		if home.Register {
			if err := read.readRegister(home); err != nil {
				return nil, err
			}
			continue
		}
		if err := read.readRecords(home); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(read.Records, func(i, j int) bool { return read.Records[i].Path < read.Records[j].Path })
	read.readBooks()
	read.check()
	sort.SliceStable(read.Problems, func(i, j int) bool {
		left, right := read.Problems[i], read.Problems[j]
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Line != right.Line {
			return left.Line < right.Line
		}
		return left.Message < right.Message
	})
	return read, nil
}

// readRecords walks one home. Subdirectories are read, which is what "any file
// name" and "subdirectories allowed" mean, and is also how a book's chapters may
// sit anywhere beneath it. A directory that is not there is not a problem: a
// project declares the homes it uses.
func (p *Project) readRecords(home Home) error {
	walk := func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		name := entry.Name()
		if strings.HasPrefix(name, ".") && path != home.Path {
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".md") {
			return nil
		}
		rel := relative(p.Roots.Checkout, path)
		data, err := os.ReadFile(path)
		if err != nil {
			p.problem(rel, 1, "cannot be read: "+err.Error())
			return nil
		}
		record, problems, isRecord := parseRecord(rel, string(data))
		if !isRecord {
			return nil
		}
		record.Home = home.Rel
		p.Records = append(p.Records, record)
		p.Problems = append(p.Problems, problems...)
		return nil
	}
	if err := filepath.WalkDir(home.Path, walk); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readBooks reads the index of each book home: the areas it declares (the
// intent index alone) and the reading order it names. A home whose index.md is
// absent or carries no head is no book at all, and declares no areas.
func (p *Project) readBooks() {
	for _, home := range p.Homes {
		if !home.Book {
			continue
		}
		index := p.recordAtPath(path.Join(home.Rel, "index.md"))
		if index == nil {
			continue
		}
		book := Book{Kind: home.Kind, Home: home.Rel, Index: index}
		if home.Kind == KindIntent {
			book.Areas = parseAreas(index.Body, index.BodyLine)
			p.Areas = append(p.Areas, book.Areas...)
		}
		book.Chapters = parseChapters(index.Body, index.BodyLine)
		p.Books = append(p.Books, book)
	}
}

// readRegister reads the one questions table. An absent register is an absent
// register, not a refusal.
func (p *Project) readRegister(home Home) error {
	data, err := os.ReadFile(home.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		p.problem(home.Rel, 1, "cannot be read: "+err.Error())
		return nil
	}
	p.Questions = parseQuestions(string(data))
	return nil
}

func (p *Project) recordAtPath(rel string) *Record {
	for index := range p.Records {
		if p.Records[index].Path == rel {
			return &p.Records[index]
		}
	}
	return nil
}

// Record returns the record with this id, or nil.
func (p *Project) Record(id string) *Record {
	for index := range p.Records {
		if p.Records[index].ID == id {
			return &p.Records[index]
		}
	}
	return nil
}

// DeclaredAreas are the areas a record may name: the intent index's own list,
// plus the project as a whole.
func (p *Project) DeclaredAreas() map[string]bool {
	declared := map[string]bool{ProjectArea: true}
	for _, area := range p.Areas {
		declared[area.Slug] = true
	}
	return declared
}

func (p *Project) problem(path string, line int, message string) {
	p.Problems = append(p.Problems, Problem{Path: path, Line: line, Message: message})
}

// relative names a path the way every answer names it: relative to the
// checkout, with forward slashes, so one spelling appears in a message, in a
// listing, and in a chapter binding alike. A path the checkout does not contain
// keeps its absolute form rather than a run of parent steps.
func relative(checkout, target string) string {
	rel, err := filepath.Rel(checkout, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(target)
	}
	return filepath.ToSlash(rel)
}

func canonical(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	if resolved, resolveErr := filepath.EvalSymlinks(absolute); resolveErr == nil {
		return resolved
	}
	return filepath.Clean(absolute)
}
