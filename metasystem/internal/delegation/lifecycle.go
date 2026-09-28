package delegation

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Phase is one public command of the delegate lifecycle. The set mirrors
// the command router of the retired scripts/agents/dispatch.sh one for one.
type Phase string

const (
	PhaseDispatch Phase = "dispatch"
	PhaseFollowUp Phase = "follow-up"
	PhaseWatch    Phase = "watch"
	PhaseStatus   Phase = "status"
	PhaseCancel   Phase = "cancel"
	PhaseClose    Phase = "close"
	PhaseReap     Phase = "reap"
)

// Phases lists the lifecycle's phases in dispatch.sh's router order.
func Phases() []Phase {
	return []Phase{PhaseDispatch, PhaseWatch, PhaseFollowUp, PhaseStatus, PhaseCancel, PhaseClose, PhaseReap}
}

// Config locates one lifecycle's installation.
type Config struct {
	// Root is the metasystem installation root ($root).
	Root string
	// RepoScope is the Git top level holding Root ($repo_scope); empty
	// resolves it through Git.
	RepoScope string
	// Engine is the engine binary path, named in texts and handed to the
	// launched adapters as METASYSTEM_BIN; empty is Root/bin/metasystem.
	Engine string
	// LookupEnv answers the environment the evidence root resolves under
	// (its variable and HOME); nil is os.LookupEnv.
	LookupEnv func(string) (string, bool)
}

// Lifecycle is the composed delegate lifecycle. It owns no decision of its
// own that an owner already makes; it sequences owner operations in
// dispatch.sh's order.
type Lifecycle struct {
	ports     Ports
	root      string
	repoScope string
	engine    string
	lookupEnv func(string) (string, bool)
}

