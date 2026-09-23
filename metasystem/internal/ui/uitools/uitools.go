// Package uitools is the interface's own read tools, as one stdio tool server.
//
// The Partner is told what the human sees; this is how it sees more. Eight
// named operations answer from the same readers the pages are composed from —
// the accepted ledger's projection, the project's records, a document as it
// stands, the landing page, the steward's journal — so an answer about a goal
// and the card that goal is on cannot disagree.
//
// Three rules hold for every operation, and they are the whole of what makes a
// bounded tool honest:
//
//   - Every result names the source it read from: the accepted tip and the
//     moment it was observed, or the file's own revision, "as it stands". A
//     ledger tip cannot stamp a document, and a document's revision cannot
//     stamp a goal.
//   - Every result is bounded, says how much of the whole it supplied, and
//     carries a cursor when more remains. "Twenty-five of a hundred and
//     sixty-three" is a fact the Partner can act on; a silent truncation is a
//     fact it cannot.
//   - Nothing here writes. There is no operation that could, and the process
//     that serves these tools opens the checkout read-only.
package uitools

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/manifest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// ServerName is what this tool server is called on the wire. A runtime that
// prefixes its tool names with the server's own name — Claude's adapter
// presents them as mcp__metasystem__board — composes that prefix from this.
const ServerName = "metasystem"

// The ten operations. They are named here, once, because two things depend
// on the same list: the tool catalogue this server publishes, and the
// permission rule that admits calls to it.
//
// Eight read this workspace. The last two read what the workspace is made of:
// the interface's own manifest, and the kit's own glossary, verbs, rulings and
// routes. They answer from owners rather than from anything written for the
// Partner, which is what keeps one account of each fact.
const (
	OpBoard         = "board"
	OpGoal          = "goal"
	OpDocument      = "document"
	OpRecords       = "records"
	OpQuestions     = "questions"
	OpOverview      = "overview"
	OpNotifications = "notifications"
	OpSearch        = "search"
	OpInterface     = "interface"
	OpKit           = "kit"
)

// Operations is every operation this server answers, in the order the
// catalogue lists them.
var Operations = []string{
	OpBoard, OpGoal, OpDocument, OpRecords, OpQuestions,
	OpOverview, OpNotifications, OpSearch, OpInterface, OpKit,
}

// Names reports whether a bare operation name is one this server answers.
func Names(operation string) bool {
	for _, known := range Operations {
		if known == operation {
			return true
		}
	}
	return false
}

// MaxBody is the ceiling on one result's body, in characters. Past it the
// result stops at a whole row and hands back a cursor, so what was left out is
// a number and a way to fetch it rather than a silence.
const MaxBody = 8000

// The two ceilings inside one result that are not the body's: how much of one
// document travels in one call, and how many rows a page of a listing holds
// before the body's own bound decides.
const (
	maxRows = 200
	// maxIntent is how much of a goal's intent one row line carries.
	maxIntent = 160
)

// Result is one answer: what it read, how much of it, and where the rest is.
type Result struct {
	// Source is the reading this answer is of: the accepted tip and the moment
	// it was observed, or a file's revision as it stands.
	Source string
	// Supplied and Total are the rows this answer carries and the rows the
	// reading held. They are equal when nothing was left out.
	Supplied int
	Total    int
	// Cursor is what a second call passes to continue, or empty where nothing
	// remains.
	Cursor string
	Body   string
	// Problem is the reader's own words where the read failed. A result with a
	// problem carries no rows and is never a look.
	Problem string
}

// Failed reports whether this result is a read that did not happen.
func (r Result) Failed() bool { return r.Problem != "" }

// Text is the result on the wire: the header every operation carries, then
// what it read. The header is first so that a Partner quoting the body has
// already read what the body is a reading of.
func (r Result) Text() string {
	var built strings.Builder
	if r.Source != "" {
		built.WriteString("Source: " + r.Source + "\n")
	}
	if r.Problem != "" {
		built.WriteString("Outcome: this read failed — " + r.Problem + "\n")
		return built.String()
	}
	built.WriteString(fmt.Sprintf("Supplied: %d of %d\n", r.Supplied, r.Total))
	if r.Cursor != "" {
		built.WriteString("More remains: call this tool again with cursor \"" + r.Cursor + "\".\n")
	}
	built.WriteString("\n")
	built.WriteString(r.Body)
	return built.String()
}

