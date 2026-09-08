package proofrun

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

type testCreationClaim struct {
	mu     sync.Mutex
	closed bool
}

func (c *testCreationClaim) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

func (c *testCreationClaim) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

func TestLaunchSuiteWritesBannerProgressAndReapsWatchdog(t *testing.T) {
	root := t.TempDir()
	watchdog := filepath.Join(root, "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	progress := filepath.Join(root, "progress.jsonl")
	logPath := filepath.Join(root, "logs", "suite.log")
	banner := "suite-cost suite=fixture witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log"
	sectionCommand := `printf '{"suite":"fixture","section":"only","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
echo suite-output
printf '{"suite":"fixture","section":"only","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	var output bytes.Buffer
	var errors bytes.Buffer
	result := LaunchSuite(LaunchOptions{
		Suite: "fixture", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"), ProgressPath: progress, LogPath: logPath,
		TmpPaths: []string{filepath.Join(root, "tmp")}, Banner: banner,
		ExpectedSections: []string{"only"}, TwiceConsulted: map[string]bool{},
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: 10 * time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
		WatchdogExecutable: watchdog, Command: []string{"bash", "-c", sectionCommand, "fixture", progress},
		Output: &output, ErrorOutput: &errors,
	})
	if result != 0 {
		t.Fatalf("result = %d, output = %q, errors = %q", result, output.String(), errors.String())
	}
	if !strings.Contains(output.String(), banner+"\n") || !strings.Contains(output.String(), "suite-output") {
		t.Fatalf("output = %q", output.String())
	}
	if _, err := os.Stat(logPath + ".done"); !os.IsNotExist(err) {
		t.Fatalf("launcher did not remove reaped watchdog done file: %v", err)
	}
	record, err := ReadRecord(root, "fixture")
	if err != nil || record.Status != StatusDone || record.FenceGeneration != 0 {
		t.Fatalf("proof-run record = %+v, %v", record, err)
	}
	if record.Root != root || record.Launcher.Ref().Mode() == identity.CompareInvalid ||
		record.SuiteProcess.Ref().Mode() == identity.CompareInvalid || record.Watchdog.Ref().Mode() == identity.CompareInvalid {
		t.Fatalf("proof-run identities are incomplete: %+v", record)
	}
	run, err := ReadLatestProgressRun(progress)
	if err != nil || len(run.Events) != 2 || len(run.Header.TmpPaths) != 1 {
		t.Fatalf("progress run = %+v, %v", run, err)
	}
}

func TestLaunchSuiteRefusesAClosedFenceBeforeStartingAnything(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "started")
	var errors bytes.Buffer
	result := LaunchSuite(LaunchOptions{
		Suite: "closed", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		ProgressPath: filepath.Join(root, "progress.jsonl"), LogPath: filepath.Join(root, "suite.log"),
		Banner: "cost banner", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1, Poll: time.Millisecond,
		TermGrace: time.Millisecond, KillGrace: time.Millisecond,
		Command: []string{"sh", "-c", "touch \"$1\"", "fixture", marker}, ErrorOutput: &errors,
		FenceReader: func(string) (stopfence.Record, error) {
			return stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopped, Generation: 3,
				ChangedAt: "2026-09-07T12:00:00Z", By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 73}}}, nil
		},
	})
	if result != 1 || !strings.Contains(errors.String(), "the metasystem is stopped for "+root+" since 2026-09-07T12:00:00Z, by stop pid 73") ||
		!strings.Contains(errors.String(), "at an agent-free terminal, run: metasystem arm --repo "+root) {
		t.Fatalf("result = %d, errors = %q", result, errors.String())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("closed fence started the suite: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("closed fence published artifacts: %v", err)
	}
}

func TestLaunchSuiteCreationClaimSpansBothStartsAndSecondFenceRead(t *testing.T) {
	root := t.TempDir()
	watchdog := filepath.Join(root, "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	claim := &testCreationClaim{}
	reads := 0
	var errors bytes.Buffer
	result := LaunchSuite(LaunchOptions{
		Suite: "race", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		ProgressPath: filepath.Join(root, "progress.jsonl"), LogPath: filepath.Join(root, "suite.log"),
		Banner: "cost banner", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1, Poll: 5 * time.Millisecond,
		TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: watchdog,
		Command: []string{"bash", "-c", `trap 'exit 0' TERM; while :; do sleep 0.05; done`}, ErrorOutput: &errors,
		FenceReader: func(string) (stopfence.Record, error) {
			reads++
			if reads == 1 {
				return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 7}, nil
			}
			if claim.isClosed() {
				t.Fatal("creation claim closed before the second fence read")
			}
			return stopfence.Record{State: stopfence.StateClosed, Phase: stopfence.PhaseStopping, Generation: 8}, nil
		},
		ClaimCreator: func(_ string, verb string, generation int64, ref identity.Ref) (CreationClaim, error) {
			if verb != "proof-run-launch" || generation != 7 || ref.Pid != int64(os.Getpid()) {
				t.Fatalf("claim = verb %q generation %d ref %+v", verb, generation, ref)
			}
			return claim, nil
		},
	})
	if result != 1 || reads != 2 || !claim.isClosed() {
		t.Fatalf("result = %d, reads = %d, claim closed = %v, errors = %q", result, reads, claim.isClosed(), errors.String())
	}
	if !strings.Contains(errors.String(), "stop unfinished for ") || !strings.Contains(errors.String(), "while the proof run started, it has been ended") || !strings.Contains(errors.String(), "metasystem stop --repo ") {
		t.Fatalf("second-read refusal = %q", errors.String())
	}
	record, err := ReadRecord(root, "race")
	if err != nil || record.Status != StatusDone || record.FenceGeneration != 7 {
		t.Fatalf("proof-run record = %+v, %v", record, err)
	}
	if identity.AliveRef(identity.KernelProber{}, record.SuiteProcess.Ref()) != identity.Dead ||
		identity.AliveRef(identity.KernelProber{}, record.Watchdog.Ref()) != identity.Dead {
		t.Fatalf("creator-race children survived: %+v", record)
	}
}

func TestLaunchHelpersRejectBadInputsAndParseSelectorRows(t *testing.T) {
	var errors bytes.Buffer
	if result := LaunchSuite(LaunchOptions{ErrorOutput: &errors}); result != 2 || !strings.Contains(errors.String(), "required") {
		t.Fatalf("invalid result = %d, errors = %q", result, errors.String())
	}
	root := t.TempDir()
	selector := filepath.Join(root, "selector")
	if err := os.WriteFile(selector, []byte("first\tFirst section\nsecond\tSecond section\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sections, err := ReadSelectorSections(selector)
	if err != nil || strings.Join(sections, ",") != "first,second" {
		t.Fatalf("sections = %v, %v", sections, err)
	}
	if err := os.WriteFile(selector, []byte("invalid\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadSelectorSections(selector); err == nil {
		t.Fatal("invalid selector row passed")
	}
	if exitStatus(nil) != 0 || exitStatus(os.ErrInvalid) != 1 {
		t.Fatal("exit status helper returned an invalid result")
	}
	watchdog := watchdogCommand(LaunchOptions{
		Suite: "fixture", Root: root, ConfPath: "conf", ProgressPath: "progress", LogPath: "log",
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1,
		Poll: time.Second, TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: "watchdog",
	}, identity.Ref{Pid: 7, StartedAtSec: 8}, "done", 11)
	if !strings.Contains(strings.Join(watchdog.Args, " "), "--fence-generation 11") {
		t.Fatalf("watchdog arguments omit the fence generation: %q", watchdog.Args)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}
