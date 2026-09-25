// Package application composes the Application page: what this workspace has
// concluded, what is known to be wrong with it, and what it says it is.
//
// It reads nothing. Everything it answers from is read by somebody else: the
// board projection every other page shares, the known-issues register, this
// seat's own presence record as the Fleet page reads it, the checkout's
// documents, and the visit window. Compose is a pure function over those, so
// every rule below is a rule a test states rather than a shape a request
// happens to produce.
//
// One rule matters more than the rest, and it is the reason the block is
// called "What concluded" rather than "What it does". A done goal carries a
// conclusion — one sentence written when it concluded — and a doneAt that
// dates that conclusion. Some of those conclusions record an administrative
// end: a duplicate withdrawn, a requirement absorbed into another goal. So
// this page answers "what has landed, in the ledger's own words, and when",
// which is a work history. It does not establish what the application can do
// today, and its help term says so. A capability map would answer that, and
// the kit records none.
//
// Nothing here acts, and nothing here judges a problem. The register's words
// are the record: a row is concluded when its status begins with one of the
// five words the reader knows, the word stays visible, and a row the reader
// could not read is counted and named rather than dropped.
//
// The design is plans/designs/user-interface/g1-s49-application-what-it-does-now.md.
package application

import (
	"sort"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/knownissues"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// SchemaVersion is the shape of the application resource a reader parses.
const SchemaVersion = 1

// PageName is this page's own entry in the visit file. It is spelled here so
// that the server and every fixture name one entry rather than two spellings
// of one page.
const PageName = "application"

// registerPath is where the known-issues register is from the checkout, where
// the caller names no path. It is the kit's own layout and is right only where
// the checkout and the installation are the same directory; every caller that
// knows both roots hands the real one in.
const registerPath = "memory/known-issues.md"

// The documents this page links to, in the order it lists them, and the name
// each link carries.
//
// The name is written here rather than taken from the file's own first
// heading: "README" is what a human is looking for, and a heading that reads
// "MetaSystem" would be a link that says what the workspace is called rather
// than what the document is. Only the ones this checkout actually has are
// listed — an adopted application with no glossary gets no glossary link
// rather than a link that would refuse.
var documentsShown = []Document{
	{Title: "README", Path: "README.md"},
	{Title: "Concepts", Path: "docs/concepts.md"},
	{Title: "Glossary", Path: "docs/glossary.md"},
}

// Engine is this seat's last published engine build, as the Fleet page reads
// it from the seat's own presence record.
//
// It is a record at ONE TICK and never a statement about what is running now:
// a seat that has not ticked since it was rebuilt publishes the build it had
// when it last ticked. PublishedAt is that tick, carried so the page can say
// when the claim was true. A seat with no record at all has no Engine, and the
// page says that rather than showing a blank build.
type Engine struct {
	Build      string `json:"build"`
	Generation int    `json:"generation"`
	// PublishedAt is the tick the record was written at, in RFC3339.
	PublishedAt string `json:"publishedAt"`
}

// Counts is the header's own line: the whole history, this month of it, and
// what landed since this human last read this page.
type Counts struct {
	// Landed is every concluded goal the ledger carries. It is named landed
	// in the payload and said as "goals concluded" on the page: a conclusion
	// is an end, and "landed" alone would claim every one of them shipped
	// something.
	Landed    int `json:"landed"`
	ThisMonth int `json:"thisMonth"`
	New       int `json:"new"`
}

// Landed is one concluded goal, in the ledger's own words.
type Landed struct {
	ID     string `json:"id"`
	Intent string `json:"intent"`
	// Concluded is the one sentence written when the goal concluded, whole.
	Concluded string `json:"concluded"`
	// DoneAt is when that conclusion was written, or "" where nothing dated
	// it. An undated row sorts last and is never new.
	DoneAt string   `json:"doneAt"`
	Labels []string `json:"labels"`
	Arc    string   `json:"arc"`
	New    bool     `json:"new"`
}

// Problems is the known-issues register as this page shows it.
type Problems struct {
	// Columns are the header's own names, kept as supplied because the fifth
	// differs between the two column sets.
	Columns   []string          `json:"columns"`
	Open      []knownissues.Row `json:"open"`
	Concluded []knownissues.Row `json:"concluded"`
	Unread    int               `json:"unread"`
	Defects   []string          `json:"defects"`
	// Register is where the register is RELATIVE TO THE CHECKOUT, which is
	// what the document reader opens a path against, so that "Open the
	// register" opens.
	Register string `json:"register"`
}

// Document is one link into the Project pane's document reader. Nothing is
// rendered on this page: the reader renders documents, and there is one of it.
type Document struct {
	Title string `json:"title"`
	Path  string `json:"path"`
}

// Visit is the window every row's New was decided against: the end of this
// human's previous visit to THIS page, or a day back on a first one.
type Visit struct {
	Since string `json:"since"`
	First bool   `json:"first"`
}

// Page is the whole of Application, composed once, as it was at readAt.
type Page struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReadAt        string `json:"readAt"`
	// Subject and Mode are the workspace's own, so the page can tell the
	// product under development from the machinery doing the work without a
	// second read.
	Subject  string     `json:"subject"`
	Mode     string     `json:"mode"`
	Engine   *Engine    `json:"engine"`
	Counts   Counts     `json:"counts"`
	Landed   []Landed   `json:"landed"`
	Problems Problems   `json:"problems"`
	Docs     []Document `json:"docs"`
	Visit    Visit      `json:"visit"`
}

