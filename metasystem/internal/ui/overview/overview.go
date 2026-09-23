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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
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

// The window an alert or a handoff is recent enough to still need a human.
const alertWindow = 7 * 24 * time.Hour

// The kinds a Where can name. Each one is a surface this build serves, and the
// browser decides what address, tab or panel that is.
const (
	// WhereGoal is one goal of the ledger, named by its id.
	WhereGoal = "goal"
	// WhereDocument is one document of the checkout, named by its path.
	WhereDocument = "document"
	// WhereQuestion is one row of the register, named by its id.
	WhereQuestion = "question"
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

// The journal sources that are addressed to a human rather than recorded at
// one: an alert is something wrong, and a handoff is a seat saying it is your
// turn. Everything else the steward writes is a log of its own work.
var forTheHuman = []string{"alert", "handoff"}

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
// the design leads with.
type NeedsYou struct {
	// Approvals are goals in To Do that carry no approval at all.
	Approvals Group `json:"approvals"`
	Questions Group `json:"questions"`
	// Drafts are records declaring draft status, whatever their kind.
	Drafts Group `json:"drafts"`
	// Designs are designs whose named goals have all landed and which are
	// not marked done.
	Designs Group `json:"designs"`
	// Alerts are the steward's alerts and handoffs from the last seven days.
	Alerts Group `json:"alerts"`
	// SignIn is true when nothing proves a human on this seat, which is a
	// row of its own: everything else here is something to read, and this is
	// the one thing that stops the page from being able to act.
	SignIn bool `json:"signIn"`
	// Total is the five groups' counts added up, and does not count SignIn:
	// a seat nobody is signed into still has nothing waiting on them, and
	// the two statements are different. The page is empty when Total is zero
	// and SignIn is false.
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

// Claimed is one In Progress goal, with the seat that holds it, the phase the
// projection could name, and when the claim was taken.
type Claimed struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Seat  Seat   `json:"seat"`
	Phase string `json:"phase"`
	At    string `json:"at"`
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

// Memory is what the project's records hold, as counts that open their tabs.
type Memory struct {
	Intent    Book    `json:"intent"`
	Doctrine  Book    `json:"doctrine"`
	Decisions Tally   `json:"decisions"`
	Designs   Designs `json:"designs"`
	Questions int     `json:"questions"`
}

// Health is one calm line, or the problems.
type Health struct {
	OK bool `json:"ok"`
	// SyncedAt is when the last fetch finished, in RFC3339, which is what the
	// calm line names. The page writes the clock time in the reader's own
	// locale, so the instant travels rather than the words.
	SyncedAt string `json:"syncedAt"`
	Problems Group  `json:"problems"`
}

// Ledger is what the backlog's own statement says about the accepted ledger,
// reduced to the three facts this page judges health on. It is a value rather
// than the observation itself so that Compose stays a function over data and
// the route is the one place that reads a running engine.
type Ledger struct {
	// AtTip is the statement's own answer: the tree was read, and it is not
	// older than the staleness threshold.
	AtTip bool
	// Fetched is whether the last fetch of the canonical branch succeeded.
	Fetched bool
	// SyncedAt is when that fetch finished.
	SyncedAt time.Time
	// Statement is the engine's own words for what is wrong, where the two
	// above say something is.
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
	// Journal is the newest page of the steward's notification journal,
	// newest first, as the notifications package serves it.
	Journal []notifications.Notice
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
		NeedsYou:      needsYou(in, now),
		Changed:       changed(in),
		Work:          workNow(in, now),
		Memory:        memory(in),
		Health:        health(in, now),
	}
}

/* ------------------------------------------------------------- needs you -- */

func needsYou(in Inputs, now time.Time) NeedsYou {
	block := NeedsYou{
		Approvals: awaitingApproval(in.Rows),
		Questions: openQuestions(in.Project.Questions),
		Drafts:    draftRecords(in.Project.Records),
		Designs:   landedDesigns(in.Project),
		Alerts:    recentAlerts(in.Journal, now),
		SignIn:    !in.Human.Proven,
	}
	block.Total = block.Approvals.Count + block.Questions.Count +
		block.Drafts.Count + block.Designs.Count + block.Alerts.Count
	return block
}

// awaitingApproval is the goals in To Do that carry no approval at all, in
// backlog order: the band first, then the position in it.
//
// A goal whose approval expired is in To Do too, and is not here: it carries
// an approval, the projection says so in its gap, and asking a human to admit
// work they already admitted is a different request from asking them to admit
// work nobody has. What this row means is "nobody has said yes to this yet".
func awaitingApproval(rows []backlog.Row) Group {
	waiting := []backlog.Row{}
	for _, row := range rows {
		if row.Lane == backlog.LaneToDo && row.Approved == nil {
			waiting = append(waiting, row)
		}
	}
	sortByRank(waiting)
	return group(len(waiting), take(waiting, shortList), goalItem(""))
}

// openQuestions is the register's open rows, newest first.
func openQuestions(questions []project.Question) Group {
	open := []project.Question{}
	for _, question := range questions {
		if question.Status == "open" {
			open = append(open, question)
		}
	}
	// Newest first, and by id where two were opened at the same instant, so
	// the order is the same on every read of the same register.
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].Opened != open[j].Opened {
			return open[i].Opened > open[j].Opened
		}
		return open[i].ID < open[j].ID
	})
	items := []Item{}
	for _, question := range take(open, shortList) {
		items = append(items, Item{
			ID: question.ID, Title: question.Question, Note: "open", At: question.Opened,
			Where: Where{Kind: WhereQuestion, ID: question.ID},
		})
	}
	return Group{Count: len(open), Items: items}
}

