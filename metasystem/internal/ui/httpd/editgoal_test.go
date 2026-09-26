package httpd

// The goal editor's route: POST /api/backlog/goals/<id>/edit.
//
// What the acts do to the ledger is proved in internal/ui/act, so the actor
// here is a recorder and every case asserts the one thing only this layer
// can: whether the engine was reached, and with what. The one refusal the
// route owns is the line break, because the ledger keeps an intent and a next
// step on one line each; every other refusal is the engine's and comes back
// in the engine's own words.

import (
	"net/http"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/act"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// edits records what reached the engine through the edit route, and refuses
// with whatever it was given. It is its own recorder rather than a field on
// the board's, because the board's recorder is the six acts' and this act is
// not one of them.
type edits struct {
	seen    []act.Edited
	ids     []string
	hands   []string
	refusal error
}

// serving is one handler whose edit route this recorder answers, over the
// board's own Info so that the policy under test is the policy every act
// route takes.
func (rec *edits) serving(authority AuthorityInfo) http.Handler {
	info := (&acted{authorized: authority}).acting()
	info.Edit = func(signed *session.Session, id string, edited act.Edited) error {
		hand := ""
		if signed != nil {
			hand = signed.Human
		}
		rec.hands = append(rec.hands, hand)
		rec.ids = append(rec.ids, id)
		rec.seen = append(rec.seen, edited)
		return rec.refusal
	}
	return New(info, loopback(), testBundle())
}

// The body's shape is the whole point of it: a field that was sent and a
// field that was not are two different statements, and the route must carry
// the difference rather than filling in what it did not receive.
func TestTheEditRouteCarriesOnlyTheFieldsTheSheetSent(t *testing.T) {
	t.Parallel()

	rec := &edits{}
	served := rec.serving(proven())

	response := post(t, served, "/api/backlog/goals/ui-1/edit",
		`{"nextStep":"Take it to a working end state."}`, nil)

	testutil.Require(t, "status of an edit", response.Code, http.StatusOK)
	testutil.Require(t, "the engine was reached once", len(rec.seen), 1)
	testutil.Expect(t, "which goal", rec.ids[0], "ui-1")
	testutil.Expect(t, "the next step travelled", *rec.seen[0].NextStep, "Take it to a working end state.")
	testutil.Expect(t, "the intent said nothing", rec.seen[0].Intent == nil, true)
	testutil.Expect(t, "the labels said nothing", rec.seen[0].Labels == nil, true)

	// It answers the board as every other act does, so the page that acted
	// reads the ledger as it then stood rather than guessing.
	payload := decodeBacklog(t, response.Result())
	testutil.Expect(t, "the answer is the backlog", payload["schemaVersion"] != nil, true)
}

// An emptied label list is a statement and not an absence: it clears the
// labels, and it must arrive as an empty list rather than as nothing.
func TestAnEmptiedLabelListTravelsAsAnEmptyList(t *testing.T) {
	t.Parallel()

	rec := &edits{}
	served := rec.serving(proven())

	response := post(t, served, "/api/backlog/goals/ui-1/edit", `{"labels":[]}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Require(t, "the engine was reached", len(rec.seen), 1)
	testutil.Require(t, "the labels were sent", rec.seen[0].Labels != nil, true)
	testutil.Expect(t, "and they are none", len(*rec.seen[0].Labels), 0)
}

// The one refusal this route owns. The ledger keeps each of the two lines on
// one `- Key: value` line, so a client that did not fold its breaks is
// refused before a broken record can be written.
func TestTheEditRouteRefusesALineBreakInEitherLine(t *testing.T) {
	t.Parallel()

	for name, body := range map[string]string{
		"an intent":         `{"intent":"Two\nlines."}`,
		"a next step":       `{"nextStep":"Two\nlines."}`,
		"a carriage return": `{"intent":"Two\rlines."}`,
	} {
		rec := &edits{}
		served := rec.serving(proven())

		response := post(t, served, "/api/backlog/goals/ui-1/edit", body, nil)

		testutil.Expect(t, name+" is a bad request", response.Code, http.StatusBadRequest)
		testutil.Expect(t, name+" names the code", actRefusal(t, response).Code, "one-line")
		testutil.Expect(t, name+" says what to do", actRefusal(t, response).Error, oneLineRefusal)
		testutil.Expect(t, "the engine was not reached by "+name, len(rec.seen), 0)
	}
}

// Every other refusal is the engine's, and each arrives with the status that
// says what a human can do about it. The three states the allowlist refuses
// are the ones a page shows in the engine's own sentence.
func TestTheEnginesEditRefusalsReachThePageInItsOwnWords(t *testing.T) {
	t.Parallel()

	for name, refusal := range map[string]*act.Refusal{
		"an approved goal": {Kind: act.KindEngine, Code: "rejected",
			Message: "goal ui-1 is approved: withdraw the approval, edit it, then approve it again"},
		"a claimed goal": {Kind: act.KindEngine, Code: "rejected",
			Message: "goal ui-1 is claimed by mac-b+lin-1; edit it at a terminal"},
		"a parked goal": {Kind: act.KindEngine, Code: "rejected",
			Message: "goal ui-1 is parked: return it to the queue to edit it"},
		"a label outside the grammar": {Kind: act.KindEngine, Code: "refused",
			Message: `label "Board" must match ^[a-z][a-z0-9-]{0,31}$`},
	} {
		rec := &edits{refusal: refusal}
		served := rec.serving(proven())

		response := post(t, served, "/api/backlog/goals/ui-1/edit", `{"intent":"A rewrite."}`, nil)

		testutil.Expect(t, name+" answers a conflict", response.Code, http.StatusConflict)
		testutil.Expect(t, name+" keeps the engine's sentence", actRefusal(t, response).Error, refusal.Message)
	}
}

// The act layer's own guards come back as bad requests, with the code that
// says which one refused.
func TestTheEditRoutesRequestRefusalsKeepTheirCode(t *testing.T) {
	t.Parallel()

	rec := &edits{refusal: &act.Refusal{Kind: act.KindRequest, Code: "no-change",
		Message: "an edit changes at least one of the intent, the next step or the labels"}}
	served := rec.serving(proven())

	response := post(t, served, "/api/backlog/goals/ui-1/edit", `{}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the code", actRefusal(t, response).Code, "no-change")
	testutil.Expect(t, "an empty body reached the engine saying nothing",
		rec.seen[0].Intent == nil && rec.seen[0].NextStep == nil && rec.seen[0].Labels == nil, true)
}

// The policy of every act route, and nothing of its own.
func TestTheEditRouteTakesThePolicyOfEveryActRoute(t *testing.T) {
	t.Parallel()

	const path = "/api/backlog/goals/ui-1/edit"
	rec := &edits{}
	served := rec.serving(proven())

	got := request(t, served, http.MethodGet, path, "127.0.0.1:7878", nil)
	testutil.Expect(t, "a read of the edit route", got.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the verb it takes", got.Header().Get("Allow"), "POST")

	foreign := post(t, served, path, `{"intent":"A rewrite."}`,
		map[string]string{"Origin": "http://attacker.invalid"})
	testutil.Expect(t, "a cross-origin edit", foreign.Code, http.StatusForbidden)

	unknown := post(t, served, path, `{"intent":"A rewrite.","tier":2}`, nil)
	testutil.Expect(t, "a field this act does not take", unknown.Code, http.StatusBadRequest)

	nameless := post(t, served, "/api/backlog/goals//edit", `{"intent":"A rewrite."}`, nil)
	testutil.Expect(t, "a path naming no goal", nameless.Code, http.StatusMethodNotAllowed)

	testutil.Expect(t, "the engine was not reached", len(rec.seen), 0)
}

// A server nothing proves writes nothing, and says the remedy is in the page.
func TestAnUnprovenServerRefusesAnEditWithTheProofsOwnReason(t *testing.T) {
	t.Parallel()

	reason := "the interface was started by an agent process (claude-code)"
	rec := &edits{}
	served := rec.serving(AuthorityInfo{Reason: reason})

	response := post(t, served, "/api/backlog/goals/ui-1/edit", `{"intent":"A rewrite."}`, nil)

	testutil.Expect(t, "status", response.Code, http.StatusForbidden)
	refused := actRefusal(t, response)
	testutil.Expect(t, "it carries the proof's own reason", strings.Contains(refused.Error, reason), true)
	testutil.Expect(t, "it says the remedy is in this page", refused.SignIn, true)
	testutil.Expect(t, "the engine was not reached", len(rec.seen), 0)
}

// An engine built without the editor says so, and goes on answering the acts
// it does carry: a missing act refuses its own route and no others.
func TestAnEngineWithoutTheEditorSaysSoAndStillApproves(t *testing.T) {
	t.Parallel()

	board := &acted{authorized: proven()}
	served := New(board.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/ui-1/edit", `{"intent":"A rewrite."}`, nil)
	testutil.Expect(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "it says what is missing",
		strings.Contains(actRefusal(t, response).Error, "without the backlog's edit"), true)

	approved := post(t, served, "/api/backlog/goals/ui-1/approve", wholeBudget, nil)
	testutil.Expect(t, "the acts it does carry still answer", approved.Code, http.StatusOK)
}

// A goal's "/edit" and a document's "/edit" are two routes under two
// prefixes, and neither shadows the other.
func TestAGoalsEditAndADocumentsEditAreTwoRoutes(t *testing.T) {
	t.Parallel()

	for path, route := range map[string]string{
		"/api/backlog/goals/ui-1/edit":    routeEditGoal,
		"/api/backlog/goals/ui-1/approve": routeApprove,
		"/api/backlog/goals/a/b/edit":     "",
		"/api/backlog/goals//edit":        "",
	} {
		named, ok := actRouteOf(path)
		if route == "" {
			testutil.Expect(t, path+" names no act route", ok, false)
			continue
		}
		testutil.Require(t, path+" names an act route", ok, true)
		testutil.Expect(t, path+"'s route", named.route, route)
		testutil.Expect(t, path+"'s id", named.id, "ui-1")
	}

	// The document editor's own route is untouched: it cuts a different
	// prefix, and a goal id never lives under it.
	document, ok := writeRouteOf(documentPrefix + "plans/designs/one.md" + editSuffix)
	testutil.Require(t, "a document still edits", ok, true)
	testutil.Expect(t, "and it is the document's route", document.route, routeEditDocument)
}
