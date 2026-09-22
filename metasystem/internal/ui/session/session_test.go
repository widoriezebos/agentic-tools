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

// bed is one store over an in-memory floor, with a way to break the floor the
// way a full or read-only disk breaks it.
type bed struct {
	store *Store
	clock *clock
	root  string

	floor      int64
	remembered string
	// readErr and writeErr stand in for a floor file that cannot be read and
	// one that cannot be written.
	readErr  error
	writeErr error
	// recorded is every floor this store wrote, in order.
	recorded [][2]any
	lines    [][]string
}

func newBed(t *testing.T, options ...func(*Options, *bed)) *bed {
	t.Helper()
	held := &bed{clock: &clock{at: signedInAt}, root: t.TempDir()}
	opts := Options{
		Root:     held.root,
		Human:    "Wido",
		Lifetime: 12 * time.Hour,
		Secret:   func() (string, error) { return testSecret, nil },
		Now:      held.clock.now,
		Floor: func() (int64, string, error) {
			if held.readErr != nil {
				return 0, "", held.readErr
			}
			return held.floor, held.remembered, nil
		},
		Record: func(lastStep int64, human string) error {
			if held.writeErr != nil {
				return held.writeErr
			}
			held.floor, held.remembered = lastStep, human
			held.recorded = append(held.recorded, [2]any{lastStep, human})
			return nil
		},
		Lines: func(lines []string) { held.lines = append(held.lines, lines) },
	}
	for _, apply := range options {
		apply(&opts, held)
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
// comes back is a session bound to this checkout with no secret in it — and a
// bearer that is handed back separately and is in nothing else.
func TestAValidCodeSignsTheHumanIn(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	signed, bearer, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	testutil.Expect(t, "the handle it acts as", signed.Human, "Wido")
	testutil.Expect(t, "when it stops", signed.Until, signedInAt.Add(12*time.Hour))
	testutil.Expect(t, "the proof is this checkout's", signed.Proof.SessionValidFor(held.root), true)
	testutil.Expect(t, "the proof names the reference", signed.Proof.ChannelRef, signed.Reference)
	for name, value := range map[string]string{"bearer": bearer, "reference": signed.Reference} {
		if len(value) < 40 || strings.ContainsAny(value, "+/= ") {
			t.Fatalf("the %s is not 32 unguessable bytes in base64url: %q", name, value)
		}
	}
	if bearer == signed.Reference {
		t.Fatal("the bearer and the reference are the same value")
	}
	if strings.Contains(bearer, testSecret) || signed.Proof.ChannelUser == testSecret {
		t.Fatal("the secret reached the session")
	}

	found, liveness := held.store.Lookup(bearer)
	testutil.Expect(t, "the cookie finds it", liveness, Live)
	testutil.Expect(t, "it is the same session", found.Reference, signed.Reference)
	_, byReference := held.store.Lookup(signed.Reference)
	testutil.Expect(t, "the reference opens nothing", byReference, Absent)
}

// The bearer is what the cookie holds and what nothing else holds. Every line
// this store hands out names the reference instead, because those lines are
// read by everybody the ledger is read by.
func TestNothingButTheCookieHoldsTheBearer(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	signed, bearer, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	lines := held.store.Lines()
	if len(lines) != 1 || !strings.Contains(lines[0], signed.Reference) {
		t.Fatalf("the status line does not name the reference: %v", lines)
	}
	for _, line := range lines {
		if strings.Contains(line, bearer) {
			t.Fatalf("a status line carries the bearer: %q", line)
		}
	}
	for _, carried := range []string{
		signed.Proof.ChannelRef, signed.Proof.ChannelUser, signed.Proof.ChannelProvider,
		signed.Reference, signed.Human,
	} {
		if carried == bearer {
			t.Fatalf("the bearer travels as %q", carried)
		}
	}
	// And what the floor recorded is the step and the handle, never a bearer.
	for _, wrote := range held.recorded {
		if wrote[1] == bearer {
			t.Fatalf("the floor file carries the bearer: %v", wrote)
		}
	}
}

func TestAWrongCodeSignsNobodyIn(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	signed, bearer, err := held.store.SignIn("127.0.0.1", "000000", "")
	if signed != nil || bearer != "" {
		t.Fatalf("a wrong code minted %+v %q", signed, bearer)
	}
	refusal := refusalOf(t, err)
	testutil.Expect(t, "the code", refusal.Code, CodeInvalid)
	testutil.Expect(t, "the reason", refusal.Message, "the code is not valid now")
	testutil.Expect(t, "the floor did not move", len(held.recorded), 0)
}

// A step is spent once. The same code offered again is refused even though it
// is still the code this secret produces at this instant.
func TestACodeIsSpentOnce(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	code := codeAt(t, signedInAt)

	if _, _, err := held.store.SignIn("127.0.0.1", code, ""); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	_, _, err := held.store.SignIn("127.0.0.1", code, "")

	refusal := refusalOf(t, err)
	testutil.Expect(t, "the code", refusal.Code, CodeReplayed)
	testutil.Expect(t, "the reason", refusal.Message, "that code was already used")
}

// A restart is not a way to spend a code twice: the floor the first run wrote
// is the floor the next run reads.
func TestACodeIsStillSpentAfterARestart(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	code := codeAt(t, signedInAt)
	if _, _, err := held.store.SignIn("127.0.0.1", code, ""); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	carried := held.floor

	restarted := newBed(t, func(o *Options, b *bed) { b.floor = carried })
	_, _, err := restarted.store.SignIn("127.0.0.1", code, "")

	testutil.Expect(t, "the code", refusalOf(t, err).Code, CodeReplayed)
	// And the next step still signs in, so the floor refuses a replay rather
	// than the seat.
	restarted.clock.at = signedInAt.Add(time.Duration(channel.TOTPStep) * time.Second)
	if _, _, err := restarted.store.SignIn("127.0.0.1", codeAt(t, restarted.clock.at), ""); err != nil {
		t.Fatalf("the next code was refused too: %v", err)
	}
}

// The floor fails closed. A seat that cannot record a code as spent does not
// accept it: accepting one and failing to write the floor would leave that
// code usable again after a restart, which is the one thing the floor is for.
func TestASeatThatCannotRecordTheFloorSignsNobodyIn(t *testing.T) {
	t.Parallel()

	t.Run("the write fails", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { b.writeErr = errors.New("no space left on device") })

		signed, bearer, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")

		if signed != nil || bearer != "" {
			t.Fatalf("a code that could not be recorded minted %+v %q", signed, bearer)
		}
		refusal := refusalOf(t, err)
		testutil.Expect(t, "the code", refusal.Code, CodeUnrecorded)
		testutil.Expect(t, "the reason", refusal.Message,
			"this seat cannot record the code as spent: no space left on device")
		testutil.Expect(t, "nothing is signed in", len(held.store.Lines()), 0)
	})

	t.Run("the read fails", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { b.readErr = errors.New("sessions.json is schema 2") })

		_, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")

		refusal := refusalOf(t, err)
		testutil.Expect(t, "the code", refusal.Code, CodeUnrecorded)
		testutil.Expect(t, "the reason", refusal.Message,
			"this seat cannot record the code as spent: sessions.json is schema 2")
	})

	t.Run("there is no floor at all", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { o.Floor, o.Record = nil, nil })

		_, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")

		refusal := refusalOf(t, err)
		testutil.Expect(t, "the code", refusal.Code, CodeUnrecorded)
		if !strings.Contains(refusal.Message, "no durable floor") {
			t.Fatalf("the refusal does not say why: %q", refusal.Message)
		}
	})

	t.Run("and the code stays unspent", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { b.writeErr = errors.New("read-only file system") })
		code := codeAt(t, signedInAt)
		if _, _, err := held.store.SignIn("127.0.0.1", code, ""); err == nil {
			t.Fatal("a code was accepted on a seat that could not record it")
		}

		// The disk comes back; the same code is still this step's, and it is
		// accepted now, because nothing pretended it had been spent.
		held.writeErr = nil
		if _, _, err := held.store.SignIn("127.0.0.1", code, ""); err != nil {
			t.Fatalf("the code was refused after the floor became writable: %v", err)
		}
	})
}

