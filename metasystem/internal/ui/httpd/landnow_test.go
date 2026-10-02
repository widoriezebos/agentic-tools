package httpd

// POST /api/fleet/land-now, at the boundary (goal fleet-card-can-land-now):
// who may reach it, that the verb seam is called once per press, and that the
// verb's own answer — success, a repeat that started nothing, or a refusal —
// comes back as the verb said it.
//
// What landing run does to the lane is the verb's; the seam here is a
// recorder answering the verb's documented --json envelope.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

type landingNow struct {
	calls    int
	answered LandNowAnswer
	failure  error
	sessions *session.Store
}

func (rec *landingNow) serving() Info {
	return Info{
		Observe:  readObservation,
		Sessions: rec.sessions,
		LandNow: func() (LandNowAnswer, error) {
			rec.calls++
			if rec.failure != nil {
				return LandNowAnswer{}, rec.failure
			}
			return rec.answered, nil
		},
	}
}

func startedLanding() LandNowAnswer {
	return LandNowAnswer{Outcome: "confirmed", Summary: "started the landing agent for batch b-20 (2 members)",
		Next: &LandNowNext{Argv: []string{"metasystem", "landing", "status"}, Reason: "follows it"}}
}

// signedInLandNow serves rec and answers the cookie of a signed-in human.
func signedInLandNow(t *testing.T, rec *landingNow) (http.Handler, map[string]string) {
	t.Helper()
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	return served, carrying(mintedCookie(t, signedIn).Value)
}

func landNowAnswerOf(t *testing.T, response *httptest.ResponseRecorder) LandNowAnswer {
	t.Helper()
	var answered LandNowAnswer
	testutil.Require(t, "decode the answer", json.Unmarshal(response.Body.Bytes(), &answered), nil)
	return answered
}

