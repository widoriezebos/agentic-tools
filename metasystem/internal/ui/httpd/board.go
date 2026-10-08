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
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
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
	// Stuck reads the host's unit runs with the serving seat's bounds.
	Stuck func(now time.Time) ([]launch.UnitStanding, error)
	// Dial connects to the bridge; nil dials its socket under Home. Retry is
	// the cadence a lost bridge is tried again at, and Silent the clock of
	// the two-heartbeat silence bound; nil for either is the wall clock.
	Dial   func() (net.Conn, error)
	Retry  func() (<-chan time.Time, func())
	Silent func(time.Duration) <-chan time.Time
	// Lane reads the host's landing lane (U12) at the server's clock, as
	// landing status --json carries it (plain.Status); nil serves a lane
	// with no root.
	Lane func(now time.Time) plain.Status
	// ProofLog is the log file the lane's records name for one proof
	// attempt, lying directly inside the lane's proofs folder
	// (plain.ProofLog over this computer's lane); an error wrapping
	// plain.ErrNoProofLog is a log it does not serve, said in words. nil
	// serves no proof log.
	ProofLog func(attempt string) (string, error)
}

// boardPayload is the classified board, and each seat's line as a person
// reads it, rendered once here so the page and the terminal say the same.
type boardPayload struct {
	board.View
	Lines []boardLine `json:"lines"`
	// Lane is the host's landing lane, landing status --json's data: the
	// view, paused, agent_alive, the queue, the running proof, the last
	// proof and the last push; null when no lane is registered on this host.
	Lane *plain.Status `json:"lane"`
	// Titles is the title of every goal the seats, the lane's queue and the
	// questions name, by goal id, from the same ledger observation the cards
	// are checked against. A goal the ledger does not carry has none.
	Titles map[string]string `json:"titles"`
	// Ended says, for each goal in the lane's queue that the ledger holds as
	// concluded, how it ended: "done" or "abandoned". A return of an ended
	// goal is history, not something that needs the person.
	Ended map[string]string `json:"ended"`
	// Questions are this checkout's open channel questions: what a seat has
	// asked the person and nobody has answered. QuestionsProblem says why
	// they, or some of their records, could not be read; "" when every
	// record was read.
	Questions        []boardQuestion `json:"questions"`
	QuestionsProblem string          `json:"questionsProblem"`
	// Unreadable are the parts of this computer's board that were not read,
	// one plain line each: the goal ledger the cards are checked against, a
	// nickname two armed checkouts share, a board or a seat directory that
	// can't be listed, a card that can't be parsed. A directory no armed seat
	// names is a leftover and is not among them.
	Unreadable []string `json:"unreadable"`
}

// UnreadQuestions is a question reader's answer when it read some of this
// checkout's question records and not others: the open questions it could
// read come back beside it, and Records names the ones it could not.
type UnreadQuestions struct{ Records []string }

func (u *UnreadQuestions) Error() string {
	switch len(u.Records) {
	case 0:
		return "no record was named as unreadable"
	case 1:
		return "1 of its records can't be read: " + u.Records[0]
	}
	return fmt.Sprintf("%d of its records can't be read; the first: %s", len(u.Records), u.Records[0])
}

type boardLine struct {
	Machine string `json:"machine"`
	Text    string `json:"text"`
}

// boardQuestion is one open question as the panel shows it: who asks, about
// which goal or what, the question's first line, and since when.
type boardQuestion struct {
	ID       string `json:"id"`
	Goal     string `json:"goal"`
	About    string `json:"about,omitempty"`
	Machine  string `json:"machine"`
	Question string `json:"question"`
	OpenedAt string `json:"openedAt"`
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
	var laneView *plain.Status
	if source.Lane != nil {
		// A lane with no root is no lane, unless reading it failed: then
		// the page is told what could not be read rather than that there
		// is none.
		if read := source.Lane(now); read.Root != nil || len(read.Problems) > 0 {
			laneView = &read
		}
	}
	observed, observable := h.boardObservation()
	payload := boardPayload{Lines: []boardLine{}, Lane: laneView, Unreadable: []string{}}
	if !observable {
		payload.Unreadable = append(payload.Unreadable, "the goal ledger can't be read: "+ledgerProblem(h.info.Observe != nil, observed))
	}
	payload.Questions, payload.QuestionsProblem = h.boardQuestions()
	view := board.View{Bridge: board.BridgeState(source.Home), Seats: []board.SeatView{}}
	seats, err := source.Seats()
	if err != nil {
		view.Reason = "registry: " + err.Error()
		payload.View = view
		payload.Titles = titlesOf(observed, payload)
		payload.Ended = endedOf(observed, payload)
		return payload
	}
	picture, unreadable := board.Read(source.Home, seats, source.Prober, now, source.Stall)
	payload.Unreadable = append(payload.Unreadable, unreadParts(unreadable, picture.Unknown)...)
	if observable {
		picture = board.CheckClaims(picture, seats, claimsOf(observed))
	}
	bridge := view.Bridge
	view = board.NewView(seats, picture)
	if observable {
		view.ProjectScope(func(id, unit string) bool {
			return observed.Tree.Live[id].ExcludesScope(unit, "")
		})
	}
	view.Readable, view.Bridge = true, bridge
	if source.Stuck != nil {
		standings, stuckErr := source.Stuck(now)
		if stuckErr != nil {
			payload.Unreadable = append(payload.Unreadable, "stuck units: "+stuckErr.Error())
		} else {
			readable := observable
			claims := map[string]string{}
			if observable {
				claims = claimsOf(observed)
			}
			for _, standing := range standings {
				if standing.Unreadable != "" {
					payload.Unreadable = append(payload.Unreadable, "stuck units: "+standing.Unreadable)
					readable = false
				}
			}
			for _, standing := range standings {
				if !readable || !standing.Stuck {
					continue
				}
				for seatIndex := range view.Seats {
					seat := &view.Seats[seatIndex]
					if seat.Machine != claims[standing.Goal] {
						continue
					}
					for goalIndex := range seat.Goals {
						card := &seat.Goals[goalIndex]
						if card.Goal == standing.Goal && card.Stuck == nil {
							card.Stuck = &board.StuckUnit{Step: standing.Step, Kind: standing.Kind, Launch: standing.Launch,
								Minutes: standing.Minutes, Rounds: standing.Rounds, Limit: standing.Limit}
						}
					}
				}
			}
		}
	}
	for _, seat := range view.Seats {
		payload.Lines = append(payload.Lines, boardLine{Machine: seat.Machine, Text: seat.Text(now, time.Local)})
	}
	payload.View = view
	payload.Titles = titlesOf(observed, payload)
	payload.Ended = endedOf(observed, payload)
	return payload
}

