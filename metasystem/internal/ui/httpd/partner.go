package httpd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	resolver "github.com/widoriezebos/agentic-tools/metasystem/internal/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/notifications"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/overview"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
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
	partnerPath        = "/api/partner"
	partnerTurnsPath   = "/api/partner/turns"
	partnerTurnsPre    = "/api/partner/turns/"
	partnerSeeingPath  = "/api/partner/seeing"
	stopSuffix         = "/stop"
	routePartnerTurn   = "partner-turn"
	routePartnerStop   = "partner-stop"
	routePartnerSeeing = "partner-seeing"
	// The sitting's own two: one opens a working conversation on a record, one
	// ends it. The end path is exact and lies under the start path's own
	// address; no sitting is named by an id, because there is one sitting on one
	// conversation.
	partnerSittingPath    = "/api/partner/sitting"
	partnerSittingEndPath = "/api/partner/sitting/end"
	routePartnerSitting   = "partner-sitting"
	routePartnerRise      = "partner-sitting-end"
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
	// Composing what the next question will carry is a POST because it sends a
	// capture, and it is a read: nothing is admitted, nothing is remembered,
	// and the Partner is told nothing by it.
	if path == partnerSeeingPath {
		return written{route: routePartnerSeeing}, true
	}
	// The end path first: it lies under the start path, and an exact match is
	// what tells the two apart.
	if path == partnerSittingEndPath {
		return written{route: routePartnerRise}, true
	}
	if path == partnerSittingPath {
		return written{route: routePartnerSitting}, true
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
	observed := h.info.Observe()
	board := backlogOf(observed)
	rows := plainRows(board.Rows)
	return overview.Compose(overview.Inputs{
		Project: pane,
		Rows:    rows,
		Closed:  plainRows(board.Closed),
		Counts:  board.Counts,
		Ledger:  ledgerFor(board),
		Journal: journal,
		Holders: h.holders(observed, rows, now),
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
	// A question creates no draft, so there is none to offer again.
	h.refuseTurn(w, err, "")
}

// seeingBody is one capture, asked about rather than sent.
type seeingBody struct {
	About partner.Page `json:"about"`
}

// partnerSeeing composes what the Partner will be given for this capture.
//
// It goes through the same composer the turn's own block goes through, which
// is the whole point: a sheet composed a second way would be a second account
// of the same page, and the human would be reading a rehearsal rather than the
// thing. It speaks of the NEXT question — nothing is sent, nothing is
// remembered, and no answer's stamp changes because somebody opened it.
func (h *handler) partnerSeeing(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body seeingBody
	if !decodeDocument(w, r, &body) {
		return
	}
	seen := h.info.Partner.See(body.About, h.now())
	_ = json.NewEncoder(w).Encode(struct {
		Label     string `json:"label"`
		Source    string `json:"source"`
		Displayed string `json:"displayed"`
		Supplied  int    `json:"supplied"`
		Total     int    `json:"total"`
		Block     string `json:"block"`
	}{
		Label: seen.Label, Source: seen.Source, Displayed: seen.Displayed,
		Supplied: seen.Supplied, Total: seen.Total,
		Block: partner.ComposeSeen(seen, body.About, ""),
	})
}

// sittingBody is one sitting, as the page opens it: what it is for, the record
// it is about, and where the human was standing when they pressed Start.
//
// Subject and Title are the two ways of naming the record, and exactly one of
// them is used: a subject names a record this checkout already has, and a title
// names a draft to create now, in the home the purpose's own kind names. A body
// carrying both uses the subject, because a human who was on a record's page
// pressed Start there.
type sittingBody struct {
	Purpose string          `json:"purpose"`
	Subject partner.Subject `json:"subject"`
	Title   string          `json:"title"`
	About   partner.Page    `json:"about"`
}

// partnerSitting opens a sitting, and answers the conversation as it now stands
// — with the sitting on it and the opening turn already in the transcript.
//
// It answers the snapshot rather than the sitting alone, for the stop route's
// reason: the opening turn is a turn this page did not send, so the page has to
// be handed the transcript that now holds it rather than left to guess that one
// appeared.
//
// The draft, where the human named one, is created BEFORE the sitting, through
// the project's own writer and its own refusals. A sitting whose subject does
// not exist is a sitting with nothing to record into, and the order means a
// refused creation refuses the whole act with the project's own words.
//
// But nothing is created until the act could succeed at all. Sol's third
// finding: the draft went in before the purpose had been judged or the runtime
// asked whether it could take a turn, so a purpose this build does not offer, or
// a Partner that is not installed, left a record in the project nobody asked for
// and no sitting names. Admits asks both, and creates nothing.
//
// What can still fail after the draft exists is the opening turn — the runtime
// was there a moment ago and is not now, or another turn started in between. Then
// the refusal carries the draft's own path, so the page offers Start again on
// that draft rather than creating a second one for the same wish.
func (h *handler) partnerSitting(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body sittingBody
	if !decodeDocument(w, r, &body) {
		return
	}
	if err := h.info.Partner.Admits(r.Context(), body.Purpose); err != nil {
		h.refuseTurn(w, err, "")
		return
	}
	subject := body.Subject
	draft := ""
	if strings.TrimSpace(subject.ID) == "" {
		created, ok := h.draftFor(w, body)
		if !ok {
			return
		}
		subject = created
		draft = created.ID
	}
	if _, err := h.info.Partner.Sit(r.Context(), h.partnerHuman(r), subject, body.Purpose, body.About); err != nil {
		h.refuseTurn(w, err, draft)
		return
	}
	h.answerPartner(w, r)
}

// draftFor creates the draft a sitting was started on, and reports whether the
// route may go on. The kind is the purpose's own: shaping intent writes an
// intent record and shaping a design writes a design, so nothing here asks a
// human for a kind they have already chosen by choosing what the sitting is for.
func (h *handler) draftFor(w http.ResponseWriter, body sittingBody) (partner.Subject, bool) {
	if h.info.CreateRecord == nil {
		writeFailure(w, "this engine was built without a project writer")
		return partner.Subject{}, false
	}
	kind := resolver.KindDesign
	if strings.TrimSpace(body.Purpose) == partner.PurposeShapeIntent {
		kind = resolver.KindIntent
	}
	written, err := h.info.CreateRecord(project.NewRecord{Kind: kind, Title: body.Title})
	if err != nil {
		var refusal *project.Refusal
		if errors.As(err, &refusal) {
			writeRefusal(w, statusOf(refusal.Kind), refusal.Message, refusal.Problems)
			return partner.Subject{}, false
		}
		writeFailure(w, err.Error())
		return partner.Subject{}, false
	}
	return partner.Subject{
		Kind: partner.SubjectRecord, ID: written.Path, Title: written.Record.Title,
	}, true
}

// partnerRise ends the sitting, and answers the conversation without it. What
// was recorded stays in the record, which is the whole of what a sitting leaves.
func (h *handler) partnerRise(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body struct{}
	if !decode(w, r, &body) {
		return
	}
	if err := h.info.Partner.Rise(h.partnerHuman(r)); err != nil {
		writeFailure(w, err.Error())
		return
	}
	h.answerPartner(w, r)
}

// answerPartner writes the conversation as it stands, which is what every route
// that changes it answers with.
func (h *handler) answerPartner(w http.ResponseWriter, r *http.Request) {
	snapshot, err := h.info.Partner.Snapshot(h.partnerHuman(r), partnerMessages)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}

// refuseTurn is the turn route's own three answers, reached from both routes
// that admit one: 409 for a turn already running, 503 with the runtime's own
// words and the line that installs it, and 400 for a request this server could
// read and would not act on.
//
// draft is the record this request created before the refusal, and "" where it
// created none. It travels because a sitting refused AFTER its draft exists has
// left a real record in the project: the page offers Start again on that draft,
// so a second press is a second attempt at one wish rather than a second record
// for it.
func (h *handler) refuseTurn(w http.ResponseWriter, err error, draft string) {
	if errors.Is(err, partner.ErrBusy) {
		w.WriteHeader(http.StatusConflict)
		writeDraftRefusal(w, "busy", err.Error(), "", draft)
		return
	}
	var start *partner.StartError
	if errors.As(err, &start) {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeDraftRefusal(w, "runtime", start.Reason, start.Install, draft)
		return
	}
	w.WriteHeader(http.StatusBadRequest)
	writeDraftRefusal(w, "request", err.Error(), "", draft)
}

// writeDraftRefusal is the refusal body the Partner's two admitting routes share:
// the reason, what kind of refusal it is, the line that installs a runtime where
// there is one, and the draft this request created before it was refused.
func writeDraftRefusal(w http.ResponseWriter, code, reason, install, draft string) {
	_ = json.NewEncoder(w).Encode(struct {
		Error   string `json:"error"`
		Code    string `json:"code"`
		Install string `json:"install,omitempty"`
		Draft   string `json:"draft,omitempty"`
	}{Error: reason, Code: code, Install: install, Draft: draft})
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
