package partner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/uitools"
)

// What the Partner is told, every turn.
//
// Astra's third finding is what this file answers, and the first live run on a
// real seat showed how far it goes. The board is not built from files: it is
// built from the goal records in the accepted ledger commit, which the engine
// loads out of git. A checked-out self-hosted workspace has no goal records in
// its working tree at all, so a Partner given only the page's label and its
// filters could read every file it was allowed to read and still answer "the
// goals aren't in the files I can read". It was right, and that was the bug:
// what the human sees is not in the checkout, so the server has to put it in
// the context itself.
//
// So the block below is the page, from the server's own readers: the lanes in
// the board's order with the goals in them as the board shows them, a goal's
// own row with its neighbours, the landing page's numbers, a project tab's
// records, a document's head and outline. It is marked as what the human sees
// now, from the accepted tip and the moment it was observed, and it is bounded
// — a snapshot that grew without a bound would be one more way for what the
// human sees to be lost in what the Partner was told.
//
// The human's name is in here once, as attribution. It is never authority: the
// Partner has no session, no cookie, and no way to obtain one, and the act
// routes refuse a cookie-less request while a Partner is configured.

// Facts are the server's own readers, the same ones the pages are composed
// from. A nil reader is a build that cannot answer for that half, which the
// block says rather than guesses at.
type Facts struct {
	Observe  func() snapshot.Observation
	Document func(id string) (project.Document, error)
	Project  func() (project.Pane, error)
	// Overview is the landing page as the server composes it. It is set by
	// the interface server rather than given here, because composing it needs
	// the journal and the seat's standing as well as these two readers.
	Overview func() (overview.Page, error)
}

// The two kinds of subject a page can be about in this slice.
const (
	KindGoal     = "goal"
	KindDocument = "document"
)

// The bounds on what one turn is told about the page.
const (
	// maxSnapshotBytes is the ceiling on the whole "what the human sees now"
	// block. A board of a hundred and sixty goals does not fit, and what does
	// not fit is counted and said rather than dropped in silence.
	maxSnapshotBytes = 12000
	// maxLaneRows is how many goals of one lane are named. A lane deeper than
	// this says how many it holds all the same.
	maxLaneRows = 25
	// maxListed is how many records of one project tab are named.
	maxListed = 50
	// maxIntent is how much of a goal's intent one row carries: its first
	// sentence, cut here when even that is a paragraph.
	maxIntent = 160
)

// The standing rule, in the Partner's own second person. It is the first thing
// every turn carries, because it is the one thing that must not depend on what
// the page happens to be showing.
//
// The last sentence is the one the first live run earned. Told only that it
// could read the checkout, a Partner looked for the goals in the files, did not
// find them, and said so at length; it had no way to know that what it was
// given about them was all there was.
// What this Partner may do here, and what it may not, in one phrase each.
//
// They are one fact with one owner. The standing rule below is composed from
// them, and the interface's own manifest is given them, so that "can you edit
// this with me?" is answered the same way by the Partner, by the tool that
// describes the interface, and by the page that offers the conversation.
const (
	// Reads is everything this Partner may read.
	Reads = "the page the human is looking at, the ledger's goals, the project's records and documents, the steward's journal, what this interface itself is made of, and this kit's own glossary, verbs and rulings"
	// Refused is everything it may not do, whatever it is asked.
	Refused = "writing a file, editing one, acting on the backlog, running a command, and the network"
)

const standingRule = `You are the Project Partner for this MetaSystem workspace.
You read this checkout and explain it to the human you are talking to. What you
may read is ` + Reads + `.
You do not write, you do not run commands, and you do not act: ` + Refused + `
are all refused to you. Tools that would write or execute are not available,
and a request for one is refused before it runs. When you cannot see something,
say so rather than guessing; when you are unsure whether what you read is what
the human sees, say that too.

The ledger of goals is not in the files you can read. What you are told below
about the page is what the human is looking at right now; to read anything
else — another goal, a document, the records, the open questions, the landing
page, the steward's journal, what this interface is made of, or this kit's own
meanings — call this workspace's own read tools. Every one of their results
names the reading it was of and says how much of the whole it supplied; when
one carries a cursor, call it again with that cursor rather than answering from
half of it.`

