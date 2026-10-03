package httpd

// The verdict that does something, and the candidate the room runs (g1-s69
// §6).
//
// POST /api/backlog/goals/<id>/review performs goal review under the signed-in
// session, as the approve route performs approve, and with the launch route's
// stronger session rule: a live session whose proof stands for this checkout,
// never the boot proof. It answers the history line the act wrote, and the
// backlog as it now stands.
//
// GET /api/app/<goal>/status, and POST /api/app/<goal>/start and /stop, wrap
// the three app forms for the goal's candidate. The two writes spend this host's
// processes and ports under the human's session, so they take the same rule.

import (
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/partner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/review"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

const (
	reviewSuffix = "/review"
	routeReview  = "review-goal"

	appPrefix      = "/api/app/"
	appStatusPart  = "/status"
	appStartPart   = "/start"
	appStopPart    = "/stop"
	routeAppStart  = "app-start"
	routeAppStop   = "app-stop"
	actNeedsSignIn = "a verdict and a candidate's run are your own acts, so they need a signed-in human; " + signInRemedy
)

// Candidate is a goal's candidate run as the room's pill reads it: whether it
// lives and answers, where, since when, and the commit it runs.
type Candidate struct {
	Goal      string `json:"goal"`
	State     string `json:"state"`
	Readiness string `json:"readiness"`
	Address   string `json:"address,omitempty"`
	Commit    string `json:"commit,omitempty"`
	Since     string `json:"since,omitempty"`
	Said      string `json:"said"`
}

// CandidateRefusal is an app form refused in its own words: a goal whose
// project has no launch contract, or anything else the verb refused.
type CandidateRefusal struct {
	Code    string
	Message string
}

func (r *CandidateRefusal) Error() string { return r.Message }

// CodeNoContract is a project with no launch contract to run a candidate by.
const CodeNoContract = "no-contract"

// reviewBody is one verdict as the room performs it.
type reviewBody struct {
	Record  string `json:"record"`
	Verdict string `json:"verdict"`
	Brief   string `json:"brief"`
	Work    string `json:"work"`
	// Tip and Revision are the version and the saved record the person
	// decided on (review-findings-read-as-decisions RF-02).
	Tip      string `json:"tip"`
	Revision string `json:"revision"`
}

// sessionFor is the launch route's rule, for every act here: a live session
// whose proof stands for this checkout. The boot proof never answers.
func (h *handler) sessionFor(w http.ResponseWriter, r *http.Request) (*session.Session, bool) {
	return h.sessionWith(w, r, actNeedsSignIn)
}

// sessionWith is sessionFor telling a request with no live session what it
// needs, in the words of the act it asked for.
func (h *handler) sessionWith(w http.ResponseWriter, r *http.Request, needs string) (*session.Session, bool) {
	signed, liveness := h.signedIn(r)
	switch liveness {
	case session.Live:
	case session.Expired:
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "expired", expiredRefusal)
		return nil, false
	default:
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "session", needs)
		return nil, false
	}
	if signed == nil || h.info.Sessions == nil || !signed.Proof.SessionValidFor(h.info.Sessions.Root()) {
		w.WriteHeader(http.StatusForbidden)
		writeSignInRefusal(w, "session", launchNeedsProof)
		return nil, false
	}
	return signed, true
}

