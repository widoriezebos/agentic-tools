// Package project is a read-only reader of the project's declared memory: its
// intent, its doctrine, its decisions, its designs, and its open questions,
// each in the one home it has, each declaring what it is in a short head at
// the top of the file, and the questions in one register of rows.
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
//
// Nothing here is read from outside the homes: an entry opens inside the home
// that holds it and a bound document inside the checkout, so no symlink and no
// parent step reaches a byte the project does not contain.
package project

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// Roots names the three directories this package reads from. They are declared
// here rather than taken from the interface's own package: this package is
// read by that one, and importing its types back would close an import loop.
type Roots struct{ Checkout, Installation, StateRoot string }

// The four kinds a head declares itself to be, and the fifth no head declares:
// a question is a row of the register rather than a document with a head.
const (
	KindIntent   = "intent"
	KindDoctrine = "doctrine"
	KindDecision = "decision"
	KindDesign   = "design"
	KindQuestion = "question"
)

// The four statuses a record carries. Status is maintained by hand, as every
// decision log does it; done is a design whose work shipped.
const (
	StatusDraft      = "draft"
	StatusAccepted   = "accepted"
	StatusSuperseded = "superseded"
	StatusDone       = "done"
)

// WholeProject names the one bucket no goal declares: everything that belongs
// to the project as a whole rather than to one goal of it.
const WholeProject = "project"

// Kinds is what a head may declare, in reading order rather than alphabetical
// order: intent, then the doctrine that serves it, then the decisions, then
// the designs.
var Kinds = []string{KindIntent, KindDoctrine, KindDecision, KindDesign}

// QueryKinds is every kind a query answers about: the four a head declares,
// then the questions, which are rows and not pages.
var QueryKinds = append(append([]string(nil), Kinds...), KindQuestion)

// Statuses is the life of a record, in the order it is lived.
var Statuses = []string{StatusDraft, StatusAccepted, StatusSuperseded, StatusDone}

// QuestionStatuses is the life of a question, in the order it is lived.
var QuestionStatuses = []string{QuestionOpen, QuestionAnswered, QuestionWithdrawn}

// QueryStatuses is every status a query answers about and every tally has a
// row for: a record's four, then a question's three.
var QueryStatuses = append(append([]string(nil), Statuses...), QuestionStatuses...)

