package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	runpkg "github.com/widoriezebos/agentic-tools/metasystem/internal/run"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/strictjson"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
	"golang.org/x/sys/unix"
)

func init() {
	testHelperCommands["proof-run"] = func(args []string) int {
		if len(args) > 2 && args[0] == "watchdog" && args[1] == "--suite" && args[2] == "testing" {
			if path := os.Getenv("MANAGED_TEST_DEADLINE_ARGS"); path != "" {
				data, err := json.Marshal(args)
				if err != nil || os.WriteFile(path, data, 0o600) != nil {
					return 97
				}
			}
		}
		return dispatch(append([]string{"proof-run"}, args...))
	}
}

func TestManagedSuiteResourceAcquireAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{})
}

func TestManagedSuiteCancellationDuringAcquireAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{cancel: true})
}

func TestManagedSuiteCancellationBeforeCommandAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{cancelAtCommand: true})
}

func TestManagedSuiteApproveThenClaimAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{approveThenClaim: true})
}

func TestManagedSuiteOwnerDeadlineDuringAcquireAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{ownerWait: "resource"})
}

func TestManagedSuiteOwnerDeadlineDuringProducerWaitAdapterIntegration(t *testing.T) {
	t.Parallel()
	if runGoGateCommandTestInOwnedProcess(t) {
		return
	}
	testManagedSuiteResourceAcquire(t, managedSuiteWaitOptions{ownerWait: "producer"})
}

type managedSuiteWaitOptions struct {
	cancel, cancelAtCommand, approveThenClaim bool
	ownerWait                                 string
}

