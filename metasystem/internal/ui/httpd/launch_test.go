package httpd

// POST /api/fleet/launch, at the boundary: who may reach it, what the body
// becomes, how a refusal comes back, and what a started launch answers with.
//
// What a launch does to this host is proved in internal/seat/launch; the
// starter here is a recorder, so every case asserts the one thing only this
// layer can — whether the act was reached at all, and under what.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

type launching struct {
	asked    []launch.Request
	refusal  error
	answered launch.Record
	sessions *session.Store
}

func (rec *launching) serving() Info {
	return Info{
		Observe:  readObservation,
		Sessions: rec.sessions,
		Launch: func(_ *session.Session, asked launch.Request) (launch.Record, error) {
			rec.asked = append(rec.asked, asked)
			if rec.refusal != nil {
				return launch.Record{}, rec.refusal
			}
			return rec.answered, nil
		},
	}
}

// wholeLaunch is everything the sheet sends: signed in is enough (g1-s72), so
// no word and no date travel with it.
const wholeLaunch = `{"machine":"m1f","destination":"/w/agentic-tools-m1f"}`

func startedLaunch() launch.Record {
	return launch.Record{
		SchemaVersion: launch.SchemaVersion, Launch: "01M3BQAVYXE2AT6F0JG9YB64PG",
		Machine: "m1f", Destination: "/w/agentic-tools-m1f",
		Outcome: launch.OutcomeRunning,
	}
}

// Nobody signed in, nothing launched: the route checks the session itself
// rather than leaning on the act layer's gate, which still admits the boot
// proof on a seat with no Partner.
func TestLaunchingRefusesEveryHandButASignedInOne(t *testing.T) {
	t.Parallel()

	rec := &launching{answered: startedLaunch()}
	info := rec.serving()
	info.Authority = proven()
	served := New(info, loopback(), testBundle())

	response := post(t, served, launchPath, wholeLaunch, nil)

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	refusal := actRefusal(t, response)
	testutil.Expect(t, "the browser is told the remedy is here", refusal.SignIn, true)
	testutil.Expect(t, "nothing was launched", len(rec.asked), 0)
}

func TestASignedInHumanLaunchesAMachine(t *testing.T) {
	t.Parallel()

	signing := newSigning(t, "Wido", workingSecret)
	rec := &launching{answered: startedLaunch(), sessions: signing.store}
	served := New(rec.serving(), loopback(), testBundle())

	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	testutil.Require(t, "signed in", signedIn.Code, http.StatusOK)

	response := post(t, served, launchPath, wholeLaunch, carrying(mintedCookie(t, signedIn).Value))

	testutil.Require(t, "status", response.Code, http.StatusAccepted)
	testutil.Expect(t, "what the act was asked for", rec.asked, []launch.Request{{
		Machine: "m1f", Destination: "/w/agentic-tools-m1f",
	}})
	var answered launch.Record
	testutil.Require(t, "decode the record", json.Unmarshal(response.Body.Bytes(), &answered), nil)
	testutil.Expect(t, "the answer is the record", answered.Launch, "01M3BQAVYXE2AT6F0JG9YB64PG")
	testutil.Expect(t, "it is running", answered.Outcome, launch.OutcomeRunning)
}

// A retry names the launch and nothing else: the record carries the
// signed-in session's verdict, and the starter stamps it again.
func TestARetryResumesByLaunchIdAlone(t *testing.T) {
	t.Parallel()

	signing := newSigning(t, "Wido", workingSecret)
	rec := &launching{answered: startedLaunch(), sessions: signing.store}
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

	response := post(t, served, launchPath, `{"resume":"01M3BQAVYXE2AT6F0JG9YB64PG"}`,
		carrying(mintedCookie(t, signedIn).Value))

	testutil.Require(t, "status", response.Code, http.StatusAccepted)
	testutil.Expect(t, "what the act was asked for", rec.asked, []launch.Request{{
		Resume: "01M3BQAVYXE2AT6F0JG9YB64PG",
	}})
}

