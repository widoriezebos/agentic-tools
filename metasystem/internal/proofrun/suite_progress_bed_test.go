package proofrun

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// These tests port the watchdog scenarios of
// scripts/agents/suite-progress-fixtures.sh. The bash bed froze a real suite
// with SIGSTOP and waited for the kernel; here the same decisions run through
// RunWatchdog with an injected prober, signal sender, clock, and evidence
// copier, so the order of preservation and signals is observed exactly.

type suiteProgressSignal struct {
	target int
	signal syscall.Signal
}

// suiteProgressBed writes the progress journal the stopped suite left: a
// header naming its tmp and log evidence, the section start, and the
// supervisor's dead verdict on that still-open section.
func suiteProgressBed(t *testing.T, suite, section string) (root, progress, logPath, tmp string) {
	t.Helper()
	root = t.TempDir()
	tmp = filepath.Join(root, "tmp")
	logPath = filepath.Join(root, "logs", "suite.log")
	for _, dir := range []string{tmp, filepath.Dir(logPath)} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "evidence.txt"), []byte("evidence written before stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("suite output before the stop\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	progress = filepath.Join(root, "progress.jsonl")
	if err := AppendProgressHeader(progress, ProgressHeader{TmpPaths: []string{tmp}, LogPaths: []string{logPath}}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	for _, event := range []SectionEvent{
		{Suite: suite, Section: section, Event: "start", At: at},
		{Suite: suite, Section: section, Event: "verdict", At: at, Verdict: "dead"},
	} {
		if err := AppendSectionEvent(progress, event); err != nil {
			t.Fatal(err)
		}
	}
	return root, progress, logPath, tmp
}

func suiteProgressWatchdogOptions(root, suite, progress, logPath string, suiteRef identity.Ref) WatchdogOptions {
	clock := &watchdogTestClock{now: time.Date(2026, 9, 27, 10, 5, 0, 0, time.UTC)}
	return WatchdogOptions{
		Suite: suite, Root: root, ProgressPath: progress, DonePath: filepath.Join(root, "done"),
		LogPaths: []string{logPath}, SuiteIdentity: suiteRef,
		Silence: 300 * time.Millisecond, SectionCap: 5 * time.Second, EvidenceTimeout: time.Second,
		EvidenceMax: 1 << 20, Poll: 50 * time.Millisecond, TermGrace: 100 * time.Millisecond, KillGrace: 100 * time.Millisecond,
		ErrorOutput: os.Stderr,
		Now:         clock.Now, Sleep: clock.Sleep,
		NewTicker: func(time.Duration) (<-chan time.Time, func()) { return nil, func() {} },
	}
}

// stopped-suite: a suite judged dead in its open section cannot run its own
// cleanup. The watchdog preserves the suite's tmp evidence first, then
// continues, terminates, and kills the suite group, and then sweeps the
// separately detached execution-guard member through the same ladder,
// never signalling the suite itself as a guard member.
func TestSuiteProgressBedStoppedSuiteIsPreservedThenStoppedWithItsDetachedMember(t *testing.T) {
	t.Parallel()
	const suite, section = "stopped", "stopped-section"
	root, progress, logPath, _ := suiteProgressBed(t, suite, section)
	started := time.Date(2026, 9, 27, 9, 59, 0, 0, time.UTC).Unix()
	suiteRef := identity.Ref{Pid: 999990, StartedAtSec: started}
	detachedRef := identity.Ref{Pid: 999980, StartedAtSec: started}
	guard := filepath.Join(root, "artifacts", "agents", "supervision", "gate-runs", "checkout-execution.lock.d")
	if err := os.MkdirAll(guard, 0o700); err != nil {
		t.Fatal(err)
	}
	members := fmt.Sprintf(`{"members":[{"pid":%d,"pidStartedAt":%d},{"pid":%d,"pidStartedAt":%d}]}`,
		suiteRef.Pid, suiteRef.StartedAtSec, detachedRef.Pid, detachedRef.StartedAtSec)
	if err := os.WriteFile(filepath.Join(guard, "owner.json"), []byte(members), 0o600); err != nil {
		t.Fatal(err)
	}

	var signals []suiteProgressSignal
	shutdowns := 0
	signalsAtPreservation := -1
	options := suiteProgressWatchdogOptions(root, suite, progress, logPath, suiteRef)
	options.Prober = pidProbe{started: started}
	options.Signal = func(target int, signal syscall.Signal) error {
		signals = append(signals, suiteProgressSignal{target: target, signal: signal})
		return nil
	}
	options.Shutdown = func() error { shutdowns++; return nil }
	options.PreserveEvidence = func(destination string, sources []string) string {
		signalsAtPreservation = len(signals) + shutdowns
		result, err := PreserveEvidence(destination, sources, options.EvidenceMax)
		if err != nil {
			t.Errorf("preserve evidence: %v", err)
		}
		return fmt.Sprintf("copied %d bytes; dropped %d paths", result.CopiedBytes, len(result.Dropped))
	}

	err := RunWatchdog(options)
	if err == nil || !strings.Contains(err.Error(), "suite stalled in section "+section) ||
		!strings.Contains(err.Error(), "the supervisor judged section "+section+" dead") ||
		!strings.Contains(err.Error(), "evidence preserved before kill at ") {
		t.Fatalf("stopped suite verdict = %v", err)
	}
	if signalsAtPreservation != 0 {
		t.Fatalf("evidence was preserved after %d kill-capable actions, want before any", signalsAtPreservation)
	}
	if shutdowns != 1 {
		t.Fatalf("supervision shutdowns = %d, want 1", shutdowns)
	}

	evidenceDirs, globErr := filepath.Glob(filepath.Join(root, "artifacts", "agents", "suite-failures", "*"))
	if globErr != nil || len(evidenceDirs) != 1 {
		t.Fatalf("suite-failure evidence directories = %v, %v", evidenceDirs, globErr)
	}
	if _, err := os.Stat(filepath.Join(evidenceDirs[0], "copy-note.txt")); err != nil {
		t.Fatalf("external watchdog left no copy note: %v", err)
	}
	var preserved []string
	walkErr := filepath.WalkDir(evidenceDirs[0], func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && entry.Name() == "evidence.txt" {
			data, readErr := os.ReadFile(path)
			if readErr == nil && string(data) == "evidence written before stop\n" {
				preserved = append(preserved, path)
			}
		}
		return err
	})
	if walkErr != nil || len(preserved) != 1 {
		t.Fatalf("external watchdog did not preserve in-suite evidence: found %v, walk error %v", preserved, walkErr)
	}

	group := -int(suiteRef.Pid)
	want := []suiteProgressSignal{
		{target: group, signal: syscall.SIGCONT}, {target: group, signal: syscall.SIGTERM}, {target: group, signal: syscall.SIGKILL},
	}
	if len(signals) != 6 || fmt.Sprint(signals[:3]) != fmt.Sprint(want) {
		t.Fatalf("suite group ladder = %v, want %v then three detached-member signals", signals, want)
	}
	for index, wantSignal := range []syscall.Signal{syscall.SIGCONT, syscall.SIGTERM, syscall.SIGKILL} {
		got := signals[3+index]
		if (got.target != int(detachedRef.Pid) && got.target != -int(detachedRef.Pid)) || got.signal != wantSignal {
			t.Fatalf("detached member signal %d = %+v, want %s to pid %d", index, got, wantSignal, detachedRef.Pid)
		}
	}
}

