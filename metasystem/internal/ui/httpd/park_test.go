package httpd

// The two routes the Decisions queue acts through: park and unpark.
//
// They are not board moves, and what is proven here is that they take the
// same policy as every other act route — the method, the origin, the bounded
// body, the hand — and that the engine's own refusal is what a human reads.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
)

func TestParkRouteCarriesTheHumansReasonAndUnparkCarriesNothing(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	served := New(rec.acting(), loopback(), testBundle())

	parked := post(t, served, "/api/backlog/goals/waiting/park",
		`{"because":"not now; the census format is still being decided"}`, nil)
	testutil.Require(t, "status of a park", parked.Code, http.StatusOK)
	testutil.Expect(t, "what the engine was asked for", rec.parked,
		[][2]string{{"waiting", "not now; the census format is still being decided"}})

	returned := post(t, served, "/api/backlog/goals/waiting/unpark", `{}`, nil)
	testutil.Require(t, "status of an unpark", returned.Code, http.StatusOK)
	testutil.Expect(t, "which goal the unpark named", rec.unparked, []string{"waiting"})

	// Both answer the board as every other act does, so the page that acted
	// reads the ledger as it then stood rather than guessing.
	payload := decodeBacklog(t, returned.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
}

// The reason is the one field a park carries and the engine requires it. A
// route that invented a default would be writing a why nobody wrote — so a
// blank one reaches the engine blank, and the engine refuses it.
func TestAParkWithNoReasonReachesTheEngineBlank(t *testing.T) {
	t.Parallel()

	rec := &acted{
		authorized: proven(),
		refusal: &act.Refusal{Kind: act.KindRequest, Code: "no-reason",
			Message: "park needs its reason — a pause without a why is a stall in disguise"},
	}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/park", `{"because":"   "}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the reason travelled as nothing", rec.parked, [][2]string{{"waiting", ""}})
	testutil.Expect(t, "the engine's own words reach the page",
		actRefusal(t, response).Error,
		"park needs its reason — a pause without a why is a stall in disguise")
}

// Every refusal a park can draw is the engine's, and each one arrives with
// the status that says what a human can do about it.
func TestTheEnginesParkRefusalsReachThePageInItsOwnWords(t *testing.T) {
	t.Parallel()

	for name, refusal := range map[string]*act.Refusal{
		"a claim another pair holds": {Kind: act.KindEngine, Code: "refused",
			Message: "goal unit: the human authority proof for park of another pair's claim is not valid for this checkout"},
		"a branch that is not on origin": {Kind: act.KindEngine, Code: "refused",
			Message: "GOAL_PARK_UNPUSHED: local goal/unit is abc while origin is <absent>; push the branch before parking"},
	} {
		rec := &acted{authorized: proven(), refusal: refusal}
		served := New(rec.acting(), loopback(), testBundle())

		response := post(t, served, "/api/backlog/goals/unit/park", `{"because":"not now"}`, nil)

		testutil.Expect(t, name+" answers a conflict", response.Code, http.StatusConflict)
		testutil.Expect(t, name+" keeps the engine's sentence",
			actRefusal(t, response).Error, refusal.Message)
	}
}

// The policy of every act route, and nothing of its own: POST from this
// origin, a bounded body, and a hand behind it.
func TestTheTwoParkRoutesTakeThePolicyOfEveryActRoute(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/api/backlog/goals/waiting/park", "/api/backlog/goals/waiting/unpark"} {
		rec := &acted{authorized: proven()}
		served := New(rec.acting(), loopback(), testBundle())

		got := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
		testutil.Expect(t, "a read of "+path, got.Code, http.StatusMethodNotAllowed)
		testutil.Expect(t, "the verb "+path+" takes", got.Header().Get("Allow"), "POST")

		foreign := post(t, served, path, `{"because":"not now"}`,
			map[string]string{"Origin": "http://attacker.invalid"})
		testutil.Expect(t, "a cross-origin act on "+path, foreign.Code, http.StatusForbidden)

		unknown := post(t, served, path, `{"because":"not now","who":"somebody"}`, nil)
		testutil.Expect(t, "an unknown field on "+path, unknown.Code, http.StatusBadRequest)

		nameless := post(t, served, strings.Replace(path, "waiting", "", 1), `{"because":"not now"}`, nil)
		testutil.Expect(t, "a path naming no goal under "+path, nameless.Code, http.StatusMethodNotAllowed)

		testutil.Expect(t, "the engine was not reached from "+path, rec.reached(), 0)
	}
}

// A server nothing proves writes nothing, and says the remedy is in the page.
func TestAnUnprovenServerRefusesParkAndUnparkWithTheProofsOwnReason(t *testing.T) {
	t.Parallel()

	reason := "the interface was started by an agent process (claude-code)"
	rec := &acted{authorized: AuthorityInfo{Reason: reason}}
	served := New(rec.acting(), loopback(), testBundle())

	for path, body := range map[string]string{
		"/api/backlog/goals/waiting/park":   `{"because":"not now"}`,
		"/api/backlog/goals/waiting/unpark": `{}`,
	} {
		response := post(t, served, path, body, nil)
		testutil.Expect(t, "status of "+path, response.Code, http.StatusForbidden)
		refused := actRefusal(t, response)
		testutil.Expect(t, path+" carries the proof's own reason",
			strings.Contains(refused.Error, reason), true)
		testutil.Expect(t, path+" says the remedy is in this page", refused.SignIn, true)
	}
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// An engine built without the two acts says so rather than half-answering.
func TestAnEngineWithoutParkAndUnparkSaysSo(t *testing.T) {
	t.Parallel()

	rec := &acted{authorized: proven()}
	info := rec.acting()
	info.Park, info.Unpark = nil, nil
	served := New(info, loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/park", `{"because":"not now"}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "what it says",
		strings.Contains(actRefusal(t, response).Error, "without the backlog's acts"), true)
}
