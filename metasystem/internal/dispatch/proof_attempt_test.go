package dispatch

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goalbudget"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/governance"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

func TestOrdinaryProofSharesGoalBudget(t *testing.T) {
	root, identity := dispatchProofFixture(t, "ordinary-full-proof")
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 28, 8, 15, 0, 0, time.UTC)
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 45, Identity: identity, Launcher: launcher, Now: started,
	})
	if err != nil || attempt.BudgetEpoch != nil {
		t.Fatalf("reserve ordinary proof with nullable epoch: attempt=%+v err=%v", attempt, err)
	}
	writeBudgetJob(t, root, "delegate", "delegate-reservation", 3, 30, "completed", budgetJobLife{
		pid: 4242, startedAt: "2026-08-28T08:15:00Z", endedAt: "2026-08-28T08:45:00Z",
	})
	file := budgetGoal()
	file.Claimed.AccountingRevision = 3
	projection := ProjectBudget(root, file, time.Date(2026, 8, 28, 10, 0, 0, 0, time.UTC))
	if projection.Status != BudgetKnown || projection.Attempts != 2 || projection.ReservedJobMinutes != 75 || projection.ActiveJobs != 1 {
		t.Fatalf("proof and delegate did not share one budget projection: %+v", projection)
	}
	var exhausted []string
	for _, breach := range budgetAdmissionBreaches(projection) {
		exhausted = append(exhausted, breach.Field)
	}
	if strings.Join(exhausted, ",") != "attemptLimit,reservedJobMinutesLimit,activeJobLimit" {
		t.Fatalf("next reservation was not refused at shared equality boundaries: %v", exhausted)
	}
	if _, err := proofrun.FinalizeAttempt(root, attempt.AttemptID, proofrun.TerminalFailed, 23, "controlled gate failure", nil, started.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
}