// Home is one place the resolver looks: a directory holding records of one
// kind, or the single file the questions register lives in.
type Home struct {
	Kind string // the kind the home holds; empty for the register
	Path string // absolute
	Rel  string // checkout-relative, the way every answer names it
	// Name is what a reader calls this home where its path does not say it.
	// Empty means the path is the name, which is true of every home but the
	// kit's own history.
	Name string
	// Glob makes a home flat and historical, and only the kit's own history is
	// one. Two rules arrive with it, together because the one home that has it
	// needs both: only the names this pattern matches, directly beneath the
	// home and never in a subdirectory, are read at all; and a file there that
	// declares no Kind is a document the typing pass has not reached, not a
	// record with a faulty head — the legacy bullets some of them open with
	// are prose left where it was written, not a declaration.
	Glob     string
	Book     bool // a directory whose index.md is itself a record of the kind
	Register bool // the one questions table
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

// Book is a home whose index.md is a record of the kind, carrying the reading
// order it names.
type Book struct {
	Kind     string
	Home     string // checkout-relative
	Index    *Record
	Chapters []Chapter
}

// Question is one row of the register: opened by hand, answered by reference,
// or withdrawn.
type Question struct {
	ID     string
	Opened string
	Text   string
	Goals  []string
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
	Goals     []Goal
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

// HistoricalDesignsName is what the pane calls the kit's own history, because
// "metasystem/plans" names a directory of briefs and ledger files and says
// nothing about the one convention read out of it.
const HistoricalDesignsName = "designs (historical)"

// HistoricalDesignsGlob is the whole of the convention the kit's designs were
// written under before a design was a record: the goal's id, then -design.md,
// flat beneath the installation's plans directory. It is deliberately narrow —
// that directory also holds the briefs, the goal ledger and plans/designs — so
// a home read recursively would read the ledger and read plans/designs twice.
const HistoricalDesignsGlob = "*-design.md"

// Homes are the homes in reading order. The second design home and the
// historical one belong to the self-hosted layout alone: there the state root
// is the installation, the kit's own designs sit at the checkout root beside
// it, and the kit's own history sits flat beneath the installation's plans. An
// adopted application has neither and never acquires the kit's past.
func Homes(roots Roots) []Home {
	homes := []Home{
		{Kind: KindIntent, Path: filepath.Join(roots.StateRoot, "docs", "intent"), Book: true},
		{Kind: KindDoctrine, Path: filepath.Join(roots.StateRoot, "docs", "doctrine"), Book: true},
		{Kind: KindDecision, Path: filepath.Join(roots.StateRoot, "docs", "decisions")},
		{Kind: KindDesign, Path: filepath.Join(roots.StateRoot, "plans", "designs")},
	}
	if SelfHosted(roots) {
		homes = append(homes,
			Home{Kind: KindDesign, Path: filepath.Join(roots.Checkout, "plans", "designs")},
			Home{
				Kind: KindDesign,
				Path: filepath.Join(roots.Installation, "plans"),
				Name: HistoricalDesignsName,
				Glob: HistoricalDesignsGlob,
			})
	}
	homes = append(homes, Home{Path: filepath.Join(roots.StateRoot, "memory", "questions.md"), Register: true})
	for index := range homes {
		homes[index].Rel = relative(roots.Checkout, homes[index].Path)
	}
	return homes
}

// HomeFor is the home that would read a record at this checkout-relative
// path, and whether there is one at all.
//
// It answers the question a writer asks before it judges a file by the head
// grammar: a document the homes do not read is never a record, so nothing may
// refuse it over a head it does not have. The register is not a home here —
// it is a file of rows — and a home that is the checkout root itself would
// admit everything, so only a proper prefix counts. A historical home admits
// only what its glob names directly beneath it, which is how the briefs and
// the goal ledger sharing that directory stay out.
func HomeFor(roots Roots, rel string) (Home, bool) {
	if !strings.HasSuffix(rel, ".md") {
		return Home{}, false
	}
	for _, home := range Homes(roots) {
		if home.Register || home.Rel == "" || home.Rel == "." {
			continue
		}
		name, inside := strings.CutPrefix(rel, home.Rel+"/")
		if !inside || name == "" {
			continue
		}
		if home.Glob == "" {
			return home, true
		}
		if strings.Contains(name, "/") {
			continue
		}
		if matched, err := path.Match(home.Glob, name); err == nil && matched {
			return home, true
		}
	}
	return Home{}, false
}

// SelfHosted is the template layout and nothing else: there, and only there,
// the state root stateroot resolves is the installation rather than the Git
// checkout that carries it.
func SelfHosted(roots Roots) bool { return roots.StateRoot != roots.Checkout }

// ulidAlphabet is Crockford's base32: ten digits and twenty-two letters, with
// I, L, O and U left out so that no two characters can be misread for each
// other.
const ulidAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// NewID mints a fresh identity for a record: a ULID, which is a 48-bit
// millisecond timestamp followed by 80 bits of randomness, written as 26
// Crockford base32 characters. Two ids minted in one millisecond differ in
// their randomness, and two minted a millisecond apart sort in the order they
// were minted.
//
// An id is any non-empty string the project keeps unique; this is for those who
// would rather not invent one.
func NewID() (string, error) { return newID(time.Now(), rand.Reader) }

// newID is NewID with the instant and the randomness a test can supply.
func newID(now time.Time, entropy io.Reader) (string, error) {
	var raw [16]byte
	binary.BigEndian.PutUint64(raw[:8], uint64(now.UnixMilli())<<16)
	if _, err := io.ReadFull(entropy, raw[6:]); err != nil {
		return "", fmt.Errorf("mint a project id: %w", err)
	}
	return encodeULID(raw), nil
}

// encodeULID reads the sixteen bytes as one 128-bit number and writes it five
// bits at a time, least significant last. Twenty-six characters carry 130
// bits, so the first one carries only the top three: a ULID never begins above
// 7, and a decoder that read an eighth value there would see an overflow.
func encodeULID(raw [16]byte) string {
	high, low := binary.BigEndian.Uint64(raw[:8]), binary.BigEndian.Uint64(raw[8:])
	id := make([]byte, 26)
	for index := len(id) - 1; index >= 0; index-- {
		id[index] = ulidAlphabet[low&0x1f]
		low = low>>5 | high<<59
		high >>= 5
	}
	return string(id)
}

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
	read.readGoals()
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
// sit anywhere beneath it; a name that begins with a dot is a name like any
// other. A directory that is not there is not a problem: a project declares the
// homes it uses.
//
// The walk is bounded by the home: entries are listed and opened through it, so
// a symlink whose target lies outside does not open and a directory the walk
// would otherwise descend is not followed out.
func (p *Project) readRecords(home Home) error {
	root, err := os.OpenRoot(home.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() { _ = root.Close() }()
	walk := func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			return nil
		}
		p.readRecord(root, home, name)
		return nil
	}
	if home.Glob != "" {
		return p.readFlat(root, home)
	}
	if err := fs.WalkDir(root.FS(), ".", walk); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// readFlat walks a historical home: the names its glob matches, directly
// beneath it, in name order, and nothing in a subdirectory. A subdirectory is
// not descended at all, which is why the goal ledger and the second design
// home, both beneath this one's path, are not read again here.
func (p *Project) readFlat(root *os.Root, home Home) error {
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		matched, matchErr := path.Match(home.Glob, entry.Name())
		if matchErr != nil || !matched {
			continue
		}
		p.readRecord(root, home, entry.Name())
	}
	return nil
}

// readRecord reads one file of a home and keeps what it declares. A file that
// declares nothing is not a record and is not refused; in a historical home a
// file that declares no Kind is not a record either, because the legacy
// bullets it opens with are the prose its author wrote and not a head the
// typing pass has yet given it.
func (p *Project) readRecord(root *os.Root, home Home, name string) {
	rel := relative(p.Roots.Checkout, filepath.Join(home.Path, filepath.FromSlash(name)))
	data, read := p.readWithin(root, name, rel)
	if !read {
		return
	}
	record, problems, isRecord := parseRecord(rel, string(data))
	if !isRecord {
		return
	}
	if home.Glob != "" && !record.declares("Kind") {
		return
	}
	record.Home = home.Rel
	record.HomeName = home.Name
	p.Records = append(p.Records, record)
	p.Problems = append(p.Problems, problems...)
}

// readWithin reads one entry of a home, and never a byte from outside it. A
// name that does not open inside the home — a symlink whose target left it, a
// parent step — is refused where it was found, and so is anything that is not
// a regular file. An absent file is absent rather than refused.
func (p *Project) readWithin(root *os.Root, name, rel string) ([]byte, bool) {
	file, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		p.problem(rel, 1, "cannot be read: "+err.Error())
		return nil, false
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		p.problem(rel, 1, "cannot be read: "+err.Error())
		return nil, false
	}
	if !info.Mode().IsRegular() {
		p.problem(rel, 1, "cannot be read: it is not a regular file")
		return nil, false
	}
	data, err := io.ReadAll(file)
	if err != nil {
		p.problem(rel, 1, "cannot be read: "+err.Error())
		return nil, false
	}
	return data, true
}

