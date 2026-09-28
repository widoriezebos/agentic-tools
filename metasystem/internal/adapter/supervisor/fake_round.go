package supervisor

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/delegate"
)

// The fake delegate round (formerly scripts/agents/adapters/fake.sh's
// supervise). The shared round owns the launch capability, the gate, the
// envelope comparison, reference verification, the handshake callback and
// the terminal record; the fake contributes what its CLI would: the
// handshake facts through observe, the FAKE:<behavior> markers acted out by
// the simulated CLI, and the named refusals and return candidate through
// finalize.

// fakeRound is one delegate round's simulated CLI and what it observed.
type fakeRound struct {
	t         *Turn
	d         Deps
	heartbeat string
	interval  time.Duration
	// handshaken closes when the shared layer recorded the handshake.
	handshaken chan struct{}
	once       sync.Once

	mu      sync.Mutex
	ready   bool
	session string

	// refusal and err are the simulated CLI's outcome other than a
	// candidate: a named running failure, or a failure that ends the round
	// without a record write (the script's `set -e` exits).
	refusal *Refusal
	err     error
}

// prepareFakeRound is everything the script did before the handshake that
// the shared preparation does not: the fake's own job-log banner, the
// build-cache record, the raw output, the markers that end or refuse the
// round before its CLI, the effective-envelope markers, and the tamper hook.
func prepareFakeRound(t *Turn) (Launch, error) {
	d := t.Deps()
	interval, err := fakeInterval(d, "METASYSTEM_HEARTBEAT_INTERVAL_MS", 200)
	if err != nil {
		return Launch{}, err
	}
	r := &fakeRound{t: t, d: d, heartbeat: filepath.Join(d.agents(), "hb", t.Job), interval: interval,
		handshaken: make(chan struct{})}
	// The fixture beds read the fake's own banner; the shared log stays
	// open for appending, so rewriting the file keeps every later line.
	if err := os.WriteFile(filepath.Join(d.jobs(), t.Job+".log"), []byte(fmt.Sprintf("fake supervisor started value=%s\n", t.Tag)), 0o644); err != nil {
		return Launch{}, err
	}
	fakeRecordBuildCachePath(d.git(), d.agents(), t.Workspace, t.Dir)
	if err := os.WriteFile(r.path("raw.out"), []byte("fake raw output\n"), 0o644); err != nil {
		return Launch{}, err
	}
	if r.present("pending-process-loss") {
		if err := r.writeLost(); err != nil {
			return Launch{}, err
		}
		r.killSelf()
	}
	if r.present("handshake-failure") {
		return Launch{EarlyRefusal: &Refusal{Error: "authentication_failed", Phase: "handshake"}}, nil
	}
	if r.present("effective-wider") {
		if err := adapter.SetEffectiveNetwork(t.Effective, "allow"); err != nil {
			return Launch{}, err
		}
	}
	if r.present("effective-unreadable") {
		// An envelope nobody can compare: the shared layer must refuse it.
		if err := os.WriteFile(t.Effective, []byte("[]\n"), 0o644); err != nil {
			return Launch{}, err
		}
	}
	if r.present("effective-narrower") {
		if err := adapter.SetEffectiveNetwork(t.Effective, "deny"); err != nil {
			return Launch{}, err
		}
	}
	if d.Getenv("METASYSTEM_FAKE_TAMPER_REFERENCE") != "" {
		staged := r.path("staged", "task-direction.md")
		if info, err := os.Stat(staged); err == nil && info.Mode().IsRegular() {
			if err := appendText(staged, "tampered\n"); err != nil {
				return Launch{}, err
			}
		}
	}
	return Launch{Simulated: r.run, Private: r, OnHandshake: r.handshakeRecorded}, nil
}

func (r *fakeRound) path(names ...string) string {
	return filepath.Join(append([]string{r.t.Dir}, names...)...)
}

// FakeBehaviorFile is the installation-relative file a fixture writes FAKE
// markers to when it drives rounds whose prompts it does not author (a
// public verb composing the brief): every fake round of the installation
// reads it beside its prompt while it exists.
const FakeBehaviorFile = "artifacts/agents/fake-behavior.md"

// behaviorSources are the files markers are read from: the round's prompt,
// the installation's fixture behavior file when present, and, when a body
// over the directive limit was staged, its staged task direction (the
// prompt then carries only the reference stanza).
func (r *fakeRound) behaviorSources() []string {
	sources := []string{r.t.Prompt}
	for _, extra := range []string{filepath.Join(r.t.Root, FakeBehaviorFile), r.path("staged", "task-direction.md")} {
		if info, err := os.Stat(extra); err == nil && info.Mode().IsRegular() {
			sources = append(sources, extra)
		}
	}
	return sources
}

