package proofrun

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
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

type watchdogTestClock struct {
	now    time.Time
	sleeps int
}

func newWatchdogTestClock() *watchdogTestClock {
	return &watchdogTestClock{now: time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)}
}

func (clock *watchdogTestClock) Now() time.Time { return clock.now }

func (clock *watchdogTestClock) Sleep(duration time.Duration) {
	clock.sleeps++
	clock.now = clock.now.Add(duration)
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
	if err := testexec.WriteFile(blocker, []byte("#!/usr/bin/env bash\nexec sleep 2\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	started := time.Now().Add(-time.Minute).Unix()
	timerCalls := 0
	options := WatchdogOptions{
		Suite: "fixture", Root: root, ProgressPath: "progress", DonePath: "done",
		LogPaths: []string{"log"}, SuiteIdentity: identity.Ref{Pid: 72, StartedAtSec: started},
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: 20 * time.Millisecond, EvidenceMax: 1,
		TermGrace: time.Millisecond, KillGrace: time.Millisecond, Executable: blocker, ErrorOutput: os.Stderr,
		Prober: fixedProbe{exact: identity.Exact{Pid: 72, StartedAt: time.Unix(started+1, 0)}, state: identity.Alive},
		NewTimer: func(time.Duration) (<-chan time.Time, func()) {
			timerCalls++
			fired := make(chan time.Time, 1)
			fired <- time.Unix(1, 0)
			return fired, func() {}
		},
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
	if timerCalls != 1 {
		t.Fatalf("evidence timer creations = %d, want 1", timerCalls)
	}
}

func TestDoneFileWinsBeforeAndDuringEvidencePreservation(t *testing.T) {
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
			if test.doneBefore {
				if err := os.WriteFile(done, nil, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			actions := 0
			preservations := 0
			err := stopStalledSuite(WatchdogOptions{
				Suite: "fixture", Root: root, DonePath: done, LogPaths: []string{"log"},
				SuiteIdentity: identity.Ref{Pid: 7, StartedAtSec: 8}, ErrorOutput: os.Stderr,
				PreserveEvidence: func(string, []string) string {
					preservations++
					if err := os.WriteFile(done, nil, 0o600); err != nil {
						t.Fatal(err)
					}
					return "bounded copy completed"
				},
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
				if preservations != 0 {
					t.Fatalf("evidence preservations after done = %d", preservations)
				}
				if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
					t.Fatalf("evidence preservation ran after done: %v", err)
				}
			} else if preservations != 1 {
				t.Fatalf("evidence preservations before done = %d", preservations)
			}
		})
	}
}

func (p fixedProbe) Probe(int64) (identity.Exact, identity.Liveness, error) {
	return p.exact, p.state, nil
}

func endSuiteAfterNotes(t *testing.T, donePath string, fragments ...string) (*os.File, <-chan time.Time, func() string) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})
	wake := make(chan time.Time, 1)
	notes := &synchronizedBuffer{}
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			_, _ = notes.Write([]byte(scanner.Text() + "\n"))
			found := true
			for _, fragment := range fragments {
				found = found && strings.Contains(notes.String(), fragment)
			}
			if found {
				if err := os.WriteFile(donePath, nil, 0o600); err != nil {
					t.Errorf("publish suite completion after watchdog notes: %v", err)
				}
				wake <- time.Unix(1, 0)
				return
			}
		}
		t.Errorf("watchdog note stream ended before it contained %q; notes = %q; read error = %v", fragments, notes.String(), scanner.Err())
	}()
	return writer, wake, notes.String
}

// A printing section past its cap, and a suite past its reservation's
// deadline, run on: the watchdog notes both and neither clock reading
// authorizes the suite to end.
func TestRunWatchdogLetsAPrintingSectionAndAnExpiredDeadlineRunOn(t *testing.T) {
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
	done := filepath.Join(root, "done")
	notePipe, wake, readNotes := endSuiteAfterNotes(t, done, "passed its 1ms cap", "deadline")
	observedAt := started.Add(2 * time.Minute)
	nowCalls, tickerCalls := 0, 0
	err := RunWatchdog(WatchdogOptions{
		Suite: "fixture", Root: root, ProgressPath: progress, DonePath: done, LogPaths: []string{logPath},
		SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started.Unix()},
		Deadline:      started.Add(time.Minute),
		Silence:       time.Hour, SectionCap: time.Millisecond, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
		Executable: preserve, Output: os.Stdout, ErrorOutput: notePipe,
		Prober:   fixedProbe{exact: identity.Exact{Pid: 999999, StartedAt: time.Unix(started.Unix(), 0)}, state: identity.Alive},
		Signal:   func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
		Shutdown: func() error { shutdowns++; return nil },
		Now: func() time.Time {
			nowCalls++
			return observedAt
		},
		NewTicker: func(time.Duration) (<-chan time.Time, func()) {
			tickerCalls++
			return wake, func() {}
		},
	})
	if err != nil {
		t.Fatalf("a printing section past its cap and a passed deadline ended the suite: %v", err)
	}
	if shutdowns != 0 || len(signals) != 0 {
		t.Fatalf("the clock signalled the suite: shutdowns = %d, signals = %v", shutdowns, signals)
	}
	if nowCalls == 0 || tickerCalls != 1 {
		t.Fatalf("watchdog artificial time use: now calls=%d ticker creations=%d", nowCalls, tickerCalls)
	}
	if got := readNotes(); strings.Count(got, "passed its 1ms cap") != 1 || strings.Count(got, "the reservation's deadline") != 1 {
		t.Fatalf("the notes were not written once each:\n%s", got)
	}
}

