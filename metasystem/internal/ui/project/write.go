package project

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// The one write surface of the Project section.
//
// It is the human editing their own checkout through the browser, on loopback,
// and it records no authority: a created record is a draft, a status is a line
// anyone with the file could have typed, and a question is a row appended to a
// table. Nothing here accepts anything.
//
// Three rules hold for every write:
//
//   - No path comes from the caller. A route takes an id, a title it turns
//     into a slug of its own making, and areas the intent index declares; the
//     directory is the resolver's own home for the kind. Nothing a caller
//     sends can name a file.
//   - Nothing is written that the resolver would then refuse. What is about to
//     be written is put through the resolver's own parser and through the same
//     checks the check verb makes, and a refusal carries those problems and
//     leaves the filesystem untouched.
//   - Every write is followed by a re-read through the resolver, so what the
//     caller is answered with is what the pane will see, not what this package
//     believes it wrote.

// RefusalKind is why a write was refused, in the three shapes a caller can act
// on: the request was wrong, what it named is not there, or what it would
// create already exists. The route turns each into its status.
type RefusalKind string

const (
	RefusalBad    RefusalKind = "bad"
	RefusalAbsent RefusalKind = "absent"
	RefusalExists RefusalKind = "exists"
)

// Refusal is a write this package declined to make. Problems are the check
// verb's own refusals where the request would have produced a record the
// resolver rejects, and empty otherwise.
type Refusal struct {
	Kind     RefusalKind
	Message  string
	Problems []Problem
}

func (r *Refusal) Error() string { return r.Message }

func refuse(kind RefusalKind, message string) *Refusal {
	return &Refusal{Kind: kind, Message: message}
}

// NewRecord is what a human asked for: a kind, a title, the areas it belongs
// to, and the records it rests on or touches. Everything else — the id, the
// status, the file name, the home, the body — is this package's to decide.
type NewRecord struct {
	Kind    string   `json:"kind"`
	Title   string   `json:"title"`
	Areas   []string `json:"areas"`
	Affects []string `json:"affects"`
	Cites   []string `json:"cites"`
}

// NewQuestion is one row of the register, before it has an id or a date.
type NewQuestion struct {
	Question string   `json:"question"`
	Areas    []string `json:"areas"`
}

// Written is what a write produced: the record as the pane shapes it, the
// checkout-relative path every answer names it by, and the file itself, which
// is what an editor opens.
type Written struct {
	Record   Record `json:"record"`
	Path     string `json:"path"`
	Absolute string `json:"absolute"`
}

// Asked is one register row as the pane shapes it.
type Asked struct {
	Question Question `json:"question"`
}

// The kinds this surface creates. Doctrine is not among them: a doctrine
// chapter is how the project is shaped, and shaping it is not a thing to do
// from a form.
var templates = map[string][]string{
	resolver.KindIntent:   {"Users", "Outcomes", "Constraints", "Open questions"},
	resolver.KindDecision: {"Context", "Decision", "Consequences"},
	resolver.KindDesign:   {"Outcome", "Scope", "What changes", "Verification"},
}

// CreatableKinds is the three, in reading order, for a caller that offers them.
var CreatableKinds = []string{resolver.KindIntent, resolver.KindDecision, resolver.KindDesign}

// slugLimit is how long a file name this surface makes. A title longer than
// that is a title; the file name is only how the record is found on disk, and
// the record is found by its id everywhere else.
const slugLimit = 60

// chaptersHeading is the list an intent chapter joins.
const chaptersHeading = "Chapters"