// present is `grep -Fqi FAKE:<name>` over the sources: a case-insensitive
// substring anywhere in either file.
func (r *fakeRound) present(name string) bool {
	needle := strings.ToLower("FAKE:" + name)
	for _, source := range r.behaviorSources() {
		data, err := os.ReadFile(source)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(data)), needle) {
			return true
		}
	}
	return false
}

// value is the value of a `FAKE:<name>=value` line: per source, the first
// line (leading whitespace allowed, case-sensitive) carrying the marker; the
// first source whose first such line has a non-empty value wins.
func (r *fakeRound) value(name string) string {
	prefix := "FAKE:" + name + "="
	for _, source := range r.behaviorSources() {
		data, err := os.ReadFile(source)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if value, found := strings.CutPrefix(strings.TrimLeft(line, shellSpace), prefix); found {
				if value != "" {
					return value
				}
				break
			}
		}
	}
	return ""
}

func (r *fakeRound) logf(format string, args ...any) {
	fmt.Fprintf(r.t.Log, format, args...)
}

func (r *fakeRound) writeLost() error {
	return os.WriteFile(r.heartbeat, []byte("{\"lost\":true}\n"), 0o644)
}

// killSelf is `kill -KILL $$`: the supervisor dies without its terminal
// compare-and-swap. Delivery to a multithreaded process is asynchronous, so
// it waits for the signal rather than racing it to a plain exit.
func (r *fakeRound) killSelf() {
	if err := syscall.Kill(r.d.Pid, syscall.SIGKILL); err != nil {
		fmt.Fprintln(r.d.Stderr, err)
		return
	}
	for {
		time.Sleep(time.Second)
	}
}

// handshakeRecorded is the shared layer's word that the handshake landed.
func (r *fakeRound) handshakeRecorded() { r.once.Do(func() { close(r.handshaken) }) }

// observe answers the handshake once the simulated CLI reached it: the
// session the script handed the lifecycle's handshake, which refuses an
// empty one (missing-session-id) by name.
func (r *fakeRound) observe(t *Turn) Events {
	r.mu.Lock()
	ready, session := r.ready, r.session
	r.mu.Unlock()
	if !ready {
		return Events{}
	}
	if session == "" {
		return Events{Refusal: &Refusal{Error: "handshake_missing_session_id", Phase: "handshake"}}
	}
	events := Events{Session: session, Turn: "fake-turn-" + t.Round, Model: t.Model}
	if !r.present("no-event-stream") {
		events.Lines = []string{fmt.Sprintf(`{"event":"session-established","sessionId":"%s","round":%s}`, session, t.Round)}
	}
	return events
}

// finalize answers the round: the fixed fake usage always (the script's
// every terminal patch carried it), then a failure, a refusal, or the
// return candidate.
func (r *fakeRound) finalize(in FinalInput) (Final, error) {
	if err := adapter.WriteFakeUsage(in.Usage); err != nil {
		return Final{}, err
	}
	if r.err != nil {
		return Final{}, r.err
	}
	if r.refusal != nil {
		return Final{Refusal: r.refusal}, nil
	}
	return Final{Candidate: r.path("return.json")}, nil
}

func (r *fakeRound) fail(err error) int {
	r.err = err
	return 1
}

func (r *fakeRound) refuse(failure, phase string) int {
	r.refusal = &Refusal{Error: failure, Phase: phase}
	return 1
}

// stopped is the simulated CLI's status when the shared layer stops it.
const stopped = 128 + int(syscall.SIGTERM)

// run is the simulated CLI.
func (r *fakeRound) run(stop <-chan struct{}) int {
	if r.present("no-session-signal") {
		r.logf("ordinary output without a session-established event\n")
		hold, err := startFakeHold(r.d, r.t.Tag)
		if err != nil {
			return r.fail(err)
		}
		select {
		case <-hold.done:
		case <-stop:
			stopHold(hold)
			return stopped
		}
	}
	session := "fake-session-" + r.t.RootJob
	if round, err := strconv.Atoi(r.t.Round); err == nil && r.t.Verb == "dispatch" && round > 1 {
		session = "fake-session-" + r.t.RootJob + "-fresh-" + r.t.Round
	}
	if r.t.Verb == "follow-up" {
		session = r.t.ResumeSession
		if session == "" {
			session = "null"
		}
	}
	if r.present("missing-session-id") {
		session = ""
	}
	r.mu.Lock()
	r.session, r.ready = session, true
	r.mu.Unlock()
	select {
	case <-r.handshaken:
	case <-stop:
		return stopped
	}
	return r.afterHandshake(stop)
}

