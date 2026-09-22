package httpd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
)

// The board's two acts, at the boundary: who may reach them at all, what a
// body has to be, how a refusal comes back, and what a confirmed act answers
// with. What the acts do to the ledger is proved in internal/ui/act, so the
// actor here is a recorder and every case can assert the one thing only this
// layer can — whether the engine was reached.

type acted struct {
	approvals  [][2]any
	withdrawn  [][2]string
	refusal    error
	budgetLaw  map[string]goalbudget.Budget
	authorized AuthorityInfo
}

func (rec *acted) acting() Info {
	return Info{
		Observe:   readObservation,
		Authority: rec.authorized,
		Approve: func(id string, budget goalbudget.Budget) error {
			rec.approvals = append(rec.approvals, [2]any{id, budget})
			return rec.refusal
		},
		Withdraw: func(id, reason string) error {
			rec.withdrawn = append(rec.withdrawn, [2]string{id, reason})
			return rec.refusal
		},
		BudgetDefaults: func() (map[string]goalbudget.Budget, error) { return rec.budgetLaw, nil },
	}
}

func (rec *acted) reached() int { return len(rec.approvals) + len(rec.withdrawn) }

func proven() AuthorityInfo {
	return AuthorityInfo{Proven: true, Human: "Wido"}
}

func agentStarted() AuthorityInfo {
	return AuthorityInfo{
		Reason: "the interface was started by an agent process (claude-code); " + act.Restart,
	}
}

const wholeBudget = `{"elapsedLimit":"4h","attemptLimit":6,"reservedJobMinutesLimit":720,"activeJobLimit":1,"reviewRoundLimit":2}`

type refused struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

func actRefusal(t *testing.T, response *httptest.ResponseRecorder) refused {
	t.Helper()
	var body refused
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("the refusal is not the shape every refusal shares: %v\n%s", err, response.Body.Bytes())
	}
	return body
}

// A proven server approves, and answers with the backlog as it now stands, so
// the board moves the card because the ledger moved rather than because the
// browser asked.
func TestApproveRouteActsAndAnswersWithTheRefreshedBacklog(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "what the engine was asked for", rec.approvals, [][2]any{{"waiting", goalbudget.Budget{
		ElapsedLimit: "4h", AttemptLimit: 6, ReservedJobMinutesLimit: 720, ActiveJobLimit: 1, ReviewRoundLimit: 2,
	}}})
	payload := decodeBacklog(t, response.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
	var authority struct {
		Proven bool   `json:"proven"`
		Human  string `json:"human"`
	}
	testutil.Require(t, "decode the authority", json.Unmarshal(payload["authority"], &authority), nil)
	testutil.Expect(t, "who the board is told it acts as", authority, struct {
		Proven bool   `json:"proven"`
		Human  string `json:"human"`
	}{true, "Wido"})
}

func TestWithdrawRouteCarriesTheHumansReason(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/withdraw", `{"reason":"the design is not settled"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "what the engine was asked for", rec.withdrawn,
		[][2]string{{"waiting", "the design is not settled"}})
}

// The server an agent started cannot act as anybody. It answers 403 with what
// the proof actually found and the one command that repairs it, and the
// engine is never reached.
func TestAnUnprovenServerRefusesBothActsWithTheProofsOwnReason(t *testing.T) {
	t.Parallel()

	for path, body := range map[string]string{
		"/api/backlog/goals/waiting/approve":  wholeBudget,
		"/api/backlog/goals/waiting/withdraw": `{"reason":"no"}`,
	} {
		rec := &acted{authorized: agentStarted()}
		served := New(rec.acting(), loopback(), testBundle())

		response := post(t, served, path, body, nil)

		testutil.Require(t, "status for "+path, response.Code, http.StatusForbidden)
		refusal := actRefusal(t, response)
		testutil.Expect(t, "the reason for "+path, refusal.Error, agentStarted().Reason)
		testutil.Expect(t, "the code for "+path, refusal.Code, "unproven")
		testutil.Expect(t, "the engine was not reached for "+path, rec.reached(), 0)
	}
}

