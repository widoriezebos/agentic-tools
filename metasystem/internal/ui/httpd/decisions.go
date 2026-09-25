package httpd

// The Decisions resource: what needs a human's choice, and what they decided.
//
// It is one read of six answers this server already has — the project's
// records, the accepted ledger's projection, the steward's journal, this
// seat's open channel questions, the rulings register, and who the server is
// acting as — composed into one shape by internal/ui/decisions. Nothing is
// decided here: this file gathers and hands the six to Compose.
//
// It takes the same policy as every other read, for the same reason: it is a
// GET on loopback from this origin, and reaching this port already means being
// on this machine.

import (
	"encoding/json"
	"net/http"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/decisions"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
)

// decisionsPath is the decisions resource, matched exactly: what lies beneath
// it belongs to no resource, so it is a 404 like any other unserved path under
// a reserved prefix.
const decisionsPath = "/api/decisions"

// decisions answers the page a human rules from.
//
// The records and the ledger are required: a page missing either would be a
// page that says nothing needs you because it could not look, so a reader that
// fails is a 500 carrying the reason.
//
// The other three are not. A seat with no channel questions on disk, a
// checkout with no rulings register, and a build with no journal are all
// ordinary states of an adopted repository rather than failures, and each one
// costs the page one block and nothing else. What they may not do is fail
// silently: a register this seat could not read is a defect line on the page,
// in the reader's own words, rather than an empty Rulings tab that looks like
// a project that has decided nothing.
func (h *handler) decisions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Project == nil {
		writeFailure(w, "this engine was built without a project reader")
		return
	}
	if h.info.Observe == nil {
		writeFailure(w, "this engine was built without a ledger reader")
		return
	}
	pane, err := h.info.Project()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	journal := []notifications.Notice{}
	if h.info.NotificationJournal != "" {
		read, journalErr := notifications.Page(h.info.NotificationJournal, notifications.DefaultLimit, "")
		if journalErr != nil {
			writeFailure(w, journalErr.Error())
			return
		}
		journal = read
	}

	board := backlogOf(h.info.Observe())
	in := decisions.Inputs{
		Project: pane,
		Rows:    plainRows(board.Rows),
		Closed:  plainRows(board.Closed),
		Journal: journal,
		// Where the register is from the checkout, which is the one root a
		// destination in this payload can be opened against.
		RegisterPath: h.info.RegisterPath,
		Human:        decisions.Standing{Proven: h.state(r).SignedIn},
	}
	if h.info.Asks != nil {
		asked, asksErr := h.info.Asks()
		if asksErr != nil {
			writeFailure(w, asksErr.Error())
			return
		}
		in.Asks = asked
	}
	if h.info.Rulings != nil {
		read, registerErr := h.info.Rulings()
		if registerErr != nil {
			writeFailure(w, registerErr.Error())
			return
		}
		in.Register = read
	}
	_ = json.NewEncoder(w).Encode(decisions.Compose(in, h.now()))
}
