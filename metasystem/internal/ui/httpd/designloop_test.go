package httpd

// The loop from the room at the boundary (g1-s66 §6, §8): Send to critique and
// a decisions row need a signed-in session and never the boot proof; the act
// is reached with the goal, the reader budget and the answered examination
// the sheet sent, and answers the engine's own words; the chain is read at the
// same address; a row the engine's rules refuse is refused in words.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

type designing struct {
	asked    []DesignAsked
	designs  []string
	rows     []DesignRow
	rounds   []int64
	answer   DesignAnswer
	refusal  error
	sessions *session.Store
}

const designID = "plans/designs/user-interface/g1-s66-the-loop.md"

func (rec *designing) serving() Info {
	loop := DesignLoop{Design: designID, ToolCalls: 30, Chain: "rev1", State: "deciding", Round: 1, Limit: 2,
		Rounds: []DesignRound{{Round: 1, Findings: []DesignFinding{{ID: "F1", Severity: "high", Material: true, Claim: "c", Evidence: "e"}}}}}
	return Info{
		Observe:  readObservation,
		Sessions: rec.sessions,
		DesignReview: func(_ *session.Session, design string, asked DesignAsked) (DesignAnswer, error) {
			rec.designs, rec.asked = append(rec.designs, design), append(rec.asked, asked)
			return rec.answer, nil
		},
		DesignLoop: func(design string) (DesignLoop, error) {
			rec.designs = append(rec.designs, design)
			return loop, nil
		},
		DesignDecide: func(design string, round int64, row DesignRow) (DesignLoop, error) {
			rec.designs, rec.rounds, rec.rows = append(rec.designs, design), append(rec.rounds, round), append(rec.rows, row)
			if rec.refusal != nil {
				return DesignLoop{}, rec.refusal
			}
			return loop, nil
		},
	}
}

func designSignedIn(t *testing.T, rec *designing) (http.Handler, map[string]string) {
	t.Helper()
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	return served, carrying(mintedCookie(t, signedIn).Value)
}

// The boot proof never sends a design to critique nor writes a decision.
func TestTheDesignLoopRefusesEveryHandButASignedInOne(t *testing.T) {
	t.Parallel()
	rec := &designing{}
	info := rec.serving()
	info.Authority = proven()
	served := New(info, loopback(), testBundle())
	for _, path := range []string{designPrefix + designID + designReviewSuffix, designPrefix + designID + "/review/1/decisions"} {
		response := post(t, served, path, `{"goal":"g","toolCalls":30}`, nil)
		testutil.Require(t, path+" status", response.Code, http.StatusForbidden)
		testutil.Expect(t, path+" says the remedy is here", actRefusal(t, response).SignIn, true)
	}
	testutil.Expect(t, "nothing was sent or written", len(rec.asked)+len(rec.rows), 0)
}

// Send to critique reaches the act with the design, the goal and the budget,
// and Answer the round with the examination it answers; the answer is the
// engine's own words, a refusal among them.
func TestSendToCritiqueCarriesTheGoalAndTheBudget(t *testing.T) {
	t.Parallel()
	rec := &designing{answer: DesignAnswer{Outcome: "refused", Summary: "a review brief states the reader's tool-call budget, and none is configured",
		Decision: "name it with --tool-calls N"}}
	served, cookie := designSignedIn(t, rec)
	response := post(t, served, designPrefix+designID+designReviewSuffix, `{"goal":"partner-runs-the-design-loop","toolCalls":30}`, cookie)
	testutil.Require(t, "sent", response.Code, http.StatusOK)
	var answer DesignAnswer
	testutil.Require(t, "answer decodes", json.Unmarshal(response.Body.Bytes(), &answer), nil)
	testutil.Expect(t, "the engine's words", answer.Summary, rec.answer.Summary)
	testutil.Expect(t, "the design", rec.designs[0], designID)
	testutil.Expect(t, "the goal and the budget", rec.asked[0], DesignAsked{Goal: "partner-runs-the-design-loop", ToolCalls: 30})
	post(t, served, designPrefix+designID+designReviewSuffix, `{"goal":"g","toolCalls":12,"after":1}`, cookie)
	testutil.Expect(t, "the answered examination", rec.asked[1], DesignAsked{Goal: "g", ToolCalls: 12, After: 1})
	unknown := post(t, served, designPrefix+designID+designReviewSuffix, `{"goal":"g","budget":1}`, cookie)
	testutil.Expect(t, "an unknown field", unknown.Code, http.StatusBadRequest)
}

// The chain is read at the address it is sent from, by GET and HEAD, and a
// third method learns the three.
func TestTheDesignLoopIsReadWhereItIsSent(t *testing.T) {
	t.Parallel()
	rec := &designing{}
	served, _ := designSignedIn(t, rec)
	response := get(t, served, designPrefix+designID+designReviewSuffix, nil)
	testutil.Require(t, "read", response.Code, http.StatusOK)
	var loop DesignLoop
	testutil.Require(t, "loop decodes", json.Unmarshal(response.Body.Bytes(), &loop), nil)
	testutil.Expect(t, "the round's finding", loop.Rounds[0].Findings[0].ID, "F1")
	testutil.Expect(t, "the design read", rec.designs[0], designID)
	request := httptest.NewRequest(http.MethodPut, "http://example.invalid"+designPrefix+designID+designReviewSuffix, strings.NewReader("{}"))
	request.Host = "127.0.0.1:7878"
	recorder := httptest.NewRecorder()
	served.ServeHTTP(recorder, request)
	testutil.Expect(t, "PUT", recorder.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the three methods", recorder.Header().Get("Allow"), "GET, HEAD, POST")
}

// A decisions row reaches the act with its round and its four cells, and a
// row the engine's rules refuse is refused in their words.
func TestADecisionsRowIsWrittenOrRefusedInWords(t *testing.T) {
	t.Parallel()
	rec := &designing{}
	served, cookie := designSignedIn(t, rec)
	path := designPrefix + designID + "/review/2/decisions"
	response := post(t, served, path, `{"finding":"F1","disposition":"refuted","reasoning":" searched reader.md ","amendment":""}`, cookie)
	testutil.Require(t, "written", response.Code, http.StatusOK)
	testutil.Expect(t, "the round", rec.rounds[0], int64(2))
	testutil.Expect(t, "the row", rec.rows[0], DesignRow{Finding: "F1", Disposition: "refuted", Reasoning: "searched reader.md"})
	testutil.Expect(t, "the design", rec.designs[0], designID)
	rec.refusal = &DesignRefusal{Code: "noted", Message: "material finding F1 cannot be noted; defer it as out-of-scope with the evidence that it is outside the brief"}
	refused := post(t, served, path, `{"finding":"F1","disposition":"noted","reasoning":"","amendment":""}`, cookie)
	testutil.Require(t, "refused", refused.Code, http.StatusConflict)
	testutil.Expect(t, "in words", actRefusal(t, refused).Error, rec.refusal.Error())
	for _, bad := range []string{designPrefix + designID + "/review/x/decisions", designPrefix + "/review"} {
		if _, ok := designRouteOf(bad); ok {
			t.Fatalf("%s is not a design-loop route", bad)
		}
	}
}
