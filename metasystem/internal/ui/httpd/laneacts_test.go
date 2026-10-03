package httpd

// The fleet panel's Pause, Resume and Stop as the signed-in person
// (fleet-panel-ux-step2.md slice 2b, ruling R-142-ui), at the boundary: who
// may reach each route; that the route asks act.SignedIn for the person and
// hands the seam that person and nothing else; that Stop refuses a name that
// is not one word, a name that is no machine of this computer and the
// checkout serving the page, each before any stop runs (S2-01); and that the
// verb's answer, a repeat's unchanged success included (S2-02, R-129-ui),
// comes back as the verb said it.
//
// What the verbs do is cmd/metasystem's to prove; the seams here are
// recorders answering the verbs' documented --json envelope.

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

type fleetActing struct {
	// paused and resumed are the persons the lane's seams were handed;
	// admitted the names Stop's admission was asked; stopped each stop's
	// person and machine.
	paused, resumed, admitted []string
	stopped                   [][2]string
	answered                  LandNowAnswer
	// refusal is what admission answers: nil admits the stop.
	refusal      *LandNowAnswer
	failure      error
	admitFailure error
	sessions     *session.Store
}

func (rec *fleetActing) answer() (LandNowAnswer, error) {
	if rec.failure != nil {
		return LandNowAnswer{}, rec.failure
	}
	return rec.answered, nil
}

func (rec *fleetActing) serving() Info {
	return Info{
		Observe:  readObservation,
		Sessions: rec.sessions,
		PauseLane: func(human string) (LandNowAnswer, error) {
			rec.paused = append(rec.paused, human)
			return rec.answer()
		},
		ResumeLane: func(human string) (LandNowAnswer, error) {
			rec.resumed = append(rec.resumed, human)
			return rec.answer()
		},
		AdmitStop: func(machine string) (*LandNowAnswer, error) {
			rec.admitted = append(rec.admitted, machine)
			if rec.admitFailure != nil {
				return nil, rec.admitFailure
			}
			return rec.refusal, nil
		},
		StopMachine: func(human, machine string) (LandNowAnswer, error) {
			rec.stopped = append(rec.stopped, [2]string{human, machine})
			return rec.answer()
		},
	}
}

// ran counts every seam call: a refused hand calls none of them.
func (rec *fleetActing) ran() int {
	return len(rec.paused) + len(rec.resumed) + len(rec.admitted) + len(rec.stopped)
}

// fleetActRoutes are the three routes with the body each takes.
var fleetActRoutes = []struct{ path, body string }{
	{lanePausePath, `{}`},
	{laneResumePath, `{}`},
	{machineStopPath, `{"machine":"m1f"}`},
}

// signedInFleetActs serves rec and answers the cookie of a signed-in human,
// with the store, so a test can reach the session it minted.
func signedInFleetActs(t *testing.T, rec *fleetActing) (http.Handler, map[string]string, *signing) {
	t.Helper()
	signing := newSigning(t, "Wido", workingSecret)
	rec.sessions = signing.store
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)
	return served, carrying(mintedCookie(t, signedIn).Value), signing
}

func paused() LandNowAnswer {
	return LandNowAnswer{Outcome: "confirmed", Summary: "stopped the landing lane for Wido; no landing agent starts until metasystem landing start",
		Next: &LandNowNext{Argv: []string{"metasystem", "landing", "unset"}, Reason: "lets each seat land its own work instead"}}
}

// Nobody signed in, nothing run: no session at all, a live session whose
// proof stands for nothing, and a session whose name is not the one its proof
// was minted for (act.SignedIn's own check, D1) are each a 403 the browser
// answers with the sign-in sheet, and no seam is called.
func TestFleetActsRefuseEveryHandButASignedInOne(t *testing.T) {
	t.Parallel()
	spoiled := map[string]func(*session.Session){
		"a session whose proof stands for nothing":     func(held *session.Session) { held.Proof = humanauthority.Proof{} },
		"a session named for another than its proof's": func(held *session.Session) { held.Human = "Mallory" },
	}
	for _, route := range fleetActRoutes {
		t.Run(route.path+" with no session", func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{answered: paused()}
			info := rec.serving()
			info.Authority = proven()
			served := New(info, loopback(), testBundle())

			response := post(t, served, route.path, route.body, nil)

			testutil.Require(t, "status", response.Code, http.StatusForbidden)
			refusal := actRefusal(t, response)
			testutil.Expect(t, "the browser is told the remedy is here", refusal.SignIn, true)
			testutil.Expect(t, "the refusal says why in words", refusal.Error != "", true)
			testutil.Expect(t, "nothing ran", rec.ran(), 0)
		})
		for name, spoil := range spoiled {
			t.Run(route.path+" with "+name, func(t *testing.T) {
				t.Parallel()
				rec := &fleetActing{answered: paused()}
				served, cookie, signing := signedInFleetActs(t, rec)
				held, liveness := signing.store.Lookup(cookie["Cookie"][len(session.Cookie)+1:])
				testutil.Require(t, "the session is live", liveness, session.Live)
				spoil(held)

				response := post(t, served, route.path, route.body, cookie)

				testutil.Require(t, "status", response.Code, http.StatusForbidden)
				testutil.Expect(t, "the browser opens the sign-in sheet", actRefusal(t, response).SignIn, true)
				testutil.Expect(t, "nothing ran", rec.ran(), 0)
			})
		}
	}
}

