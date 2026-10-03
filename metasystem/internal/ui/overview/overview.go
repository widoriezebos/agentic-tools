// Package overview composes the page a human lands on when they come back to
// the project.
//
// It reads nothing. Everything it answers from is already read by somebody
// else — the project's records, the backlog projection, the steward's journal,
// who the server is acting as, and the visit window beside this file — and
// Compose is a pure function over those five, so every rule below is a rule a
// test states rather than a shape a request happens to produce.
//
// The order of the page is the order of the questions: what needs me, what
// changed while I was away, what is being worked on now, what the project's
// memory holds, and whether anything is wrong. Every number the page shows is
// a count of something this package placed, and every item carries a Where:
// the kind of thing it is and which one, never a route. Addresses belong to
// the browser, and a server that spelled them would be a second router.
//
// Nothing here judges what the engine judges. A lane is the projection's, an
// approval is the record's, a refusal is the check verb's, and a delivery is
// the steward's; this package counts them, caps the lists, and says which
// surface opens each one.
package overview

import (
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// SchemaVersion is the shape of the overview resource a reader parses.
const SchemaVersion = 1

// The two caps. A block that leads with what needs a human shows three,
// because three is what fits above the fold of a block that has five of them;
// a block that reports a window shows five, because a window that changed
// nine things is a window worth scrolling once. Both carry the whole count
// beside the list, so "and N more" is arithmetic the reader can check.
const (
	shortList = 3
	longList  = 5
)

// The kinds a Where can name. Each one is a surface this build serves, and the
// browser decides what address, tab or panel that is.
const (
	// WhereGoal is one goal of the ledger, named by its id.
	WhereGoal = "goal"
	// WhereDocument is one document of the checkout, named by its path.
	WhereDocument = "document"
	// WhereDecisions is the Decisions page, where a seat's open question is
	// read and answered; the id names the question.
	WhereDecisions = "decisions"
	// WhereNotification is one line of the steward's journal, named by its
	// id: the notifications panel, opened at that row.
	WhereNotification = "notification"
	// WhereBacklog is the board, named by the lane it is about, or by
	// nothing at all.
	WhereBacklog = "backlog"
)

// The lane the strip names that is not a lane of the projection: the goals
// concluded on the observation day.
const LaneDoneToday = "done-today"

// The record kinds this page counts separately.
const (
	kindDecision = "decision"
	kindDesign   = "design"
)

// The record status a draft declares, and the one a finished design declares.
const (
	statusDraft = "draft"
	statusDone  = "done"
)

// The goal state a concluded goal carries.
const stateDone = "done"

// Where names one destination without spelling it: what kind of thing it is
// and which one. The browser owns addresses, panels and sheets, so a page that
// carried a path here would be a second router disagreeing with the first.
type Where struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Item is one thing a human can open: what it is called, the one fact that
// says why it is on this page, when that fact happened, and where it opens.
//
// Every field is written, including the ones a particular row has nothing for,
// so a reader never has to tell an absent field from an empty one.
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Note is the one fact beside the title: a kind, a verb, a reason.
	Note string `json:"note"`
	// At is when the fact happened, in RFC3339, or "" where nothing dated it.
	At    string `json:"at"`
	Where Where  `json:"where"`
}

// Group is a capped list with the whole count beside it. The count is what the
// page shows as a number and the items are what it shows as rows, so "and N
// more" is Count minus len(Items) and never a second count.
type Group struct {
	Count int    `json:"count"`
	Items []Item `json:"items"`
}

// Page is the whole of Overview, composed once, as it was at readAt.
type Page struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReadAt        string `json:"readAt"`
	// Since is the start of the window "what changed" is read over: the end
	// of the previous visit, or a day back on a first visit.
	Since string `json:"since"`
	// First says the window is a first visit's day rather than a marker the
	// server kept, so the page can say so instead of naming an instant
	// nobody was there for.
	First    bool     `json:"first"`
	NeedsYou NeedsYou `json:"needsYou"`
	Changed  Changed  `json:"changed"`
	Work     Work     `json:"work"`
	Memory   Memory   `json:"memory"`
	Health   Health   `json:"health"`
}