// recycled-identity: the section carries a dead verdict, but the suite pid
// no longer has its recorded start identity. The watchdog preserves the
// evidence and refuses loudly; neither supervision shutdown nor any signal
// is authorized against the process now holding that pid.
func TestSuiteProgressBedRecycledIdentityRefusesEveryKillAction(t *testing.T) {
	t.Parallel()
	const suite, section = "recycle", "guard"
	root, progress, logPath, _ := suiteProgressBed(t, suite, section)
	suiteRef := identity.Ref{Pid: 999970, StartedAtSec: 1}
	signals, shutdowns, preservations := 0, 0, 0
	options := suiteProgressWatchdogOptions(root, suite, progress, logPath, suiteRef)
	options.Prober = fixedProbe{exact: identity.Exact{Pid: suiteRef.Pid, StartedAt: time.Unix(1_700_000_000, 0)}, state: identity.Alive}
	options.Signal = func(int, syscall.Signal) error { signals++; return nil }
	options.Shutdown = func() error { shutdowns++; return nil }
	options.PreserveEvidence = func(string, []string) string { preservations++; return "bounded copy completed" }

	err := RunWatchdog(options)
	if err == nil || !strings.Contains(err.Error(), "pid 999970 is no longer the suite, so it was not killed") ||
		!strings.Contains(err.Error(), "(dead)") {
		t.Fatalf("recycled-identity refusal = %v", err)
	}
	if signals != 0 || shutdowns != 0 {
		t.Fatalf("recycled identity authorized %d signals and %d shutdowns", signals, shutdowns)
	}
	if preservations != 1 {
		t.Fatalf("evidence preservations = %d, want 1 before the refusal", preservations)
	}
}
