package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/report"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/roots"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The runtime hook's tests run the entry in process against a fake of its
// owner operations, one fresh fake per test. The fake answers as the former
// fixture engines did (a verb's output and status); Git is the fake's own
// answer, never the real tool. Stop deadline tests launch real worker
// processes (this test binary in worker mode), because the deadline owner
// proves and signals exact process identities.

const hookWorkerModeEnv = "METASYSTEM_HOOK_TEST_WORKER"

// runHookTestWorker is the Stop worker a deadline test launches: it prints
// the configured output, optionally after its release FIFO closes, and exits
// with the configured status.
func runHookTestWorker() int {
	if os.Getenv("METASYSTEM_HOOK_TEST_WORKER_IGNORE_TERM") == "1" {
		signal.Ignore(syscall.SIGTERM)
	}
	output := os.Getenv("METASYSTEM_HOOK_TEST_WORKER_OUTPUT")
	release := os.Getenv("METASYSTEM_HOOK_TEST_WORKER_RELEASE")
	entered := os.Getenv("METASYSTEM_HOOK_TEST_WORKER_ENTERED")
	_, _ = io.Copy(io.Discard, os.Stdin)
	if os.Getenv("METASYSTEM_HOOK_TEST_WORKER_PUBLISH_FIRST") == "1" && output != "" {
		fmt.Println(output)
		output = ""
	}
	// Both handshakes are FIFOs carrying one message, so neither side polls
	// a clock (see releaseFIFO): the entered FIFO carries the worker's pid to
	// the test, and the release FIFO blocks until the test's release line
	// arrives (or the worker is signalled, which is what a never-released
	// worker waits for).
	if entered != "" {
		if file, err := os.OpenFile(entered, os.O_WRONLY, 0); err == nil {
			_, _ = fmt.Fprint(file, os.Getpid())
			_ = file.Close()
		}
	}
	if release != "" {
		if file, err := os.Open(release); err == nil {
			_, _ = file.Read(make([]byte, 1))
			_ = file.Close()
		}
	}
	if output != "" {
		fmt.Println(output)
	}
	fmt.Fprint(os.Stderr, os.Getenv("METASYSTEM_HOOK_TEST_WORKER_STDERR"))
	status := 0
	fmt.Sscan(os.Getenv("METASYSTEM_HOOK_TEST_WORKER_STATUS"), &status)
	return status
}

// hookInstallation is one fixture installation: an executable engine
// placeholder and the runtime state directory.
type hookInstallation struct {
	root string
}

func newHookInstallation(t *testing.T) hookInstallation {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return hookInstallationAt(t, root)
}