// New composes a lifecycle over a complete port set.
func New(config Config, ports Ports) (*Lifecycle, error) {
	if err := ports.Validate(); err != nil {
		return nil, fmt.Errorf("delegation lifecycle refused construction: %w", err)
	}
	if config.Root == "" {
		return nil, fmt.Errorf("delegation lifecycle refused construction: the installation root is required")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	engine := config.Engine
	if engine == "" {
		engine = filepath.Join(root, "bin", "metasystem")
	}
	l := &Lifecycle{ports: ports, root: root, repoScope: config.RepoScope, engine: engine, lookupEnv: config.LookupEnv}
	if l.repoScope == "" {
		out, _, gitErr := ports.Git.Run(context.Background(), root, "rev-parse", "--show-toplevel")
		if gitErr != nil {
			return nil, fmt.Errorf("metasystem installation is not inside a git repository: %s", root)
		}
		scope := strings.TrimSpace(string(out))
		if resolved, resolveErr := filepath.EvalSymlinks(scope); resolveErr == nil {
			scope = resolved
		}
		l.repoScope = scope
	}
	return l, nil
}

// Ports returns the wired operations (for a caller composing a sibling
// action over the same owners).
func (l *Lifecycle) Ports() Ports { return l.ports }

// Root is the installation root the lifecycle acts on.
func (l *Lifecycle) Root() string { return l.root }

// Env is one invocation's per-call state that dispatch.sh read from its
// environment (design 6.2, VOA-14): the entry fills it once from its own
// environment, and an in-process caller fills it explicitly.
type Env struct {
	// RecordOutcome is METASYSTEM_DELEGATE_OUTCOME_FILE being set: typed
	// outcomes are recorded into Result.Outcome.
	RecordOutcome bool
	// DelegateInternal is METASYSTEM_DELEGATE_INTERNAL=1: the caller is the
	// delegate boundary, not an operator reaching the legacy grammar.
	DelegateInternal bool
	// ClaimCapability is the delegate claim capability the boundary minted
	// (METASYSTEM_DELEGATE_CLAIM_CAPABILITY).
	ClaimCapability string
	// OwnerLineage is METASYSTEM_OWNER_LINEAGE.
	OwnerLineage string
	// MissionID, MissionLease and MissionTurn are the inherited mission
	// context (METASYSTEM_MISSION_ID, _LEASE, _TURN).
	MissionID, MissionLease, MissionTurn string
	// FixtureCapScaleMilli is METASYSTEM_FIXTURE_CAP_SCALE_MILLI.
	FixtureCapScaleMilli string
	// HandshakePollMS is METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS.
	HandshakePollMS string
	// FixtureHazard is METASYSTEM_DISPATCH_FIXTURE_HAZARD.
	FixtureHazard string
	// GuardFixture and GuardFixtureRoot are the checkout execution guard's
	// fixture control (METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE, _ROOT).
	GuardFixture, GuardFixtureRoot string
	// FixturePauseBeforeLaunch is METASYSTEM_FIXTURE_PAUSE_BEFORE_LAUNCH.
	FixturePauseBeforeLaunch string
	// TempDir is TMPDIR (empty is the system default).
	TempDir string
}

// EnvFromEnviron reads the invocation state from an environment lookup.
func EnvFromEnviron(lookup func(string) (string, bool)) Env {
	get := func(key string) string {
		value, _ := lookup(key)
		return value
	}
	return Env{
		RecordOutcome:            get("METASYSTEM_DELEGATE_OUTCOME_FILE") != "",
		DelegateInternal:         get("METASYSTEM_DELEGATE_INTERNAL") == "1",
		ClaimCapability:          get("METASYSTEM_DELEGATE_CLAIM_CAPABILITY"),
		OwnerLineage:             get("METASYSTEM_OWNER_LINEAGE"),
		MissionID:                get("METASYSTEM_MISSION_ID"),
		MissionLease:             get("METASYSTEM_MISSION_LEASE"),
		MissionTurn:              get("METASYSTEM_MISSION_TURN"),
		FixtureCapScaleMilli:     get("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"),
		HandshakePollMS:          get("METASYSTEM_HANDSHAKE_POLL_INTERVAL_MS"),
		FixtureHazard:            get("METASYSTEM_DISPATCH_FIXTURE_HAZARD"),
		GuardFixture:             get("METASYSTEM_CHECKOUT_EXECUTION_GUARD_FIXTURE"),
		GuardFixtureRoot:         get("METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT"),
		FixturePauseBeforeLaunch: get("METASYSTEM_FIXTURE_PAUSE_BEFORE_LAUNCH"),
		TempDir:                  get("TMPDIR"),
	}
}

// Request is one invocation of the lifecycle's command grammar.
type Request struct {
	// Invocation is the supplied caller identity (design 6.2). An entry
	// process and an in-process caller both supply the current process.
	Invocation Invocation
	Env        Env
	// LockTag is the instance tag the invocation's owner locks carry. A
	// contender judges the holder alive only while the holder's argv
	// contains it, so it must be a substring of the current process's
	// command line (LockTagOf builds one).
	LockTag string
	// Stdin feeds the escalation approval prompt; StdinTTY and StderrTTY
	// report whether both ends are interactive terminals.
	Stdin     io.Reader
	StdinTTY  bool
	StderrTTY bool
	// Stderr receives every diagnostic the retired script wrote to its
	// standard error; nil discards.
	Stderr io.Writer
}

// LockTagOf is the owner-lock instance tag for a process whose argv is args:
// the whole command line, which the kernel's argv read reproduces exactly.
func LockTagOf(args []string) string { return strings.Join(args, " ") }

// Result is what one command produced: the standard output the retired
// script printed, the typed outcome it recorded, and its exit code.
type Result struct {
	ExitCode int
	Stdout   []byte
	// Outcome is the typed outcome JSON line when one was recorded.
	Outcome []byte
}

// Exit is a command's process-shaped end: the exit code dispatch.sh
// returned, with the message it printed when it died.
type Exit struct {
	Code    int
	Message string
}

func (e *Exit) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("exit %d", e.Code)
}

// ExitCode maps an error to its exit code: an *Exit keeps its code, any
// other error is 1, nil is 0.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *Exit
	if errors.As(err, &exit) {
		return exit.Code
	}
	return 1
}

// Run executes one command of the retired dispatch.sh grammar and returns
// what it produced. argv is the script's argv after its path.
func (l *Lifecycle) Run(ctx context.Context, request Request, argv []string) Result {
	s := l.newSession(ctx, request)
	err := s.route(argv)
	code := ExitCode(err)
	if s.trap != nil {
		trap := s.trap
		s.trap = nil
		if trapErr := trap(code); trapErr != nil && code == 0 {
			code = ExitCode(trapErr)
		}
	}
	return Result{ExitCode: code, Stdout: s.stdout.Bytes(), Outcome: s.outcome}
}

