package httpd

// The Fleet resource: who is doing what, and whether execution is healthy.
//
// It is one read of two sources the server already has — the presence copy
// this interface fetched for itself, and one observation of the accepted
// ledger with its board projection — composed into one shape by
// internal/ui/fleet. Nothing is decided here: this file gathers and hands
// them to Compose.
//
// One observation, taken once. The holders, the tip, the titles and the lanes
// all come out of it, so a holder named from one commit can never be shown
// beside a title read from another.

import (
	"encoding/json"
	"net/http"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// fleetPath is the fleet resource, matched exactly: what lies beneath it
// belongs to no resource, so it is a 404 like any other unserved path under a
// reserved prefix.
const fleetPath = "/api/fleet"

// fleet answers the Fleet page. A build with no fleet reader is a 500
// carrying the reason, like every other read: the reason is what a human acts
// on.
func (h *handler) fleet(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Fleet == nil {
		writeFailure(w, "this engine was built without a fleet reader")
		return
	}
	if h.info.Observe == nil {
		writeFailure(w, "this engine was built without a ledger reader")
		return
	}
	observed := h.info.Observe()
	board := backlog.Board{}
	if observed.State == snapshot.StateRead && observed.Tree != nil {
		board = backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
	}
	page, err := h.info.Fleet(observed, board, h.now())
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(page)
}

// holderOf is the flag one claimed row carries, joined from the same reading
// the page was composed from.
//
// It lives here rather than in the backlog package because presence is not
// the ledger's: a backlog row says who claimed a goal, and whether that
// machine has been heard from is a second reading this layer joins to it.
func holderOf(page fleet.Page, row backlog.Row) *holderPayload {
	if row.Claim == nil || row.Claim.Machine == "" {
		return nil
	}
	for _, machine := range page.Machines {
		if machine.Machine != row.Claim.Machine {
			continue
		}
		held := holderPayload{
			Machine: machine.Machine, Standing: machine.Standing, Since: machine.Since,
		}
		for _, hold := range machine.Holds {
			if hold.Goal == row.ID {
				held.Flag = hold.Flag
			}
		}
		return &held
	}
	return nil
}
