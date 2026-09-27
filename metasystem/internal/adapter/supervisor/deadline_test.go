package supervisor

// The custodian's own-deadline enforcement (F4/D32), driven against the Go
// lifecycle with a recording dispatcher, a controllable clock and real child
// processes in a real kill domain. Ported from the retired
// scripts/agents/adapter-deadline-fixtures.sh (ADPT-DL-001..005).

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// deadlineClock is a controllable clock: Now reads a set instant, Sleep
// advances it and runs the optional hook (the point at which a test acts
// between ticks). No test waits on wall time.
type deadlineClock struct {
	mu      sync.Mutex
	now     time.Time
	onSleep func(d time.Duration)
	// limit, when set, is the latest instant a test's wait may reach; past
	// it, onLimit ends the wait (a supervisor that never enforces fails
	// fast in fake time instead of waiting out its real child).
	limit   time.Time
	onLimit func()
}

func (c *deadlineClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *deadlineClock) Sleep(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	hook := c.onSleep
	overdue := !c.limit.IsZero() && c.now.After(c.limit)
	limitHook := c.onLimit
	c.mu.Unlock()
	if overdue && limitHook != nil {
		limitHook()
	}
	if hook != nil {
		hook(d)
	}
}

// recordingDispatcher records every dispatch.sh callback's argv and answers
// with a chosen status (3 is a lost compare-and-swap).
type recordingDispatcher struct {
	mu     sync.Mutex
	calls  [][]string
	status int
}

func (d *recordingDispatcher) Run(_, _ io.Writer, args ...string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.calls = append(d.calls, append([]string(nil), args...))
	return d.status
}

func (d *recordingDispatcher) snapshot() [][]string {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][]string(nil), d.calls...)
}

// killDomain is the production topology: a process-group leader standing in
// for the supervisor, so the sweep enumerates a real group that is not the
// test binary's own.
type killDomain struct {
	leader *exec.Cmd
}

func newKillDomain(t *testing.T) *killDomain {
	t.Helper()
	leader := exec.Command("sleep", "300")
	leader.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := leader.Start(); err != nil {
		t.Fatalf("start kill-domain leader: %v", err)
	}
	t.Cleanup(func() {
		_ = leader.Process.Kill()
		_ = leader.Wait()
	})
	return &killDomain{leader: leader}
}

func (k *killDomain) pgid() int { return k.leader.Process.Pid }

// startCLI starts the runtime CLI stand-in inside the kill domain.
func (k *killDomain) startCLI(t *testing.T) *child {
	t.Helper()
	command := exec.Command("sleep", "300")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Pgid: k.pgid()}
	c, err := startChild(command)
	if err != nil {
		t.Fatalf("start cli child: %v", err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(c.pid, syscall.SIGKILL)
		<-c.done
	})
	return c
}

// bound ends the wait on cli once fake time passes an hour: the deadlines
// under test are long expired by then, so reaching it is a failure.
func (f *deadlineFixture) bound(t *testing.T, cli *child) {
	f.clock.mu.Lock()
	defer f.clock.mu.Unlock()
	f.clock.limit = f.clock.now.Add(time.Hour)
	f.clock.onLimit = func() {
		t.Error("an hour of fake time passed without the expected verdict")
		_ = syscall.Kill(cli.pid, syscall.SIGKILL)
		<-cli.done
	}
}

// deadlineFixture is one supervision with its record, log and seams.
type deadlineFixture struct {
	s        *Supervision
	dispatch *recordingDispatcher
	clock    *deadlineClock
	logPath  string
}

// phantomPid is a pid no process can hold (above every platform's pid
// ceiling), so signalling it never reaches a real process.
const phantomPid = 2_000_000_000

