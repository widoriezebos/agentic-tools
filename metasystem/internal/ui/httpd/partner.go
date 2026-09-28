package httpd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
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
	// And the close, which is the drafting of the sitting's outcome and not the
	// end of it: it asks one more turn and leaves the mark where it is, so the
	// card it offers is admitted against the record the sitting is on
	// (g1-s55 D2). Ending is still the route above, reached after the human has
	// recorded the outcome or has said they are leaving without it.
	partnerSittingClosePath = "/api/partner/sitting/close"
	routePartnerClose       = "partner-sitting-close"
	// And the outcome of one proposed action, beneath the turn that proposed it:
	// what the human's press did, written where the proposal is. It carries no
	// authority at all — the act itself went to the ledger's own route, under the
	// human's session — and changes nothing but the transcript.
	proposalsInfix       = "/proposals/"
	routePartnerProposal = "partner-proposal"
)

// partnerMessages is how much of the transcript the read route carries. A
// conversation older than that is in the file, and this slice offers no way to
// page back through it — which the page says rather than hides.
const partnerMessages = 100

// turnBody is one send: the key the page minted so a retry is the same turn,
// the question, and where the human was when they asked it.
type turnBody struct {
	// Conversation names a sitting's record, whose own conversation the turn
	// is asked in; absent, the human's own (g1-s65 D16).
	Conversation string       `json:"conversation"`
	Key          string       `json:"key"`
	Text         string       `json:"text"`
	About        partner.Page `json:"about"`
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
	// The exact paths beneath the start path first: they lie under it, and an
	// exact match is what tells them apart.
	if path == partnerSittingEndPath {
		return written{route: routePartnerRise}, true
	}
	if path == partnerSittingClosePath {
		return written{route: routePartnerClose}, true
	}
	if path == partnerWalkPath {
		return written{route: routePartnerWalk}, true
	}
	if path == partnerSittingPath {
		return written{route: routePartnerSitting}, true
	}
	// The outcome of one proposed action, before the stop below: both lie under
	// the turns prefix, and this one carries a second name after it.
	if turn, at, ok := proposalAt(path); ok {
		return written{route: routePartnerProposal, id: turn, at: at}, true
	}
	if id, ok := idBetween(path, partnerTurnsPre, stopSuffix); ok {
		return written{route: routePartnerStop, id: id}, true
	}
	return written{}, false
}

// proposalAt reads the turn and the action a path names. Both halves must be
// there and neither may be empty: a path naming a turn and no action is no
// route, and falls through to the 404 every unserved path under a reserved
// prefix gets.
func proposalAt(path string) (string, string, bool) {
	rest, beneath := strings.CutPrefix(path, partnerTurnsPre)
	if !beneath {
		return "", "", false
	}
	turn, at, split := strings.Cut(rest, proposalsInfix)
	if !split || turn == "" || at == "" || strings.Contains(turn, "/") || strings.Contains(at, "/") {
		return "", "", false
	}
	return turn, at, true
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
	return partnerSeat
}

// partnerSeat is the human a seat that knows nobody is: the transcript what is
// said before anybody signs in goes into. The sign-in hands it to the name it
// binds, so the conversation is not left behind by being named.
const partnerSeat = "seat"

// partner answers what the Partner is and what it has said.
//
// It is also where a sitting resumes (g1-s55 D3). A conversation whose sitting
// mark stands, read by a page whose Partner session has since ended, is asked
// the opening question again before this answers — so the transcript the page is
// handed already holds the turn, and the first words it reads say what is on the
// table. The decision is the service's one freshness decision, not this route's,
// and no browser effect submits a turn: an effect would submit one per tab and
// per reload, in a human's name.
//
// A resume that cannot be asked — a runtime that has gone away since the sitting
// started — is not a failure of the read. The conversation is still there to
// show, the sitting still stands, and the human can still type; so the refusal
// is left where the next turn will meet it rather than turned into a page that
// will not load.
func (h *handler) partner(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	// Which conversation the page is showing: a sitting's, named by its
	// record, or the human's own (g1-s65 D16).
	where := strings.TrimSpace(r.URL.Query().Get("conversation"))
	_, _ = h.info.Partner.ResumeIn(r.Context(), h.partnerHuman(r), where)
	snapshot, err := h.info.Partner.SnapshotIn(h.partnerHuman(r), where, partnerMessages)
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
	id, err := h.info.Partner.SubmitIn(r.Context(), h.partnerHuman(r), strings.TrimSpace(body.Conversation),
		body.Key, body.Text, body.About)
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
	// Conversation and Room are the room's keep (g1-s65 D9): the desk, the
	// face and the unfinished words, written onto that sitting's mark through
	// this route rather than a second one.
	Conversation string        `json:"conversation"`
	Room         *partner.Room `json:"room"`
}