// The body is {machine, destination} or {resume}: the word, the date and the
// browser's day are no longer part of it, so a body that still carries them
// is refused as the unknown fields they are and launches nothing.
func TestALaunchBodyCarryingTheOldAuthorizationIsRefused(t *testing.T) {
	t.Parallel()

	for what, body := range map[string]string{
		"a word": `{"machine":"m1f","destination":"/w/agentic-tools-m1f","word":"Wido says so"}`,
		"a date": `{"machine":"m1f","destination":"/w/agentic-tools-m1f","reviewBy":"2026-10-02"}`,
		"a day":  `{"resume":"01M3BQAVYXE2AT6F0JG9YB64PG","today":"2026-09-29"}`,
	} {
		signing := newSigning(t, "Wido", workingSecret)
		rec := &launching{answered: startedLaunch(), sessions: signing.store}
		served := New(rec.serving(), loopback(), testBundle())
		signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

		response := post(t, served, launchPath, body, carrying(mintedCookie(t, signedIn).Value))

		if response.Code == http.StatusAccepted {
			t.Fatalf("%s: a body with %s was accepted", what, what)
		}
		testutil.Expect(t, "nothing was launched for "+what, len(rec.asked), 0)
	}
}

// A refusal comes back in the act's own words under its own code, with the
// status that says what a human can do about it: a conflict is something in
// the way, and everything else is the request.
func TestALaunchRefusalCarriesItsCodeAndTheStatusThatFitsIt(t *testing.T) {
	t.Parallel()

	for code, status := range map[string]int{
		launch.CodeRunning:                    http.StatusConflict,
		launch.CodeNicknameTaken:              http.StatusConflict,
		launch.CodeDestinationExists:          http.StatusConflict,
		launch.CodeDiskShort:                  http.StatusConflict,
		launch.CodeNicknameInvalid:            http.StatusUnprocessableEntity,
		launch.CodeDestinationInsideACheckout: http.StatusUnprocessableEntity,
		launch.CodeWordInvalid:                http.StatusUnprocessableEntity,
		launch.CodeWordRequired:               http.StatusUnprocessableEntity,
		launch.CodeRecordConflict:             http.StatusUnprocessableEntity,
	} {
		signing := newSigning(t, "Wido", workingSecret)
		rec := &launching{sessions: signing.store, refusal: &launch.Refusal{Code: code, Message: "the owner's own sentence"}}
		served := New(rec.serving(), loopback(), testBundle())
		signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

		response := post(t, served, launchPath, wholeLaunch, carrying(mintedCookie(t, signedIn).Value))

		testutil.Require(t, "status for "+code, response.Code, status)
		refusal := actRefusal(t, response)
		testutil.Expect(t, "code for "+code, refusal.Code, code)
		testutil.Expect(t, "the owner's words for "+code, refusal.Error, "the owner's own sentence")
	}
}

// A live cookie is not a human. The route asks the proof itself, so a session
// record whose proof stands for nothing — or for another checkout — launches
// nothing.
func TestALiveSessionWithNoProofLaunchesNothing(t *testing.T) {
	t.Parallel()

	signing := newSigning(t, "Wido", workingSecret)
	rec := &launching{answered: startedLaunch(), sessions: signing.store}
	served := New(rec.serving(), loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)
	cookie := mintedCookie(t, signedIn)
	// The session stays live and its proof is emptied, which is exactly what
	// a record restored from anything but a fresh in-process mint looks like.
	held, liveness := signing.store.Lookup(cookie.Value)
	testutil.Require(t, "the session is live", liveness, session.Live)
	held.Proof = humanauthority.Proof{}

	response := post(t, served, launchPath, wholeLaunch, carrying(cookie.Value))

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	testutil.Expect(t, "the browser is told the remedy is here", actRefusal(t, response).SignIn, true)
	testutil.Expect(t, "nothing was launched", len(rec.asked), 0)
}

// A build with no launcher says so rather than pretending it started one.
func TestAnEngineThatCannotLaunchSaysSo(t *testing.T) {
	t.Parallel()

	signing := newSigning(t, "Wido", workingSecret)
	info := Info{Observe: readObservation, Sessions: signing.store}
	served := New(info, loopback(), testBundle())
	signedIn := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":""}`, nil)

	response := post(t, served, launchPath, wholeLaunch, carrying(mintedCookie(t, signedIn).Value))

	testutil.Require(t, "status", response.Code, http.StatusInternalServerError)
}

// The route takes POST and nothing else, like every other write.
func TestTheLaunchRouteTakesPostAndNothingElse(t *testing.T) {
	t.Parallel()

	rec := &launching{answered: startedLaunch()}
	served := New(rec.serving(), loopback(), testBundle())

	request := httptest.NewRequest(http.MethodGet, "http://example.invalid"+launchPath, nil)
	request.Host = "127.0.0.1:7878"
	response := httptest.NewRecorder()
	served.ServeHTTP(response, request)

	testutil.Require(t, "status", response.Code, http.StatusMethodNotAllowed)
	testutil.Expect(t, "the verb it takes", response.Header().Get("Allow"), "POST")
}
