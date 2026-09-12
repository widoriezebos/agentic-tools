package proofrun

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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

func (c *testCreationClaim) Path() string { return "fixture-creation-claim" }

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

func TestProofFinalizationStopRace(t *testing.T) {
	executionRoot, proofIdentity := proofAttemptFixture(t, "finalization-race")
	controlRoot := t.TempDir()
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(AdmissionRequest{
		ControlRoot: controlRoot, ExecutionRoot: executionRoot, GoalID: "goal-a", GoalRevision: 2,
		AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	watchdog := filepath.Join(t.TempDir(), "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	claim := &testCreationClaim{}
	progress := filepath.Join(controlRoot, "progress.jsonl")
	logPath := filepath.Join(controlRoot, "suite.log")
	var sawFinalizingInventory bool
	result := LaunchSuite(LaunchOptions{
		Suite: "finalization-race", Root: executionRoot, ControlRoot: controlRoot, AttemptID: attempt.AttemptID,
		Deadline: now.Add(2 * time.Minute), ConfPath: filepath.Join(executionRoot, "metasystem.conf"),
		ProgressPath: progress, LogPath: logPath, Banner: "controlled finalization race",
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: 10 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second,
		WatchdogExecutable: watchdog, Command: []string{"bash", "-c", "echo controlled-proof-body"},
		FenceReader: func(string) (stopfence.Record, error) {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 4}, nil
		},
		ClaimCreator: func(string, string, int64, identity.Ref) (CreationClaim, error) { return claim, nil },
		CommitTerminal: func(completion CompletionContext, _ json.RawMessage) error {
			records, readErr := ReadRecords(controlRoot)
			if readErr != nil || len(records) != 1 || records[0].Status != StatusRunning ||
				records[0].Key() != completion.RecordKey || records[0].Launcher.Ref() != launcher.Ref() {
				t.Fatalf("finalizing proof was absent from live process inventory: records=%+v err=%v", records, readErr)
			}
			sawFinalizingInventory = true
			if err := RequestCancellation(controlRoot, attempt.AttemptID, "isolated stop won terminal ordering"); err != nil {
				t.Fatal(err)
			}
			return errors.New("stop won before terminal success")
		},
	})
	if result != 1 || !sawFinalizingInventory {
		t.Fatalf("winning stop result=%d inventory=%v", result, sawFinalizingInventory)
	}
	stopped, err := ReadAttempt(controlRoot, attempt.AttemptID)
	if err != nil || stopped.Terminal != nil || stopped.CancellationIntent == "" {
		t.Fatalf("stop race wrote success or lost cancellation: attempt=%+v err=%v", stopped, err)
	}
	if _, err := FinalizeAttempt(controlRoot, attempt.AttemptID, TerminalCancelled, 1, "isolated stop joined", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
	record, err := ReadProcessRecord(controlRoot, stopped.ProcessKeys[0])
	if err != nil || record.Status != StatusRunning {
		t.Fatalf("process record stopped being live before durable terminal join: record=%+v err=%v", record, err)
	}
	if err := markDone(controlRoot, record, launcher.Ref()); err != nil {
		t.Fatal(err)
	}
	terminal, err := ReadAttempt(controlRoot, attempt.AttemptID)
	if err != nil || terminal.Terminal == nil || terminal.Terminal.Result != TerminalCancelled {
		t.Fatalf("winning stop did not prevent success: attempt=%+v err=%v", terminal, err)
	}
}

func TestProofCancellationBeforeChildCreationStartsNoChild(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "cancel-before-child")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: proofIdentity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	if err := RequestCancellation(root, attempt.AttemptID, "controlled pre-child cancellation"); err != nil {
		t.Fatal(err)
	}
	count := filepath.Join(root, "child-started")
	claim := &testCreationClaim{}
	result := LaunchSuite(LaunchOptions{Suite: "cancel-before-child", Root: root, ControlRoot: root,
		AttemptID: attempt.AttemptID, Deadline: now.Add(2 * time.Minute), ConfPath: filepath.Join(root, "metasystem.conf"),
		ProgressPath: filepath.Join(root, "progress.jsonl"), LogPath: filepath.Join(root, "proof.log"), Banner: "cancel before child",
		Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: 10 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second,
		FenceReader: func(string) (stopfence.Record, error) {
			return stopfence.Record{State: stopfence.StateOpen, Phase: stopfence.PhaseArmed, Generation: 4}, nil
		},
		ClaimCreator: func(string, string, int64, identity.Ref) (CreationClaim, error) { return claim, nil },
		Command:      []string{"bash", "-c", "printf started >\"$1\"", "fixture", count},
	})
	if result != 1 || !claim.isClosed() {
		t.Fatalf("cancelled launch result=%d claimClosed=%v", result, claim.isClosed())
	}
	if _, err := os.Stat(count); !os.IsNotExist(err) {
		t.Fatalf("cancelled reservation started its proof child: %v", err)
	}
	records, err := ReadRecords(root)
	if err != nil || len(records) != 0 {
		t.Fatalf("cancelled reservation published process records: records=%+v err=%v", records, err)
	}
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalCancelled, 1, "controlled cleanup", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

func TestJoinedLaunchUsesItsOwnInputParity(t *testing.T) {
	testJoinedLaunchInputParity(t, "parent-validation")
}

func TestJoinedSharedLaunchUsesEngineIdentityForShellCommand(t *testing.T) {
	testJoinedLaunchInputParity(t, "testing")
}

func testJoinedLaunchInputParity(t *testing.T, commandClass string) {
	controlRoot, parentIdentity := proofAttemptFixture(t, commandClass)
	childRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(childRoot, "metasystem.conf"), []byte("dispatch.cap-max=120\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(childRoot, "fixture.txt"), []byte("different nested fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: controlRoot, ExecutionRoot: controlRoot, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 2, Identity: parentIdentity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	watchdog := filepath.Join(controlRoot, "joined-watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.01; done
`)
	launch := func(suite string, command []string) int {
		return LaunchSuite(LaunchOptions{Suite: suite, Root: childRoot, ControlRoot: controlRoot, AttemptID: attempt.AttemptID,
			JoinedAttempt: true, Deadline: now.Add(2 * time.Minute), ConfPath: filepath.Join(childRoot, "metasystem.conf"),
			ProgressPath: filepath.Join(controlRoot, suite+".progress.jsonl"), LogPath: filepath.Join(controlRoot, suite+".log"),
			Banner: "joined fixture", Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
			Poll: 5 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: watchdog,
			Command: command, ErrorOutput: io.Discard})
	}
	if result := launch("joined-different-root", []string{"env", "JOINED_FIXTURE=1", "true"}); result != 0 {
		t.Fatalf("byte-different nested fixture was compared with parent identity: exit=%d", result)
	}
	mutation := "printf 'mutated\\n' > fixture.txt"
	if commandClass == "testing" {
		mutation = "printf 'dispatch.cap-max=121\n' > metasystem.conf"
	}
	if result := launch("joined-mutated-root", []string{"sh", "-c", mutation}); result == 0 {
		t.Fatal("joined child mutation passed its own before/after parity check")
	}
	stoppedChild := filepath.Join(childRoot, "stopped-child-started")
	if err := RequestCancellation(controlRoot, attempt.AttemptID, "joined parent stop"); err != nil {
		t.Fatal(err)
	}
	if result := launch("joined-stopped", []string{"sh", "-c", "printf started >\"$1\"", "fixture", stoppedChild}); result == 0 {
		t.Fatal("joined child ignored the parent cancellation")
	}
	if _, err := os.Stat(stoppedChild); !os.IsNotExist(err) {
		t.Fatalf("joined stopped child started despite the parent cancellation: %v", err)
	}
	attempts, err := ReadAttempts(controlRoot)
	if err != nil || len(attempts) != 1 {
		t.Fatalf("joined launches changed the one-reservation accounting: attempts=%d err=%v", len(attempts), err)
	}
	if _, err := FinalizeAttempt(controlRoot, attempt.AttemptID, TerminalCancelled, 1, "joined parent stop", nil, time.Now().UTC()); err != nil {
		t.Fatal(err)
	}
}

// A top-level receipt whose preparation outlives its reservation still
// commits its success: the deadline is a horizon, not a kill rule, and the
// minutes it ran are accounted (decision 3 of the hang-detection design).
func TestPreparationCrossingTheReservationStillCommitsSuccess(t *testing.T) {
	root, proofIdentity := proofAttemptFixture(t, "deadline-preparation")
	launcher, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Add(-59*time.Second - 500*time.Millisecond)
	attempt, _, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a",
		GoalRevision: 2, AccountingRevision: 2, ReservedMinutes: 1, Identity: proofIdentity, Launcher: launcher, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := time.Parse(time.RFC3339Nano, attempt.Deadline)
	if err != nil {
		t.Fatal(err)
	}
	watchdog := filepath.Join(root, "artifacts", "deadline-watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.005; done
`)
	result := LaunchSuite(LaunchOptions{Suite: "deadline-preparation", Root: root, ControlRoot: root, AttemptID: attempt.AttemptID,
		Deadline: deadline, ConfPath: filepath.Join(root, "metasystem.conf"), ProgressPath: filepath.Join(root, "artifacts", "deadline.progress.jsonl"),
		LogPath: filepath.Join(root, "artifacts", "deadline.log"), Banner: "deadline fixture", Silence: time.Second, SectionCap: time.Second,
		EvidenceTimeout: time.Second, EvidenceMax: 1024, Poll: 5 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second,
		WatchdogExecutable: watchdog, Command: []string{"true"}, ErrorOutput: io.Discard,
		PrepareSuccess: func(CompletionContext) (json.RawMessage, error) {
			// Preparation outlives the deadline: it waits for that fact.
			for !time.Now().After(deadline) {
				time.Sleep(5 * time.Millisecond)
			}
			return json.RawMessage(`{"prepared":true}`), nil
		}})
	if result != 0 {
		t.Fatalf("a top-level receipt that outlived its reservation was refused: result %d", result)
	}
	stored, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || stored.Terminal == nil || stored.Terminal.Result != TerminalSuccess || stored.ObservedMinutes <= stored.ReservedMinutes {
		t.Fatalf("the overrun was not committed and accounted: attempt=%+v err=%v", stored, err)
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

func TestLegacyWorkerRequiresLiveProcessRecord(t *testing.T) {
	root := t.TempDir()
	process, err := CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	record := Record{Suite: "legacy-worker", Root: root, Launcher: process, SuiteProcess: process, Watchdog: process, Status: StatusRunning}
	if err := writeRecord(record); err != nil {
		t.Fatal(err)
	}
	if err := AuthenticateWorker(root, "", record.Suite, "", int64(os.Getpid())); err != nil {
		t.Fatalf("live legacy worker was not authenticated: %v", err)
	}
	if err := markDone(root, record, process.Ref()); err != nil {
		t.Fatal(err)
	}
	if err := AuthenticateWorker(root, "", record.Suite, "", int64(os.Getpid())); err == nil {
		t.Fatal("completed legacy process record remained worker authority")
	}
}

func TestLaunchSuiteAuthenticatesLegacyWorker(t *testing.T) {
	if os.Getenv("GO_WANT_LEGACY_PROOF_WORKER") == "1" {
		if os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT") != os.Getenv("LEGACY_PROOF_EXPECTED_ROOT") ||
			os.Getenv("METASYSTEM_PROOF_ATTEMPT") != "" || os.Getenv("METASYSTEM_PROOF_RUN_ROOT") != "" ||
			os.Getenv("METASYSTEM_PROOF_RUN_ID") != "" {
			fmt.Fprintln(os.Stderr, "legacy child inherited a contradictory proof locator")
			os.Exit(97)
		}
		err := AuthenticateWorker(os.Getenv("METASYSTEM_PROOF_CONTROL_ROOT"), os.Getenv("METASYSTEM_PROOF_ATTEMPT"),
			os.Getenv("METASYSTEM_PROOF_RECORD_KEY"), os.Getenv("METASYSTEM_PROOF_CREATION_CLAIM"), int64(os.Getppid()))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(97)
		}
		if err := os.WriteFile(os.Getenv("LEGACY_PROOF_WORKER_MARKER"), []byte("authorized\n"), 0o600); err != nil {
			os.Exit(97)
		}
		os.Exit(0)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "artifacts"), 0o755); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root, "artifacts", "legacy-worker-authorized")
	watchdog := filepath.Join(root, "artifacts", "legacy-worker-watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.005; done
`)
	t.Setenv("GO_WANT_LEGACY_PROOF_WORKER", "1")
	t.Setenv("LEGACY_PROOF_WORKER_MARKER", marker)
	t.Setenv("LEGACY_PROOF_EXPECTED_ROOT", root)
	// Model the exact environment in which the full validator runs package
	// tests. The legacy launcher must replace every enclosing proof locator
	// with the context it owns.
	t.Setenv("METASYSTEM_PROOF_CONTROL_ROOT", filepath.Join(root, "foreign-control"))
	t.Setenv("METASYSTEM_PROOF_ATTEMPT", "proof-inherited")
	t.Setenv("METASYSTEM_PROOF_RECORD_KEY", "inherited-record")
	t.Setenv("METASYSTEM_PROOF_CREATION_CLAIM", "inherited-claim")
	t.Setenv("METASYSTEM_PROOF_AUTH_BIN", "inherited-engine")
	t.Setenv("METASYSTEM_PROOF_RUN_ROOT", filepath.Join(root, "foreign-governed"))
	t.Setenv("METASYSTEM_PROOF_RUN_ID", "foreign-governed")
	result := LaunchSuite(LaunchOptions{Suite: "legacy-worker-launch", Root: root, ConfPath: filepath.Join(root, "metasystem.conf"),
		ProgressPath: filepath.Join(root, "artifacts", "legacy.progress.jsonl"), LogPath: filepath.Join(root, "artifacts", "legacy.log"),
		Banner: "legacy worker fixture", Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
		Poll: 5 * time.Millisecond, TermGrace: time.Second, KillGrace: time.Second, WatchdogExecutable: watchdog,
		Command: []string{os.Args[0], "-test.run=^TestLaunchSuiteAuthenticatesLegacyWorker$"}, ErrorOutput: os.Stderr})
	if result != 0 {
		t.Fatalf("legacy worker launch exit=%d", result)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "authorized\n" {
		t.Fatalf("legacy launch did not authenticate its worker: marker=%q err=%v", data, err)
	}
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		t.Fatal(err)
	}
}

func TestLaunchSuiteKeepsTheWatchdogsLastLine(t *testing.T) {
	// The watchdog writes its verdict last and exits at once. exec.Cmd.Wait
	// closes the pipes when the process exits, so a launcher that waited for
	// the process before draining lost that line one launch in five
	// (2026-09-12, the suite-progress fixture's chatty scenario). The
	// launcher drains the watchdog's pipes first; twenty-five launches must
	// all carry the line.
	root := t.TempDir()
	watchdog := filepath.Join(root, "watchdog.sh")
	writeExecutable(t, watchdog, `#!/usr/bin/env bash
done_path=
while (($#)); do
  if [[ "$1" == --done ]]; then done_path=$2; shift 2; else shift; fi
done
while [[ ! -e "$done_path" ]]; do sleep 0.001; done
for ((i = 0; i < 3000; i++)); do echo "suite watchdog: cleanup line $i" >&2; done
echo "suite watchdog: the verdict written last" >&2
exit 1
`)
	banner := "suite-cost suite=fixture witness=armed duration=minutes heartbeat=progress.jsonl logs=logs/suite.log"
	sectionCommand := `printf '{"suite":"fixture","section":"only","event":"start","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"
printf '{"suite":"fixture","section":"only","event":"end","at":"%s","depth":0}\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$1"`
	for round := 0; round < 25; round++ {
		bed := filepath.Join(root, fmt.Sprintf("round-%02d", round))
		if err := os.MkdirAll(bed, 0o755); err != nil {
			t.Fatal(err)
		}
		progress := filepath.Join(bed, "progress.jsonl")
		var output, errors bytes.Buffer
		result := LaunchSuite(LaunchOptions{
			Suite: "fixture", Root: bed, ConfPath: filepath.Join(bed, "metasystem.conf"), ProgressPath: progress, LogPath: filepath.Join(bed, "logs", "suite.log"),
			TmpPaths: []string{filepath.Join(bed, "tmp")}, Banner: banner,
			ExpectedSections: []string{"only"}, TwiceConsulted: map[string]bool{},
			Silence: time.Second, SectionCap: time.Second, EvidenceTimeout: time.Second, EvidenceMax: 1024,
			Poll: 10 * time.Millisecond, TermGrace: time.Millisecond, KillGrace: time.Millisecond,
			WatchdogExecutable: watchdog, Command: []string{"bash", "-c", sectionCommand, "fixture", progress},
			Output: &output, ErrorOutput: &errors,
		})
		if result != 1 {
			t.Fatalf("round %d: result = %d (the watchdog exits 1), errors = %q", round, result, errors.String())
		}
		if !strings.Contains(errors.String(), "suite watchdog: the verdict written last\n") || !strings.Contains(errors.String(), "watchdog ended: exit status 1") {
			t.Fatalf("round %d lost the watchdog's last line: errors = %q", round, errors.String())
		}
	}
}