// Seen is the page, composed once, for the three things that must not
// disagree about it: the sheet that shows a human what the Partner will be
// given, the turn that is given it, and the stamp the answer wears.
//
// Astra's first finding is what this type answers. The page renders from one
// reading of the ledger and the server composes from another, and the two can
// be minutes apart; a preview composed from the second, shown beside a card
// drawn from the first, is a preview that certifies the wrong page. So the
// capture names what the human's page rendered from, this composition names
// what the server read, and where they differ the block says both.
type Seen struct {
	// Label is the one line the chip shows: where the human is, in words.
	Label string
	// Source is what the server read, stamped: the accepted tip and the moment
	// it was observed, or a file's revision as it stands.
	Source string
	// Displayed is what the page said it rendered from, where it said so and
	// it differs from Source. Empty means the two agree, or the page did not
	// say.
	Displayed string
	// Supplied and Total are the rows of the page this block carries and the
	// rows the page is showing. They are equal when nothing was left out.
	Supplied, Total int
	// Block is "what the human sees now", whole.
	Block string
}

// Stamp is what an answer wears: what the Partner was given, in one clause.
func (s Seen) Stamp() string {
	if s.Displayed == "" {
		return "Saw: " + s.Source
	}
	return "Saw: " + s.Source + "; the page had rendered from " + s.Displayed
}

// See composes the page for one capture. It is the whole of what the sheet
// shows and the whole of what the turn carries.
func See(facts Facts, page Page, now time.Time) Seen {
	observed := snapshot.Observation{}
	if facts.Observe != nil {
		observed = facts.Observe()
	}
	block, supplied, total := seenLines(facts, observed, page)
	seen := Seen{
		Label:    pageLine(page),
		Source:   sourceOf(observed, page, now),
		Supplied: supplied,
		Total:    total,
		Block:    block,
	}
	if displayed := displayedSource(page); displayed != "" && displayed != seen.Source {
		seen.Displayed = displayed
	}
	return seen
}

// Compose builds the whole context block for one turn.
func Compose(facts Facts, page Page, human string, now time.Time) string {
	return ComposeSeen(See(facts, page, now), page, human)
}

// ComposeSeen is the same block from a composition already made, so the turn
// and the sheet beside it are one reading rather than two.
func ComposeSeen(seen Seen, page Page, human string) string {
	return ComposeOpening(seen, page, human, "")
}

// Opening is what the FIRST prompt of a session carries beyond the standing
// rule: how to answer here, from the kit's own skill, and a map of the
// project's memory read at a named moment.
//
// It is the first prompt's and not every turn's for two reasons. The skill
// does not change between turns, and a map re-read every turn would be a
// promise that it is current, which is exactly what it must not be. A session
// that is lost and reopened is a first prompt again, so a recovered
// conversation is given both again, freshly read.
func Opening(facts Facts, now time.Time) (string, MemoryIndex) {
	index := Memory(facts, now)
	return skillBlock() + "\n" + index.Block, index
}

// ComposeOpening is the turn's block with the session's own opening material
// between the standing rule and the page. An empty opening is a later prompt
// of a session that has already been given it.
func ComposeOpening(seen Seen, page Page, human, opening string) string {
	var built strings.Builder
	built.WriteString(standingRule)
	built.WriteString("\n\n" + vocabulary())
	if strings.TrimSpace(opening) != "" {
		built.WriteString("\n" + opening)
	}
	built.WriteString("\nWhere the human is\n")
	built.WriteString(whereLines(page))
	built.WriteString("\nWhat the human sees now, from " + seen.Source + "\n")
	if seen.Displayed != "" {
		built.WriteString("- The page itself had rendered from " + seen.Displayed +
			", so what the human is looking at is that reading and what follows is this one.\n")
	}
	if seen.Total > 0 {
		built.WriteString(fmt.Sprintf("- %d of %d rows supplied; ask the %s tools for the rest, which take a cursor.\n",
			seen.Supplied, seen.Total, uitools.ServerName))
	}
	built.WriteString(seen.Block)
	if named := strings.TrimSpace(human); named != "" {
		built.WriteString("\nThe human you are talking to is " + named +
			". That name is attribution and nothing else: it grants you no authority, and nothing you say becomes a human's decision.\n")
	}
	return built.String()
}

