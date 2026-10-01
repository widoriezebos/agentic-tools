package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	goalbranch "github.com/widoriezebos/agentic-tools/metasystem/internal/goal/branch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lease"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/registry"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testutil"
)


func waitForPortableOwnerLease(t *testing.T, root string, pid int, exited <-chan struct{}, exitErr func() error) {
	t.Helper()
	deadline, bounded := t.Deadline()
	for {
		current, err := lease.CurrentHolder(root)
		if err == nil && current.Pid == int64(pid) {
			return
		}
		select {
		case <-exited:
			t.Fatalf("landing owner %d exited before acquiring the checkout: %v", pid, exitErr())
		case <-t.Context().Done():
			t.Fatalf("landing owner %d did not acquire the checkout before test cancellation: %v", pid, t.Context().Err())
		default:
		}
		if bounded && !time.Now().Before(deadline) {
			t.Fatalf("landing owner %d did not acquire the checkout before the test deadline: %v", pid, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// landingBatchOwnerParkEnv names a directory the test-helper landing owner
// parks in: after it holds its lease and before any registry or store work
// it writes "parked" (holding the machine store registry's lock path) and
// waits for "release". A test pauses the owner only there, a quiet point: a
// SIGSTOP the moment the lease appeared could land inside
// diskstore.Registry.Register and hold ~/.metasystem/stores/.register.lock,
// blocking every other process's registration (fencedflake C3, VM).
const landingBatchOwnerParkEnv = "METASYSTEM_TEST_BATCH_OWNER_PARK"

func parkBatchOwnerForFixture() error {
	directory := os.Getenv(landingBatchOwnerParkEnv)
	if directory == "" {
		return nil
	}
	armed, err := registry.DefaultPath()
	if err != nil {
		return fmt.Errorf("fixture park: %w", err)
	}
	lockPath := filepath.Join(filepath.Dir(armed), "stores", ".register.lock")
	staging := filepath.Join(directory, "parked.tmp")
	if err := os.WriteFile(staging, []byte(lockPath), 0o600); err != nil {
		return fmt.Errorf("fixture park: %w", err)
	}
	if err := os.Rename(staging, filepath.Join(directory, "parked")); err != nil {
		return fmt.Errorf("fixture park: %w", err)
	}
	for attempt := 0; attempt < 36000; attempt++ {
		if _, err := os.Stat(filepath.Join(directory, "release")); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("fixture park: never released")
}

// pauseParkedBatchOwner waits for the owner to park, then stops it while this
// test holds the machine store registry's lock: at the instant of the stop
// the owner provably holds no registration critical section.
func pauseParkedBatchOwner(t *testing.T, directory string, owner *exec.Cmd, exited <-chan struct{}) {
	t.Helper()
	var lockPath string
	for attempt := 0; ; attempt++ {
		if data, err := os.ReadFile(filepath.Join(directory, "parked")); err == nil {
			lockPath = string(data)
			break
		}
		select {
		case <-exited:
			t.Fatal("the landing owner exited before it parked")
		default:
		}
		if attempt > 6000 {
			t.Fatal("the landing owner did not park after taking its lease")
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := os.MkdirAll(filepath.Dir(lockPath), 0o700); err != nil {
		t.Fatal(err)
	}
	guard, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	for attempt := 0; ; attempt++ {
		err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("lock the store registry: %v", err)
		}
		if attempt > 3000 {
			t.Fatalf("the store registry lock %s stayed held; the owner would be stopped inside a registration", lockPath)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err := owner.Process.Signal(syscall.SIGSTOP); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_UN); err != nil {
		t.Fatal(err)
	}
}

func holdPortableFixtureSlots(t *testing.T, root string, max int, slots *[]*os.File) {
	t.Helper()
	directory, selected, err := proofrun.FixtureHostAdmissionDirectory(root)
	if err != nil || !selected {
		t.Fatalf("capacity fixture has no authenticated private admission namespace: selected=%t err=%v", selected, err)
	}
	guard, err := os.OpenFile(filepath.Join(directory, "admission.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer guard.Close()
	deadline, bounded := t.Deadline()
	for {
		if err := syscall.Flock(int(guard.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			break
		} else if !errors.Is(err, syscall.EWOULDBLOCK) {
			t.Fatalf("lock private host admission guard: %v", err)
		}
		if bounded && !time.Now().Before(deadline) {
			t.Fatal("private host admission guard did not become available before test deadline")
		}
		select {
		case <-t.Context().Done():
			t.Fatalf("private host admission guard did not become available: %v", t.Context().Err())
		default:
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer syscall.Flock(int(guard.Fd()), syscall.LOCK_UN)
	for index := range max {
		path := filepath.Join(directory, fmt.Sprintf("slot-%02d", index))
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
			_ = file.Close()
			t.Fatalf("private host slot %d was not free behind named gate: %v", index, err)
		}
		*slots = append(*slots, file)
	}
}



func requirePortablePrefixTimestampEvidence(t *testing.T, controlRoot string, cancelled proofrun.Attempt, landed batch.Record, cancelClock time.Time, survivorClocks []time.Time) {
	t.Helper()
	cancelledIdentity := cancelled.TestOwned["app-a"]
	if cancelledIdentity == "" || cancelled.TestOwned["app-b"] == "" || cancelled.TestOwned["app-b"] == cancelledIdentity {
		t.Fatalf("cancelled union does not retain distinct A and B ownership: %+v", cancelled.TestOwned)
	}
	if cancelled.Terminal == nil || cancelled.Terminal.Result != proofrun.TerminalCancelled {
		t.Fatalf("cancelled union has no cancellation terminal: %+v", cancelled.Terminal)
	}
	cancelledAt, err := time.Parse(time.RFC3339Nano, cancelled.Terminal.At)
	if err != nil || !cancelledAt.Equal(cancelClock) {
		t.Fatalf("cancelled union terminal is outside its governing fixture clock: at=%q want=%s err=%v", cancelled.Terminal.At, cancelClock.Format(time.RFC3339Nano), err)
	}

	if landed.Proof == nil || landed.Proof.Status != "green" || landed.Proof.GroupIdentities["app-a"] != cancelledIdentity {
		t.Fatalf("survivor proof is not the matching green app-a tip: cancelled=%s proof=%+v", cancelledIdentity, landed.Proof)
	}
	reusedProducerID, reusePresent := landed.Proof.Reuse["app-a"]
	executed := false
	for _, groupID := range landed.Proof.Executions {
		if groupID == "app-a" {
			executed = true
			break
		}
	}
	if reusePresent && reusedProducerID == "" {
		t.Fatalf("survivor proof has empty app-a reuse provenance: %+v", landed.Proof)
	}
	if reusedProducerID != "" && executed {
		t.Fatalf("survivor proof both reused and executed app-a: %+v", landed.Proof)
	}
	selectorStatus := "reused"
	selectedProducerID := reusedProducerID
	if selectedProducerID == "" {
		if !executed || landed.Proof.AttemptID == "" {
			t.Fatalf("survivor proof has no app-a producer: %+v", landed.Proof)
		}
		selectorStatus = "executed"
		selectedProducerID = landed.Proof.AttemptID
	}
	producer, err := proofrun.ReadAttempt(controlRoot, selectedProducerID)
	if err != nil {
		t.Fatalf("read selected app-a producer %s: %v", selectedProducerID, err)
	}
	if producer.AttemptID != selectedProducerID || producer.Terminal == nil || producer.Terminal.Result != proofrun.TerminalSuccess || producer.TestResult == nil {
		t.Fatalf("selected app-a producer is not a terminal native green: %+v", producer)
	}
	if cancelled.FreshnessEpisode != "" || cancelled.FreshnessBinding != "" || cancelled.FreshnessExpiresAt != "" ||
		producer.FreshnessEpisode != "" || producer.FreshnessBinding != "" || producer.FreshnessExpiresAt != "" ||
		!proofrun.MatchesTestResultFreshness(cancelled, *producer.TestResult, "app-a") ||
		!proofrun.MatchesTestResultFreshness(producer, *producer.TestResult, "app-a") {
		t.Fatalf("ordinary app-a proof has inconsistent freshness: cancelled=(%q,%q,%q) producer=(%q,%q,%q)",
			cancelled.FreshnessEpisode, cancelled.FreshnessBinding, cancelled.FreshnessExpiresAt,
			producer.FreshnessEpisode, producer.FreshnessBinding, producer.FreshnessExpiresAt)
	}

	var green proofrun.GroupResult
	for _, group := range producer.TestResult.Groups {
		if group.ID == "app-a" {
			green = group
			break
		}
	}
	if green.ID == "" || green.Status != "passed" || !green.CollectionComplete || !proofrun.NativeTestProducer(producer, green) ||
		green.ExecutionIdentity != cancelledIdentity || producer.TestOwned["app-a"] != cancelledIdentity {
		t.Fatalf("selected app-a record is not the matching owned native green: cancelled=%s producer-owned=%s group=%+v proof=%+v",
			cancelledIdentity, producer.TestOwned["app-a"], green, landed.Proof)
	}
	startedAt, startErr := time.Parse(time.RFC3339Nano, producer.StartedAt)
	groupStartedAt, groupStartErr := time.Parse(time.RFC3339Nano, green.StartedAt)
	groupEndedAt, groupEndErr := time.Parse(time.RFC3339Nano, green.EndedAt)
	producerEndedAt, producerEndErr := time.Parse(time.RFC3339Nano, producer.Terminal.At)
	if startErr != nil || groupStartErr != nil || groupEndErr != nil || producerEndErr != nil {
		t.Fatalf("selected app-a timestamps are malformed: attempt-start=%q group=(%q,%q) terminal=%q errors=(%v,%v,%v,%v)",
			producer.StartedAt, green.StartedAt, green.EndedAt, producer.Terminal.At, startErr, groupStartErr, groupEndErr, producerEndErr)
	}
	clockMatched := false
	for _, at := range survivorClocks {
		if startedAt.Equal(at) && groupStartedAt.Equal(at) && groupEndedAt.Equal(at) && producerEndedAt.Equal(at) {
			clockMatched = true
			break
		}
	}
	if !clockMatched || !groupEndedAt.After(cancelledAt) {
		t.Fatalf("selector timestamps do not retain the safe fixture-clock order: cancelled=%s producer-start=%s group=(%s,%s) producer-end=%s clocks=%v",
			cancelledAt.Format(time.RFC3339Nano), startedAt.Format(time.RFC3339Nano), groupStartedAt.Format(time.RFC3339Nano),
			groupEndedAt.Format(time.RFC3339Nano), producerEndedAt.Format(time.RFC3339Nano), survivorClocks)
	}
	if green.DurationMS < 0 || green.CPUSeconds < 0 {
		t.Fatalf("selected app-a physical observations are invalid: duration-ms=%d cpu-seconds=%f", green.DurationMS, green.CPUSeconds)
	}

	marker := struct {
		SchemaVersion       int     `json:"schemaVersion"`
		HistoricalR2Stamps  string  `json:"historicalR2Stamps"`
		Component           string  `json:"component"`
		ExecutionIdentity   string  `json:"executionIdentity"`
		CancelledAttempt    string  `json:"cancelledAttempt"`
		CancelledTerminalAt string  `json:"cancelledTerminalAt"`
		NativeGreenAttempt  string  `json:"nativeGreenAttempt"`
		NativeGreenStarted  string  `json:"nativeGreenStartedAt"`
		NativeGreenEnded    string  `json:"nativeGreenEndedAt"`
		PhysicalDurationMS  int64   `json:"physicalDurationMs"`
		PhysicalCPUSeconds  float64 `json:"physicalCpuSeconds"`
		FreshnessEpisode    string  `json:"freshnessEpisode"`
		FreshnessBinding    string  `json:"freshnessBinding"`
		FreshnessExpiresAt  string  `json:"freshnessExpiresAt"`
		SelectorOrder       string  `json:"selectorOrder"`
		SelectorStatus      string  `json:"selectorStatus"`
		SelectedAttempt     string  `json:"selectedAttempt"`
	}{
		SchemaVersion: 1, HistoricalR2Stamps: "unavailable", Component: "app-a", ExecutionIdentity: cancelledIdentity,
		CancelledAttempt: cancelled.AttemptID, CancelledTerminalAt: cancelled.Terminal.At,
		NativeGreenAttempt: producer.AttemptID, NativeGreenStarted: green.StartedAt, NativeGreenEnded: green.EndedAt,
		PhysicalDurationMS: green.DurationMS, PhysicalCPUSeconds: green.CPUSeconds,
		FreshnessEpisode: producer.FreshnessEpisode, FreshnessBinding: producer.FreshnessBinding, FreshnessExpiresAt: producer.FreshnessExpiresAt,
		SelectorOrder: "cancelled-terminal-at<green-group-ended-at", SelectorStatus: selectorStatus, SelectedAttempt: selectedProducerID,
	}
	encoded, err := json.Marshal(marker)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("TB-CMD003-EVIDENCE %s", encoded)
}

func portableGoalBranch(t *testing.T, bed *batchE2EFixture, seat, goalID, input string) {
	t.Helper()
	base := batchE2EGit(t, seat, "rev-parse", "origin/main")
	path := "app/" + input + ".txt"
	if err := os.WriteFile(filepath.Join(seat, filepath.FromSlash(path)), []byte("green "+input+" goal contribution\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, seat, "add", "--", path)
	commit, err := goalbranch.CommitStaged(goalbranch.CommitRequest{Repo: seat, Remote: "origin", EndpointTip: base,
		GoalID: goalID, Unit: "u1", OpID: "build-" + goalID, Kind: goalbranch.Unit, CheckClaim: func() error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	subject, present, err := dispatchcore.ComputeReadSubject(dispatchcore.ReadSubjectRequest{RepoRoot: seat, Role: "code-critic", Reviews: "commit:" + commit})
	if err != nil || !present {
		t.Fatalf("compute %s read subject: present=%t error=%v", goalID, present, err)
	}
	job := "critic-" + goalID
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", "jobs", job+".json"), map[string]any{
		"jobId": job, "goalId": nil, "role": "code-critic", "round": 1, "status": "completed", "chainClosed": true,
		"findingRegister": []any{}, "findingRegisterRound": 1, "findingRegisterSubjectDigest": subject.Digest(),
		"closure": map[string]any{"criticRoot": job, "round": 1, "subject": subject, "mechanism": "clean"},
	})
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", job, "rounds", "1", "subject.json"), subject)
	bed.writeJSON(filepath.Join(seat, "artifacts", "agents", job, "rounds", "1", "return.json"), map[string]any{
		"jobId": job, "round": 1, "reviewedTree": subject.Tree,
	})
	if _, _, err := goalbranch.CommitRead(goalbranch.CommitReadRequest{Repo: seat, Remote: "origin", EndpointTip: base,
		GoalID: goalID, Unit: "u1", OpID: "read-" + goalID, RootJob: job, GateRunID: "fast-" + goalID,
		GateTree: subject.Tree, CheckClaim: func() error { return nil }}); err != nil {
		t.Fatal(err)
	}
	batchE2EGit(t, seat, "push", "-q", "origin", "refs/heads/goal/"+goalID+":refs/heads/goal/"+goalID)
	if tip := batchE2EGit(t, bed.origin, "rev-parse", "refs/heads/goal/"+goalID); tip == base {
		t.Fatalf("%s goal branch did not advance origin", goalID)
	}
}

// startPortableStewardRunner starts the enrolled engine's resident runner as
// steward arm does (steward run in its own session) with its terminal fact
// staged false: a shell gate holds the process until its pid's fact is in
// the fixture identity table, then execs the runner in that same process, so
// the classifier never reads the kernel's terminal for it.
func startPortableStewardRunner(t *testing.T, engine, repo string, environment []string) (*exec.Cmd, string, <-chan error) {
	t.Helper()
	dir := t.TempDir()
	table := filepath.Join(dir, "runner-identity.json")
	log := filepath.Join(dir, "runner.log")
	output, err := os.Create(log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = output.Close() })
	runner := exec.Command("/bin/sh", "-c", `read -r gate && exec "$0" "$@"`, engine, "steward", "run", "--repo", repo, "--lineage", "no-lease")
	runner.Dir = repo
	runner.Env = append(append([]string(nil), environment...), "METASYSTEM_FAKE_PROCESS_IDENTITY_FILE="+table)
	runner.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	runner.Stdout, runner.Stderr = output, output
	gate, err := runner.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	pid := 0
	testenv.ReapFixtureProcessGroups(t, []testenv.FixtureProcessGroup{{
		Verb:    "portable steward runner",
		Resolve: func() (int, bool, error) { return pid, pid != 0, nil },
	}})
	if err := runner.Start(); err != nil {
		t.Fatal(err)
	}
	pid = runner.Process.Pid
	exited := make(chan error, 1)
	go func() { exited <- runner.Wait() }()
	if err := os.WriteFile(table, fmt.Appendf(nil, `{"%d": {"terminal": false}}`, runner.Process.Pid), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := gate.Write([]byte("go\n")); err != nil {
		t.Fatalf("release the steward runner gate: %v", err)
	}
	if err := gate.Close(); err != nil {
		t.Fatal(err)
	}
	return runner, log, exited
}

// stopPortableStewardRunner ends the runner's session and waits for its exit.
func stopPortableStewardRunner(t *testing.T, runner *exec.Cmd, exited <-chan error) {
	t.Helper()
	if err := syscall.Kill(-runner.Process.Pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("stop the steward runner: %v", err)
	}
	select {
	case <-exited:
	case <-t.Context().Done():
		t.Fatalf("the steward runner did not exit after SIGTERM before test cancellation")
	}
}

func readPortableRunnerLog(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	return string(data)
}

// landingBatchHelperCommand runs the landing batch owner, a one-shot tick or
// a join in a child of this test binary. The engine has no landing batch
// entry any more (production runs the owner as the supervised landing-owner
// component, and work land joins in process); these beds keep each owner in
// a process of its own so it holds the landing lease under its own pid, can
// be stopped with SIGSTOP and killed, and a successor tick recovers from the
// persisted record alone. The child re-executes the enrolled fixture engine
// for tree plans and prefix receipts, as the former engine entry did with its
// own executable, and commits through the planted commit script, as that
// plantedcommit engine did.
const landingBatchHelperCommand = "test-helper-landing-batch"

const landingBatchHelperEngine = "METASYSTEM_TEST_LANDING_BATCH_ENGINE"

func init() {
	testHelperCommands[landingBatchHelperCommand] = func(args []string) int {
		if engine := os.Getenv(landingBatchHelperEngine); engine != "" {
			executable := func() (string, error) { return engine, nil }
			batchowner.BatchTreePlanExecutable, batchowner.BatchPrefixReceiptExecutable = executable, executable
		}
		// The enrolled fixture engine is a plantedcommit build; the child
		// commits each unit through the bed's planted commit script as it did.
		batchowner.BatchCommitBoundary = plantedOrLandingCommit
		return runLandingBatch(args, os.Stdout, os.Stderr)
	}
}

func landingBatchChild(fixture proofBinaryFixture, environment []string, engine string, args ...string) *exec.Cmd {
	fixture.t.Helper()
	command := fixture.command(environment, commandTestExecutable(fixture.t), append([]string{landingBatchHelperCommand}, args...)...)
	command.Env = append(command.Env, "GO_WANT_BATCH_E2E_COMMAND=1", landingBatchHelperEngine+"="+engine)
	return command
}
