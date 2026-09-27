package httpd

// The abandon route: POST /api/backlog/goals/<id>/abandon.
//
// What the act does to the ledger is proved in internal/ui/act, so the actor
// here is a recorder and every case asserts the one thing only this layer can:
// whether the engine was reached, and with what. The one refusal this route
// owns is the empty reason, which travels blank to the engine exactly as a
// park's does; every other refusal is the engine's and comes back in the
// engine's own words.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// abandoned is one abandon as the route passed it on.
type abandoned struct {
	ID        string
	Because   string
	Successor string
}

// abandons records what reached the engine through the abandon route, and
// refuses with whatever it was given. It is its own recorder for the reason the
// editor's is: the board's recorder holds the acts the board itself offers, and
// no page offers this one.
type abandons struct {
	seen    []abandoned
	hands   []string
	refusal error
}

func (rec *abandons) serving(authority AuthorityInfo) http.Handler {
	return New(rec.info(authority), loopback(), testBundle())
}

func (rec *abandons) info(authority AuthorityInfo) Info {
	info := (&acted{authorized: authority}).acting()
	info.Abandon = func(signed *session.Session, id, because, successor string) error {
		hand := ""
		if signed != nil {
			hand = signed.Human
		}
		rec.hands = append(rec.hands, hand)
		rec.seen = append(rec.seen, abandoned{ID: id, Because: because, Successor: successor})
		return rec.refusal
	}
	return info
}

