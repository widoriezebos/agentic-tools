package httpd

// POST /api/fleet/launch: one machine of this fleet joins on this host.
//
// It is a write under the same policy every other write takes — the allowed
// host, the same-site check, the single allowed origin, POST and nothing
// else, a bounded JSON object with no unknown fields — and one thing no other
// write has: it needs a SIGNED-IN human and nothing weaker. The act layer's
// own gate still admits the boot proof on a seat with no Partner, and a
// launch spends disk, a build and the human's own credentials, so this route
// asks for the session itself.
//
// Signed in is enough (g1-s72): the body names the machine and where it
// lands, or the launch a retry resumes, and nothing else. The session this
// route admitted is what the starter stamps on the record as the new
// machine's enrollment.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// launchPath is the fleet's one write. routeLaunch names it.
const (
	launchPath  = "/api/fleet/launch"
	routeLaunch = "launch-machine"
)

// launchNeedsSignIn is what a request with no live session is told. The
// browser opens the sign-in sheet on it exactly as it does for the acts.
const launchNeedsSignIn = "launching a machine spends this host's disk and your own authorization, so it needs a signed-in human; " + signInRemedy

// launchNeedsProof is what a session whose proof does not stand for this
// checkout is told. It is a different fact from "nobody is signed in", and
// the remedy is the same one: sign in again, here.
const launchNeedsProof = "this session carries no signed-in proof for this checkout; " + signInRemedy

// launchBody is one launch as the sheet asks for it, {machine, destination},
// or one retry, {resume}. No word, no date and no day travel with either:
// the enrollment is the signed-in session's, stamped by the starter from the
// proof this route checked, never from anything the body says.
type launchBody struct {
	Machine     string `json:"machine"`
	Destination string `json:"destination"`
	Resume      string `json:"resume"`
}

func (h *handler) launchMachine(w http.ResponseWriter, r *http.Request) {
	if h.info.Launch == nil {
		writeFailure(w, "this engine cannot launch a machine")
		return
	}
	signed, liveness := h.signedIn(r)
	switch liveness {
	case session.Live:
	case session.Expired:
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "expired", expiredRefusal)
		return
	default:
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "launch", launchNeedsSignIn)
		return
	}
	// A live cookie is the store's bookkeeping; the proof is the human. This
	// route asks the proof itself, about the root it was minted for, rather
	// than taking a session record's liveness as the answer — which is what
	// the design means by the act checking the session outcome for itself.
	if signed == nil || h.info.Sessions == nil || !signed.Proof.SessionValidFor(h.info.Sessions.Root()) {
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "launch", launchNeedsProof)
		return
	}
	var body launchBody
	if !decode(w, r, &body) {
		return
	}
	record, err := h.info.Launch(signed, launch.Request{
		Machine: body.Machine, Destination: body.Destination, Resume: body.Resume,
	})
	if err != nil {
		var refusal *launch.Refusal
		if errors.As(err, &refusal) {
			w.WriteHeader(launchStatus(refusal.Code))
			writeActRefusal(w, refusal.Code, refusal.Message)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	// 202: the record is written and the verb is running. Everything after
	// this reaches the page through /api/fleet, which the launch record's own
	// changes are a cause of the `fleet` event for.
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(record)
}

// launchStatus says what a human can do about a refusal: a conflict is
// something on this host that is in the way and may not be tomorrow, and
// everything else is the request itself.
func launchStatus(code string) int {
	switch code {
	case launch.CodeRunning, launch.CodeNicknameTaken, launch.CodeDestinationExists, launch.CodeDiskShort:
		return http.StatusConflict
	default:
		return http.StatusUnprocessableEntity
	}
}
