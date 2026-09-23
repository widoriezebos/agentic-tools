package partner

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// The project's memory, as a map handed over once at the start of a session.
//
// A Partner that has to search before it can answer "what has this project
// decided about X" spends a turn finding out that the project has decisions at
// all. So the first prompt of a session carries an index of them: the intent
// and doctrine chapters in the order their books say to read them, then the
// decisions, the designs and the open questions, newest first, one line each.
//
// It is a MAP AND NOT EVIDENCE, and it says so in its own first sentence. It
// is composed once, at a named moment; the session then runs for as long as an
// hour of idleness allows, and a question answered through the interface in
// the meantime leaves this index saying "open". Anything that turns on what is
// true NOW — does this exist, is it still open, is there nothing of this kind
// — is a tool call, and the tools read the project again when they are called.
//
// It is bounded twice, by lines and by characters, because summaries are prose
// and a hundred rich ones exhaust a character bound long before a line bound.
// What does not fit is dropped a WHOLE KIND at a time, from the end, and the
// kinds dropped are named with their counts: a reader that knows the designs
// were left out can ask for them, where a reader handed a list silently cut
// after the decisions cannot tell a short project from a trimmed one.

// The bounds on the index. They are the design's, and they are two because
// either alone can be reached first.
const (
	maxIndexLines      = 300
	maxIndexCharacters = 12000
)

// MemoryIndex is the index and the moment it was composed.
type MemoryIndex struct {
	// At is when the index was read. Every later prompt of the session names
	// it, so the Partner knows how old its map is.
	At time.Time
	// Block is the whole index, heading and all.
	Block string
}

// indexKind is one kind of record as the index lists it: what it is called,
// and the lines it contributed.
type indexKind struct {
	name  string
	lines []string
}

// Memory composes the index from the same reader the Project pane is composed
// from, so a row here and the row on the page are one reading.
func Memory(facts Facts, now time.Time) MemoryIndex {
	at := now.UTC()
	index := MemoryIndex{At: at}
	var built strings.Builder
	built.WriteString("The project's memory, indexed at " + at.Format(time.RFC3339) + "\n")
	built.WriteString("This index is a map from " + at.Format(time.RFC3339) +
		"; it is not evidence that something exists, is absent, or has the status shown. " +
		"For any of those, use the tools.\n")

	if facts.Project == nil {
		built.WriteString("- This build has no reader for the project's records, so there is no index of them.\n")
		index.Block = built.String()
		return index
	}
	pane, err := facts.Project()
	if err != nil {
		built.WriteString("- The project's records could not be read, so this index is empty rather than short: " +
			err.Error() + "\n")
		index.Block = built.String()
		return index
	}

	kinds := []indexKind{
		{name: "Intent, in reading order", lines: chapterLines(pane.Intent)},
		{name: "Doctrine, in reading order", lines: chapterLines(pane.Doctrine)},
		{name: "Decisions, newest first", lines: recordLines(pane.Records, "decision")},
		{name: "Designs, newest first", lines: recordLines(pane.Records, "design")},
		{name: "Open questions, newest first", lines: questionLines(pane.Questions)},
	}
	kept, dropped := fit(built.Len(), kinds)
	for _, kind := range kept {
		built.WriteString(kind.name + " (" + strconv.Itoa(len(kind.lines)) + "):\n")
		if len(kind.lines) == 0 {
			built.WriteString("- This reading found none.\n")
			continue
		}
		for _, line := range kind.lines {
			built.WriteString(line + "\n")
		}
	}
	if len(dropped) > 0 {
		left := make([]string, 0, len(dropped))
		for _, kind := range dropped {
			left = append(left, kind.name+" ("+strconv.Itoa(len(kind.lines))+")")
		}
		built.WriteString("- This index reached its bound, so whole kinds were left out of it rather than cut: " +
			strings.Join(left, "; ") + ". The records tool lists them.\n")
	}
	index.Block = built.String()
	return index
}

