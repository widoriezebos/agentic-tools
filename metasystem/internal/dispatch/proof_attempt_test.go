package dispatch

import (
	"bufio"
	"encoding/json"
	"io"
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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

type proofStopProbe struct {
	exact identity.Exact
	state identity.Liveness
}

func (p *proofStopProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if pid != p.exact.Pid || p.state == identity.Dead {
		return identity.Exact{}, identity.Dead, nil
	}
	return p.exact, p.state, nil
}

func TestOrdinaryProofSharesGoalBudget(t *testing.T) {
	root, identity := dispatchProofFixture(t, "ordinary-full-proof")
	launcher, err := proofrun.CurrentProcessIdentity(nil)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Date(2026, 8, 28, 8, 15, 0, 0, time.UTC)
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 45, Identity: identity, Launcher: launcher, Now: started,
	}))

	if err != nil || attempt.BudgetEpoch != nil {
		t.Fatalf("reserve ordinary proof with nullable epoch: attempt=%+v err=%v", attempt, err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
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
		ready := os.NewFile(3, "proof-stop-ready")
		if ready == nil {
			os.Exit(97)
		}
		if _, err := ready.Write([]byte{1}); err != nil {
			os.Exit(97)
		}
		_ = ready.Close()
		for {
			time.Sleep(time.Minute)
		}
	}
	root, proofIdentity := dispatchProofFixture(t, "stop-proof")
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = readyRead.Close() })
	child := exec.Command(os.Args[0], "-test.run=^TestProofOnlyGoalStopBatch$")
	child.Env = append(os.Environ(), "GO_WANT_PROOF_STOP_HELPER=1")
	child.ExtraFiles = []*os.File{readyWrite}
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	t.Logf("proof stop helper pid: %d", child.Process.Pid)
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	killed := false
	t.Cleanup(func() {
		if !killed {
			_ = child.Process.Kill()
			<-waited
		}
	})
	if _, err := io.ReadFull(readyRead, make([]byte, 1)); err != nil {
		t.Fatalf("proof stop helper did not become ready: %v", err)
	}
	launcher, err := proofrun.ProcessIdentityForPID(int64(child.Process.Pid), nil)
	if err != nil {
		t.Fatal(err)
	}
	exact, state, err := (identity.KernelProber{}).Probe(int64(child.Process.Pid))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe proof stop helper: state=%s err=%v", state, err)
	}
	prober := &proofStopProbe{exact: exact, state: identity.Alive}
	started := time.Date(2026, 9, 19, 8, 0, 0, 0, time.UTC)
	cancelRequestedAt := started.Add(58 * time.Second)
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 2, AccountingRevision: 2,
		ReservedMinutes: 5, Identity: proofIdentity, Launcher: launcher, Now: started,
	}))

	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	unrelatedIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "unrelated-proof", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	self, _ := proofrun.CurrentProcessIdentity(nil)
	unrelated, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "other-goal", GoalRevision: 1, AccountingRevision: 1,
		ReservedMinutes: 5, Identity: unrelatedIdentity, Launcher: self, Now: started,
	}))

	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	competingIdentity, err := proofrun.BuildProofIdentity(root, filepath.Join(root, "metasystem.conf"), "full", "competing-green", []string{"gate"}, behaviorsurface.SupportedVersion)
	if err != nil {
		t.Fatal(err)
	}
	competing, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "competing-goal", GoalRevision: 1, AccountingRevision: 1,
		ReservedMinutes: 5, Identity: competingIdentity, Launcher: self, Now: started.Add(time.Second),
	}))
	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
	stamp := cancelRequestedAt.Format(time.RFC3339)
	batch := goal.StopBatch{StopID: "stop-bounded-r3-f1", GoalID: "bounded", GoalRevision: 3,
		FenceEpoch: 1, CapabilityGeneration: 3, Machine: "bed-m1", ClaimEpoch: 7,
		Reason: goal.StopReasonElapsedLimit, State: goal.StopBatchOpen, OpenedAt: stamp, UpdatedAt: stamp}
	if err := goal.WriteStopBatch(root, batch); err != nil {
		t.Fatal(err)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, cancelRequestedAt)
	if err != nil || strings.Join(batch.PendingProofs, ",") != attempt.AttemptID || len(batch.ObservedProofs) != 1 {
		t.Fatalf("proof-only stop batch did not retain its exact member: batch=%+v err=%v", batch, err)
	}
	member := batch.ObservedProofs[0]
	if member.GoalID != "bounded" || member.GoalRevision != 2 || member.AccountingRevision != 2 ||
		member.StopID != batch.StopID || member.FenceEpoch != 1 || member.CapabilityGeneration != 3 ||
		member.Machine != "bed-m1" || member.ClaimEpoch != 7 {
		t.Fatalf("proof stop member lost authority coordinates: %+v", member)
	}
	clock := cancelRequestedAt
	competingGreenAt := started.Add(61 * time.Second)
	var competingErr error
	var proofStopWaits int
	killSent := false
	if err := cancelStopProof(root, batch.StopID, attempt.AttemptID, proofrun.StopOptions{
		Prober: prober,
		Now:    func() time.Time { return clock },
		Signal: func(target int, signal syscall.Signal) error {
			if signal != syscall.SIGKILL {
				return nil
			}
			killSent = true
			err := syscall.Kill(target, signal)
			prober.state = identity.Dead
			return err
		},
		Sleep: func(duration time.Duration) {
			proofStopWaits++
			clock = clock.Add(duration)
			if competingErr == nil && !clock.Before(competingGreenAt) {
				var retained proofrun.Attempt
				retained, competingErr = proofrun.ReadAttempt(root, competing.AttemptID)
				if competingErr == nil && retained.Terminal == nil {
					_, competingErr = proofrun.FinalizeAttempt(root, competing.AttemptID, proofrun.TerminalSuccess, 0, "competing green completed during cleanup", nil, competingGreenAt)
				}
			}
		},
	}); err != nil {
		t.Fatal(err)
	}
	if proofStopWaits == 0 || !killSent {
		t.Fatalf("proof stop did not advance through TERM grace and KILL: waits=%d kill=%t", proofStopWaits, killSent)
	}
	<-waited
	killed = true
	stopped, err := proofrun.ReadAttempt(root, attempt.AttemptID)
	if err != nil || stopped.Terminal == nil || stopped.Terminal.Result != proofrun.TerminalCancelled {
		t.Fatalf("proof cancellation was not durably joined: attempt=%+v err=%v", stopped, err)
	}
	terminalAt, parseErr := time.Parse(time.RFC3339Nano, stopped.Terminal.At)
	green, greenErr := proofrun.ReadAttempt(root, competing.AttemptID)
	if competingErr != nil || greenErr != nil || green.Terminal == nil || green.Terminal.Result != proofrun.TerminalSuccess || green.Terminal.At != competingGreenAt.Format(time.RFC3339Nano) {
		t.Fatalf("competing green did not become terminal during cleanup: attempt=%+v finalize=%v read=%v", green, competingErr, greenErr)
	}
	if parseErr != nil || !terminalAt.Equal(started.Add(63*time.Second)) || !terminalAt.After(competingGreenAt) || stopped.ObservedMinutes != 2 {
		t.Fatalf("cleanup completion accounting: terminal=%s parse=%v observed=%d, want 63s after start, after competing green, charged two minutes", stopped.Terminal.At, parseErr, stopped.ObservedMinutes)
	}
	batch, err = ReconcileStopBatch(root, batch.StopID, clock.Add(time.Second))
	if err != nil || batch.State != goal.StopBatchComplete || len(batch.PendingProofs) != 0 ||
		strings.Join(batch.TerminalProofs, ",") != attempt.AttemptID {
		t.Fatalf("proof-only stop batch did not reach its terminal fixed point: batch=%+v err=%v", batch, err)
	}
	stillLive, err := proofrun.ReadAttempt(root, unrelated.AttemptID)
	if err != nil || stillLive.Terminal != nil || stillLive.CancellationIntent != "" {
		t.Fatalf("unrelated goal proof was affected: attempt=%+v err=%v", stillLive, err)
	}
	if _, err := proofrun.FinalizeAttempt(root, unrelated.AttemptID, proofrun.TerminalFailed, 1, "fixture cleanup", nil, clock.Add(2*time.Second)); err != nil {
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
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	releaseRead, releaseWrite, err := os.Pipe()
	if err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		t.Fatal(err)
	}
	child := exec.Command("/bin/sh", "-c", "printf 'ready\\n' >&3; IFS= read -r _ <&4")
	child.ExtraFiles = []*os.File{readyWrite, releaseRead}
	child.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := child.Start(); err != nil {
		_ = readyRead.Close()
		_ = readyWrite.Close()
		_ = releaseRead.Close()
		_ = releaseWrite.Close()
		t.Fatal(err)
	}
	_ = readyWrite.Close()
	_ = releaseRead.Close()
	waited := make(chan error, 1)
	go func() { waited <- child.Wait() }()
	finished := false
	t.Cleanup(func() {
		if !finished {
			_ = releaseWrite.Close()
			_ = child.Process.Kill()
			<-waited
		}
	})
	ready, readyErr := bufio.NewReader(readyRead).ReadString('\n')
	_ = readyRead.Close()
	if readyErr != nil || ready != "ready\n" {
		t.Fatalf("governed owner readiness=%q err=%v", ready, readyErr)
	}
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
	attempt, decision, err := proofrun.ReserveLocked(candidateProofAdmission(proofrun.AdmissionRequest{
		ControlRoot: root, ExecutionRoot: root, GoalID: "bounded", GoalRevision: 3, AccountingRevision: 3,
		ReservedMinutes: 5, Identity: proofIdentity, Launcher: launcher, Now: started,
		ReservationOwner: &proofrun.ReservationOwner{ControlRoot: root, RunID: record.RunId,
			RunGeneration: record.Generation, LaunchNonce: record.LaunchNonce, GoalRevision: record.Governed.GoalRevision,
			ObligationRevision: record.Governed.ObligationRevision, AttemptOrdinal: record.Governed.AttemptOrdinal,
			BudgetEpoch: record.Governed.BudgetEpoch, Deadline: deadline},
	}))

	if err != nil {
		t.Fatal(err)
	}
	requireProofReservationNotAdmissionRefused(t, decision)
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
	_ = releaseWrite.Close()
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
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
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