// A signed-in Pause runs landing stop once with the session's human as who
// paused the lane, and answers the verb's envelope as it was.
func TestASignedInPauseRunsLandingStopForTheSessionsHuman(t *testing.T) {
	t.Parallel()
	rec := &fleetActing{answered: paused()}
	served, cookie, _ := signedInFleetActs(t, rec)

	response := post(t, served, lanePausePath, `{}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the pause was run once, for the session's human", rec.paused, []string{"Wido"})
	testutil.Expect(t, "nothing else ran", len(rec.resumed)+len(rec.admitted)+len(rec.stopped), 0)
	testutil.Expect(t, "the verb's answer, as it said it", landNowAnswerOf(t, response), paused())
}

// A signed-in Resume runs landing start once with the session's human as the
// person it asks, and a resume of a lane that was not paused is the verb's
// unchanged success, passed through.
func TestASignedInResumeRunsLandingStartAsTheSessionsHuman(t *testing.T) {
	t.Parallel()
	running := LandNowAnswer{Outcome: "unchanged", Summary: "the landing lane at /w/landing is already running"}
	rec := &fleetActing{answered: running}
	served, cookie, _ := signedInFleetActs(t, rec)

	first := post(t, served, laneResumePath, `{}`, cookie)
	second := post(t, served, laneResumePath, `{}`, cookie)

	testutil.Require(t, "first status", first.Code, http.StatusOK)
	testutil.Require(t, "second status", second.Code, http.StatusOK)
	testutil.Expect(t, "each press resumed as the session's human", rec.resumed, []string{"Wido", "Wido"})
	testutil.Expect(t, "the repeat is the verb's unchanged", landNowAnswerOf(t, second), running)
}

// Stop refuses, before the stop seam is ever called: a name that is not one
// word of letters, digits, '.', '-' and '_' (before admission too), and every
// name admission refuses — no machine of this computer, one on another
// computer, the checkout serving this page — with admission's own two lines.
func TestStopRefusesBeforeAnyStopRuns(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{"machine":""}`, `{"machine":"../m1f"}`, `{"machine":"m1f/x"}`, `{"machine":".."}`, `{}`} {
		t.Run(body, func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{answered: paused()}
			served, cookie, _ := signedInFleetActs(t, rec)

			response := post(t, served, machineStopPath, body, cookie)

			testutil.Expect(t, "it is refused as the request's fault", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "it says why in words", refusalBody(t, "the refusal", response).Error != "", true)
			testutil.Expect(t, "it reached neither admission nor the stop", rec.ran(), 0)
		})
	}
	for name, refusal := range map[string]LandNowAnswer{
		"no machine of this computer": {Outcome: "refused", Summary: "no machine m9z on this computer or in the fleet; nothing was done",
			Next: &LandNowNext{Argv: []string{"metasystem", "machine", "list", "--verbose"}, Reason: "names every machine of this computer"}},
		"the checkout serving this page": {Outcome: "refused", Summary: "m1e serves this page, so it is stopped at a terminal and not from here; nothing was stopped",
			Next: &LandNowNext{Argv: []string{"metasystem", "machine", "stop", "m1e"}, Reason: "at a terminal on this computer"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{answered: paused(), refusal: &refusal}
			served, cookie, _ := signedInFleetActs(t, rec)

			response := post(t, served, machineStopPath, `{"machine":"m1e"}`, cookie)

			testutil.Require(t, "status", response.Code, http.StatusOK)
			testutil.Expect(t, "admission was asked about the name", rec.admitted, []string{"m1e"})
			testutil.Expect(t, "the stop seam was never called", len(rec.stopped), 0)
			testutil.Expect(t, "admission's two lines reach the page", landNowAnswerOf(t, response), refusal)
		})
	}
}

// An admitted Stop runs machine stop once, for the session's human and the
// machine named, and answers the verb's envelope.
func TestASignedInStopRunsMachineStopOnceForTheSessionsHuman(t *testing.T) {
	t.Parallel()
	stopped := LandNowAnswer{Outcome: "confirmed", Summary: "stopped MetaSystem on m1f (/w/m1f)"}
	rec := &fleetActing{answered: stopped}
	served, cookie, _ := signedInFleetActs(t, rec)

	response := post(t, served, machineStopPath, `{"machine":"m1f"}`, cookie)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "admission came first", rec.admitted, []string{"m1f"})
	testutil.Expect(t, "the stop ran once, as the session's human", rec.stopped, [][2]string{{"Wido", "m1f"}})
	testutil.Expect(t, "the verb's answer, as it said it", landNowAnswerOf(t, response), stopped)
}

