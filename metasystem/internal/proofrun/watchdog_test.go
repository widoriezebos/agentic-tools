package proofrun

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

type fixedProbe struct {
	exact identity.Exact
	state identity.Liveness
}

type sequenceProbe struct {
	exacts []identity.Exact
	states []identity.Liveness
	calls  int
}

func (p *sequenceProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	index := p.calls
	p.calls++
	if index >= len(p.exacts) {
		index = len(p.exacts) - 1
	}
	return p.exacts[index], p.states[index], nil
}

type pidProbe struct{ started int64 }

func (p pidProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(p.started, 0)}, identity.Alive, nil
}

func TestEvidenceTimeoutLeavesLoudPartialNoteBeforeRefusingKill(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "block-preserve.sh")
	if err := os.WriteFile(blocker, []byte("#!/usr/bin/env bash\nexec sleep 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute).Unix()
	options := WatchdogOptions{
		Suite: "fixture", Root: root, ProgressPath: "progress", DonePath: "done",
		LogPaths: []string{"log"}, SuiteIdentity: identity.Ref{Pid: 72, StartedAtSec: started},
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: 20 * time.Millisecond, EvidenceMax: 1,
		TermGrace: time.Millisecond, KillGrace: time.Millisecond, Executable: blocker, ErrorOutput: os.Stderr,
		Prober: fixedProbe{exact: identity.Exact{Pid: 72, StartedAt: time.Unix(started+1, 0)}, state: identity.Alive},
	}
	err := stopStalledSuite(options, "fixture-section", "fixture stall", ProgressRun{})
	if err == nil || !strings.Contains(err.Error(), "partial evidence retained") {
		t.Fatalf("error = %v", err)
	}
	matches, globErr := filepath.Glob(filepath.Join(root, "artifacts", "agents", "suite-failures", "*", "copy-note.txt"))
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("partial note paths = %v, %v", matches, globErr)
	}
	note, readErr := os.ReadFile(matches[0])
	if readErr != nil || !strings.Contains(string(note), "DROPPED evidence copy exceeded") {
		t.Fatalf("partial note = %q, %v", note, readErr)
	}
}

func TestDoneFileWinsBeforeAndDuringEvidencePreservation(t *testing.T) {
	evidenceTimeout := time.Second
	if raw := os.Getenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI"); raw != "" {
		scaleMilli, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || scaleMilli < 1 {
			t.Fatalf("METASYSTEM_FIXTURE_CAP_SCALE_MILLI must be a positive integer: %q", raw)
		}
		scaledSeconds := scaleMilli / 1000
		if scaleMilli%1000 != 0 {
			scaledSeconds++
		}
		evidenceTimeout = time.Duration(scaledSeconds) * time.Second
	}
	for _, test := range []struct {
		name       string
		doneBefore bool
	}{
		{name: "before preservation", doneBefore: true},
		{name: "during preservation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			done := filepath.Join(root, "done")
			preserve := filepath.Join(root, "preserve.sh")
			if test.doneBefore {
				if err := os.WriteFile(done, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				writeExecutable(t, preserve, "#!/usr/bin/env bash\nexit 99\n")
			} else {
				writeExecutable(t, preserve, fmt.Sprintf("#!/usr/bin/env bash\ntouch %q\n", done))
			}
			actions := 0
			err := stopStalledSuite(WatchdogOptions{
				Suite: "fixture", Root: root, DonePath: done, LogPaths: []string{"log"},
				SuiteIdentity: identity.Ref{Pid: 7, StartedAtSec: 8}, EvidenceTimeout: evidenceTimeout, EvidenceMax: 1,
				Executable: preserve, ErrorOutput: os.Stderr,
				Shutdown: func() error { actions++; return nil },
				Signal:   func(int, syscall.Signal) error { actions++; return nil },
				Prober: fixedProbe{
					exact: identity.Exact{Pid: 7, StartedAt: time.Unix(8, 0)},
					state: identity.Alive,
				},
			}, "finished", "stale observation", ProgressRun{})
			if err != nil || actions != 0 {
				t.Fatalf("error = %v, kill-capable actions = %d", err, actions)
			}
			if test.doneBefore {
				if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
					t.Fatalf("evidence preservation ran after done: %v", err)
				}
			}
		})
	}
}

func (p fixedProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, nil
}