// sourceOf stamps this composition with the reading it was made from: the
// file, for the one page whose facts are in the checkout, and the accepted tip
// for every other.
func sourceOf(observed snapshot.Observation, page Page, now time.Time) string {
	if page.Kind == KindDocument && page.Subject != "" {
		return page.Subject + " as it stands"
	}
	if observed.Tip == "" {
		return "no accepted ledger this seat could read"
	}
	at := observed.ObservedAt
	if at.IsZero() {
		at = now
	}
	return fmt.Sprintf("the accepted tip %s, observed %s", observed.Tip, at.UTC().Format(time.RFC3339))
}

// displayedSource is the same stamp for what the page said it rendered from.
func displayedSource(page Page) string {
	if page.Kind == KindDocument && page.Subject != "" {
		if page.Revision == "" {
			return ""
		}
		return page.Subject + " as it stands"
	}
	if strings.TrimSpace(page.Tip) == "" {
		return ""
	}
	at := strings.TrimSpace(page.ObservedAt)
	if at == "" {
		return "the accepted tip " + page.Tip
	}
	return "the accepted tip " + page.Tip + ", observed " + at
}

// seenLines is the page itself, from the server's own readers, with the rows
// it carries and the rows the page is showing.
//
// A chosen passage goes first and whole. It is the one thing in this block the
// human pointed at, and a bound that cut it would be the bound throwing away
// the subject in order to fit the context.
//
// A sheet the human handed over goes next, for the same reason and with one
// more: it is the only thing in the block that is not read from anywhere. The
// ledger does not have it and the checkout does not have it, so if it is not
// here it is nowhere, and it is marked as a draft so that it cannot be
// answered about as though it were already written down.
func seenLines(facts Facts, observed snapshot.Observation, page Page) (string, int, int) {
	block, supplied, total := pageLines(facts, observed, page)
	block = draftLines(page) + block
	if quote := strings.TrimSpace(page.Quote); quote != "" {
		return quoteLines(page) + block, supplied, total
	}
	return block, supplied, total
}

// Draft is a sheet as the human has filled it in, handed over by pressing
// "Ask about this" in that sheet's head. It is not a record and never becomes
// one here: the acts that write are the acts, and this is what is on screen
// while somebody decides whether to make one.
type Draft struct {
	// Sheet is what the sheet is called, as its head says it: "New goal".
	Sheet string `json:"sheet"`
	// Fields are what the human has written, in the order the sheet asks
	// them, with the empty ones already dropped by the page.
	Fields []DraftField `json:"fields,omitempty"`
}

// DraftField is one field of a sheet: what it is called, and what is in it.
type DraftField struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// draftPhrase is what a draft is called every time one appears, so that the
// Partner is never left to infer whether it is reading the ledger.
const draftPhrase = "a draft the human is filling in, not saved"

// maxDraftField is how much of one field of a draft travels. A sheet's field
// is a line or a paragraph; a human who pasted a chapter into one still gets
// a block, not a context spent on it.
const maxDraftField = 2000

// draftLines is the sheet a human offered, field by field, marked for what it
// is. A field holding a secret never reaches here: the one sheet that takes a
// secret has no "Ask about this" at all.
func draftLines(page Page) string {
	if page.Draft == nil {
		return ""
	}
	sheet := strings.TrimSpace(page.Draft.Sheet)
	if sheet == "" {
		return ""
	}
	var built strings.Builder
	built.WriteString("- The human offered the " + sheet + " sheet — " + draftPhrase + ":\n")
	for _, field := range page.Draft.Fields {
		value := strings.TrimSpace(field.Value)
		name := strings.TrimSpace(field.Name)
		if value == "" || name == "" {
			continue
		}
		if len(value) > maxDraftField {
			value = value[:maxDraftField] + "…"
		}
		if strings.Contains(value, "\n") {
			built.WriteString("  - " + name + ":\n" + quoted(value))
			continue
		}
		built.WriteString("  - " + name + ": " + value + "\n")
	}
	return built.String()
}

// quoteLines is the selected passage, with where it came from.
func quoteLines(page Page) string {
	from := strings.TrimSpace(page.QuoteFrom)
	if from == "" {
		from = page.Path
	}
	if revision := strings.TrimSpace(page.QuoteRevision); revision != "" {
		from += ", revision " + revision
	}
	if anchor := strings.TrimSpace(page.QuoteAnchor); anchor != "" {
		from += ", under " + anchor
	}
	return "- The human selected this passage, from " + from + ":\n" +
		quoted(page.Quote) + "\n"
}

