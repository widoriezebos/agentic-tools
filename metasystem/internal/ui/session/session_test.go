package session

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)

// A synthetic secret. It proves nothing about this machine and is not one:
// it is the base32 string every TOTP example uses, kept here so the tests
// never read a configured secret.
const testSecret = "JBSWY3DPEHPK3PXP"

var signedInAt = time.Date(2026, 9, 22, 9, 0, 0, 0, time.UTC)

type clock struct{ at time.Time }

func (c *clock) now() time.Time { return c.at }

type bed struct {
	store *Store
	clock *clock
	kept  []Snapshot
	root  string
}

func newBed(t *testing.T, options ...func(*Options)) *bed {
	t.Helper()
	held := &bed{clock: &clock{at: signedInAt}, root: t.TempDir()}
	opts := Options{
		Root:     held.root,
		Human:    "Wido",
		Lifetime: 12 * time.Hour,
		Secret:   func() (string, error) { return testSecret, nil },
		Now:      held.clock.now,
		Persist:  func(snap Snapshot) { held.kept = append(held.kept, snap) },
	}
	for _, apply := range options {
		apply(&opts)
	}
	held.store = New(opts)
	return held
}

func codeAt(t *testing.T, at time.Time) string {
	t.Helper()
	code, err := channel.TOTPCode(testSecret, at)
	if err != nil {
		t.Fatal(err)
	}
	return code
}

func refusalOf(t *testing.T, err error) *Refusal {
	t.Helper()
	var refusal *Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("refusal = %v, want a session.Refusal", err)
	}
	return refusal
}

// The code the channel's own generator produces signs a human in, and what
// comes back is a session bound to this checkout with no secret in it.
func TestAValidCodeSignsTheHumanIn(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	signed, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	testutil.Expect(t, "the handle it acts as", signed.Human, "Wido")
	testutil.Expect(t, "when it stops", signed.Until, signedInAt.Add(12*time.Hour))
	testutil.Expect(t, "the proof is this checkout's", signed.Proof.SessionValidFor(held.root), true)
	testutil.Expect(t, "the proof names the session", signed.Proof.ChannelRef, signed.ID)
	if len(signed.ID) < 40 || strings.ContainsAny(signed.ID, "+/= ") {
		t.Fatalf("the session identifier is not 32 unguessable bytes in base64url: %q", signed.ID)
	}
	if strings.Contains(signed.ID, testSecret) || signed.Proof.ChannelUser == testSecret {
		t.Fatal("the secret reached the session")
	}

	found, liveness := held.store.Lookup(signed.ID)
	testutil.Expect(t, "the cookie finds it", liveness, Live)
	testutil.Expect(t, "it is the same session", found.ID, signed.ID)

	if len(held.kept) != 1 || held.kept[0].LastStep == 0 || len(held.kept[0].Lines) != 1 {
		t.Fatalf("the record was not told what happened: %+v", held.kept)
	}
	if !strings.HasPrefix(held.kept[0].Lines[0], "signed in: Wido until ") {
		t.Fatalf("the status line reads %q", held.kept[0].Lines[0])
	}
}

func TestAWrongCodeSignsNobodyIn(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	signed, err := held.store.SignIn("127.0.0.1", "000000", "")
	if signed != nil {
		t.Fatalf("a wrong code minted %+v", signed)
	}
	refusal := refusalOf(t, err)
	testutil.Expect(t, "the code", refusal.Code, CodeInvalid)
	testutil.Expect(t, "the reason", refusal.Message, "the code is not valid now")
	testutil.Expect(t, "nothing was recorded", len(held.kept), 0)
}

// A step is spent once. The same code offered again is refused even though it
// is still the code this secret produces at this instant.
func TestACodeIsSpentOnce(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	code := codeAt(t, signedInAt)

	if _, err := held.store.SignIn("127.0.0.1", code, ""); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	_, err := held.store.SignIn("127.0.0.1", code, "")

	refusal := refusalOf(t, err)
	testutil.Expect(t, "the code", refusal.Code, CodeReplayed)
	testutil.Expect(t, "the reason", refusal.Message, "that code was already used")
}

// A restart is not a way to spend a code twice: the floor the first run wrote
// into the server's record is the floor the next run starts from.
func TestACodeIsStillSpentAfterARestart(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	code := codeAt(t, signedInAt)
	if _, err := held.store.SignIn("127.0.0.1", code, ""); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	carried := held.kept[len(held.kept)-1]

	restarted := newBed(t, func(o *Options) { o.LastStep = carried.LastStep })
	_, err := restarted.store.SignIn("127.0.0.1", code, "")

	testutil.Expect(t, "the code", refusalOf(t, err).Code, CodeReplayed)
	// And the next step still signs in, so the floor refuses a replay rather
	// than the seat.
	restarted.clock.at = signedInAt.Add(time.Duration(channel.TOTPStep) * time.Second)
	if _, err := restarted.store.SignIn("127.0.0.1", codeAt(t, restarted.clock.at), ""); err != nil {
		t.Fatalf("the next code was refused too: %v", err)
	}
}

