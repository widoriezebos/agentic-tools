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
// It is ordered by scope, because scope is what the project's own shape is:
// the records whose head names no goal come first, as the small set that
// shapes everything, and the goal-scoped ones follow grouped under the goal
// each one names. A Partner asked what this project has decided is asked
// about the first group far more often than about the last.
//
// It is bounded twice, by lines and by characters, because summaries are prose
// and a hundred rich ones exhaust a character bound long before a line bound.
// What does not fit is dropped a WHOLE GROUP at a time, from the end, and the
// groups dropped are named with their counts: a reader that knows which goals
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
		{name: projectWide("Decisions"), lines: recordLines(projectRecords(pane.Records), "decision")},
		{name: projectWide("Designs"), lines: recordLines(projectRecords(pane.Records), "design")},
		{name: projectWide("Open questions"), lines: questionLines(projectQuestions(pane.Questions))},
	}
	kinds = append(kinds, goalGroups(pane)...)
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
		built.WriteString("- This index reached its bound, so whole groups were left out of it rather than cut: " +
			strings.Join(left, "; ") + ". The records tool lists them.\n")
	}
	index.Block = built.String()
	return index
}

// fit keeps the groups that stay inside both bounds and reports the rest. It
// counts from the end because the order is the reading order: intent before
// doctrine before the decisions that rest on them, and the project's own
// records before the ones under one goal of it.
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

/* ----------------------------------------------------- the index by scope -- */

// projectWide is what one of the first three groups is called: the kind, said
// to be about the project as a whole, newest first.
//
// It says "about the project as a whole" and not "ungrouped", because that is
// what a head with no Goals line means in this grammar. A Partner told that
// the decisions here are the project's own can answer "what has this project
// decided" from them without reading the four hundred under goals.
func projectWide(kind string) string {
	return kind + " about the project as a whole, newest first"
}

// projectRecords is the records whose head names no goal.
func projectRecords(records []project.Record) []project.Record {
	kept := make([]project.Record, 0, len(records))
	for _, record := range records {
		if len(record.Goals) == 0 {
			kept = append(kept, record)
		}
	}
	return kept
}

// projectQuestions is the register's rows that name no goal.
func projectQuestions(questions []project.Question) []project.Question {
	kept := make([]project.Question, 0, len(questions))
	for _, question := range questions {
		if len(question.Goals) == 0 {
			kept = append(kept, question)
		}
	}
	return kept
}

// goalGroups is one group per goal anything is about: the decisions, the
// designs and the open questions whose head names it, in that order, newest
// first within each.
//
// The goals come in the ledger's own order, which is the order every listing
// of goals in this interface uses, and a goal named by a record that the
// ledger does not carry keeps its place at the end under the id it is named
// by: dropping it would drop the records that name it, and the check verb
// refuses those records in the same breath. A goal nothing is about has no
// group, because an empty group here would cost a line to say nothing.
//
// A record naming three goals stands under each of the three. That is the
// point of the grouping and not a duplication to be tidied away: a Partner
// asked about one goal is asked about every record that says it is about that
// goal, whatever else those records also say.
func goalGroups(pane project.Pane) []indexKind {
	order := make([]string, 0, len(pane.Goals))
	seen := map[string]bool{}
	titles := map[string]string{}
	for _, goal := range pane.Goals {
		if seen[goal.ID] {
			continue
		}
		seen[goal.ID] = true
		order = append(order, goal.ID)
		titles[goal.ID] = strings.TrimSpace(goal.Title)
	}
	named := func(id string) {
		if id == "" || seen[id] {
			return
		}
		seen[id] = true
		order = append(order, id)
	}
	for _, record := range pane.Records {
		for _, id := range record.Goals {
			named(id)
		}
	}
	for _, question := range pane.Questions {
		for _, id := range question.Goals {
			named(id)
		}
	}

	groups := make([]indexKind, 0, len(order))
	for _, id := range order {
		lines := recordLines(aboutGoal(pane.Records, id), "decision")
		lines = append(lines, recordLines(aboutGoal(pane.Records, id), "design")...)
		lines = append(lines, questionLines(questionsAbout(pane.Questions, id))...)
		if len(lines) == 0 {
			continue
		}
		groups = append(groups, indexKind{name: goalGroupName(id, titles[id]), lines: lines})
	}
	return groups
}

// goalGroupName is what one goal's group is called: its ledger id, and the
// title where the ledger carries one that says something the id does not.
func goalGroupName(id, title string) string {
	name := "Under the goal " + id
	if title != "" && title != id {
		name += " · " + title
	}
	return name + ", newest first"
}

// aboutGoal is the records whose head names this goal.
func aboutGoal(records []project.Record, id string) []project.Record {
	kept := make([]project.Record, 0, len(records))
	for _, record := range records {
		if includes(record.Goals, id) {
			kept = append(kept, record)
		}
	}
	return kept
}

// questionsAbout is the register's rows whose goals column names this goal.
func questionsAbout(questions []project.Question, id string) []project.Question {
	kept := make([]project.Question, 0, len(questions))
	for _, question := range questions {
		if includes(question.Goals, id) {
			kept = append(kept, question)
		}
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