// Nobody signed in, nothing run: the route asks the session itself, and the
// browser is told the remedy is the sign-in sheet.
func TestLandNowRefusesEveryHandButASignedInOne(t *testing.T) {
	t.Parallel()
	rec := &landingNow{answered: startedLanding()}
	info := rec.serving()
	info.Authority = proven()
	served := New(info, loopback(), testBundle())

	response := post(t, served, landNowPath, `{}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	refusal := actRefusal(t, response)
	testutil.Expect(t, "the browser is told the remedy is here", refusal.SignIn, true)
	testutil.Expect(t, "the refusal says why in words", refusal.Error, landNowNeedsSignIn)
	testutil.Expect(t, "the verb was not run", rec.calls, 0)
}

// A live cookie whose proof stands for nothing runs nothing.
func TestLandNowWithALiveSessionButNoProofRunsNothing(t *testing.T) {
	t.Parallel()
	signing := newSigning(t, "Wido", workingSecret)
	rec := &landingNow{answered: startedLanding(), sessions: signing.store}
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	cookie := mintedCookie(t, signedIn)
	held, liveness := signing.store.Lookup(cookie.Value)
	testutil.Require(t, "the session is live", liveness, session.Live)
	held.Proof = humanauthority.Proof{}

	response := post(t, served, landNowPath, `{}`, carrying(cookie.Value))

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	testutil.Expect(t, "the browser is told the remedy is here", actRefusal(t, response).SignIn, true)
	testutil.Expect(t, "the verb was not run", rec.calls, 0)
}

// A signed-in press runs the verb once and answers its envelope as it was.
func TestASignedInLandNowRunsTheVerbOnceAndPassesItsAnswerThrough(t *testing.T) {
	t.Parallel()
	rec := &landingNow{answered: startedLanding()}
	served, cookie := signedInLandNow(t, rec)

	response := post(t, served, landNowPath, `{}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the verb ran once", rec.calls, 1)
	testutil.Expect(t, "the verb's answer, as it said it", landNowAnswerOf(t, response), startedLanding())
}

// A repeat while the agent runs is the verb's success that started nothing
// (R-129-ui): the route passes it through as the success it is.
func TestARepeatedLandNowIsTheVerbsUnchangedSuccess(t *testing.T) {
	t.Parallel()
	unchanged := LandNowAnswer{Outcome: "unchanged", Summary: "the landing agent already runs (pid 4242); nothing was started"}
	rec := &landingNow{answered: unchanged}
	served, cookie := signedInLandNow(t, rec)

	first := post(t, served, landNowPath, `{}`, cookie)
	second := post(t, served, landNowPath, `{}`, cookie)

	testutil.Require(t, "first status", first.Code, http.StatusOK)
	testutil.Require(t, "second status", second.Code, http.StatusOK)
	testutil.Expect(t, "each press asked the verb once", rec.calls, 2)
	testutil.Expect(t, "the repeat is the verb's unchanged", landNowAnswerOf(t, second), unchanged)
}

// The verb's refusal is its answer too: both lines reach the card, so the
// route answers it as a read of what the verb said rather than as an error
// that would keep only one line.
func TestALandNowRefusalComesBackWithBothOfTheVerbsLines(t *testing.T) {
	t.Parallel()
	refused := LandNowAnswer{Outcome: "refused", Summary: "this seat is at the helm, so the landing agent was not started",
		Next: &LandNowNext{Argv: []string{"metasystem", "helm", "return"}, Reason: "hands the helm back"}}
	rec := &landingNow{answered: refused}
	served, cookie := signedInLandNow(t, rec)

	response := post(t, served, landNowPath, `{}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the refusal, line 1 and line 2", landNowAnswerOf(t, response), refused)
}

// A seam that could not run the verb at all is a failure, said in words.
func TestALandNowSeamFailureIsA500InWords(t *testing.T) {
	t.Parallel()
	rec := &landingNow{failure: errors.New("the verb answered nothing readable")}
	served, cookie := signedInLandNow(t, rec)

	response := post(t, served, landNowPath, `{}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
	testutil.Expect(t, "the reason", refusalBody(t, "failure", response).Error, "the verb answered nothing readable")
}

// The body is the empty object and nothing more: a field it does not take is
// refused before the verb is reached.
func TestALandNowBodyWithAFieldIsRefused(t *testing.T) {
	t.Parallel()
	rec := &landingNow{answered: startedLanding()}
	served, cookie := signedInLandNow(t, rec)

	response := post(t, served, landNowPath, `{"force":true}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusBadRequest)
	testutil.Expect(t, "the verb was not run", rec.calls, 0)
}

// A build with no seam says so rather than pretending it started anything.
func TestAnEngineThatCannotLandNowSaysSo(t *testing.T) {
	t.Parallel()
	signing := newSigning(t, "Wido", workingSecret)
	served := New(Info{Observe: readObservation, Sessions: signing.store}, loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

	response := post(t, served, landNowPath, `{}`, carrying(mintedCookie(t, signedIn).Value))

	testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
}

// POST from this origin and nothing else, like every other write.
func TestTheLandNowRouteTakesPostFromThisOriginAndNothingElse(t *testing.T) {
	t.Parallel()
	rec := &landingNow{answered: startedLanding()}
	served, cookie := signedInLandNow(t, rec)

	request := httptest.NewRequest(http.MethodGet, "http://example.invalid"+landNowPath, nil)
	request.Host = "127.0.0.1:7878"
	response := httptest.NewRecorder()
	served.ServeHTTP(response, request)
	testutil.Require(t, "status", response.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the verb it takes", response.Header().Get("Allow"), "POST")

	headers := map[string]string{"Origin": "http://attacker.invalid"}
	for name, value := range cookie {
		headers[name] = value
	}
	crossSite := post(t, served, landNowPath, `{}`, headers)
	testutil.Expect(t, "a foreign origin is refused", crossSite.Code, http.StatusForbidden)
	testutil.Expect(t, "the verb was not run", rec.calls, 0)
}
