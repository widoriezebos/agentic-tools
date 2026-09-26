package project

import (
	"sort"
	"strings"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
)

// Project → Sittings: the records this project has sat on (g1-s55 D3).
//
// It is a view over the records themselves and not a second store. A sitting
// keeps no working material of its own — the record IS the memory, in its four
// headed sections and, once the sitting has been closed, its Outcome — so the
// list of sittings is a reading of what those records carry.
//
// What it is NOT is a reading of the headings. Astra's F2: the record creator
// gives every design an "Outcome" heading and every intent an "Open questions"
// heading the moment it is written (write.go's templates), so a predicate over
// headings would list every design and every intent in the project as a sitting,
// which is the opposite of a list of the ones that were sat on. So sitting
// material is known by the marks its entries carry: the browser writes the
// identity of the deposit each entry was recorded from at the end of that entry's
// own line, and an Outcome written by End carries one the same way. A record with
// the four headings and no mark anywhere in them has never been sat on.
//
// The counts are of the piles as they stand, marked or not, because that is what
// the table counts and a row that disagreed with the table would be a second
// reading of one record. The instant is the last entry's own date, as the entry
// was written, because a record's own words are the only dating of an entry there
// is: the file's modification time is when the file was last touched, which a
// typo fixed by hand would move.

// The five headings a sitting writes under: the four piles the table shows, and
// the Outcome the close writes. They are the browser's own spellings
// (web/_app/src/partner/sitting.ts), because they are one agreement about one
// record and a second spelling of them here would be a second agreement.
const (
	pileFacts     = "Facts"
	pileProposals = "Proposals"
	pileDecisions = "Decisions"
	pileQuestions = "Open questions"
	pileOutcome   = "Outcome"
)

// The mark an entry carries, as it is written: the deposit's identity, in
// brackets, at the end of the entry's own line. What is read from it is only
// whether it is there — which is the whole of what "this was recorded in a
// sitting" means to this list.
const (
	markOpens  = "[d:"
	markCloses = "]"
)

// SatOn is the record a sitting was on, named as the Project pane names a
// record: by its id, which is what the record declares, and by its path, which
// is what the reader serves and what the sitting's own subject is.
type SatOn struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Path  string `json:"path"`
	Title string `json:"title"`
}

// PileCounts is how many entries each of the four piles holds.
type PileCounts struct {
	Facts     int `json:"facts"`
	Proposals int `json:"proposals"`
	Decisions int `json:"decisions"`
	Questions int `json:"questions"`
}

// Sitting is one row of the Sittings tab.
type Sitting struct {
	Record SatOn      `json:"record"`
	Counts PileCounts `json:"counts"`
	// LastAt is the date of the last entry recorded into this record, as the
	// entry itself carries it, or "" for a record whose only sitting material is
	// an Outcome that says no date.
	LastAt string `json:"lastAt"`
	// Standing says a sitting is open on this record right now.
	Standing bool `json:"standing"`
}

// sittingsOf is every record carrying recorded sitting material, newest entry
// first. A record with no mark anywhere in its five sections is not one.
func sittingsOf(read *resolver.Project) []Sitting {
	rows := []Sitting{}
	for _, record := range read.Records {
		counts, last, marked := satOn(record.Body)
		if !marked {
			continue
		}
		rows = append(rows, Sitting{
			Record: SatOn{Kind: record.Kind, ID: record.ID, Path: record.Path, Title: record.Title},
			Counts: counts,
			LastAt: last,
		})
	}
	newestFirst(rows)
	return rows
}

// newestFirst orders the list by the last entry recorded, latest first, and
// settles ties by the record's path so the order is the same on two reads.
//
// A row with no date at all comes first: it is either a sitting nobody has
// recorded into yet or one whose material carries no dating, and in both cases
// it is the row a human is most likely to be in the middle of.
func newestFirst(rows []Sitting) {
	sort.SliceStable(rows, func(one, two int) bool {
		if rows[one].LastAt != rows[two].LastAt {
			return rows[one].LastAt > rows[two].LastAt
		}
		return rows[one].Record.Path < rows[two].Record.Path
	})
}