func TestASessionStopsActingWhenItsHoursRunOut(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	signed, bearer, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	held.clock.at = signed.Until.Add(-time.Second)
	if _, liveness := held.store.Lookup(bearer); liveness != Live {
		t.Fatal("a session inside its hours is not live")
	}
	held.clock.at = signed.Until

	_, liveness := held.store.Lookup(bearer)
	testutil.Expect(t, "at its last instant", liveness, Expired)
	_, again := held.store.Lookup(bearer)
	testutil.Expect(t, "and it is forgotten", again, Absent)
	testutil.Expect(t, "no live session is reported", len(held.lines[len(held.lines)-1]), 0)
}

func TestSigningOutForgetsTheSession(t *testing.T) {
	t.Parallel()
	held := newBed(t)
	_, bearer, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	if err != nil {
		t.Fatalf("sign in: %v", err)
	}

	held.store.SignOut(bearer)

	_, liveness := held.store.Lookup(bearer)
	testutil.Expect(t, "the cookie finds nothing", liveness, Absent)
	testutil.Expect(t, "no live session is reported", len(held.lines[len(held.lines)-1]), 0)
	// Signing out twice is signing out, and a cookie nobody minted is not an
	// error either.
	held.store.SignOut(bearer)
	held.store.SignOut("not-a-session")
}