// CreateRecord writes one draft record in its kind's home at the state root and
// answers it as the pane will read it.
//
// The home is the state root's, never the checkout's second design home: the
// second home holds the kit's own designs beside the installation that reads
// them, and a record written from an application's interface belongs to the
// application's state root.
//
// An intent chapter is also listed in the intent index's reading order, because
// a chapter no index names is a chapter nobody reads.
func CreateRecord(roots Roots, asked NewRecord, now time.Time) (Written, error) {
	body, creatable := templates[asked.Kind]
	if !creatable {
		return Written{}, refuse(RefusalBad,
			"the kind "+quoted(asked.Kind)+" is not one of "+strings.Join(CreatableKinds, ", "))
	}
	title := normalizeSpace(asked.Title)
	if title == "" {
		return Written{}, refuse(RefusalBad, "a record needs a title")
	}
	name := slug(title)
	if name == "" {
		return Written{}, refuse(RefusalBad, "the title "+quoted(title)+" yields no file name")
	}

	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Written{}, err
	}
	areas, refusal := declaredAreas(read, asked.Areas)
	if refusal != nil {
		return Written{}, refusal
	}
	if len(areas) == 0 {
		return Written{}, refuse(RefusalBad, "a record names at least one declared area")
	}

	home, found := homeFor(roots, asked.Kind)
	if !found {
		return Written{}, fmt.Errorf("this project has no home for a %s record", asked.Kind)
	}
	absolute := filepath.Join(home.Path, name+".md")
	relative := path.Join(home.Rel, name+".md")
	// The file lies in the home, and the home lies under the state root. The
	// slug is this package's own and carries no separator, so neither can fail
	// today; they are asserted because a home or a slug that changed and left
	// the state root has to fail here rather than write there.
	if err := within(home.Path, absolute); err != nil {
		return Written{}, err
	}
	if err := within(roots.StateRoot, absolute); err != nil {
		return Written{}, err
	}
	if _, err := os.Lstat(absolute); err == nil {
		return Written{}, refuse(RefusalExists, "a file already lives at "+relative)
	} else if !os.IsNotExist(err) {
		return Written{}, fmt.Errorf("cannot look at %s: %w", relative, err)
	}

	// An intent chapter that cannot be listed is not written at all, so the
	// index and the chapter never disagree about what exists.
	index := ""
	if asked.Kind == resolver.KindIntent {
		index = filepath.Join(home.Path, "index.md")
		if _, err := os.Stat(index); err != nil {
			return Written{}, refuse(RefusalBad,
				"this project has no intent index at "+path.Join(home.Rel, "index.md")+", so a chapter cannot be listed")
		}
	}

	id, err := resolver.NewID()
	if err != nil {
		return Written{}, fmt.Errorf("cannot mint an id: %w", err)
	}
	if read.Record(id) != nil {
		return Written{}, refuse(RefusalExists, "the id "+id+" is already declared")
	}

	text := page(title, asked.Kind, id, areas, asked.Cites, asked.Affects, body)
	if problems := wouldRefuse(read, relative, text); len(problems) > 0 {
		return Written{}, &Refusal{
			Kind:     RefusalBad,
			Message:  "the record this would write is one the project refuses",
			Problems: problems,
		}
	}

	if _, err := atomicfile.WriteText(absolute, text, roots.Checkout); err != nil {
		return Written{}, fmt.Errorf("cannot write %s: %w", relative, err)
	}
	if index != "" {
		if err := listChapter(index, roots.Checkout, id, title); err != nil {
			return Written{}, fmt.Errorf("%s was written, but the intent index could not list it: %w", relative, err)
		}
	}
	return writtenRecord(roots, id, relative, absolute)
}

// SetStatus rewrites one record's `- Status:` line and leaves every other byte
// of the file exactly as it was, because a status is one line and a status
// change that reformatted the page would be a status change nobody could read
// in a diff.
func SetStatus(roots Roots, id, status string) (Written, error) {
	if !includes(resolver.Statuses, status) {
		return Written{}, refuse(RefusalBad,
			"the status "+quoted(status)+" is not one of "+strings.Join(resolver.Statuses, ", "))
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Written{}, err
	}
	record := read.Record(id)
	if record == nil {
		return Written{}, refuse(RefusalAbsent, "no record declares the id "+id)
	}
	field, declared := headField(*record, "Status")
	if !declared {
		return Written{}, refuse(RefusalBad, record.Path+" declares no Status line to change")
	}

	absolute := filepath.Join(roots.Checkout, filepath.FromSlash(record.Path))
	if err := within(roots.Checkout, absolute); err != nil {
		return Written{}, err
	}
	raw, err := os.ReadFile(absolute)
	if err != nil {
		return Written{}, fmt.Errorf("cannot read %s: %w", record.Path, err)
	}
	lines := strings.Split(string(raw), "\n")
	at := field.Line - 1
	if at < 0 || at >= len(lines) {
		return Written{}, fmt.Errorf("%s changed while it was being read", record.Path)
	}
	line, carriage := strings.CutSuffix(lines[at], "\r")
	if key, _, isHead := headLine(line); !isHead || !strings.EqualFold(key, "Status") {
		return Written{}, fmt.Errorf("%s changed while it was being read", record.Path)
	}
	lines[at] = "- " + field.Key + ": " + status
	if carriage {
		lines[at] += "\r"
	}
	text := strings.Join(lines, "\n")
	if problems := wouldRefuse(read, record.Path, text); len(problems) > 0 {
		return Written{}, &Refusal{
			Kind:     RefusalBad,
			Message:  "the record this would write is one the project refuses",
			Problems: problems,
		}
	}
	if _, err := atomicfile.WriteText(absolute, text, roots.Checkout); err != nil {
		return Written{}, fmt.Errorf("cannot write %s: %w", record.Path, err)
	}
	return writtenRecord(roots, id, record.Path, absolute)
}