// fit keeps the kinds that stay inside both bounds and reports the rest. It
// counts from the end because the order is the reading order: intent before
// doctrine before the decisions that rest on them.
func fit(already int, kinds []indexKind) (kept, dropped []indexKind) {
	lines, characters := 0, already
	for at, kind := range kinds {
		cost, length := len(kind.lines)+1, len(kind.name)+16
		for _, line := range kind.lines {
			length += len(line) + 1
		}
		if lines+cost > maxIndexLines || characters+length > maxIndexCharacters {
			return kinds[:at], kinds[at:]
		}
		lines += cost
		characters += length
	}
	return kinds, nil
}

// chapterLines is one book's reading order. A chapter is a record by its id
// where it declares one and a bound document by its path where it does not,
// and neither is invented into the other: fetching them is two different
// tools, and a reference that lied about which would send a reader to a
// document the resolver does not carry.
func chapterLines(book project.Book) []string {
	lines := make([]string, 0, len(book.Chapters)+1)
	if book.Index != nil {
		lines = append(lines, indexLine(reference(book.Index.ID, book.Index.Path),
			book.Index.Title, book.Index.Status, book.Index.Summary))
	}
	for _, chapter := range book.Chapters {
		lines = append(lines, indexLine(reference(chapter.ID, chapter.Path),
			chapter.Title, "", chapter.Summary))
	}
	return lines
}

// recordLines is one kind of record, newest first by when its file was last
// written — which is the only age the records carry, and which the pane
// already reads for the same question a human asks on the page.
func recordLines(records []project.Record, kind string) []string {
	chosen := make([]project.Record, 0, len(records))
	for _, record := range records {
		if strings.EqualFold(record.Kind, kind) {
			chosen = append(chosen, record)
		}
	}
	sort.SliceStable(chosen, func(left, right int) bool {
		if chosen[left].ChangedAt != chosen[right].ChangedAt {
			return chosen[left].ChangedAt > chosen[right].ChangedAt
		}
		return chosen[left].Path < chosen[right].Path
	})
	lines := make([]string, 0, len(chosen))
	for _, record := range chosen {
		lines = append(lines, indexLine(reference(record.ID, record.Path),
			record.Title, record.Status, record.Summary))
	}
	return lines
}

// questionLines is the register's own rows, newest first by when each was
// opened. A question is named by its row id, which is what the questions tool
// answers about.
func questionLines(questions []project.Question) []string {
	chosen := append([]project.Question{}, questions...)
	sort.SliceStable(chosen, func(left, right int) bool {
		if chosen[left].Opened != chosen[right].Opened {
			return chosen[left].Opened > chosen[right].Opened
		}
		return chosen[left].ID < chosen[right].ID
	})
	lines := make([]string, 0, len(chosen))
	for _, question := range chosen {
		lines = append(lines, indexLine(question.ID, firstSentence(question.Question, maxIntent),
			question.Status, ""))
	}
	return lines
}

// reference is how to fetch the thing this line names: its declared record id,
// or the path it is bound at, marked so the two cannot be confused.
func reference(id, path string) string {
	if strings.TrimSpace(id) != "" {
		return id
	}
	if strings.TrimSpace(path) != "" {
		return "doc:" + path
	}
	return "this record declares no id and the index could not name its path"
}

// indexLine is one row: how to fetch it, what it is called, the status where
// one is declared, and the record's own first words where it has them. A field
// the record does not carry is left out rather than filled in.
func indexLine(reference, title, status, summary string) string {
	line := "- " + reference
	if trimmed := strings.TrimSpace(title); trimmed != "" {
		line += " · " + oneLine(trimmed)
	}
	if trimmed := strings.TrimSpace(status); trimmed != "" {
		line += " · " + trimmed
	}
	if trimmed := strings.TrimSpace(summary); trimmed != "" {
		line += " — " + firstSentence(trimmed, maxIndexSummary)
	}
	return line
}

// maxIndexSummary is how much of a record's own summary one line carries. The
// pane's summary is up to a paragraph; an index is a map, and a map's labels
// are short.
const maxIndexSummary = 240

// Returning is the one line a later prompt of the same session carries in the
// index's place: the map is still in the conversation, and this says how old
// it now is.
func Returning(at time.Time) string {
	return "The index of the project's memory you were given at the start of this conversation was read at " +
		at.UTC().Format(time.RFC3339) + ". It has not been read again since. " +
		"Anything that turns on what is true now — whether a record exists, whether a question is still open — " +
		"is a tool call, not a line of that index.\n"
}