// TestRunWatchdogEndsASuiteForAVerdictOrACancellationAndNothingElse is
// row 11 of the hang-detection design: a suite whose output stopped for
// longer than any silence window runs on; the supervisor's dead or runaway
// verdict on the current section ends it, and so does a cancellation
// intent recorded on the attempt.
func TestRunWatchdogEndsASuiteForAVerdictOrACancellationAndNothingElse(t *testing.T) {
	newBed := func(t *testing.T) (string, string, string) {
		root := t.TempDir()
		progress := filepath.Join(root, "progress.jsonl")
		logPath := filepath.Join(root, "suite.log")
		if err := os.WriteFile(logPath, []byte("once\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := AppendProgressHeader(progress, ProgressHeader{LogPaths: []string{logPath}}); err != nil {
			t.Fatal(err)
		}
		if err := AppendSectionEvent(progress, SectionEvent{Suite: "fixture", Section: "quiet", Event: "start",
			At: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339Nano), Depth: 0}); err != nil {
			t.Fatal(err)
		}
		preserve := filepath.Join(root, "preserve.sh")
		writeExecutable(t, preserve, "#!/usr/bin/env bash\necho bounded-copy-completed\n")
		return root, progress, logPath
	}
	started := time.Now().Add(-time.Minute)
	options := func(root, progress, logPath string, shutdowns *int) WatchdogOptions {
		clock := &watchdogTestClock{now: started.Add(2 * time.Hour)}
		return WatchdogOptions{
			Suite: "fixture", Root: root, ProgressPath: progress, DonePath: filepath.Join(root, "done"), LogPaths: []string{logPath},
			SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started.Unix()},
			Silence:       time.Millisecond, SectionCap: time.Millisecond, EvidenceTimeout: time.Second, EvidenceMax: 1024,
			Poll: time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
			Executable: filepath.Join(root, "preserve.sh"), Output: os.Stdout, ErrorOutput: os.Stderr,
			Prober:   fixedProbe{exact: identity.Exact{Pid: 999999, StartedAt: time.Unix(started.Unix(), 0)}, state: identity.Alive},
			Signal:   func(int, syscall.Signal) error { return nil },
			Shutdown: func() error { *shutdowns++; return nil },
			Now:      clock.Now,
			Sleep:    clock.Sleep,
		}
	}
	t.Run("silence and a passed cap end nothing; the done file does", func(t *testing.T) {
		root, progress, logPath := newBed(t)
		shutdowns := 0
		opts := options(root, progress, logPath, &shutdowns)
		// The done file lands once the watchdog has noted the cap, a fact
		// the note proves it read the silent section past its window.
		notePipe, wake, _ := endSuiteAfterNotes(t, opts.DonePath, "passed its 1ms cap")
		opts.ErrorOutput = notePipe
		opts.NewTicker = func(time.Duration) (<-chan time.Time, func()) { return wake, func() {} }
		if err := RunWatchdog(opts); err != nil || shutdowns != 0 {
			t.Fatalf("a silent suite past every window was ended by the clock: err = %v, shutdowns = %d", err, shutdowns)
		}
	})
	t.Run("a verdict followed by an end does not end a later invocation of the section", func(t *testing.T) {
		root, progress, logPath := newBed(t)
		for _, progressEvent := range []SectionEvent{
			{Suite: "fixture", Section: "quiet", Event: "verdict", At: time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano), Depth: 0, Verdict: "dead"},
			{Suite: "fixture", Section: "quiet", Event: "end", At: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), Depth: 0},
			{Suite: "fixture", Section: "quiet", Event: "start", At: time.Now().Add(-time.Minute).UTC().Format(time.RFC3339Nano), Depth: 0},
		} {
			if err := AppendSectionEvent(progress, progressEvent); err != nil {
				t.Fatal(err)
			}
		}
		run, err := ReadLatestProgressRun(progress)
		if err != nil {
			t.Fatal(err)
		}
		if verdict := sectionVerdict(run, "fixture", "quiet"); verdict != "" {
			t.Fatalf("closed section verdict %q attached to a later invocation", verdict)
		}
		shutdowns := 0
		opts := options(root, progress, logPath, &shutdowns)
		notePipe, wake, _ := endSuiteAfterNotes(t, opts.DonePath, "passed its 1ms cap")
		opts.ErrorOutput = notePipe
		opts.NewTicker = func(time.Duration) (<-chan time.Time, func()) { return wake, func() {} }
		if err := RunWatchdog(opts); err != nil || shutdowns != 0 {
			t.Fatalf("a verdict from a closed section invocation ended the suite: err = %v, shutdowns = %d", err, shutdowns)
		}
		if _, err := os.Stat(opts.DonePath); err != nil {
			t.Fatalf("suite did not continue to its completion file: %v", err)
		}
	})
	t.Run("the supervisor's verdict on the still-open section ends the suite", func(t *testing.T) {
		root, progress, logPath := newBed(t)
		if err := AppendSectionEvent(progress, SectionEvent{Suite: "fixture", Section: "quiet", Event: "verdict",
			At: time.Now().UTC().Format(time.RFC3339Nano), Depth: 0, Verdict: "dead"}); err != nil {
			t.Fatal(err)
		}
		shutdowns := 0
		err := RunWatchdog(options(root, progress, logPath, &shutdowns))
		if err == nil || !strings.Contains(err.Error(), "the supervisor judged section quiet dead") || shutdowns != 1 {
			t.Fatalf("a dead verdict did not end the suite: err = %v, shutdowns = %d", err, shutdowns)
		}
	})
	t.Run("a cancellation intent recorded on the attempt ends the suite", func(t *testing.T) {
		root, progress, logPath := newBed(t)
		controlRoot, proofIdentity := proofAttemptFixture(t, "watchdog-cancellation")
		launcher, err := CurrentProcessIdentity(nil)
		if err != nil {
			t.Fatal(err)
		}
		attempt, _, err := ReserveLocked(candidateAdmission(AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: controlRoot, GoalID: "goal-a",
			GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: time.Now().UTC()}))

		if err != nil {
			t.Fatal(err)
		}
		if err := RequestCancellation(controlRoot, attempt.AttemptID, "a person said stop"); err != nil {
			t.Fatal(err)
		}
		shutdowns := 0
		opts := options(root, progress, logPath, &shutdowns)
		opts.ControlRoot, opts.AttemptID = controlRoot, attempt.AttemptID
		err = RunWatchdog(opts)
		if err == nil || !strings.Contains(err.Error(), "cancellation intent recorded: a person said stop") || shutdowns != 1 {
			t.Fatalf("a recorded cancellation did not end the suite: err = %v, shutdowns = %d", err, shutdowns)
		}
	})
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
	clock := newWatchdogTestClock()
	err := signalSuiteGroup(WatchdogOptions{
		SuiteIdentity: identity.Ref{Pid: 999997, StartedAtSec: started},
		TermGrace:     time.Millisecond, KillGrace: time.Millisecond,
		Signal: func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
		Now:    clock.Now, Sleep: clock.Sleep,
	}, probe)
	if err == nil || !strings.Contains(err.Error(), "kill") || !strings.Contains(err.Error(), "recorded start identity") {
		t.Fatalf("error = %v", err)
	}
	if probe.calls != 4 || fmt.Sprint(signals) != fmt.Sprint([]syscall.Signal{syscall.SIGCONT, syscall.SIGTERM}) {
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
	clock := newWatchdogTestClock()
	err := stopGuardMember(WatchdogOptions{
		TermGrace: time.Millisecond,
		Signal:    func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
		Now:       clock.Now, Sleep: clock.Sleep,
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
	clock := newWatchdogTestClock()
	options := WatchdogOptions{
		Root: root, SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started},
		TermGrace: time.Millisecond,
		Signal:    func(_ int, signal syscall.Signal) error { signals = append(signals, signal); return nil },
		Now:       clock.Now, Sleep: clock.Sleep,
	}
	probe := fixedProbe{exact: identity.Exact{Pid: 999998, StartedAt: time.Unix(started, 0)}, state: identity.Alive}
	if err := sweepExecutionGuard(options, probe); err != nil {
		t.Fatal(err)
	}
	if len(signals) != 3 || signals[0] != syscall.SIGCONT || signals[2] != syscall.SIGKILL {
		t.Fatalf("signals = %v", signals)
	}
	if clock.sleeps == 0 {
		t.Fatal("execution-guard stop did not use the artificial sleeper")
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
	clock := newWatchdogTestClock()
	options := WatchdogOptions{
		Root: root, SuiteIdentity: identity.Ref{Pid: 999999, StartedAtSec: started}, TermGrace: time.Millisecond,
		Signal: func(target int, signal syscall.Signal) error {
			if target == 999991 || target == -999991 {
				return syscall.EPERM
			}
			secondSignals = append(secondSignals, signal)
			return nil
		},
		Now: clock.Now, Sleep: clock.Sleep,
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
