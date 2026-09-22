package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
)

// The board's four acts.
//
// Two are the drag-and-drop moves between lanes that are real: To Do to Ready
// for Work is goal approve, and Ready for Work back to To Do is goal
// unapprove. The third is the drag that happens inside a lane, which is a
// re-rank rather than a move: goal set-priority. The fourth is the one act
// that has no card to start from, because it makes one: goal open. Every
// other move on the board would need a fact this server cannot manufacture —
// a seat's claim, a park with its reason, a landing — and is refused in the
// browser without ever reaching here.
//
// All four take the same policy every other route takes: the allowed host,
// the same-site check, the single allowed origin, POST and nothing else, a
// bounded JSON object with no unknown fields. What they add is the one thing
// no read route needs: the server's boot-time human proof. Without it they
// write nothing and answer 403 with what the proof actually found, because
// the reason is the only thing a human can act on.
const (
	// goalsPath names the collection: a POST to it opens a goal. goalsPrefix
	// is the same resource with an id beneath it, which is where the acts on
	// one goal live.
	goalsPath       = "/api/backlog/goals"
	goalsPrefix     = "/api/backlog/goals/"
	approveSuffix   = "/approve"
	withdrawSuffix  = "/withdraw"
	prioritySuffix  = "/priority"
	routeApprove    = "approve-goal"
	routeWithdraw   = "withdraw-goal"
	routePriority   = "set-goal-priority"
	routeOpen       = "open-goal"
	unprovenRefusal = "this interface cannot act as a human"
)

// AuthorityInfo is what the server's boot-time observation found, as every
// caller is told it. A proof is not in it and never travels.
type AuthorityInfo struct {
	Proven bool
	Human  string
	Reason string
}

// approveBody is the complete budget tuple a human confirmed in the sheet.
// Four limits are required; the review rounds are a member of the same tuple
// and default to none rather than to a number this server chose.
type approveBody struct {
	ElapsedLimit            string `json:"elapsedLimit"`
	AttemptLimit            int64  `json:"attemptLimit"`
	ReservedJobMinutesLimit int64  `json:"reservedJobMinutesLimit"`
	ActiveJobLimit          int64  `json:"activeJobLimit"`
	ReviewRoundLimit        *int64 `json:"reviewRoundLimit"`
}

type withdrawBody struct {
	Reason string `json:"reason"`
}

// priorityBody is where in the backlog a human put one goal: the band, and
// the one-based position in it. A null sequence appends, which is what the
// command edge does when it is given no --sequence.
type priorityBody struct {
	Priority uint8   `json:"priority"`
	Sequence *uint64 `json:"sequence"`
}

// openBody is one goal as a human stated it at intake. The four risk answers
// and their basis travel flat rather than nested, because that is how the
// sheet asks them and nothing here is a record the browser assembles.
type openBody struct {
	ID           string   `json:"id"`
	Intent       string   `json:"intent"`
	NextStep     string   `json:"nextStep"`
	Tier         uint8    `json:"tier"`
	Why          string   `json:"why"`
	Blocks       string   `json:"blocks"`
	Labels       []string `json:"labels"`
	Severity     uint8    `json:"severity"`
	Novelty      uint8    `json:"novelty"`
	Exposure     uint8    `json:"exposure"`
	Accumulation uint8    `json:"accumulation"`
	Basis        string   `json:"basis"`
}

// actRouteOf reports which act route a path names. The id is the segment
// between the prefix and the suffix; an empty id names no goal, so it is no
// route and falls through to the 404 every unserved path under a reserved
// prefix gets.
func actRouteOf(path string) (written, bool) {
	if path == goalsPath {
		return written{route: routeOpen}, true
	}
	suffixes := map[string]string{
		approveSuffix:  routeApprove,
		withdrawSuffix: routeWithdraw,
		prioritySuffix: routePriority,
	}
	for suffix, route := range suffixes {
		if id, ok := actID(path, suffix); ok {
			return written{route: route, id: id}, true
		}
	}
	return written{}, false
}

func actID(path, suffix string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, goalsPrefix)
	if !beneath {
		return "", false
	}
	id, ends := strings.CutSuffix(rest, suffix)
	if !ends || id == "" || strings.Contains(id, "/") {
		return "", false
	}
	return id, true
}

