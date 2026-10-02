package httpd

// POST /api/fleet/land-now: the landing lane card's Land now (goal
// fleet-card-can-land-now; Wido 2026-10-01: "I want a verb that does that
// (from any seat) and from the UI especially").
//
// It runs `metasystem landing run` once, through the seam Info.LandNow, and
// answers the verb's own one-result envelope. The verb decides everything the
// keeper decides — work queued, the lane not paused, no agent alive, one agent
// per computer — and a press while an agent runs is the verb's success that
// started nothing (R-129-ui), so this route judges none of it and adds no
// state of its own.
//
// It is a write under the policy every write takes — the allowed host, the
// same-site check, the single allowed origin, POST and nothing else, a bounded
// JSON object with no unknown fields (the body is `{}`) — and the launch
// route's hand: a live session whose proof stands for this checkout, never the
// boot proof. Starting the landing agent spends this host's model and proof
// time under the human's session.
//
// The verb's answer is 200 whatever its outcome: a refusal is two lines the
// card shows (what happened, and the one thing to run), and the browser's
// error path keeps one line. The HTTP refusals are this route's own: no
// session (403, which opens the sign-in sheet), a body that is not `{}`, and a
// verb that could not be run or read at all (500, in words).

import (
	"encoding/json"
	"net/http"
)

// landNowPath is the lane card's one act. routeLandNow names it.
const (
	landNowPath  = "/api/fleet/land-now"
	routeLandNow = "land-now"
)

// landNowNeedsSignIn is what a press with no live session is told; the
// browser opens the sign-in sheet on it and presses again once.
const landNowNeedsSignIn = "starting the landing agent spends this host's model and proof time under your name, so it needs a signed-in human; " + signInRemedy

// LandNowNext is the verb's line 2: the one command that does what needs
// doing, and why. An empty Argv is the reason alone.
type LandNowNext struct {
	Argv   []string `json:"argv"`
	Reason string   `json:"reason"`
}

// LandNowAnswer is landing run's one-result envelope, reduced to what a
// person reads: its outcome (confirmed, unchanged, refused, failed), its
// summary — line 1 — and its next step — line 2, null where it names none.
type LandNowAnswer struct {
	Outcome string       `json:"outcome"`
	Summary string       `json:"summary"`
	Next    *LandNowNext `json:"next"`
}

func (h *handler) landNow(w http.ResponseWriter, r *http.Request) {
	if h.info.LandNow == nil {
		writeFailure(w, "this engine cannot start the landing agent from the interface")
		return
	}
	if _, ok := h.sessionWith(w, r, landNowNeedsSignIn); !ok {
		return
	}
	var body struct{}
	if !decode(w, r, &body) {
		return
	}
	answered, err := h.info.LandNow()
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answered)
}
