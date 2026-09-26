// Package decisions composes the page a human rules from.
//
// It answers two questions and nothing else: what needs your choice, and what
// you decided. The first is one list — every kind of thing that is waiting on
// a human, complete rather than capped — and the second is the record of what
// this human has already said, in their own words where the register keeps
// them.
//
// It reads nothing. Everything it answers from is read by somebody else: the
// project's records, the backlog projection, the steward's journal, this
// seat's open channel questions, the rulings register, and whether a human is
// proven on this seat. Compose is a pure function over those six, so every
// rule below is a rule a test states rather than a shape a request happens to
// produce.
//
// Two rules matter more than the rest, because they are the two a human acts
// on. The first: every inbox row says what happens if the human does nothing,
// and that sentence is a claim about the engine. Each one below names the verb
// or the projection that makes it true, and a kind whose record has no
// recorded consequence says exactly that rather than inventing one. The
// second: nothing here judges a ruling. The register's words are the record,
// an event condition is shown and never evaluated, and a defective row is
// named in the steward's own words rather than repaired.
package decisions

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/rulings"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
)

// SchemaVersion is the shape of the decisions resource a reader parses.
//
// Three, since the inbox became an inbox: every row says whether it is new
// since the last visit here, the page names the window it decided that over,
// and the three kinds whose row used to need a second read now carry what
// they need — a ruling review the register row's own words, a draft and a
// landed design the record's path, and a landed design the goals it named
// with where each stands. Additions only: nothing was renamed and nothing was
// removed, so a reader of the second shape reads every field it read before.
const SchemaVersion = 3

// The kinds of thing that wait on a human. Each one is a row of the design's
// own table, and each one carries its own silence line.
const (
	KindApproval     = "approval"
	KindRenewal      = "renewal"
	KindAsk          = "ask"
	KindQuestion     = "question"
	KindParked       = "parked"
	KindStopped      = "stopped"
	KindDraft        = "draft"
	KindLanded       = "landed"
	KindRulingReview = "ruling-review"
	KindAlert        = "alert"
)

// The acts this page offers. Two are the board's own; two are this page's,
// admitted from a browser under R-125-m1u and offered nowhere else.
const (
	ActApprove  = "approve"
	ActWithdraw = "withdraw"
	// ActPark is "Not now": a pause with the reason the human types.
	ActPark = "park"
	// ActUnpark returns a paused goal to the queue.
	ActUnpark = "unpark"
)

// The kinds a Where can name. A destination is said as what kind of thing it
// is, never as an address: the browser owns addresses, and a server that
// spelled one would be a second router.
const (
	WhereGoal     = "goal"
	WhereRecord   = "record"
	WhereQuestion = "question"
	// WhereRegister is the rulings register, opened in the reader.
	WhereRegister = "register"
	// WhereNotifications is the steward's journal, opened at one line.
	WhereNotifications = "notifications"
	// WhereChannel is the fleet channel a seat's question is answered on. It
	// is not a page of this interface and never will be: the answer carries
	// the human's own code, which reaches the channel and nothing else.
	WhereChannel = "channel"
)

// The window an alert is recent enough to still need a human, which is the
// Overview's own window over the same journal.
const alertWindow = 7 * 24 * time.Hour

// The register path the reader opens, where the caller names none.
//
// It is the kit's own layout — the register beside the installation's memory
// — and it is right only where the checkout and the installation are the same
// directory. Every caller that knows both roots hands the real one in through
// Inputs.RegisterPath, because the reader that opens it opens it relative to
// the CHECKOUT, and a path that named the installation's copy would open
// nothing on the layout this interface most often serves.
const registerPath = "memory/rulings.md"

// humanPark is the engine's own test of a park a person made: the park's By
// is "human:<name>" for any actor with a human behind it (verbs.go:203-208),
// and the unpark verb tells a person's park from a seat's by this same prefix
// (verbs.go:2772). A row it answers true for is a decision this human already
// made, so it is not something waiting on them.
const humanPark = "human:"

// The journal sources that are addressed to a human rather than recorded at
// one. It is the Overview's own list, because it is the same journal.
var forTheHuman = []string{"alert", "handoff"}

// The record kinds and statuses this page tells apart, which are the
// resolver's own words.
const (
	kindDecision = "decision"
	kindDesign   = "design"
	statusDraft  = "draft"
	statusDone   = "done"
	statusOpen   = "open"
	// answeredPrefix is how the questions register names what answered a
	// row: the status column carries "answered: <ref>".
	answeredPrefix = "answered:"
)

// Where is one destination, said as what kind of thing it is and which one.
type Where struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

