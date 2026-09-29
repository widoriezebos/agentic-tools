package httpd

// The loop from the room (g1-s66 §6): a design's critique, sent, read and
// answered from the design's own page.
//
// POST /api/design/<path>/review runs `design review` for the design at that
// checkout-relative path, under the launch route's session rule: a live
// session whose proof stands for this checkout, never the boot proof, because
// what it starts spends the critique lane's time and the goal's budget. It
// carries the funding goal and the reader budget the sheet showed, and for
// Answer the round the examination the round's own decisions file answers. It
// answers what the engine did, in the engine's words; a refusal is one of
// them, and is shown as it was said.
//
// GET on the same address reads the chain: its state, and each round's
// findings from that round's retained return with its decisions file as it
// stands. POST /api/design/<path>/review/<round>/decisions writes one row
// into that round's decisions file. The design names that write PUT; every
// write this server serves is a POST, by the policy every write shares, so
// it is a POST here as well.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

const (
	designPrefix         = "/api/design/"
	designReviewSuffix   = "/review"
	designDecisionsPart  = "/decisions"
	routeDesignReview    = "design-review"
	routeDesignDecision  = "design-decision"
	designNeedsSignIn    = "a design's critique and its decisions are your own acts, so they need a signed-in human; " + signInRemedy
	designUnknownRefusal = "this engine was built without the design loop"
)

// DesignAsked is Send to critique, or Answer the round when After names the
// examination whose decisions are answered.
type DesignAsked struct {
	Goal      string `json:"goal"`
	ToolCalls int    `json:"toolCalls"`
	After     int64  `json:"after"`
}

// DesignAnswer is what the engine did, in its own words: the outcome, the
// sentence, what it says is decided next, and the lines it printed beside
// them (a close owner's refusal, the obligations a close published).
type DesignAnswer struct {
	Outcome  string   `json:"outcome"`
	Summary  string   `json:"summary"`
	Decision string   `json:"decision,omitempty"`
	Lines    []string `json:"lines,omitempty"`
	Chain    string   `json:"chain,omitempty"`
}

// DesignLoop is one design's critique as the page reads it.
type DesignLoop struct {
	Design string `json:"design"`
	// ToolCalls is the reader budget the sheet prefills: the project's
	// review.design.tool-calls.
	ToolCalls int `json:"toolCalls"`
	// Chain is the critique chain's root, or "" when the design has none.
	Chain string `json:"chain"`
	Goal  string `json:"goal,omitempty"`
	// State is none, reading, deciding, answered, closed, or ended when the
	// newest examination ended without a return.
	State  string `json:"state"`
	Status string `json:"status,omitempty"`
	Round  int64  `json:"round"`
	Limit  int64  `json:"limit"`
	// Critic is the model the newest examination runs, where its record says.
	Critic string        `json:"critic,omitempty"`
	Rounds []DesignRound `json:"rounds"`
	// Chains says how many chains the design has when it has several; the
	// engine refuses to choose between them, and so does the page.
	Chains int `json:"chains,omitempty"`
}

// DesignRound is one examination: its findings from its own return, and its
// decisions file as it stands.
type DesignRound struct {
	Round    int64           `json:"round"`
	Findings []DesignFinding `json:"findings"`
	// Prose is the return as it was written, where it carries no findings
	// the page can read; such a round offers no presses.
	Prose     string      `json:"prose,omitempty"`
	Decisions []DesignRow `json:"decisions"`
	// Answerable is every finding decided, on the newest completed round of
	// an open chain: Answer the round may be pressed.
	Answerable bool `json:"answerable"`
}

// DesignFinding is one finding as the return carries it, and the change it
// asks for and its tests only where the return's own words carry them.
type DesignFinding struct {
	ID       string `json:"id"`
	Severity string `json:"severity"`
	Material bool   `json:"material"`
	Claim    string `json:"claim"`
	Evidence string `json:"evidence"`
	Change   string `json:"change,omitempty"`
	Tests    string `json:"tests,omitempty"`
}

// DesignRow is one decided row of a round's decisions file.
type DesignRow struct {
	Finding     string `json:"finding"`
	Disposition string `json:"disposition"`
	Reasoning   string `json:"reasoning"`
	Amendment   string `json:"amendment"`
}