// AskQuestion appends one row to the register, open, dated today. The register
// is created with its header where a project has none yet.
func AskQuestion(roots Roots, asked NewQuestion, now time.Time) (Asked, error) {
	text := normalizeSpace(asked.Question)
	if text == "" {
		return Asked{}, refuse(RefusalBad, "a question needs words")
	}
	if strings.ContainsAny(asked.Question, "|\n\r") {
		// A cell is bounded by pipes and a row by a line, so neither can
		// appear in a question without making the row something else.
		return Asked{}, refuse(RefusalBad, "a question carries no | and no line break")
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Asked{}, err
	}
	areas, refusal := declaredAreas(read, asked.Areas)
	if refusal != nil {
		return Asked{}, refusal
	}
	register, found := registerOf(roots)
	if !found {
		return Asked{}, fmt.Errorf("this project has no questions register")
	}
	id, err := resolver.NewID()
	if err != nil {
		return Asked{}, fmt.Errorf("cannot mint an id: %w", err)
	}
	id = "Q-" + id
	row := "| " + id + " | " + now.Format(time.DateOnly) + " | " + text + " | " +
		strings.Join(areas, " ") + " | " + resolver.QuestionOpen + " |"

	if err := within(roots.StateRoot, register.Path); err != nil {
		return Asked{}, err
	}
	existing, err := os.ReadFile(register.Path)
	if err != nil && !os.IsNotExist(err) {
		return Asked{}, fmt.Errorf("cannot read %s: %w", register.Rel, err)
	}
	if _, err := atomicfile.WriteText(register.Path, appendRow(string(existing), row), roots.Checkout); err != nil {
		return Asked{}, fmt.Errorf("cannot write %s: %w", register.Rel, err)
	}
	return askedQuestion(roots, id)
}

// SetQuestionStatus rewrites one row's status cell and nothing else in the
// register, for the reason a record's status line is rewritten alone.
func SetQuestionStatus(roots Roots, id, status string) (Asked, error) {
	if !questionStatus(status) {
		return Asked{}, refuse(RefusalBad,
			"the status "+quoted(status)+" is not open, answered: <reference>, or withdrawn")
	}
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Asked{}, err
	}
	var question *resolver.Question
	for index := range read.Questions {
		if read.Questions[index].ID == id {
			question = &read.Questions[index]
			break
		}
	}
	if question == nil {
		return Asked{}, refuse(RefusalAbsent, "no row of the register declares the id "+id)
	}
	register, found := registerOf(roots)
	if !found {
		return Asked{}, fmt.Errorf("this project has no questions register")
	}
	if err := within(roots.StateRoot, register.Path); err != nil {
		return Asked{}, err
	}
	raw, err := os.ReadFile(register.Path)
	if err != nil {
		return Asked{}, fmt.Errorf("cannot read %s: %w", register.Rel, err)
	}
	lines := strings.Split(string(raw), "\n")
	at := question.Line - 1
	if at < 0 || at >= len(lines) {
		return Asked{}, fmt.Errorf("%s changed while it was being read", register.Rel)
	}
	line, carriage := strings.CutSuffix(lines[at], "\r")
	rewritten, ok := withStatusCell(line, status)
	if !ok {
		return Asked{}, fmt.Errorf("%s changed while it was being read", register.Rel)
	}
	lines[at] = rewritten
	if carriage {
		lines[at] += "\r"
	}
	if _, err := atomicfile.WriteText(register.Path, strings.Join(lines, "\n"), roots.Checkout); err != nil {
		return Asked{}, fmt.Errorf("cannot write %s: %w", register.Rel, err)
	}
	return askedQuestion(roots, id)
}

