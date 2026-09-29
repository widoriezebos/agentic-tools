package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/project"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
)

// The room's routes (g1-s65 §6, g1-s67 §6).
//
// Three reads for the desk, each over one review record's Reviewed line and
// answered by the review owner from the candidate's own tree: a source file at a
// range, the change index, and one file's hunks. A sitting that shapes an intent
// or a design reads the first of them from the checkout as it stands, and has no
// change for the other two. One write for the walks. The
// review's start and the room's keep ride the sitting route, and the door's
// counts ride the board.
const (
	reviewPrefix  = "/api/review/"
	sourceSuffix  = "/source"
	changesSuffix = "/changes"
	// evidenceSuffix reads a review's evidence (g1-s71 D4): the listing, or
	// with ?path= one file of it.
	evidenceSuffix = "/evidence"
	// partnerWalkPath asks one of the five walks in a review's conversation.
	partnerWalkPath  = "/api/partner/sitting/walk"
	routePartnerWalk = "partner-sitting-walk"
)

// reviewRead answers one of the desk's three reads. The record is the rest of
// the path before the read's own suffix, because a record's id is a path with
// slashes in it.
func (h *handler) reviewRead(w http.ResponseWriter, r *http.Request, rest string) {
	if record, evidence := strings.CutSuffix(rest, evidenceSuffix); evidence && record != "" {
		h.evidenceRead(w, r, record)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	record, source := strings.CutSuffix(rest, sourceSuffix)
	changes := false
	if !source {
		record, changes = strings.CutSuffix(rest, changesSuffix)
	}
	if (!source && !changes) || record == "" {
		w.WriteHeader(http.StatusNotFound)
		writeError(w, "no review read at "+r.URL.Path)
		return
	}
	if h.info.Review == nil || h.info.Document == nil {
		writeFailure(w, "this engine was built without a review reader")
		return
	}
	reviewed, kind, ok := h.reviewedOf(w, record)
	if !ok {
		return
	}
	query := r.URL.Query()
	var answer any
	var err error
	switch {
	case kind != "review" && changes:
		// A sitting that shapes a record has no change: its desk reads the
		// checkout as it stands (g1-s67 D2, §6).
		err = &review.Refusal{Reason: "a sitting on " + article(kind) + " has no change to index; " +
			"its desk reads the checkout as it stands"}
	case source:
		from, fromErr := lineOf(query.Get("from"), "from")
		to, toErr := lineOf(query.Get("to"), "to")
		switch err = errors.Join(fromErr, toErr); {
		case err == nil && kind != "review" && h.sittingOn(r, record) == nil:
			// The checkout is read for a desk, and a desk stands only in a
			// sitting of this human's on the record, so nobody reads the
			// checkout through a room they are not in.
			err = &review.Refusal{Reason: "no sitting of yours stands on " + record + "; its desk is read in its room"}
		case err == nil && kind != "review":
			answer, err = h.info.Review.AsItStands(query.Get("path"), from, to)
		case err == nil:
			answer, err = h.info.Review.Source(reviewed, query.Get("path"), from, to)
		}
	case query.Get("path") != "":
		answer, err = h.info.Review.Diff(reviewed, query.Get("path"), query.Get("since") != "")
	case query.Get("since") != "":
		answer, err = h.info.Review.ChangesSince(reviewed)
	default:
		answer, err = h.info.Review.Changes(reviewed)
	}
	if err != nil {
		var refusal *review.Refusal
		if errors.As(err, &refusal) {
			w.WriteHeader(http.StatusBadRequest)
			writeError(w, err.Error())
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answer)
}

// reviewedOf reads the record a room's desk reads for, answering the refusal
// where the record is not there or is no sitting's: a review's head, or the kind
// of a record a sitting shapes, whose desk reads the checkout (g1-s67 D2).
func (h *handler) reviewedOf(w http.ResponseWriter, record string) (review.Reviewed, string, bool) {
	document, err := h.info.Document(record)
	if err != nil {
		if errors.Is(err, project.ErrNotFound) {
			w.WriteHeader(http.StatusNotFound)
			writeError(w, "no document at "+record)
			return review.Reviewed{}, "", false
		}
		writeFailure(w, err.Error())
		return review.Reviewed{}, "", false
	}
	kind := ""
	if document.Record != nil {
		kind = document.Record.Kind
	}
	switch kind {
	case "intent", "design":
		return review.Reviewed{}, kind, true
	case "review":
	default:
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, record+" is not a record a sitting is about")
		return review.Reviewed{}, "", false
	}
	reviewed, err := review.ReviewedIn(document.Source)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, err.Error())
		return review.Reviewed{}, "", false
	}
	return reviewed, kind, true
}