// unreadParts are the parts of the board a read failed on, as the page says
// them: the reader's own unreadable list without its stray directories, and
// the seats and cards it could not read.
func unreadParts(unreadable []board.Unreadable, unknown []board.Unknown) []string {
	parts := []string{}
	for _, one := range unreadable {
		if !one.Stray {
			parts = append(parts, one.Path+": "+one.Reason)
		}
	}
	for _, one := range unknown {
		switch {
		case strings.HasPrefix(one.Reason, board.ReasonSeatUnreadable):
			parts = append(parts, one.Seat.Machine+": "+one.Reason)
		case one.Reason == board.ReasonCardUnreadable:
			parts = append(parts, one.Seat.Machine+": "+one.Goal+": "+one.Reason)
		}
	}
	return parts
}

// boardObservation is the accepted ledger this server reads, taken once per
// read so the claims the cards are checked against and the titles beside
// them are of one commit; false when it cannot read one, and then no card is
// checked against it and no goal is titled.
func (h *handler) boardObservation() (snapshot.Observation, bool) {
	if h.info.Observe == nil {
		return snapshot.Observation{}, false
	}
	observed := h.info.Observe()
	return observed, observed.State == snapshot.StateRead && observed.Tree != nil
}

// ledgerProblem is why a board read has no ledger to check its cards against
// and title its goals from, in the observation's own words where it has any.
func ledgerProblem(observing bool, observed snapshot.Observation) string {
	switch {
	case !observing:
		return "this server reads no goal ledger"
	case observed.Message != "":
		return observed.Message
	}
	return "the accepted ledger could not be read"
}

// claimsOf are the live claims of one observation: every claimed goal and
// the machine that holds it.
func claimsOf(observed snapshot.Observation) map[string]string {
	claims := map[string]string{}
	for id, file := range observed.Tree.Live {
		if file != nil && file.Claimed != nil && file.Claimed.Machine != "" {
			claims[id] = file.Claimed.Machine
		}
	}
	return claims
}

// endedOf says how each goal in the lane's queue ended, for the ones the
// ledger holds as done or abandoned.
func endedOf(observed snapshot.Observation, payload boardPayload) map[string]string {
	ended := map[string]string{}
	if observed.State != snapshot.StateRead || observed.Tree == nil || payload.Lane == nil {
		return ended
	}
	for _, entry := range payload.Lane.Queue {
		if _, done := observed.Tree.Done[entry.Goal]; done {
			ended[entry.Goal] = "done"
		} else if _, abandoned := observed.Tree.Abandoned[entry.Goal]; abandoned {
			ended[entry.Goal] = "abandoned"
		}
	}
	return ended
}

// titlesOf titles every goal the payload names, live, done or abandoned:
// a hand-in that landed is usually a goal its seat has since concluded.
func titlesOf(observed snapshot.Observation, payload boardPayload) map[string]string {
	titles := map[string]string{}
	if observed.State != snapshot.StateRead || observed.Tree == nil {
		return titles
	}
	name := func(id string) {
		if id == "" {
			return
		}
		file, live := observed.Tree.Live[id]
		if !live {
			file, _ = observed.Tree.Archived(id)
		}
		if file != nil && file.Intent != "" {
			titles[id] = fleet.Title(file.Intent)
		}
	}
	for _, seat := range payload.Seats {
		for _, entry := range seat.Goals {
			name(entry.Goal)
		}
	}
	if payload.Lane != nil {
		for _, entry := range payload.Lane.Queue {
			name(entry.Goal)
		}
	}
	for _, question := range payload.Questions {
		name(question.Goal)
	}
	return titles
}

// boardQuestions are this checkout's open questions, or why they could not
// be read. The question is its first fact, the line the asker wrote first.
func (h *handler) boardQuestions() ([]boardQuestion, string) {
	questions := []boardQuestion{}
	if h.info.Asks == nil {
		return questions, "this server reads no questions"
	}
	open, err := h.info.Asks()
	problem := ""
	var unread *UnreadQuestions
	switch {
	case errors.As(err, &unread):
		problem = unread.Error()
	case err != nil:
		return questions, err.Error()
	}
	for _, question := range open {
		asked := ""
		if len(question.Facts) > 0 {
			asked = strings.TrimSpace(question.Facts[0])
		}
		opened := ""
		if !question.OpenedAt.IsZero() {
			opened = question.OpenedAt.UTC().Format(time.RFC3339)
		}
		questions = append(questions, boardQuestion{ID: question.ID, Goal: question.Goal, About: question.About,
			Machine: question.Machine, Question: asked, OpenedAt: opened})
	}
	return questions, problem
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
