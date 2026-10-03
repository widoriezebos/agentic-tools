package httpd

// POST /api/fleet/lane/pause, /api/fleet/lane/resume and
// /api/fleet/machines/stop: the fleet panel's Pause, Resume and Stop as the
// signed-in person (fleet-panel-ux-step2.md slice 2b; ruling R-142-ui, which
// admits the session for these three verbs from this panel and nothing else).
//
// Each is a write under the policy every write takes (the allowed host, the
// same-site check, the single allowed origin, POST and nothing else, a bounded
// JSON object with no unknown field) and Land now's hand: a live session whose
// proof stands for this checkout, never the boot proof. Then the route asks
// act.SignedIn for the person, the call every goal act makes, which checks the
// name and the session are the ones that session was minted for (design D1).
// The seam is handed that person and nothing else, and runs the public verb in
// this process with it as the person the verb asks for: landing stop records
// them as who paused the lane, landing start and machine stop take them as the
// person their own check asks for.
//
// Stop names a machine, so it is refused before any stop runs when the name is
// not one word of letters, digits, '.', '-' and '_', and when admission (the
// verb's own reading of this computer's machines) refuses it: a name that is
// no machine of this computer, one that runs on another computer, and the
// checkout serving this page (S2-01), whose stop would end this server in the
// middle of the act. A machine already stopped is still this computer's, so a
// second tab's Stop reaches the verb and its unchanged success (S2-02).
//
// The answers are Land now's: the verb's envelope is 200 whatever its outcome,
// and so is admission's refusal, so both lines reach the page. The HTTP
// refusals are the route's own: no person (403, which opens the sign-in
// sheet), a body it does not take or a name that is not one word (400), and a
// seam that could not run or read at all (500, in words).

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
)

// The three addresses, and the route names the act table knows them by.
const (
	lanePausePath    = "/api/fleet/lane/pause"
	laneResumePath   = "/api/fleet/lane/resume"
	machineStopPath  = "/api/fleet/machines/stop"
	routeLanePause   = "lane-pause"
	routeLaneResume  = "lane-resume"
	routeMachineStop = "machine-stop"
)

// What a press with no person behind it is told; the browser opens the
// sign-in sheet on it and presses again once.
const (
	laneActNeedsSignIn     = "pausing or resuming the landing lane is a person's act, so it needs a signed-in human; " + signInRemedy
	machineStopNeedsSignIn = "stopping a machine ends its seat and every job on it, so it needs a signed-in human; " + signInRemedy
)

// signedPerson is the person a live session stands for, as act.SignedIn
// admits it for this checkout. false when the route has already answered.
func (h *handler) signedPerson(w http.ResponseWriter, r *http.Request, needs string) (string, bool) {
	signed, ok := h.sessionWith(w, r, needs)
	if !ok {
		return "", false
	}
	hand, err := act.SignedIn(h.info.Sessions.Root(), signed.Human, signed.Reference, signed.Proof)
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "session", err.Error())
		return "", false
	}
	return hand.Human(), true
}

// laneAct is Pause and Resume: the person, the empty body, the verb once.
func (h *handler) laneAct(w http.ResponseWriter, r *http.Request, run func(human string) (LandNowAnswer, error), missing string) {
	if run == nil {
		writeFailure(w, missing)
		return
	}
	human, ok := h.signedPerson(w, r, laneActNeedsSignIn)
	if !ok {
		return
	}
	var body struct{}
	if !decode(w, r, &body) {
		return
	}
	answered, err := run(human)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answered)
}

// stopMachine is Stop: the person, the one name, admission, then the verb.
func (h *handler) stopMachine(w http.ResponseWriter, r *http.Request) {
	if h.info.AdmitStop == nil || h.info.StopMachine == nil {
		writeFailure(w, "this engine cannot stop a machine from the interface; run metasystem machine stop at a terminal")
		return
	}
	human, ok := h.signedPerson(w, r, machineStopNeedsSignIn)
	if !ok {
		return
	}
	var body struct {
		Machine string `json:"machine"`
	}
	if !decode(w, r, &body) {
		return
	}
	if !board.SafeName(body.Machine) {
		writeRefusal(w, http.StatusBadRequest, "no machine was stopped: "+strconv.Quote(body.Machine)+" is not a machine's name, which is one word of letters, digits, '.', '-' and '_'", nil)
		return
	}
	refused, err := h.info.AdmitStop(body.Machine)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	if refused != nil {
		_ = json.NewEncoder(w).Encode(refused)
		return
	}
	answered, err := h.info.StopMachine(human, body.Machine)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answered)
}
