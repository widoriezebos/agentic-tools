package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// Stop deadline parent tests. The worker is a real child process (this test
// binary in worker mode) so the typed wait and cleanup prove and signal its
// exact identity; the deadline itself is the fixture's event, never elapsed
// time. Ported from the supervision-hook bed's deadline, worker-interruption
// and slow-resolution legs and the nested bed's worktree-deadline rows.

type deadlineCase struct {
	worker   map[string]string
	payload  string
	env      map[string]string
	now      func() time.Time
	mono     func() time.Duration
	deadline bool // fire the fixture deadline once the worker entered and the resolver answered
	// unresolved: the fixture resolver never answers, so the deadline fires
	// on the worker's entry alone.
	unresolved bool
	// resolverAnswered observes the coordinate resolver's answer.
	resolverAnswered func()
	after            func(time.Duration) <-chan time.Time
	hang             bool // the worker waits for a release that never comes
	tempDir          string
	// afterWorker observes the launched worker.
	afterWorker func(Worker)
}

func runDeadline(t *testing.T, installation hookInstallation, ops *fakeOps, test deadlineCase) (hookRun, []string) {
	t.Helper()
	worker := map[string]string{}
	if test.deadline || test.hang {
		worker["METASYSTEM_HOOK_TEST_WORKER_RELEASE"] = makeFIFO(t, "never-released")
	}
	for key, value := range test.worker {
		worker[key] = value
	}
	launched := make(chan []string, 1)
	fixture := make(chan time.Time, 1)
	resolverAnswered := test.resolverAnswered
	// The parent adopts the record coordinates only if the resolver has
	// answered when the deadline fires, and it rightly never waits past a
	// spent budget; so the fixture deadline waits for the answer, or a
	// loaded scheduler drops the checkout.
	answered := make(chan struct{})
	if test.deadline && !test.unresolved {
		observe := resolverAnswered
		resolverAnswered = func() {
			if observe != nil {
				observe()
			}
			close(answered)
		}
	} else {
		close(answered)
	}
	if test.deadline {
		// The deadline fires once the worker has entered: the entered FIFO
		// carries its pid. A worker that never enters leaves the reader to
		// the cleanup, which writes without blocking so the reader returns.
		entered := makeFIFO(t, "worker.entered")
		worker["METASYSTEM_HOOK_TEST_WORKER_ENTERED"] = entered
		go func() {
			_ = awaitFIFO(entered)
			<-answered
			fixture <- time.Unix(0, 0)
		}()
		t.Cleanup(func() { unblockFIFO(entered) })
	}
	payload := test.payload
	if payload == "" {
		payload = `{"session_id":"line-fixture","hook_event_name":"Stop"}`
	}
	env := test.env
	if env == nil {
		env = map[string]string{}
	}
	call := hookCall{
		runtime: "claude", event: "stop", payload: payload, env: env, now: test.now, monotonic: test.mono,
		startWorker: startTestWorker(t, worker, launched), tempDir: test.tempDir, after: test.after,
		resolverAnswered: resolverAnswered,
	}
	if test.afterWorker != nil {
		start := call.startWorker
		call.startWorker = func(script, runtime string, env []string, stdin, stdout, stderr *os.File) (Worker, error) {
			worker, err := start(script, runtime, env, stdin, stdout, stderr)
			if err == nil {
				test.afterWorker(worker)
			}
			return worker, err
		}
	}
	if test.deadline {
		call.fixtureDeadline = fixture
	}
	run := runHook(t, installation, ops, call)
	var argv []string
	select {
	case argv = <-launched:
	default:
	}
	return run, argv
}

func TestDeadlineParentPublishesAValidWorkerResponse(t *testing.T) {
	t.Parallel()
	for _, output := range []string{
		`{"systemMessage":"Just completed: fixture"}`,
		`{"decision":"block","reason":"fixture block"}`,
		`{"decision":"block","reason":"fixture block","systemMessage":"carried beside"}`,
	} {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		run, argv := runDeadline(t, installation, ops, deadlineCase{worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": output,
			"METASYSTEM_HOOK_TEST_WORKER_STDERR": "worker diagnostic\n"}})
		if run.status != 0 || run.stdout != output+"\n" || run.stderr != "worker diagnostic\n" {
			t.Fatalf("valid output %q = status %d stdout %q stderr %q", output, run.status, run.stdout, run.stderr)
		}
		if len(argv) < 2 || argv[0] != installation.root || argv[1] != "claude" ||
			!containsEnv(argv[2:], stopDeadlineParentEnv+"="+strconv.Itoa(os.Getpid())) {
			t.Fatalf("worker launch = %q", argv)
		}
		if strings.Contains(readHookLog(t, installation), "stop-condition") {
			t.Fatalf("a valid response logged a condition: %q", readHookLog(t, installation))
		}
	}
}