// quoted marks a passage off from everything around it, so a document that
// happens to be written in list items cannot be read as this block's own.
func quoted(passage string) string {
	var built strings.Builder
	for _, line := range strings.Split(strings.TrimRight(passage, "\n"), "\n") {
		built.WriteString("  > " + line + "\n")
	}
	return built.String()
}

func pageLines(facts Facts, observed snapshot.Observation, page Page) (string, int, int) {
	switch {
	case page.Kind == KindDocument:
		return bounded(documentLines(facts, page)), 0, 0
	case page.Kind == KindGoal:
		return bounded(goalLines(observed, page)), 0, 0
	case page.Section == "Overview":
		return bounded(overviewLines(facts)), 0, 0
	case page.Section == "Decisions":
		block, supplied, total := decisionsLines(page)
		return bounded(block), supplied, total
	case page.Fleet != nil:
		return bounded(fleetLines(page)), 0, 0
	case page.Section == "Project":
		block, supplied, total := projectLines(facts, page)
		return bounded(block), supplied, total
	default:
		return boardLines(observed, page)
	}
}

// bounded is the ceiling on a block that is built whole. The board builds
// itself to the ceiling instead, because only it can say how many rows it left
// out.
func bounded(snapshot string) string {
	if len(snapshot) <= maxSnapshotBytes {
		return snapshot
	}
	cut := strings.LastIndex(snapshot[:maxSnapshotBytes], "\n")
	if cut <= 0 {
		cut = maxSnapshotBytes
	}
	return snapshot[:cut] + "\n- (the rest of what the page shows is longer than this block carries)\n"
}

// whereLines is the page's own answer about itself: section, address, tab,
// subject identity and the filters it is narrowed to.
func whereLines(page Page) string {
	var built strings.Builder
	write := func(label, value string) {
		if strings.TrimSpace(value) != "" {
			built.WriteString("- " + label + ": " + value + "\n")
		}
	}
	write("Section", page.Section)
	write("Address", page.Path)
	write("View", page.View)
	write("Tab", page.Tab)
	write("Done reaches back", page.Window)
	// A sheet open over the work area is where the human is standing, so it
	// belongs here and not among the facts of the page: it says what they are
	// looking at, and what is IN it travels only if they offered it.
	if sheet := strings.TrimSpace(page.Sheet); sheet != "" {
		write("Open sheet", sheet+" (open over the page; nothing in it is saved)")
	}
	if page.Subject != "" {
		kind := page.Kind
		if kind == "" {
			kind = "subject"
		}
		identity := page.Subject
		if page.Title != "" {
			identity += " — " + page.Title
		}
		write(strings.ToUpper(kind[:1])+kind[1:], identity)
	}
	if len(page.Filters) > 0 {
		write("Filters", strings.Join(page.Filters, "; "))
	}
	if built.Len() == 0 {
		return "- The page did not say where it is.\n"
	}
	return built.String()
}

/* ------------------------------------------------------------- the board -- */

// boardLines is the board as the human is looking at it: the lanes the page
// shows, in the page's own order, with the goals the page's filters and its
// Done window left in them — and each goal's facts read here, from the
// projection of the accepted tip, rather than taken from the page.
//
// The page says which goals are on screen because only the page knows: the
// filters, the ordering and the Done window are the browser's. The facts are
// this server's because only this server has them.
func boardLines(observed snapshot.Observation, page Page) (string, int, int) {
	if observed.Tip == "" && observed.State == "" {
		return "- This build has no ledger reader, so what the page shows cannot be described.\n", 0, 0
	}
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return "- The accepted ledger could not be read: " + observed.Message + "\n", 0, 0
	}
	board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	rows := map[string]backlog.Row{}
	for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
		rows[row.ID] = row
	}
	if len(page.Lanes) == 0 {
		return laneCounts(board) +
			"- The page did not say which goals it is showing, so these are the counts and not the rows.\n", 0, 0
	}

	var built strings.Builder
	built.WriteString("- The board, lane by lane, as this page is showing it:\n")
	shown, named := 0, 0
	for _, lane := range page.Lanes {
		shown += lane.Total
		built.WriteString("  - " + laneTitle(lane) + " (" + strconv.Itoa(lane.Total) + "):\n")
		for at, id := range lane.Goals {
			if at >= maxLaneRows || built.Len() >= maxSnapshotBytes {
				break
			}
			row, known := rows[id]
			if !known {
				built.WriteString("    - " + id + " — the accepted tip carries no such goal, so the page is showing something this server cannot see\n")
				named++
				continue
			}
			built.WriteString("    - " + rowLine(row) + "\n")
			named++
		}
	}
	if left := shown - named; left > 0 {
		built.WriteString("- " + strconv.Itoa(left) +
			" more goals are on the board than are named here; the board tool reads the rest, and takes a cursor.\n")
	}
	return built.String(), named, shown
}