/* ------------------------------------------------------------ the page -- */

// page is the record as it will be written: the title, the head the memory
// system's grammar requires, the references the caller named, and the kind's
// own empty sections. The optional keys are written in the resolver's own
// order, so two records made here never disagree about where a key goes.
func page(title, kind, id string, areas, cites, affects, sections []string) string {
	var out strings.Builder
	out.WriteString("# " + title + "\n\n")
	out.WriteString("- Kind: " + kind + "\n")
	out.WriteString("- Id: " + id + "\n")
	out.WriteString("- Status: " + resolver.StatusDraft + "\n")
	out.WriteString("- Areas: " + strings.Join(areas, " ") + "\n")
	if references := cleaned(cites); len(references) > 0 {
		out.WriteString("- Cites: " + strings.Join(references, " ") + "\n")
	}
	if references := cleaned(affects); len(references) > 0 {
		out.WriteString("- Affects: " + strings.Join(references, " ") + "\n")
	}
	for _, section := range sections {
		out.WriteString("\n## " + section + "\n")
	}
	return out.String()
}

// slug is the file name a title yields: lower case, every character that is not
// an unaccented letter or a digit a hyphen, runs of hyphens collapsed, the ends
// trimmed, and at most slugLimit characters. It is deliberately ASCII: the
// file name is a name on a filesystem shared between machines, and the title is
// where a project's own words live.
func slug(title string) string {
	var out strings.Builder
	for _, character := range strings.ToLower(title) {
		switch {
		case character >= 'a' && character <= 'z', character >= '0' && character <= '9':
			out.WriteRune(character)
		default:
			if !strings.HasSuffix(out.String(), "-") {
				out.WriteByte('-')
			}
		}
	}
	name := strings.Trim(out.String(), "-")
	if len(name) > slugLimit {
		name = name[:slugLimit]
	}
	return strings.Trim(name, "-")
}

/* ------------------------------------------------------- the index and the register -- */

// listChapter appends one chapter to the intent index's reading order, in the
// shape the index parser reads: an id, an em dash, and the title. A `##
// Chapters` list that is not there is started at the end of the file.
func listChapter(index, anchor, id, title string) error {
	raw, err := os.ReadFile(index)
	if err != nil {
		return err
	}
	if _, err := atomicfile.WriteText(index, withChapter(string(raw), "- "+id+" — "+title), anchor); err != nil {
		return err
	}
	return nil
}

// withChapter puts one line at the end of the `## Chapters` list: after the
// last item of it, or right under the heading where the list is empty, or in a
// section of its own at the end where there is no such heading.
func withChapter(text, entry string) string {
	lines := strings.Split(text, "\n")
	inside, after := false, -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			if inside {
				break
			}
			if strings.TrimSpace(strings.TrimLeft(trimmed, "#")) == chaptersHeading {
				inside, after = true, index
			}
			continue
		}
		if inside && strings.HasPrefix(trimmed, "- ") {
			after = index
		}
	}
	if after < 0 {
		return strings.TrimRight(text, "\n") + "\n\n## " + chaptersHeading + "\n\n" + entry + "\n"
	}
	return strings.Join(append(append(append([]string{}, lines[:after+1]...), entry), lines[after+1:]...), "\n")
}