// The worker's intentional skip (a delegate, another runtime's session) is a
// silent Stop, even after a delay.
func TestDeadlineParentPassesTheSkipQuietly(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	release := makeFIFO(t, "release")
	releaseFIFO(t, release)
	run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{worker: map[string]string{
		"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": internalSkipResult, "METASYSTEM_HOOK_TEST_WORKER_RELEASE": release}})
	if run.status != 0 || run.stdout != "" {
		t.Fatalf("skip = status %d stdout %q", run.status, run.stdout)
	}
}

// A killed worker, a failed one, and any output outside the two provider
// shapes are answered with the fixed unreadable-output allowance and one
// condition line under the resolved installation.
func TestDeadlineParentRejectsUnreadableOutput(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		worker map[string]string
	}{
		{"killed", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_STATUS": "137"}},
		{"failed", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"systemMessage":"x"}`, "METASYSTEM_HOOK_TEST_WORKER_STATUS": "1"}},
		{"partial", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"systemMessage":`}},
		{"unknown key", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"systemMessage":"x","extra":1}`}},
		{"reasonless block", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"decision":"block"}`}},
		{"empty message", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"systemMessage":""}`}},
		{"numeric message", map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": `{"systemMessage":7}`}},
		{"silent", map[string]string{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			installation := newHookInstallation(t)
			run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{worker: test.worker})
			if run.status != 0 || run.stdout != mustForm(t, "allowed", "unreadable-output")+"\n" {
				t.Fatalf("%s = status %d stdout %q", test.name, run.status, run.stdout)
			}
			log := readHookLog(t, installation)
			if !regexp.MustCompile(`(?m)^stop-condition infrastructure stop-hook-output-was-unreadable stop-worker - [0-9]+ degraded-allow$`).MatchString(log) ||
				!regexp.MustCompile(`(?m) stop response outcome=invalid-worker-output-allow elapsed=[0-9]+s$`).MatchString(log) {
				t.Fatalf("%s hook log = %q", test.name, log)
			}
		})
	}
}

// Pre-verdict work and the verdict share one Stop budget: an expired worker
// is cleaned by its exact identity, the Stop is allowed with the fixed
// notice, the occurrence is recorded and counted, and the launch-to-cleanup
// measurement is retained.
func TestDeadlineExpiryAllowsAndRecords(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	var clock atomic.Int64
	clock.Store(1000)
	now := func() time.Time {
		// The first reading starts the budget; every later one is 17
		// seconds on.
		return time.Unix(clock.Swap(1017), 0)
	}
	for occurrence := 1; occurrence <= 2; occurrence++ {
		clock.Store(1000)
		run, _ := runDeadline(t, installation, ops, deadlineCase{now: now, deadline: true,
			env: map[string]string{stopDeadlineBudgetEnv: "20"}})
		if run.status != 0 || run.stdout != mustForm(t, "allowed", "deadline-expired")+"\n" {
			t.Fatalf("occurrence %d = status %d stdout %q stderr %q", occurrence, run.status, run.stdout, run.stderr)
		}
	}
	log := readHookLog(t, installation)
	if !strings.Contains(log, "stop-condition infrastructure stop-deadline-expired stop-deadline - 1020 degraded-allow\n") ||
		strings.Count(log, " stop response outcome=deadline-expired-allow elapsed=17s\n") != 2 {
		t.Fatalf("hook log = %q", log)
	}
	if !ops.called("steward hook-expire elapsed=17") {
		t.Fatalf("the expired attempt was not closed with its measurement: %s", ops.trace())
	}
	record, err := os.ReadFile(filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals", "line-fixture.json"))
	if err != nil || !strings.Contains(string(record), `"cause": "stop deadline expired"`) || !strings.Contains(string(record), `"count": 2`) {
		t.Fatalf("refusal record = %s (%v)", record, err)
	}
	if blocks := ops.stopBlocks; len(blocks) != 2 || blocks[0].OpenWorkRoot != installation.root || blocks[0].Class != "infrastructure" {
		t.Fatalf("deadline refusals = %+v", blocks)
	}
}

// A worker that published its verdict before the deadline keeps it: a
// complete response is already a safe decision.
func TestDeadlinePublicationRaceKeepsTheVerdict(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	output := `{"decision":"block","reason":"published before the deadline"}`
	run, _ := runDeadline(t, installation, ops, deadlineCase{deadline: true, worker: map[string]string{
		"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": output, "METASYSTEM_HOOK_TEST_WORKER_PUBLISH_FIRST": "1"}})
	if run.status != 0 || run.stdout != output+"\n" || strings.Contains(run.stdout, "stop deadline expired") {
		t.Fatalf("publication race = status %d stdout %q", run.status, run.stdout)
	}
	if len(ops.stopBlocks) != 0 {
		t.Fatalf("a published verdict still recorded a deadline refusal: %+v", ops.stopBlocks)
	}
}

// Failure to record the refusal, or to log the condition, is a qualifier on
// the one fixed allowance, never a block.
func TestDeadlineRecordAndLogFailures(t *testing.T) {
	t.Parallel()
	t.Run("record", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.stopBlock = func(StopBlockRequest) (string, int) { return "", 3 }
		run, _ := runDeadline(t, installation, ops, deadlineCase{deadline: true})
		if run.stdout != mustForm(t, "allowed", "deadline-expired", "record-update-failed")+"\n" ||
			!strings.Contains(readHookLog(t, installation), "stop response outcome=deadline-expired-record-failure-allow") {
			t.Fatalf("record failure = stdout %q log %q", run.stdout, readHookLog(t, installation))
		}
		if _, err := os.Stat(filepath.Join(installation.root, "artifacts", "agents", "supervision", "stop-refusals")); !os.IsNotExist(err) {
			t.Fatalf("a failed record wrote its refusal: %v", err)
		}
	})
	t.Run("condition log", func(t *testing.T) {
		installation := newHookInstallation(t)
		logPath := filepath.Join(installation.root, "artifacts", "agents", "supervision", "hooks.log")
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(logPath, nil, 0o444); err != nil {
			t.Fatal(err)
		}
		run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{deadline: true})
		if run.stdout != mustForm(t, "allowed", "deadline-expired", "condition-log-failed")+"\n" {
			t.Fatalf("log failure = stdout %q", run.stdout)
		}
	})
}

// With the installation's engine gone the parent accepts exactly the
// worker's fixed missing-engine allowance and logs nothing.
func TestDeadlineMissingEngineAcceptsTheWorkersAllowance(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	if err := os.Remove(filepath.Join(installation.root, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	form := mustForm(t, "allowed", "engine-missing")
	run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_OUTPUT": form}})
	if run.status != 0 || run.stdout != form+"\n" || readHookLog(t, installation) != "" {
		t.Fatalf("missing engine = status %d stdout %q log %q", run.status, run.stdout, readHookLog(t, installation))
	}
}

// An installation outside Git never guesses a governed world: the worker's
// silence is answered with the unresolved allowance and nothing is written.
func TestDeadlineWithoutAWorldWritesNothing(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.git = func(...string) (string, error) { return "", fmt.Errorf("not a git repository") }
	run, _ := runDeadline(t, installation, ops, deadlineCase{})
	if run.stdout != mustForm(t, "allowed", "unreadable-output", "condition-log-failed", "no-resolved-checkout")+"\n" {
		t.Fatalf("no world = stdout %q", run.stdout)
	}
	if _, err := os.Stat(filepath.Join(installation.root, "artifacts", "agents", "supervision")); !os.IsNotExist(err) {
		t.Fatalf("an installation outside Git received evidence: %v", err)
	}
}

// The budget accepts four to sixty seconds; anything else is sixty.
func TestDeadlineBudgetValidation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		input string
		want  int64
	}{{"3", 60}, {"61", 60}, {"20", 20}, {"007", 7}, {"", 60}, {"abc", 60}} {
		if got := deadlineBudget(test.input); got != test.want {
			t.Errorf("deadlineBudget(%q) = %d, want %d", test.input, got, test.want)
		}
	}
	installation := newHookInstallation(t)
	now := func() time.Time { return time.Unix(5000, 0) }
	runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{now: now,
		env: map[string]string{stopDeadlineBudgetEnv: "20"}, worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_STATUS": "1"}})
	if !strings.Contains(readHookLog(t, installation), "stop-condition infrastructure stop-hook-output-was-unreadable stop-worker - 5020 degraded-allow") {
		t.Fatalf("condition horizon = %q", readHookLog(t, installation))
	}
}

// Installation lookup is inside the worker budget: setup that consumed the
// worker window leaves no fresh window, so a hung worker expires at once.
func TestDeadlineSetupConsumesTheWorkerWindow(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	var consumed atomic.Bool
	ops.git = func(args ...string) (string, error) {
		consumed.Store(true)
		return ops.ordinaryGit(args...)
	}
	mono := func() time.Duration {
		if consumed.Load() {
			return 2 * time.Second
		}
		return 0
	}
	// The coordinate resolver runs beside the worker. With the window
	// already spent the deadline is due at once, and the parent adopts the
	// coordinates only if the resolver has answered by then; the deadline
	// fires once it has, so a loaded scheduler cannot drop the checkout.
	resolved := make(chan time.Time)
	var windows []time.Duration
	after := func(wait time.Duration) <-chan time.Time {
		windows = append(windows, wait)
		if len(windows) == 1 {
			return resolved
		}
		return nil // the worker's cleanup grace; it ends on SIGTERM
	}
	run, _ := runDeadline(t, installation, ops, deadlineCase{mono: mono, hang: true, after: after,
		resolverAnswered: func() { close(resolved) }, env: map[string]string{stopDeadlineBudgetEnv: "4"}})
	if run.stdout != mustForm(t, "allowed", "deadline-expired")+"\n" {
		t.Fatalf("setup-consumed window = stdout %q stderr %q", run.stdout, run.stderr)
	}
	// Setup consumed the whole worker window: the deadline wait asks for
	// nothing more.
	if len(windows) == 0 || windows[0] != 0 {
		t.Fatalf("a consumed window still waited: deadline waits %v", windows)
	}
}

// A decoded session may carry a newline: the payload's text never becomes
// the state root for the log or the record.
func TestDeadlineSessionTextNeverBecomesTheStateRoot(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	injected := t.TempDir()
	payload := fmt.Sprintf(`{"session_id":"deadline-coordinate\u000a%s","hook_event_name":"Stop"}`, injected)
	run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{payload: payload,
		worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_STATUS": "137"}})
	if run.stdout != mustForm(t, "allowed", "unreadable-output")+"\n" ||
		!strings.Contains(readHookLog(t, installation), "stop response outcome=invalid-worker-output-allow") {
		t.Fatalf("newline session = stdout %q log %q", run.stdout, readHookLog(t, installation))
	}
	if entries, _ := os.ReadDir(injected); len(entries) != 0 {
		t.Fatalf("payload session text became a state root: %v", entries)
	}
}

// When the worker fails before the resolver answers, the parent waits for
// the answer instead of treating it as no checkout; a resolver that never
// answers within the budget leaves the coordinates unresolved.
func TestDeadlineSlowResolution(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	release := make(chan struct{})
	ops.stateRoot = func(root string) (string, int) {
		<-release
		return root + "\n", 0
	}
	// The resolver answers only after the worker has failed and exited, so
	// the parent must wait for it rather than treat it as no checkout.
	run, _ := runDeadline(t, installation, ops, deadlineCase{worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_STATUS": "1"},
		afterWorker: func(worker Worker) { go func() { <-worker.Done(); close(release) }() }})
	if run.stdout != mustForm(t, "allowed", "unreadable-output")+"\n" ||
		!strings.Contains(readHookLog(t, installation), "stop response outcome=invalid-worker-output-allow") {
		t.Fatalf("slow resolution = stdout %q log %q", run.stdout, readHookLog(t, installation))
	}

	unresolved := newHookInstallation(t)
	never := newFakeOps(t, unresolved)
	block := make(chan struct{})
	defer close(block)
	never.stateRoot = func(string) (string, int) { <-block; return "", 1 }
	run, _ = runDeadline(t, unresolved, never, deadlineCase{deadline: true, unresolved: true, env: map[string]string{stopDeadlineBudgetEnv: "4"}})
	if run.stdout != mustForm(t, "allowed", "deadline-expired", "record-update-failed", "condition-log-failed", "no-resolved-checkout")+"\n" {
		t.Fatalf("unresolved deadline = stdout %q", run.stdout)
	}
	if _, err := os.Stat(filepath.Join(unresolved.root, "artifacts", "agents", "supervision")); !os.IsNotExist(err) {
		t.Fatalf("an unresolved deadline guessed a record root: %v", err)
	}
}

// A worker that survives TERM is killed; without a terminal acknowledgement
// the parent keeps custody evidence, says so, and still answers at once.
func TestDeadlineKillWithoutTerminalAcknowledgement(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	temp := t.TempDir()
	run, _ := runDeadline(t, installation, newFakeOps(t, installation), deadlineCase{deadline: true, tempDir: temp,
		worker: map[string]string{"METASYSTEM_HOOK_TEST_WORKER_IGNORE_TERM": "1"}})
	if run.stdout != mustForm(t, "allowed", "deadline-expired")+"\n" {
		t.Fatalf("kill = stdout %q stderr %q", run.stdout, run.stderr)
	}
	pattern := regexp.MustCompile(`^stop deadline: worker [0-9]+ cleanup sent KILL without terminal acknowledgement; custody retained; not waiting\n$`)
	if !pattern.MatchString(run.stderr) {
		t.Fatalf("cleanup diagnostic = %q", run.stderr)
	}
	if retained, _ := filepath.Glob(filepath.Join(temp, "metasystem-stop-deadline.*")); len(retained) != 1 {
		t.Fatalf("custody evidence was not retained: %v", retained)
	}
}

// Cleanup proves the worker's exact identity before it consults parent
// custody or signals: a dead, zombie, replaced or uninspectable worker is
// never signalled, and a live worker under a foreign parent is unverified.
// Ported from the former cmd/metasystem stop-deadline cleanup test.
func TestStopDeadlineCleanupClassifiesIdentityBeforeParentCustody(t *testing.T) {
	t.Parallel()
	exact := identity.Exact{Pid: 42, StartedAt: time.Unix(100, 123000)}
	worker := exact.Ref()
	cleanup := func(probe *stopDeadlineProbe, parentChecked, signalled *bool, owner int64) StopDeadlineCleanupResult {
		parent := &deadlineParent{inv: Invocation{Pid: 7, Deadline: DeadlineDeps{
			Prober: probe,
			ParentPid: func(int64) (int64, bool) {
				*parentChecked = true
				return owner, true
			},
			Signal: func(int, syscall.Signal) error {
				*signalled = true
				return nil
			},
		}}}
		return parent.cleanupWorker(&worker)
	}
	for _, test := range []struct {
		name  string
		probe *stopDeadlineProbe
		want  string
	}{
		{name: "dead", probe: &stopDeadlineProbe{state: identity.Dead}, want: StopWorkerGone},
		{name: "zombie", probe: &stopDeadlineProbe{exact: identity.Exact{Pid: 42, StartedAt: exact.StartedAt, Zombie: true}, state: identity.Alive}, want: StopWorkerGone},
		{name: "replaced", probe: &stopDeadlineProbe{exact: identity.Exact{Pid: 42, StartedAt: exact.StartedAt.Add(time.Second)}, state: identity.Alive}, want: StopWorkerGone},
		{name: "unknown", probe: &stopDeadlineProbe{state: identity.Unknown, err: errors.New("uninspectable")}, want: StopWorkerUnverified},
	} {
		t.Run(test.name, func(t *testing.T) {
			parentChecked, signalled := false, false
			result := cleanup(test.probe, &parentChecked, &signalled, 7)
			if result.State != test.want || parentChecked || signalled {
				t.Fatalf("cleanup result=%+v parentChecked=%v signalled=%v", result, parentChecked, signalled)
			}
		})
	}
	parentChecked, signalled := false, false
	result := cleanup(&stopDeadlineProbe{exact: exact, state: identity.Alive}, &parentChecked, &signalled, 8)
	if result.State != StopWorkerUnverified || !parentChecked || signalled {
		t.Fatalf("foreign-parent cleanup result=%+v parentChecked=%v signalled=%v", result, parentChecked, signalled)
	}
}