// laneTitle is what the page calls the lane, and its id where it called it
// nothing.
func laneTitle(lane Lane) string {
	if title := strings.TrimSpace(lane.Title); title != "" {
		return title
	}
	if id := strings.TrimSpace(lane.ID); id != "" {
		return id
	}
	return "a lane the page did not name"
}

// laneCounts is the fallback for a page that named no rows: the projection's
// own count per lane, which is the whole board rather than what is on screen.
func laneCounts(board backlog.Board) string {
	var built strings.Builder
	built.WriteString("- Goals per lane at the accepted tip:\n")
	for _, lane := range backlog.LaneOrder {
		built.WriteString("  - " + string(lane) + ": " + strconv.Itoa(board.Counts[lane]) + "\n")
	}
	return built.String()
}

// rowLine is one goal as the board's card reads: which goal, what it is for,
// its tier, where it is ranked, where it stands, who holds it, and why it is
// waiting.
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
	if row.Waiting != nil {
		if reason := firstSentence(row.Waiting.Reason, maxIntent); reason != "" {
			parts = append(parts, "waiting: "+reason)
		}
	}
	return strings.Join(parts, " · ")
}

// firstSentence is one line of prose: up to the first full stop, and cut at
// the bound where even that is a paragraph.
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

/* -------------------------------------------------------------- one goal -- */

// goalLines is one goal's row exactly as the board shows it, and the ids of
// the goals beside it in its lane — the neighbours are what "the ones after
// this" and "the rest of Ready" mean on a goal's own page.
func goalLines(observed snapshot.Observation, page Page) string {
	if observed.Tip == "" && observed.State == "" {
		return "- This build has no ledger reader, so the displayed goal cannot be described.\n"
	}
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return "- The accepted ledger could not be read, so the displayed goal cannot be described.\n"
	}
	board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	all := append(append([]backlog.Row{}, board.Rows...), board.Closed...)
	var found *backlog.Row
	for index := range all {
		if all[index].ID == page.Subject {
			found = &all[index]
			break
		}
	}
	if found == nil {
		return "- The accepted ledger carries no goal " + page.Subject +
			", so what the page shows was read from somewhere this server cannot see.\n"
	}
	built := strings.Builder{}
	built.WriteString(fullRow(*found))
	neighbours := []string{}
	for _, row := range all {
		if row.Lane == found.Lane && row.ID != found.ID {
			neighbours = append(neighbours, row.ID)
		}
	}
	if len(neighbours) == 0 {
		built.WriteString("- It is the only goal in " + string(found.Lane) + ".\n")
		return built.String()
	}
	kept := neighbours
	if len(kept) > maxLaneRows {
		kept = kept[:maxLaneRows]
	}
	built.WriteString("- The other goals in " + string(found.Lane) + " (" +
		strconv.Itoa(len(neighbours)) + "): " + strings.Join(kept, ", "))
	if len(kept) < len(neighbours) {
		built.WriteString(", and " + strconv.Itoa(len(neighbours)-len(kept)) + " more")
	}
	built.WriteString("\n")
	return built.String()
}

// fullRow is one goal, field by field, as the board renders it.
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

/* -------------------------------------------------------- the other pages -- */

