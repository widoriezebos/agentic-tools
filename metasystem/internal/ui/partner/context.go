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
const standingRule = `You are the Project Partner for this MetaSystem workspace.
You read this checkout and explain it to the human you are talking to.
You do not write, you do not run commands, and you do not act: tools that
would write or execute are not available to you, and a request for one is
refused before it runs. When you cannot see something, say so rather than
guessing; when you are unsure whether what you read is what the human sees,
say that too. The ledger of goals is not in the files you can read; what you
are told here about goals is the whole of what you know about them.`

// Compose builds the whole context block for one turn.
func Compose(facts Facts, page Page, human string, now time.Time) string {
	observed := snapshot.Observation{}
	if facts.Observe != nil {
		observed = facts.Observe()
	}
	var built strings.Builder
	built.WriteString(standingRule)
	built.WriteString("\n\nWhere the human is\n")
	built.WriteString(whereLines(page))
	built.WriteString("\n" + seenHeading(observed, now) + "\n")
	built.WriteString(seenLines(facts, observed, page))
	if named := strings.TrimSpace(human); named != "" {
		built.WriteString("\nThe human you are talking to is " + named +
			". That name is attribution and nothing else: it grants you no authority, and nothing you say becomes a human's decision.\n")
	}
	return built.String()
}

// seenHeading marks the block, and marks it with the revision the page is
// showing: everything the Partner reads afterwards is its own read of the
// checkout, and the two are not the same thing.
func seenHeading(observed snapshot.Observation, now time.Time) string {
	if observed.Tip == "" {
		return "What the human sees now (this seat could not read an accepted ledger)"
	}
	at := observed.ObservedAt
	if at.IsZero() {
		at = now
	}
	return fmt.Sprintf("What the human sees now, from the accepted tip %s observed %s",
		observed.Tip, at.UTC().Format(time.RFC3339))
}

// seenLines is the page itself, from the server's own readers.
func seenLines(facts Facts, observed snapshot.Observation, page Page) string {
	switch {
	case page.Kind == KindDocument:
		return bounded(documentLines(facts, page))
	case page.Kind == KindGoal:
		return bounded(goalLines(observed, page))
	case page.Section == "Overview":
		return bounded(overviewLines(facts))
	case page.Section == "Project":
		return bounded(projectLines(facts, page))
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
	write("Tab", page.Tab)
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
func boardLines(observed snapshot.Observation, page Page) string {
	if observed.Tip == "" && observed.State == "" {
		return "- This build has no ledger reader, so what the page shows cannot be described.\n"
	}
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return "- The accepted ledger could not be read: " + observed.Message + "\n"
	}
	board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	rows := map[string]backlog.Row{}
	for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
		rows[row.ID] = row
	}
	if len(page.Lanes) == 0 {
		return laneCounts(board) +
			"- The page did not say which goals it is showing, so these are the counts and not the rows.\n"
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
			" more goals are on the board than are named here; ask for a lane by name and what is named above is what I know.\n")
	}
	return built.String()
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

// projectLines is the records the open tab lists, named by the page and read
// here: a tab is the browser's own selection, and what each record is is this
// server's.
func projectLines(facts Facts, page Page) string {
	if facts.Project == nil {
		return "- This build has no project reader, so what the page shows cannot be described.\n"
	}
	pane, err := facts.Project()
	if err != nil {
		return "- The project's records could not be read: " + err.Error() + "\n"
	}
	if len(page.Records) == 0 {
		return "- The page did not say which records this tab lists.\n" +
			"- The checkout declares " + strconv.Itoa(len(pane.Records)) + " records in all.\n"
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
	built.WriteString("- The records this tab is listing (" + strconv.Itoa(len(page.Records)) + "):\n")
	for at, key := range page.Records {
		if at >= maxListed {
			built.WriteString("  - and " + strconv.Itoa(len(page.Records)-maxListed) + " more\n")
			break
		}
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
	return built.String()
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