// Guessing six digits is not a strategy: five refusals from one client close
// sign-in for a minute, and the client that did not guess is unaffected.
func TestFiveRefusedCodesCloseSignInForAMinute(t *testing.T) {
	t.Parallel()
	held := newBed(t)

	for attempt := 0; attempt < Failures; attempt++ {
		_, _, err := held.store.SignIn("127.0.0.1", "000000", "")
		if code := refusalOf(t, err).Code; code != CodeInvalid {
			t.Fatalf("attempt %d refused as %q, want %q", attempt+1, code, CodeInvalid)
		}
	}

	_, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "")
	testutil.Expect(t, "the right code is refused too", refusalOf(t, err).Code, CodeThrottled)
	if _, _, other := held.store.SignIn("127.0.0.2", codeAt(t, signedInAt), ""); other != nil {
		t.Fatalf("another client was locked out too: %v", other)
	}

	held.clock.at = signedInAt.Add(Cooling).Add(time.Second)
	if _, _, after := held.store.SignIn("127.0.0.1", codeAt(t, held.clock.at), ""); after != nil {
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
			held := newBed(t, func(o *Options, b *bed) { o.Secret = secret })

			_, _, err := held.store.SignIn("127.0.0.1", "000000", "")

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
		signed, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "Somebody-Else")
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		testutil.Expect(t, "the handle", signed.Human, "Wido")
		testutil.Expect(t, "the seat named it", held.store.Configured(), true)
	})

	t.Run("asked once", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { o.Human = "" })
		testutil.Expect(t, "nothing is known yet", held.store.Human(), "")

		if _, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), ""); err == nil {
			t.Fatal("a nameless seat signed in without a name")
		}
		held.clock.at = signedInAt.Add(time.Duration(channel.TOTPStep) * time.Second)
		signed, _, err := held.store.SignIn("127.0.0.1", codeAt(t, held.clock.at), "Wido")
		if err != nil {
			t.Fatalf("sign in: %v", err)
		}
		testutil.Expect(t, "the handle", signed.Human, "Wido")
		testutil.Expect(t, "and it is remembered", held.store.Human(), "Wido")
		testutil.Expect(t, "the floor keeps it", held.remembered, "Wido")
		testutil.Expect(t, "the seat did not name it", held.store.Configured(), false)
	})

	t.Run("one word", func(t *testing.T) {
		t.Parallel()
		held := newBed(t, func(o *Options, b *bed) { o.Human = "" })
		_, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "Wido Riezebos")
		testutil.Expect(t, "the code", refusalOf(t, err).Code, CodeHandle)
	})
}

// The first sign-in on a seat that names nobody binds the handle — here, and
// in the floor file, so that a later sign-in cannot be somebody else and a
// restart does not reopen the question.
func TestTheFirstSignInBindsTheHandleForGood(t *testing.T) {
	t.Parallel()
	held := newBed(t, func(o *Options, b *bed) { o.Human = "" })

	first, _, err := held.store.SignIn("127.0.0.1", codeAt(t, signedInAt), "Wido")
	if err != nil {
		t.Fatalf("first sign in: %v", err)
	}
	testutil.Expect(t, "who it bound", first.Human, "Wido")

	// A second sign-in on the same store, naming somebody else.
	held.clock.at = signedInAt.Add(time.Duration(channel.TOTPStep) * time.Second)
	second, _, err := held.store.SignIn("127.0.0.1", codeAt(t, held.clock.at), "Somebody-Else")
	if err != nil {
		t.Fatalf("second sign in: %v", err)
	}
	testutil.Expect(t, "who it acts as", second.Human, "Wido")

	// And after a restart, which reads the handle back out of the floor.
	restarted := newBed(t, func(o *Options, b *bed) {
		o.Human = ""
		b.floor, b.remembered = held.floor, held.remembered
	})
	restarted.clock.at = held.clock.at.Add(time.Duration(channel.TOTPStep) * time.Second)
	after, _, err := restarted.store.SignIn("127.0.0.1", codeAt(t, restarted.clock.at), "Somebody-Else")
	if err != nil {
		t.Fatalf("sign in after the restart: %v", err)
	}
	testutil.Expect(t, "who it acts as after a restart", after.Human, "Wido")
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