// Readers are the server's own readers, the same ones the pages are composed
// from. A nil reader is a build that cannot answer for that half, which the
// result says rather than guesses at.
type Readers struct {
	Observe  func() snapshot.Observation
	Document func(id string) (project.Document, error)
	Project  func() (project.Pane, error)
	Overview func() (overview.Page, error)
	Notices  func(limit int, before string) ([]notifications.Notice, error)
	Now      func() time.Time
	// Interface composes what this interface is made of: the half the bundle
	// carries joined to the half this seat resolves. A nil composer is a build
	// that cannot describe itself, which the result says.
	Interface func() (manifest.Manifest, error)
	// Kit is where this kit's own knowledge lives: the glossary AGENTS.md
	// points at, the rulings register, the routes, and the command catalogue
	// the binary routes with.
	Kit Kit
}

func (r Readers) now() time.Time {
	if r.Now == nil {
		return time.Now().UTC()
	}
	return r.Now().UTC()
}

// Answer runs one operation. An unknown operation, a missing reader and a
// refused read are all the same shape: a result that says what failed.
func (r Readers) Answer(operation string, args Args) Result {
	switch operation {
	case OpBoard:
		return r.board(args.Text("filters"), args.Cursor())
	case OpGoal:
		return r.goal(args.Text("id"))
	case OpDocument:
		return r.document(args.Text("id"), args.Cursor())
	case OpRecords:
		return r.records(args.Text("kind"), args.Cursor())
	case OpQuestions:
		return r.questions()
	case OpOverview:
		return r.overview()
	case OpNotifications:
		return r.notifications(args.Number("limit", notifications.DefaultLimit), args.Cursor())
	case OpSearch:
		return r.search(args.Text("text"), args.Cursor())
	case OpInterface:
		return r.describe(args.Text("part"), args.Cursor())
	case OpKit:
		return r.kit(args.Text("topic"), args.Cursor())
	default:
		return Result{Problem: "this server answers " + strings.Join(Operations, ", ") + ", not " + operation}
	}
}

/* ---------------------------------------------------------- the readings -- */

// ledgerSource is what a reading of the accepted tip is stamped with.
func ledgerSource(observed snapshot.Observation, now time.Time) string {
	if observed.Tip == "" {
		return "this seat could not read an accepted ledger"
	}
	at := observed.ObservedAt
	if at.IsZero() {
		at = now
	}
	return "the accepted tip " + observed.Tip + ", observed " + at.UTC().Format(time.RFC3339)
}

// projection is the board at the accepted tip, or the refusal that stands in
// for it.
func (r Readers) projection() (backlog.Board, snapshot.Observation, string) {
	if r.Observe == nil {
		return backlog.Board{}, snapshot.Observation{}, "this build has no ledger reader"
	}
	observed := r.Observe()
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		problem := observed.Message
		if problem == "" {
			problem = "the accepted ledger could not be read"
		}
		return backlog.Board{}, observed, problem
	}
	return backlog.Project(observed.Tree, observed.Horizon, observed.Admission), observed, ""
}

func (r Readers) board(filters, cursor string) Result {
	board, observed, problem := r.projection()
	source := ledgerSource(observed, r.now())
	if problem != "" {
		return Result{Source: source, Problem: problem}
	}
	rows := append(append([]backlog.Row{}, board.Rows...), board.Closed...)
	sort.SliceStable(rows, func(left, right int) bool {
		return laneOrder(rows[left].Lane) < laneOrder(rows[right].Lane)
	})
	lines := make([]string, 0, len(rows))
	for _, row := range rows {
		line := "- " + string(row.Lane) + ": " + rowLine(row)
		if !contains(line, filters) {
			continue
		}
		lines = append(lines, line)
	}
	return paged(source, lines, cursor)
}

// laneOrder is the board's own column order, so a listing reads down the board
// the way the page reads across it.
func laneOrder(lane backlog.Lane) int {
	for at, known := range backlog.LaneOrder {
		if known == lane {
			return at
		}
	}
	return len(backlog.LaneOrder)
}