// article is a record kind as a sentence names one.
func article(kind string) string {
	if strings.HasPrefix(kind, "i") {
		return "an " + kind
	}
	return "a " + kind
}

// lineOf is one line-number parameter: absent is zero, which the owner reads as
// "from the start" or "as far as the bound allows".
func lineOf(said, name string) (int, error) {
	said = strings.TrimSpace(said)
	if said == "" {
		return 0, nil
	}
	number, err := strconv.Atoi(said)
	if err != nil || number < 1 {
		return 0, &review.Refusal{Reason: name + " is a line number, and " + strconv.Quote(said) + " is not one"}
	}
	return number, nil
}

// startReview opens the review of one goal: the door where the human's review
// of it still stands, and otherwise a new review record, created with the head
// the server resolved, and a sitting on it (g1-s65 D1, D2).
func (h *handler) startReview(w http.ResponseWriter, r *http.Request, body sittingBody) {
	if h.info.CreateReview == nil || h.info.Review == nil {
		writeFailure(w, "this engine was built without a review reader")
		return
	}
	goal := strings.TrimSpace(body.Subject.ID)
	human := h.partnerHuman(r)
	if standing := h.standingReviewOf(human, goal); standing != "" {
		h.answerPartnerIn(w, r, standing)
		return
	}
	// A Start of this human's whose sitting failed to open left its record:
	// the next press opens the sitting on that one rather than making a second
	// (Sol SOL-A-05).
	if unopened := h.unopenedReviewOf(human, goal); unopened != "" {
		h.sitOnReview(w, r, human, body, unopened, "Review of "+goal)
		return
	}
	standing := h.standingOf(goal)
	if standing == review.Waiting && h.info.Sitting != nil {
		// The hold is a ledger fact, and a sitting on a waiting goal takes it
		// under the sign-in before anything is written (g1-s70 D2).
		if _, ok := h.sessionFor(w, r); !ok {
			return
		}
	}
	written, err := h.info.CreateReview(project.NewReview{
		Goal: goal, Reviewed: h.info.Review.ReviewedLine(goal, standing),
	})
	if err != nil {
		var refusal *project.Refusal
		if errors.As(err, &refusal) {
			writeRefusal(w, statusOf(refusal.Kind), refusal.Message, refusal.Problems)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	h.sitOnReview(w, r, human, body, written.Path, written.Record.Title)
}

// sitOnReview opens the review sitting on one record, answering its path as
// the draft where the opening is refused, so the next press is about it.
func (h *handler) sitOnReview(w http.ResponseWriter, r *http.Request, human string, body sittingBody, record, title string) {
	// A sitting on a goal waiting to land holds it on the ledger before the
	// room reports it open, at every tier (g1-s70 D2).
	if goalID := strings.TrimSpace(body.Subject.ID); h.standingOf(goalID) == review.Waiting && !h.holdForSitting(w, r, goalID, record) {
		return
	}
	subject := partner.Subject{Kind: partner.SubjectRecord, ID: record, Title: title}
	if _, err := h.info.Partner.Sit(r.Context(), human, subject, partner.PurposeReview, body.About); err != nil {
		h.refuseTurn(w, err, record)
		return
	}
	h.answerPartnerIn(w, r, record)
}

// unopenedReviewOf is a review record of one goal whose sitting this human
// began and never opened, or "".
func (h *handler) unopenedReviewOf(human, goal string) string {
	if h.info.Project == nil {
		return ""
	}
	pane, err := h.info.Project()
	if err != nil {
		return ""
	}
	for _, record := range pane.Records {
		if record.Kind != "review" || !slices.Contains(record.Goals, goal) {
			continue
		}
		if h.info.Partner.Unopened(human, record.Path) {
			return record.Path
		}
	}
	return ""
}

// standingOf is where a goal stands on the board: waiting to land, done, or
// anywhere else — which decides what its review reads.
func (h *handler) standingOf(goal string) review.Standing {
	if h.info.Observe == nil {
		return review.Elsewhere
	}
	board := backlogOf(h.info.Observe())
	for _, row := range append(plainRows(board.Rows), plainRows(board.Closed)...) {
		if row.ID != goal {
			continue
		}
		switch row.Lane {
		case backlog.LaneReview:
			return review.Waiting
		case backlog.LaneDone:
			return review.Done
		}
	}
	return review.Elsewhere
}

// standingReviewOf is the record of this human's review of one goal that still
// stands, or "".
func (h *handler) standingReviewOf(human, goal string) string {
	standing, err := h.info.Partner.Standing(human)
	if err != nil || h.info.Document == nil {
		return ""
	}
	for _, sitting := range standing {
		if sitting.Purpose != partner.PurposeReview {
			continue
		}
		document, err := h.info.Document(sitting.Subject.ID)
		if err != nil || document.Record == nil {
			continue
		}
		for _, named := range document.Record.Goals {
			if named == goal {
				return sitting.Subject.ID
			}
		}
	}
	return ""
}

// walkBody is one walk: which review, which of the five, and where the human
// was.
type walkBody struct {
	Conversation string       `json:"conversation"`
	Part         string       `json:"part"`
	About        partner.Page `json:"about"`
}

// partnerWalk asks one of the five walks (D6), and answers the room's
// conversation with the walk already in it.
func (h *handler) partnerWalk(w http.ResponseWriter, r *http.Request) {
	if h.info.Partner == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		writeActRefusal(w, "partner", h.partnerRefusal())
		return
	}
	var body walkBody
	if !decodeDocument(w, r, &body) {
		return
	}
	if _, err := h.info.Partner.WalkWith(r.Context(), h.partnerHuman(r), body.Conversation, body.Part, body.About, h.candidateFor(body.Conversation, body.Part)); err != nil {
		h.refuseTurn(w, err, "")
		return
	}
	h.answerPartnerIn(w, r, body.Conversation)
}

// reviewDoor is one review of one goal as the board's card shows it: the door
// line's counts, read from the record (Astra S65-01), and whether it stands with
// the time its room was last kept.
type reviewDoor struct {
	Goal         string `json:"goal"`
	Record       string `json:"record"`
	Title        string `json:"title"`
	Findings     int    `json:"findings"`
	Unanswered   int    `json:"unanswered"`
	Standing     bool   `json:"standing"`
	SteppedOutAt string `json:"steppedOutAt,omitempty"`
}

// reviewDoors is every review record naming a goal, with its door. It is best
// effort: a project this server cannot read costs the board its door lines and
// never a card.
func (h *handler) reviewDoors(r *http.Request) []reviewDoor {
	doors := []reviewDoor{}
	if h.info.Project == nil {
		return doors
	}
	pane, err := h.info.Project()
	if err != nil {
		return doors
	}
	if h.info.Partner != nil && r != nil {
		if standing, err := h.info.Partner.Standing(h.partnerHuman(r)); err == nil {
			for _, sitting := range standing {
				pane.MarkStandingAt(sitting.Subject.ID, keptAt(sitting))
			}
		}
	}
	rows := map[string]project.Sitting{}
	for _, row := range pane.Sittings {
		rows[row.Record.Path] = row
	}
	for _, record := range pane.Records {
		if record.Kind != "review" {
			continue
		}
		row := rows[record.Path]
		for _, goal := range record.Goals {
			doors = append(doors, reviewDoor{
				Goal: goal, Record: record.Path, Title: record.Title,
				Findings: row.Counts.Findings, Unanswered: row.Counts.Unanswered,
				Standing: row.Standing, SteppedOutAt: row.SteppedOutAt,
			})
		}
	}
	return doors
}

// keptAt is when a standing sitting's room was last kept, or "".
func keptAt(sitting partner.Sitting) string {
	if sitting.Room == nil {
		return ""
	}
	return sitting.Room.At
}

// evidenceRead answers the evidence read of one review (g1-s71 D4, §6): the
// listing of what its record's Evidence path holds, or one file of it by its
// evidence-relative path — an image as itself, text as its lines. A record a
// sitting shapes has no evidence read: the Evidence path a review names is the
// one the Behaves walk asks about.
func (h *handler) evidenceRead(w http.ResponseWriter, r *http.Request, record string) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.Review == nil || h.info.Document == nil {
		writeFailure(w, "this engine was built without a review reader")
		return
	}
	named, ok := h.evidenceOf(w, record)
	if !ok {
		return
	}
	var answer any
	var err error
	file := r.URL.Query().Get("path")
	if !r.URL.Query().Has("path") {
		answer, err = h.info.Review.EvidenceList(named)
	} else {
		var read review.EvidenceFile
		read, err = h.info.Review.EvidenceFile(named, file)
		if err == nil && read.Kind == review.EvidenceImage {
			w.Header().Set("Content-Type", read.Type)
			w.Header().Set("X-Content-Type-Options", "nosniff")
			_, _ = w.Write(read.Body)
			return
		}
		answer = read
	}
	if err != nil {
		var refusal *review.Refusal
		if errors.As(err, &refusal) {
			w.WriteHeader(http.StatusBadRequest)
			writeError(w, err.Error())
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answer)
}

// evidenceOf is the Evidence path a review record's head names, answering the
// refusal where the record is not there or is not a review's.
func (h *handler) evidenceOf(w http.ResponseWriter, record string) (string, bool) {
	_, kind, ok := h.reviewedOf(w, record)
	if !ok {
		return "", false
	}
	if kind != "review" {
		w.WriteHeader(http.StatusBadRequest)
		writeError(w, "a review's record names its evidence, and "+record+" is "+article(kind)+"'s")
		return "", false
	}
	document, err := h.info.Document(record)
	if err != nil {
		writeFailure(w, err.Error())
		return "", false
	}
	return review.EvidenceIn(document.Source), true
}

// evidenceFor is the listing the Behaves walk is handed (g1-s71 D4): what the
// review's Evidence path holds, each entry as the Partner reads it, or the
// words its read was refused with.
func (h *handler) evidenceFor(source string) *partner.Evidence {
	named := review.EvidenceIn(source)
	listing, err := h.info.Review.EvidenceList(named)
	if err != nil {
		return &partner.Evidence{Path: named, Refusal: err.Error()}
	}
	entries := make([]string, 0, len(listing.Entries))
	for _, entry := range listing.Entries {
		entries = append(entries, entry.Path+" ("+entry.Kind+", "+sizeWords(entry.Size)+")")
	}
	return &partner.Evidence{Path: named, Entries: entries, Supplied: listing.Supplied, Total: listing.Total, Cut: listing.Cut}
}

// sizeWords is a file's size as a person reads it.
func sizeWords(size int64) string {
	switch {
	case size < 1024:
		return strconv.FormatInt(size, 10) + " B"
	case size < 1024*1024:
		return strconv.FormatInt((size+512)/1024, 10) + " KB"
	default:
		return strconv.FormatFloat(float64(size)/(1024*1024), 'f', 1, 64) + " MB"
	}
}