// Inputs is everything Compose reads. Each field is somebody else's answer,
// carried here as it was given.
type Inputs struct {
	Subject string
	Mode    string
	// Engine is this seat's presence record as Fleet reads it, or nil where
	// this seat has published none.
	Engine *Engine
	// Closed is the board projection's concluded goals, exactly as the board
	// and the Decisions page read them.
	Closed []backlog.Row
	// Register is one read of the known-issues register.
	Register knownissues.Register
	// RegisterPath is where that register is relative to the CHECKOUT, which
	// is not where it was read from. An empty path takes the kit's layout.
	RegisterPath string
	// Documents is the checkout's Markdown documents as the Project pane
	// lists them; this page links the few of them it names.
	Documents []project.File
	// Since is the start of the window New is decided against, from this
	// page's own visit entry. A zero instant is a page composed over no
	// window, and nothing on it is new.
	Since time.Time
	First bool
}

// Compose is the whole page, from the inputs above, as they stood at now.
func Compose(in Inputs, now time.Time) Page {
	landed := landedOf(in.Closed, in.Since)
	return Page{
		SchemaVersion: SchemaVersion,
		ReadAt:        stamp(now),
		Subject:       in.Subject,
		Mode:          in.Mode,
		Engine:        in.Engine,
		Counts:        countsOf(landed, now),
		Landed:        landed,
		Problems:      problemsOf(in),
		Docs:          documentsOf(in.Documents),
		Visit:         Visit{Since: stamp(in.Since), First: in.First},
	}
}

// landedOf is every concluded goal, newest first, the undated last.
//
// It is the done lane only. An abandoned goal is in the closed rows too, and
// it is not something that concluded: it is work this project decided not to
// do, which belongs to the backlog's own reading of the ledger and not to an
// account of what the application has.
func landedOf(closed []backlog.Row, since time.Time) []Landed {
	rows := []Landed{}
	for _, row := range closed {
		if row.Lane != backlog.LaneDone {
			continue
		}
		rows = append(rows, Landed{
			ID: row.ID, Intent: row.Intent, Concluded: row.Concluded,
			DoneAt: row.DoneAt, Labels: list(row.Labels), Arc: row.Arc,
			New: newSince(row.DoneAt, since),
		})
	}
	// Newest first, and an undated row after every dated one: a goal nothing
	// dated cannot be placed in a week, and putting it at the top would put
	// the least-known rows where the freshest ones belong. The id breaks a
	// tie so that two conclusions written in the same second keep one order
	// between reads.
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if (left.DoneAt == "") != (right.DoneAt == "") {
			return right.DoneAt == ""
		}
		if left.DoneAt != right.DoneAt {
			return left.DoneAt > right.DoneAt
		}
		return left.ID < right.ID
	})
	return rows
}

// countsOf is the header's line. This month is the observing clock's own UTC
// month, because the dates are UTC instants and a month boundary read in
// another zone would move rows between two answers on the same read.
func countsOf(landed []Landed, now time.Time) Counts {
	counts := Counts{Landed: len(landed)}
	month := now.UTC().Format("2006-01")
	for _, row := range landed {
		if at, dated := instant(row.DoneAt); dated && at.UTC().Format("2006-01") == month {
			counts.ThisMonth++
		}
		if row.New {
			counts.New++
		}
	}
	return counts
}

// newSince is whether a conclusion written at doneAt is new against a window
// that began at since.
//
// A row nothing dated is never new: the record does not say when it
// concluded, and a page that guessed would be marking rows new on no
// evidence. A window nothing set makes nothing new, for the same reason.
func newSince(doneAt string, since time.Time) bool {
	if doneAt == "" || since.IsZero() {
		return false
	}
	at, dated := instant(doneAt)
	return dated && at.After(since)
}

// problemsOf is the register as the page shows it, with every list written
// whether the register had anything to say or not.
func problemsOf(in Inputs) Problems {
	problems := Problems{
		Columns:   list(in.Register.Columns),
		Open:      rows(in.Register.Open),
		Concluded: rows(in.Register.Concluded),
		Unread:    in.Register.Unread,
		Defects:   list(in.Register.Defects),
		Register:  registerPath,
	}
	if in.RegisterPath != "" {
		problems.Register = in.RegisterPath
	}
	return problems
}

// documentsOf is the few documents this page links, in its own order, and
// only the ones this checkout has.
func documentsOf(files []project.File) []Document {
	held := map[string]bool{}
	for _, file := range files {
		held[file.Path] = true
	}
	shown := []Document{}
	for _, candidate := range documentsShown {
		if held[candidate.Path] {
			shown = append(shown, candidate)
		}
	}
	return shown
}

func rows(read []knownissues.Row) []knownissues.Row {
	if read == nil {
		return []knownissues.Row{}
	}
	return read
}

func list(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func instant(at string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(at))
	return parsed, err == nil
}

func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}