// The fixture drives the input-bound managed command, including its real
// resource locks, worker, watchdog and retained result.
// Native Git establishes that the waited command's retained tree names its
// actual candidate worktree; a Git stub cannot prove that adapter binding.
func testManagedSuiteResourceAcquire(t *testing.T, options managedSuiteWaitOptions) {
	t.Helper()
	f := newPortableProofFixtureWithSetup(t, "", func(f *portableProofFixture) {
		f.contract.SchemaVersion = 2
		f.contract.Groups[0].Phase = "acceptance"
		f.contract.Groups[0].EnvironmentMode = "inherit"
		f.contract.Groups[0].Resources = testpolicy.GroupResources{Class: "cheap", Exclusive: []string{"managed-deadline"}}
		f.writeContract()
	})
	root, err := canonicalProofRoot(f.root)
	if err != nil {
		t.Fatal(err)
	}
	if options.approveThenClaim {
		path := "plans/goals/portable.md"
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		file, problems := goal.ParseFile(data)
		if len(problems) != 0 {
			t.Fatal(problems)
		}
		file.Approved.Revision, file.Claimed.Revision, file.Claimed.AccountingRevision = 2, 3, 3
		file.Approved.At, file.Claimed.At = file.Claimed.At, file.Approved.At
		file.StopCapability.Generation, file.StopCapability.Revision = 3, 3
		file.History[1], file.History[2] = file.History[2], file.History[1]
		file.History[1].At, file.History[2].At = file.Approved.At, file.Claimed.At
		f.writeBytes(path, goal.RenderFile(file), 0o644)
		f.git("add", path)
		f.git("commit", "-qm", "approve before claiming the fixture goal")
		for _, ref := range []string{goal.LocalLedgerBranch, goal.AcceptedRef, "refs/remotes/origin/main"} {
			f.git("update-ref", ref, "HEAD")
		}
		enrollFixturePolicyEngine(t, root, time.Now().UTC(), f.git("rev-parse", "HEAD"), &fixturePolicyEngine{path: f.engine})
	}
	for _, entry := range f.commandEnvironment() {
		key, value, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "METASYSTEM_") {
			setOwnedGoGateProcessEnvironment(t, key, value)
		}
	}
	argsPath := filepath.Join(t.TempDir(), "watchdog.json")
	setOwnedGoGateProcessEnvironment(t, "GO_WANT_BATCH_E2E_COMMAND", "1")
	setOwnedGoGateProcessEnvironment(t, "MANAGED_TEST_DEADLINE_ARGS", argsPath)
	var ownerDeadline time.Time
	if options.ownerWait != "" {
		started := time.Now().UTC().Truncate(time.Second)
		ownerDeadline = started.Add(time.Hour)
		weight := uint64(0)
		store := &runpkg.Store{Root: root, Now: func() time.Time { return started },
			AdmitGoverned: func(runpkg.GovernedAdmissionRequest) (runpkg.GovernedAdmissionResult, error) {
				return runpkg.GovernedAdmissionResult{Attempt: runpkg.GovernedAttempt{
					GoalRevision: 2, ObligationRevision: 7, ExecutionCostMinutes: 60, AttemptOrdinal: 1,
					WeightGeneration: &weight, Recurrence: governance.StandingSharedProcess,
					Budget:          goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 8, ReservedJobMinutesLimit: 360, ActiveJobLimit: 2},
					BudgetStartedAt: started.Format(time.RFC3339), CorrelationPolicy: "exact-run-generation",
					ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
						Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-surface", MaxActiveJobs: 1,
						TimingEnvelopeSeconds: 3600, ObservationSource: "run-terminal-record"},
					AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: runpkg.BreakerClosed}}, nil
			}}
		nonce, err := store.Launch(runpkg.Caller{Class: "HUMAN"}, runpkg.LaunchParams{
			Id: "managed-owner", Kind: "suite", GoalId: "portable", ObligationRevision: 7,
			Display: "managed test owner", Log: "artifacts/managed-owner.log"})
		if err != nil {
			t.Fatal(err)
		}
		pgid, err := syscall.Getpgid(os.Getpid())
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Bind("managed-owner", nonce, int64(os.Getpid()), int64(pgid)); err != nil {
			t.Fatal(err)
		}
		if options.ownerWait == "resource" {
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ROOT", root)
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ID", "managed-owner")
		}
	}
	holder, err := proofrun.AcquireHostResources(t.Context(), root, filepath.Join(root, "metasystem.conf"), "cheap", []string{"managed-deadline"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = holder.Close() })
	resourcePath := holder.Files()[0].Name()
	parentContext, cancelParent := context.WithCancel(t.Context())
	defer cancelParent()
	now := time.Now().UTC()
	waits := 0
	var admitted proofrun.Attempt
	ctx := proofrun.WithHostResourceWaitObserver(parentContext, func() {
		waits++
		attempts, err := proofrun.ReadAttempts(root)
		if err != nil || len(attempts) != 1 {
			t.Fatalf("waiting run has no unique admitted attempt: %+v, %v", attempts, err)
		}
		admitted = attempts[0]
		deadline, err := time.Parse(time.RFC3339Nano, admitted.Deadline)
		if err != nil {
			t.Fatal(err)
		}
		now = deadline.Add(2 * time.Minute)
		if options.ownerWait != "" {
			now = ownerDeadline.Add(2 * time.Minute)
		}
		if options.cancel {
			if err := proofrun.RequestCancellation(root, admitted.AttemptID, "cancel while queued"); err != nil {
				t.Fatal(err)
			}
		}
		if options.ownerWait == "resource" {
			return // Keep capacity unavailable until the owner's deadline refuses the wait.
		}
		if options.ownerWait == "producer" {
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ROOT", root)
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ID", "managed-owner")
			consumerContext, cancelConsumer := context.WithCancel(t.Context())
			defer cancelConsumer()
			var consumerOut, consumerErr bytes.Buffer
			consumerStatus := runTestRunWith(testRunInvocation{callerPID: int64(os.Getpid()), stdout: &consumerOut, stderr: &consumerErr,
				nativeContext: consumerContext, nativeClock: func() time.Time {
					// Bound the defective path deterministically if it ignores the owner.
					cancelConsumer()
					return now
				}}, []string{"--root", root, "--goal", "portable", "--tree", f.git("rev-parse", "HEAD^{tree}"),
				"--mode", "auto", "--purpose", "delivery", "--cap-min", "1"})
			attempts, err := proofrun.ReadAttempts(root)
			if err != nil || len(attempts) != 2 {
				t.Fatalf("shared producer consumer was not reserved: %+v, %v\n%s", attempts, err, consumerErr.String())
			}
			var consumer proofrun.Attempt
			for _, attempt := range attempts {
				if attempt.AttemptID != admitted.AttemptID {
					consumer = attempt
				}
			}
			if consumerStatus == 0 || !strings.Contains(consumerErr.String(), "await shared producer: the test run's admitted deadline") ||
				consumer.TestWaits["app-a"] != admitted.AttemptID || consumer.ReservationOwner == nil || consumer.Terminal == nil {
				t.Fatalf("shared producer wait outlived its owner: status=%d attempt=%+v\n%s", consumerStatus, consumer, consumerErr.String())
			}
			_, native := f.counts()
			if native["a"] != 0 {
				t.Fatal("the expired consumer started a native command")
			}
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ROOT", "")
			setOwnedGoGateProcessEnvironment(t, "METASYSTEM_PROOF_RUN_ID", "")
		}
		if err := holder.Close(); err != nil {
			t.Fatal(err)
		}
	})
	var stdout, stderr bytes.Buffer
	resultPath := filepath.Join(t.TempDir(), "result.json")
	clock := func() time.Time {
		if options.ownerWait == "resource" && waits != 0 {
			// Cancellation bounds a mutant that ignores the owner's expired clock.
			cancelParent()
		}
		if options.cancelAtCommand && waits != 0 {
			// Cancellation arrives after the command owns the released resource.
			// Probe the real lock rather than assuming an acquisition call order.
			file, err := os.OpenFile(resourcePath, os.O_RDWR, 0)
			if err != nil {
				t.Fatal(err)
			}
			err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
			if errors.Is(err, unix.EWOULDBLOCK) {
				cancelParent()
			} else if err != nil {
				t.Fatal(err)
			} else if err := unix.Flock(int(file.Fd()), unix.LOCK_UN); err != nil {
				t.Fatal(err)
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
		}
		return now
	}
	status := runTestRunWith(testRunInvocation{callerPID: int64(os.Getpid()), stdout: &stdout, stderr: &stderr,
		nativeContext: ctx, nativeClock: clock}, []string{"--root", root, "--goal", "portable",
		"--tree", f.git("rev-parse", "HEAD^{tree}"), "--mode", "auto", "--purpose", "delivery", "--cap-min", "1", "--result", resultPath})
	if waits != 1 {
		t.Fatalf("real resource acquisition observed %d waits: status=%d\n%s", waits, status, stderr.String())
	}
	stored, err := proofrun.ReadAttempt(root, admitted.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	_, native := f.counts()
	if options.cancel || options.cancelAtCommand || options.ownerWait == "resource" {
		want := "admit native testing: the test run ended or was cancelled"
		if options.cancelAtCommand {
			want = "admit native testing: context canceled"
		}
		if options.ownerWait == "resource" {
			want = "admit native testing: the test run's admitted deadline"
			if stored.ReservationOwner == nil || stored.Deadline != stored.ReservationOwner.Deadline {
				t.Fatal("the queued command did not inherit its owner's absolute deadline")
			}
		}
		if !strings.Contains(stderr.String(), want) {
			t.Fatalf("queued cancellation was not refused at acquisition: status=%d\n%s", status, stderr.String())
		}
		if status == 0 || native["a"] != 0 || stored.Terminal == nil || stored.Terminal.Result == proofrun.TerminalSuccess {
			t.Fatalf("cancelled queued command ran or passed: status=%d native=%v terminal=%+v\n%s", status, native, stored.Terminal, stderr.String())
		}
		if _, err := os.Stat(argsPath); !os.IsNotExist(err) {
			t.Fatalf("cancelled wait started a watchdog: %v", err)
		}
		return
	}
	if status != 0 || native["a"] != 1 || stored.Terminal == nil || stored.Terminal.Result != proofrun.TerminalSuccess {
		t.Fatalf("capacity wait spent the execution allowance: status=%d native=%v terminal=%+v\n%s\n%s", status, native, stored.Terminal, stdout.String(), stderr.String())
	}
	var watchdog []string
	if err := strictjson.Read(argsPath, &watchdog); err != nil {
		t.Fatal(err)
	}
	deadline := ""
	for i, arg := range watchdog {
		if arg == "--deadline" && i+1 < len(watchdog) {
			deadline = watchdog[i+1]
		}
	}
	if want := now.Add(time.Minute).Format(time.RFC3339Nano); deadline != want {
		t.Fatalf("command deadline=%s, want full execution minute after acquisition: %s", deadline, want)
	}
	packets, err := filepath.Glob(filepath.Join(root, "artifacts", "agents", "proof-runs", admitted.AttemptID, "testing", "*", "request.json"))
	if err != nil || len(packets) != 1 {
		t.Fatalf("managed command packet: %v, %v", packets, err)
	}
	var packet proofrun.TestRunRequest
	if err := strictjson.Read(packets[0], &packet); err != nil {
		t.Fatal(err)
	}
	var result proofrun.TestResult
	if err := strictjson.Read(resultPath, &result); err != nil {
		t.Fatal(err)
	}
	if tree := f.git("rev-parse", "HEAD^{tree}"); packet.CandidateTree != tree || result.CandidateTree != tree || stored.CandidateTree != tree {
		t.Fatalf("waiting changed the native candidate binding: tree=%s packet=%s result=%s retained=%s", tree, packet.CandidateTree, result.CandidateTree, stored.CandidateTree)
	}
	if packet.QueueDurationMS <= 0 || result.Cost.QueueDurationMS != packet.QueueDurationMS || stored.TestResult == nil ||
		stored.TestResult.Cost.QueueDurationMS != packet.QueueDurationMS {
		t.Fatalf("queue wait was lost or counted again: packet=%d result=%d retained=%+v", packet.QueueDurationMS, result.Cost.QueueDurationMS, stored.TestResult)
	}
	if stored.Deadline != admitted.Deadline {
		t.Fatal("execution timing rewrote the accounting reservation")
	}
}