// readBooks reads the index of each book home for the reading order it names.
// The index of a book is a record of the book's own kind, so a home whose
// index.md is absent, carries no head, or declares another kind is no book at
// all — the file stays a readable record of whatever kind it does declare.
func (p *Project) readBooks() {
	for _, home := range p.Homes {
		if !home.Book {
			continue
		}
		index := p.recordAtPath(path.Join(home.Rel, "index.md"))
		if index == nil || index.Kind != home.Kind {
			continue
		}
		book := Book{Kind: home.Kind, Home: home.Rel, Index: index}
		book.Chapters = parseChapters(index.Body, index.BodyLine)
		p.Books = append(p.Books, book)
	}
}

// readRegister reads the one questions table, from inside the directory that
// holds it and nowhere else. An absent register is an absent register, not a
// refusal; a register that is not a regular file of its own home is refused
// rather than followed.
func (p *Project) readRegister(home Home) error {
	root, err := os.OpenRoot(filepath.Dir(home.Path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		p.problem(home.Rel, 1, "cannot be read: "+err.Error())
		return nil
	}
	defer func() { _ = root.Close() }()
	data, read := p.readWithin(root, filepath.Base(home.Path), home.Rel)
	if !read {
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

// Record returns the record with this id, or nil. A question is a record too:
// the id it is asked under names its row the way a head's id names a page.
func (p *Project) Record(id string) *Record {
	for index := range p.Records {
		if p.Records[index].ID == id {
			return &p.Records[index]
		}
	}
	if id == "" {
		return nil
	}
	questions := p.questionRecords()
	for index := range questions {
		if questions[index].ID == id {
			return &questions[index]
		}
	}
	return nil
}

// registerRel is where the questions live, the way every answer names it.
func (p *Project) registerRel() string {
	for _, home := range p.Homes {
		if home.Register {
			return home.Rel
		}
	}
	return ""
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
