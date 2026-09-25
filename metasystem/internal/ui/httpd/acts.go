package httpd

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// The board's six acts, and the Decisions page's two.
//
// Two are the drag-and-drop moves between lanes that are real: To Do to Ready
// for Work is goal approve, and Ready for Work back to To Do is goal
// unapprove. The third is the drag that happens inside a lane, which is a
// re-rank rather than a move: goal set-priority. The fourth is the one act
// that has no card to start from, because it makes one: goal open. The last
// two write the relation between two goals rather than the state of one: goal
// block and goal unblock, from the goal's own page where both directions of
// that relation are read. Every other move on the board would need a fact
// this server cannot manufacture — a seat's claim, a landing — and is refused
// in the browser without ever reaching here.
//
// Two more arrive with the Decisions queue under R-125-m1u: goal park, which
// is that page's "Not now", and goal unpark, which returns a paused goal to
// the queue. They are not board moves and the board grows no button for them;
// the engine admits this hand's session proof at the three rows the ruling
// names and at no others, so every other refusal stands exactly as it did.
//
// All eight take the same policy every other route takes: the allowed host,
// the same-site check, the single allowed origin, POST and nothing else, a
// bounded JSON object with no unknown fields. What they add is the one thing
// no read route needs: a human's proof. Two can supply one — the live browser
// session the request carries, and the server's boot-time observation, in that
// order, because a human who signed in just now is the one at the keyboard.
// Without either they write nothing and answer 403 with what both actually
// found, and with the one thing a human can do about it.
const (
	// goalsPath names the collection: a POST to it opens a goal. goalsPrefix
	// is the same resource with an id beneath it, which is where the acts on
	// one goal live.
	goalsPath       = "/api/backlog/goals"
	goalsPrefix     = "/api/backlog/goals/"
	approveSuffix   = "/approve"
	withdrawSuffix  = "/withdraw"
	prioritySuffix  = "/priority"
	blockSuffix     = "/block"
	unblockSuffix   = "/unblock"
	parkSuffix      = "/park"
	unparkSuffix    = "/unpark"
	routeApprove    = "approve-goal"
	routeWithdraw   = "withdraw-goal"
	routePriority   = "set-goal-priority"
	routeOpen       = "open-goal"
	routeBlock      = "block-goal"
	routeUnblock    = "unblock-goal"
	routePark       = "park-goal"
	routeUnpark     = "unpark-goal"
	unprovenRefusal = "this interface cannot act as a human"
	// expiredRefusal is the one refusal a human fixes without reading
	// anything: the session ran out while the page stayed open.
	expiredRefusal = "your session expired; sign in again"
	// signInRemedy is what an unproven server offers instead of the restart
	// it used to offer alone. The restart still works and is still named; the
	// code is the one that needs no terminal.
	signInRemedy = "sign in with this seat's one-time code to act as yourself"
	// partnerNeedsSignIn is what a cookie-less act is told on a seat that runs
	// a Partner. It says the rule and the remedy in one sentence, and the
	// browser opens the sign-in sheet on it exactly as it does for the others.
	partnerNeedsSignIn = "a Partner runs on this seat, so acts need a signed-in human; " + signInRemedy
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

// parkBody is the reason a human gave for pausing one goal. It is required,
// because the engine requires it: a pause without a why is a stall in
// disguise, and this route refuses an empty one rather than inventing a
// sentence the human did not write. An unpark carries nothing at all, and its
// body is an empty object like every other act's.
type parkBody struct {
	Because string `json:"because"`
}

type unparkBody struct{}

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
	ID       string `json:"id"`
	Intent   string `json:"intent"`
	NextStep string `json:"nextStep"`
	Tier     uint8  `json:"tier"`
	Why      string `json:"why"`
	// Blocks names the goals that will wait for this one. It reads a list and
	// it also reads the one string it used to be, because a browser that was
	// open when this engine was replaced still sends `"blocks": ""` or
	// `"blocks": "some-goal"`, and a decoder that rejected those would answer
	// an ordinary intake with a 400 for a field that was not even filled in.
	Blocks goalList `json:"blocks"`
	// BlockedBy names the goals this one will wait for. It is new, so it
	// reads a list alone - but it reads the same type, because two fields of
	// one relation that parsed differently would be a trap for the next
	// person to add a third.
	BlockedBy    goalList `json:"blockedBy"`
	Labels       []string `json:"labels"`
	Severity     uint8    `json:"severity"`
	Novelty      uint8    `json:"novelty"`
	Exposure     uint8    `json:"exposure"`
	Accumulation uint8    `json:"accumulation"`
	Basis        string   `json:"basis"`
}

// edgeBody is one end of one edge. The path names the goal that waits; this
// names the goal it waits for. Authority is never in here: the hand that acts
// is the one mayAct found, and a name in a body authorizes nothing.
type edgeBody struct {
	Blocker string `json:"blocker"`
}

// goalList is a field that takes goal ids either as a JSON array or as the
// one string an older browser sends, where a comma separates them. Absent,
// null, "" and [] all mean none, which is the common case and must never be
// an error.
type goalList []string

