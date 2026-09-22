// Package session is the interface server's signed-in browser sessions.
//
// A human signs in with the same one-time code the channel already asks them
// for: the seat's TOTP secret, six digits, verified through the channel's own
// VerifyTOTP with the channel's own replay rule. What a successful sign-in
// mints is not a credential the browser can act with on its own — it is an
// opaque session identifier held in an HttpOnly cookie, and beside it, in this
// process's memory, the humanauthority proof that identifier stands for. The
// proof never leaves the process and the secret never enters the proof.
//
// The step a code was accepted at is durable: it is handed back to the server's
// own state record through Persist, and handed in again through LastStep when
// the server starts, so a code accepted before a restart cannot be replayed
// after one.
package session

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// SecretKey names the seat's one-time-code secret. It is the channel's key
// rather than one of the interface's own, deliberately: a human signs into the
// browser with the code they already carry for the fleet.
const SecretKey = "channel.human.totp-secret"

// Issuer is what the ledger records as the provider of a session proof.
const Issuer = "browser"

// Cookie is the name the session identifier travels under.
const Cookie = "ms_session"

// The refusal codes a sign-in answers with, each one a different thing for a
// human to do about it.
const (
	// CodeInvalid is a code this seat's secret does not produce now.
	CodeInvalid = "invalid-code"
	// CodeReplayed is a code from a step this server already accepted.
	CodeReplayed = "replayed-code"
	// CodeUnconfigured is a seat with no one-time-code secret at all.
	CodeUnconfigured = "no-secret"
	// CodeThrottled is a client that has failed too often to keep guessing.
	CodeThrottled = "too-many-attempts"
	// CodeHandle is a sign-in that named no usable handle.
	CodeHandle = "no-handle"
)

// Failures is how many wrong codes from one client close sign-in, and Cooling
// is how long it stays closed. Five is enough for a mistyped code twice over;
// a minute is long enough that guessing six digits is not a strategy.
const (
	Failures = 5
	Cooling  = time.Minute
)

// Refusal is one sign-in that was not made, in the words a human acts on.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Message }

func refuse(code, message string) *Refusal { return &Refusal{Code: code, Message: message} }

// Session is one signed-in browser, and the proof its acts carry.
type Session struct {
	ID        string
	Human     string
	CreatedAt time.Time
	Until     time.Time
	// Proof is the in-process observation this session stands for. It is
	// bound to this checkout and carries no secret.
	Proof humanauthority.Proof
}

// Liveness is what a cookie found.
type Liveness int

const (
	// Absent is a cookie that names no session this server minted.
	Absent Liveness = iota
	// Expired is a session whose hours have run out.
	Expired
	// Live is a session that may act.
	Live
)

// Snapshot is what the store hands back for the server's state record: the
// replay floor, the remembered handle, and one line per live session.
type Snapshot struct {
	LastStep int64
	Human    string
	Lines    []string
}

// Options is everything a store needs that it cannot decide for itself.
type Options struct {
	// Root is the checkout the minted proofs are bound to.
	Root string
	// Human is the handle this seat already knows: the boot proof's derived
	// name, or the configured one. Empty means the sheet asks once.
	Human string
	// Lifetime is how long a session lasts from the moment it is signed into.
	Lifetime time.Duration
	// Secret reads the seat's one-time-code secret, per sign-in, so a secret
	// configured while the server runs is picked up without a restart.
	Secret func() (string, error)
	// Now is the server's clock.
	Now func() time.Time
	// LastStep is the replay floor restored from the state record.
	LastStep int64
	// Persist carries a changed floor, handle or session set back to the
	// state record. It may be nil in a build that keeps no record.
	Persist func(Snapshot)
	// Random fills a slice with unguessable bytes. It may be nil, which takes
	// crypto/rand.
	Random func([]byte) error
}

// Store is one server's sessions.
type Store struct {
	options  Options
	mutex    sync.Mutex
	sessions map[string]*Session
	human    string
	lastStep int64
	failures map[string]*attempts
}

type attempts struct {
	count int
	until time.Time
}

func New(options Options) *Store {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.Random == nil {
		options.Random = func(into []byte) error {
			_, err := rand.Read(into)
			return err
		}
	}
	return &Store{
		options:  options,
		sessions: map[string]*Session{},
		human:    options.Human,
		lastStep: options.LastStep,
		failures: map[string]*attempts{},
	}
}

// Human is the handle this seat signs in as, or "" when nothing has named one
// and the sheet must ask.
func (s *Store) Human() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.human
}

// Configured reports whether the handle came from the seat rather than from a
// browser. A configured handle is never replaced by one a client supplies.
func (s *Store) Configured() bool { return strings.TrimSpace(s.options.Human) != "" }