// satOn reads one record's body: how many entries each pile holds, the date of
// the last entry recorded from a deposit, and whether any sitting material is
// there at all.
//
// The heading matches at any level and without regard to case, as the slice
// reader's does, because a record that nests its piles under a section is still
// carrying them. A section ends at the next heading of any level.
func satOn(body []string) (PileCounts, string, bool) {
	counts, last, marked := PileCounts{}, "", false
	pile := ""
	for _, line := range body {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			pile = headed(strings.TrimSpace(strings.TrimLeft(trimmed, "#")))
			continue
		}
		if pile == "" {
			continue
		}
		// The Outcome is prose and a mark on it is a mark: it says the close was
		// recorded here, which is the one thing this list reads it for.
		if pile == pileOutcome {
			marked = marked || hasMark(trimmed)
			continue
		}
		item, is := pileEntry(line)
		if !is {
			continue
		}
		countOne(&counts, pile)
		if !hasMark(item) {
			continue
		}
		marked = true
		if when := dated(item); when > last {
			last = when
		}
	}
	return counts, last, marked
}

// pileEntry is one entry of a pile: an unindented list item, exactly as the
// browser reads one back (web/_app/src/partner/sitting.ts's entriesIn).
//
// Unindented is the whole of the rule, and it is the rule because of what an
// entry's own clause looks like: "  - Anchor: internal/session/session.go:212"
// is a nested item under the entry above it, and a reader that counted it would
// say a pile of one entry holds two. So the indentation is not trimmed away
// before the question is asked, and the counts here are the counts the table
// shows.
func pileEntry(line string) (string, bool) {
	for _, marker := range []string{"- ", "* "} {
		if rest, found := strings.CutPrefix(line, marker); found {
			return strings.TrimSpace(rest), true
		}
	}
	return "", false
}

// hasMark reports whether one line carries a deposit's mark: the opener, and a
// closer after it. Both, because a line that says "[d:" and never closes it is a
// human writing brackets rather than this build writing a mark.
func hasMark(said string) bool {
	_, after, opened := strings.Cut(said, markOpens)
	return opened && strings.Contains(after, markCloses)
}

// headed is which of the five sections a heading names, or "" for any other
// heading — which ends whichever section was open.
func headed(said string) string {
	for _, one := range []string{pileFacts, pileProposals, pileDecisions, pileQuestions, pileOutcome} {
		if strings.EqualFold(said, one) {
			return one
		}
	}
	return ""
}

func countOne(counts *PileCounts, pile string) {
	switch pile {
	case pileFacts:
		counts.Facts++
	case pileProposals:
		counts.Proposals++
	case pileDecisions:
		counts.Decisions++
	case pileQuestions:
		counts.Questions++
	}
}

// entryParts is the separator an entry's dated, attributed head is written
// with: the date, the name, and then the words. It is the browser's own
// separator, for the headings' reason.
const entryParts = " · "

// dated is the date one entry was recorded on, as the entry carries it, or ""
// for a line whose head is not the dated one this build writes.
func dated(item string) string {
	said, _, split := strings.Cut(item, entryParts)
	if !split {
		return ""
	}
	return strings.TrimSpace(said)
}

// MarkStanding says that a sitting is open on one record right now, and lists
// that record even where nothing has been recorded into it yet (g1-s55 D3).
//
// A standing sitting with no entry is exactly the sitting a human is in the
// middle of, and the list it must not be missing from. What it carries is what
// is true of it: the record, no counts, and no last entry — rather than a row
// pretending something was recorded.
//
// The subject is a record's path, which is what a sitting's subject is. A path
// this pane's records do not carry is listed by its path alone: it is still the
// record the human is sitting on, and saying so with the little that is known is
// better than dropping the one row they are working in.
func (p *Pane) MarkStanding(subject string) {
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return
	}
	for at := range p.Sittings {
		if p.Sittings[at].Record.Path == subject {
			p.Sittings[at].Standing = true
			return
		}
	}
	row := Sitting{Record: SatOn{Path: subject}, Standing: true}
	for _, record := range p.Records {
		if record.Path == subject {
			row.Record = SatOn{Kind: record.Kind, ID: record.ID, Path: record.Path, Title: record.Title}
			break
		}
	}
	p.Sittings = append([]Sitting{row}, p.Sittings...)
}