// DesignRefusal is a read or a row this server refuses in words.
type DesignRefusal struct {
	Code    string
	Message string
}

func (r *DesignRefusal) Error() string { return r.Message }

// designRouteOf reports which design-loop write a POST to this path is, with
// the design's id and, for a decisions row, the round.
func designRouteOf(path string) (written, bool) {
	rest, beneath := strings.CutPrefix(path, designPrefix)
	if !beneath {
		return written{}, false
	}
	if id, ok := strings.CutSuffix(rest, designReviewSuffix); ok && id != "" {
		return written{route: routeDesignReview, id: id}, true
	}
	if head, ok := strings.CutSuffix(rest, designDecisionsPart); ok {
		at := strings.LastIndex(head, designReviewSuffix+"/")
		if at > 0 {
			round := head[at+len(designReviewSuffix)+1:]
			if _, err := strconv.ParseInt(round, 10, 64); err == nil {
				return written{route: routeDesignDecision, id: head[:at], at: round}, true
			}
		}
	}
	return written{}, false
}

// designReadOf is the design a GET of the loop names.
func designReadOf(path string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, designPrefix)
	if !beneath {
		return "", false
	}
	id, ok := strings.CutSuffix(rest, designReviewSuffix)
	return id, ok && id != ""
}

func (h *handler) designLoop(w http.ResponseWriter, id string) {
	w.Header().Set("Content-Type", "application/json")
	if h.info.DesignLoop == nil {
		writeFailure(w, designUnknownRefusal)
		return
	}
	loop, err := h.info.DesignLoop(id)
	h.answerDesign(w, loop, err)
}

func (h *handler) designReview(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.DesignReview == nil {
		writeFailure(w, designUnknownRefusal)
		return
	}
	signed, ok := h.designSession(w, r)
	if !ok {
		return
	}
	var body DesignAsked
	if !decode(w, r, &body) {
		return
	}
	body.Goal = strings.TrimSpace(body.Goal)
	answer, err := h.info.DesignReview(signed, id, body)
	if err != nil {
		var refusal *DesignRefusal
		if errors.As(err, &refusal) {
			w.WriteHeader(http.StatusConflict)
			writeActRefusal(w, refusal.Code, refusal.Message)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(answer)
}

func (h *handler) designDecision(w http.ResponseWriter, r *http.Request, id, at string) {
	if h.info.DesignDecide == nil {
		writeFailure(w, designUnknownRefusal)
		return
	}
	if _, ok := h.designSession(w, r); !ok {
		return
	}
	var body DesignRow
	if !decodeDocument(w, r, &body) {
		return
	}
	round, _ := strconv.ParseInt(at, 10, 64)
	loop, err := h.info.DesignDecide(id, round, DesignRow{
		Finding: strings.TrimSpace(body.Finding), Disposition: strings.TrimSpace(body.Disposition),
		Reasoning: strings.TrimSpace(body.Reasoning), Amendment: strings.TrimSpace(body.Amendment),
	})
	h.answerDesign(w, loop, err)
}

// designSession is the launch route's rule, said in this act's own words.
func (h *handler) designSession(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	signed, liveness := h.signedIn(r)
	if liveness == session.Live && signed != nil && h.info.Sessions != nil && signed.Proof.SessionValidFor(h.info.Sessions.Root()) {
		return signed, true
	}
	w.WriteHeader(http.StatusForbidden)
	switch {
	case liveness == session.Expired:
		writeSignInRefusal(w, "expired", expiredRefusal)
	case liveness == session.Live:
		writeSignInRefusal(w, "session", launchNeedsProof)
	default:
		writeSignInRefusal(w, "session", designNeedsSignIn)
	}
	return nil, false
}

func (h *handler) answerDesign(w http.ResponseWriter, loop DesignLoop, err error) {
	if err == nil {
		_ = json.NewEncoder(w).Encode(loop)
		return
	}
	var refusal *DesignRefusal
	if errors.As(err, &refusal) {
		status := http.StatusConflict
		if refusal.Code == "not-found" {
			status = http.StatusNotFound
		}
		w.WriteHeader(status)
		writeActRefusal(w, refusal.Code, refusal.Message)
		return
	}
	writeFailure(w, err.Error())
}
