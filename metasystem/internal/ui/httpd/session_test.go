package httpd

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// Signing in, at the boundary: what a code buys, what it does not, what the
// cookie is allowed to be, and which hand an act afterwards publishes under.
// What a session does to the ledger is proved in internal/ui/act; the store's
// own rules are proved in internal/ui/session.

// A synthetic secret, never a configured one.
const routeSecret = "JBSWY3DPEHPK3PXP"

var routeNow = time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)

type signing struct {
	store *session.Store
	at    time.Time
	// The floor, in memory, with a way to break it the way a full disk does.
	floor      int64
	remembered string
	writeErr   error
	readErr    error
}

func newSigning(t *testing.T, human string, secret func() (string, error)) *signing {
	t.Helper()
	held := &signing{at: routeNow, remembered: human}
	held.store = session.New(session.Options{
		Root:     t.TempDir(),
		Human:    human,
		Lifetime: 12 * time.Hour,
		Secret:   secret,
		Now:      func() time.Time { return held.at },
		Floor: func() (int64, string, error) {
			if held.readErr != nil {
				return 0, "", held.readErr
			}
			return held.floor, held.remembered, nil
		},
		Record: func(lastStep int64, named string) error {
			if held.writeErr != nil {
				return held.writeErr
			}
			held.floor, held.remembered = lastStep, named
			return nil
		},
	})
	return held
}

func workingSecret() (string, error) { return routeSecret, nil }

func routeCode(t *testing.T, at time.Time) string {
	t.Helper()
	code, err := channel.TOTPCode(routeSecret, at)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

type answered struct {
	Human    string `json:"human"`
	SignedIn bool   `json:"signedIn"`
	Until    string `json:"until"`
	Source   string `json:"source"`
}

func sessionAnswer(t *testing.T, response *httptest.ResponseRecorder) answered {
	t.Helper()
	var body answered
	testutil.Require(t, "decode the session answer", json.Unmarshal(response.Body.Bytes(), &body), nil)
	return body
}

// get is post's read counterpart, with the headers a cookie travels in.
func get(t *testing.T, handler http.Handler, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "http://example.invalid"+path, nil)
	req.Host = "127.0.0.1:7878"
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	return response
}

func carrying(id string) map[string]string {
	return map[string]string{"Cookie": session.Cookie + "=" + id}
}

// mintedCookie is the one Set-Cookie a sign-in wrote, refusing a response that
// wrote none.
func mintedCookie(t *testing.T, response *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	cookies := response.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("the response set %d cookies, want exactly one", len(cookies))
	}
	return cookies[0]
}

// The code the channel's own generator produces signs the human in, and what
// the browser is given back is an opaque identifier no script can read and no
// other site can send.
func TestSignInWithTheSeatsCodeMintsALockedDownCookie(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	cookie := mintedCookie(t, response)
	testutil.Expect(t, "the cookie's name", cookie.Name, session.Cookie)
	testutil.Expect(t, "no script reads it", cookie.HttpOnly, true)
	testutil.Expect(t, "no other site sends it", cookie.SameSite, http.SameSiteStrictMode)
	testutil.Expect(t, "it is the whole application's", cookie.Path, "/")
	if strings.Contains(cookie.Value, "Wido") || strings.Contains(cookie.Value, routeSecret) {
		t.Fatalf("the cookie carries more than an identifier: %q", cookie.Value)
	}

	answer := sessionAnswer(t, response)
	testutil.Expect(t, "who is signed in", answer.Human, "Wido")
	testutil.Expect(t, "signed in", answer.SignedIn, true)
	testutil.Expect(t, "how", answer.Source, "code")
	testutil.Expect(t, "until", answer.Until, routeNow.Add(12*time.Hour).Format(time.RFC3339))

	// The cookie holds the bearer and the bearer is in nothing else: not in
	// the answer, not in what `ui status` prints, and not in the proof the
	// ledger records.
	if strings.Contains(response.Body.String(), cookie.Value) {
		t.Fatalf("the answer carries the bearer: %s", response.Body.String())
	}
	lines := held.store.Lines()
	if len(lines) != 1 || strings.Contains(lines[0], cookie.Value) {
		t.Fatalf("what ui status prints carries the bearer: %v", lines)
	}
	signed, liveness := held.store.Lookup(cookie.Value)
	testutil.Require(t, "the cookie names a live session", liveness, session.Live)
	testutil.Expect(t, "the proof names the reference", signed.Proof.ChannelRef, signed.Reference)
	if signed.Reference == cookie.Value {
		t.Fatal("the reference the ledger records is the cookie's own value")
	}
	if !strings.Contains(lines[0], signed.Reference) {
		t.Fatalf("the status line does not name the reference: %q", lines[0])
	}
}

// A seat that cannot record a code as spent refuses to accept one, and says
// so as a seat problem rather than as a wrong code.
func TestASeatThatCannotRecordTheFloorRefusesToSignAnybodyIn(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	held.writeErr = errors.New("no space left on device")
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusServiceUnavailable)
	testutil.Expect(t, "the reason", actRefusal(t, response).Error,
		"this seat cannot record the code as spent: no space left on device")
	testutil.Expect(t, "no cookie was set", len(response.Result().Cookies()), 0)
}

