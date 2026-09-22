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

// requiredKeys are the four a head must declare. A key present with an empty
// value has declared nothing, so it is missing.
var requiredKeys = []string{"Kind", "Id", "Status", "Areas"}

// referenceKeys are the optional keys, parsed as plain whitespace-separated
// references and otherwise unvalidated: what they name may not exist yet, and
// step 1 does not pretend to know.
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
	Areas  []string
	Title  string
	Path   string // checkout-relative
	Home   string // checkout-relative home the record was found in

	Cites      []string
	Affects    []string
	Governs    []string
	By         []string
	Supersedes []string

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
	record.Areas = strings.Fields(values[foldKey("Areas")])
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
