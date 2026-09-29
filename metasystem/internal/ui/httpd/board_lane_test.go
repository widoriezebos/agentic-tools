package httpd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// TestBoardCarriesTheLandingLane (U12): /api/board carries the host's
// landing lane as a top-level "lane", the one view landing status renders,
// read on every request; with no lane reader, and with an unreadable
// registry, the key is still there.
func TestBoardCarriesTheLandingLane(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	root := "/lanes/landing"
	reads := 0
	laneView := func(now time.Time) lane.View {
		reads++
		testutil.Expect(t, "the lane is read at the server's clock", now, fleetNow)
		return lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerRunning}, Summary: "landing lane " + root + ": owner running"}
	}
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute, Lane: laneView}}, loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var got lane.View
	testutil.Require(t, "lane", json.Unmarshal(payload["lane"], &got), nil)
	testutil.Expect(t, "lane summary", got.Summary, "landing lane /lanes/landing: owner running")
	testutil.Expect(t, "lane read once per request", reads, 1)

	bare := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute}}, loopback(), testBundle())
	payload = nil
	testutil.Require(t, "decode bare", json.Unmarshal(request(t, bare, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var none lane.View
	testutil.Require(t, "bare lane", json.Unmarshal(payload["lane"], &none), nil)
	testutil.Expect(t, "no lane reader: no root", none.Root == nil, true)
	testutil.Expect(t, "no lane reader: owner state", none.Owner.State, lane.OwnerNotStarted)
}