func TestRunWatchdogStopsPrintingSectionAtAbsoluteCap(t *testing.T) {
	root := t.TempDir()
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "suite.log")
	if err := os.WriteFile(logPath, []byte("still printing\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{logPath}}); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute)
	if err := AppendSectionEvent(progress, SectionEvent{
		Suite: "fixture", Section: "printing", Event: "start", At: started.UTC().Format(time.RFC3339Nano), Depth: 0,
	}); err != nil {
		t.Fatal(err)
	}
	preserve := filepath.Join(root, "preserve.sh")
	writeExecutable(t, preserve, "#!/usr/bin/env bash\necho bounded-copy-completed\n")
	shutdowns := 0
	var signals []syscall.Signal
	err := RunWatchdog(WatchdogOptions{
		Suite: "fixture", Root: root, ProgressPath: progress, DonePath: filepath.Join(root, "done"), LogPaths: []string{logPath},
		SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started.Unix()},
		Silence:       time.Hour, SectionCap: time.Millisecond, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
		Executable: preserve, Output: os.Stdout, ErrorOutput: os.Stderr,
		Prober:   fixedProbe{exact: identity.Exact{Pid: 999999, StartedAt: time.Unix(started.Unix(), 0)}, state: identity.Alive},
		Signal:   func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
		Shutdown: func() error { shutdowns++; return nil },
	})
	if err == nil || !strings.Contains(err.Error(), "printing") || !strings.Contains(err.Error(), "section exceeded") {
		t.Fatalf("error = %v", err)
	}
	if shutdowns != 1 || fmt.Sprint(signals) != fmt.Sprint([]syscall.Signal{syscall.SIGCONT, syscall.SIGTERM, syscall.SIGKILL}) {
		t.Fatalf("shutdowns = %d, signals = %v", shutdowns, signals)
	}
}