// The engine's refusal is the answer: its own sentence, its own code, and a
// status that says what a human can do about it.
func TestAnEngineRefusalComesBackWithItsOwnWordsAndCode(t *testing.T) {
	t.Parallel()

	rec := &acted{
		authorized: proven(),
		refusal:    &act.Refusal{Kind: act.KindEngine, Code: "CONFLICT", Message: "goal waiting has no standing approval to withdraw"},
	}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/withdraw", `{"reason":"no"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusConflict)
	testutil.Expect(t, "the refusal", actRefusal(t, response),
		refused{"goal waiting has no standing approval to withdraw", "CONFLICT"})
}

// The budget is the human's, in full. A tuple that is not a budget is the
// request's own fault and never reaches the engine.
func TestAnIncompleteBudgetIsRefusedBeforeTheEngine(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/approve",
		`{"elapsedLimit":"4h","attemptLimit":0,"reservedJobMinutesLimit":720,"activeJobLimit":1,"reviewRoundLimit":2}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the code", actRefusal(t, response).Code, "budget")
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// A field this route does not take is a body it does not read: the four
// limits and the rounds are the whole tuple, and nothing rides beside them.
func TestAnUnknownFieldIsABadRequest(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/approve",
		`{"elapsedLimit":"4h","attemptLimit":6,"reservedJobMinutesLimit":720,"activeJobLimit":1,"reviewRoundLimit":2,"by":"someone else"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// The acts take POST and the same origin policy every other write takes.
func TestTheActsTakeOnlyPostFromThisOrigin(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	got := request(t, served, http.MethodGet, "/api/backlog/goals/waiting/approve", "127.0.0.1:7878", nil)
	testutil.Expect(t, "a read of an act", got.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the verb it takes", got.Header().Get("Allow"), "POST")

	foreign := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget,
		map[string]string{"Origin": "http://attacker.invalid"})
	testutil.Expect(t, "a cross-origin act", foreign.Code, http.StatusForbidden)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// The prefix alone names no goal, and neither does a path with a segment in
// the id: both are the 404 every unserved path under a reserved prefix gets.
func TestAPathThatNamesNoGoalIsNotAnAct(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	for _, path := range []string{"/api/backlog/goals//approve", "/api/backlog/goals/a/b/approve", "/api/backlog/goals/waiting"} {
		response := post(t, served, path, wholeBudget, nil)
		testutil.Expect(t, "status for "+path, response.Code, http.StatusMethodNotAllowed)
	}
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// The board prefills the approval sheet from the project's budget law, so the
// law rides in the payload the board already reads.
func TestTheBacklogCarriesTheProjectsBudgetLawAndItsAuthority(t *testing.T) {
	t.Parallel()

	rec := &acted{
		authorized: agentStarted(),
		budgetLaw: map[string]goalbudget.Budget{
			"3": {ElapsedLimit: "8h", AttemptLimit: 10, ReservedJobMinutesLimit: 1200, ActiveJobLimit: 1, ReviewRoundLimit: 3},
		},
	}
	served := New(rec.acting(), loopback(), testBundle())

	response := request(t, served, http.MethodGet, "/api/backlog", "127.0.0.1:7878", nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	payload := decodeBacklog(t, response.Result())
	var law map[string]goalbudget.Budget
	testutil.Require(t, "decode the law", json.Unmarshal(payload["budgetDefaults"], &law), nil)
	testutil.Expect(t, "the law by tier", law, rec.budgetLaw)
	var authority struct {
		Proven bool   `json:"proven"`
		Reason string `json:"reason"`
	}
	testutil.Require(t, "decode the authority", json.Unmarshal(payload["authority"], &authority), nil)
	testutil.Expect(t, "the board is told it cannot act", authority.Proven, false)
	testutil.Expect(t, "and why", authority.Reason, agentStarted().Reason)
}

// An engine built without the acts says so rather than pretending to refuse
// on the ledger's behalf.
func TestAnEngineWithoutTheActsSaysSo(t *testing.T) {
	t.Parallel()

	served := New(Info{Observe: readObservation, Authority: proven()}, loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, nil)

	testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "the reason", actRefusal(t, response).Error, "this engine was built without the backlog's acts")
}