// A second tab's Stop of a machine already stopped is admitted and reaches
// the verb, whose unchanged success comes back as the success it is (S2-02,
// R-129-ui).
func TestARepeatedStopIsTheVerbsUnchangedSuccess(t *testing.T) {
	t.Parallel()
	already := LandNowAnswer{Outcome: "unchanged", Summary: "MetaSystem is already stopped on m1f (/w/m1f); nothing of MetaSystem's is running"}
	rec := &fleetActing{answered: already}
	served, cookie, _ := signedInFleetActs(t, rec)

	first := post(t, served, machineStopPath, `{"machine":"m1f"}`, cookie)
	second := post(t, served, machineStopPath, `{"machine":"m1f"}`, cookie)

	testutil.Require(t, "first status", first.Code, http.StatusOK)
	testutil.Require(t, "second status", second.Code, http.StatusOK)
	testutil.Expect(t, "each press reached the verb", len(rec.stopped), 2)
	testutil.Expect(t, "the repeat is the verb's unchanged", landNowAnswerOf(t, second), already)
}

// A seam that could not run its verb at all is a 500 in words, and an
// admission that could not read this computer's machines stops nothing.
func TestAFleetActSeamFailureIsA500InWords(t *testing.T) {
	t.Parallel()
	for _, route := range fleetActRoutes {
		t.Run(route.path, func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{failure: errors.New("the verb answered nothing the interface can read")}
			served, cookie, _ := signedInFleetActs(t, rec)

			response := post(t, served, route.path, route.body, cookie)

			testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
			testutil.Expect(t, "it says why", refusalBody(t, "failure", response).Error, "the verb answered nothing the interface can read")
		})
	}
	t.Run("admission", func(t *testing.T) {
		t.Parallel()
		rec := &fleetActing{answered: paused(), admitFailure: errors.New("the host registry cannot be located")}
		served, cookie, _ := signedInFleetActs(t, rec)

		response := post(t, served, machineStopPath, `{"machine":"m1f"}`, cookie)

		testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
		testutil.Expect(t, "it says why", refusalBody(t, "failure", response).Error, "the host registry cannot be located")
		testutil.Expect(t, "the stop seam was never called", len(rec.stopped), 0)
	})
}

// The lane's bodies are the empty object and Stop's names the machine and
// nothing more: a field a route does not take is refused before any seam.
func TestAFleetActBodyWithAFieldItDoesNotTakeIsRefused(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ path, body string }{
		{lanePausePath, `{"by":"Mallory"}`},
		{laneResumePath, `{"force":true}`},
		{machineStopPath, `{"machine":"m1f","all":true}`},
	} {
		t.Run(route.path, func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{answered: paused()}
			served, cookie, _ := signedInFleetActs(t, rec)

			response := post(t, served, route.path, route.body, cookie)

			testutil.Require(t, "status", response.Code, http.StatusBadRequest)
			testutil.Expect(t, "nothing ran", rec.ran(), 0)
		})
	}
}

// A build with none of the seams says so rather than pretending it acted.
func TestAnEngineWithoutTheFleetActsSaysSo(t *testing.T) {
	t.Parallel()
	for _, route := range fleetActRoutes {
		t.Run(route.path, func(t *testing.T) {
			t.Parallel()
			signing := newSigning(t, "Wido", workingSecret)
			served := New(Info{Observe: readObservation, Sessions: signing.store}, loopback(), testBundle())
			signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

			response := post(t, served, route.path, route.body, carrying(mintedCookie(t, signedIn).Value))

			testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
			testutil.Expect(t, "it says why in words", refusalBody(t, "failure", response).Error != "", true)
		})
	}
}

// POST from this origin and nothing else, like every other write.
func TestTheFleetActRoutesTakePostFromThisOriginAndNothingElse(t *testing.T) {
	t.Parallel()
	for _, route := range fleetActRoutes {
		t.Run(route.path, func(t *testing.T) {
			t.Parallel()
			rec := &fleetActing{answered: paused()}
			served, cookie, _ := signedInFleetActs(t, rec)

			request := httptest.NewRequest(http.MethodGet, "http://example.invalid"+route.path, nil)
			request.Host = "127.0.0.1:7878"
			response := httptest.NewRecorder()
			served.ServeHTTP(response, request)
			testutil.Require(t, "GET status", response.Code, http.StatusMethodNotAllowed)
			testutil.Expect(t, "the verb it takes", response.Header().Get("Allow"), "POST")

			headers := map[string]string{"Origin": "http://attacker.invalid"}
			for name, value := range cookie {
				headers[name] = value
			}
			crossSite := post(t, served, route.path, route.body, headers)
			testutil.Expect(t, "a foreign origin is refused", crossSite.Code, http.StatusForbidden)
			testutil.Expect(t, "nothing ran", rec.ran(), 0)
		})
	}
}