func (r Readers) goal(id string) Result {
	id = strings.TrimSpace(id)
	board, observed, problem := r.projection()
	source := ledgerSource(observed, r.now())
	if problem != "" {
		return Result{Source: source, Problem: problem}
	}
	if id == "" {
		return Result{Source: source, Problem: "this tool needs the id of a goal"}
	}
	all := append(append([]backlog.Row{}, board.Rows...), board.Closed...)
	for _, row := range all {
		if row.ID != id {
			continue
		}
		var built strings.Builder
		built.WriteString(fullRow(row))
		neighbours := []string{}
		for _, other := range all {
			if other.Lane == row.Lane && other.ID != row.ID {
				neighbours = append(neighbours, other.ID)
			}
		}
		if len(neighbours) > 0 {
			built.WriteString("- The other goals in " + string(row.Lane) + " (" +
				strconv.Itoa(len(neighbours)) + "): " + strings.Join(neighbours, ", ") + "\n")
		}
		return bounded(source, built.String(), 1, 1)
	}
	return Result{Source: source, Problem: "the accepted tip carries no goal " + id}
}

func (r Readers) document(id, cursor string) Result {
	id = strings.TrimSpace(id)
	if r.Document == nil {
		return Result{Problem: "this build has no document reader"}
	}
	if id == "" {
		return Result{Problem: "this tool needs the checkout-relative path of a document"}
	}
	document, err := r.Document(id)
	if err != nil {
		return Result{Source: id + " as it stands", Problem: err.Error()}
	}
	// The id, not the path: a source a human reads beside an answer has to be
	// the checkout-relative name the pages use, not this seat's own directory.
	source := document.ID + " as it stands, revision " + document.Revision
	head := ""
	if document.Record != nil {
		head = recordHead(*document.Record)
	}
	// A document is prose, not rows, so it is paged by characters: the offset
	// is where this call starts, and the cursor is where the next one does.
	at := offsetOf(cursor)
	body := document.Source
	total := len(body)
	if at > total {
		at = total
	}
	rest := body[at:]
	supplied := len(rest)
	next := ""
	if supplied > MaxBody {
		cut := strings.LastIndex(rest[:MaxBody], "\n")
		if cut <= 0 {
			cut = MaxBody
		}
		rest = rest[:cut]
		supplied = cut
		next = strconv.Itoa(at + cut)
	}
	prefix := ""
	if at == 0 && head != "" {
		prefix = head + "\n"
	}
	return Result{
		Source: source, Supplied: at + supplied, Total: total, Cursor: next,
		Body: prefix + rest,
	}
}

func recordHead(head project.Head) string {
	var built strings.Builder
	writeIf(&built, "Kind", head.Kind)
	writeIf(&built, "Record id", head.ID)
	writeIf(&built, "Status", head.Status)
	writeList(&built, "Goals", head.Goals)
	writeList(&built, "Cites", head.Cites)
	writeList(&built, "Affects", head.Affects)
	writeList(&built, "Governs", head.Governs)
	writeList(&built, "Supersedes", head.Supersedes)
	return built.String()
}

func (r Readers) pane() (project.Pane, string) {
	if r.Project == nil {
		return project.Pane{}, "this build has no project reader"
	}
	pane, err := r.Project()
	if err != nil {
		return project.Pane{}, err.Error()
	}
	return pane, ""
}

// checkoutSource is what a reading of the working tree is stamped with: the
// files as they stand, at the moment they were read.
func checkoutSource(readAt string, now time.Time) string {
	if strings.TrimSpace(readAt) == "" {
		readAt = now.UTC().Format(time.RFC3339)
	}
	return "the checkout's records as they stand, read " + readAt
}

func (r Readers) records(kind, cursor string) Result {
	pane, problem := r.pane()
	source := checkoutSource(pane.ReadAt, r.now())
	if problem != "" {
		return Result{Source: source, Problem: problem}
	}
	kind = strings.ToLower(strings.TrimSpace(kind))
	lines := make([]string, 0, len(pane.Records))
	for _, record := range pane.Records {
		if kind != "" && strings.ToLower(record.Kind) != kind {
			continue
		}
		lines = append(lines, "- "+recordLine(record))
	}
	return paged(source, lines, cursor)
}

