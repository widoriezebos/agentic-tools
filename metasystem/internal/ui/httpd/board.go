package httpd

// The board resource: what every seat of this host works on and how far it
// is (batch-lane design D14-r2, R23, R24, U10d).
//
// It is a direct read of the host board, made on every request and
// classified here, in the server, with the server's own prober and clock:
// the one classifier every reader runs (board.Classify through board.Read),
// then the ledger's checks against the same observation the backlog and the
// Fleet page read. Nothing is decided here, and the bridge's socket is never
// connected: bridge live or absent is the socket's presence.

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/fleet"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// boardPath is the board resource, matched exactly.
const boardPath = "/api/board"

// BoardSource is where this server reads the host board from: the board's
// home, the armed seats of this host (the registry projected onto their
// nicknames; an error is an unreadable registry, never an empty board), the
// prober that decides whether an owner lives, and the stall bound.
type BoardSource struct {
	Home   string
	Seats  func() ([]board.Seat, error)
	Prober identity.Prober
	Stall  time.Duration
	// Dial connects to the bridge; nil dials its socket under Home. Retry is
	// the cadence a lost bridge is tried again at, and Silent the clock of
	// the two-heartbeat silence bound; nil for either is the wall clock.
	Dial   func() (net.Conn, error)
	Retry  func() (<-chan time.Time, func())
	Silent func(time.Duration) <-chan time.Time
	// Lane reads the host's landing lane (U12) at the server's clock; nil
	// serves a lane with no root.
	Lane func(now time.Time) lane.View
}

// boardPayload is the classified board, and each seat's line as a person
// reads it, rendered once here so the page and the terminal say the same.
type boardPayload struct {
	board.View
	Lines []boardLine `json:"lines"`
	// Lane is the host's landing lane, the view landing status renders.
	Lane lane.View `json:"lane"`
}

type boardLine struct {
	Machine string `json:"machine"`
	Text    string `json:"text"`
}

// board answers the board panel. A build with no board reader is a 500
// carrying the reason, like every other read.
func (h *handler) board(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	source := h.info.Board
	if source == nil || source.Seats == nil {
		writeFailure(w, "this engine was built without a board reader")
		return
	}
	_ = json.NewEncoder(w).Encode(h.boardView(source))
}

func (h *handler) boardView(source *BoardSource) boardPayload {
	now := h.now()
	laneView := lane.View{Owner: lane.OwnerView{State: lane.OwnerNotStarted}, Summary: "this engine was built without a landing lane reader"}
	if source.Lane != nil {
		laneView = source.Lane(now)
	}
	view := board.View{Bridge: board.BridgeState(source.Home), Seats: []board.SeatView{}}
	seats, err := source.Seats()
	if err != nil {
		view.Reason = "registry: " + err.Error()
		return boardPayload{View: view, Lines: []boardLine{}, Lane: laneView}
	}
	picture, _ := board.Read(source.Home, seats, source.Prober, now, source.Stall)
	if claims, ok := h.boardClaims(); ok {
		picture = board.CheckClaims(picture, seats, claims)
	}
	bridge := view.Bridge
	view = board.NewView(seats, picture)
	view.Readable, view.Bridge = true, bridge
	lines := make([]boardLine, 0, len(view.Seats))
	for _, seat := range view.Seats {
		lines = append(lines, boardLine{Machine: seat.Machine, Text: seat.Text(now, time.Local)})
	}
	return boardPayload{View: view, Lines: lines, Lane: laneView}
}

// boardClaims are the live claims of the accepted ledger this server reads;
// false when it cannot read one, and then no card is checked against it.
func (h *handler) boardClaims() (map[string]string, bool) {
	if h.info.Observe == nil {
		return nil, false
	}
	observed := h.info.Observe()
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return nil, false
	}
	claims := map[string]string{}
	for id, file := range observed.Tree.Live {
		if file != nil && file.Claimed != nil && file.Claimed.Machine != "" {
			claims[id] = file.Claimed.Machine
		}
	}
	return claims, true
}

// bridgeFollower is this server's one subscription to the host board's
// bridge (batch-lane design D14-r2, U10c-2), alive while at least one
// notification stream is open: every bridge event, and every
// (re)connection, which may have missed changes, is announced on the fleet
// watch, and the page re-reads /api/board, which reads and classifies the
// board afresh. The subscription carries nothing the server trusts. Without
// the bridge the page reads the board directly on request; the follower
// tries the bridge again at its retry cadence and never waits for it.
type bridgeFollower struct {
	source *BoardSource
	watch  *fleet.Watch

	mu      sync.Mutex
	streams int
	stop    chan struct{}
	done    chan struct{}
}

// followBridge counts one open stream, starting the subscription on the
// first; the returned function counts it closed, ending the subscription
// with the last.
func (h *handler) followBridge() func() {
	follower := h.bridge
	if follower == nil {
		return func() {}
	}
	follower.mu.Lock()
	follower.streams++
	if follower.streams == 1 {
		follower.stop, follower.done = make(chan struct{}), make(chan struct{})
		go follower.follow(follower.stop, follower.done)
	}
	follower.mu.Unlock()
	return func() {
		follower.mu.Lock()
		follower.streams--
		var stop, done chan struct{}
		if follower.streams == 0 {
			stop, done = follower.stop, follower.done
		}
		follower.mu.Unlock()
		if stop != nil {
			close(stop)
			<-done
		}
	}
}

func (f *bridgeFollower) follow(stop, done chan struct{}) {
	defer close(done)
	retry, stopRetry := f.retry()
	defer stopRetry()
	for {
		if sub := f.connect(); sub != nil {
			f.watch.Announce()
			if f.relay(sub, stop) {
				return
			}
		}
		select {
		case <-stop:
			return
		case <-retry:
		}
	}
}

// relay announces every event until the subscription ends; true when the
// last stream left.
func (f *bridgeFollower) relay(sub *board.Subscription, stop chan struct{}) bool {
	defer sub.Close()
	for {
		select {
		case <-stop:
			return true
		case _, open := <-sub.Events:
			if !open {
				return false
			}
			f.watch.Announce()
		}
	}
}

func (f *bridgeFollower) connect() *board.Subscription {
	dial := f.source.Dial
	if dial == nil {
		dial = func() (net.Conn, error) { return board.Dial(f.source.Home) }
	}
	conn, err := dial()
	if err != nil {
		return nil
	}
	sub, err := board.Subscribe(conn, board.SubscribeOptions{Kinds: []string{board.KindCard, board.KindStall},
		Heartbeat: board.DefaultHeartbeat, After: f.source.Silent})
	if err != nil {
		return nil
	}
	return sub
}

func (f *bridgeFollower) retry() (<-chan time.Time, func()) {
	if f.source.Retry != nil {
		return f.source.Retry()
	}
	ticker := time.NewTicker(board.DefaultHeartbeat)
	return ticker.C, ticker.Stop
}