func (h *handler) reviewGoal(w http.ResponseWriter, r *http.Request, id string) {
	if h.info.Verdict == nil {
		writeFailure(w, "this engine was built without the review's verdict")
		return
	}
	signed, ok := h.sessionFor(w, r)
	if !ok {
		return
	}
	var body reviewBody
	if !decodeDocument(w, r, &body) {
		return
	}
	recorded, err := h.info.Verdict(signed, id, act.Reviewed{
		Record: strings.TrimSpace(body.Record), Verdict: strings.TrimSpace(body.Verdict),
		Brief: body.Brief, Work: strings.TrimSpace(body.Work),
		Tip: strings.TrimSpace(body.Tip), Revision: strings.TrimSpace(body.Revision),
		Branch: h.branchAtThePress(id),
	})
	if err != nil {
		var refusal *act.Refusal
		if errors.As(err, &refusal) {
			h.refuseAct(w, refusal)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(struct {
		Recorded act.Recorded   `json:"recorded"`
		Backlog  backlogPayload `json:"backlog"`
	}{Recorded: recorded, Backlog: h.backlogPayload(r)})
}

// branchAtThePress is the version of one goal that would land, read now —
// fetched from origin first, as the landing gate reads a word to land — and
// never the room's earlier read (fix round 1, F-3); "" where it cannot be read,
// which the act refuses in words.
func (h *handler) branchAtThePress(goal string) string {
	if h.info.Review == nil {
		return ""
	}
	tip, err := h.info.Review.BranchTipAtOrigin(goal)
	if err != nil {
		return ""
	}
	return tip
}

// appRouteOf is one of the two candidate writes a path names.
func appRouteOf(path string) (written, bool) {
	for suffix, route := range map[string]string{appStartPart: routeAppStart, appStopPart: routeAppStop} {
		if goal, ok := appGoal(path, suffix); ok {
			return written{route: route, id: goal}, true
		}
	}
	return written{}, false
}

func appGoal(path, suffix string) (string, bool) {
	rest, beneath := strings.CutPrefix(path, appPrefix)
	if !beneath {
		return "", false
	}
	goal, ends := strings.CutSuffix(rest, suffix)
	if !ends || goal == "" || strings.Contains(goal, "/") {
		return "", false
	}
	return goal, true
}

func (h *handler) appStatus(w http.ResponseWriter, r *http.Request, goal string) {
	w.Header().Set("Content-Type", "application/json")
	h.answerCandidate(w, goal, r.URL.Query().Get("commit"), "status")
}

func (h *handler) appAct(w http.ResponseWriter, r *http.Request, goal, action string) {
	if _, ok := h.sessionFor(w, r); !ok {
		return
	}
	var body struct {
		// Commit is the version the page shows, which the run is at
		// (review-findings-read-as-decisions RF-04, fix round 3 F-1).
		Commit string `json:"commit"`
	}
	if !decode(w, r, &body) {
		return
	}
	h.answerCandidate(w, goal, body.Commit, action)
}

// answerCandidate runs one app form for a goal's candidate: at the commit the
// page shows — the version a person is deciding on, never the review as it
// may have moved since the page read it (fix round 3, F-1) — or at the goal's
// branch as it stands where no commit is named. A commit that is not one of
// the goal's branch is refused in words, before anything runs.
func (h *handler) answerCandidate(w http.ResponseWriter, goal, commit, action string) {
	if h.info.Candidate == nil {
		writeFailure(w, "this engine was built without the candidate's run")
		return
	}
	at, refusal := h.versionToRun(goal, strings.TrimSpace(commit))
	if refusal != "" {
		w.WriteHeader(http.StatusBadRequest)
		writeActRefusal(w, "commit", refusal)
		return
	}
	run, err := h.info.Candidate(goal, at, action)
	if err != nil {
		var refusal *CandidateRefusal
		if errors.As(err, &refusal) {
			w.WriteHeader(http.StatusConflict)
			writeActRefusal(w, refusal.Code, refusal.Message)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	_ = json.NewEncoder(w).Encode(run)
}

// reviewedTipOf is the branch tip a review record's head names, or "" for a
// record that names none or cannot be read: what a review's closing deposit is
// bound to (g1-s69 D1).
func (h *handler) reviewedTipOf(record string) string {
	if h.info.Document == nil || strings.TrimSpace(record) == "" {
		return ""
	}
	document, err := h.info.Document(record)
	if err != nil || document.Record == nil || document.Record.Kind != "review" {
		return ""
	}
	reviewed, err := review.ReviewedIn(document.Source)
	if err != nil {
		return ""
	}
	return reviewed.Tip
}

// versionToRun is the commit a candidate's run is at, or why it is not run:
// a whole commit id of the goal's branch, or "" for the branch as it stands.
func (h *handler) versionToRun(goal, commit string) (string, string) {
	if commit == "" {
		return "", ""
	}
	if !wholeCommit.MatchString(commit) {
		return "", commit + " is not a whole commit id; Try it runs a version by its commit"
	}
	if h.info.Review == nil {
		return "", "this engine reads no goal branch, so it cannot say whether " + commit[:12] + " is a version of goal " + goal
	}
	if on, err := h.info.Review.OnBranch(goal, commit); err != nil || !on {
		return "", commit[:12] + " is not a version of goal " + goal + "'s branch, so it is not run here"
	}
	return commit, ""
}

// wholeCommit is a commit id written out whole, which is all a run is at.
var wholeCommit = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// candidateFor is what the Behaves walk of one review is told of the goal's
// candidate: its address and running commit as app status reads them, beside
// the tip the record names. Every other walk is told nothing.
func (h *handler) candidateFor(record, part string) *partner.Candidate {
	if part != "behaves" || h.info.Document == nil {
		return nil
	}
	document, err := h.info.Document(record)
	if err != nil || document.Record == nil || document.Record.Kind != "review" {
		return nil
	}
	reviewed, err := review.ReviewedIn(document.Source)
	if err != nil || reviewed.Goal == "" {
		return nil
	}
	candidate := &partner.Candidate{Reviewed: reviewed.Tip}
	if h.info.Review != nil {
		candidate.Evidence = h.evidenceFor(document.Source)
	}
	if h.info.Candidate != nil {
		if run, err := h.info.Candidate(reviewed.Goal, reviewed.Tip, "status"); err == nil && run.State == "running" && run.Address != "" {
			candidate.Address, candidate.Running = run.Address, run.Commit
		}
	}
	return candidate
}
