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
	"net/http"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
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
}

// boardPayload is the classified board, and each seat's line as a person
// reads it, rendered once here so the page and the terminal say the same.
type boardPayload struct {
	board.View
	Lines []boardLine `json:"lines"`
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
	view := board.View{Bridge: board.BridgeState(source.Home), Seats: []board.SeatView{}}
	seats, err := source.Seats()
	if err != nil {
		view.Reason = "registry: " + err.Error()
		return boardPayload{View: view, Lines: []boardLine{}}
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
	return boardPayload{View: view, Lines: lines}
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
