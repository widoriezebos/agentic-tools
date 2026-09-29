package httpd

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// boardProber reports pid 41 alive with start 1000 and every other pid dead.
type boardProber struct{}

func (boardProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid == 41 {
		return identity.Exact{Pid: pid, StartedAt: time.Unix(1000, 0)}, identity.Alive, nil
	}
	return identity.Exact{}, identity.Dead, nil
}

// TestBoardRouteServesTheClassifiedPicture (R23, R24, U10d): /api/board
// reads the host board on request and classifies it in the server with its
// own prober and clock, checked against the ledger observation this server
// already reads: a live card under its claim holder is underway, a card
// whose owner is dead and a card for a goal nobody claims are Unknown with
// their reasons; the payload carries one line per seat, and bridge absent
// with no socket; an unreadable registry is an unreadable board, not an
// empty one; a build without a board reader says so.
func TestBoardRouteServesTheClassifiedPicture(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	m1c := board.Seat{Machine: "m1c", Installation: "/c/c/metasystem"}
	write := func(card board.Card) {
		t.Helper()
		card.Seat, card.Writer = m1c, board.Writer{At: fleetNow.Add(-2 * time.Minute)}
		testutil.Require(t, "write "+card.Goal, board.WriteAt(home, card), nil)
	}
	write(board.Card{Goal: "tests-parallel-and-deterministic", Stage: board.StageBuild, Owner: &board.Owner{Pid: 41, PidStartedAt: 1000}})
	write(board.Card{Goal: "goal-dead", Stage: board.StageReview, Owner: &board.Owner{Pid: 77, PidStartedAt: 1000}})
	seats := func() ([]board.Seat, error) { return []board.Seat{m1c}, nil }
	served := New(Info{
		Observe: silentHolder, Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute},
	}, loopback(), testBundle())

	response := request(t, served, http.MethodGet, boardPath, "127.0.0.1:7878", nil)
	testutil.Require(t, "status", response.Code, http.StatusOK)
	var payload boardPayload
	testutil.Require(t, "decode", json.Unmarshal(response.Body.Bytes(), &payload), nil)
	testutil.Expect(t, "readable", payload.Readable, true)
	testutil.Expect(t, "bridge", payload.Bridge, board.BridgeAbsent)
	testutil.Require(t, "one seat", len(payload.Seats), 1)
	reasons := map[string]string{}
	for _, entry := range payload.Seats[0].Goals {
		reasons[entry.Goal] = entry.Unknown
	}
	testutil.Expect(t, "the claimed live card is underway", reasons["tests-parallel-and-deterministic"], "")
	testutil.Expect(t, "the dead owner is Unknown", reasons["goal-dead"], "writer dead (pid 77)")
	testutil.Require(t, "one line per seat", len(payload.Lines), 1)
	testutil.Expect(t, "the seat's line", payload.Lines[0].Machine, "m1c")

	unclaimed := New(Info{Now: func() time.Time { return fleetNow },
		Observe: func() snapshot.Observation { observed := silentHolder(); observed.Tree.Live = nil; return observed },
		Board:   &BoardSource{Home: home, Seats: seats, Prober: boardProber{}, Stall: 20 * time.Minute}}, loopback(), testBundle())
	payload = boardPayload{}
	testutil.Require(t, "decode unclaimed", json.Unmarshal(request(t, unclaimed, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	for _, entry := range payload.Seats[0].Goals {
		if entry.Goal == "tests-parallel-and-deterministic" {
			testutil.Expect(t, "a live card nobody claims", entry.Unknown, "not claimed")
		}
	}

	broken := New(Info{Now: func() time.Time { return fleetNow },
		Board: &BoardSource{Home: home, Seats: func() ([]board.Seat, error) { return nil, errors.New("permission denied") }, Prober: boardProber{}, Stall: time.Minute}},
		loopback(), testBundle())
	payload = boardPayload{}
	testutil.Require(t, "decode broken", json.Unmarshal(request(t, broken, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Body.Bytes(), &payload), nil)
	testutil.Expect(t, "an unreadable registry is an unreadable board", payload.Readable, false)
	testutil.Expect(t, "naming why", payload.Reason, "registry: permission denied")

	none := New(Info{Now: func() time.Time { return fleetNow }}, loopback(), testBundle())
	testutil.Expect(t, "no reader", request(t, none, http.MethodGet, boardPath, "127.0.0.1:7878", nil).Code, http.StatusInternalServerError)
}

// TestTheInterfaceFollowsTheBridgeWhileAStreamIsOpen (R25, U10c-2): the
// server subscribes to the bridge once, only while a notification stream is
// open, and re-announces every bridge event on the fleet watch, which the
// page re-reads /api/board on; the last stream leaving ends the
// subscription; without the bridge nothing waits and the page reads the
// board directly on request.
func TestTheInterfaceFollowsTheBridgeWhileAStreamIsOpen(t *testing.T) {
	t.Parallel()
	dials := make(chan struct{}, 4)
	events := make(chan string)
	ended := make(chan struct{})
	source := &BoardSource{Home: t.TempDir(), Seats: func() ([]board.Seat, error) { return nil, nil }, Prober: boardProber{}, Stall: time.Minute,
		Dial: func() (net.Conn, error) {
			dials <- struct{}{}
			server, client := net.Pipe()
			go func() {
				defer close(ended)
				reader := bufio.NewReader(server)
				if _, err := reader.ReadString('\n'); err != nil {
					return
				}
				io.WriteString(server, `{"snapshot":[]}`+"\n")
				for line := range events {
					io.WriteString(server, line+"\n")
				}
				// The subscription ends: the client closes its end.
				reader.ReadString('\n')
			}()
			return client, nil
		},
		Retry:  func() (<-chan time.Time, func()) { return nil, func() {} },
		Silent: func(time.Duration) <-chan time.Time { return nil },
	}
	watch := fleet.NewWatch()
	h := newHandler(Info{Board: source, Watch: watch, Now: func() time.Time { return fleetNow }}, loopback(), testBundle(), randomNonce)
	signals, leaveWatch := watch.Join()
	defer leaveWatch()
	select {
	case <-dials:
		t.Fatal("the server dialled the bridge with no stream open")
	default:
	}
	leave := h.followBridge()
	<-dials
	<-signals // the connection itself: changes may have been missed
	events <- `{"event":"card"}`
	<-signals
	close(events)
	leave()
	<-ended
	select {
	case <-dials:
		t.Fatal("the subscription was dialled twice")
	default:
	}
}
