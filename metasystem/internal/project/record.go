package project

import (
	"regexp"
	"strings"
)

// headLinePattern is the whole of a head line: a bare list item whose key is a
// word and whose value is the rest. It is also the test that tells a record
// from a document, so it is deliberately narrow — a prose bullet does not begin
// with a word and a colon.
var headLinePattern = regexp.MustCompile(`^- ([A-Za-z][A-Za-z0-9_-]*):[ \t]*(.*)$`)

// requiredKeys are the three a head must declare. A key present with an empty
// value has declared nothing, so it is missing.
var requiredKeys = []string{"Kind", "Id", "Status"}

// goalsKey is the one optional key this package validates: it names goals of
// the ledger, which either has them or does not, so a name it does not carry
// is a refusal rather than a reference to something unwritten. A head with no
// Goals line is about the project as a whole.
const goalsKey = "Goals"

// GoalsRefused is what a head is told when it names goals on a kind that
// cannot be about one. The intent and the doctrine are the project's own —
// what it is for and how it is shaped — so a chapter of either is about the
// whole by definition, and a Goals line there would narrow a book that has no
// narrower scope to be in.
const GoalsRefused = "an intent or doctrine record names goals"

// BookKind is the two kinds that are books of the project itself. They are the
// kinds the homes mark Book, said here as a question about a kind alone, which
// is what the head grammar asks and what a writer asks before it composes a
// head it would then be refused for.
func BookKind(kind string) bool { return kind == KindIntent || kind == KindDoctrine }

// referenceKeys are the optional keys a head may declare, parsed as plain
// whitespace-separated references and intentionally unvalidated: a reference
// may name something nobody has written yet, and a reader that refused it
// would be refusing the project's own order of work.
//
// Answers is not among them. It is the register's reference, taken from a
// question row's status, and a head that writes it is writing a key this
// package keeps and reads nothing from.
var referenceKeys = []string{"Cites", "Affects", "Governs", "By", "Supersedes"}

// Field is one head line as it was written. Unknown keys are kept here, in the
// order they appear, and read by nothing.
type Field struct {
	Key   string
	Value string
	Line  int
}

// Record is one declared document: what it says it is, and where it was found.
type Record struct {
	Kind   string
	ID     string
	Status string
	Goals  []string // the ledger goals this record is about; none is the whole
	Title  string
	Path   string // checkout-relative
	Home   string // checkout-relative home the record was found in

	Cites      []string
	Affects    []string
	Governs    []string
	By         []string
	Supersedes []string
	Answers    []string // a question's own, from the register rather than a head

	Head     []Field  // every head line, unknown keys included
	HeadLine int      // where the head begins, and where a missing key is anchored
	Body     []string // the free prose after the head
	BodyLine int      // the file line of Body[0]

	lines map[string]int // the line each declared key was read from
}

// References are the optional keys a record carries, in a fixed order, for show
// to print and for the referenced-by walk to read.
func (r Record) References(key string) []string {
	switch key {
	case "Cites":
		return r.Cites
	case "Affects":
		return r.Affects
	case "Governs":
		return r.Governs
	case "By":
		return r.By
	case "Supersedes":
		return r.Supersedes
	case "Answers":
		return r.Answers
	}
	return nil
}

// line is where a key was declared, or the head's first line when it was not.
func (r Record) line(key string) int {
	if at, ok := r.lines[foldKey(key)]; ok {
		return at
	}
	return r.HeadLine
}

// ParseRecord reads one Markdown file's text exactly as Read reads a file it
// walked: the record it declares, every refusal in its head, and whether the
// text is a record at all.
//
// It opens nothing and keeps nothing, and it is here so that a writer can put
// the bytes it is about to publish through the same parser that will read them
// back, and refuse before writing rather than leave a refused record on disk.
func ParseRecord(path, text string) (Record, []Problem, bool) { return parseRecord(path, text) }

// parseRecord reads one Markdown file. The last return says whether the file is
// a record at all: a title, a blank line, and a head line. A document that is
// none of these is not listed and is not refused — the rest of the checkout's
// Markdown stays browsable by path, with no kind claimed. Once a file does
// declare itself, every fault in its head is a refusal.
func parseRecord(path, text string) (Record, []Problem, bool) {
	lines := splitLines(text)
	if len(lines) < 3 {
		return Record{}, nil, false
	}
	title := strings.TrimSpace(strings.TrimPrefix(lines[0], "#"))
	if !strings.HasPrefix(lines[0], "# ") || title == "" {
		return Record{}, nil, false
	}
	if strings.TrimSpace(lines[1]) != "" {
		return Record{}, nil, false
	}
	if !headLinePattern.MatchString(lines[2]) {
		return Record{}, nil, false
	}

	record := Record{
		Title:    normalizeSpace(title),
		Path:     path,
		HeadLine: 3,
		lines:    map[string]int{},
	}
	var problems []Problem
	refuse := func(line int, message string) {
		problems = append(problems, Problem{Path: path, Line: line, Message: message})
	}

	values := map[string]string{}
	end := len(lines)
	for index := 2; index < len(lines); index++ {
		line, number := lines[index], index+1
		if strings.TrimSpace(line) == "" {
			end = index + 1
			break
		}
		match := headLinePattern.FindStringSubmatch(line)
		if match == nil {
			refuse(number, "the head line is not a `- Key: value` declaration: "+strings.TrimSpace(line))
			continue
		}
		key, value := match[1], strings.TrimSpace(match[2])
		record.Head = append(record.Head, Field{Key: key, Value: value, Line: number})
		if first, duplicate := record.lines[foldKey(key)]; duplicate {
			refuse(number, "the head declares "+key+" twice; it is already declared on line "+itoa(first))
			continue
		}
		record.lines[foldKey(key)] = number
		values[foldKey(key)] = value
	}
	if end < len(lines) {
		record.Body = lines[end:]
		record.BodyLine = end + 1
	}

	for _, key := range requiredKeys {
		if values[foldKey(key)] == "" {
			refuse(record.HeadLine, "the head does not declare "+key)
		}
	}
	record.Kind = values[foldKey("Kind")]
	record.ID = values[foldKey("Id")]
	record.Status = values[foldKey("Status")]
	record.Goals = strings.Fields(values[foldKey(goalsKey)])
	for _, key := range referenceKeys {
		references := strings.Fields(values[foldKey(key)])
		switch key {
		case "Cites":
			record.Cites = references
		case "Affects":
			record.Affects = references
		case "Governs":
			record.Governs = references
		case "By":
			record.By = references
		case "Supersedes":
			record.Supersedes = references
		}
	}
	return record, problems, true
}

// foldKey is how two spellings of one key meet: a head that writes `kind` has
// declared the kind, and has declared it once.
func foldKey(key string) string { return strings.ToLower(key) }

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	return strings.Split(text, "\n")
}

// normalizeSpace collapses a title's inner whitespace, so one record is one
// line in every listing whatever its file contains.
func normalizeSpace(value string) string { return strings.Join(strings.Fields(value), " ") }