// overviewLines is the landing page's own numbers: what is waiting on this
// human, by kind, and the glance the rest of the page is.
func overviewLines(facts Facts) string {
	if facts.Overview == nil {
		return "- This build cannot compose the landing page, so what it shows cannot be described.\n"
	}
	page, err := facts.Overview()
	if err != nil {
		return "- The landing page could not be composed: " + err.Error() + "\n"
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
	if needs.SignIn {
		built.WriteString("  - nobody is signed in on this seat, so it cannot act\n")
	}
	built.WriteString("- Work now:\n")
	built.WriteString("  - in progress: " + strconv.Itoa(len(page.Work.InProgress)) + "\n")
	built.WriteString("  - next in Ready for Work: " + strconv.Itoa(len(page.Work.Next)) + "\n")
	for _, lane := range page.Work.Lanes {
		built.WriteString("  - " + lane.ID + ": " + strconv.Itoa(lane.Count) + "\n")
	}
	built.WriteString("- The window this page compares against opens " + page.Since + "\n")
	return built.String()
}

// fleetLines is the fleet as the PAGE was showing it, and only that.
//
// Every other block here is the server reading its own owners a moment after
// the question was asked. This one cannot be: a standing is a judgement made
// against a clock, and re-judging it here would answer "why is m1c
// unreachable" about a reading the human never saw. So the rows travel with
// the capture, the block says they are the page's, and the fleet tool is what
// the Partner calls when it wants a reading of its own.
func fleetLines(page Page) string {
	shown := page.Fleet
	var built strings.Builder
	built.WriteString("- The fleet as this page displayed it")
	if shown.Source != "" {
		built.WriteString(", from presence " + shown.Source)
	}
	if shown.FetchedAt != "" {
		built.WriteString(", fetched " + shown.FetchedAt)
	}
	built.WriteString(":\n")
	if shown.Problem != "" {
		built.WriteString("  - the copy's own trouble, as the page said it: " + shown.Problem + "\n")
	}
	for _, line := range shown.NeedsYou {
		built.WriteString("  - needs a human: " + line + "\n")
	}
	for _, machine := range shown.Machines {
		row := "  - " + machine.Machine + ": " + machine.Standing
		if machine.Seen != "" {
			row += ", seen " + machine.Seen
		}
		if len(machine.Holds) > 0 {
			row += ", holds " + strings.Join(machine.Holds, ", ")
		}
		if machine.Flag != "" {
			row += " (" + machine.Flag + ")"
		}
		built.WriteString(row + "\n")
	}
	if shown.Total > len(shown.Machines) {
		built.WriteString("  - " + strconv.Itoa(len(shown.Machines)) + " of " +
			strconv.Itoa(shown.Total) + " machines travelled with this question; the fleet tool reads the rest.\n")
	}
	// The launch cards, which are not machines yet: a launch in flight is a
	// clone being made, and a failed one is a directory on this host. The
	// human's authorization is not in any of these fields and never was.
	for _, launched := range shown.Launches {
		row := "  - launching " + launched.Machine + ": " + launched.Outcome
		if launched.Destination != "" {
			row += " into " + launched.Destination
		}
		if launched.Step != "" {
			row += ", at " + launched.Step
		}
		if launched.Words != "" {
			row += " — " + launched.Words
		}
		built.WriteString(row + "\n")
	}
	return built.String()
}

// decisionsLines is the Decisions page's inbox as the PAGE is showing it: one
// row per thing waiting on a human, with the kind, the id and what is being
// asked, in the page's own order.
//
// It is the page's list and not a second composition of it. Only the page
// knows what it put on the screen, and a server that composed its own a
// moment later would answer a question about a page nobody was looking at.
//
// A ruling's own words never travel. The register is a document of the
// checkout, and the documents tool reads it whole; a capture that carried a
// hundred and fifty rulings would spend the block on the one thing the
// Partner can already fetch.
func decisionsLines(page Page) (string, int, int) {
	if len(page.Records) == 0 {
		return "- The page did not say which rows its inbox is showing.\n", 0, 0
	}
	var built strings.Builder
	named := 0
	built.WriteString("- What needs this human's choice, as the page is showing it (" +
		strconv.Itoa(len(page.Records)) + "):\n")
	for at, row := range page.Records {
		if at >= maxListed {
			built.WriteString("  - and " + strconv.Itoa(len(page.Records)-maxListed) +
				" more rows on the page.\n")
			break
		}
		named++
		built.WriteString("  - " + row + "\n")
	}
	built.WriteString("- What this human decided is on the tab named above; the rulings are rows of " +
		"memory/rulings.md, which the documents tool reads whole.\n")
	return built.String(), named, len(page.Records)
}

// projectLines is the records the open tab lists, named by the page and read
// here: a tab is the browser's own selection, and what each record is is this
// server's.
func projectLines(facts Facts, page Page) (string, int, int) {
	if facts.Project == nil {
		return "- This build has no project reader, so what the page shows cannot be described.\n", 0, 0
	}
	pane, err := facts.Project()
	if err != nil {
		return "- The project's records could not be read: " + err.Error() + "\n", 0, 0
	}
	if len(page.Records) == 0 {
		return "- The page did not say which records this tab lists.\n" +
			"- The checkout declares " + strconv.Itoa(len(pane.Records)) + " records in all.\n", 0, 0
	}
	byPath := map[string]project.Record{}
	for _, record := range pane.Records {
		byPath[record.Path] = record
	}
	question := map[string]project.Question{}
	for _, asked := range pane.Questions {
		question[asked.ID] = asked
	}
	var built strings.Builder
	named := 0
	built.WriteString("- The records this tab is listing (" + strconv.Itoa(len(page.Records)) + "):\n")
	for at, key := range page.Records {
		if at >= maxListed {
			built.WriteString("  - and " + strconv.Itoa(len(page.Records)-maxListed) +
				" more; the records tool reads the rest, and takes a cursor.\n")
			break
		}
		named++
		if record, known := byPath[key]; known {
			line := record.Title
			if record.Status != "" {
				line += " · " + record.Status
			}
			if record.ID != "" {
				line += " · " + record.ID
			}
			built.WriteString("  - " + line + " (" + record.Path + ")\n")
			continue
		}
		if asked, known := question[key]; known {
			built.WriteString("  - " + firstSentence(asked.Question, maxIntent) + " · " + asked.Status + " · " + asked.ID + "\n")
			continue
		}
		built.WriteString("  - " + key + " — this server's own read of the project does not carry it\n")
	}
	return built.String(), named, len(page.Records)
}

// documentLines is a document's own revision, its record head, and its
// headings — the outline the reader shows beside it. It is the one page whose
// facts are in the files, and it says so.
func documentLines(facts Facts, page Page) string {
	if facts.Document == nil {
		return "- This build has no document reader, so the displayed document cannot be described.\n"
	}
	document, err := facts.Document(page.Subject)
	if err != nil {
		return "- The displayed document could not be read: " + err.Error() + "\n"
	}
	var built strings.Builder
	built.WriteString("- This page is a file in the checkout, read as it stands rather than from the accepted tip.\n")
	built.WriteString("- Document: " + document.ID + "\n")
	if document.Title != "" {
		built.WriteString("- Title: " + document.Title + "\n")
	}
	built.WriteString("- Displayed revision: " + displayedRevision(page.Revision, document.Revision) + "\n")
	built.WriteString("- Read at: " + document.ReadAt + "\n")
	if document.Record != nil {
		head := document.Record
		writeIf(&built, "Kind", head.Kind)
		writeIf(&built, "Record id", head.ID)
		writeIf(&built, "Status", head.Status)
		writeList(&built, "Goals", head.Goals)
		writeList(&built, "Cites", head.Cites)
		writeList(&built, "Affects", head.Affects)
		writeList(&built, "Governs", head.Governs)
		writeList(&built, "Supersedes", head.Supersedes)
	}
	if len(document.Headings) > 0 {
		built.WriteString("- Headings:\n")
		for _, heading := range document.Headings {
			line := "  " + strings.Repeat("  ", max(heading.Level-1, 0)) + "- " + heading.Text + "\n"
			if built.Len()+len(line) > maxSnapshotBytes {
				built.WriteString("  - (the rest of the outline is longer than this block carries)\n")
				break
			}
			built.WriteString(line)
		}
	}
	return built.String()
}

// displayedRevision prefers what the page said it was showing, because that is
// what the human read; the server's own read is the fallback and is named as
// such when the two differ.
func displayedRevision(displayed, read string) string {
	displayed = strings.TrimSpace(displayed)
	if displayed == "" {
		return read + " (this server's read; the page did not say)"
	}
	if read != "" && read != displayed {
		return displayed + " (the page's; the file now reads " + read + ", so it changed after the human loaded it)"
	}
	return displayed
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