// Need is one thing that is waiting on a human.
//
// No row is a bare label. What is asked, who asks, since when, what happens
// if the human does nothing, and the asker's recommendation where the record
// carries one — and then either the act this interface has, the place the
// decision is made, or the command that makes it.
//
// Every field is written, including the ones a particular row has nothing
// for, so a reader never has to tell an absent field from an empty one.
type Need struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Title string `json:"title"`
	// Asked is what is being asked, in one sentence, from the record.
	Asked string `json:"asked"`
	By    string `json:"by"`
	// Since is when the asking began, in RFC3339, or "" where nothing dated it.
	Since string `json:"since"`
	// Deadline is the instant the record says this must be answered by, in
	// RFC3339. No record this build reads carries one: a channel question has
	// no deadline field, and the dates the other kinds carry are dates
	// already passed rather than dates to come. It is written because the
	// order below leads with it, and a reader that saw the field appear one
	// day would be reading a new shape rather than a filled one.
	Deadline string `json:"deadline"`
	// Silence is what the machinery does if this human does nothing. It is a
	// claim about the engine and never an encouragement.
	Silence string `json:"silence"`
	// Recommend is the asker's own recommendation, where the record carries
	// one. Only a seat's channel question does.
	Recommend string `json:"recommend"`
	Where     Where  `json:"where"`
	// Act is the interface's own act for this row: approve, withdraw, or
	// nothing at all.
	Act string `json:"act"`
	// Command is the terminal command that makes this decision, where the
	// decision is made at a terminal.
	Command string `json:"command"`
	// Row is the whole backlog row, carried for the rows whose act is the
	// board's sheet, which prefills from it.
	Row *backlog.Row `json:"row"`
	// New is true when this row was recorded after the start of the window
	// this page was composed over, which is the end of the reader's previous
	// visit HERE. It is by recorded dates and is honest about them: the dates
	// the kinds carry are uneven — an instant on a goal, a calendar date on a
	// question, a file time on a landed design, and sometimes nothing at all —
	// so an instant is compared as an instant, a calendar date counts as new
	// from the window's own day, and a row nothing dated is never new. See
	// newSince.
	New bool `json:"new"`
	// sinceIsDay says Since was made from a calendar date rather than read as
	// an instant somebody recorded: a ruling review's due date and an
	// approval's review-by date are days, and dayStamp writes them as
	// midnight so that they sort beside the instants every other row carries.
	// Midnight is not when that day began for this human, so the newness test
	// reads them back as the days they were. It is not part of the payload —
	// the page reads Since as the stamp it is — and nothing but newSince
	// looks at it.
	sinceIsDay bool
	// Words, Context, Owner, Class and Due are the register row a ruling
	// review names, carried on the row rather than joined by the reader: the
	// review's own record is the id, the owner and the schedule, and what the
	// human actually ruled is in the register beside it under the same id.
	// Every other kind writes them empty.
	Words   string `json:"words"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	Class   string `json:"class"`
	Due     string `json:"due"`
	// Path is where the record this row is about lives, relative to the
	// checkout, for the two kinds that are about a record. Where names the
	// same file as a destination; this is the line the open row shows, and a
	// reader that had to take it out of a destination would be reading an
	// address the server said was not one.
	Path string `json:"path"`
	// Goals are the ledger goals a landed design named, each with where it
	// stands, which is the evidence for marking it done. Written as an empty
	// list on every other kind rather than as nothing.
	Goals []GoalState `json:"goals"`
}

// GoalState is one goal a record names and where the ledger says it stands.
type GoalState struct {
	ID string `json:"id"`
	// State is the projection's own word for the goal, or "" where this
	// checkout carries no goal under that id at all.
	State string `json:"state"`
}

// Visit is the window a page's "new" was decided against: the end of this
// human's previous visit to THIS page, or a day back on a first one.
//
// It is carried so that the page can say what it means by new rather than
// leaving a reader to infer a boundary from the dots.
type Visit struct {
	// Since is the start of the window in RFC3339, or "" where the page was
	// composed over no window at all — in which case nothing is new.
	Since string `json:"since"`
	// First says this is the first visit this file has recorded, whose window
	// is a day back rather than a previous visit.
	First bool `json:"first"`
}

// Ruling is one row of the register as the page renders it: whole, with the
// review condition read but never judged beyond its date.
type Ruling struct {
	ID      string `json:"id"`
	Date    string `json:"date"`
	Words   string `json:"words"`
	Context string `json:"context"`
	Owner   string `json:"owner"`
	// Class, Due and Event are the scheduled parts, where the condition
	// parses as a scheduled review. Condition is what the register wrote,
	// whether it parsed or not.
	Class     string `json:"class"`
	Due       string `json:"due"`
	Event     string `json:"event"`
	Condition string `json:"condition"`
	// DuePassed is a valid due date at or before the observing day. An event
	// condition never sets it: what an event means is the steward's own
	// evaluation, with observed and unobservable outcomes, and this page
	// shows the event and judges nothing.
	DuePassed bool `json:"duePassed"`
	// Mentions are the ledger goals this ruling's words name verbatim. A
	// mention is a link and never a claimed subject: a ruling is about what
	// it says, and naming a goal is not being filed under it.
	Mentions []string `json:"mentions"`
}

// Item is one decided thing that is not a ruling: a decision record, an
// answered question, or anything else a tab lists. It is the Overview's own
// row shape, because these tabs are the Overview's own rows.
type Item struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Note is the one fact beside the title: a status, or what answered a
	// question.
	Note  string `json:"note"`
	At    string `json:"at"`
	Where Where  `json:"where"`
}

// Approved is one approval on record, with the whole row it was recorded on
// so that Withdraw can open from it where the board's own eligibility allows.
type Approved struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	By        string `json:"by"`
	At        string `json:"at"`
	Authority string `json:"authority"`
	Expired   bool   `json:"expired"`
	Row       Row    `json:"row"`
}

// Row is the backlog row as the payload carries it. It is an alias rather
// than a copy so that the sheet the board already has reads the same fields
// it reads on the board.
type Row = backlog.Row

// Decided is what this human has already said, four ways.
// NotNow is one goal a person paused, with the whole of what they said: the
// reason, who paused it, when, and the blocker where the park names one.
//
// It is a decided thing rather than a waiting one. The inbox used to count it
// as "needs your choice", which said that a human who wrote down a reason and
// a date had not decided anything.
type NotNow struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	By    string `json:"by"`
	At    string `json:"at"`
	// Because is the reason the park recorded, which is the whole of why.
	Because string `json:"because"`
	// Blocker is the goal this park waits for, where a human directed the
	// park at one. A blocker park returns by itself when its blockers finish,
	// which is why the row names what it is waiting for.
	Blocker string `json:"blocker"`
	Where   Where  `json:"where"`
}

type Decided struct {
	Rulings []Ruling `json:"rulings"`
	// Defects are the register's own broken rows, in the steward's words,
	// listed rather than hidden.
	Defects   []string   `json:"defects"`
	Decisions []Item     `json:"decisions"`
	Answered  []Item     `json:"answered"`
	Approved  []Approved `json:"approved"`
	// NotNow is every park a person made, newest first.
	NotNow []NotNow `json:"notNow"`
}

// Counts is what the page shows as figures: the whole inbox, its two blocks,
// and the whole register.
//
// NeedsYou is the inbox as listed, and Asked and Waiting are its two halves —
// everything that is not an approval, and the approvals. They always sum to
// NeedsYou, because the page splits the one list the server composed rather
// than composing two.
type Counts struct {
	NeedsYou int `json:"needsYou"`
	Asked    int `json:"asked"`
	Waiting  int `json:"waiting"`
	Rulings  int `json:"rulings"`
}

// Page is the whole of Decisions, composed once, as it was at readAt.
type Page struct {
	SchemaVersion int    `json:"schemaVersion"`
	ReadAt        string `json:"readAt"`
	// SignIn is true when nothing proves a human on this seat, which is a row
	// of its own at the top: everything else here is something to decide, and
	// this is the one thing that stops the page from being able to.
	SignIn   bool    `json:"signIn"`
	NeedsYou []Need  `json:"needsYou"`
	Decided  Decided `json:"decided"`
	Counts   Counts  `json:"counts"`
	// Visit is the window every row's New was decided against, so the page can
	// say what it means by new rather than leaving it to be inferred.
	Visit Visit `json:"visit"`
	// Register is where the rulings register is, relative to the checkout,
	// which is what the document reader opens paths against. Every register
	// destination in this payload names it, so "open the register" opens.
	Register string `json:"register"`
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
	// Journal is the newest page of the steward's notification journal,
	// newest first, as the notifications package serves it.
	Journal []notifications.Notice
	// Asks are this seat's open channel questions. They are this seat's own
	// files: a fleet where another seat holds a question shows only this
	// seat's, and the row says where the rest of them reach a human.
	Asks []channel.Question
	// Register is one read of the rulings register, whole rows and all.
	Register rulings.Register
	// RegisterPath is where that register is RELATIVE TO THE CHECKOUT, which
	// is not where it was read from: the reader takes the installation root,
	// because that is where the kit keeps its memory, and the document reader
	// opens against the checkout. The caller knows both roots and derives the
	// one path both ends can use. An empty path takes the kit's own layout.
	RegisterPath string
	Human        Standing
	// Since is the start of the window "new" is decided against: the end of
	// this human's previous visit to THIS page, which the caller takes from
	// the visit owner under this page's own entry. A zero instant is a page
	// composed over no window, and nothing on it is new — which is what a
	// build with no marker store answers, rather than a page on which
	// everything ever recorded is new.
	Since time.Time
	// First says the window is a first visit's day rather than a previous
	// visit's end. It is carried through to the payload so the page can say so.
	First bool
}

// Compose is the whole page, from the six answers above, as they stood at
// now. It reads no file, opens no connection and keeps nothing.
func Compose(in Inputs, now time.Time) Page {
	inbox := needsYou(in, now)
	waiting := 0
	for index := range inbox {
		inbox[index].New = newSince(inbox[index].Since, inbox[index].sinceIsDay, in.Since)
		// Every field is written, including the ones a particular row has
		// nothing for: a reader never has to tell an absent list from an
		// empty one.
		if inbox[index].Goals == nil {
			inbox[index].Goals = []GoalState{}
		}
		if inbox[index].Kind == KindApproval {
			waiting++
		}
	}
	return Page{
		SchemaVersion: SchemaVersion,
		ReadAt:        stamp(now),
		SignIn:        !in.Human.Proven,
		NeedsYou:      inbox,
		Decided:       decided(in, now),
		Counts: Counts{
			NeedsYou: len(inbox), Asked: len(inbox) - waiting, Waiting: waiting,
			Rulings: len(in.Register.Rows),
		},
		Visit:    Visit{Since: stamp(in.Since), First: in.First},
		Register: registerOf(in),
	}
}

// newSince is whether a row recorded at since is new against a window that
// began at from.
//
// It is by recorded dates and says so, because the dates are uneven and no
// amount of arithmetic makes them even. A goal carries the instant it was
// opened, and an instant is compared as an instant. A question carries a
// calendar date, which is a day in nobody's particular zone, so it is new when
// it is the window's own day or later — the alternative is reading a day as
// midnight UTC and telling a human that this morning's question is old. A row
// nothing dated is never new: the record does not say when it began, and a
// page that guessed would be marking rows new on no evidence. And a window
// nothing set makes nothing new, for the same reason.
//
// Two of the calendar dates arrive here already written as an instant, because
// they sort beside the instants: byDay says the day is still what was
// recorded, and it is compared as one. Without it a ruling due today reads as
// midnight and a window that opened this morning calls it old.
func newSince(since string, byDay bool, from time.Time) bool {
	if since == "" || from.IsZero() {
		return false
	}
	if at, dated := instant(since); dated {
		if byDay {
			return !dayOf(at).Before(dayOf(from))
		}
		return at.After(from)
	}
	if day, err := time.Parse(rulings.DateLayout, since); err == nil {
		return !day.UTC().Before(dayOf(from))
	}
	return false
}

// dayOf is the calendar day a window began on, at its start, in UTC.
func dayOf(at time.Time) time.Time {
	utc := at.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}

// registerOf is where the register is from the checkout, which is what every
// destination naming it must say.
func registerOf(in Inputs) string {
	if in.RegisterPath != "" {
		return in.RegisterPath
	}
	return registerPath
}

/* ------------------------------------------------------ needs your choice -- */

// needsYou is the whole inbox, in the order the design reads it. It is a
// complete list and never a capped group: a human deciding what to do next
// needs all of it, and "and 4 more" is what a summary says.
func needsYou(in Inputs, now time.Time) []Need {
	needs := []Need{}
	needs = append(needs, approvals(in.Rows)...)
	needs = append(needs, renewals(in.Rows)...)
	needs = append(needs, asks(in.Asks)...)
	needs = append(needs, questions(in.Project.Questions)...)
	needs = append(needs, parked(in.Rows)...)
	needs = append(needs, stopped(in.Rows)...)
	needs = append(needs, drafts(in.Project.Records)...)
	needs = append(needs, landed(in.Project)...)
	needs = append(needs, rulingReviews(in.Register, registerOf(in), now)...)
	needs = append(needs, alerts(in.Journal, now)...)
	order(needs)
	return needs
}

// approvals is the goals in To Do that carry no approval at all, in backlog
// order: the band first, then the position in it.
//
// A goal whose approval expired is in To Do too and is not here: it carries
// an approval, and asking a human to admit work they already admitted is a
// different request from asking them to admit work nobody has. That row is a
// renewal below.
//
// Silence: internal/goal/approval.go's requireApprovedForClaim is the single
// admission gate for every path that creates a claimed revision, and it
// refuses a goal with no approval record with APPROVAL_REQUIRED. So the goal
// stays in To Do — the projection places an unapproved queued goal there —
// and no seat may claim it.
func approvals(rows []backlog.Row) []Need {
	waiting := []backlog.Row{}
	for _, row := range rows {
		if row.Lane == backlog.LaneToDo && row.Approved == nil {
			waiting = append(waiting, row)
		}
	}
	sortByRank(waiting)
	needs := []Need{}
	for index := range waiting {
		row := waiting[index]
		needs = append(needs, Need{
			Kind: KindApproval, ID: row.ID, Title: titleOf(row),
			Asked: "Approve " + titleOf(row) + " for execution",
			By:    "the backlog", Since: row.OpenedAt,
			Silence: "it stays in To Do and no seat may claim it",
			Where:   Where{Kind: WhereGoal, ID: row.ID},
			Act:     ActApprove, Row: &row,
		})
	}
	return needs
}

// renewals is the rows whose approval the projection judged expired.
//
// They are two rows, not one, because the engine treats them differently and
// so must this page. An unclaimed row is offered the sheet: approve re-admits
// it, and the engine takes the budget. A claimed row is not: approval.go
// refuses a budget on claimed work ("its tuple changes through goal
// set-budget"), so a sheet that prefilled one would be a form the engine
// throws away.
//
// Silence, unclaimed: requireApprovedForClaim refuses an expired approval
// with APPROVAL_EXPIRED — "that approval no longer admits new work" — so no
// fresh claim is admitted. Silence, claimed: that gate is the gate for
// CREATING a claimed revision, and dispatch under an existing claim checks
// the claim, the fence and the budget rather than the approval's expiry, so
// work already claimed continues.
func renewals(rows []backlog.Row) []Need {
	needs := []Need{}
	expired := []backlog.Row{}
	for _, row := range rows {
		if row.Approved != nil && row.Approved.Expired {
			expired = append(expired, row)
		}
	}
	sortByRank(expired)
	for index := range expired {
		row := expired[index]
		since, fromDay := expiredAt(*row.Approved)
		need := Need{
			Kind: KindRenewal, ID: row.ID, Title: titleOf(row),
			Asked: "Renew the approval of " + titleOf(row) + ": " + row.Approved.ExpiredWhy,
			By:    row.Approved.By, Since: since, sinceIsDay: fromDay,
			Where: Where{Kind: WhereGoal, ID: row.ID},
		}
		if row.Claim == nil {
			need.Silence = "no fresh claim is admitted"
			need.Act = ActApprove
			need.Row = &row
		} else {
			need.Silence = "work already claimed continues; renew at the goal"
		}
		needs = append(needs, need)
	}
	return needs
}

// expiredAt is when this approval stopped admitting new work, which is what a
// renewal has been waiting on a human since.
//
// It is the review date the approval names, where it names one and the date
// reads: that is the instant the gate began refusing. An approval that
// expired for one of the other reasons the horizon carries — a terminal
// enrolled, the standing authority horizon passing — names no date of its
// own here, so the row is dated from the approval itself rather than from an
// instant this page would have to invent.
//
// The second result says which of the two it answered with: a review date is
// a day written as midnight, and the newness test has to know that before it
// compares it to a window that opened after midnight.
func expiredAt(approval backlog.Approval) (string, bool) {
	if stamped := dayStamp(approval.ReviewBy); stamped != "" {
		return stamped, true
	}
	return approval.At, false
}

// asks is this seat's open channel questions, shown as recorded.
//
// The record has no question text field: a seat states its facts, what it
// wants, the options with their consequences, and its recommendation, and
// those are what the row shows. It has no deadline either, and no safe stop
// the engine takes when nobody answers, so the silence line says what is
// true — nothing is recorded about what happens next — rather than inventing
// a timeout the machinery does not have.
func asks(open []channel.Question) []Need {
	needs := []Need{}
	for _, question := range open {
		needs = append(needs, Need{
			Kind: KindAsk, ID: question.ID, Title: askTitle(question),
			Asked: askedOf(question), By: askedBy(question),
			Since:     stamp(question.OpenedAt),
			Silence:   "no recorded consequence",
			Recommend: question.Recommendation,
			Where:     Where{Kind: WhereChannel, ID: question.ID},
		})
	}
	return needs
}

// askTitle is what the row is called: the goal it is about, with the kind of
// question it is, or the question's own id where it names no goal.
func askTitle(question channel.Question) string {
	if question.Goal == "" {
		return question.ID
	}
	if question.Kind == "" {
		return question.Goal
	}
	return question.Goal + " · " + question.Kind
}

func askedBy(question channel.Question) string {
	if question.Machine == "" {
		return "a seat"
	}
	return "seat " + question.Machine
}

// askedOf is the record's own content, in the order the seat-communication
// law names it: what is wanted, the facts it is wanted on, each option with
// its consequence, and the proposed budget where the question carries one.
// Nothing is summarised and nothing is added.
func askedOf(question channel.Question) string {
	parts := []string{}
	if wants := strings.TrimSpace(question.Wants); wants != "" {
		parts = append(parts, wants)
	}
	for _, fact := range question.Facts {
		if trimmed := strings.TrimSpace(fact); trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	for _, option := range question.Options {
		label := strings.TrimSpace(option.Label)
		consequence := strings.TrimSpace(option.Consequence)
		switch {
		case label == "" && consequence == "":
		case consequence == "":
			parts = append(parts, label)
		default:
			parts = append(parts, label+": "+consequence)
		}
	}
	if budget := question.Budget; budget != nil {
		// The compact box form the engine itself parses, so a human reading
		// the row reads the tuple they would type.
		parts = append(parts, "proposed budget "+budget.ElapsedLimit+
			"/"+strconv.FormatUint(budget.AttemptLimit, 10)+
			"/"+strconv.FormatUint(budget.ReservedJobMinutesLimit, 10)+
			"/"+strconv.FormatUint(budget.ActiveJobLimit, 10)+
			"/"+strconv.FormatInt(budget.ReviewRoundLimit, 10))
	}
	return strings.Join(parts, " · ")
}

// questions is the register's open rows, newest first.
//
// Silence: nothing in the engine answers a register question. The status
// column is written by a human, through the reader's own act or by hand, so a
// row nobody answers stays open.
func questions(register []project.Question) []Need {
	open := []project.Question{}
	for _, question := range register {
		if question.Status == statusOpen {
			open = append(open, question)
		}
	}
	sort.SliceStable(open, func(i, j int) bool {
		if open[i].Opened != open[j].Opened {
			return open[i].Opened > open[j].Opened
		}
		return open[i].ID < open[j].ID
	})
	needs := []Need{}
	for _, question := range open {
		needs = append(needs, Need{
			Kind: KindQuestion, ID: question.ID, Title: question.Question,
			Asked: question.Question, By: "the register", Since: question.Opened,
			Silence: "it stays open",
			Where:   Where{Kind: WhereQuestion, ID: question.ID},
		})
	}
	return needs
}

// parked is the goals a SEAT parked: the state is parked, the park names no
// blocker, and the park is not a person's.
//
// The blocker is what tells a seat's pause from a dependency's. An approved
// goal an open dependency holds is in the Waiting lane too and its Waiting
// carries no blocker either (project.go's own else branch), but its state is
// approved rather than parked — it is waiting on work, not on a human. A
// dependency park carries the blocker it waits for, and returns by itself
// when that blocker lands.
//
// The prefix is what tells a seat's pause from a person's. A park a person
// made is a decision they already took, with their reason on it, so it is in
// Not now rather than here; a seat's park stays, because a human has not seen
// it. The row carries the act that returns it either way.
//
// Silence: goal.Store.Unpark is the only verb that lifts a park with no
// blocker, and nothing runs it on its own, so it stays parked.
func parked(rows []backlog.Row) []Need {
	needs := []Need{}
	for _, row := range rows {
		if row.State != goal.StateParked || row.Waiting == nil || row.Waiting.Blocker != "" {
			continue
		}
		if strings.HasPrefix(row.Waiting.By, humanPark) {
			continue
		}
		needs = append(needs, Need{
			Kind: KindParked, ID: row.ID, Title: titleOf(row),
			Asked: "Unpark " + titleOf(row) + "? parked by " + row.Waiting.By + " " + row.Waiting.Since + ": " + row.Waiting.Reason,
			By:    row.Waiting.By, Since: row.Waiting.Since,
			Silence: "it stays parked",
			Where:   Where{Kind: WhereGoal, ID: row.ID},
			// The act rather than the command: a signed-in session may lift a
			// seat's park from here (R-125-m1u), and a command shown beside a
			// button that does the same thing is one more thing to read.
			Act: ActUnpark,
		})
	}
	return needs
}

// stopped is the goals a breach fence closed.
//
// Silence: the fence outranks everything else the claim carries — the
// projection puts a fenced claim in Waiting whatever else is true of it — and
// only goal resume lifts it. The claim record is untouched by the fence, so
// the goal stays held by the seat that holds it.
func stopped(rows []backlog.Row) []Need {
	needs := []Need{}
	for _, row := range rows {
		if row.Fence == nil {
			continue
		}
		needs = append(needs, Need{
			Kind: KindStopped, ID: row.ID, Title: titleOf(row),
			Asked: "Resume " + titleOf(row) + ", stopped " + row.Fence.ClosedAt + ": " + row.Fence.Reason,
			By:    "the engine", Since: row.Fence.ClosedAt,
			Silence: "it stays stopped; its claim keeps the goal",
			Where:   Where{Kind: WhereGoal, ID: row.ID},
			Command: "metasystem goal resume --id " + row.ID,
		})
	}
	return needs
}

// drafts is every record declaring draft status, whatever its kind.
//
// Silence: nothing writes a record's Status line but a human, through the
// reader's own act or by hand, so a draft nobody accepts stays a draft — and
// the Project page counts it as one.
func drafts(records []project.Record) []Need {
	needs := []Need{}
	for _, record := range records {
		if record.Status != statusDraft {
			continue
		}
		needs = append(needs, Need{
			Kind: KindDraft, ID: recordID(record), Title: record.Title,
			Asked: "Accept the draft " + record.Title + "?",
			By:    record.Kind, Since: record.ChangedAt,
			Silence: "it stays a draft, shown as one on Project",
			Where:   Where{Kind: WhereRecord, ID: record.Path},
			Path:    record.Path,
		})
	}
	return needs
}

// landed is the designs whose named goals have all landed and which nobody
// has marked done.
//
// A design naming no goal is not one of them: "every goal has landed" over an
// empty list is true and means nothing, and offering it would be asking a
// human to conclude a design that never started.
//
// Silence: the Status line is a human's, so the record stays marked whatever
// it says now — which the line names rather than assuming it says "accepted".
func landed(pane project.Pane) []Need {
	states := goalStates(pane.Goals)
	needs := []Need{}
	for _, record := range pane.Records {
		if record.Kind != kindDesign || record.Status == statusDone || len(record.Goals) == 0 {
			continue
		}
		if !allDone(record.Goals, states) {
			continue
		}
		needs = append(needs, Need{
			Kind: KindLanded, ID: recordID(record), Title: record.Title,
			Asked: "Mark " + record.Title + " done?",
			By:    "every goal landed", Since: record.ChangedAt,
			Silence: "it stays marked " + recordedStatus(record),
			Where:   Where{Kind: WhereRecord, ID: record.Path},
			Path:    record.Path,
			// The goals it named, with where each stands. They are the whole
			// of the evidence for marking it done, and the row carries them
			// so the human reads the evidence beside the act rather than
			// taking the claim "every goal landed" on trust.
			Goals: goalsNamed(record.Goals, states),
		})
	}
	return needs
}

// goalsNamed is the goals a record names, in the order it names them, each
// with the state the projection gives it — or nothing where this checkout has
// no goal under that id, which is a record naming work that is not here rather
// than work that has not started.
func goalsNamed(named []string, states map[string]string) []GoalState {
	listed := make([]GoalState, 0, len(named))
	for _, id := range named {
		listed = append(listed, GoalState{ID: id, State: states[id]})
	}
	return listed
}

// recordedStatus is what the record says it is, or the plain statement that it
// says nothing. A line reading "it stays marked" with nothing after it would
// be the page losing a word rather than the record lacking one.
func recordedStatus(record project.Record) string {
	if record.Status == "" {
		return "with no status of its own"
	}
	return record.Status
}

// rulingReviews is the scheduled rulings whose valid due date is today or
// earlier.
//
// It is date-based and only date-based. An event condition has no due state
// this interface can read: whether an event happened is the steward's own
// evaluation, with outcomes it calls unobservable, and a page that guessed
// would be telling a human a ruling is overdue on no evidence. Event
// conditions are shown on the cards below and judged by nobody here.
//
// Silence: no verb of this engine writes the register. The sweep reads it and
// appends one line to the steward's digest; the row itself is never touched,
// so the ruling stays in force exactly as written.
func rulingReviews(register rulings.Register, at string, now time.Time) []Need {
	// The register's rows by id, so that a review carries what was actually
	// ruled. The review record is the schedule — the id, the owner, the class
	// and the date — and the words are in the row beside it under the same id.
	// A row that carries the id and not the words is the one thing this page
	// used to make a human open the register to read.
	worded := map[string]rulings.Row{}
	for _, row := range register.Rows {
		worded[row.ID] = row
	}
	needs := []Need{}
	for _, review := range register.Reviews {
		if !rulings.DuePassed(review.Due, now) {
			continue
		}
		row := worded[review.ID]
		due := dayStamp(review.Due)
		needs = append(needs, Need{
			Kind: KindRulingReview, ID: review.ID, Title: review.ID,
			Asked: "Review " + review.ID + ", due " + review.Due + ": adopt, revise or withdraw",
			By:    review.Owner, Since: due, sinceIsDay: due != "",
			Silence: "it stays in force as written",
			Where:   Where{Kind: WhereRegister, ID: at},
			// The schedule is the review's own; the words and the context are
			// the register row's. A review whose row the reader could not read
			// carries the schedule and empty words rather than nothing at all.
			Words: row.Words, Context: row.Context,
			Owner: review.Owner, Class: review.Class, Due: review.Due,
		})
	}
	return needs
}

// alerts is the steward's alerts and handoffs from the last seven days,
// newest first. The journal is served newest first, so nothing is sorted
// again.
//
// Silence: the journal is a log of what the steward attempted to deliver.
// Nothing reads a line back, nothing expires one, and no verb acts on one, so
// there is no recorded consequence of leaving it.
func alerts(journal []notifications.Notice, now time.Time) []Need {
	from := now.Add(-alertWindow)
	needs := []Need{}
	for _, notice := range journal {
		if !addressesTheHuman(notice) {
			continue
		}
		at, dated := instant(notice.At)
		if !dated || !at.After(from) {
			continue
		}
		needs = append(needs, Need{
			Kind: KindAlert, ID: notice.ID, Title: notice.Message,
			Asked: notice.Message, By: notice.Source, Since: notice.At,
			Silence: "no recorded consequence",
			Where:   Where{Kind: WhereNotifications, ID: notice.ID},
		})
	}
	return needs
}

func addressesTheHuman(notice notifications.Notice) bool {
	for _, source := range forTheHuman {
		if notice.Source == source {
			return true
		}
	}
	return false
}

/* --------------------------------------------------------------- the order -- */

// The four tiers the inbox is read in. A deadline is a date to come and beats
// everything; then what is already past due, most overdue first; then the
// work nobody has admitted, in the order the backlog itself ranks it; then
// everything else, oldest first, because the thing that has waited longest is
// the thing a human has most likely forgotten.
const (
	tierDeadline = iota
	tierPastDue
	tierApproval
	tierRest
)

// placed is one row with the two things it is sorted by, worked out once so
// that the comparison below reads what it compares rather than recomputing it
// per pair.
type placed struct {
	need Need
	tier int
	key  string
}

// order sorts the inbox in place. It is stable, so the two tiers that carry
// their own order — the approvals, ranked by the backlog, and the rest,
// oldest first — keep the order they were composed in where the key ties.
func order(needs []Need) {
	rows := make([]placed, len(needs))
	for index, need := range needs {
		tier := tierOf(need)
		rows[index] = placed{need: need, tier: tier, key: orderKey(need, tier)}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].tier != rows[j].tier {
			return rows[i].tier < rows[j].tier
		}
		if rows[i].key == rows[j].key {
			return false
		}
		// An undated row is never the oldest and never the most overdue:
		// nothing recorded when it began, so nothing here may claim it began
		// first.
		if rows[i].key == "" {
			return false
		}
		if rows[j].key == "" {
			return true
		}
		return rows[i].key < rows[j].key
	})
	for index := range rows {
		needs[index] = rows[index].need
	}
}

func tierOf(need Need) int {
	switch {
	case need.Deadline != "":
		return tierDeadline
	case need.Kind == KindRenewal || need.Kind == KindRulingReview:
		return tierPastDue
	case need.Kind == KindApproval:
		return tierApproval
	default:
		return tierRest
	}
}

// orderKey is what a tier sorts on, smallest first. The deadline tier sorts
// by the deadline, the past-due tier by the date that passed, the approvals
// by nothing at all — the backlog already ranked them — and the rest by when
// they began.
func orderKey(need Need, tier int) string {
	switch tier {
	case tierDeadline:
		return need.Deadline
	case tierApproval:
		return ""
	default:
		return need.Since
	}
}

/* ------------------------------------------------------------- what you said -- */

func decided(in Inputs, now time.Time) Decided {
	return Decided{
		Rulings:   register(in, now),
		Defects:   defects(in.Register),
		Decisions: decisionRecords(in.Project.Records),
		Answered:  answeredQuestions(in.Project.Questions),
		Approved:  approvedGoals(in.Rows, in.Closed),
		NotNow:    notNow(in.Rows),
	}
}

// notNow is every park a person made, newest first.
//
// Every one of them, blocker parks included: a human who parks a goal behind
// another goal has decided the same thing as one who parks it for a reason —
// not now — and the row says what it is waiting for so the difference is
// read rather than hidden. It leaves by itself when the blockers finish,
// which is the engine's rule and not this page's.
//
// Newest first, because a human reading what they paused reads what they
// paused last; an undated park sorts after every dated one rather than
// claiming a position this page would have to invent.
func notNow(rows []backlog.Row) []NotNow {
	parks := []NotNow{}
	for _, row := range rows {
		if row.State != goal.StateParked || row.Waiting == nil {
			continue
		}
		if !strings.HasPrefix(row.Waiting.By, humanPark) {
			continue
		}
		parks = append(parks, NotNow{
			ID: row.ID, Title: titleOf(row), By: row.Waiting.By,
			At: row.Waiting.Since, Because: row.Waiting.Reason,
			Blocker: row.Waiting.Blocker,
			Where:   Where{Kind: WhereGoal, ID: row.ID},
		})
	}
	sort.SliceStable(parks, func(i, j int) bool {
		if parks[i].At == parks[j].At {
			return false
		}
		if parks[i].At == "" {
			return false
		}
		if parks[j].At == "" {
			return true
		}
		return parks[i].At > parks[j].At
	})
	return parks
}

// register is every row the reader could read whole, newest first, with the
// goals its words name.
//
// Newest first is the register's own order reversed: it is append-only, so
// the last row is the newest, and a human reading what they decided reads
// what they decided last.
func register(in Inputs, now time.Time) []Ruling {
	goals := goalIDs(in.Rows, in.Closed)
	rows := in.Register.Rows
	shown := make([]Ruling, 0, len(rows))
	for index := len(rows) - 1; index >= 0; index-- {
		row := rows[index]
		shown = append(shown, Ruling{
			ID: row.ID, Date: row.Date, Words: row.Words, Context: row.Context,
			Owner: row.Owner, Class: row.Class, Due: row.Due, Event: row.Event,
			Condition: row.Condition,
			DuePassed: rulings.DuePassed(row.Due, now),
			Mentions:  mentions(row.Words, goals),
		})
	}
	return shown
}

// defects is the register's broken rows, in the steward's own words, as the
// digest would print them.
func defects(read rulings.Register) []string {
	listed := []string{}
	for _, defect := range read.Defects {
		listed = append(listed, defect.Label+": "+defect.Reason)
	}
	return listed
}

// decisionRecords is the checkout's decision records, newest first by the
// instant their file was written.
func decisionRecords(records []project.Record) []Item {
	items := []Item{}
	for _, record := range records {
		if record.Kind != kindDecision {
			continue
		}
		items = append(items, Item{
			ID: recordID(record), Title: record.Title, Note: record.Status,
			At: record.ChangedAt, Where: Where{Kind: WhereRecord, ID: record.Path},
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].At > items[j].At })
	return items
}

// answeredQuestions is the register's answered rows, each with what the row
// names as having answered it.
func answeredQuestions(register []project.Question) []Item {
	items := []Item{}
	for _, question := range register {
		if !strings.HasPrefix(question.Status, answeredPrefix) {
			continue
		}
		items = append(items, Item{
			ID: question.ID, Title: question.Question,
			Note:  strings.TrimSpace(strings.TrimPrefix(question.Status, answeredPrefix)),
			At:    question.Opened,
			Where: Where{Kind: WhereQuestion, ID: question.ID},
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].At > items[j].At })
	return items
}

// approvedGoals is every row that carries an approval, live and concluded,
// newest approval first, each with its whole row.
//
// The row travels because the sheet needs it: Withdraw opens the board's own
// sheet over the same row the board would have handed it, and the board's own
// eligibility — which lane the row is in — decides whether the act is offered
// at all. That judgement is the board's and is made where the board makes it.
func approvedGoals(rows, closed []backlog.Row) []Approved {
	approved := []Approved{}
	for _, set := range [][]backlog.Row{rows, closed} {
		for _, row := range set {
			if row.Approved == nil {
				continue
			}
			approved = append(approved, Approved{
				ID: row.ID, Title: titleOf(row), By: row.Approved.By, At: row.Approved.At,
				Authority: row.Approved.Authority, Expired: row.Approved.Expired, Row: row,
			})
		}
	}
	sort.SliceStable(approved, func(i, j int) bool {
		if approved[i].At != approved[j].At {
			return approved[i].At > approved[j].At
		}
		return approved[i].ID < approved[j].ID
	})
	return approved
}

/* ------------------------------------------------------------- the mentions -- */

// mentions is the goals this ruling's words name verbatim, in the order they
// appear in the words.
//
// Verbatim means the whole id with nothing of an identifier on either side of
// it: a ruling about g1-s4 does not mention g1-s44, and a ruling about g1-s44
// does not mention g1-s4. Record ids are not scanned for: this checkout mints
// them as ULIDs, and no ruling has ever written one into its words, so
// scanning for six hundred of them would cost every read and find nothing.
func mentions(words string, goals []string) []string {
	found := []string{}
	seen := map[string]bool{}
	for position := 0; position < len(words); {
		matched := ""
		for _, id := range goals {
			if len(id) < 3 || !strings.HasPrefix(words[position:], id) {
				continue
			}
			if !boundedAt(words, position, len(id)) {
				continue
			}
			if len(id) > len(matched) {
				matched = id
			}
		}
		if matched == "" {
			position++
			continue
		}
		if !seen[matched] {
			seen[matched] = true
			found = append(found, matched)
		}
		position += len(matched)
	}
	return found
}

// boundedAt reports that the run of length characters at position is a whole
// token: what precedes and follows it is neither a letter, a digit, an
// underscore nor a hyphen, so an id is never found inside a longer name.
func boundedAt(words string, position, length int) bool {
	if position > 0 && identifierByte(words[position-1]) {
		return false
	}
	after := position + length
	return after >= len(words) || !identifierByte(words[after])
}

func identifierByte(character byte) bool {
	switch {
	case character >= 'a' && character <= 'z':
		return true
	case character >= 'A' && character <= 'Z':
		return true
	case character >= '0' && character <= '9':
		return true
	case character == '_' || character == '-':
		return true
	default:
		return false
	}
}

func goalIDs(rows, closed []backlog.Row) []string {
	ids := make([]string, 0, len(rows)+len(closed))
	for _, set := range [][]backlog.Row{rows, closed} {
		for _, row := range set {
			ids = append(ids, row.ID)
		}
	}
	return ids
}

/* ----------------------------------------------------------------- the small -- */

// titleOf is what a goal is called: the first sentence of its intent, or its
// id where the record states none.
func titleOf(row backlog.Row) string {
	if lede := firstSentence(row.Intent); lede != "" {
		return lede
	}
	return row.ID
}

// firstSentence is a record's first statement, without the rest of it.
func firstSentence(text string) string {
	trimmed := strings.TrimSpace(text)
	if cut := strings.IndexAny(trimmed, ".\n"); cut > 0 {
		return strings.TrimSpace(trimmed[:cut])
	}
	return trimmed
}

// recordID is what a record calls itself, or its path where it declares no
// id: a row has to be identified by something, and the path is what the
// reader opens it by anyway.
func recordID(record project.Record) string {
	if record.ID != "" {
		return record.ID
	}
	return record.Path
}

func goalStates(goals []project.Goal) map[string]string {
	states := map[string]string{}
	for _, one := range goals {
		states[one.ID] = one.State
	}
	return states
}

func allDone(named []string, states map[string]string) bool {
	for _, id := range named {
		if states[id] != statusDone {
			return false
		}
	}
	return true
}

func sortByRank(rows []backlog.Row) {
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Priority != rows[j].Priority {
			return rows[i].Priority < rows[j].Priority
		}
		if rows[i].Sequence != rows[j].Sequence {
			return rows[i].Sequence < rows[j].Sequence
		}
		return rows[i].ID < rows[j].ID
	})
}

func instant(at string) (time.Time, bool) {
	parsed, err := time.Parse(time.RFC3339, at)
	return parsed, err == nil
}

func stamp(at time.Time) string {
	if at.IsZero() {
		return ""
	}
	return at.UTC().Format(time.RFC3339)
}

// dayStamp is a register date as an instant, so that a due date sorts beside
// the RFC3339 stamps every other row carries. A date that does not parse
// carries nothing rather than an instant nobody recorded.
func dayStamp(day string) string {
	parsed, err := time.Parse(rulings.DateLayout, day)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}
