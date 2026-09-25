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
// The word never travels beyond this function's argument list. It is not
// written into the record, it is not in the answer, and it reaches nothing
// but the verb's own arguments.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

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

// launchNeedsAuthorization is what a launch with no word is refused with. The
// interface never launches a machine under an authorization nobody typed: a
// caller at a terminal may arm from their own enrolled terminal, and that
// path is the verb's, not this route's.
const launchNeedsAuthorization = "a machine launched from this interface is enrolled under your own words, so the word and the review date travel with every launch and every retry"

// launchBody is one launch as the sheet asks for it, or one retry.
//
// A retry carries the launch it resumes and the word again, because the
// record never held the word: the verb asks for it when a resume has to reach
// enrollment, and a browser that kept it would be a browser storing an
// authorization.
type launchBody struct {
	Machine     string `json:"machine"`
	Destination string `json:"destination"`
	Word        string `json:"word"`
	ReviewBy    string `json:"reviewBy"`
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
	// The word and the date travel with every launch and every retry. A
	// retry's enrollment may already be done — the verb decides that from the
	// clone's own identity and installed binary — but this side cannot read
	// either, and a retry that arrived without the pair and then reached the
	// arming step would be a launch this interface started and could not
	// finish.
	if strings.TrimSpace(body.Word) == "" || strings.TrimSpace(body.ReviewBy) == "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
		writeActRefusal(w, launch.CodeWordRequired, launchNeedsAuthorization)
		return
	}
	record, err := h.info.Launch(signed, launch.Request{
		Machine: body.Machine, Destination: body.Destination,
		Word: body.Word, ReviewBy: body.ReviewBy, Resume: body.Resume,
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