// A code that is not this seat's, and a code this seat already accepted, are
// two different things to be told.
func TestARefusedCodeSaysWhichKindOfRefusalItIs(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())
	code := routeCode(t, routeNow)

	wrong := post(t, served, signInPath, `{"code":"000000"}`, nil)
	testutil.Require(t, "status for a wrong code", wrong.Code, http.StatusUnauthorized)
	testutil.Expect(t, "the reason for a wrong code", actRefusal(t, wrong).Error, "the code is not valid now")
	testutil.Expect(t, "no cookie was set for a wrong code", len(wrong.Result().Cookies()), 0)

	if first := post(t, served, signInPath, `{"code":"`+code+`"}`, nil); first.Code != http.StatusOK {
		t.Fatalf("the first use of the code answered %d", first.Code)
	}
	again := post(t, served, signInPath, `{"code":"`+code+`"}`, nil)
	testutil.Require(t, "status for a replayed code", again.Code, http.StatusUnauthorized)
	testutil.Expect(t, "the reason for a replayed code", actRefusal(t, again).Error, "that code was already used")
	testutil.Expect(t, "no cookie was set for a replayed code", len(again.Result().Cookies()), 0)
}

// A seat with no one-time-code secret cannot verify anything. It says which
// key is missing and where it is set, and it is a 503 rather than a 401,
// because nothing the human typed was wrong.
func TestASeatWithoutASecretCannotSignAnybodyIn(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", func() (string, error) { return "", nil })
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, signInPath, `{"code":"000000"}`, nil)

	testutil.Require(t, "status", response.Code, http.StatusServiceUnavailable)
	refusal := actRefusal(t, response)
	if !strings.Contains(refusal.Error, session.SecretKey) || !strings.Contains(refusal.Error, "metasystem.conf.local") {
		t.Fatalf("the refusal does not name the key and where the channel's setup writes it: %q", refusal.Error)
	}
}

// Guessing is answered with a wait rather than with another guess.
func TestGuessingClosesSignInWithAWait(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())

	for attempt := 0; attempt < session.Failures; attempt++ {
		if refused := post(t, served, signInPath, `{"code":"000000"}`, nil); refused.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d answered %d", attempt+1, refused.Code)
		}
	}

	response := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil)
	testutil.Require(t, "status", response.Code, http.StatusTooManyRequests)
}

// Who the server is acting as, in one shape, whether anybody signed in or not.
func TestTheSessionRouteSaysWhoTheServerActsAs(t *testing.T) {
	t.Parallel()

	t.Run("nobody", func(t *testing.T) {
		t.Parallel()
		held := newSigning(t, "", workingSecret)
		rec := &acted{authorized: agentStarted(), sessions: held.store}
		served := New(rec.acting(), loopback(), testBundle())

		answer := sessionAnswer(t, get(t, served, sessionPath, nil))

		testutil.Expect(t, "signed in", answer.SignedIn, false)
		testutil.Expect(t, "how", answer.Source, "none")
		testutil.Expect(t, "and the sheet must ask the name", answer.Human, "")
	})

	t.Run("the terminal that started it", func(t *testing.T) {
		t.Parallel()
		held := newSigning(t, "Wido", workingSecret)
		rec := &acted{authorized: proven(), sessions: held.store}
		served := New(rec.acting(), loopback(), testBundle())

		answer := sessionAnswer(t, get(t, served, sessionPath, nil))

		testutil.Expect(t, "signed in", answer.SignedIn, true)
		testutil.Expect(t, "how", answer.Source, "terminal")
		testutil.Expect(t, "who", answer.Human, "Wido")
		testutil.Expect(t, "the terminal's authority does not expire", answer.Until, "")
	})

	t.Run("a browser that signed in", func(t *testing.T) {
		t.Parallel()
		held := newSigning(t, "Wido", workingSecret)
		rec := &acted{authorized: agentStarted(), sessions: held.store}
		served := New(rec.acting(), loopback(), testBundle())
		cookie := mintedCookie(t, post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil))

		answer := sessionAnswer(t, get(t, served, sessionPath, carrying(cookie.Value)))

		testutil.Expect(t, "signed in", answer.SignedIn, true)
		testutil.Expect(t, "how", answer.Source, "code")
		testutil.Expect(t, "who", answer.Human, "Wido")
	})
}

// The whole point of a session: an act a signed-in browser makes on a server
// nothing else proves reaches the engine, and reaches it as that session.
func TestAnActUnderASessionReachesTheEngineAsThatSession(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())
	cookie := mintedCookie(t, post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil))

	response := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, carrying(cookie.Value))

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the act reached the engine", len(rec.approvals), 1)
	testutil.Expect(t, "the hand it was published under", rec.hands, []string{"Wido"})
}

// A server the terminal proved keeps acting as that terminal when no browser
// has signed in, and the act carries no session.
func TestAnActWithoutASessionStillUsesTheBootProof(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: proven(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())

	response := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, nil)

	testutil.Require(t, "status", response.Code, http.StatusOK)
	testutil.Expect(t, "the hand it was published under", rec.hands, []string{""})
}