// The body's two fields, and the answer every act route gives.
func TestTheAbandonRouteCarriesTheReasonAndTheSuccessor(t *testing.T) {
	t.Parallel()

	rec := &abandons{}
	served := rec.serving(proven())

	response := post(t, served, "/api/backlog/goals/old-idea/abandon",
		`{"because":"superseded by the seat inventory","successor":"new-idea"}`, nil)

	testutil.Require(t, "status of an abandon", response.Code, http.StatusOK)
	testutil.Expect(t, "what the engine was asked for", rec.seen, []abandoned{
		{ID: "old-idea", Because: "superseded by the seat inventory", Successor: "new-idea"},
	})
	// It answers the board as every other act does, so the page that acted
	// reads the ledger as it then stood rather than guessing.
	payload := decodeBacklog(t, response.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
}

// A successor nobody named is absent rather than invented, and the two say the
// same thing to the engine: no successor.
func TestTheAbandonRouteSendsNoSuccessorWhereNobodyNamedOne(t *testing.T) {
	t.Parallel()

	rec := &abandons{}
	served := rec.serving(proven())

	for _, body := range []string{
		`{"because":"nobody will work it"}`,
		`{"because":"nobody will work it","successor":"  "}`,
	} {
		response := post(t, served, "/api/backlog/goals/old-idea/abandon", body, nil)
		testutil.Expect(t, "status of "+body, response.Code, http.StatusOK)
	}
	testutil.Expect(t, "what the engine was asked for both times", rec.seen, []abandoned{
		{ID: "old-idea", Because: "nobody will work it", Successor: ""},
		{ID: "old-idea", Because: "nobody will work it", Successor: ""},
	})
}

// The reason is the one field the engine requires, and a route that invented a
// default would be writing a why nobody wrote — so a blank one reaches the
// engine blank and the engine refuses it.
func TestAnAbandonWithNoReasonReachesTheEngineBlank(t *testing.T) {
	t.Parallel()

	rec := &abandons{refusal: &act.Refusal{Kind: act.KindRequest, Code: "no-reason",
		Message: "abandon needs its reason — a goal that will never be worked owes the reader why"}}
	served := rec.serving(proven())

	response := post(t, served, "/api/backlog/goals/old-idea/abandon", `{"because":"   "}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the reason travelled as nothing", rec.seen,
		[]abandoned{{ID: "old-idea", Because: "", Successor: ""}})
	testutil.Expect(t, "the engine's own words reach the page", actRefusal(t, response).Error,
		"abandon needs its reason — a goal that will never be worked owes the reader why")
}

// Every refusal an abandon can draw is the engine's, and each one arrives with
// the status that says what a human can do about it. The dependents refusal is
// the one the card is written around: it names the terminal forms.
func TestTheEnginesAbandonRefusalsReachThePageInItsOwnWords(t *testing.T) {
	t.Parallel()

	for name, refusal := range map[string]*act.Refusal{
		"a live dependent nobody covered": {Kind: act.KindEngine, Code: "rejected",
			Message: "goal g1-s45 is blocked by old-idea; re-point it with --successor, " +
				"waive it with --waive g1-s45=<reason>, or abandon it with --also g1-s45"},
		"a successor that is not live": {Kind: act.KindEngine, Code: "rejected",
			Message: "carried must name a live successor"},
		"a job still naming the goal": {Kind: act.KindEngine, Code: "refused",
			Message: "goal abandon refuses while non-terminal jobs name the abandoned set: job-live; " +
				"stop each dispatch job (metasystem work stop j2:job-live), then repeat the abandon"},
	} {
		rec := &abandons{refusal: refusal}
		served := rec.serving(proven())

		response := post(t, served, "/api/backlog/goals/old-idea/abandon",
			`{"because":"nobody will work it"}`, nil)

		testutil.Expect(t, name+" answers a conflict", response.Code, http.StatusConflict)
		testutil.Expect(t, name+" keeps the engine's sentence",
			actRefusal(t, response).Error, refusal.Message)
	}
}

// The policy of every act route, and nothing of its own: POST from this origin,
// a bounded body with no unknown fields, and a hand behind it. Neither waive
// nor also is a field this body takes, so a proposal that smuggled one is
// refused before anything is published.
func TestTheAbandonRouteTakesThePolicyOfEveryActRoute(t *testing.T) {
	t.Parallel()

	const path = "/api/backlog/goals/old-idea/abandon"
	rec := &abandons{}
	served := rec.serving(proven())

	got := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
	testutil.Expect(t, "a read of the abandon route", got.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the verb it takes", got.Header().Get("Allow"), "POST")

	foreign := post(t, served, path, `{"because":"nobody will work it"}`,
		map[string]string{"Origin": "http://attacker.invalid"})
	testutil.Expect(t, "a cross-origin abandon", foreign.Code, http.StatusForbidden)

	for _, body := range []string{
		`{"because":"nobody will work it","who":"somebody"}`,
		`{"because":"nobody will work it","waive":"g1-s45=it no longer applies"}`,
		`{"because":"nobody will work it","also":"g1-s45"}`,
	} {
		unknown := post(t, served, path, body, nil)
		testutil.Expect(t, "an unknown field in "+body, unknown.Code, http.StatusBadRequest)
	}

	nameless := post(t, served, strings.Replace(path, "old-idea", "", 1),
		`{"because":"nobody will work it"}`, nil)
	testutil.Expect(t, "a path naming no goal", nameless.Code, http.StatusMethodNotAllowed)

	testutil.Expect(t, "the engine was never reached", len(rec.seen), 0)
}

// A server nothing proves writes nothing, and says the remedy is in the page.
func TestAnUnprovenServerRefusesAbandonWithTheProofsOwnReason(t *testing.T) {
	t.Parallel()

	rec := &abandons{}
	served := rec.serving(agentStarted())

	response := post(t, served, "/api/backlog/goals/old-idea/abandon",
		`{"because":"nobody will work it"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	refusal := actRefusal(t, response)
	testutil.Expect(t, "the remedy is in this page", refusal.SignIn, true)
	if !strings.Contains(refusal.Error, "claude-code") {
		t.Fatalf("the refusal does not carry the proof's own reason: %q", refusal.Error)
	}
	testutil.Expect(t, "the engine was never reached", len(rec.seen), 0)
}

// An engine built without this act refuses this route and no others: the board
// can still approve and park.
func TestAnEngineWithoutAbandonRefusesOnlyThatRoute(t *testing.T) {
	t.Parallel()

	served := New((&acted{authorized: proven()}).acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/old-idea/abandon",
		`{"because":"nobody will work it"}`, nil)
	testutil.Expect(t, "status of an abandon nothing can make", response.Code, http.StatusInternalServerError)

	parked := post(t, served, "/api/backlog/goals/old-idea/park", `{"because":"not now"}`, nil)
	testutil.Expect(t, "the park still works", parked.Code, http.StatusOK)
}

// The hand is the signed-in human's where the request carries a session, which
// is what makes the ledger name them rather than the terminal that booted the
// server.
func TestTheAbandonRouteActsUnderTheSignedInHand(t *testing.T) {
	t.Parallel()

	rec := &abandons{}
	signing := newSigning(t, "Wido", workingSecret)
	info := rec.info(proven())
	info.Sessions = signing.store
	info.PartnerConfigured = true
	served := New(info, loopback(), testBundle())

	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)

	response := post(t, served, "/api/backlog/goals/old-idea/abandon",
		`{"because":"nobody will work it"}`, carrying(mintedCookie(t, signedIn).Value))

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the hand the act published under", rec.hands, []string{"Wido"})
}