func recordLine(record project.Record) string {
	parts := []string{record.Title}
	if record.Kind != "" {
		parts = append(parts, record.Kind)
	}
	if record.Status != "" {
		parts = append(parts, record.Status)
	}
	if record.ID != "" {
		parts = append(parts, "id "+record.ID)
	}
	if len(record.Goals) > 0 {
		parts = append(parts, "goals "+strings.Join(record.Goals, " "))
	}
	parts = append(parts, record.Path)
	return strings.Join(parts, " · ")
}

func (r Readers) questions() Result {
	pane, problem := r.pane()
	source := checkoutSource(pane.ReadAt, r.now())
	if problem != "" {
		return Result{Source: source, Problem: problem}
	}
	lines := make([]string, 0, len(pane.Questions))
	for _, asked := range pane.Questions {
		line := "- " + firstSentence(asked.Question, maxIntent) + " · " + asked.Status + " · " + asked.ID
		if asked.Opened != "" {
			line += " · opened " + asked.Opened
		}
		if len(asked.Goals) > 0 {
			line += " · goals " + strings.Join(asked.Goals, " ")
		}
		lines = append(lines, line)
	}
	return paged(source, lines, "")
}

func (r Readers) overview() Result {
	if r.Overview == nil {
		return Result{Problem: "this build cannot compose the landing page"}
	}
	page, err := r.Overview()
	if err != nil {
		return Result{Problem: err.Error()}
	}
	var built strings.Builder
	needs := page.NeedsYou
	built.WriteString("- Needs you (" + strconv.Itoa(needs.Total) + " in all):\n")
	for _, group := range []struct {
		name  string
		count int
	}{
		{"approvals waiting", needs.Approvals.Count},
		{"open questions", needs.Questions.Count},
		{"drafts", needs.Drafts.Count},
		{"designs whose work has landed", needs.Designs.Count},
		{"alerts and handoffs", needs.Alerts.Count},
	} {
		built.WriteString("  - " + group.name + ": " + strconv.Itoa(group.count) + "\n")
	}
	built.WriteString("- Work now:\n")
	built.WriteString("  - in progress: " + strconv.Itoa(len(page.Work.InProgress)) + "\n")
	built.WriteString("  - next in Ready for Work: " + strconv.Itoa(len(page.Work.Next)) + "\n")
	for _, lane := range page.Work.Lanes {
		built.WriteString("  - " + lane.ID + ": " + strconv.Itoa(lane.Count) + "\n")
	}
	built.WriteString("- The window this page compares against opens " + page.Since + "\n")
	source := "the landing page as this server composes it, read " + page.ReadAt
	return bounded(source, built.String(), 1, 1)
}

func (r Readers) notifications(limit int, cursor string) Result {
	if r.Notices == nil {
		return Result{Problem: "this seat keeps no steward journal"}
	}
	if limit <= 0 || limit > notifications.DefaultLimit {
		limit = notifications.DefaultLimit
	}
	notices, err := r.Notices(limit, cursor)
	source := "the steward's journal as it stands, read " + r.now().Format(time.RFC3339)
	if err != nil {
		return Result{Source: source, Problem: err.Error()}
	}
	lines := make([]string, 0, len(notices))
	oldest := ""
	for _, notice := range notices {
		line := "- " + notice.At + " · " + notice.Source + " · " + oneLine(notice.Message)
		if notice.Ref != "" {
			line += " · " + notice.Ref
		}
		if !notice.Delivered {
			line += " · not delivered: " + notice.Error
		}
		lines = append(lines, line)
		oldest = notice.ID
	}
	result := paged(source, lines, "")
	// The journal's own cursor is an id rather than an offset: it is read
	// newest first, and the next page is what stands before the oldest here.
	if len(notices) == limit && oldest != "" {
		result.Cursor = oldest
		result.Total = result.Supplied + 1
	}
	return result
}