func TestASessionStopsActingWhenItsHoursRunOut(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	signed, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	held.clock.at = signed.Until.Add(-time.Second)
	if _, liveness := held.store.Lookup(signed.ID); liveness != Live {
		t.Fatal("a session inside its hours is not live")
	}
	held.clock.at = signed.Until

	_, liveness := held.store.Lookup(signed.ID)
	testutil.Expect(t, "at its last instant", liveness, Expired)
	_, again := held.store.Lookup(signed.ID)
	testutil.Expect(t, "and it is forgotten", again, Absent)
	testutil.Expect(t, "the record has no live session", len(held.kept[len(held.kept)-1].Lines), 0)
}

func TestSigningOutForgetsTheSession(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	signed, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	held.store.SignOut(signed.ID)

	_, liveness := held.store.Lookup(signed.ID)
	testutil.Expect(t, "the cookie finds nothing", liveness, Absent)
	testutil.Expect(t, "the record has no live session", len(held.kept[len(held.kept)-1].Lines), 0)
	// Signing out twice is signing out, and a cookie nobody minted is not an
	// error either.
	held.store.SignOut(signed.ID)
	held.store.SignOut("not-a-session")
}

// Guessing six digits is not a strategy: five refusals from one client close
// sign-in for a minute, and the client that did not guess is unaffected.
func TestFiveRefusedCodesCloseSignInForAMinute(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	for attempt := 0; attempt < Failures; attempt++ {
		_, err := held.store.SignIn("127.0.0.1", "000000", "")
		if code := refusalOf(t, err).Code; code != CodeInvalid {
			t.Fatalf("attempt %d refused as %q, want %q", attempt+1, code, CodeInvalid)
		}
	}

	_, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	testutil.Expect(t, "the right code is refused too", refusalOf(t, err).Code, CodeThrottled)
	if _, other := held.store.SignIn("127.0.0.2", codeAt(t, signedInAt), ""); other != nil {
		t.Fatalf("another client was locked out too: %v", other)
	}

	held.clock.at = signedInAt.Add(Cooling)
	held.clock.at = held.clock.at.Add(time.Second)
	if _, after := held.store.SignIn("127.0.0.1", codeAt(t, held.clock.at), ""); after != nil {
		t.Fatalf("sign-in stayed closed after the minute: %v", after)
	}
}

// A seat with no secret cannot verify anything, and says so with the key and
// where it is set rather than calling the code wrong.
func TestASeatWithNoSecretSaysSo(t *testing.T) {
	t.Parallel()
	for name, secret := range map[string]func() (string, error){
		"no reader":  nil,
		"no value":   func() (string, error) { return "", nil },
		"unreadable": func() (string, error) { return "", errors.New("no value configured") },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			held := newBed(t, func(o *Options) { o.Secret = secret })

			_, err := held.store.SignIn("127.0.0.1", "000000", "")

			refusal := refusalOf(t, err)
			testutil.Expect(t, "the code", refusal.Code, CodeUnconfigured)
			if !strings.Contains(refusal.Message, SecretKey) || !strings.Contains(refusal.Message, "metasystem.conf.local") {
				t.Fatalf("the refusal does not name the key and where it is set: %q", refusal.Message)
			}
		})
	}
}

// A seat that knows its human never takes a name from a browser; a seat that
// does not asks once and keeps the answer.
func TestTheHandleComesFromTheSeatWhereverTheSeatHasOne(t *testing.T) {
	t.Parallel()

	t.Run("configured wins", func(t *testing.T) {
		t.Parallel()
		held := newBed(t)
		signed, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "Somebody-Else")
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		testutil.Expect(t, "the handle", signed.Human, "Wido")
		testutil.Expect(t, "the seat named it", held.store.Configured(), true)
	})

	t.Run("asked once", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options) { o.Human = "" })
		testutil.Expect(t, "nothing is known yet", held.store.Human(), "")

		if _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), ""); err == nil {
			t.Fatal("a nameless seat signed in without a name")
		}
		held.clock.at = signedInAt.Add(time.Duration(channel.TOTPStep) * time.Second)
		signed, err := held.store.SignIn("127.0.0.1", codeAt(t, held.clock.at), "Wido")
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		testutil.Expect(t, "the handle", signed.Human, "Wido")
		testutil.Expect(t, "and it is remembered", held.store.Human(), "Wido")
		testutil.Expect(t, "the record keeps it", held.kept[len(held.kept)-1].Human, "Wido")
		testutil.Expect(t, "the seat did not name it", held.store.Configured(), false)
	})

	t.Run("one word", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options) { o.Human = "" })
		_, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "Wido Riezebos")
		testutil.Expect(t, "the code", refusalOf(t, err).Code, CodeHandle)
	})
}

// The secret is read the way every secret is read, and a secret committed to
// metasystem.conf is refused rather than used.
func TestTheSecretIsReadFromTheUncommittedFileAndNeverTheCommittedOne(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	conf := filepath.Join(root, "metasystem.conf")

	if _, err := Secret(conf); err == nil {
		t.Fatal("a seat with no configuration produced a secret")
	}

	if err := os.WriteFile(conf, []byte(SecretKey+"="+testSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Secret(conf); err == nil || !strings.Contains(err.Error(), "committed") {
		t.Fatalf("a committed secret was not refused: %v", err)
	}

	if err := os.WriteFile(conf+".local", []byte(SecretKey+"="+testSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := Secret(conf)
	if err != nil {
		t.Fatalf("the uncommitted secret was not read: %v", err)
	}
	testutil.Expect(t, "the secret", read, testSecret)
}