// NeedsYou is the primary block: what is waiting on this human, in the order
// the design leads with. It is the Decisions inbox counted by kind, so every
// row of that inbox is in exactly one group here.
type NeedsYou struct {
	// Approvals are the first-priority goals in To Do nobody has approved.
	Approvals Group `json:"approvals"`
	// Questions are the open questions seats are waiting on an answer to.
	Questions Group `json:"questions"`
	// Drafts are records declaring draft status, whatever their kind.
	Drafts Group `json:"drafts"`
	// Designs are designs whose named goals have all landed and which are
	// not marked done.
	Designs Group `json:"designs"`
	// Alerts are the steward's alerts and handoffs from the last seven days.
	Alerts Group `json:"alerts"`
	// Other counts every inbox row of a kind the five groups above do not
	// list. It carries no items: the Decisions page is where they are read.
	Other Group `json:"other"`
	// SignIn is true when nothing proves a human on this seat, which is a
	// row of its own: everything else here is something to read, and this is
	// the one thing that stops the page from being able to act.
	SignIn bool `json:"signIn"`
	// Total is the length of the Decisions inbox, which is the six groups'
	// counts added up, and does not count SignIn: a seat nobody is signed
	// into still has nothing waiting on them, and the two statements are
	// different. The page is empty when Total is zero and SignIn is false.
	Total int `json:"total"`
}

// Changed is what happened in the window, whether or not it needs anybody.
type Changed struct {
	Concluded Group `json:"concluded"`
	// Moved are goals the ledger wrote to that are not concluded, with the
	// last verb the row can be trusted to carry.
	Moved   Group `json:"moved"`
	Records Group `json:"records"`
	// Messages is how many lines the steward wrote in the window. It is a
	// count and not a list: the panel is where they are read.
	Messages int `json:"messages"`
	Total    int `json:"total"`
}

// Seat is who holds a goal: the claim's machine and its lineage.
type Seat struct {
	Machine string `json:"machine"`
	Lineage string `json:"lineage"`
}

// Holder is the presence standing of the machine a claim names: whether it
// has been heard from, since when, and the words the row carries when it has
// not. It is a flag and never an act — the goal stays claimed — and it is
// filled by this composition from a lookup the caller hands in, because the
// backlog projection knows who claimed a goal and nothing about presence.
type Holder struct {
	Machine  string `json:"machine"`
	Standing string `json:"standing"`
	Since    string `json:"since"`
	Flag     string `json:"flag"`
}

// Claimed is one In Progress goal, with the seat that holds it, the phase the
// projection could name, and when the claim was taken.
type Claimed struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Seat  Seat   `json:"seat"`
	Phase string `json:"phase"`
	At    string `json:"at"`
	// Holder is the standing of that seat, where this build could read one.
	Holder *Holder `json:"holder,omitempty"`
}