func TestProofOnlyGoalStopBatch(t *testing.T) {
	if os.Getenv("GO_WANT_PROOF_STOP_HELPER") == "1" {
		if err := os.WriteFile(os.Getenv("PROOF_STOP_READY"), []byte("ready\n"), 0o600); err != nil {
			os.Exit(97)
		}
		for {
			time.Sleep(time.Minute)
		}
	}
	root, identity := dispatchProofFixture(t, "stop-proof")
	ready := filepath.Join(t.TempDir(), "ready")
	child := exec.Command(os.Args[0], "-test.run=^TestProofOnlyGoalStopBatch$")
	child.Env = append(os.Environ(), "GO_WANT_PROOF_STOP_HELPER=1", "PROOF_STOP_READY="+ready)
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = child.Process.Kill()
			<-waited
		}
	})
	deadline := time.Now().Add(wiringBound)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("proof stop helper did not become ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	launcher, err := proofrun.ProcessIdentityForPID(int64(child.Process.Pid), nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: identity, Launcher: launcher, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	unrelatedIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "unrelated-proof", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	self, _ := proofrun.CurrentProcessIdentity(nil)
	unrelated, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "other-goal", GoalRevision: 1, AccountingRevision: 1,
		ReservedMinutes: 5, Identity: unrelatedIdentity, Launcher: self, Now: now,
	})
	if err != nil {
		t.Fatal(err)
	}
	stamp := now.Format(time.RFC3339)
	batch := goal.StopBatch{StopID: "stop-bounded-r3-f1", GoalID: "bounded", GoalRevision: 3,
		FenceEpoch: 1, CapabilityGeneration: 3, Machine: "bed-m1", ClaimEpoch: 7,
		Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, now)
	if err != nil || strings.Join(batch.PendingProofs, ",") != attempt.AttemptID || len(batch.ObservedProofs) != 1 {
		t.Fatalf("proof-only stop batch did not retain its exact member: batch=%+v err=%v", batch, err)
	}
	member := batch.ObservedProofs[0]
	if member.GoalID != "bounded" || member.GoalRevision != 2 || member.AccountingRevision != 2 ||
		member.StopID != batch.StopID || member.FenceEpoch != 1 || member.CapabilityGeneration != 3 ||
		member.Machine != "bed-m1" || member.ClaimEpoch != 7 {
		t.Fatalf("proof stop member lost authority coordinates: %+v", member)
	}
	if err := CancelStopProof(root, batch.StopID, attempt.AttemptID); err != nil {
		t.Fatal(err)
	}
	<-waited
	killed = true
	stopped, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stopped.Terminal == nil || stopped.Terminal.Result != proofrun.TerminalCancelled {
		t.Fatalf("proof cancellation was not durably joined: attempt=%+v err=%v", stopped, err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, now.Add(time.Second))
	if err != nil || batch.State != goal.StopBatchComplete || len(batch.PendingProofs) != 0 ||
		strings.Join(batch.TerminalProofs, ",") != attempt.AttemptID {
		t.Fatalf("proof-only stop batch did not reach its terminal fixed point: batch=%+v err=%v", batch, err)
	}
	stillLive, err := proofrun.ReadAttempt(root, unrelated.AttemptID)
	if err != nil || stillLive.Terminal != nil || stillLive.CancellationIntent != "" {
		t.Fatalf("unrelated goal proof was affected: attempt=%+v err=%v", stillLive, err)
	}
	if _, err := proofrun.FinalizeAttempt(root, unrelated.AttemptID, proofrun.TerminalFailed, 1, "fixture cleanup", nil, now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
}

func TestProofGovernedReservationJoin(t *testing.T) {
	root, proofIdentity := dispatchProofFixture(t, "governed-join")
	started := time.Date(2026, 8, 28, 8, 15, 0, 0, time.UTC)
	clock := started
	weightGeneration := uint64(0)
	store := &run.Store{Root: root, Now: func() time.Time { return clock }}
	store.AdmitGoverned = func(run.GovernedAdmissionRequest) (run.GovernedAdmissionResult, error) {
		return run.GovernedAdmissionResult{Attempt: run.GovernedAttempt{
			GoalRevision: 3, ObligationRevision: 7, WeightGeneration: &weightGeneration,
			Recurrence: governance.StandingSharedProcess, ExecutionCostMinutes: 5, AttemptOrdinal: 1,
			Budget:          goalbudget.Budget{ElapsedLimit: "4h", AttemptLimit: 2, ReservedJobMinutesLimit: 75, ActiveJobLimit: 1},
			BudgetStartedAt: started.Format(time.RFC3339), CorrelationPolicy: "exact-run-generation",
			ExpectedAssumptions: governance.ObligationAssumptions{Recurrence: governance.StandingSharedProcess,
				Platform: "fixture/os", ToolchainIdentity: "fixture-go", SurfaceDigest: "fixture-surface",
				MaxActiveJobs: 1, TimingEnvelopeSeconds: 300, ObservationSource: "run-terminal-record"},
			AdmissionDecision: governance.ConsequenceDecision{Apply: true}, Breaker: run.BreakerClosed,
		}}, nil
	}
	store.ObserveGoverned = func(*run.Record, time.Time) run.AssumptionObservation {
		return run.AssumptionObservation{ObservedAt: clock.Format(time.RFC3339), AssumptionState: run.AssumptionMatch}
	}
	store.ProjectSpend = func(*run.Record, time.Time) (run.SpendSnapshot, string) { return run.SpendSnapshot{}, "" }
	nonce, err := store.Launch(run.Caller{Class: "MAIN", MainId: "main-fixture", OwnerLineage: "main-fixture"}, run.LaunchParams{
		Id: "governed-proof", Kind: "suite", Display: "governed proof owner", Log: "artifacts/governed.log",
		GoalId: "bounded", ObligationRevision: 7, StandingShared: true,
		Expect: run.Expect{Green: "green", Red: "red", Hung: "hung", Unknown: "unknown"},
	})
	if err != nil {
		t.Fatal(err)
	}
	child := exec.Command("sleep", "60")
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = child.Process.Kill()
			<-waited
		}
	})
	pgid, err := syscall.Getpgid(child.Process.Pid)
	bindErr := store.Bind("governed-proof", nonce, int64(child.Process.Pid), int64(pgid))
	if err != nil || bindErr != nil {
		t.Fatalf("bind governed owner: pgid=%d pgidErr=%v bindErr=%v", pgid, err, bindErr)
	}
	record, err := store.Read("governed-proof")
	if err != nil || record == nil || record.Governed == nil {
		t.Fatalf("read governed owner: record=%+v err=%v", record, err)
	}
	deadline := started.Add(5 * time.Minute).Format(time.RFC3339Nano)
	launcher, _ := proofrun.CurrentProcessIdentity(nil)
	attempt, _, err := proofrun.ReserveLocked(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 5, Identity: proofIdentity, Launcher: launcher, Now: started,
		ReservationOwner: &proofrun.ReservationOwner{ControlRoot: root, RunID: record.RunId,
			RunGeneration: record.Generation, LaunchNonce: record.LaunchNonce, GoalRevision: record.Governed.GoalRevision,
			ObligationRevision: record.Governed.ObligationRevision, AttemptOrdinal: record.Governed.AttemptOrdinal,
			BudgetEpoch: record.Governed.BudgetEpoch, Deadline: deadline},
	})
	if err != nil {
		t.Fatal(err)
	}
	file := budgetGoal()
	file.Claimed.AccountingRevision = 3
	live := ProjectBudget(root, file, started.Add(time.Minute))
	if live.Status != BudgetKnown || live.Attempts != 1 || live.ReservedJobMinutes != 5 || live.ActiveJobs != 1 {
		t.Fatalf("joined proof double-charged its live governed owner: %+v", live)
	}
	if _, err := proofrun.FinalizeAttempt(root, attempt.AttemptID, proofrun.TerminalSuccess, 0, "joined proof green", nil, started.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := child.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-waited
	finished = true
	clock = started.Add(time.Minute)
	if err := store.WriteSidecar(record.RunId, record.Generation, record.LaunchNonce, 0); err != nil {
		t.Fatal(err)
	}
	if result, err := store.Assess(record.RunId); err != nil || !result.Transitioned || result.To != run.StatusGreen {
		t.Fatalf("terminal governed owner did not join: result=%+v err=%v", result, err)
	}
	terminal := ProjectBudget(root, file, clock.Add(time.Second))
	if terminal.Status != BudgetKnown || terminal.Attempts != 1 || terminal.ReservedJobMinutes != 1 || terminal.ActiveJobs != 0 {
		t.Fatalf("joined proof double-charged its terminal governed owner: %+v", terminal)
	}
	stored, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil {
		t.Fatal(err)
	}
	stored.ReservationOwner.LaunchNonce = strings.Repeat("f", 32)
	path, _ := proofrun.AttemptPath(root, stored.AttemptID)
	encoded, _ := json.MarshalIndent(stored, "", "  ")
	if err := os.WriteFile(path, append(encoded, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}
	mismatch := ProjectBudget(root, file, clock.Add(2*time.Second))
	if mismatch.Status != BudgetUnknown || mismatch.Unknown == nil || !strings.Contains(mismatch.Unknown.Reason, "exact run-generation owner") {
		t.Fatalf("mismatched governed owner was treated as a free proof join: %+v", mismatch)
	}
}

func dispatchProofFixture(t *testing.T, commandClass string) (string, proofrun.ProofIdentity) {
	t.Helper()
	root := budgetProjectionRoot(t)
	if err := os.MkdirAll(filepath.Join(root, "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"coverage-ratchet.json", "coverage-ratchet-linux.json"} {
		if err := os.WriteFile(filepath.Join(root, "scripts", "agents", name), []byte(`{"floors":{"internal/proofrun":1},"exempt":{}}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	identity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", commandClass,
		[]string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	return root, identity
}