func newDeadlineFixture(t *testing.T, recordJSON string, handshakeDone bool, domain *killDomain, members func(int, ...int) ([]int, error)) *deadlineFixture {
	t.Helper()
	dir := t.TempDir()
	record := filepath.Join(dir, "job.json")
	if err := os.WriteFile(record, []byte(recordJSON+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(dir, "job.log")
	log, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { log.Close() })
	clock := &deadlineClock{now: time.Date(2026, 9, 27, 12, 0, 0, 0, time.UTC)}
	dispatch := &recordingDispatcher{}
	if members == nil {
		members = groupMembers
	}
	d := Deps{
		Root:         dir,
		Getenv:       func(name string) string { return map[string]string{"METASYSTEM_HEARTBEAT_INTERVAL_MS": "50"}[name] },
		Pid:          domain.pgid(),
		Stdout:       io.Discard,
		Stderr:       io.Discard,
		Clock:        clock,
		Dispatch:     dispatch,
		GroupMembers: members,
	}
	s := &Supervision{
		d: d, runtime: "fake", verb: "dispatch", job: "f4fix",
		record: record, roundDir: dir, logPath: logPath, log: log,
		heartbeat: filepath.Join(dir, "hb"), requestedModel: "m",
		effective: filepath.Join(dir, "eff.json"), handshakeDone: handshakeDone,
	}
	return &deadlineFixture{s: s, dispatch: dispatch, clock: clock, logPath: logPath}
}

func (f *deadlineFixture) logText(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(f.logPath)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func casCalls(calls [][]string, expect, status string) int {
	count := 0
	for _, call := range calls {
		joined := strings.Join(call, " ")
		if strings.HasPrefix(joined, "__record-cas --job f4fix --expect "+expect+" --status "+status+" --patch ") {
			count++
		}
	}
	return count
}

func requireEndedTurn(t *testing.T, err error) {
	t.Helper()
	var request exit
	if !errors.As(err, &request) || request.code != 0 {
		t.Fatalf("enforcement must end the supervisor's turn with status 0; got %v", err)
	}
}

// ADPT-DL-001: an expired cap kills the child and lands running->timeout
// exactly once, and the supervisor's turn ends there.
func TestExpiredCapKillsChildAndLandsTimeoutOnce(t *testing.T) {
	t.Parallel()
	domain := newKillDomain(t)
	f := newDeadlineFixture(t, `{"jobId":"f4fix","status":"running","capDeadline":"2020-01-01T00:00:00Z"}`, true, domain, nil)
	cli := domain.startCLI(t)
	f.bound(t, cli)

	_, err := f.s.waitForCLI(cli)
	requireEndedTurn(t, err)
	if cli.alive() {
		t.Fatal("the child survived cap enforcement")
	}
	calls := f.dispatch.snapshot()
	if len(calls) != 1 || casCalls(calls, "running", "timeout") != 1 {
		t.Fatalf("expected exactly one running->timeout CAS: %q", calls)
	}
	patch, _ := os.ReadFile(filepath.Join(f.s.roundDir, "terminal-patch.json"))
	if !strings.Contains(string(patch), `"budget-cap"`) || !strings.Contains(string(patch), `"supervision"`) {
		t.Fatalf("terminal patch is not the reaper's budget-cap/supervision spelling: %s", patch)
	}
	if !strings.Contains(string(patch), `"groupDeathProvenAt"`) {
		t.Fatalf("the custodian's cap patch carries no death proof: %s", patch)
	}
	if !strings.Contains(f.logText(t), "cap deadline enforced by the custodian") {
		t.Fatalf("enforcement did not say itself in the job log: %s", f.logText(t))
	}
	if members, err := groupMembers(domain.pgid(), domain.pgid()); err != nil || len(members) != 0 {
		t.Fatalf("the kill domain was not proven empty: %v %v", members, err)
	}
}

// ADPT-DL-002: an expired handshake deadline (no session ever recorded)
// lands pending->failed through the handshake_timeout path.
func TestExpiredHandshakeLandsPendingFailed(t *testing.T) {
	t.Parallel()
	domain := newKillDomain(t)
	f := newDeadlineFixture(t, `{"jobId":"f4fix","status":"pending","handshakeDeadline":5}`, false, domain, nil)
	cli := domain.startCLI(t)
	f.bound(t, cli)

	_, err := f.s.waitForCLI(cli)
	requireEndedTurn(t, err)
	if cli.alive() {
		t.Fatal("the child survived handshake enforcement")
	}
	calls := f.dispatch.snapshot()
	if len(calls) != 1 || casCalls(calls, "pending", "failed") != 1 {
		t.Fatalf("expected exactly one pending->failed CAS: %q", calls)
	}
	patch, _ := os.ReadFile(filepath.Join(f.s.roundDir, "pending-failure.json"))
	if !strings.Contains(string(patch), `"handshake_timeout"`) {
		t.Fatalf("pending failure patch does not carry handshake_timeout: %s", patch)
	}
}

// ADPT-DL-003: a won handshake stands down BEFORE any signal: zero CAS, zero
// signals, the wait continues undisturbed and returns normally once the child
// ends.
func TestWonHandshakeStandsDownBeforeAnySignal(t *testing.T) {
	t.Parallel()
	domain := newKillDomain(t)
	f := newDeadlineFixture(t, `{"jobId":"f4fix","status":"running","handshakeDeadline":5}`, true, domain, nil)
	cli := domain.startCLI(t)
	f.bound(t, cli)

	checked := false
	f.clock.onSleep = func(time.Duration) {
		// The first sleep follows the first completed deadline check.
		if checked {
			return
		}
		checked = true
		if !cli.alive() {
			t.Error("the child should still be running under a won handshake")
		}
		if calls := f.dispatch.snapshot(); len(calls) != 0 {
			t.Errorf("a won handshake attempted a CAS: %q", calls)
		}
		_ = syscall.Kill(cli.pid, syscall.SIGTERM)
		<-cli.done
	}
	status, err := f.s.waitForCLI(cli)
	if err != nil {
		t.Fatalf("the wait did not return normally after the child ended: %v", err)
	}
	if !checked {
		t.Fatal("the deadline monitor never completed a check")
	}
	if status != 128+int(syscall.SIGTERM) {
		t.Fatalf("the wait returned status %d, want the child's own TERM status", status)
	}
	if calls := f.dispatch.snapshot(); len(calls) != 0 {
		t.Fatalf("a won handshake attempted a CAS: %q", calls)
	}
}

// ADPT-DL-004: when the waiter's verdict already landed, the supervisor's CAS
// loses (status 3) and the turn still settles with exactly one attempt.
func TestLostCASSettlesWithOneAttempt(t *testing.T) {
	t.Parallel()
	domain := newKillDomain(t)
	f := newDeadlineFixture(t, `{"jobId":"f4fix","status":"running","capDeadline":"2020-01-01T00:00:00Z"}`, true, domain, nil)
	f.dispatch.status = 3
	cli := domain.startCLI(t)
	f.bound(t, cli)

	_, err := f.s.waitForCLI(cli)
	requireEndedTurn(t, err)
	if cli.alive() {
		t.Fatal("the child survived")
	}
	if calls := f.dispatch.snapshot(); len(calls) != 1 {
		t.Fatalf("expected exactly one CAS attempt: %q", calls)
	}
}

// ADPT-DL-005: a kill domain that cannot be proven dead (a phantom member
// survives every sweep) leaves the record NONTERMINAL: no CAS, the decline
// said in the log, the supervisor not ended.
func TestUnprovenKillDomainLeavesRecordNonterminal(t *testing.T) {
	t.Parallel()
	domain := newKillDomain(t)
	phantom := func(int, ...int) ([]int, error) { return []int{phantomPid}, nil }
	f := newDeadlineFixture(t, `{"jobId":"f4fix","status":"running","capDeadline":"2020-01-01T00:00:00Z"}`, true, domain, phantom)
	cli := domain.startCLI(t)
	f.bound(t, cli)

	if err := f.s.checkRecordDeadlines(cli); err != nil {
		t.Fatalf("an unproven domain must not end the supervisor: %v", err)
	}
	if !strings.Contains(f.logText(t), "cap-deadline sweep left the kill domain unproven; record stays nonterminal") {
		t.Fatalf("the unproven domain was not said in the log: %s", f.logText(t))
	}
	if calls := f.dispatch.snapshot(); len(calls) != 0 {
		t.Fatalf("an unproven domain still landed a CAS: %q", calls)
	}
	if cli.alive() {
		t.Fatal("the direct child must be stopped before the sweep")
	}
	// The next tick retries, and declines again while the phantom stands.
	if err := f.s.checkRecordDeadlines(cli); err != nil {
		t.Fatalf("the retry tick ended the supervisor: %v", err)
	}
	if got := strings.Count(f.logText(t), "left the kill domain unproven"); got != 2 {
		t.Fatalf("expected the decline on each tick, got %d", got)
	}
	if calls := f.dispatch.snapshot(); len(calls) != 0 {
		t.Fatalf("the retry tick landed a CAS: %q", calls)
	}
}
