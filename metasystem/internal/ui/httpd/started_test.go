package httpd

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner/fakeacp"
)

// The question's answer carries the instant the turn was admitted beside its
// id, and the snapshot carries the same instant while the turn runs and ""
// once it ends (g1-s74 D4): the page's clock has one origin, the server's.
func TestTheAcceptedTurnAndTheSnapshotCarryOneStart(t *testing.T) {
	t.Parallel()
	hold := make(chan struct{})
	served, service := servedPartner(t, fakeacp.Script{Chunks: []string{"done"}, Hold: hold})
	events, stop := service.Subscribe()
	defer stop()

	accepted := post(t, served, partnerTurnsPath, `{"key":"k1","text":"how long?","about":{}}`, nil)
	testutil.Require(t, "accepted", accepted.Code, http.StatusAccepted)
	var body struct {
		Turn      string `json:"turn"`
		StartedAt string `json:"startedAt"`
	}
	testutil.Require(t, "the answer decodes", json.Unmarshal(accepted.Body.Bytes(), &body), nil)
	testutil.Expect(t, "the admit instant beside the turn", body.StartedAt, "2026-09-23T12:00:00Z")

	running := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "running", running.Busy, true)
	testutil.Expect(t, "the snapshot's start is the same", running.StartedAt, body.StartedAt)

	close(hold)
	drain(t, events)
	idle := partnerSnapshot(t, get(t, served, partnerPath, nil))
	testutil.Expect(t, "idle carries no start", idle.StartedAt, "")
}
