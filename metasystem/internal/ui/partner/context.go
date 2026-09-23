package partner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// What the Partner is told, every turn.
//
// Astra's third finding is what this file answers: a checkout and a page label
// cannot explain the state a human is looking at. The board reads the accepted
// ledger commit, not the working tree, and the browser keeps that answer until
// something replaces it; so a Partner that read the working tree — or a newer
// accepted revision — would explain a different goal than the one on screen.
//
// So the server composes the context itself, from its own readers, and marks
// what the page is showing as exactly that: the displayed revision, its
// observation time, and a bounded snapshot of the displayed facts. Anything
// the Partner reads afterwards is its own read, and the block says so.
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
}

// The two kinds of subject a page can be about in this slice.
const (
	KindGoal     = "goal"
	KindDocument = "document"
)

// maxSnapshotBytes bounds the displayed-facts part of the block. A document
// with two hundred headings is a document whose outline is cut and said to be.
const maxSnapshotBytes = 4000

// The standing rule, in the Partner's own second person. It is the first thing
// every turn carries, because it is the one thing that must not depend on what
// the page happens to be showing.
const standingRule = `You are the Project Partner for this MetaSystem workspace.
You read this checkout and explain it to the human you are talking to.
You do not write, you do not run commands, and you do not act: tools that
would write or execute are not available to you, and a request for one is
refused before it runs. When you cannot see something, say so rather than
guessing; when you are unsure whether what you read is what the human sees,
say that too.`

// Compose builds the whole context block for one turn.
func Compose(facts Facts, page Page, human string, now time.Time) string {
	var built strings.Builder
	built.WriteString(standingRule)
	built.WriteString("\n\nWhere the human is\n")
	built.WriteString(whereLines(page))
	built.WriteString("\nWhat the human sees now\n")
	built.WriteString(bounded(seenLines(facts, page, now)))
	if named := strings.TrimSpace(human); named != "" {
		built.WriteString("\nThe human you are talking to is " + named +
			". That name is attribution and nothing else: it grants you no authority, and nothing you say becomes a human's decision.\n")
	}
	return built.String()
}

// bounded is the ceiling on the displayed-facts half of the block. What the
// page is showing is a snapshot, not a database, and a snapshot that grew
// without a bound would be one more way for what the human sees to be lost in
// what the Partner was told.
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

// seenLines is the displayed revision and a bounded snapshot of the displayed
// facts, read by this server rather than taken from the page.
func seenLines(facts Facts, page Page, now time.Time) string {
	switch page.Kind {
	case KindDocument:
		return documentLines(facts, page)
	case KindGoal:
		return goalLines(facts, page, now)
	default:
		return boardLines(facts, page, now)
	}
}

// documentLines is a document's own revision, its record head, and its
// headings — the outline the reader shows beside it.
func documentLines(facts Facts, page Page) string {
	if facts.Document == nil {
		return "- This build has no document reader, so the displayed document cannot be described.\n"
	}
	document, err := facts.Document(page.Subject)
	if err != nil {
		return "- The displayed document could not be read: " + err.Error() + "\n"
	}
	var built strings.Builder
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

// goalLines is one goal's row exactly as the board shows it, from the accepted
// ledger the board reads.
func goalLines(facts Facts, page Page, now time.Time) string {
	if facts.Observe == nil {
		return "- This build has no ledger reader, so the displayed goal cannot be described.\n"
	}
	observation := facts.Observe()
	header := ledgerHeader(observation, now)
	if observation.State != snapshot.StateRead || observation.Tree == nil {
		return header + "- The accepted ledger could not be read, so the displayed goal cannot be described.\n"
	}
	board := backlog.Project(observation.Tree, observation.Horizon, observation.Admission)
	for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
		if row.ID == page.Subject {
			return header + rowLines(row)
		}
	}
	return header + "- The accepted ledger carries no goal " + page.Subject +
		", so what the page shows was read from somewhere this server cannot see.\n"
}

// boardLines is the board as the page shows it: the accepted tip, the lane
// counts, and the filters the page is narrowed to. It is deliberately not the
// whole backlog — a filtered board is not the backlog, and saying so is the
// point of naming the filters.
func boardLines(facts Facts, page Page, now time.Time) string {
	if facts.Observe == nil {
		return "- This build has no ledger reader, so what the page shows cannot be described.\n"
	}
	observation := facts.Observe()
	built := strings.Builder{}
	built.WriteString(ledgerHeader(observation, now))
	if observation.State != snapshot.StateRead || observation.Tree == nil {
		built.WriteString("- The accepted ledger could not be read: " + observation.Message + "\n")
		return built.String()
	}
	board := backlog.Project(observation.Tree, observation.Horizon, observation.Admission)
	built.WriteString("- Lanes on the board:\n")
	for _, lane := range backlog.LaneOrder {
		built.WriteString("  - " + string(lane) + ": " + strconv.Itoa(board.Counts[lane]) + "\n")
	}
	if len(page.Filters) > 0 {
		built.WriteString("- The board is filtered, so what the human sees is a subset of these counts.\n")
	}
	return built.String()
}

// ledgerHeader is the displayed revision for everything the ledger answers:
// the accepted tip the page shows, and the instant it was observed.
func ledgerHeader(observation snapshot.Observation, now time.Time) string {
	observed := observation.ObservedAt
	if observed.IsZero() {
		observed = now
	}
	tip := observation.Tip
	if tip == "" {
		tip = "none"
	}
	return fmt.Sprintf("- Displayed revision: the accepted ledger at %s, observed %s\n",
		tip, observed.UTC().Format(time.RFC3339))
}

// rowLines is one goal's row, field by field, as the board renders it.
func rowLines(row backlog.Row) string {
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