func (r Readers) search(text, cursor string) Result {
	text = strings.TrimSpace(text)
	if text == "" {
		return Result{Problem: "this tool needs something to search for"}
	}
	lines := []string{}
	sources := []string{}
	board, observed, problem := r.projection()
	if problem == "" {
		sources = append(sources, ledgerSource(observed, r.now()))
		for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
			line := "- goal " + string(row.Lane) + ": " + rowLine(row)
			if contains(line, text) {
				lines = append(lines, line)
			}
		}
	}
	pane, paneProblem := r.pane()
	if paneProblem == "" {
		sources = append(sources, checkoutSource(pane.ReadAt, r.now()))
		for _, record := range pane.Records {
			line := "- record: " + recordLine(record)
			if contains(line, text) || contains(record.Summary, text) {
				lines = append(lines, line)
			}
		}
		for _, document := range pane.Documents {
			line := "- document: " + document.Path
			if contains(line, text) {
				lines = append(lines, line)
			}
		}
		for _, asked := range pane.Questions {
			line := "- question: " + firstSentence(asked.Question, maxIntent) + " · " + asked.Status + " · " + asked.ID
			if contains(line, text) {
				lines = append(lines, line)
			}
		}
	}
	if len(sources) == 0 {
		return Result{Problem: "neither the ledger nor the project could be read: " + problem + "; " + paneProblem}
	}
	return paged(strings.Join(sources, "; and "), lines, cursor)
}

// describe is the interface's own manifest, one part at a time.
//
// The whole of it is longer than one bounded result, and a manifest cut in
// half is a manifest that teaches half an interface. So a reader names a part
// — the sections, the lanes, the terms, the suggested questions, the acts, the
// settings, the record kinds, the runtimes — and pages within it; naming none
// answers the summary, which says what the parts are and what the Partner may
// itself do here.
func (r Readers) describe(part, cursor string) Result {
	if r.Interface == nil {
		return Result{Problem: "this build cannot describe its own interface"}
	}
	described, err := r.Interface()
	if err != nil {
		return Result{Problem: err.Error()}
	}
	source := "this build's own interface: the half the bundle carries joined to the half this seat resolves, read " +
		r.now().Format(time.RFC3339)
	lines, known := described.Lines(part)
	if !known {
		return Result{Source: source, Problem: "this manifest has the parts " +
			strings.Join(manifest.Parts, ", ") + ", not " + part}
	}
	return paged(source, lines, cursor)
}

/* ------------------------------------------------------------- the bound -- */

// paged is one listing, from the cursor's offset, to the body's bound.
func paged(source string, lines []string, cursor string) Result {
	at := offsetOf(cursor)
	total := len(lines)
	if at > total {
		at = total
	}
	rest := lines[at:]
	body := strings.Builder{}
	taken := 0
	for _, line := range rest {
		if taken >= maxRows || body.Len()+len(line)+1 > MaxBody {
			break
		}
		body.WriteString(line + "\n")
		taken++
	}
	if taken == 0 && len(rest) > 0 {
		// One row longer than the whole bound: it is cut rather than dropped,
		// because a listing that answers nothing answers nothing at all.
		body.WriteString(rest[0][:min(len(rest[0]), MaxBody)] + "…\n")
		taken = 1
	}
	next := ""
	if at+taken < total {
		next = strconv.Itoa(at + taken)
	}
	if total == 0 {
		body.WriteString("Nothing here.\n")
	}
	return Result{Source: source, Supplied: at + taken, Total: total, Cursor: next, Body: body.String()}
}

// bounded is one block that is built whole: it is cut at the bound and says so.
func bounded(source, body string, supplied, total int) Result {
	if len(body) <= MaxBody {
		return Result{Source: source, Supplied: supplied, Total: total, Body: body}
	}
	cut := strings.LastIndex(body[:MaxBody], "\n")
	if cut <= 0 {
		cut = MaxBody
	}
	return Result{Source: source, Supplied: supplied, Total: total,
		Body: body[:cut] + "\n(the rest of this reading is longer than one result carries)\n"}
}

func offsetOf(cursor string) int {
	at, err := strconv.Atoi(strings.TrimSpace(cursor))
	if err != nil || at < 0 {
		return 0
	}
	return at
}

