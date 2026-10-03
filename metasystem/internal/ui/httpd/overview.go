package httpd

// The Overview resource: the page a human lands on.
//
// It is one read of five answers this server already has — the project's
// records, the accepted ledger's projection, the steward's journal, who the
// server is acting as, and this human's visit marker — composed into one
// shape by internal/ui/overview. Nothing is decided here: this file gathers,
// records the visit, and hands the five to Compose.
//
// It takes the same policy as every other read, for the same reason: it is a
// GET on loopback from this origin, and reaching this port already means being
// on this machine.

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// overviewPath is the overview resource, matched exactly: what lies beneath it
// belongs to no resource, so it is a 404 like any other unserved path under a
// reserved prefix.
const overviewPath = "/api/overview"

// overview answers the landing page.
//
// Every part of it is required: a page missing its records or its ledger would
// be a page that says nothing needs you because it could not look. So a reader
// that fails is a 500 carrying the reason, like the other reads, and the
// browser shows the reason rather than a calm page that is a lie.
//
// The visit marker is the one exception. It is preference state, and losing it
// widens the comparison window and nothing else, so a marker that could not be
// read or written is a window and not a refusal.
func (h *handler) overview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	now := h.now()
	standing := h.state(r)
	read, err := h.gather(h.partnerHuman(r), standing.SignedIn, now)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	// The visit is recorded as part of answering, because reading the page IS
	// the visit. It is recorded before the page is composed so that the
	// window the page is composed over is the window this read established.
	since, first := h.visit(standing.Human, now)
	_ = json.NewEncoder(w).Encode(h.overviewOf(read, since, first, now))
}

// overviewOf composes the landing page over one gathered reading. "Needs you"
// is the Decisions inbox composed from that same reading at the same instant,
// so the page's total is the count the Decisions page gives.
func (h *handler) overviewOf(read gathered, since time.Time, first bool, now time.Time) overview.Page {
	return overview.Compose(overview.Inputs{
		Project: read.in.Project,
		Rows:    read.in.Rows,
		Closed:  read.in.Closed,
		Counts:  read.board.Counts,
		Ledger:  ledgerFor(read.board),
		Journal: read.in.Journal,
		Inbox:   decisions.ForOverview(decisions.Inbox(read.in, now)),
		Stages:  h.stages(),
		Holders: h.holders(read.observed, read.in.Rows, now),
		Human:   overview.Standing{Proven: read.in.Human.Proven},
		Since:   since,
		First:   first,
	}, now)
}

// stages is the stage of every goal the host board believes a card for, read
// through the view the board resource serves and worded as the board words it.
// A card the classifier calls Unknown names no stage, and a build with no board
// reader names none at all, which leaves each In Progress row with the
// ledger's own phase.
func (h *handler) stages() map[string]string {
	source := h.info.Board
	if source == nil || source.Seats == nil {
		return nil
	}
	held := map[string]string{}
	for _, seat := range h.boardView(source).Seats {
		for _, card := range seat.Goals {
			if card.Unknown == "" && card.Stage != "" {
				held[card.Goal] = board.StageText(card.Stage, card.Round, card.Proof)
			}
		}
	}
	return held
}

// holders is the presence standing of every machine the board's rows name,
// read from the same observation those rows were projected from.
//
// It is best effort for the board's reason: a presence copy this seat cannot
// read costs the In Progress rows a flag beside the seat and nothing else.
func (h *handler) holders(observed snapshot.Observation, rows []backlog.Row, now time.Time) map[string]overview.Holder {
	if h.info.Fleet == nil {
		return nil
	}
	page, err := h.info.Fleet(observed, backlog.Board{Rows: rows}, now)
	if err != nil {
		return nil
	}
	held := map[string]overview.Holder{}
	for _, machine := range page.Machines {
		flag := ""
		for _, hold := range machine.Holds {
			if hold.Flag != "" {
				flag = hold.Flag
				break
			}
		}
		held[machine.Machine] = overview.Holder{
			Machine: machine.Machine, Standing: machine.Standing,
			Since: machine.Since, Flag: flag,
		}
	}
	return held
}

// now is this server's clock, or the one a test handed it.
func (h *handler) now() time.Time {
	if h.info.Now == nil {
		return time.Now().UTC()
	}
	return h.info.Now().UTC()
}

// visit records this read and answers the window. A build with no marker
// store, and a marker that could not be read or written, both answer the
// first visit's window: the page still renders, over a day of history.
func (h *handler) visit(human string, now time.Time) (time.Time, bool) {
	if h.info.Visit == nil {
		return now.Add(-24 * time.Hour), true
	}
	since, first, err := h.info.Visit(human, now)
	if err != nil {
		// The window came back beside the error and is the usable one; the
		// error is about the file, which nothing on the page depends on.
		return since, first
	}
	return since, first
}

// ledgerFor reduces the board's own statement to the facts the page judges
// health on, so that Overview and the Backlog cannot disagree about whether
// the ledger is where it should be: both read the same projection, and both
// read the one freshness judgement backlogOf made.
//
// SyncedAt is the last fetch that landed rather than the last fetch that
// started, because that is the instant the calm line claims about: a tick in
// flight has no finish yet, and dating the page from a look that has not
// answered would be dating it from nothing.
func ledgerFor(board backlogPayload) overview.Ledger {
	ledger := overview.Ledger{
		Freshness: board.Ledger.Freshness.State,
		AtTip: board.Ledger.State == string(snapshot.StateRead) &&
			board.Ledger.Freshness.State == snapshot.FreshnessCurrent,
	}
	if at, err := time.Parse(time.RFC3339, board.Ledger.Fetch.SucceededAt); err == nil {
		ledger.SyncedAt = at
	}
	ledger.Statement = ledgerStatement(board.Ledger)
	return ledger
}

// ledgerStatement is the engine's own words for what is wrong, in the order a
// human can act on them: what this interface's freshness found about its own
// fetch loop, then what the ledger reader found in the tree it read. An empty
// answer leaves the composing package to say the plain thing.
func ledgerStatement(read ledgerPayload) string {
	if read.Freshness.State != snapshot.FreshnessCurrent && read.Freshness.Detail != "" {
		return read.Freshness.Detail
	}
	if read.Message != "" {
		return read.Message
	}
	return ""
}

// humanDuration says a duration the way a person would: "30 minutes",
// "2 hours", "1 hour 30 minutes", "45 seconds". Go's own form, "30m0s", is
// for logs.
func humanDuration(d time.Duration) string {
	d = d.Round(time.Second)
	hours, minutes, seconds := int(d/time.Hour), int(d%time.Hour/time.Minute), int(d%time.Minute/time.Second)
	unit := func(n int, word string) string {
		if n == 1 {
			return "1 " + word
		}
		return strconv.Itoa(n) + " " + word + "s"
	}
	switch {
	case hours > 0 && minutes > 0:
		return unit(hours, "hour") + " " + unit(minutes, "minute")
	case hours > 0:
		return unit(hours, "hour")
	case minutes > 0:
		return unit(minutes, "minute")
	default:
		return unit(seconds, "second")
	}
}
