package project

import (
	"strconv"
	"strings"
)

// DocPrefix binds a chapter to a document that is not a record: an existing
// file, named by its checkout-relative path, left exactly where it is. It is
// how a paper becomes a project's intent without moving a byte.
const DocPrefix = "doc:"

// The statuses a question row carries, as everything that counts a question
// counts it. An answer names what answered it, so the register writes the word
// with a colon and the reference after it; the word is the status.
const (
	QuestionOpen      = "open"
	QuestionAnswered  = "answered"
	QuestionWithdrawn = "withdrawn"
)

// AnsweredPrefix is how the register spells an answer: the word, a colon, and
// what answered the question.
const AnsweredPrefix = QuestionAnswered + ":"

// registerHeader is the one table the register holds, by its columns.
var registerHeader = []string{"id", "opened", "question", "areas", "status"}

// dashes are the separators an index line may put between a slug or an id and
// the name that reads it. The design writes an em dash; an en dash is the same
// intent typed differently, and is read the same way.
var dashes = []string{"—", "–"}

// parseAreas reads the `## Areas` list: the one place areas are declared, flat,
// a slug and a name. A line with no name declares the slug alone.
func parseAreas(body []string, firstLine int) []Area {
	var areas []Area
	for _, entry := range section(body, firstLine, "Areas") {
		slug, name := splitOnDash(entry.text)
		if slug == "" {
			continue
		}
		areas = append(areas, Area{Slug: slug, Name: name, Line: entry.line})
	}
	return areas
}

// parseChapters reads the `## Chapters` list: reading order, by record id or by
// bound document.
func parseChapters(body []string, firstLine int) []Chapter {
	var chapters []Chapter
	for _, entry := range section(body, firstLine, "Chapters") {
		name, title := splitOnDash(entry.text)
		if name == "" {
			continue
		}
		chapter := Chapter{Title: title, Line: entry.line}
		if binding := strings.TrimPrefix(name, DocPrefix); binding != name {
			chapter.Doc = strings.TrimSpace(binding)
		} else {
			chapter.ID = name
		}
		chapters = append(chapters, chapter)
	}
	return chapters
}

type sectionEntry struct {
	text string
	line int
}

// section collects the list items under one heading, whatever its level, and
// stops at the next heading.
func section(body []string, firstLine int, heading string) []sectionEntry {
	var entries []sectionEntry
	inside := false
	for index, line := range body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			inside = strings.TrimSpace(strings.TrimLeft(trimmed, "#")) == heading
			continue
		}
		if !inside || !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		entries = append(entries, sectionEntry{
			text: strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")),
			line: firstLine + index,
		})
	}
	return entries
}

// splitOnDash divides a list item into the name it declares and the prose that
// reads it.
func splitOnDash(text string) (string, string) {
	for _, dash := range dashes {
		if before, after, found := strings.Cut(text, dash); found {
			return strings.TrimSpace(before), normalizeSpace(after)
		}
	}
	return strings.TrimSpace(text), ""
}

// parseQuestions reads the one table in the register. Rows before the header
// and prose after the table are not rows; a row with fewer cells than the
// header has declared nothing in the cells it omits, which the check reads as
// the absence it is.
func parseQuestions(text string) []Question {
	var questions []Question
	lines := splitLines(text)
	started := false
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			if started {
				break
			}
			continue
		}
		cells := tableCells(trimmed)
		if !started {
			started = sameCells(lower(cells), registerHeader)
			continue
		}
		if isDelimiterRow(cells) {
			continue
		}
		questions = append(questions, Question{
			ID:     cell(cells, 0),
			Opened: cell(cells, 1),
			Text:   cell(cells, 2),
			Areas:  strings.Fields(cell(cells, 3)),
			Status: cell(cells, 4),
			Line:   index + 1,
		})
	}
	return questions
}

func tableCells(line string) []string {
	trimmed := strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	cells := strings.Split(trimmed, "|")
	for index := range cells {
		cells[index] = strings.TrimSpace(cells[index])
	}
	return cells
}

func isDelimiterRow(cells []string) bool {
	for _, value := range cells {
		if strings.Trim(value, "-: ") != "" || value == "" {
			return false
		}
	}
	return len(cells) > 0
}

func sameCells(observed, expected []string) bool {
	if len(observed) != len(expected) {
		return false
	}
	for index := range observed {
		if observed[index] != expected[index] {
			return false
		}
	}
	return true
}

func lower(values []string) []string {
	lowered := make([]string, len(values))
	for index, value := range values {
		lowered[index] = strings.ToLower(value)
	}
	return lowered
}

func cell(cells []string, index int) string {
	if index >= len(cells) {
		return ""
	}
	return cells[index]
}

// questionStatusValid admits the three the register may carry, and requires an
// answer to name what answered it.
func questionStatusValid(status string) bool {
	switch {
	case status == QuestionOpen, status == QuestionWithdrawn:
		return true
	case strings.HasPrefix(status, AnsweredPrefix):
		return strings.TrimSpace(strings.TrimPrefix(status, AnsweredPrefix)) != ""
	}
	return false
}

// State is the word this question stands under, whatever the row wrote after
// it: an answer is answered. A status the register does not carry stands as it
// was written, so what the check refuses is also what a listing shows.
func (q Question) State() string {
	if strings.HasPrefix(q.Status, AnsweredPrefix) {
		return QuestionAnswered
	}
	return q.Status
}

// Answers is what an answered row names as having answered it, and nothing for
// a row that names nothing. It is the one reference a question carries, and no
// head declares it: the register's status column does.
func (q Question) Answers() []string {
	if !strings.HasPrefix(q.Status, AnsweredPrefix) {
		return nil
	}
	return strings.Fields(strings.TrimPrefix(q.Status, AnsweredPrefix))
}

func itoa(value int) string { return strconv.Itoa(value) }