// appendRow puts one row at the end of the register's table. A register that
// does not exist, or that carries no table, is given the header the memory
// system's grammar names.
func appendRow(text, row string) string {
	header := "| " + strings.Join(registerColumns, " | ") + " |"
	rule := "| " + strings.Repeat("--- | ", len(registerColumns)-1) + "--- |"
	lines := strings.Split(text, "\n")
	started, after := false, -1
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if started {
				break
			}
			continue
		}
		if !started {
			started = sameColumns(trimmed)
			if started {
				after = index
			}
			continue
		}
		after = index
	}
	if after < 0 {
		if strings.TrimSpace(text) == "" {
			return "# Open questions\n\n" + header + "\n" + rule + "\n" + row + "\n"
		}
		return strings.TrimRight(text, "\n") + "\n\n" + header + "\n" + rule + "\n" + row + "\n"
	}
	return strings.Join(append(append(append([]string{}, lines[:after+1]...), row), lines[after+1:]...), "\n")
}

// registerColumns is the register's one table, by its columns, in the order the
// grammar writes them.
var registerColumns = []string{"id", "opened", "question", "areas", "status"}

func sameColumns(line string) bool {
	cells := strings.Split(strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|"), "|")
	if len(cells) != len(registerColumns) {
		return false
	}
	for index, cell := range cells {
		if !strings.EqualFold(strings.TrimSpace(cell), registerColumns[index]) {
			return false
		}
	}
	return true
}

// withStatusCell replaces the fifth cell of one row and leaves the rest of the
// line, its spacing included, exactly as it was written.
func withStatusCell(line, status string) (string, bool) {
	if !strings.HasPrefix(strings.TrimSpace(line), "|") {
		return "", false
	}
	segments := strings.Split(line, "|")
	// A row is "" | id | opened | question | areas | status | "": seven
	// segments around five cells. The status cell is the fifth of them.
	if len(segments) < len(registerColumns)+2 {
		return "", false
	}
	segments[len(registerColumns)] = " " + status + " "
	return strings.Join(segments, "|"), true
}

// questionStatus admits the three the register may carry, and requires an
// answer to name what answered it. It refuses a status that would break the
// row it is written into.
func questionStatus(status string) bool {
	if strings.ContainsAny(status, "|\n\r") {
		return false
	}
	switch {
	case status == resolver.QuestionOpen, status == resolver.QuestionWithdrawn:
		return true
	case strings.HasPrefix(status, resolver.AnsweredPrefix):
		return strings.TrimSpace(strings.TrimPrefix(status, resolver.AnsweredPrefix)) != ""
	}
	return false
}

/* ------------------------------------------------------------- the checks -- */

// wouldRefuse puts the bytes about to be written through the resolver's own
// parser and the check verb's own rules, and answers what a human would be
// shown if the file were already on disk. A write that carries a problem is not
// made at all.
func wouldRefuse(read *resolver.Project, relative, text string) []Problem {
	record, problems, isRecord := resolver.ParseRecord(relative, text)
	if !isRecord {
		return []Problem{{Path: relative, Line: 1, Message: "this text does not declare itself a record"}}
	}
	refusals := problemsOfResolver(problems)
	declared := read.DeclaredAreas()
	if !includes(resolver.Kinds, record.Kind) {
		refusals = append(refusals, Problem{Path: relative, Line: record.HeadLine,
			Message: "the kind " + record.Kind + " is not one of " + strings.Join(resolver.Kinds, ", ")})
	}
	if !includes(resolver.Statuses, record.Status) {
		refusals = append(refusals, Problem{Path: relative, Line: record.HeadLine,
			Message: "the status " + record.Status + " is not one of " + strings.Join(resolver.Statuses, ", ")})
	}
	for _, area := range record.Areas {
		if !declared[area] {
			refusals = append(refusals, Problem{Path: relative, Line: record.HeadLine,
				Message: "the area " + area + " is declared by no intent index"})
		}
	}
	if first := read.Record(record.ID); first != nil && first.Path != relative {
		refusals = append(refusals, Problem{Path: relative, Line: record.HeadLine,
			Message: "the id " + record.ID + " is already declared by " + first.Path})
	}
	return refusals
}

func problemsOfResolver(problems []resolver.Problem) []Problem {
	refusals := []Problem{}
	for _, problem := range problems {
		refusals = append(refusals, Problem{Path: problem.Path, Line: problem.Line, Message: problem.Message})
	}
	return refusals
}

// declaredAreas is the areas a caller named, each declared by the intent index,
// each once, in the order they were named.
func declaredAreas(read *resolver.Project, asked []string) ([]string, *Refusal) {
	declared := read.DeclaredAreas()
	areas, seen := []string{}, map[string]bool{}
	for _, area := range asked {
		area = strings.TrimSpace(area)
		if area == "" || seen[area] {
			continue
		}
		if !declared[area] {
			return nil, refuse(RefusalBad, "the area "+quoted(area)+" is declared by no intent index")
		}
		seen[area] = true
		areas = append(areas, area)
	}
	return areas, nil
}

/* ----------------------------------------------------------- the re-reads -- */

// writtenRecord reads the project again and answers the record by its id, so
// the caller is told what the pane will show rather than what was composed.
func writtenRecord(roots Roots, id, relative, absolute string) (Written, error) {
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Written{}, err
	}
	record := read.Record(id)
	if record == nil {
		return Written{}, fmt.Errorf("%s was written, but the resolver does not read a record with the id %s back from it", relative, id)
	}
	return Written{Record: describe(*record), Path: record.Path, Absolute: absolute}, nil
}

