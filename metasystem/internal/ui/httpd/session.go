package httpd

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/session"
)

// Signing in, and who the server is acting as.
//
// The interface has always been able to act as the human who started it, and
// only for as long as that was the whole story: a server started from the
// enrolled terminal carries a boot proof, and a server started any other way
// carries a reason instead and refuses every act. Signing in is the second way
// a human reaches their own backlog from a browser — with the seat's one-time
// code, the same six digits the fleet channel asks for.
//
// What the browser holds afterwards is the session's bearer in an HttpOnly
// cookie, and nothing else: no name, no proof, no secret. The proof that
// bearer stands for lives in this process, and the cookie is how a request
// names it. SameSite=Strict, beside the same-site and origin checks every
// other route takes, is why a page on another origin cannot spend it.
//
// The bearer is not what the ledger records. A session act names the
// session's REFERENCE — a second random value, minted beside the bearer and
// carrying no power — because History is read by every seat in the fleet, and
// a bearer written there would be a credential handed to all of them.
const (
	sessionPath  = "/api/session"
	signInPath   = "/api/session/sign-in"
	signOutPath  = "/api/session/sign-out"
	routeSignIn  = "sign-in"
	routeSignOut = "sign-out"
)

// The three things a request can be acting under.
const (
	// sourceCode is a browser session a human signed into with the code.
	sourceCode = "code"
	// sourceTerminal is the boot proof: this server was started from the
	// human's own terminal and has acted as them ever since.
	sourceTerminal = "terminal"
	// sourceNone is a server nothing proves.
	sourceNone = "none"
)

// signInBody is what the sheet sends: the six digits, and — only on a seat
// that does not know its human — the handle to record acts under.
type signInBody struct {
	Code  string `json:"code"`
	Human string `json:"human"`
}

// sessionState is what every session route answers with, signed in or not, so
// one shape tells the page everything it renders the identity control from.
type sessionState struct {
	Human    string `json:"human"`
	SignedIn bool   `json:"signedIn"`
	// Until is when this session stops acting, in RFC3339. The terminal's
	// authority has no expiry and carries none.
	Until  string `json:"until"`
	Source string `json:"source"`
}

// cookieOf reads the bearer a request carries, or "" for none.
func cookieOf(r *http.Request) string {
	carried, err := r.Cookie(session.Cookie)
	if err != nil || carried == nil {
		return ""
	}
	return carried.Value
}

// signedIn reports the live session a request carries, and what the cookie
// found. A nil store is a build with no sessions, which finds nothing.
func (h *handler) signedIn(r *http.Request) (*session.Session, session.Liveness) {
	if h.info.Sessions == nil {
		return nil, session.Absent
	}
	return h.info.Sessions.Lookup(cookieOf(r))
}

// state is who this server is acting as for one request.
func (h *handler) state(r *http.Request) sessionState {
	signed, liveness := h.signedIn(r)
	if liveness == session.Live {
		return sessionState{Human: signed.Human, SignedIn: true,
			Until: signed.Until.UTC().Format(time.RFC3339), Source: sourceCode}
	}
	if h.info.Authority.Proven {
		return sessionState{Human: h.info.Authority.Human, SignedIn: true, Source: sourceTerminal}
	}
	return sessionState{Human: h.knownHuman(), SignedIn: false, Source: sourceNone}
}

// knownHuman is the handle the sheet does not have to ask for: the boot
// proof's derived name, or the one this seat configured and the store keeps.
func (h *handler) knownHuman() string {
	if h.info.Authority.Human != "" {
		return h.info.Authority.Human
	}
	if h.info.Sessions == nil {
		return ""
	}
	return h.info.Sessions.Human()
}

// session answers who the server is acting as. It is a read, so it takes GET
// like every other read, and it says the same thing whether anybody is signed
// in or not.
func (h *handler) session(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(h.state(r))
}

func (h *handler) signIn(w http.ResponseWriter, r *http.Request) {
	if h.info.Sessions == nil {
		writeFailure(w, "this engine was built without browser sessions")
		return
	}
	var body signInBody
	if !decode(w, r, &body) {
		return
	}
	signed, bearer, err := h.info.Sessions.SignIn(clientOf(r), body.Code, body.Human)
	if err != nil {
		var refusal *session.Refusal
		if errors.As(err, &refusal) {
			w.WriteHeader(signInStatus(refusal.Code))
			writeActRefusal(w, refusal.Code, refusal.Message)
			return
		}
		writeFailure(w, err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     session.Cookie,
		Value:    bearer,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  signed.Until.UTC(),
	})
	_ = json.NewEncoder(w).Encode(sessionState{
		Human: signed.Human, SignedIn: true,
		Until: signed.Until.UTC().Format(time.RFC3339), Source: sourceCode,
	})
}

func (h *handler) signOut(w http.ResponseWriter, r *http.Request) {
	if h.info.Sessions == nil {
		writeFailure(w, "this engine was built without browser sessions")
		return
	}
	h.info.Sessions.SignOut(cookieOf(r))
	// The cookie is cleared with the same attributes it was set with, so the
	// browser replaces the one it holds rather than keeping a second.
	http.SetCookie(w, &http.Cookie{
		Name: session.Cookie, Value: "", Path: "/",
		HttpOnly: true, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
	// The request still carries the cookie it just spent, so the answer is
	// composed from what is true now rather than from that request.
	state := sessionState{Human: h.knownHuman(), SignedIn: false, Source: sourceNone}
	if h.info.Authority.Proven {
		state = sessionState{Human: h.info.Authority.Human, SignedIn: true, Source: sourceTerminal}
	}
	_ = json.NewEncoder(w).Encode(state)
}

// signInStatus says what a human can do about a refused sign-in: 401 for a
// code that did not prove anything, 429 for a client that has to wait, 503 for
// a seat that cannot verify a code or record one as spent, and 400 for a
// request that was wrong. The two 503s are both "nothing you typed was
// wrong; this seat cannot do its part".
func signInStatus(code string) int {
	switch code {
	case session.CodeInvalid, session.CodeReplayed:
		return http.StatusUnauthorized
	case session.CodeThrottled:
		return http.StatusTooManyRequests
	case session.CodeUnconfigured, session.CodeUnrecorded:
		return http.StatusServiceUnavailable
	default:
		return http.StatusBadRequest
	}
}

// clientOf is what failed codes are counted against. The server listens on
// loopback, so this is one browser on this machine; the port is dropped so
// that a fresh connection is the same client.
func clientOf(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