// afterHandshake is the script's sequence after its handshake callback.
func (r *fakeRound) afterHandshake(stop <-chan struct{}) int {
	if r.t.Verb == "follow-up" && r.sessionOf() != "fake-session-"+r.t.RootJob {
		return r.refuse("resume_collision", "resume")
	}
	if r.present("resume-collision") {
		return r.refuse("resume_collision", "resume")
	}
	if release := r.value("custodial-critique"); release != "" {
		if status, done := r.custodialCritique(release, stop); done {
			return status
		}
	}
	if r.present("process-loss") {
		if _, err := r.startRecordedHold(); err != nil {
			return r.fail(err)
		}
		if err := r.writeLost(); err != nil {
			return r.fail(err)
		}
		r.killSelf()
	}
	if r.present("return-then-process-loss") {
		// The recollection scenario: the delivered return lands on disk,
		// then the supervisor dies before its terminal compare-and-swap.
		if err := r.writeValidReturn(); err != nil {
			return r.fail(err)
		}
		if err := r.writeLost(); err != nil {
			return r.fail(err)
		}
		r.killSelf()
	}
	// worktree-file=<path relative to the workspace>: the round writes that
	// file through the guarded write before anything else happens to it, so
	// a round cut off at its cap leaves work in its worktree for the
	// continuation fixtures to find. Round 1 only, so a continuation that
	// inherits the brief through the prior-brief slot cannot recreate what
	// its predecessor wrote; the path is a clean relative path inside the
	// workspace.
	if file := r.value("worktree-file"); file != "" && r.t.Round == "1" {
		if status, done := r.worktreeFile(file); done {
			return status
		}
	}
	// cap-hold-round=<n>: hold (as timeout does) in that round only, so a
	// continuation round that inherits the brief completes instead of
	// holding again.
	holdRound := r.value("cap-hold-round")
	if r.present("timeout") || r.present("concurrent-turn") || (holdRound != "" && holdRound == r.t.Round) {
		if _, err := r.startRecordedHold(); err != nil {
			return r.fail(err)
		}
		return r.holdUntil(stop, nil)
	}
	if r.present("cancel-race") {
		// SIGTERM completes the round valid: the cancel races a completion
		// the record owner must arbitrate.
		terms := make(chan os.Signal, 1)
		signal.Notify(terms, syscall.SIGTERM)
		defer signal.Stop(terms)
		if _, err := r.startRecordedHold(); err != nil {
			return r.fail(err)
		}
		return r.holdUntil(stop, terms)
	}
	if r.present("malformed-return") {
		if err := os.WriteFile(r.path("return.json"), []byte("{malformed\n"), 0o644); err != nil {
			return r.fail(err)
		}
		r.logf("malformed return\n")
		return 0
	}
	if r.present("interrupted-atomic-write") {
		if err := os.WriteFile(filepath.Join(r.d.agents(), "record-locks", r.t.Job+".interrupted"), []byte(`{"status":"corrupt`), 0o644); err != nil {
			return r.fail(err)
		}
	}
	if r.present("nested-agent-events") {
		if err := appendText(r.t.Events, "{\"event\":\"agent.completed\",\"agent\":\"nested\",\"topLevel\":false}\n"+
			"{\"event\":\"turn.completed\",\"agent\":\"root\",\"topLevel\":true}\n"); err != nil {
			return r.fail(err)
		}
	} else if !r.present("no-event-stream") {
		if err := appendText(r.t.Events, "{\"event\":\"turn.completed\",\"topLevel\":true}\n"); err != nil {
			return r.fail(err)
		}
	}
	if r.present("hook-unavailable") {
		r.logf("hooks unavailable; polling fallback used\n")
	}
	if r.present("mirror-failure") {
		if err := touchStrict(filepath.Join(r.d.agents(), r.t.RootJob, ".mirror-fail-once")); err != nil {
			return r.fail(err)
		}
	}
	if argument := fakeArgument(r.t.Prompt); argument != "" {
		if err := appendText(r.path("raw.out"), fmt.Sprintf("provider argument value=%s\n", argument)); err != nil {
			return r.fail(err)
		}
	}
	if err := r.writeValidReturn(); err != nil {
		return r.fail(err)
	}
	return 0
}