// draftRecords is every record declaring draft status, whatever its kind: a
// draft is a record somebody wrote and nobody has accepted, and the kind is
// the note beside it rather than a filter on it.
func draftRecords(records []project.Record) Group {
	drafts := []project.Record{}
	for _, record := range records {
		if record.Status == statusDraft {
			drafts = append(drafts, record)
		}
	}
	items := []Item{}
	for _, record := range take(drafts, shortList) {
		items = append(items, recordItem(record, record.Kind))
	}
	return Group{Count: len(drafts), Items: items}
}

// landedDesigns is the designs whose work is all in and which nobody has
// marked done.
//
// A design naming no goal is not one of them. "Every goal has landed" over an
// empty list is true and means nothing: the design governs no work, so there
// is no work to have landed, and offering it as something to close would be
// asking a human to conclude a design that never started.
func landedDesigns(pane project.Pane) Group {
	states := goalStates(pane.Goals)
	landed := []project.Record{}
	for _, record := range pane.Records {
		if record.Kind != kindDesign || record.Status == statusDone || len(record.Goals) == 0 {
			continue
		}
		if allDone(record.Goals, states) {
			landed = append(landed, record)
		}
	}
	items := []Item{}
	for _, record := range take(landed, shortList) {
		items = append(items, recordItem(record, "every goal landed"))
	}
	return Group{Count: len(landed), Items: items}
}

// recentAlerts is the steward's alerts and handoffs from the last seven days,
// newest first. The journal is served newest first, so the order is the
// journal's own and nothing is sorted again.
func recentAlerts(journal []notifications.Notice, now time.Time) Group {
	from := now.Add(-alertWindow)
	recent := []notifications.Notice{}
	for _, notice := range journal {
		if !addressesTheHuman(notice) {
			continue
		}
		if at, dated := instant(notice.At); dated && at.After(from) {
			recent = append(recent, notice)
		}
	}
	items := []Item{}
	for _, notice := range take(recent, shortList) {
		items = append(items, Item{
			ID: notice.ID, Title: notice.Message, Note: notice.Source, At: notice.At,
			Where: Where{Kind: WhereNotification, ID: notice.ID},
		})
	}
	return Group{Count: len(recent), Items: items}
}

func addressesTheHuman(notice notifications.Notice) bool {
	for _, source := range forTheHuman {
		if notice.Source == source {
			return true
		}
	}
	return false
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
		InProgress: inProgress(in.Rows),
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
func inProgress(rows []backlog.Row) []Claimed {
	claimed := []Claimed{}
	for _, row := range rows {
		if row.Lane != backlog.LaneInProgress {
			continue
		}
		one := Claimed{ID: row.ID, Title: lede(row.Intent), Phase: row.Phase}
		if row.Claim != nil {
			one.Seat = Seat{Machine: row.Claim.Machine, Lineage: row.Claim.Lineage}
			one.At = row.Claim.At
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
	block.Reason = lede(reasonFor(oldest))
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
	return Memory{
		Intent:    book(pane.Intent),
		Doctrine:  book(pane.Doctrine),
		Decisions: tally(pane.Records, kindDecision),
		Designs:   designs(pane),
		Questions: openQuestions(pane.Questions).Count,
	}
}

// book is a book's reading order as a number, and its index's own first
// sentence. A home with no index is no chapters and no sentence.
func book(read project.Book) Book {
	one := Book{Chapters: len(read.Chapters)}
	if read.Index != nil {
		one.Summary = firstSentence(read.Index.Summary)
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
	if !in.Ledger.AtTip || !in.Ledger.Fetched {
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
		OK:       len(problems) == 0,
		SyncedAt: stamp(in.Ledger.SyncedAt),
		Problems: Group{Count: len(problems), Items: take(problems, longList)},
	}
}

// ledgerProblem is the engine's own words where it has some, and this page's
// where the statement says nothing: a reader is being told something is wrong
// and must be told what.
func ledgerProblem(ledger Ledger) string {
	if ledger.Statement != "" {
		return ledger.Statement
	}
	if !ledger.Fetched {
		return "the last fetch of the canonical branch did not succeed"
	}
	return "the accepted ledger is not at the canonical tip"
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

// allDone is true when every named goal is one the pane carries and concluded.
// A goal the pane does not carry is not a goal that landed: the design names
// something this checkout cannot see, and calling that "done" would close a
// design on a goal nobody has read.
func allDone(goals []string, states map[string]string) bool {
	for _, id := range goals {
		if states[id] != stateDone {
			return false
		}
	}
	return true
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

// firstSentence is the opening sentence of a summary: everything up to the
// first full stop that ends one, or the whole of a summary that ends without
// one. A full stop inside a number or an abbreviation is not the end of a
// sentence, so the stop has to be followed by a space or by nothing.
func firstSentence(summary string) string {
	trimmed := strings.TrimSpace(summary)
	for at, character := range trimmed {
		if character != '.' && character != '!' && character != '?' {
			continue
		}
		rest := trimmed[at+1:]
		if rest == "" {
			return trimmed
		}
		if strings.HasPrefix(rest, " ") || strings.HasPrefix(rest, "\n") {
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

// lede is the first sentence of a text, cut at a sentence end and then at the
// last word boundary before ledeRunes, with an ellipsis where it was cut. The
// whole text is one click away on the goal's page.
func lede(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if at := strings.Index(text, ". "); at >= 0 {
		text = text[:at+1]
	}
	runes := []rune(text)
	if len(runes) <= ledeRunes {
		return text
	}
	cut := ledeRunes
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut == 0 {
		cut = ledeRunes
	}
	return strings.TrimRight(string(runes[:cut]), " ,;:") + "…"
}