func (h *handler) approveGoal(w http.ResponseWriter, r *http.Request, id string) {
	if !h.mayAct(w) {
		return
	}
	var body approveBody
	if !decode(w, r, &body) {
		return
	}
	rounds := int64(0)
	if body.ReviewRoundLimit != nil {
		rounds = *body.ReviewRoundLimit
	}
	budget, err := goalbudget.New(body.ElapsedLimit, body.AttemptLimit,
		body.ReservedJobMinutesLimit, body.ActiveJobLimit, rounds)
	if err != nil {
		h.refuseAct(w, &act.Refusal{Kind: act.KindRequest, Code: "budget", Message: err.Error()})
		return
	}
	h.answerAct(w, h.info.Approve(id, budget))
}

func (h *handler) withdrawGoal(w http.ResponseWriter, r *http.Request, id string) {
	if !h.mayAct(w) {
		return
	}
	var body withdrawBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Withdraw(id, body.Reason))
}

func (h *handler) setGoalPriority(w http.ResponseWriter, r *http.Request, id string) {
	if !h.mayAct(w) {
		return
	}
	var body priorityBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.SetPriority(id, body.Priority, body.Sequence))
}

func (h *handler) openGoal(w http.ResponseWriter, r *http.Request) {
	if !h.mayAct(w) {
		return
	}
	var body openBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Open(act.Opened{
		ID: body.ID, Intent: body.Intent, NextStep: body.NextStep,
		Tier: body.Tier, Why: body.Why, Blocks: body.Blocks, Labels: body.Labels,
		Risk: goal.RiskRecord{
			Severity: body.Severity, Novelty: body.Novelty, Exposure: body.Exposure,
			Accumulation: body.Accumulation, Basis: body.Basis,
		},
	}))
}

// mayAct refuses before a body is read, so an unproven server parses nothing
// a caller sent. The reason is the proof's own, in full.
func (h *handler) mayAct(w http.ResponseWriter) bool {
	if h.info.Approve == nil || h.info.Withdraw == nil || h.info.SetPriority == nil || h.info.Open == nil {
		writeFailure(w, "this engine was built without the backlog's acts")
		return false
	}
	if h.info.Authority.Proven {
		return true
	}
	reason := h.info.Authority.Reason
	if reason == "" {
		reason = unprovenRefusal + "; " + act.Restart
	}
	w.WriteHeader(http.StatusForbidden)
	writeActRefusal(w, "unproven", reason)
	return false
}

// answerAct says what the ledger did. A confirmed act answers with the
// backlog as it stands after the accepted ref was carried forward, so the
// board moves the card only because the ledger moved it.
func (h *handler) answerAct(w http.ResponseWriter, err error) {
	if err == nil {
		_ = json.NewEncoder(w).Encode(h.backlogPayload())
		return
	}
	var refusal *act.Refusal
	if errors.As(err, &refusal) {
		h.refuseAct(w, refusal)
		return
	}
	writeFailure(w, err.Error())
}

func (h *handler) refuseAct(w http.ResponseWriter, refusal *act.Refusal) {
	w.WriteHeader(actStatus(refusal.Kind))
	writeActRefusal(w, refusal.Code, refusal.Message)
}

// actStatus maps a refusal to the status that says what a human can do about
// it: 403 for a server that is not the human's, 400 for a request that was
// wrong, 409 for a ledger that refuses the act in the state it is in, and 500
// for an engine that could not answer.
func actStatus(kind string) int {
	switch kind {
	case act.KindUnproven:
		return http.StatusForbidden
	case act.KindRequest:
		return http.StatusBadRequest
	case act.KindEngine:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

// writeActRefusal is the body every act refusal shares: the engine's own
// sentence, and the code it refused under where there is one. A caller that
// reads only `error` reads every refusal.
func writeActRefusal(w http.ResponseWriter, code, reason string) {
	_ = json.NewEncoder(w).Encode(struct {
		Error string `json:"error"`
		Code  string `json:"code,omitempty"`
	}{Error: reason, Code: code})
}