func (l *Lifecycle) newSession(ctx context.Context, request Request) *session {
	if request.Invocation.CallerPid == 0 {
		request.Invocation.CallerPid = int64(os.Getpid())
	}
	if request.LockTag == "" {
		request.LockTag = LockTagOf(os.Args)
	}
	stderr := request.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	s := &session{
		l: l, ctx: ctx, request: request, env: request.Env, stderr: stderr,
		root: l.root, repoScope: l.repoScope, ms: l.engine,
		inv:       request.Invocation,
		tag:       request.LockTag,
		guardRoot: l.root,
	}
	s.inv.ClaimEpoch, s.inv.MainID, s.inv.CallerClass = nil, "", ""
	s.agents = filepath.Join(s.root, "artifacts", "agents")
	s.jobs = filepath.Join(s.agents, "jobs")
	s.heartbeats = filepath.Join(s.agents, "hb")
	s.locks = filepath.Join(s.agents, "locks")
	s.recordLocks = filepath.Join(s.agents, "record-locks")
	s.capabilities = filepath.Join(s.agents, "capabilities")
	s.worktrees = filepath.Join(s.agents, "worktrees")
	if s.env.GuardFixture != "" && s.env.GuardFixtureRoot != "" {
		s.guardRoot = s.env.GuardFixtureRoot
	}
	return s
}

// session is one command's state: the globals the retired script kept.
type session struct {
	l         *Lifecycle
	ctx       context.Context
	request   Request
	env       Env
	stdout    bytes.Buffer
	stderr    io.Writer
	outcome   []byte
	root      string
	repoScope string
	ms        string

	agents, jobs, heartbeats, locks, recordLocks, capabilities, worktrees string

	inv  Invocation
	tag  string
	trap func(code int) error

	leaseReentry bool

	capAuthorityHeld bool
	goalLockHeld     bool
	goalLockDir      string

	stopCancelAuthorized string
	cancelRefusalClass   string
	windDownKilled       bool

	lastDie string
	// dieJob and dieChild are the job and child ids die names in its typed
	// refusal (dispatch.sh's ${job:-${child:-}}).
	dieJob, dieChild string
	// mission is dispatch_job/follow_up's mission, read by the setup-husk
	// cleanup.
	mission string

	cleanupJob           string
	cleanupChain         string
	cleanupAuthorization string
	cleanupMessage       string
	cleanupSubject       string
	cleanupAdmission     string
	cleanupContinuation  string
	cleanupPrompt        string
	cleanupComposition   string
	cleanupStage         string
	cleanupLifecycle     string
	cleanupCreationClaim string

	guardHeld bool
	guardRoot string
}

func (s *session) printf(format string, args ...any) { fmt.Fprintf(&s.stdout, format, args...) }

func (s *session) println(line string) { fmt.Fprintln(&s.stdout, line) }

func (s *session) eprintf(format string, args ...any) { fmt.Fprintf(s.stderr, format, args...) }

func (s *session) eprintln(line string) { fmt.Fprintln(s.stderr, line) }

// die is dispatch.sh's die: a typed REFUSED-INTERNAL outcome unless one was
// already recorded, the message on stderr, and the exit code.
func (s *session) die(code int, message string) error {
	s.lastDie = message
	if s.env.RecordOutcome && len(s.outcome) == 0 {
		job := s.dieJob
		if job == "" {
			job = s.dieChild
		}
		s.recordOutcome("REFUSED-INTERNAL", "refused", message, job)
	}
	s.eprintln(message)
	return &Exit{Code: code, Message: message}
}

// usageExit is dispatch.sh's `{ usage; exit 2; }`.
func (s *session) usageExit() error {
	s.eprintln(usageText)
	return &Exit{Code: 2}
}

// exitWith is a bare `exit N`/`return N`: no message, no outcome.
func exitWith(code int) error {
	if code == 0 {
		return nil
	}
	return &Exit{Code: code}
}

const usageText = `Usage:
  metasystem internal delegate --role <role> --brief <file> --goal <id|none-explicit> --destructive-reach <class> [--op <id>] [--wait]
  metasystem internal delegate --follow-up <job-id> --brief <file> [--op <id>] [--wait]
  metasystem internal delegate --cancel <job-id>

Exit codes: 0 success/completed; 2 usage; 3 failed; 4 timeout;
5 vanished; 6 unknown status job; 7 malformed status record; 8 cancelled;
10 critique cap exhausted and waiting on a human raise.`