func askedQuestion(roots Roots, id string) (Asked, error) {
	read, err := resolver.Read(resolver.Roots(roots))
	if err != nil {
		return Asked{}, err
	}
	for _, question := range read.Questions {
		if question.ID != id {
			continue
		}
		return Asked{Question: Question{
			ID:       question.ID,
			Opened:   question.Opened,
			Question: question.Text,
			Areas:    list(question.Areas),
			Status:   question.Status,
		}}, nil
	}
	return Asked{}, fmt.Errorf("the register was written, but the resolver does not read a row with the id %s back from it", id)
}

/* ------------------------------------------------------------- the places -- */

// homeFor is the state root's home for one kind. The resolver lists the state
// root's homes first and the checkout's second design home after them, so the
// first match is the one a record written here belongs in.
func homeFor(roots Roots, kind string) (resolver.Home, bool) {
	for _, home := range resolver.Homes(resolver.Roots(roots)) {
		if !home.Register && home.Kind == kind {
			return home, true
		}
	}
	return resolver.Home{}, false
}

func registerOf(roots Roots) (resolver.Home, bool) {
	for _, home := range resolver.Homes(resolver.Roots(roots)) {
		if home.Register {
			return home, true
		}
	}
	return resolver.Home{}, false
}

// within refuses a target that does not lie beneath a root. Nothing a caller
// sends reaches a path, so this is a second wall rather than the first: it
// holds whatever a home, a slug or a record's own path turns out to be.
func within(root, target string) error {
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) ||
		filepath.IsAbs(relative) {
		return fmt.Errorf("%s does not lie beneath %s", target, root)
	}
	return nil
}

/* -------------------------------------------------------------- the words -- */

// headField is one declared key of a record's head, as it was written, with the
// line it was read from.
func headField(record resolver.Record, key string) (resolver.Field, bool) {
	for _, field := range record.Head {
		if strings.EqualFold(field.Key, key) {
			return field, true
		}
	}
	return resolver.Field{}, false
}

// headLine reads one line the way the resolver's head parser reads it, so the
// line about to be rewritten is checked against the same grammar.
func headLine(line string) (key, value string, isHead bool) {
	rest, cut := strings.CutPrefix(line, "- ")
	if !cut {
		return "", "", false
	}
	name, after, found := strings.Cut(rest, ":")
	if !found || name == "" {
		return "", "", false
	}
	for index, character := range name {
		letter := character >= 'A' && character <= 'Z' || character >= 'a' && character <= 'z'
		digit := character >= '0' && character <= '9'
		if index == 0 && !letter {
			return "", "", false
		}
		if !letter && !digit && character != '_' && character != '-' {
			return "", "", false
		}
	}
	return name, strings.TrimSpace(after), true
}

func cleaned(values []string) []string {
	kept := []string{}
	for _, value := range values {
		kept = append(kept, strings.Fields(value)...)
	}
	return kept
}

func includes(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func normalizeSpace(value string) string { return strings.Join(strings.Fields(value), " ") }

func quoted(value string) string { return strconv.Quote(value) }