// reviewSubject is the kind a review's subject is named by on Start: the goal it
// reviews. The server turns it into the review record the sitting is about.
const reviewSubject = "goal"

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
	if body.Room != nil {
		h.keepRoom(w, r, body)
		return
	}
	if err := h.info.Partner.Admits(r.Context(), body.Purpose); err != nil {
		h.refuseTurn(w, err, "")
		return
	}
	if strings.TrimSpace(body.Purpose) == partner.PurposeReview && strings.TrimSpace(body.Subject.Kind) == reviewSubject {
		h.startReview(w, r, body)
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
	// A sitting is a conversation of its own (g1-s65 D16), and the answer is
	// that conversation, with the sitting on it and its opening turn in it.
	h.answerPartnerIn(w, r, subject.ID)
}

// keepRoom writes the room's working state onto one sitting's mark, and answers
// when it was kept.
func (h *handler) keepRoom(w http.ResponseWriter, r *http.Request, body sittingBody) {
	if err := h.info.Partner.KeepRoom(h.partnerHuman(r), strings.TrimSpace(body.Conversation), *body.Room); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Kept string `json:"kept"`
	}{Kept: h.now().UTC().Format(time.RFC3339)})
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

// partnerClose asks the Partner to draft the sitting's closing deposit, and
// answers the conversation with that turn already in the transcript.
//
// It ends nothing. The sitting stands until the human has recorded the outcome
// or has said they are leaving without it, because the card this turn offers is
// admitted against the sitting's subject and a conversation with no mark on it
// would offer it against no record at all.
//
// It answers the snapshot for the start route's reason: the closing turn is a
// turn this page did not send, so the page has to be handed the transcript that
// now holds it rather than left to guess that one appeared.
func (h *handler) partnerClose(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body struct {
		Conversation string       `json:"conversation"`
		Verdict      string       `json:"verdict"`
		About        partner.Page `json:"about"`
	}
	if !decodeDocument(w, r, &body) {
		return
	}
	where := strings.TrimSpace(body.Conversation)
	if _, err := h.info.Partner.ClosingIn(r.Context(), h.partnerHuman(r), where, strings.TrimSpace(body.Verdict), body.About); err != nil {
		h.refuseTurn(w, err, "")
		return
	}
	h.answerPartnerIn(w, r, where)
}

// partnerRise ends the sitting, and answers the conversation without it. What
// was recorded stays in the record, which is the whole of what a sitting leaves.
func (h *handler) partnerRise(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body struct {
		Conversation string `json:"conversation"`
	}
	if !decode(w, r, &body) {
		return
	}
	where := strings.TrimSpace(body.Conversation)
	if err := h.info.Partner.RiseIn(h.partnerHuman(r), where); err != nil {
		writeFailure(w, err.Error())
		return
	}
	h.answerPartnerIn(w, r, where)
}

