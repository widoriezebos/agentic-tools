package httpd

import (
	"errors"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/workspace"
)

// The landing gate in the interface (g1-s70 §6): the board's Review rows carry
// the gate's reading, computed from the goal's history and the layered
// settings; the card's Decide sheet performs land without a sitting under the
// sign-in; the room's Start holds the goal before it reports the sitting open
// and every end of the sitting releases it; and the Settings page reads the
// two settings with the source each came from.
const (
	landWithoutSittingSuffix = "/land-without-sitting"
	routeLandWithoutSitting  = "land-without-sitting"
)

type landWithoutSittingBody struct {
	Reason string `json:"reason"`
}

// gateSettings is the two settings as the layered resolution answers them.
func (h *handler) gateSettings() (goal.GateSettings, config.LandingGate, bool) {
	if h.info.LandingGate == nil {
		return goal.GateSettings{}, config.LandingGate{}, false
	}
	resolved, err := h.info.LandingGate()
	if err != nil {
		return goal.GateSettings{}, config.LandingGate{}, false
	}
	return goal.GateSettings{HumanFromTier: resolved.HumanFromTier, AutoAfter: resolved.AutoAfter, AutoAfterText: resolved.After.Value}, resolved, true
}

// joinGates fills the gate's reading on the payload's Review rows. Settings
// this seat cannot read cost the rows their reading and nothing else.
func (h *handler) joinGates(observed snapshot.Observation, payload *backlogPayload) {
	settings, _, ok := h.gateSettings()
	if !ok || observed.Tree == nil {
		return
	}
	rows := plainRows(payload.Rows)
	backlog.JoinGates(rows, observed.Tree, settings, h.now())
	for index := range payload.Rows {
		payload.Rows[index].Gate = rows[index].Gate
	}
}

// landWithoutSitting performs the decision from the card's Decide sheet: the
// goal branch's tip at origin, the reason the human wrote, under the sign-in.
func (h *handler) landWithoutSitting(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.LandWithoutSitting == nil || h.info.Review == nil {
		writeFailure(w, "this engine was built without the landing gate's act")
		return
	}
	signed, ok := h.sessionFor(w, r)
	if !ok {
		return
	}
	var body landWithoutSittingBody
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Reason) == "" {
		h.refuseAct(w, &act.Refusal{Kind: act.KindRequest, Code: "reason", Message: "land without a sitting carries your reason: say why this goal needs no sitting"})
		return
	}
	tip, err := h.info.Review.BranchTip(id)
	if err != nil {
		h.refuseAct(w, &act.Refusal{Kind: act.KindRequest, Code: "no-branch", Message: "goal/" + id + " has no tip to let land: " + err.Error()})
		return
	}
	h.answerAct(w, r, h.info.LandWithoutSitting(signed, id, tip, strings.TrimSpace(body.Reason)))
}

// gateOf is the gate's reading of one goal on the board, or nil.
func (h *handler) gateOf(goalID string) *backlog.Gate {
	if h.info.Observe == nil {
		return nil
	}
	observed := h.info.Observe()
	payload := backlogOf(observed)
	h.joinGates(observed, &payload)
	for _, row := range payload.Rows {
		if row.ID == goalID {
			return row.Gate
		}
	}
	return nil
}

// holdForSitting writes the hold a review sitting on a waiting goal takes,
// before the room reports it open, and answers whether the route may go on.
func (h *handler) holdForSitting(w http.ResponseWriter, r *http.Request, goalID, record string) bool {
	if h.info.Sitting == nil {
		// A build without the gate's acts holds nothing, as it reads no gate.
		return true
	}
	signed, ok := h.sessionFor(w, r)
	if !ok {
		return false
	}
	return h.answerSitting(w, h.info.Sitting(signed, goalID, record, true))
}

// releaseForSitting releases the hold a review sitting on record stands on,
// where one of the goal's holds names that record: every way the sitting ends
// performs it. It answers whether the route may go on.
func (h *handler) releaseForSitting(w http.ResponseWriter, r *http.Request, record string) bool {
	if h.info.Document == nil || h.info.Sitting == nil {
		return true
	}
	document, err := h.info.Document(record)
	if err != nil || document.Record == nil || document.Record.Kind != "review" || len(document.Record.Goals) == 0 {
		return true
	}
	goalID := document.Record.Goals[0]
	gate := h.gateOf(goalID)
	if gate == nil || !holdsOn(gate, record) {
		return true
	}
	signed, ok := h.sessionFor(w, r)
	if !ok {
		return false
	}
	return h.answerSitting(w, h.info.Sitting(signed, goalID, record, false))
}

// holdsOn says one of the gate's holds is on the record the room sits on. The
// ledger names a record from the installation's root and the room from the
// checkout's, so a record the room names ends in the ledger's path.
func holdsOn(gate *backlog.Gate, record string) bool {
	for _, hold := range gate.HeldBy {
		if hold.Record == record || strings.HasSuffix(record, "/"+hold.Record) {
			return true
		}
	}
	return false
}

func (h *handler) answerSitting(w http.ResponseWriter, err error) bool {
	if err == nil {
		return true
	}
	var refusal *act.Refusal
	if errors.As(err, &refusal) {
		h.refuseAct(w, refusal)
		return false
	}
	writeFailure(w, err.Error())
	return false
}

// landingFacts are the two settings as the Settings page shows them, each
// with the source the resolution reports.
func (h *handler) landingFacts(described *workspace.Workspace) {
	if h.info.LandingGate == nil {
		return
	}
	resolved, err := h.info.LandingGate()
	if err != nil {
		described.LandingGate = &workspace.LandingGate{Problem: err.Error()}
		return
	}
	described.LandingGate = &workspace.LandingGate{Facts: []config.LandingGateFact{resolved.Tier, resolved.After}}
}
