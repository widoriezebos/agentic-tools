package httpd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// The Project Partner's three routes.
//
// One read answers everything a page needs from cold — which runtime, which
// model, whose conversation, whether a turn is running, what it has said so
// far, and the transcript — so a reload never has to guess whether a question
// was accepted or an answer complete. One write admits a turn, idempotent on
// the key the page minted, and one stops the running one.
//
// The turn's own beats do not come back on these routes. They ride the page's
// one event stream, under their own event type, in notifications.go.
const (
	partnerPath      = "/api/partner"
	partnerTurnsPath = "/api/partner/turns"
	partnerTurnsPre  = "/api/partner/turns/"
	stopSuffix       = "/stop"
	routePartnerTurn = "partner-turn"
	routePartnerStop = "partner-stop"
)

// partnerMessages is how much of the transcript the read route carries. A
// conversation older than that is in the file, and this slice offers no way to
// page back through it — which the page says rather than hides.
const partnerMessages = 100

// turnBody is one send: the key the page minted so a retry is the same turn,
// the question, and where the human was when they asked it.
type turnBody struct {
	Key   string       `json:"key"`
	Text  string       `json:"text"`
	About partner.Page `json:"about"`
}

// partnerRouteOf reports which Partner write route a path names.
func partnerRouteOf(path string) (written, bool) {
	if path == partnerTurnsPath {
		return written{route: routePartnerTurn}, true
	}
	if id, ok := idBetween(path, partnerTurnsPre, stopSuffix); ok {
		return written{route: routePartnerStop, id: id}, true
	}
	return written{}, false
}

// partnerOverview is the landing page as this server composes it, for the
// context a turn asked from Overview carries. It is the route's own
// composition with one difference: it does not record a visit. Reading the
// page IS the visit, and a Partner turn is not a human looking at it — moving
// the human's marker here would shorten the window their next visit compares
// against. So the window is a day, which is what a first visit reads over, and
// only the parts that do not depend on it are told.
func (h *handler) partnerOverview() (overview.Page, error) {
	if h.info.Project == nil || h.info.Observe == nil {
		return overview.Page{}, errors.New("this engine was built without the landing page's readers")
	}
	pane, err := h.info.Project()
	if err != nil {
		return overview.Page{}, err
	}
	journal := []notifications.Notice{}
	if h.info.NotificationJournal != "" {
		read, journalErr := notifications.Page(h.info.NotificationJournal, notifications.DefaultLimit, "")
		if journalErr != nil {
			return overview.Page{}, journalErr
		}
		journal = read
	}
	now := h.now()
	board := backlogOf(h.info.Observe())
	return overview.Compose(overview.Inputs{
		Project: pane,
		Rows:    board.Rows,
		Closed:  board.Closed,
		Counts:  board.Counts,
		Ledger:  ledgerFor(board),
		Journal: journal,
		Human:   overview.Standing{Proven: h.info.Authority.Proven},
		Since:   now.Add(-24 * time.Hour),
		First:   true,
	}, now), nil
}

// partnerHuman is whose conversation a request is about: the human signed in
// at this browser, else the one this server's boot proof names, else the one
// the seat configured, else the seat itself. It is the design's own order, and
// it is resolved per request because signing in is what changes it.
func (h *handler) partnerHuman(r *http.Request) string {
	if signed, liveness := h.signedIn(r); liveness == session.Live && signed != nil {
		if named := strings.TrimSpace(signed.Human); named != "" {
			return named
		}
	}
	if named := strings.TrimSpace(h.knownHuman()); named != "" {
		return named
	}
	return "seat"
}

// partner answers what the Partner is and what it has said.
func (h *handler) partner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	snapshot, err := h.info.Partner.Snapshot(h.partnerHuman(r), partnerMessages)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}

// partnerRefusal is what a build with no Partner says, in the words the seat
// has: the admission's own refusal where there was one, and the plain sentence
// otherwise.
func (h *handler) partnerRefusal() string {
	if h.info.PartnerRefusal != "" {
		return h.info.PartnerRefusal
	}
	return "no Partner runtime is configured on this seat; set ui.partner.runtime to claude, codex or devin"
}

// partnerTurn admits one turn.
//
// 202 with the turn's id is acceptance, and the page clears its draft on it.
// 409 is a turn already running, and the draft stays. 503 is a runtime that
// cannot start — not installed, not signed in, refusing the configured model —
// and it carries the runtime's own words and the line that installs it, so the
// human reads what the server read and the question stays in the composer.
func (h *handler) partnerTurn(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body turnBody
	if !decodeDocument(w, r, &body) {
		return
	}
	id, err := h.info.Partner.Submit(r.Context(), h.partnerHuman(r), body.Key, body.Text, body.About)
	if err == nil {
		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(struct {
			Turn string `json:"turn"`
		}{Turn: id})
		return
	}
	if errors.Is(err, partner.ErrBusy) {
		w.WriteHeader(http.StatusConflict)
		writeActRefusal(w, "busy", err.Error())
		return
	}
	var start *partner.StartError
	if errors.As(err, &start) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(struct {
			Error   string `json:"error"`
			Code    string `json:"code"`
			Install string `json:"install,omitempty"`
		}{Error: start.Reason, Code: "runtime", Install: start.Install})
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	writeActRefusal(w, "request", err.Error())
}

// partnerStop stops the running turn. It answers the snapshot, so the page
// reads the settled state from the same request that asked for it.
func (h *handler) partnerStop(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	if err := h.info.Partner.Stop(r.Context(), id); err != nil {
		writeFailure(w, err.Error())
		return
	}
	snapshot, err := h.info.Partner.Snapshot(h.partnerHuman(r), partnerMessages)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}

// writePartnerEvent writes one Partner event onto the page's one stream.
//
// There is no `id:` line, deliberately. The browser sends the last id it saw
// back as Last-Event-ID, and the server resumes the notification journal from
// it; a Partner event with an id of its own would name something that journal
// cannot find, and the reconnect would then replay nothing and lose every
// notification in between. The page joins Partner events to the
// conversation's snapshot by turn and sequence instead, and it re-reads that
// snapshot on every reconnect.
func writePartnerEvent(w http.ResponseWriter, flusher http.Flusher, event partner.Event) bool {
	body, err := json.Marshal(event)
	if err != nil {
		return true
	}
	if _, err := io.WriteString(w, "event: partner\ndata: "+string(body)+"\n\n"); err != nil {
		return false
	}
	flusher.Flush()
	return true
}
