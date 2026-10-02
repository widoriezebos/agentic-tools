package httpd

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
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
	laneView := func(now time.Time) plain.Status {
		reads++
		testutil.Expect(t, "the lane is read at the server's clock", now, fleetNow)
		return plain.Status{View: lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerRunning}, Summary: "landing lane " + root + ": owner running"}}
	}
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute, Lane: laneView}}, loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var got plain.Status
	testutil.Require(t, "lane", json.Unmarshal(payload["lane"], &got), nil)
	testutil.Expect(t, "a registered lane is an object", string(payload["lane"]) != "null", true)
	testutil.Expect(t, "lane summary", got.Summary, "landing lane /lanes/landing: owner running")
	testutil.Expect(t, "lane read once per request", reads, 1)

	bare := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute}}, loopback(), testBundle())
	payload = nil
	testutil.Require(t, "decode bare", json.Unmarshal(request(t, bare, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "no lane reader: lane is null", string(payload["lane"]), "null")

	unregistered := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Owner: lane.OwnerView{State: lane.OwnerUnready}, Summary: "no landing lane is registered"}}
			}}}, loopback(), testBundle())
	payload = nil
	testutil.Require(t, "decode unregistered", json.Unmarshal(request(t, unregistered, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "no lane registered: lane is null", string(payload["lane"]), "null")
}

// TestBoardLaneCarriesThePlainLane (goal fleet-card-can-land-now): the
// board's lane is landing status --json's data, field for field — whether
// the lane is paused and its agent alive, the queue, the running proof, the
// last proof and the last push — so the card reads what the terminal reads.
func TestBoardLaneCarriesThePlainLane(t *testing.T) {
	t.Parallel()
	root := "/lanes/landing"
	seats := func() ([]board.Seat, error) { return nil, nil }
	served := New(Info{Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: t.TempDir(), Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute,
			Lane: func(time.Time) plain.Status {
				return plain.Status{View: lane.View{Root: &root, Owner: lane.OwnerView{State: lane.OwnerIdle}, Summary: "landing lane " + root + ": idle"},
					Queue:     []plain.Entry{{Goal: "goal-a", Branch: "goal/goal-a", SHA: "abc", Seat: "m1e", State: plain.StateWaiting}},
					LastProof: &plain.Result{Tree: "t1", Commit: "c1", Result: plain.Green}, LastPush: &plain.Pushed{Old: "c0", Commit: "c1"}}
			}}}, loopback(), testBundle())
	var payload map[string]json.RawMessage
	testutil.Require(t, "decode", json.Unmarshal(request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	var carried map[string]json.RawMessage
	testutil.Require(t, "lane", json.Unmarshal(payload["lane"], &carried), nil)
	for _, key := range []string{"paused", "agent_alive", "queue", "running_proof", "last_proof", "last_push", "owner", "summary", "wake"} {
		_, present := carried[key]
		testutil.Expect(t, "the lane carries "+key, present, true)
	}
	var got plain.Status
	testutil.Require(t, "lane as status", json.Unmarshal(payload["lane"], &got), nil)
	testutil.Expect(t, "the queue as the reader read it", got.Queue, []plain.Entry{{Goal: "goal-a", Branch: "goal/goal-a", SHA: "abc", Seat: "m1e", State: plain.StateWaiting}})
	testutil.Expect(t, "the last proof", got.LastProof.Commit, "c1")
	testutil.Expect(t, "the last push", got.LastPush.Commit, "c1")
}