// answerPartner writes the conversation as it stands, which is what every route
// that changes it answers with.
// answerPartnerIn writes one conversation as it stands: a sitting's, or the
// human's own where the record is "".
func (h *handler) answerPartnerIn(w http.ResponseWriter, r *http.Request, where string) {
	snapshot, err := h.info.Partner.SnapshotIn(h.partnerHuman(r), where, partnerMessages)
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
	// The handoff to another conversation that could not wait for the running
	// answer to settle (S65-06): a state, like busy, that asking again clears.
	if errors.Is(err, partner.ErrUnsettled) {
		w.WriteHeader(http.StatusConflict)
		writeDraftRefusal(w, "unsettled", err.Error(), "", draft)
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

// proposalBody is what one press says about one proposed action: the version of
// the entry the page last rendered, the state it is writing, and the words that
// state carries.
//
// The version is what makes the write exclusive. Two tabs showing one line in
// flight would otherwise both be entitled to write its outcome, and a state
// comparison cannot tell a fresh attempt from an abandoned one; with the version
// the loser is handed the entry as it stands and must show it.
type proposalBody struct {
	Version int    `json:"version"`
	State   string `json:"state"`
	Words   string `json:"words"`
	// Attempt is the run of the runner this press belongs to: one id for the
	// lifetime of a run, sent with every write that run makes.
	//
	// It is what the version cannot say. A line goes back to `applying` on every
	// Try again and on every takeover, so the version a page reads there does
	// not tell it whose act the line is waiting for; the attempt does. The write
	// that moves a line to `applying` claims it, and the settle that follows must
	// carry the attempt the entry holds. A dismissal carries none.
	Attempt string `json:"attempt"`
	// Conversation names the sitting's conversation the proposal was made in;
	// absent, the human's own (g1-s65 D16).
	Conversation string `json:"conversation"`
}

// partnerProposal records what the human's press did to one proposed action.
//
// It carries no authority and it makes no act: the act went to the ledger's own
// route, under the human's own session, and this writes down what that answered
// where the proposal is. So it takes the conversation's hand, like the two
// routes that admit and stop a turn.
//
// A write the conversation would not admit — the version has moved, or the pair
// of states is not one the line may pass through — is 409 with the entry as it
// stands, so the page can show the line as it really is. Everything else the
// conversation refuses is a request this server could read and would not act on.
func (h *handler) partnerProposal(w http.ResponseWriter, r *http.Request, turn, at string) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	index, err := strconv.Atoi(at)
	if err != nil || index < 0 {
		w.WriteHeader(http.StatusBadRequest)
		writeActRefusal(w, "request", "an action is named by its place in the answer, counting from zero")
		return
	}
	var body proposalBody
	if !decode(w, r, &body) {
		return
	}
	where := strings.TrimSpace(body.Conversation)
	held, err := h.info.Partner.ProposedIn(h.partnerHuman(r), where, turn, index,
		body.Version, body.State, body.Words, body.Attempt)
	if err != nil {
		var conflict *partner.ProposalConflict
		if errors.As(err, &conflict) {
			w.WriteHeader(http.StatusConflict)
			writeProposalRefusal(w, "state", conflict.Error(), conflict.Held)
			return
		}
		// The line has another owner: the act this write answers for is not the
		// act the line is waiting for. It is the version's own refusal in
		// everything but the code — the entry travels, and the page holds its
		// result on the line and stops its run — and it is a code of its own
		// because the two are different facts about one write.
		var owner *partner.ProposalOwner
		if errors.As(err, &owner) {
			w.WriteHeader(http.StatusConflict)
			writeProposalRefusal(w, "attempt", owner.Error(), owner.Held)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		writeActRefusal(w, "request", err.Error())
		return
	}
	// The entry as it now stands goes back with the conversation, so one press
	// is one request: the page has the new version without reading again.
	snapshot, err := h.info.Partner.SnapshotIn(h.partnerHuman(r), where, partnerMessages)
	if err != nil {
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Proposal partner.Proposal `json:"proposal"`
		partner.Snapshot
	}{Proposal: held, Snapshot: snapshot})
}

// writeProposalRefusal is the one refusal that carries a record rather than only
// a sentence: the entry the caller must show instead of what it had.
//
// Two codes reach it. `state` is the version's own compare-and-set, or a pair
// the line may not pass through; `attempt` is a line another press owns. The
// page treats them the same way — it reconciles to the entry, holds its own
// result unrecorded and stops the run — and the code says which fact refused it.
func writeProposalRefusal(w http.ResponseWriter, code, reason string, held partner.Proposal) {
	_ = json.NewEncoder(w).Encode(struct {
		Error    string           `json:"error"`
		Code     string           `json:"code"`
		Proposal partner.Proposal `json:"proposal"`
	}{Error: reason, Code: code, Proposal: held})
}

// partnerStop stops the running turn. It answers the snapshot, so the page
// reads the settled state from the same request that asked for it.
func (h *handler) partnerStop(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body struct {
		Conversation string `json:"conversation"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := h.info.Partner.Stop(r.Context(), id); err != nil {
		writeFailure(w, err.Error())
		return
	}
	snapshot, err := h.info.Partner.SnapshotIn(h.partnerHuman(r), strings.TrimSpace(body.Conversation), partnerMessages)
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