// SignIn verifies one code and mints one session. client is what the failures
// are counted against; asked is the handle the sheet supplied, which is used
// only when this seat has none of its own.
func (s *Store) SignIn(client, code, asked string) (*Session, error) {
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if window, held := s.failures[client]; held && now.Before(window.until) {
		return nil, refuse(CodeThrottled, fmt.Sprintf("too many codes were refused from here; try again in %d seconds",
			int64(window.until.Sub(now)/time.Second)+1))
	}
	secret, err := s.secret()
	if err != nil {
		return nil, err
	}
	handle, err := s.handle(asked)
	if err != nil {
		return nil, err
	}
	step, ok := channel.VerifyTOTP(secret, code, now)
	if !ok {
		s.failed(client, now)
		return nil, refuse(CodeInvalid, "the code is not valid now")
	}
	// The channel's replay rule: a step is spent once. A step at or below the
	// floor is spent whether it was spent in this process or before a restart,
	// which is why the floor is durable.
	if step <= s.lastStep {
		s.failed(client, now)
		return nil, refuse(CodeReplayed, "that code was already used")
	}
	id, err := s.mint()
	if err != nil {
		return nil, err
	}
	proof, err := humanauthority.SignedInSessionProof(s.options.Root, handle, id, Issuer, now)
	if err != nil {
		return nil, refuse(CodeHandle, err.Error())
	}
	session := &Session{ID: id, Human: handle, CreatedAt: now, Until: now.Add(s.options.Lifetime), Proof: proof}
	s.sessions[id] = session
	s.lastStep = step
	s.human = handle
	delete(s.failures, client)
	s.persist(now)
	return session, nil
}

// Lookup reports what one cookie value names. An expired session is forgotten
// here rather than left to be found again.
func (s *Store) Lookup(id string) (*Session, Liveness) {
	if id == "" {
		return nil, Absent
	}
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	session, held := s.sessions[id]
	if !held {
		return nil, Absent
	}
	if !now.Before(session.Until) {
		delete(s.sessions, id)
		s.persist(now)
		return nil, Expired
	}
	return session, Live
}

// SignOut forgets one session. A cookie that names no session is not an error:
// signing out twice is signing out.
func (s *Store) SignOut(id string) {
	if id == "" {
		return
	}
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, held := s.sessions[id]; !held {
		return
	}
	delete(s.sessions, id)
	s.persist(now)
}

// Lines is one line per live session, for `ui status` to print.
func (s *Store) Lines() []string {
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lines(now)
}

// LastStep is the replay floor as it now stands.
func (s *Store) LastStep() int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lastStep
}

func (s *Store) lines(now time.Time) []string {
	lines := make([]string, 0, len(s.sessions))
	for _, session := range s.sessions {
		if !now.Before(session.Until) {
			continue
		}
		lines = append(lines, "signed in: "+session.Human+" until "+session.Until.Format(time.RFC3339))
	}
	sort.Strings(lines)
	return lines
}

func (s *Store) persist(now time.Time) {
	if s.options.Persist == nil {
		return
	}
	s.options.Persist(Snapshot{LastStep: s.lastStep, Human: s.human, Lines: s.lines(now)})
}

func (s *Store) failed(client string, now time.Time) {
	window := s.failures[client]
	if window == nil {
		window = &attempts{}
		s.failures[client] = window
	}
	window.count++
	if window.count >= Failures {
		window.count = 0
		window.until = now.Add(Cooling)
	}
}

func (s *Store) secret() (string, error) {
	if s.options.Secret == nil {
		return "", refuse(CodeUnconfigured, unconfigured)
	}
	secret, err := s.options.Secret()
	if err != nil || strings.TrimSpace(secret) == "" {
		return "", refuse(CodeUnconfigured, unconfigured)
	}
	return secret, nil
}

// unconfigured names the key and how the seat already sets it, because a seat
// that runs the fleet channel has this secret already and a seat that does not
// sets it the same way.
const unconfigured = "this seat has no one-time-code secret: set " + SecretKey +
	" in metasystem.conf.local, or in its environment variable, with the same base32 secret the channel's setup writes there" +
	" (bin/metasystem channel fake-code --secret <secret> prints the code it produces)"

// handle decides the name this sign-in acts under. A seat that knows its human
// never takes one from a browser; a seat that does not asks once and keeps the
// answer.
func (s *Store) handle(asked string) (string, error) {
	if s.human != "" {
		return s.human, nil
	}
	handle := strings.TrimSpace(asked)
	if handle == "" {
		return "", refuse(CodeHandle, "this seat does not know who you are: sign in with your name beside the code")
	}
	if strings.ContainsAny(handle, " \t\r\n") {
		return "", refuse(CodeHandle, "a handle is one word: the ledger records it beside the act")
	}
	return handle, nil
}

func (s *Store) mint() (string, error) {
	var value [32]byte
	if err := s.options.Random(value[:]); err != nil {
		return "", fmt.Errorf("the session identifier could not be generated: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(value[:]), nil
}

// Secret reads the seat's one-time-code secret through the kit's configuration
// reader, in the order every secret is read: the environment variable the key
// derives, then the uncommitted local file beside metasystem.conf. A value
// committed to metasystem.conf itself is refused rather than used, because a
// committed secret is not one.
func Secret(confPath string) (string, error) {
	if value, set := os.LookupEnv(config.EnvName(SecretKey)); set {
		return value, nil
	}
	if value, found, err := config.ConfLookup(confPath+".local", SecretKey); err == nil && found {
		return value, nil
	}
	if value, found, _ := config.ConfLookup(confPath, SecretKey); found && value != "" {
		return "", fmt.Errorf("committed secret setting %s is ignored", SecretKey)
	}
	return "", fmt.Errorf("no value configured for %s", SecretKey)
}

// ConfPath is the installation's configuration file, which is where every
// other ui. key is read from too.
func ConfPath(installation string) string { return filepath.Join(installation, "metasystem.conf") }