func contains(line, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	if wanted == "" {
		return true
	}
	return strings.Contains(strings.ToLower(line), strings.ToLower(wanted))
}

/* ------------------------------------------------------------- the lines -- */

// rowLine is one goal as the board's card reads it.
func rowLine(row backlog.Row) string {
	parts := []string{row.ID}
	if intent := firstSentence(row.Intent, maxIntent); intent != "" {
		parts = append(parts, intent)
	}
	if row.Tier > 0 {
		parts = append(parts, "tier "+strconv.Itoa(int(row.Tier)))
	}
	if row.Priority > 0 || row.Sequence > 0 {
		parts = append(parts, strconv.Itoa(int(row.Priority))+":"+strconv.FormatUint(row.Sequence, 10))
	}
	if state := strings.TrimSpace(row.State); state != "" {
		parts = append(parts, state)
	}
	if row.Claim != nil && row.Claim.Machine != "" {
		parts = append(parts, "seat "+row.Claim.Machine)
	}
	if row.Arc != "" {
		parts = append(parts, "arc "+row.Arc)
	}
	if row.Waiting != nil {
		if reason := firstSentence(row.Waiting.Reason, maxIntent); reason != "" {
			parts = append(parts, "waiting: "+reason)
		}
	}
	return strings.Join(parts, " · ")
}

// fullRow is one goal, field by field, as the goal's own page renders it.
func fullRow(row backlog.Row) string {
	var built strings.Builder
	built.WriteString("- Goal: " + row.ID + " (record revision " + strconv.FormatUint(row.Revision, 10) + ", " + row.Where + ")\n")
	writeIf(&built, "Lane", string(row.Lane))
	writeIf(&built, "State", row.State)
	writeIf(&built, "Phase", row.Phase)
	writeIf(&built, "Intent", row.Intent)
	writeIf(&built, "Next step", row.NextStep)
	writeIf(&built, "Concluded", row.Concluded)
	writeIf(&built, "Origin", row.Origin)
	writeIf(&built, "Priority", strconv.Itoa(int(row.Priority)))
	writeIf(&built, "Sequence", strconv.FormatUint(row.Sequence, 10))
	writeIf(&built, "Tier", strconv.Itoa(int(row.Tier)))
	writeIf(&built, "Arc", row.Arc)
	writeList(&built, "Labels", row.Labels)
	writeList(&built, "Blocked by", row.BlockedBy)
	writeList(&built, "Open blockers", row.OpenBlockers)
	if row.Approved != nil {
		approved := "by " + row.Approved.By + " at " + row.Approved.At
		if row.Approved.Expired {
			approved += " (expired: " + row.Approved.ExpiredWhy + ")"
		}
		writeIf(&built, "Approved", approved)
	}
	if row.Claim != nil {
		writeIf(&built, "Claimed", "by "+row.Claim.Machine+" since "+row.Claim.At)
	}
	if row.Waiting != nil {
		waiting := row.Waiting.Reason
		if row.Waiting.Blocker != "" {
			waiting += " (" + row.Waiting.Blocker + ")"
		}
		writeIf(&built, "Waiting", waiting)
	}
	writeList(&built, "Gaps the record leaves open", row.Gaps)
	return built.String()
}

func writeIf(built *strings.Builder, label, value string) {
	value = strings.TrimSpace(value)
	if value == "" || value == "0" {
		return
	}
	built.WriteString("- " + label + ": " + oneLine(value) + "\n")
}

func writeList(built *strings.Builder, label string, values []string) {
	if len(values) == 0 {
		return
	}
	built.WriteString("- " + label + ": " + strings.Join(values, ", ") + "\n")
}

// firstSentence is one line of prose: up to the first full stop, cut at the
// bound where even that is a paragraph.
func firstSentence(text string, bound int) string {
	one := oneLine(text)
	if one == "" {
		return ""
	}
	if at := strings.Index(one, ". "); at >= 0 {
		one = one[:at+1]
	}
	if len(one) > bound {
		cut := strings.LastIndex(one[:bound], " ")
		if cut <= 0 {
			cut = bound
		}
		one = one[:cut] + "…"
	}
	return one
}

func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}