func (list *goalList) UnmarshalJSON(raw []byte) error {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "null" {
		*list = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var named []string
		if err := json.Unmarshal(raw, &named); err != nil {
			return err
		}
		*list = goalList(cleanGoalList(named))
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err != nil {
		return fmt.Errorf("a list of goals is an array of ids, or one string of ids separated by commas")
	}
	*list = goalList(cleanGoalList(strings.Split(one, ",")))
	return nil
}

// cleanGoalList drops what is not an id: blanks, whitespace around one, and a
// goal named twice. The order a human chose is kept, because the first goal
// named is the one an open's park takes its marker from.
func cleanGoalList(named []string) []string {
	cleaned := []string{}
	seen := map[string]bool{}
	for _, id := range named {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		cleaned = append(cleaned, id)
	}
	if len(cleaned) == 0 {
		return nil
	}
	return cleaned
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
		blockSuffix:    routeBlock,
		unblockSuffix:  routeUnblock,
		parkSuffix:     routePark,
		unparkSuffix:   routeUnpark,
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
	signed, may := h.mayAct(w, r)
	if !may {
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
	h.answerAct(w, h.info.Approve(signed, id, budget))
}

func (h *handler) withdrawGoal(w http.ResponseWriter, r *http.Request, id string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body withdrawBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Withdraw(signed, id, body.Reason))
}

// parkGoal and unparkGoal are the Decisions queue's "Not now" and its undo.
// The engine owns every refusal they can draw — a goal already parked, a
// claim another pair holds, a branch that is not on origin — and the route
// answers each one in the engine's own words.
func (h *handler) parkGoal(w http.ResponseWriter, r *http.Request, id string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body parkBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Park(signed, id, strings.TrimSpace(body.Because)))
}

func (h *handler) unparkGoal(w http.ResponseWriter, r *http.Request, id string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body unparkBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Unpark(signed, id))
}

func (h *handler) setGoalPriority(w http.ResponseWriter, r *http.Request, id string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body priorityBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.SetPriority(signed, id, body.Priority, body.Sequence))
}

func (h *handler) openGoal(w http.ResponseWriter, r *http.Request) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body openBody
	if !decode(w, r, &body) {
		return
	}
	// The ledger keeps a goal's intent and next step on one line each; the
	// sheet folds a break into a space, and a client that did not is refused
	// before a broken record can be written.
	if strings.ContainsAny(body.Intent, "\r\n") || strings.ContainsAny(body.NextStep, "\r\n") {
		w.WriteHeader(http.StatusBadRequest)
		writeActRefusal(w, "one-line", "a goal's intent and next step are each one line in the ledger; fold the line breaks before sending")
		return
	}
	h.answerAct(w, h.info.Open(signed, act.Opened{
		ID: body.ID, Intent: body.Intent, NextStep: body.NextStep,
		Tier: body.Tier, Why: body.Why, Blocks: body.Blocks, BlockedBy: body.BlockedBy,
		Labels: body.Labels,
		Risk: goal.RiskRecord{
			Severity: body.Severity, Novelty: body.Novelty, Exposure: body.Exposure,
			Accumulation: body.Accumulation, Basis: body.Basis,
		},
	}))
}

// blockGoal and unblockGoal write and remove one edge. The id in the path is
// always the goal that WAITS, whichever end of the relation a human pressed
// the button on, so the two directions the goal page shows reach one checked
// mutation rather than two.
func (h *handler) blockGoal(w http.ResponseWriter, r *http.Request, dependent string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body edgeBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Block(signed, dependent, strings.TrimSpace(body.Blocker)))
}

func (h *handler) unblockGoal(w http.ResponseWriter, r *http.Request, dependent string) {
	signed, may := h.mayAct(w, r)
	if !may {
		return
	}
	var body edgeBody
	if !decode(w, r, &body) {
		return
	}
	h.answerAct(w, h.info.Unblock(signed, dependent, strings.TrimSpace(body.Blocker)))
}

// mayAct reports the hand this act publishes under, and refuses before a body
// is read, so a server nothing proves parses nothing a caller sent. A live
// browser session is that hand; otherwise the boot proof is, and a nil session
// says so. The reason a refusal carries is both proofs' own, in full.
func (h *handler) mayAct(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	if h.info.Approve == nil || h.info.Withdraw == nil || h.info.SetPriority == nil || h.info.Open == nil ||
		h.info.Block == nil || h.info.Unblock == nil || h.info.Park == nil || h.info.Unpark == nil {
		writeFailure(w, "this engine was built without the backlog's acts")
		return nil, false
	}
	signed, liveness := h.signedIn(r)
	switch liveness {
	case session.Live:
		return signed, true
	case session.Expired:
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "expired", expiredRefusal)
		return nil, false
	}
	// The boot proof is the second hand this server can act under, and it is
	// the one an agent on this machine could reach: a local process sends a
	// request with no cookie, the HTTP checks pass because a non-browser
	// sends no Origin and no Sec-Fetch-Site, and the act publishes under the
	// human who started the server. That was tolerable while nothing on this
	// seat could send such a request. A Partner runtime can, so from the
	// moment one is configured the boot proof stops answering for acts and
	// every act needs a signed-in human.
	if h.info.PartnerConfigured {
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "partner", partnerNeedsSignIn)
		return nil, false
	}
	if h.info.Authority.Proven {
		return nil, true
	}
	reason := h.info.Authority.Reason
	if reason == "" {
		reason = unprovenRefusal + "; " + act.Restart
	}
	w.WriteHeader(http.StatusForbidden)
	writeSignInRefusal(w, "unproven", reason+"; and nobody is signed in here: "+signInRemedy)
	return nil, false
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

// writeSignInRefusal is the same body with the one flag that says the remedy
// is in this page rather than in a terminal: the browser opens the sign-in
// sheet on it, and retries the act once when a human signs in.
func writeSignInRefusal(w http.ResponseWriter, code, reason string) {
	_ = json.NewEncoder(w).Encode(struct {
		Error  string `json:"error"`
		Code   string `json:"code,omitempty"`
		SignIn bool   `json:"signIn"`
	}{Error: reason, Code: code, SignIn: true})
}
