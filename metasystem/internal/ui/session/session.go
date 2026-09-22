// Package session is the interface server's signed-in browser sessions.
//
// A human signs in with the same one-time code the channel already asks them
// for: the seat's TOTP secret, six digits, verified through the channel's own
// VerifyTOTP with the channel's own replay rule.
//
// A session has two names, and the distinction is the whole of its security.
// The BEARER is what the cookie holds and what this store is keyed by; holding
// it is being signed in. The REFERENCE is the session's public name: the proof
// carries it, the ledger's History records it, the authority artifact records
// it, and `ui status` prints it. The ledger is read by every seat in the fleet
// and by anyone with the checkout, so a bearer written into it would be a
// credential published to everybody; the reference authorizes nothing, which
// is why it is the one that travels.
//
// The replay floor is durable and fails closed. A code is accepted only after
// the step it was minted from has been recorded as spent, so a seat that
// cannot write the floor — a full disk, a read-only checkout, an unreadable
// floor file — refuses to sign anybody in rather than accepting a code it
// could accept again after a restart.
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

// Cookie is the name the bearer travels under.
const Cookie = "ms_session"

// The refusal codes a sign-in answers with, each one a different thing for a
// human to do about it.
const (
	// CodeInvalid is a code this seat's secret does not produce now.
	CodeInvalid = "invalid-code"
	// CodeReplayed is a code from a step this seat already spent.
	CodeReplayed = "replayed-code"
	// CodeUnconfigured is a seat with no one-time-code secret at all.
	CodeUnconfigured = "no-secret"
	// CodeUnrecorded is a seat that cannot record a code as spent, and so
	// cannot accept one.
	CodeUnrecorded = "floor-not-recorded"
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

// unrecordable opens every refusal that comes of a floor this seat cannot
// read or write. The cause follows it.
const unrecordable = "this seat cannot record the code as spent: "

// Refusal is one sign-in that was not made, in the words a human acts on.
type Refusal struct {
	Code    string
	Message string
}

func (r *Refusal) Error() string { return r.Message }

func refuse(code, message string) *Refusal { return &Refusal{Code: code, Message: message} }

// Session is one signed-in browser, as everything but the cookie sees it.
//
// There is no bearer in it. SignIn hands the bearer back separately, once, to
// the one caller that has to set the cookie; nothing that is recorded, logged
// or rendered is ever holding it.
type Session struct {
	// Reference is this session's public name. It is in the proof, in the
	// ledger, and in `ui status`, and it authorizes nothing.
	Reference string
	Human     string
	CreatedAt time.Time
	Until     time.Time
	// Proof is the in-process observation this session stands for. It is
	// bound to this checkout and carries no secret and no bearer.
	Proof humanauthority.Proof
}

// Liveness is what a bearer found.
type Liveness int

const (
	// Absent is a cookie that names no session this server minted.
	Absent Liveness = iota
	// Expired is a session whose hours have run out.
	Expired
	// Live is a session that may act.
	Live
)

// Options is everything a store needs that it cannot decide for itself.
type Options struct {
	// Root is the checkout the minted proofs are bound to.
	Root string
	// Human is the handle this seat already knew before any browser named
	// one: the boot proof's derived name, the configured one, or the one an
	// earlier run recorded. Empty means the sheet asks once.
	Human string
	// Lifetime is how long a session lasts from the moment it is signed into.
	Lifetime time.Duration
	// Secret reads the seat's one-time-code secret, per sign-in, so a secret
	// configured while the server runs is picked up without a restart.
	Secret func() (string, error)
	// Now is the server's clock.
	Now func() time.Time
	// Floor reads the durable replay floor and the handle an earlier sign-in
	// recorded. It is read per sign-in, so a floor file that becomes readable
	// again is picked up without a restart — and a floor that cannot be read
	// refuses the sign-in rather than quietly becoming zero. A nil Floor is a
	// seat with no durable floor, which signs nobody in.
	Floor func() (lastStep int64, human string, err error)
	// Record writes the floor and the handle durably. It is called before a
	// session exists, and a sign-in whose floor it refuses mints nothing.
	Record func(lastStep int64, human string) error
	// Lines carries the live session lines to this run's own record, for `ui
	// status` to print. It is evidence rather than a rule: nothing about
	// admission depends on it, so nothing here fails when it does.
	Lines func([]string)
	// Random fills a slice with unguessable bytes. It may be nil, which takes
	// crypto/rand.
	Random func([]byte) error
}

// Store is one server's sessions, keyed by bearer.
type Store struct {
	options  Options
	mutex    sync.Mutex
	sessions map[string]*Session
	human    string
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
		human:    strings.TrimSpace(options.Human),
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

// Configured reports whether this seat already knew a handle before any
// browser named one. A handle a client supplies never displaces it.
func (s *Store) Configured() bool { return strings.TrimSpace(s.options.Human) != "" }

// SignIn verifies one code and mints one session, returning the session and,
// separately, the bearer the cookie is to hold. client is what the failures
// are counted against; asked is the handle the sheet supplied, which is used
// only where nothing else has named one.
func (s *Store) SignIn(client, code, asked string) (*Session, string, error) {
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if window, held := s.failures[client]; held && now.Before(window.until) {
		return nil, "", refuse(CodeThrottled, fmt.Sprintf("too many codes were refused from here; try again in %d seconds",
			int64(window.until.Sub(now)/time.Second)+1))
	}
	secret, err := s.secret()
	if err != nil {
		return nil, "", err
	}
	// The floor is read before the code is even looked at, because a seat
	// that cannot read it cannot tell a fresh code from a spent one, and
	// answering a fresh-looking code would be answering a guess. A seat that
	// could not write it either refuses here, for the same reason.
	if s.options.Record == nil {
		return nil, "", refuse(CodeUnrecorded, unrecordable+"this seat keeps no durable floor")
	}
	floor, recorded, floorErr := s.floor()
	if floorErr != nil {
		return nil, "", refuse(CodeUnrecorded, unrecordable+floorErr.Error())
	}
	if recorded = strings.TrimSpace(recorded); recorded != "" && !s.Configured() {
		// A handle an earlier sign-in bound outlives this process, and no
		// later sign-in takes a different name under it.
		s.human = recorded
	}
	handle, err := s.handle(asked)
	if err != nil {
		return nil, "", err
	}
	step, ok := channel.VerifyTOTP(secret, code, now)
	if !ok {
		s.failed(client, now)
		return nil, "", refuse(CodeInvalid, "the code is not valid now")
	}
	// The channel's replay rule: a step is spent once. A step at or below the
	// floor is spent whether it was spent in this process or before a restart,
	// which is why the floor is durable.
	if step <= floor {
		s.failed(client, now)
		return nil, "", refuse(CodeReplayed, "that code was already used")
	}
	// The floor moves before the session exists. A code this seat cannot
	// record as spent is a code it does not accept: recording after minting
	// would leave a failed write looking exactly like an accepted code that
	// can be presented again after a restart.
	if err := s.options.Record(step, handle); err != nil {
		return nil, "", refuse(CodeUnrecorded, unrecordable+err.Error())
	}
	bearer, err := s.mint()
	if err != nil {
		return nil, "", err
	}
	reference, err := s.mint()
	if err != nil {
		return nil, "", err
	}
	proof, err := humanauthority.SignedInSessionProof(s.options.Root, handle, reference, Issuer, now)
	if err != nil {
		return nil, "", refuse(CodeHandle, err.Error())
	}
	session := &Session{Reference: reference, Human: handle, CreatedAt: now, Until: now.Add(s.options.Lifetime), Proof: proof}
	s.sessions[bearer] = session
	s.human = handle
	delete(s.failures, client)
	s.report(now)
	return session, bearer, nil
}

// Lookup reports what one cookie value names. An expired session is forgotten
// here rather than left to be found again.
func (s *Store) Lookup(bearer string) (*Session, Liveness) {
	if bearer == "" {
		return nil, Absent
	}
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	session, held := s.sessions[bearer]
	if !held {
		return nil, Absent
	}
	if !now.Before(session.Until) {
		delete(s.sessions, bearer)
		s.report(now)
		return nil, Expired
	}
	return session, Live
}

// SignOut forgets one session. A cookie that names no session is not an error:
// signing out twice is signing out.
func (s *Store) SignOut(bearer string) {
	if bearer == "" {
		return
	}
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if _, held := s.sessions[bearer]; !held {
		return
	}
	delete(s.sessions, bearer)
	s.report(now)
}

// Lines is one line per live session, for `ui status` to print. Each names the
// session's reference, which is what the ledger records too, so a line here
// and an act there can be read as the same session.
func (s *Store) Lines() []string {
	now := s.options.Now().UTC()
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.lines(now)
}

func (s *Store) lines(now time.Time) []string {
	lines := make([]string, 0, len(s.sessions))
	for _, session := range s.sessions {
		if !now.Before(session.Until) {
			continue
		}
		lines = append(lines, "signed in: "+session.Human+" ("+session.Reference+") until "+session.Until.Format(time.RFC3339))
	}
	sort.Strings(lines)
	return lines
}

// report hands this run's own record the live session lines. Nothing about
// admission depends on them, so a caller that cannot write them loses evidence
// rather than a rule, and the sign-in that produced them stands.
func (s *Store) report(now time.Time) {
	if s.options.Lines == nil {
		return
	}
	s.options.Lines(s.lines(now))
}

func (s *Store) floor() (int64, string, error) {
	if s.options.Floor == nil {
		return 0, "", fmt.Errorf("this seat keeps no durable floor")
	}
	return s.options.Floor()
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
// answer, here and in the floor file, so the second sign-in cannot be somebody
// else.
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
