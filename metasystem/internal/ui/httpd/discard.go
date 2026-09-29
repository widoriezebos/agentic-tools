package httpd

// POST /api/fleet/launches/{id}/discard: a stopped launch goes out of sight.
//
// It marks the launch record discarded and deletes nothing — not the record,
// which is kept for the trail, and not the clone the launch made, which stays
// on disk until a human removes it. That is why the page asks for no
// confirmation, and why this route asks for no more than the checkout's own
// writes do: it is a request from this browser to this loopback server that
// lands in the checkout and records no authority. It takes the policy every
// write takes — the allowed host, the same-site check, the single allowed
// origin, POST and nothing else, a body that is the empty object.
//
// It is idempotent: discarding a discarded launch changes nothing and
// succeeds, so a second press, or a second tab, is not an error.

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
)

// The collection a launch is named beneath, and the one act on a member.
const (
	launchesPrefix     = "/api/fleet/launches/"
	discardSuffix      = "/discard"
	routeDiscardLaunch = "discard-launch"
)

func (h *handler) discardLaunch(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.DiscardLaunch == nil {
		writeFailure(w, "this engine cannot discard a launch")
		return
	}
	var body struct{}
	if !decode(w, r, &body) {
		return
	}
	record, err := h.info.DiscardLaunch(id)
	if err != nil {
		var refusal *launch.Refusal
		if errors.As(err, &refusal) {
			w.WriteHeader(discardStatus(refusal.Code))
			writeActRefusal(w, refusal.Code, refusal.Message)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(record)
}

// discardStatus says what a refusal is: a launch that is not there, one that
// is still running and may not be in a minute, or a request naming no launch.
func discardStatus(code string) int {
	switch code {
	case launch.CodeUnknown:
		return http.StatusNotFound
	case launch.CodeDiscardRunning:
		return http.StatusConflict
	default:
		return http.StatusUnprocessableEntity
	}
}