// Waiting is the held work: how much of it there is, and the oldest one's
// reason, which is the one a human can usually do something about.
type Waiting struct {
	Count  int    `json:"count"`
	ID     string `json:"id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
	Since  string `json:"since"`
}

// Lane is one count of the strip. The id is the projection's own lane id, or
// LaneDoneToday, and the browser knows what each is called.
type Lane struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// Work is what is being worked on now.
type Work struct {
	InProgress []Claimed `json:"inProgress"`
	// Next is the first three of Ready for Work, by rank.
	Next    []Item  `json:"next"`
	Waiting Waiting `json:"waiting"`
	Lanes   []Lane  `json:"lanes"`
}

// Book is one of the two indexes as a number and a sentence.
type Book struct {
	Chapters int `json:"chapters"`
	// Summary is the index's own first sentence, or "" where the index
	// declares none. A book with no index at all is zero chapters and no
	// sentence, which the page says rather than invents.
	Summary string `json:"summary"`
}

// Tally is a kind of record counted with the part of it that is still draft.
type Tally struct {
	Total  int `json:"total"`
	Drafts int `json:"drafts"`
}

// Progress is one design in flight: how many of the goals it names have
// landed, out of how many it names.
type Progress struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Path  string `json:"path"`
	Done  int    `json:"done"`
	Goals int    `json:"goals"`
}

// Designs is the design shelf: how many there are, how many are finished, and
// how far each unfinished one has got.
type Designs struct {
	Total int `json:"total"`
	Done  int `json:"done"`
	// InFlight is every design that names a goal and is not marked done;
	// Progress is the first five of them.
	InFlight int        `json:"inFlight"`
	Progress []Progress `json:"progress"`
}

// Scope is one kind of record counted by what it is about: the project's own
// — the ones whose head names no goal — and the ones that name at least one.
//
// A record naming three goals is one record here and not three: this counts
// records, and standing under each goal it names is a grouping rather than a
// count.
type Scope struct {
	Own        int `json:"own"`
	UnderGoals int `json:"underGoals"`
}

// Scoped is the three kinds the Project page's scope control narrows, each
// counted both ways.
//
// The tiles show the project's own count with the rest named beneath it,
// because scope is a filter with a sensible default rather than two lists: the
// project's own records are the small set that shapes everything, and a tile
// that counted four hundred designs said only that the project is large.
// Intent and doctrine are not here: a chapter of either is about the project
// as a whole by definition, so both sides of the count would be the same
// number and nothing.
type Scoped struct {
	Decisions Scope `json:"decisions"`
	Designs   Scope `json:"designs"`
	Questions Scope `json:"questions"`
}

// Memory is what the project's records hold, as counts that open their tabs.
type Memory struct {
	Intent    Book    `json:"intent"`
	Doctrine  Book    `json:"doctrine"`
	Decisions Tally   `json:"decisions"`
	Designs   Designs `json:"designs"`
	Questions int     `json:"questions"`
	// Scoped is the same three kinds counted by what they are about, which is
	// what the tiles read their figures from.
	Scoped Scoped `json:"scoped"`
}

// Health is one calm line, or the problems.
type Health struct {
	OK bool `json:"ok"`
	// SyncedAt is when the last fetch landed, in RFC3339, which is what the
	// calm line names. The page writes the clock time in the reader's own
	// locale, so the instant travels rather than the words.
	SyncedAt string `json:"syncedAt"`
	// Freshness is the ledger pill's state — "current", "behind" or "failed"
	// — so that the page says which of the three it is from what was judged
	// rather than by reading the problem's sentence back.
	Freshness snapshot.Freshness `json:"freshness"`
	Problems  Group              `json:"problems"`
}

// Ledger is what the backlog's own statement says about the accepted ledger,
// reduced to the facts this page judges health on. It is a value rather than
// the observation itself so that Compose stays a function over data and the
// route is the one place that reads a running engine.
type Ledger struct {
	// Freshness is what the interface's own fetch loop was judged to be:
	// "current", "behind" or "failed". It is the word the pill says, so the
	// page never has to read a sentence back to find out which of the three
	// it is looking at.
	Freshness snapshot.Freshness
	// AtTip is the statement's own answer: the tree was read, and this
	// interface's freshness is current.
	AtTip bool
	// SyncedAt is when the last fetch that landed finished.
	SyncedAt time.Time
	// Statement is the engine's own words for what is wrong, where AtTip says
	// something is.
	Statement string
}

// Standing is who the server is acting as, reduced to the one fact this page
// asks: is there a human behind it at all.
type Standing struct {
	Proven bool
}

// Inputs is everything Compose reads. Each field is somebody else's answer,
// carried here as it was given.
type Inputs struct {
	// Project is the pane over the checkout's declared records.
	Project project.Pane
	// Rows and Closed are the backlog projection's live and concluded goals.
	Rows   []backlog.Row
	Closed []backlog.Row
	// Counts is the projection's count per lane.
	Counts map[backlog.Lane]int
	Ledger Ledger
	// Journal is the steward's notification journal, newest first, as the
	// notifications package serves it.
	Journal []notifications.Notice
	// Inbox is the Decisions inbox composed from the same answers at the same
	// instant, one row for each of its rows. It is the one owner of what
	// needs this human.
	Inbox []Need
	// Stages is the stage of each goal the host board believes a card for, by
	// goal id. A goal with no card, or one whose card cannot be believed, is
	// not in it.
	Stages map[string]string
	// Holders is the presence standing of each machine the rows name, by
	// machine nickname. It is handed in rather than read here for the reason
	// every other input is: this package composes and opens nothing.
	Holders map[string]Holder
	Human   Standing
	// Since is the start of the comparison window, and First says it is a
	// first visit's day rather than a marker a previous visit left.
	Since time.Time
	First bool
}

// Compose is the whole page, from the five answers above, as they stood at
// now. It reads no file, opens no connection and keeps nothing.
func Compose(in Inputs, now time.Time) Page {
	return Page{
		SchemaVersion: SchemaVersion,
		ReadAt:        stamp(now),
		Since:         stamp(in.Since),
		First:         in.First,
		NeedsYou:      needsYou(in),
		Changed:       changed(in),
		Work:          workNow(in, now),
		Memory:        memory(in),
		Health:        health(in, now),
	}
}

/* ------------------------------------------------------------- needs you -- */

// The groups of the block a row of the inbox can be listed in. A row of any
// other kind names no group and is counted, not listed.
const (
	NeedApproval = "approval"
	NeedQuestion = "question"
	NeedDraft    = "draft"
	NeedDesign   = "design"
	NeedAlert    = "alert"
)

// Need is one row of the Decisions inbox as this page counts it: the group it
// is listed in, and the few facts a listed row shows. The inbox's owner fills
// it, because this package may not read the inbox's own shape: that package
// reads the Partner's proposals, and the Partner reads this page.
type Need struct {
	Group string
	ID    string
	Title string
	// By is who or what asks, and Since when the asking began.
	By    string
	Since string
	// Path is the record a draft or a landed design is about.
	Path string
	// Row is the backlog row of a goal awaiting approval.
	Row *backlog.Row
}

// needsYou is the Decisions inbox, counted by kind.
//
// The inbox decides what waits on this human and this block decides nothing
// again: Total is the inbox's length, each of the five groups the page leads
// with is the rows of one kind with the first three listed, and every other
// kind is counted in Other. So the groups always add up to the total, and the
// total is the figure the Decisions page shows for the same inputs.
func needsYou(in Inputs) NeedsYou {
	groups := map[string][]Need{}
	for _, need := range in.Inbox {
		groups[need.Group] = append(groups[need.Group], need)
	}
	return NeedsYou{
		Approvals: listed(groups[NeedApproval], approvalItem),
		Questions: listed(groups[NeedQuestion], func(need Need) Item {
			return Item{
				ID: need.ID, Title: need.Title, Note: "open", At: need.Since,
				Where: Where{Kind: WhereDecisions, ID: need.ID},
			}
		}),
		Drafts:  listed(groups[NeedDraft], recordNeed),
		Designs: listed(groups[NeedDesign], recordNeed),
		Alerts: listed(groups[NeedAlert], func(need Need) Item {
			return Item{
				ID: need.ID, Title: need.Title, Note: need.By, At: need.Since,
				Where: Where{Kind: WhereNotification, ID: need.ID},
			}
		}),
		Other:  Group{Count: len(groups[""]), Items: []Item{}},
		SignIn: !in.Human.Proven,
		Total:  len(in.Inbox),
	}
}

// listed is one kind of inbox row as a group: all of them counted, and the
// first three shown, in the inbox's own order.
func listed(needs []Need, shape func(Need) Item) Group {
	items := []Item{}
	for _, need := range take(needs, shortList) {
		items = append(items, shape(need))
	}
	return Group{Count: len(needs), Items: items}
}

// approvalItem is a goal awaiting approval as the board's own row words it.
// An approval row carries its backlog row; one that somehow does not is still
// listed, under the inbox's title for it.
func approvalItem(need Need) Item {
	if need.Row != nil {
		return goalItem("")(*need.Row)
	}
	return Item{ID: need.ID, Title: need.Title, At: need.Since, Where: Where{Kind: WhereGoal, ID: need.ID}}
}

// recordNeed is a draft or a landed design: the record's title, the one fact
// the inbox says about it, and the document it opens.
func recordNeed(need Need) Item {
	return Item{ID: need.ID, Title: need.Title, Note: need.By, Where: Where{Kind: WhereDocument, ID: need.Path}}
}

/* ---------------------------------------------------- since the last visit -- */

func changed(in Inputs) Changed {
	block := Changed{
		Concluded: concludedSince(in.Rows, in.Closed, in.Since),
		Moved:     movedSince(in.Rows, in.Since),
		Records:   recordsSince(in.Project.Records, in.Since),
		Messages:  messagesSince(in.Journal, in.Since),
	}
	block.Total = block.Concluded.Count + block.Moved.Count + block.Records.Count + block.Messages
	return block
}

// concludedSince is the goals whose own conclusion was written in the window.
// Concluded goals leave the live rows for the closed ones, so both lists are
// walked; a goal in neither has no conclusion to date.
func concludedSince(rows, closed []backlog.Row, since time.Time) Group {
	done := []backlog.Row{}
	for _, row := range append(append([]backlog.Row{}, rows...), closed...) {
		if at, dated := instant(row.DoneAt); dated && at.After(since) {
			done = append(done, row)
		}
	}
	// Newest first: the last thing that landed is the first thing to read.
	sort.SliceStable(done, func(i, j int) bool { return done[i].DoneAt > done[j].DoneAt })
	// The note is the goal's own conclusion where it wrote one. It is not the
	// word "concluded": the row stands under a count that already says so, and
	// a chip repeating its own heading is a chip that says nothing.
	return group(len(done), take(done, longList), func(row backlog.Row) Item {
		return Item{
			ID: row.ID, Title: lede(row.Intent), Note: row.Concluded, At: row.DoneAt,
			Where: Where{Kind: WhereGoal, ID: row.ID},
		}
	})
}

// movedSince is the live goals the ledger wrote to in the window and did not
// conclude, with the verb where the row can be trusted to carry one.
//
// The projection withholds a verb it cannot attribute — a priority compaction
// writes one operation's verb into every goal it re-ranks — and this row
// carries that withholding rather than guessing: the date is exact either way,
// and an unattributable verb is an empty note rather than a stranger's word.
func movedSince(rows []backlog.Row, since time.Time) Group {
	moved := []backlog.Row{}
	for _, row := range rows {
		if concluded(row) || row.DoneAt != "" {
			continue
		}
		if at, dated := instant(row.LastChangeAt); dated && at.After(since) {
			moved = append(moved, row)
		}
	}
	sort.SliceStable(moved, func(i, j int) bool { return moved[i].LastChangeAt > moved[j].LastChangeAt })
	return group(len(moved), take(moved, longList), func(row backlog.Row) Item {
		return Item{
			ID: row.ID, Title: lede(row.Intent), Note: row.LastVerb, At: row.LastChangeAt,
			Where: Where{Kind: WhereGoal, ID: row.ID},
		}
	})
}

// recordsSince is the records whose file was written in the window, newest
// first. A record the pane could not stamp carries no time and is not here:
// "changed at an instant nobody recorded" is not a change in a window.
func recordsSince(records []project.Record, since time.Time) Group {
	touched := []project.Record{}
	for _, record := range records {
		if at, dated := instant(record.ChangedAt); dated && at.After(since) {
			touched = append(touched, record)
		}
	}
	sort.SliceStable(touched, func(i, j int) bool { return touched[i].ChangedAt > touched[j].ChangedAt })
	items := []Item{}
	for _, record := range take(touched, longList) {
		item := recordItem(record, record.Kind)
		item.At = record.ChangedAt
		items = append(items, item)
	}
	return Group{Count: len(touched), Items: items}
}

func messagesSince(journal []notifications.Notice, since time.Time) int {
	count := 0
	for _, notice := range journal {
		if at, dated := instant(notice.At); dated && at.After(since) {
			count++
		}
	}
	return count
}

/* -------------------------------------------------------------- work now -- */

func workNow(in Inputs, now time.Time) Work {
	return Work{
		InProgress: inProgress(in.Rows, in.Holders, in.Stages),
		Next:       nextUp(in.Rows),
		Waiting:    waiting(in.Rows),
		Lanes:      lanes(in.Counts, in.Rows, in.Closed, now),
	}
}

// inProgress is the claimed work, in backlog order, with the seat that holds
// it. Every In Progress row carries a claim — that is what puts it in the lane
// — but a row whose claim the record somehow lacks is still shown, with the
// seat empty, rather than dropped from the one block that says what is being
// worked on.
//
// The phase is the host board's stage where the board believes a card for the
// goal: the board is where a seat records how far claimed work has got, and
// the ledger names a phase only once a landing begins. "not recorded" is left
// for a goal neither of them knows a phase for.
func inProgress(rows []backlog.Row, holders map[string]Holder, stages map[string]string) []Claimed {
	claimed := []Claimed{}
	for _, row := range rows {
		if row.Lane != backlog.LaneInProgress {
			continue
		}
		one := Claimed{ID: row.ID, Title: lede(row.Intent), Phase: row.Phase}
		if stage, carded := stages[row.ID]; carded {
			one.Phase = stage
		}
		if row.Claim != nil {
			one.Seat = Seat{Machine: row.Claim.Machine, Lineage: row.Claim.Lineage}
			one.At = row.Claim.At
			// The flag beside the seat, where the caller could read one. A
			// machine nothing knows about leaves the row exactly as it was.
			if held, known := holders[row.Claim.Machine]; known {
				kept := held
				one.Holder = &kept
			}
		}
		claimed = append(claimed, one)
	}
	return claimed
}

// nextUp is the first three of Ready for Work by rank, which is the order a
// seat claims from.
func nextUp(rows []backlog.Row) []Item {
	ready := []backlog.Row{}
	for _, row := range rows {
		if row.Lane == backlog.LaneReady {
			ready = append(ready, row)
		}
	}
	sortByRank(ready)
	items := []Item{}
	for _, row := range take(ready, shortList) {
		items = append(items, goalItem("ready")(row))
	}
	return items
}

// waiting is the held work and the oldest one's reason.
//
// Oldest is by the park's own stamp. A goal an open dependency holds carries
// no stamp — nothing recorded when the blocker became one — so it is never the
// oldest, and its reason, where it is the only row, is the blockers it names.
func waiting(rows []backlog.Row) Waiting {
	held := []backlog.Row{}
	for _, row := range rows {
		if row.Lane == backlog.LaneWaiting {
			held = append(held, row)
		}
	}
	block := Waiting{Count: len(held)}
	if len(held) == 0 {
		return block
	}
	sort.SliceStable(held, func(i, j int) bool {
		left, right := since(held[i]), since(held[j])
		if left == right {
			return held[i].ID < held[j].ID
		}
		// An undated hold is never the oldest: nothing recorded when it
		// began, so nothing here may claim it began first.
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		return left < right
	})
	oldest := held[0]
	block.ID, block.Title, block.Since = oldest.ID, lede(oldest.Intent), since(oldest)
	block.Reason = snapshot.Lede(reasonFor(oldest), ledeRunes)
	return block
}

func since(row backlog.Row) string {
	if row.Waiting == nil {
		return ""
	}
	return row.Waiting.Since
}

// reasonFor is why this goal is held, as the record says it: the park's own
// reason, or the blockers that are not concluded, or nothing at all.
func reasonFor(row backlog.Row) string {
	if row.Waiting != nil && row.Waiting.Reason != "" {
		return row.Waiting.Reason
	}
	if len(row.OpenBlockers) > 0 {
		return "blocked by " + strings.Join(row.OpenBlockers, ", ")
	}
	if row.Fence != nil && row.Fence.Reason != "" {
		return row.Fence.Reason
	}
	return ""
}

// lanes is the strip: the five live lanes the board shows, and Done today.
func lanes(counts map[backlog.Lane]int, rows, closed []backlog.Row, now time.Time) []Lane {
	strip := []Lane{}
	for _, lane := range []backlog.Lane{
		backlog.LaneToDo, backlog.LaneReady, backlog.LaneInProgress,
		backlog.LaneReview, backlog.LaneWaiting,
	} {
		strip = append(strip, Lane{ID: string(lane), Count: counts[lane]})
	}
	return append(strip, Lane{ID: LaneDoneToday, Count: doneToday(rows, closed, now)})
}

// doneToday is the goals concluded on the observation day.
//
// The day is the observing clock's own calendar day, not the last
// twenty-four hours: a human reading the page at nine in the morning means
// "since I got up", and a window of hours would put yesterday's evening in
// today's count. A conclusion nothing dated is in no day.
func doneToday(rows, closed []backlog.Row, now time.Time) int {
	count := 0
	for _, row := range append(append([]backlog.Row{}, rows...), closed...) {
		if at, dated := instant(row.DoneAt); dated && sameDay(at.In(now.Location()), now) {
			count++
		}
	}
	return count
}

func sameDay(at, now time.Time) bool {
	atYear, atMonth, atDay := at.Date()
	nowYear, nowMonth, nowDay := now.Date()
	return atYear == nowYear && atMonth == nowMonth && atDay == nowDay
}

/* ------------------------------------------------------------ the memory -- */

func memory(in Inputs) Memory {
	pane := in.Project
	open := questionScope(pane.Questions)
	return Memory{
		Intent:    book(pane.Intent),
		Doctrine:  book(pane.Doctrine),
		Decisions: tally(pane.Records, kindDecision),
		Designs:   designs(pane),
		Questions: open.Own + open.UnderGoals,
		Scoped: Scoped{
			Decisions: scopeOf(pane.Records, kindDecision),
			Designs:   scopeOf(pane.Records, kindDesign),
			Questions: open,
		},
	}
}

// scopeOf counts one kind of record by what it is about. A head with no Goals
// line is the project as a whole, which is the grammar's own rule and not a
// missing field.
func scopeOf(records []project.Record, kind string) Scope {
	counted := Scope{}
	for _, record := range records {
		if record.Kind != kind {
			continue
		}
		if len(record.Goals) == 0 {
			counted.Own++
			continue
		}
		counted.UnderGoals++
	}
	return counted
}

// questionScope counts the register's open rows the same way. Answered rows
// are not counted at all, because the tile beside this one is the open ones.
func questionScope(questions []project.Question) Scope {
	counted := Scope{}
	for _, question := range questions {
		if question.Status != "open" {
			continue
		}
		if len(question.Goals) == 0 {
			counted.Own++
			continue
		}
		counted.UnderGoals++
	}
	return counted
}

// book is a book's reading order as a number, and its index's own first
// sentence. A home with no index is no chapters and no sentence.
func book(read project.Book) Book {
	one := Book{Chapters: len(read.Chapters)}
	if read.Index != nil {
		one.Summary = FirstSentence(read.Index.Summary)
	}
	return one
}

func tally(records []project.Record, kind string) Tally {
	counted := Tally{}
	for _, record := range records {
		if record.Kind != kind {
			continue
		}
		counted.Total++
		if record.Status == statusDraft {
			counted.Drafts++
		}
	}
	return counted
}

// designs is the shelf: how many designs there are, how many are done, and how
// far each unfinished one that governs work has got.
func designs(pane project.Pane) Designs {
	states := goalStates(pane.Goals)
	shelf := Designs{Progress: []Progress{}}
	for _, record := range pane.Records {
		if record.Kind != kindDesign {
			continue
		}
		shelf.Total++
		if record.Status == statusDone {
			shelf.Done++
			continue
		}
		if len(record.Goals) == 0 {
			continue
		}
		shelf.InFlight++
		if len(shelf.Progress) < longList {
			shelf.Progress = append(shelf.Progress, Progress{
				ID: record.ID, Title: record.Title, Path: record.Path,
				Done: countDone(record.Goals, states), Goals: len(record.Goals),
			})
		}
	}
	return shelf
}

/* ------------------------------------------------------------- the health -- */

// health is one calm line, or every problem with the place it is fixed.
//
// The three questions are asked separately and every failing one is reported:
// a stale ledger does not excuse a refused record, and a human who fixed the
// record should not have to press Refresh to learn that the ledger is still
// behind.
func health(in Inputs, now time.Time) Health {
	problems := []Item{}
	if !in.Ledger.AtTip {
		problems = append(problems, Item{
			Title: ledgerProblem(in.Ledger), Note: "ledger",
			Where: Where{Kind: WhereBacklog},
		})
	}
	for _, refusal := range in.Project.Problems {
		problems = append(problems, Item{
			ID: refusal.Path, Title: refusal.Message, Note: anchor(refusal),
			Where: Where{Kind: WhereDocument, ID: refusal.Path},
		})
	}
	for _, notice := range undelivered(in.Journal, in.Since, now) {
		problems = append(problems, Item{
			ID: notice.ID, Title: notice.Error, Note: notice.Message, At: notice.At,
			Where: Where{Kind: WhereNotification, ID: notice.ID},
		})
	}
	return Health{
		OK:        len(problems) == 0,
		SyncedAt:  stamp(in.Ledger.SyncedAt),
		Freshness: in.Ledger.Freshness,
		Problems:  Group{Count: len(problems), Items: take(problems, longList)},
	}
}

// ledgerProblem is the engine's own words where it has some, and this page's
// where the statement says nothing: a reader is being told something is wrong
// and must be told what.
func ledgerProblem(ledger Ledger) string {
	if ledger.Statement != "" {
		return ledger.Statement
	}
	switch ledger.Freshness {
	case snapshot.FreshnessFailed:
		return "the last fetch of the canonical branch failed"
	case snapshot.FreshnessBehind:
		return "the last fetch of the canonical branch has not landed lately"
	}
	return "the accepted ledger could not be read"
}

func anchor(problem project.Problem) string {
	if problem.Line <= 0 {
		return problem.Path
	}
	return problem.Path + ":" + strconv.Itoa(problem.Line)
}

// undelivered is the steward's failed deliveries inside the window, which is
// the one thing on this page nobody else would ever see: the steward tried,
// the channel refused, and the message reached nobody.
func undelivered(journal []notifications.Notice, since, now time.Time) []notifications.Notice {
	failed := []notifications.Notice{}
	for _, notice := range journal {
		if notice.Delivered {
			continue
		}
		at, dated := instant(notice.At)
		if dated && at.After(since) && !at.After(now) {
			failed = append(failed, notice)
		}
	}
	return failed
}

/* --------------------------------------------------------------- shared -- */

// sortByRank is backlog order: the band first, then the position in it, then
// the id. A goal in no band is in no band's order, so it sorts after every
// ranked one rather than in front of band 1.
func sortByRank(rows []backlog.Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if left.Priority != right.Priority {
			if left.Priority == 0 || right.Priority == 0 {
				return right.Priority == 0
			}
			return left.Priority < right.Priority
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		return left.ID < right.ID
	})
}

// concluded reports whether the projection placed this goal out of the work.
func concluded(row backlog.Row) bool {
	return row.Lane == backlog.LaneDone || row.Lane == backlog.LaneAbandoned
}

func goalStates(goals []project.Goal) map[string]string {
	states := map[string]string{}
	for _, one := range goals {
		states[one.ID] = one.State
	}
	return states
}

func countDone(goals []string, states map[string]string) int {
	count := 0
	for _, id := range goals {
		if states[id] == stateDone {
			count++
		}
	}
	return count
}

func goalItem(note string) func(backlog.Row) Item {
	return func(row backlog.Row) Item {
		return Item{
			ID: row.ID, Title: lede(row.Intent), Note: note, At: row.OpenedAt,
			Where: Where{Kind: WhereGoal, ID: row.ID},
		}
	}
}

func recordItem(record project.Record, note string) Item {
	return Item{
		ID: record.ID, Title: record.Title, Note: note,
		Where: Where{Kind: WhereDocument, ID: record.Path},
	}
}

// group is a count with its capped list, built through one row shape.
func group(count int, rows []backlog.Row, shape func(backlog.Row) Item) Group {
	items := []Item{}
	for _, row := range rows {
		items = append(items, shape(row))
	}
	return Group{Count: count, Items: items}
}

// take is the first `most` of a list, and the whole of a shorter one.
func take[T any](values []T, most int) []T {
	if len(values) <= most {
		return append([]T{}, values...)
	}
	return append([]T{}, values[:most]...)
}

// FirstSentence is the opening sentence of a text: everything up to the first
// full stop, question mark or exclamation mark that ends one, or the whole of
// a text that ends without one. A stop inside a number, a path, a word or an
// abbreviation is not the end of a sentence, so the stop has to be followed by
// white space or by nothing.
func FirstSentence(text string) string {
	trimmed := strings.TrimSpace(text)
	for at, character := range trimmed {
		if character != '.' && character != '!' && character != '?' {
			continue
		}
		rest := trimmed[at+1:]
		if rest == "" {
			return trimmed
		}
		if next, _ := utf8.DecodeRuneInString(rest); unicode.IsSpace(next) {
			return trimmed[:at+1]
		}
	}
	return trimmed
}

// instant reads a recorded stamp, and reports whether anything recorded one. A
// value nothing wrote, and a value this build cannot parse, are both "not
// dated": neither belongs in a window of time.
func instant(recorded string) (time.Time, bool) {
	if recorded == "" {
		return time.Time{}, false
	}
	at, err := time.Parse(time.RFC3339, recorded)
	if err != nil {
		return time.Time{}, false
	}
	return at, true
}

// stamp writes an instant a reader can parse, and an empty string for an
// instant nothing recorded.
func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// ledeRunes is how much of a goal's intent a row shows. A ledger goal with no
// heading of its own is known by its intent, which here runs to paragraphs;
// a row is one line, so it carries the first sentence, and no more than this.
const ledeRunes = 140

// lede is what a row calls a goal: the title the ledger's reader makes of its
// intent, in no more than ledeRunes. The whole text is one click away on the
// goal's page.
func lede(intent string) string {
	return snapshot.GoalTitle(intent, ledeRunes)
}