// A page left open past its session's hours is told the one thing that fixes
// it, with the flag that opens the sheet on it.
func TestAnExpiredSessionAsksForAFreshOne(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())
	cookie := mintedCookie(t, post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil))
	held.at = routeNow.Add(12 * time.Hour)

	response := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, carrying(cookie.Value))

	testutil.Require(t, "status", response.Code, http.StatusForbidden)
	refusal := actRefusal(t, response)
	testutil.Expect(t, "the reason", refusal.Error, expiredRefusal)
	testutil.Expect(t, "the remedy is in the page", refusal.SignIn, true)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// Signing out ends it here and in the browser, and the answer says what is
// true afterwards rather than what the request still carried.
func TestSigningOutEndsTheSessionAndClearsTheCookie(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())
	cookie := mintedCookie(t, post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil))

	response := post(t, served, signOutPath, `{}`, carrying(cookie.Value))

	testutil.Require(t, "status", response.Code, http.StatusOK)
	cleared := mintedCookie(t, response)
	testutil.Expect(t, "the cookie is cleared", cleared.Value, "")
	testutil.Expect(t, "and expired", cleared.MaxAge < 0, true)
	answer := sessionAnswer(t, response)
	testutil.Expect(t, "signed in", answer.SignedIn, false)
	testutil.Expect(t, "how", answer.Source, "none")
	testutil.Expect(t, "the seat still knows who it is", answer.Human, "Wido")

	after := post(t, served, "/api/backlog/goals/waiting/approve", wholeBudget, carrying(cookie.Value))
	testutil.Require(t, "the spent cookie acts", after.Code, http.StatusForbidden)
	testutil.Expect(t, "the engine was not reached", rec.reached(), 0)
}

// Every session route takes the policy every other route takes: the method,
// the host, and the site. A page on another origin cannot sign anybody in and
// cannot spend a session either.
func TestTheSessionRoutesTakeTheSamePolicyAsEveryOtherRoute(t *testing.T) {
	t.Parallel()
	held := newSigning(t, "Wido", workingSecret)
	rec := &acted{authorized: agentStarted(), sessions: held.store}
	served := New(rec.acting(), loopback(), testBundle())
	cookie := mintedCookie(t, post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil))

	crossSite := map[string]string{"Sec-Fetch-Site": "cross-site", "Sec-Fetch-Mode": "cors", "Sec-Fetch-Dest": "empty"}
	for path, body := range map[string]string{
		signInPath:                           `{"code":"000000"}`,
		signOutPath:                          `{}`,
		"/api/backlog/goals/waiting/approve": wholeBudget,
	} {
		response := post(t, served, path, body, crossSite)
		if response.Code != http.StatusForbidden {
			t.Fatalf("a cross-site POST to %s answered %d, want 403", path, response.Code)
		}
	}
	// The read is refused the same way, and the session it could not read is
	// still there for the page that may read it.
	testutil.Require(t, "a cross-site read", get(t, served, sessionPath, crossSite).Code, http.StatusForbidden)
	testutil.Expect(t, "the session survived", sessionAnswer(t, get(t, served, sessionPath, carrying(cookie.Value))).SignedIn, true)

	// And the methods: a read route takes GET, a write route takes POST.
	testutil.Require(t, "POST to the read", post(t, served, sessionPath, `{}`, nil).Code, http.StatusMethodNotAllowed)
	testutil.Require(t, "GET to the write", get(t, served, signInPath, nil).Code, http.StatusMethodNotAllowed)
}

// A seat that knows nobody asks once, and never takes a name from a browser
// when it has one of its own.
func TestTheSheetNamesTheHumanOnlyWhereTheSeatDoesNot(t *testing.T) {
	t.Parallel()

	t.Run("asked", func(t *testing.T) {
		t.Parallel()
		held := newSigning(t, "", workingSecret)
		rec := &acted{authorized: agentStarted(), sessions: held.store}
		served := New(rec.acting(), loopback(), testBundle())

		nameless := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`"}`, nil)
		testutil.Require(t, "status without a name", nameless.Code, http.StatusBadRequest)

		held.at = routeNow.Add(time.Duration(channel.TOTPStep) * time.Second)
		named := post(t, served, signInPath, `{"code":"`+routeCode(t, held.at)+`","human":"Wido"}`, nil)
		testutil.Require(t, "status with a name", named.Code, http.StatusOK)
		testutil.Expect(t, "who signed in", sessionAnswer(t, named).Human, "Wido")
	})

	t.Run("ignored", func(t *testing.T) {
		t.Parallel()
		held := newSigning(t, "Wido", workingSecret)
		rec := &acted{authorized: agentStarted(), sessions: held.store}
		served := New(rec.acting(), loopback(), testBundle())

		response := post(t, served, signInPath, `{"code":"`+routeCode(t, routeNow)+`","human":"Somebody-Else"}`, nil)

		testutil.Require(t, "status", response.Code, http.StatusOK)
		testutil.Expect(t, "who signed in", sessionAnswer(t, response).Human, "Wido")
	})
}