// hookInstallationAt plants the installation shape at root.
func hookInstallationAt(t *testing.T, root string) hookInstallation {
	t.Helper()
	for _, directory := range []string{"bin", "artifacts/agents"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte("#!/bin/sh\nexit 97\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return hookInstallation{root: root}
}

// fakeOps is one test's owner operations: every call is recorded, and each
// answer is the field's function or the healthy default.
type fakeOps struct {
	t     *testing.T
	root  string
	mu    sync.Mutex
	calls []string

	runtimeNames   func() (string, int)
	stateRoot      func(string) (string, int)
	hookDelegate   func(root, metasystemRoot, job string, callerPid int) (string, int)
	findAncestor   func(repo string, pid int, runtime string, allHosts bool) (string, int)
	classify       func(root, metasystemRoot string, callerPid int) (string, int)
	startContext   func(string) (string, int)
	stewardPending func(string) (string, int)
	brainBoot      func(ctx context.Context) (string, string, int)
	brainDelivered func(sha, cursor, prefix string) int
	up             func(UpRequest, io.Writer, io.Writer) int
	sessionStart   func(io.Writer, io.Writer) int
	sessionEnd     func() int
	hookAttempt    func(pid int) (string, int)
	hookComplete   func(HookCompletion) int
	hookExpire     func(int64) int
	health         func() (string, int)
	digestPending  func() (string, int)
	digestAdvance  func(cursor, prefix string) int
	receipt        func() (string, string, int)
	protocolGrowth func() (string, int)
	protocolAdv    func(counts string) int
	renew          func() int
	watchdog       func() (string, int)
	turnVerdict    func(TurnVerdictRequest) (string, string, int)
	stopBlock      func(StopBlockRequest) (string, int)
	stopInput      func(StopInputRequest, io.Writer) int
	stopPresent    func(string, string, io.Writer) int
	stopOutput     func(string, string, string, io.Writer) int
	evidenceGC     func(io.Writer) int
	git            func(args ...string) (string, error)
	engineBehind   func() (bool, error)
	unmigratable   func() ([]string, error)
	rebuild        func() error
	tokenHex       func() (string, error)
	peerSeat       func(string) (string, int)
	peerClaims     func(string) (string, int)

	identityRuntime string
	identityPid     int
	verdict         string
	completions     []HookCompletion
	stopBlocks      []StopBlockRequest
	upRequests      []UpRequest
	turnVerdicts    []TurnVerdictRequest
	stopInputs      []StopInputRequest
	captured        []capturedPresentation
}

// capturedPresentation is what one presentation composed from: the files
// the worker staged, read while they exist.
type capturedPresentation struct {
	notices, failures, verdict, armingStderr string
}

func newFakeOps(t *testing.T, installation hookInstallation) *fakeOps {
	return &fakeOps{t: t, root: installation.root, identityRuntime: "claude", identityPid: 4321,
		verdict: `{"schemaVersion":1,"class":"seat-actionable","shouldBlock":false,"ledgerStatus":"ok","display":"fixture verdict","surfaceWatchdog":false,"idleRefusal":false,"brainStatusDue":false}`}
}

func (f *fakeOps) record(format string, args ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func (f *fakeOps) trace() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return strings.Join(f.calls, "\n") + "\n"
}

func (f *fakeOps) called(prefix string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, call := range f.calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

func (f *fakeOps) RuntimeNames() (string, int) {
	f.record("runtime list")
	if f.runtimeNames != nil {
		return f.runtimeNames()
	}
	return "claude\ncodex\ndevin\nfake\n", 0
}

func (f *fakeOps) StateRoot(installation roots.Installation) (string, int) {
	f.record("path state-root %s", installation)
	if f.stateRoot != nil {
		return f.stateRoot(installation.Path())
	}
	return installation.Path() + "\n", 0
}

func (f *fakeOps) HookDelegate(root string, metasystemRoot roots.Installation, job string, callerPid int) (string, int) {
	f.record("lease hook-delegate root=%s job=%s caller=%d", root, job, callerPid)
	if f.hookDelegate != nil {
		return f.hookDelegate(root, metasystemRoot.Path(), job, callerPid)
	}
	return "", 3
}

func (f *fakeOps) FindAncestor(installation roots.Installation, pid int, runtime string, allHosts bool) (string, int) {
	f.record("proc find-ancestor pid=%d runtime=%s all-hosts=%t", pid, runtime, allHosts)
	if f.findAncestor != nil {
		return f.findAncestor(installation.Path(), pid, runtime, allHosts)
	}
	return fmt.Sprintf(`{"runtime":%q,"pid":%d,"pidStartedAt":1700000000}`+"\n", f.identityRuntime, f.identityPid), 0
}

func (f *fakeOps) Classify(root string, metasystemRoot roots.Installation, callerPid int) (string, int) {
	f.record("lease classify caller=%d", callerPid)
	if f.classify != nil {
		return f.classify(root, metasystemRoot.Path(), callerPid)
	}
	return `{"class":"MAIN","mainId":"main-fixture","holder":true,"claimEpoch":7,"announcement":{"runtime":"claude","pid":4321,"pidStartedAt":1700000000,"ownerLineage":"fixture-lineage"}}` + "\n", 0
}

func (f *fakeOps) StartContext(runtime string) (string, int) {
	f.record("runtime start-context %s", runtime)
	if f.startContext != nil {
		return f.startContext(runtime)
	}
	if runtime == "claude" {
		return "field=hookSpecificOutput.additionalContext event=SessionStart bytes=10000 sources=startup,resume,clear,compact\n", 0
	}
	return "", 1
}

func (f *fakeOps) StewardPending(repo string) (string, int) {
	f.record("steward pending")
	if f.stewardPending != nil {
		return f.stewardPending(repo)
	}
	return "", 0
}

// PeerSeat answers no enrolled seat unless the test names one.
func (f *fakeOps) PeerSeat(repo string) (string, int) {
	f.record("peer seat")
	if f.peerSeat != nil {
		return f.peerSeat(repo)
	}
	return "", 1
}

// PeerClaims answers an empty ledger unless the test gives claims.
func (f *fakeOps) PeerClaims(repo string) (string, int) {
	f.record("peer claims")
	if f.peerClaims != nil {
		return f.peerClaims(repo)
	}
	return `{"live":{},"concluded":{}}` + "\n", 0
}

func (f *fakeOps) BrainBoot(ctx context.Context, root, repo string, bytes, deadlineMS int) (string, string, int) {
	f.record("brain boot bytes=%d deadline=%d", bytes, deadlineMS)
	if f.brainBoot != nil {
		return f.brainBoot(ctx)
	}
	return `{"declared":false}` + "\n", "", 0
}

func (f *fakeOps) BrainStartDelivered(root, repo, sha, cursor, prefix string) int {
	f.record("brain start-delivered sha=%s cursor=%s prefix=%s", sha, cursor, prefix)
	if f.brainDelivered != nil {
		return f.brainDelivered(sha, cursor, prefix)
	}
	return 0
}

func (f *fakeOps) Up(request UpRequest, stdout, stderr io.Writer) int {
	f.mu.Lock()
	f.upRequests = append(f.upRequests, request)
	f.mu.Unlock()
	f.record("up runtime=%s session=%s pid=%s start=%s tag=%s runtime-session=%s no-runtime-session=%t start-source=%s recover-only=%t if-down=%t retire=%t",
		request.Runtime, request.Session, request.Pid, request.StartTime, request.Tag, request.RuntimeSession,
		request.NoRuntimeSession, request.StartSource, request.RecoverOnly, request.IfDown, request.Retire)
	if f.up != nil {
		return f.up(request, stdout, stderr)
	}
	fmt.Fprintln(stdout, "up outcome=armed authority=writer")
	return 0
}

func (f *fakeOps) SessionStart(root, session string, stdout, stderr io.Writer) int {
	f.record("session start %s", session)
	if f.sessionStart != nil {
		return f.sessionStart(stdout, stderr)
	}
	return 64
}

func (f *fakeOps) SessionEnd(root, session string) int {
	f.record("session end %s", session)
	if f.sessionEnd != nil {
		return f.sessionEnd()
	}
	return 0
}

func (f *fakeOps) HookAttempt(repo string, pid int, turnKey string) (string, int) {
	f.record("steward hook-attempt pid=%d", pid)
	if f.hookAttempt != nil {
		return f.hookAttempt(pid)
	}
	return `{"attemptSeq":3,"generation":2}` + "\n", 0
}

func (f *fakeOps) HookComplete(request HookCompletion) int {
	f.mu.Lock()
	f.completions = append(f.completions, request)
	f.mu.Unlock()
	f.record("steward hook-complete result=%s outcome=%s elapsed=%t", request.Result, request.Outcome, request.HasElapsed)
	if f.hookComplete != nil {
		return f.hookComplete(request)
	}
	return 0
}

func (f *fakeOps) HookExpire(repo string, elapsed int64) int {
	f.record("steward hook-expire elapsed=%d", elapsed)
	if f.hookExpire != nil {
		return f.hookExpire(elapsed)
	}
	return 0
}

func (f *fakeOps) HealthPreview(repo string, metasystemRoot roots.Installation) (string, int) {
	f.record("health --hook-preview")
	if f.health != nil {
		return f.health()
	}
	return `{"line":"HEALTH healthy — fixture"}` + "\n", 0
}

func TestStopHealthPhaseReadsTheTickVerdict(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name   string
		age    time.Duration
		data   string
		cached bool
	}{
		{name: "fresh", age: 119 * time.Second, cached: true},
		{name: "two ticks old", age: 120 * time.Second},
		{name: "stale", age: 121 * time.Second},
		{name: "future", age: -time.Second},
		{name: "missing", data: "missing"},
		{name: "malformed", data: "{"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			installation := newHookInstallation(t)
			state := newHookInstallation(t)
			ops := newFakeOps(t, installation)
			ops.stateRoot = func(string) (string, int) { return state.root, 0 }
			plantOpenStopEvidence(t, state.root, now)
			ops.git = func(args ...string) (string, error) {
				if strings.HasSuffix(strings.Join(args, " "), "config --get metasystem.steward.tick-seconds") {
					return "60\n", nil
				}
				return ops.ordinaryGit(args...)
			}
			observed := now.Add(-test.age)
			verdict := steward.HealthVerdict{Schema: 1, Observation: 7, ObservedAt: observed, Aggregate: "unhealthy",
				Roles: []steward.RoleVerdict{
					{Role: steward.RoleDisk, Status: steward.HealthDead, Reason: "tick fixture"},
					{Role: steward.RoleHookFreshness, Status: steward.HealthUnknown, Reason: "tick hook fixture"},
					{Role: steward.RoleStopHookDuration, Status: steward.HealthUnknown, Reason: "tick duration fixture"},
				}}
			data, err := json.Marshal(map[string]any{"state": steward.HealthObservationState{
				Sequence: 7, ObservedAt: observed, UnknownCounts: map[steward.HealthRole]int{}}, "verdict": verdict})
			if err != nil {
				t.Fatal(err)
			}
			if test.data != "" {
				data = []byte(test.data)
			}
			path := steward.HealthRecordPath(state.root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if test.data != "missing" {
				if err := os.WriteFile(path, data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			checks := 0
			ops.health = func() (string, int) {
				checks++
				return `{"line":"HEALTH healthy — evaluated fixture"}`, 0
			}
			if test.cached {
				ops.up = func(_ UpRequest, stdout, _ io.Writer) int {
					fmt.Fprintln(stdout, `up outcome=armed re-armed="generation=4 previous=3"`)
					return 0
				}
			}
			// Only the Stop's two roles change; the machinery verdict stays cached.
			verdict.Roles[1] = steward.RoleVerdict{Role: steward.RoleHookFreshness, Status: steward.HealthAlive,
				Reason: "turn generation 2 is pending; prior generation 1 completed as OK/EMITTED"}
			verdict.Roles[2] = steward.RoleVerdict{Role: steward.RoleStopHookDuration, Status: steward.HealthAlive,
				Reason: "the last Stop carried no measurement"}
			ops.stopInput = func(request StopInputRequest, _ io.Writer) int {
				if test.cached {
					captured, err := os.ReadFile(request.HealthFile)
					var preview steward.HookHealthPreview
					if err != nil || json.Unmarshal(captured, &preview) != nil || preview.Line != verdict.Line("agent") || preview.ExitCode != 1 || preview.Verdict.Observation != 7 {
						t.Errorf("cached health capture = %s, error %v", captured, err)
					}
				}
				return writeTestFile(request.OutputFile, `{"notices":"","failures":"","verdict":""}`)
			}
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"tick-fixture"}`,
				now: func() time.Time { return now }})
			wantLine, wantChecks := "HEALTH healthy — evaluated fixture", 1
			if test.cached {
				wantLine, wantChecks = verdict.Line("agent"), 0
			}
			if run.status != 0 || ops.completion(t).HealthLine != wantLine || checks != wantChecks {
				t.Fatalf("Stop status %d, health %q, evaluations %d; want %q, %d", run.status, ops.completion(t).HealthLine, checks, wantLine, wantChecks)
			}
			if test.cached {
				arming, err := os.ReadFile(filepath.Join(state.root, "artifacts", "agents", "supervision", "arming.log"))
				if err != nil || !strings.Contains(string(arming), " stop-re-armed 2 3\n") {
					t.Fatalf("Stop did not bind its re-arm to its attempt: %s, %v", arming, err)
				}
			}
			if test.data != "missing" {
				if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, data) {
					t.Fatalf("Stop rewrote tick health: %s, %v", after, err)
				}
			}
		})
	}
}

func plantOpenStopEvidence(t *testing.T, root string, now time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(steward.ComponentEvidencePath(root, "supervision-hook")), 0o755); err != nil {
		t.Fatal(err)
	}
	previous := now.Add(-time.Second)
	data, err := json.Marshal(steward.ComponentEvidence{
		Component: "supervision-hook", Generation: 2, AttemptSeq: 3, TurnKeyDigest: "fixture-turn",
		Pid: 4321, PidStartedAt: now.Unix() - 30, Result: steward.ComponentIndeterminate, EvidenceDigest: strings.Repeat("a", 64),
		LastAttempt: now, LastCompletion: previous, LastSuccess: previous, Outcome: "ATTEMPTING",
		AttemptHistory: []steward.ComponentAttemptHistory{{Generation: 1, AttemptSeq: 2,
			AttemptedAt: previous, CompletedAt: previous, Result: steward.ComponentOK, Outcome: "EMITTED", EvidenceDigest: strings.Repeat("b", 64)}},
	})
	if err != nil || writeTestFile(steward.ComponentEvidencePath(root, "supervision-hook"), string(data)) != 0 {
		t.Fatalf("plant Stop evidence: %v", err)
	}
}

func TestStopHealthPhaseHonoursFenceAndOwnAttempt(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 10, 5, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct{ name, phase, prefix string }{
		{"own attempt", "", "HEALTH healthy"},
		{"stopped", stopfence.PhaseStopped, "HEALTH STOPPED"},
		{"unfinished", stopfence.PhaseStopping, "HEALTH STOP UNFINISHED"},
		{"incomplete", stopfence.PhaseStopIncomplete, "HEALTH STOP INCOMPLETE"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			installation, state := newHookInstallation(t), newHookInstallation(t)
			ops := newFakeOps(t, installation)
			ops.stateRoot = func(string) (string, int) { return state.root, 0 }
			plantOpenStopEvidence(t, state.root, now)
			verdict := steward.HealthVerdict{Schema: 1, Observation: 7, ObservedAt: now, Aggregate: "unknown",
				Roles: []steward.RoleVerdict{
					{Role: steward.RoleStewardRunner, Status: steward.HealthAlive, Reason: "tick machinery fixture"},
					{Role: steward.RoleHookFreshness, Status: steward.HealthUnknown, Reason: "turn generation 2 is pending within the 60s Stop budget"},
					{Role: steward.RoleStopHookDuration, Status: steward.HealthAlive, Reason: "tick duration fixture"},
				}}
			if test.phase != "" {
				verdict.Aggregate, verdict.Roles[0].Status = "unhealthy", steward.HealthDead
				if err := stopfence.Write(installation.root, stopfence.Record{State: stopfence.StateClosed,
					Phase: test.phase, Generation: 4, Checkout: installation.root}); err != nil {
					t.Fatal(err)
				}
				ops.up = func(_ UpRequest, stdout, _ io.Writer) int {
					fmt.Fprintln(stdout, "up outcome=stopped")
					return 0
				}
			}
			data, err := json.Marshal(map[string]any{"state": steward.HealthObservationState{Sequence: 7, ObservedAt: now, UnknownCounts: map[steward.HealthRole]int{}}, "verdict": verdict})
			path := steward.HealthRecordPath(state.root)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err != nil || writeTestFile(path, string(data)) != 0 {
				t.Fatalf("plant tick health: %v", err)
			}
			ops.health = func() (string, int) {
				t.Error("Stop evaluated all roles despite a usable cache or closed fence")
				return "", 2
			}
			ops.stopInput = func(request StopInputRequest, _ io.Writer) int {
				captured, err := os.ReadFile(request.HealthFile)
				var preview steward.HookHealthPreview
				if err != nil || json.Unmarshal(captured, &preview) != nil || !strings.HasPrefix(preview.Line, test.prefix) || preview.ExitCode != 0 {
					t.Fatalf("Stop health capture = %s, error %v", captured, err)
				}
				if test.phase != "" && (!preview.Verdict.Stopped || strings.Contains(preview.Line, "tick machinery fixture")) {
					t.Fatalf("closed fence used cached roles: %+v", preview)
				}
				for _, intervention := range preview.Interventions {
					if intervention.SupervisionRepair {
						t.Errorf("Stop requested repair: %+v", intervention)
					}
				}
				if test.phase == "" && (preview.Verdict.Observation != 7 || len(preview.Verdict.Roles) != 3 || preview.Verdict.Roles[0].Line() != verdict.Roles[0].Line()) {
					t.Fatalf("Stop changed cached machinery or observation: %+v", preview.Verdict)
				}
				if test.phase == "" && preview.Verdict.Roles[2].Reason != "the last Stop carried no measurement" {
					t.Fatalf("Stop kept its cached duration role: %+v", preview.Verdict.Roles[2])
				}
				return writeTestFile(request.OutputFile, `{"notices":"","failures":"","verdict":""}`)
			}
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "stop", payload: `{"session_id":"cached-stop"}`, now: func() time.Time { return now }})
			if run.status != 0 || !strings.HasPrefix(ops.completion(t).HealthLine, test.prefix) {
				t.Fatalf("Stop = %+v", run)
			}
			if after, err := os.ReadFile(path); err != nil || !bytes.Equal(after, data) {
				t.Fatalf("Stop rewrote tick health: %s, %v", after, err)
			}
		})
	}
}

func (f *fakeOps) DigestPending(repo string) (string, int) {
	f.record("steward digest-pending")
	if f.digestPending != nil {
		return f.digestPending()
	}
	return `{"cursor":0,"message":"","prefixSha256":""}` + "\n", 0
}

func (f *fakeOps) DigestAdvance(repo, cursor, prefix string) int {
	f.record("steward digest-advance cursor=%s prefix=%s", cursor, prefix)
	if f.digestAdvance != nil {
		return f.digestAdvance(cursor, prefix)
	}
	return 0
}

func (f *fakeOps) ReceiptCheck(root string) (string, string, int) {
	f.record("receipt check")
	if f.receipt != nil {
		return f.receipt()
	}
	return "", "", 0
}

func (f *fakeOps) ProtocolGrowth(root, mainID string) (string, int) {
	f.record("lease protocol-growth main=%s", mainID)
	if f.protocolGrowth != nil {
		return f.protocolGrowth()
	}
	return `{"counts":{},"message":""}` + "\n", 0
}

func (f *fakeOps) ProtocolAdvance(root, mainID string, callerPid int, counts string) int {
	f.record("lease protocol-advance main=%s caller=%d counts=%s", mainID, callerPid, counts)
	if f.protocolAdv != nil {
		return f.protocolAdv(counts)
	}
	return 0
}

func (f *fakeOps) RenewLease(root string, callerPid int) int {
	f.record("lease renew caller=%d", callerPid)
	if f.renew != nil {
		return f.renew()
	}
	return 0
}

func (f *fakeOps) WatchdogReport(repo string) (string, int) {
	f.record("supervise watchdog-report")
	if f.watchdog != nil {
		return f.watchdog()
	}
	return "", 0
}

func (f *fakeOps) TurnVerdict(request TurnVerdictRequest) (string, string, int) {
	f.mu.Lock()
	f.turnVerdicts = append(f.turnVerdicts, request)
	f.mu.Unlock()
	f.record("report turn-verdict session=%s session-absent=%t stop-hook-active=%t main=%s transcript=%s runtime=%s",
		request.Session, request.SessionAbsent, request.StopHookActive, request.MainID, request.Transcript, request.Runtime)
	if f.turnVerdict != nil {
		return f.turnVerdict(request)
	}
	_ = os.WriteFile(request.FactsFile, []byte("{}\n"), 0o600)
	_ = os.WriteFile(request.CompletionFile, []byte("{}\n"), 0o600)
	return f.verdict + "\n", "", 0
}

func (f *fakeOps) StopBlock(request StopBlockRequest) (string, int) {
	f.mu.Lock()
	f.stopBlocks = append(f.stopBlocks, request)
	f.mu.Unlock()
	f.record("report stop-block class=%s record=%s cause=%s", request.Class, filepath.Base(request.RefusalRecord), request.Cause)
	if f.stopBlock != nil {
		return f.stopBlock(request)
	}
	return realStopBlock(request)
}

// realStopBlock renders a refusal with the report owner, as the verb did.
func realStopBlock(request StopBlockRequest) (string, int) {
	systemMessage := request.SystemMessage
	if request.ArmingResult != "" {
		if systemMessage != "" {
			systemMessage += "\n"
		}
		systemMessage += request.ArmingResult
	}
	var block map[string]any
	if request.RefusalRecord != "" {
		class := report.StopClass(request.Class)
		if class == "" {
			class = report.StopClassSeatActionable
		}
		refusal, err := report.StopRefusal(request.RefusalRecord, request.Session, request.Cause, request.Remedy, request.Detail, report.BoundSystemMessage(systemMessage), class, time.Now())
		if err != nil {
			return "", 1
		}
		block = refusal
	} else {
		block = report.StopBlock(request.Detail)
		if systemMessage != "" {
			block["systemMessage"] = report.BoundSystemMessage(systemMessage)
		}
	}
	encoded, _ := json.Marshal(block)
	return string(encoded) + "\n", 0
}

func (f *fakeOps) StopInput(request StopInputRequest, stderr io.Writer) int {
	f.mu.Lock()
	f.stopInputs = append(f.stopInputs, request)
	f.mu.Unlock()
	f.record("report stop-input advisor=%t", request.Advisor)
	if f.stopInput != nil {
		return f.stopInput(request, stderr)
	}
	notices, _ := os.ReadFile(request.NoticeFile)
	failures, _ := os.ReadFile(request.FailureFile)
	verdict, _ := os.ReadFile(request.VerdictFile)
	arming, _ := os.ReadFile(request.ArmingStderrFile)
	f.mu.Lock()
	f.captured = append(f.captured, capturedPresentation{string(notices), string(failures), string(verdict), string(arming)})
	f.mu.Unlock()
	encoded, _ := json.Marshal(map[string]string{"notices": string(notices), "failures": string(failures), "verdict": string(verdict)})
	return writeTestFile(request.OutputFile, string(encoded))
}

func (f *fakeOps) StopPresent(root, input, output string, stderr io.Writer) int {
	f.record("report stop-present")
	if f.stopPresent != nil {
		return f.stopPresent(input, output, stderr)
	}
	data, _ := os.ReadFile(input)
	return writeTestFile(output, `{"report":{"id":"report-id","alias":"a1","path":"reports/r.md","sha256":"`+strings.Repeat("c", 64)+`"},"input":`+string(data)+`}`)
}

// StopOutput maps the fake presentation: a block when the retained verdict
// blocks, else an allowance carrying the notices and failures it was given.
func (f *fakeOps) StopOutput(runtime, input, output string, stderr io.Writer) int {
	f.record("adapter stop-output runtime=%s", runtime)
	if f.stopOutput != nil {
		return f.stopOutput(runtime, input, output, stderr)
	}
	data, _ := os.ReadFile(input)
	var presentation struct {
		Input struct{ Notices, Failures, Verdict string } `json:"input"`
	}
	_ = json.Unmarshal(data, &presentation)
	text := "presented: " + presentation.Input.Notices
	if presentation.Input.Failures != "" {
		text += "\nfailure: " + presentation.Input.Failures
	}
	if strings.Contains(presentation.Input.Verdict, `"shouldBlock":true`) {
		encoded, _ := json.Marshal(map[string]string{"decision": "block", "reason": text})
		return writeTestFile(output, string(encoded)+"\n")
	}
	encoded, _ := json.Marshal(map[string]string{"systemMessage": text})
	return writeTestFile(output, string(encoded)+"\n")
}

func writeTestFile(path, content string) int {
	if os.WriteFile(path, []byte(content), 0o600) != nil {
		return 1
	}
	return 0
}

func (f *fakeOps) EvidenceGC(installation roots.Installation, output io.Writer) int {
	f.record("evidence-gc")
	if f.evidenceGC != nil {
		return f.evidenceGC(output)
	}
	return 0
}

func (f *fakeOps) Slug(value string) string { return strings.ToLower(value) }

func (f *fakeOps) TokenHex(int) (string, error) {
	if f.tokenHex != nil {
		return f.tokenHex()
	}
	return strings.Repeat("ab", 16), nil
}

func (f *fakeOps) Git(args ...string) (string, error) {
	f.record("git %s", strings.Join(args, " "))
	if f.git != nil {
		return f.git(args...)
	}
	return f.ordinaryGit(args...)
}

// ordinaryGit answers the hook's Git questions for an ordinary checkout at
// the installation root.
func (f *fakeOps) ordinaryGit(args ...string) (string, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.HasSuffix(joined, "rev-parse --path-format=absolute --git-dir --git-common-dir"):
		return args[1] + "/.git\n" + args[1] + "/.git\n", nil
	case strings.HasSuffix(joined, "config --get metasystem.goal.machine"):
		return "fixture-machine\n", nil
	}
	return "", fmt.Errorf("fixture git: unexpected %s", joined)
}

func (f *fakeOps) EngineBehind(installation roots.Installation, repo string) (bool, error) {
	f.record("engine behind")
	if f.engineBehind != nil {
		return f.engineBehind()
	}
	return false, nil
}

func (f *fakeOps) UnmigratableRetainedPlans(repo string) ([]string, error) {
	f.record("retained plans")
	if f.unmigratable != nil {
		return f.unmigratable()
	}
	return nil, nil
}

func (f *fakeOps) StartEngineRebuild(installation roots.Installation) error {
	f.record("start engine rebuild")
	if f.rebuild != nil {
		return f.rebuild()
	}
	return nil
}

// hookRun is one invocation's observed result.
type hookRun struct {
	status         int
	stdout, stderr string
}

type hookCall struct {
	runtime, event, payload string
	env                     map[string]string
	ppid                    int
	signals                 chan os.Signal
	exec                    func(string, []string, []string) error
	now                     func() time.Time
	monotonic               func() time.Duration
	startWorker             func(string, string, []string, *os.File, *os.File, *os.File) (Worker, error)
	fixtureDeadline         chan time.Time
	after                   func(time.Duration) <-chan time.Time
	stdout                  io.Writer
	tempDir                 string
	resolverAnswered        func()
}

func runHook(t *testing.T, installation hookInstallation, ops Ops, call hookCall) hookRun {
	t.Helper()
	var stdout, stderr bytes.Buffer
	env := call.env
	if env == nil {
		env = map[string]string{}
	}
	if call.ppid == 0 {
		call.ppid = 777
	}
	if call.event == "stop" || call.event == "receipt" || call.event == "end" {
		if _, set := env[stopDeadlineParentEnv]; !set && call.startWorker == nil {
			// The Stop worker path unless the test drives the deadline parent.
			env[stopDeadlineParentEnv] = fmt.Sprint(call.ppid)
		}
	}
	if call.tempDir == "" {
		call.tempDir = t.TempDir()
	}
	now := call.now
	if now == nil {
		now = time.Now
	}
	var out io.Writer = &stdout
	if call.stdout != nil {
		out = call.stdout
	}
	inv := Invocation{
		Runtime: call.runtime, Event: call.event, Stdin: strings.NewReader(call.payload),
		Stdout: out, Stderr: &stderr,
		Lookup: func(name string) (string, bool) { value, ok := env[name]; return value, ok },
		Pid:    os.Getpid(), Ppid: call.ppid, Installation: installation.root,
		Now: now, Monotonic: call.monotonic, After: call.after, Sleep: func(time.Duration) { runtime.Gosched() },
		Signals: call.signals, Exec: call.exec,
		Environ:     func() []string { return os.Environ() },
		StartWorker: call.startWorker, TempDir: call.tempDir,
		Deadline: DeadlineDeps{
			BootClock: identity.BootClock, Prober: identity.KernelProber{}, ParentPid: identity.ParentPid,
			EventInterval: 10 * time.Millisecond, ResolverAnswered: call.resolverAnswered,
		},
	}
	if call.fixtureDeadline != nil {
		fixture := call.fixtureDeadline
		inv.Deadline.FixtureDeadline = func(context.Context, roots.Installation) (<-chan time.Time, error) { return fixture, nil }
	}
	status := RunRuntimeHook(inv, ops)
	return hookRun{status: status, stdout: stdout.String(), stderr: stderr.String()}
}

// testWorker launches this test binary as a Stop worker process.
type testWorker struct {
	pid    int
	done   chan struct{}
	status int
}

func (w *testWorker) Pid() int              { return w.pid }
func (w *testWorker) Done() <-chan struct{} { return w.done }
func (w *testWorker) Status() int           { return w.status }

func startTestWorker(t *testing.T, extra map[string]string, launched chan<- []string) func(string, string, []string, *os.File, *os.File, *os.File) (Worker, error) {
	return func(script, runtime string, env []string, stdin, stdout, stderr *os.File) (Worker, error) {
		if launched != nil {
			launched <- append([]string{script, runtime}, env...)
		}
		command := exec.Command(os.Args[0], "-test.run=^$")
		command.Env = append(append([]string(nil), env...), hookWorkerModeEnv+"=1")
		for key, value := range extra {
			command.Env = append(command.Env, key+"="+value)
		}
		command.Stdin, command.Stdout, command.Stderr = stdin, stdout, stderr
		if err := command.Start(); err != nil {
			return nil, err
		}
		worker := &testWorker{pid: command.Process.Pid, done: make(chan struct{})}
		go func() {
			err := command.Wait()
			if exitError, ok := err.(*exec.ExitError); ok {
				worker.status = exitError.ExitCode()
				if worker.status < 0 {
					worker.status = 137
				}
			} else if err != nil {
				worker.status = 127
			}
			close(worker.done)
		}()
		t.Cleanup(func() {
			select {
			case <-worker.done:
			default:
				_ = command.Process.Kill()
				<-worker.done
			}
		})
		return worker, nil
	}
}

func mustForm(t *testing.T, outcome, cause string, qualifiers ...string) string {
	t.Helper()
	form, err := DegradedStopForm(outcome, cause, qualifiers...)
	if err != nil {
		t.Fatal(err)
	}
	return form
}

func readHookLog(t *testing.T, installation hookInstallation) string {
	t.Helper()
	data, _ := os.ReadFile(filepath.Join(installation.root, "artifacts", "agents", "supervision", "hooks.log"))
	return string(data)
}

// makeFIFO creates a named pipe for a worker handshake.
func makeFIFO(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The FIFO handshakes never rely on the open(2) rendezvous alone. On the BSD
// kernel a reader blocked in open(2) can miss a writer that opens and closes
// before the reader is scheduled, and then waits for a writer for ever; a
// writer that closes without writing leaves a late reader nothing to return
// on. Under host load both happen. So each handshake carries one byte, and
// the test's end holds the FIFO open for reading and writing: its own open
// never waits, and a byte written on either side stays readable until it is
// read.

// releaseFIFO lets a child blocked on the release FIFO continue: the release
// line is written at once and the test keeps the FIFO open until it ends, so
// the child reads it whenever it gets there.
func releaseFIFO(t *testing.T, path string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if _, err := file.WriteString("\n"); err != nil {
		t.Fatal(err)
	}
}

// awaitFIFO waits for a child's one handshake message on the FIFO.
func awaitFIFO(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Read(make([]byte, 1))
	return err
}

// unblockFIFO releases a reader still blocked on a FIFO at cleanup without
// blocking when no reader is left.
func unblockFIFO(path string) {
	if file, err := os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0); err == nil {
		_, _ = file.WriteString("\n")
		_ = file.Close()
	}
}