func TestRunWatchdogReturnsOnDoneAndValidatesBounds(t *testing.T) {
	root := t.TempDir()
	done := filepath.Join(root, "done")
	logPath := filepath.Join(root, "log")
	if err := os.WriteFile(done, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	options := WatchdogOptions{
		Suite: "fixture", Root: root, ProgressPath: filepath.Join(root, "progress"), DonePath: done, LogPaths: []string{logPath},
		SuiteIdentity: identity.Ref{Pid: 7, StartedAtSec: 8}, Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1, Poll: time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
	}
	if err := RunWatchdog(options); err != nil {
		t.Fatal(err)
	}
	options.Poll = 0
	if err := RunWatchdog(options); err == nil || !strings.Contains(err.Error(), "polling") {
		t.Fatalf("invalid bounds error = %v", err)
	}
}

func TestSuiteSignalsReauthenticateImmediatelyAndAbortOnMismatch(t *testing.T) {
	started := time.Now().Add(-time.Minute).Unix()
	matching := identity.Exact{Pid: 999997, StartedAt: time.Unix(started, 0)}
	recycled := identity.Exact{Pid: 999997, StartedAt: time.Unix(started+1, 0)}
	probe := &sequenceProbe{
		exacts: []identity.Exact{matching, matching, recycled},
		states: []identity.Liveness{identity.Alive, identity.Alive, identity.Alive},
	}
	var signals []syscall.Signal
	err := signalSuiteGroup(WatchdogOptions{
		SuiteIdentity: identity.Ref{Pid: 999997, StartedAtSec: started},
		TermGrace:     time.Millisecond, KillGrace: time.Millisecond,
		Signal: func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
	}, probe)
	if err == nil || !strings.Contains(err.Error(), "kill") || !strings.Contains(err.Error(), "recorded start identity") {
		t.Fatalf("error = %v", err)
	}
	if probe.calls != 3 || fmt.Sprint(signals) != fmt.Sprint([]syscall.Signal{syscall.SIGCONT, syscall.SIGTERM}) {
		t.Fatalf("probe calls = %d, signals = %v", probe.calls, signals)
	}
}

func TestGuardMemberSignalsReauthenticateAndAbortOnMismatch(t *testing.T) {
	started := time.Now().Add(-time.Minute).Unix()
	matching := identity.Exact{Pid: 999996, StartedAt: time.Unix(started, 0)}
	recycled := identity.Exact{Pid: 999996, StartedAt: time.Unix(started+1, 0)}
	probe := &sequenceProbe{
		exacts: []identity.Exact{matching, recycled},
		states: []identity.Liveness{identity.Alive, identity.Alive},
	}
	var signals []syscall.Signal
	err := stopGuardMember(WatchdogOptions{
		TermGrace: time.Millisecond,
		Signal:    func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
	}, identity.Ref{Pid: 999996, StartedAtSec: started}, probe)
	if err == nil || !strings.Contains(err.Error(), "terminate") || !strings.Contains(err.Error(), "recorded start identity") {
		t.Fatalf("error = %v", err)
	}
	if probe.calls != 2 || fmt.Sprint(signals) != fmt.Sprint([]syscall.Signal{syscall.SIGCONT}) {
		t.Fatalf("probe calls = %d, signals = %v", probe.calls, signals)
	}
}

func TestExecutionGuardSweepSignalsExactDetachedMember(t *testing.T) {
	root := t.TempDir()
	guard := filepath.Join(root, "artifacts", "agents", "supervision", "gate-runs", "checkout-execution.lock.d")
	if err := os.MkdirAll(guard, 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute).Unix()
	record := fmt.Sprintf(`{"members":[{"pid":999998,"pidStartedAt":%d}]}`, started)
	if err := os.WriteFile(filepath.Join(guard, "owner.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	var signals []syscall.Signal
	options := WatchdogOptions{
		Root: root, SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started},
		TermGrace: time.Millisecond,
		Signal:    func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
	}
	probe := fixedProbe{exact: identity.Exact{Pid: 999998, StartedAt: time.Unix(started, 0)}, state: identity.Alive}
	if err := sweepExecutionGuard(options, probe); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 3 || signals[0] != syscall.SIGCONT || signals[2] != syscall.SIGKILL {
		t.Fatalf("signals = %v", signals)
	}
}

func TestExecutionGuardSweepContinuesAfterMemberFailure(t *testing.T) {
	root := t.TempDir()
	guard := filepath.Join(root, "artifacts", "agents", "supervision", "gate-runs", "checkout-execution.lock.d")
	if err := os.MkdirAll(guard, 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute).Unix()
	record := fmt.Sprintf(`{"members":[{"pid":999991,"pidStartedAt":%d},{"pid":999992,"pidStartedAt":%d}]}`, started, started)
	if err := os.WriteFile(filepath.Join(guard, "owner.json"), []byte(record), 0o600); err != nil {
		t.Fatal(err)
	}
	var secondSignals []syscall.Signal
	options := WatchdogOptions{
		Root: root, SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started}, TermGrace: time.Millisecond,
		Signal: func(target int, signal syscall.Signal) error {
			if target == 999991 || target == -999991 {
				return syscall.EPERM
			}
			secondSignals = append(secondSignals, signal)
			return nil
		},
	}
	err := sweepExecutionGuard(options, pidProbe{started: started})
	if err == nil || !strings.Contains(err.Error(), "999991") {
		t.Fatalf("union error = %v", err)
	}
	if fmt.Sprint(secondSignals) != fmt.Sprint([]syscall.Signal{syscall.SIGCONT, syscall.SIGTERM, syscall.SIGKILL}) {
		t.Fatalf("second member signals = %v", secondSignals)
	}
}

func TestRecycledSuiteIdentityAuthorizesNoKillAction(t *testing.T) {
	var signals int
	var shutdowns int
	started := time.Now().Add(-time.Minute).Unix()
	options := WatchdogOptions{
		Suite: "fixture", Root: t.TempDir(), ProgressPath: "progress", DonePath: "done",
		LogPaths: []string{"log"}, SuiteIdentity: identity.Ref{Pid: 71, StartedAtSec: started},
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1,
		TermGrace: time.Millisecond, KillGrace: time.Millisecond, Executable: os.Args[0], ErrorOutput: os.Stderr,
		Prober:   fixedProbe{exact: identity.Exact{Pid: 71, StartedAt: time.Unix(started+1, 0)}, state: identity.Alive},
		Signal:   func(int, syscall.Signal) error { signals++; return nil },
		Shutdown: func() error { shutdowns++; return nil },
	}
	err := stopStalledSuite(options, "fixture-section", "fixture stall", ProgressRun{})
	if err == nil || !strings.Contains(err.Error(), "kill refused") {
		t.Fatalf("error = %v", err)
	}
	if signals != 0 || shutdowns != 0 {
		t.Fatalf("recycled identity caused %d signals and %d shutdowns", signals, shutdowns)
	}
}
