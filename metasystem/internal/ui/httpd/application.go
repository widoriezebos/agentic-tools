package httpd

// The Application resource: what this workspace has concluded, what is known
// to be wrong with it, and what it says it is.
//
// It is one read of five answers this server already has — the workspace's own
// identity, the accepted ledger's projection, the known-issues register, this
// seat's own presence record as the Fleet page reads it, and the project's
// documents — composed into one shape by internal/ui/application. Nothing is
// decided here: this file gathers, records the visit, and hands them to
// Compose.
//
// It takes the same policy as every other read, for the same reason: it is a
// GET on loopback from this origin, and reaching this port already means being
// on this machine.

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/application"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// applicationPath is the application resource, matched exactly: what lies
// beneath it belongs to no resource, so it is a 404 like any other unserved
// path under a reserved prefix.
const applicationPath = "/api/application"

// application answers the Application page.
//
// The workspace, the ledger and the project are required: a page missing any
// of them would say what this workspace has concluded without being able to
// look, so a reader that fails is a 500 carrying the reason.
//
// The register, the presence record and the visit marker are not. A checkout
// with no known-issues register is an ordinary state of an adopted repository;
// a seat that has published no presence has no engine build to name, which the
// page says in words; and the marker is preference state whose loss widens a
// window and nothing else. None of them may fail silently: a register this
// seat could not read is a 500 like the other readers, because an empty
// Known problems block that looked like a project with no known defects is
// exactly the lie this page must not tell.
func (h *handler) application(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Describe == nil {
		writeFailure(w, "this engine was built without a workspace description")
		return
	}
	if h.info.Observe == nil {
		writeFailure(w, "this engine was built without a ledger reader")
		return
	}
	if h.info.Project == nil {
		writeFailure(w, "this engine was built without a project reader")
		return
	}
	described, err := h.info.Describe()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	pane, err := h.info.Project()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}

	now := h.now()
	standing := h.state(r)
	// The visit is recorded as part of answering, because reading the page IS
	// the visit, and it is recorded before the page is composed so that the
	// window the page is composed over is the window this read established.
	// It is this page's own entry: the landing page and Decisions keep their
	// own, and a read here must not move either.
	since, first := h.visitApplication(standing.Human, now)

	observed := h.info.Observe()
	board := backlogOf(observed)
	in := application.Inputs{
		Subject:   described.Subject,
		Mode:      described.Mode,
		Engine:    h.engineOf(observed, now),
		Closed:    plainRows(board.Closed),
		Documents: pane.Documents,
		// Where the register is from the checkout, which is the one root a
		// destination in this payload can be opened against.
		RegisterPath: h.info.KnownIssuesPath,
		Since:        since,
		First:        first,
	}
	if h.info.KnownIssues != nil {
		read, registerErr := h.info.KnownIssues()
		if registerErr != nil {
			writeFailure(w, registerErr.Error())
			return
		}
		in.Register = read
	}
	_ = json.NewEncoder(w).Encode(application.Compose(in, now))
}

// engineOf is this seat's own row of the fleet, reduced to the engine build it
// last published.
//
// It reads it through the Fleet composition rather than from the presence
// record directly, so that the build this page names and the build the Fleet
// page names are one reading of one record. A build with no fleet reader, a
// reading that failed, and a seat whose row carries no record at all all
// answer nil, which the page says in words rather than as a blank build.
func (h *handler) engineOf(observed snapshot.Observation, now time.Time) *application.Engine {
	if h.info.Fleet == nil {
		return nil
	}
	page, err := h.info.Fleet(observed, backlog.Board{}, now)
	if err != nil {
		return nil
	}
	for _, machine := range page.Machines {
		if !machine.This || machine.Engine == "" {
			continue
		}
		return &application.Engine{
			Build: machine.Engine, Generation: machine.Generation, PublishedAt: machine.Seen,
		}
	}
	return nil
}

// visitApplication records this read of the Application page and answers the
// window it compares against. A build with no marker store, and a marker that
// could not be read or written, both answer the first visit's window: the page
// still renders, over a day of history.
func (h *handler) visitApplication(human string, now time.Time) (time.Time, bool) {
	if h.info.VisitApplication == nil {
		return now.Add(-24 * time.Hour), true
	}
	since, first, err := h.info.VisitApplication(human, now)
	if err != nil {
		// The window came back beside the error and is the usable one; the
		// error is about the file, which nothing on the page depends on.
		return since, first
	}
	return since, first
}