func (r *fakeRound) sessionOf() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.session
}

// writeValidReturn writes the canned, schema-valid return.
func (r *fakeRound) writeValidReturn() error {
	ports, err := delegate.PortsFor("fake")
	if err != nil {
		return err
	}
	if ports.Return == nil {
		return errors.New("fake delegate return port is not registered")
	}
	if err := ports.Return(r.t.Record, r.t.Prompt, r.path("return.json")); err != nil {
		return err
	}
	return os.WriteFile(r.path("return.md"), []byte("# Fake return\n\nCanonical JSON: return.json\n"), 0o644)
}

// startRecordedHold starts a hold with the round's stopped file and records
// its pid in child.pid.
func (r *fakeRound) startRecordedHold() (*child, error) {
	hold, err := startFakeHold(r.d, r.t.Tag, "--stopped-file", r.path("child.stopped"))
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(r.path("child.pid"), []byte(strconv.Itoa(hold.pid)+"\n"), 0o644); err != nil {
		return nil, err
	}
	return hold, nil
}

// holdUntil keeps the liveness sidecar fresh until the shared layer stops
// the simulated CLI (the hold itself is the kill domain's to sweep) or, for
// cancel-race, a SIGTERM completes the round valid.
func (r *fakeRound) holdUntil(stop <-chan struct{}, terms <-chan os.Signal) int {
	for {
		touch(r.heartbeat)
		select {
		case <-terms:
			if err := r.writeValidReturn(); err != nil {
				return r.fail(err)
			}
			return 0
		case <-stop:
			return stopped
		default:
		}
		r.d.Clock.Sleep(r.interval)
	}
}

// custodialCritique holds one registered child until the fixture creates
// the release file, bounded by METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC. done is
// true when the round ends here.
func (r *fakeRound) custodialCritique(release string, stop <-chan struct{}) (status int, done bool) {
	if !strings.HasPrefix(release, "/") {
		return r.refuse("invalid_fixture_control", "execute"), true
	}
	rawCap := r.d.Getenv("METASYSTEM_FAKE_CRITIQUE_HOLD_CAP_SEC")
	if rawCap == "" {
		rawCap = "30"
	}
	capSeconds, err := strconv.ParseInt(rawCap, 10, 64)
	if !positiveIntegerRE.MatchString(rawCap) || err != nil {
		return r.refuse("invalid_fixture_control", "execute"), true
	}
	hold, err := startFakeHold(r.d, r.t.Tag)
	if err != nil {
		return r.fail(err), true
	}
	// The fake registers its held child itself: the shared layer never
	// registers custody for a simulated CLI.
	if r.d.Dispatch.Run(r.d.Stdout, r.d.Stderr, "__register-custody", "--job", r.t.Job, "--pid", strconv.Itoa(hold.pid)) != 0 {
		stopHold(hold)
		return r.refuse("custody_registration", "execute"), true
	}
	if err := os.WriteFile(r.path("custody-child.pid"), []byte(strconv.Itoa(hold.pid)+"\n"), 0o644); err != nil {
		return r.fail(err), true
	}
	deadline := r.d.Clock.Now().Unix() + capSeconds
	for {
		if _, err := os.Stat(release); err == nil {
			break
		}
		if r.d.Clock.Now().Unix() >= deadline {
			stopHold(hold)
			return r.refuse("fixture_release_timeout", "execute"), true
		}
		select {
		case <-stop:
			stopHold(hold)
			return stopped, true
		default:
		}
		touch(r.heartbeat)
		r.d.Clock.Sleep(r.interval)
	}
	stopHold(hold)
	return 0, false
}

// worktreeFile is the round-1 guarded write into the workspace.
func (r *fakeRound) worktreeFile(file string) (status int, done bool) {
	wrapped := "/" + file + "/"
	if strings.HasPrefix(wrapped, "//") || strings.Contains(wrapped, "/../") || strings.Contains(wrapped, "/./") {
		return r.refuse("worktree_file_path", "execute"), true
	}
	target := r.t.Workspace + "/" + file
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return r.refuse("worktree_write_refused", "execute"), true
	}
	allowed, err := adapter.FakeGuardedWrite(r.t.Effective, target)
	if err != nil {
		fmt.Fprintln(r.d.Stderr, err)
	}
	if err != nil || !allowed {
		return r.refuse("worktree_write_refused", "execute"), true
	}
	return 0, false
}
