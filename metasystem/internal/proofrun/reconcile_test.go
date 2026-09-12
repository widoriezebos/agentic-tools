package proofrun

import (
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// mapProbe answers liveness per pid; every pid was started at second 100.
type mapProbe struct{ states map[int64]identity.Liveness }

func (p *mapProbe) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	state, ok := p.states[pid]
	if !ok {
		return identity.Exact{}, identity.Dead, nil
	}
	if state == identity.Unknown {
		return identity.Exact{}, identity.Unknown, nil
	}
	return identity.Exact{Pid: pid, StartedAt: time.Unix(100, 0)}, state, nil
}

func reconcileFixture(t *testing.T, now time.Time) (string, Attempt, Record) {
	t.Helper()
	root, proof := proofAttemptFixture(t, "reconcile")
	launcher := ProcessIdentity{Pid: 1001, PidStartedAt: 100}
	attempt, result, err := ReserveLocked(AdmissionRequest{ControlRoot: root, ExecutionRoot: root, GoalID: "goal-a", GoalRevision: 3,
		AccountingRevision: 2, ReservedMinutes: 4, Identity: proof, Launcher: launcher, Now: now})
	if err != nil || result.Disposition != DispositionExecuted {
		t.Fatalf("reserve = %+v, %+v, %v", attempt, result, err)
	}
	record := Record{Suite: "testing", Root: root, ControlRoot: root, AttemptID: attempt.AttemptID, LaunchID: "launch1",
		Launcher: launcher, SuiteProcess: ProcessIdentity{Pid: 1002, Pgid: 1002, PidStartedAt: 100},
		Watchdog: ProcessIdentity{Pid: 1003, PidStartedAt: 100}, Status: StatusRunning}
	if err := writeRecord(record); err != nil {
		t.Fatal(err)
	}
	attempt.ProcessKeys = []string{record.Key()}
	if err := writeAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	return root, attempt, record
}

func reconcileOptions(probe *mapProbe, now time.Time, signal func(int, syscall.Signal) error) ReconcileOptions {
	if signal == nil {
		signal = func(int, syscall.Signal) error { return nil }
	}
	return ReconcileOptions{Prober: probe, Now: now, ObservationWindow: time.Minute,
		GroupMembers: func(int64) ([]int64, error) { return nil, nil },
		Stop:         StopOptions{TermGrace: 20 * time.Millisecond, KillGrace: 20 * time.Millisecond, Poll: time.Millisecond, Prober: probe, Signal: signal}}
}

func TestReconcileKeepsTheReservationWhileTheSuiteGroupHasAnUnrecordedMember(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, _ := reconcileFixture(t, now)
	// Every recorded identity is dead, but a child the suite leader spawned
	// still lives in the recorded group: the reservation stays held.
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Dead, 1002: identity.Dead, 1003: identity.Dead}}
	options := reconcileOptions(probe, now.Add(time.Hour), nil)
	members := []int64{1077}
	options.GroupMembers = func(pgid int64) ([]int64, error) {
		if pgid != 1002 {
			t.Fatalf("group %d was inspected; the recorded suite group is 1002", pgid)
		}
		return members, nil
	}
	outcomes, err := ReconcileAttempts(root, options)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileCustodyPending || !strings.Contains(outcomes[0].Reason, "live members [1077]") {
		t.Fatalf("outcomes = %+v, %v", outcomes, err)
	}
	if after, err := ReadAttempt(root, attempt.AttemptID); err != nil || after.Terminal != nil {
		t.Fatalf("attempt was finalized behind a live group member: %+v, %v", after.Terminal, err)
	}
	members = nil
	outcomes, err = ReconcileAttempts(root, options)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileReconciled {
		t.Fatalf("outcomes after the member ended = %+v, %v", outcomes, err)
	}
}

func TestReconcileSettlesTheRecordsOfATerminalAttempt(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, record := reconcileFixture(t, now)
	// The launcher committed its own terminal and died; its record still says
	// running, and an observation marker was left behind.
	if _, err := FinalizeAttempt(root, attempt.AttemptID, TerminalSuccess, 0, "done by the launcher", nil, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	writeObservation(root, attempt.AttemptID, now)
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Dead, 1002: identity.Dead, 1003: identity.Dead}}
	outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), func(pid int, _ syscall.Signal) error {
		t.Fatalf("pid %d of a terminal attempt was signalled", pid)
		return nil
	}))
	if err != nil || len(outcomes) != 0 {
		t.Fatalf("outcomes over a terminal attempt = %+v, %v", outcomes, err)
	}
	if stored, err := ReadProcessRecord(root, record.Key()); err != nil || stored.Status != StatusDone {
		t.Fatalf("record of the terminal attempt = %+v, %v", stored, err)
	}
	if _, seen := readObservation(root, attempt.AttemptID); seen {
		t.Fatal("the orphaned observation marker was not cleared")
	}
}

func TestReconcileTerminalizesAnAttemptWhoseLauncherIsDead(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, record := reconcileFixture(t, now)
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Dead, 1002: identity.Dead, 1003: identity.Dead}}
	var signalled []int
	outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), func(pid int, _ syscall.Signal) error {
		signalled = append(signalled, pid)
		return nil
	}))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileReconciled {
		t.Fatalf("outcomes = %+v, %v", outcomes, err)
	}
	if len(signalled) != 0 {
		t.Fatalf("dead processes were signalled: %v", signalled)
	}
	after, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || after.Terminal == nil || after.Terminal.Result != TerminalFailed || after.Terminal.ExitStatus != 1 ||
		!strings.Contains(after.Terminal.Reason, "launcher pid 1001 started 100 is dead") || !strings.Contains(after.Terminal.Reason, "1 recorded processes confirmed ended") {
		t.Fatalf("attempt after reconciliation = %+v, %v", after.Terminal, err)
	}
	if after.ObservedMinutes != 60 {
		t.Fatalf("observed minutes = %d", after.ObservedMinutes)
	}
	stored, err := ReadProcessRecord(root, record.Key())
	if err != nil || stored.Status != StatusDone {
		t.Fatalf("process record after reconciliation = %+v, %v", stored, err)
	}
	// Idempotent: a terminal attempt is not looked at again.
	again, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(2*time.Hour), nil))
	if err != nil || len(again) != 0 {
		t.Fatalf("second pass = %+v, %v", again, err)
	}
}

func TestReconcileLeavesALiveLauncherAndAnUninspectableOneAlone(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, _ := reconcileFixture(t, now)
	for _, state := range []identity.Liveness{identity.Alive, identity.Unknown} {
		probe := &mapProbe{states: map[int64]identity.Liveness{1001: state, 1002: identity.Alive, 1003: identity.Alive}}
		outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), func(pid int, _ syscall.Signal) error {
			t.Fatalf("pid %d was signalled with the launcher %s", pid, state)
			return nil
		}))
		want := ReconcileLive
		if state == identity.Unknown {
			want = ReconcileUnknown
		}
		if err != nil || len(outcomes) != 1 || outcomes[0].Action != want {
			t.Fatalf("launcher %s: outcomes = %+v, %v", state, outcomes, err)
		}
		after, err := ReadAttempt(root, attempt.AttemptID)
		if err != nil || after.Terminal != nil {
			t.Fatalf("launcher %s: attempt was touched: %+v, %v", state, after.Terminal, err)
		}
	}
}

func TestReconcileKeepsTheReservationWhileAChildResistsTheStop(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, _ := reconcileFixture(t, now)
	// The launcher is dead, the suite group is alive and ignores every signal.
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Dead, 1002: identity.Alive, 1003: identity.Dead}}
	outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), nil))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileCustodyPending || !strings.Contains(outcomes[0].Reason, "suite") {
		t.Fatalf("outcomes = %+v, %v", outcomes, err)
	}
	after, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || after.Terminal != nil {
		t.Fatalf("attempt was finalized behind a live child: %+v, %v", after.Terminal, err)
	}
	// The signal lands now: the next pass confirms the end and finalizes.
	outcomes, err = ReconcileAttempts(root, reconcileOptions(probe, now.Add(2*time.Hour), func(pid int, signal syscall.Signal) error {
		if pid == -1002 && signal == syscall.SIGTERM {
			probe.states[1002] = identity.Dead
		}
		return nil
	}))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileReconciled {
		t.Fatalf("outcomes after the child ended = %+v, %v", outcomes, err)
	}
}

func TestReconcileStopsALiveLauncherWhoseRecordedProcessesAllEnded(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, _ := reconcileFixture(t, now)
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Alive, 1002: identity.Dead, 1003: identity.Dead}}
	signal := func(pid int, sig syscall.Signal) error {
		if pid == 1001 && sig == syscall.SIGTERM {
			probe.states[1001] = identity.Dead
		}
		return nil
	}
	// First sight: observed, nothing signalled.
	outcomes, err := ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour), func(pid int, _ syscall.Signal) error {
		t.Fatalf("pid %d was signalled on the first sight", pid)
		return nil
	}))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileObserved {
		t.Fatalf("first pass = %+v, %v", outcomes, err)
	}
	// Inside the observation window: still observed.
	outcomes, err = ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour+30*time.Second), signal))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileObserved {
		t.Fatalf("second pass inside the window = %+v, %v", outcomes, err)
	}
	// Past the window: the wedged launcher is stopped and the attempt ends.
	outcomes, err = ReconcileAttempts(root, reconcileOptions(probe, now.Add(time.Hour+2*time.Minute), signal))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileReconciled {
		t.Fatalf("third pass = %+v, %v", outcomes, err)
	}
	after, err := ReadAttempt(root, attempt.AttemptID)
	if err != nil || after.Terminal == nil || !strings.Contains(after.Terminal.Reason, "recorded processes ended without a terminal; launcher pid 1001 stopped") {
		t.Fatalf("attempt after path 2 = %+v, %v", after.Terminal, err)
	}
	if _, seen := readObservation(root, attempt.AttemptID); seen {
		t.Fatal("the observation marker outlived the reconciliation")
	}
	// A launcher whose children come back alive clears the observation.
	root2, _, _ := reconcileFixture(t, now)
	probe2 := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Alive, 1002: identity.Dead, 1003: identity.Dead}}
	if _, err := ReconcileAttempts(root2, reconcileOptions(probe2, now.Add(time.Hour), nil)); err != nil {
		t.Fatal(err)
	}
	probe2.states[1002] = identity.Alive
	outcomes, err = ReconcileAttempts(root2, reconcileOptions(probe2, now.Add(2*time.Hour), nil))
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != ReconcileLive {
		t.Fatalf("live children after an observation = %+v, %v", outcomes, err)
	}
	if _, seen := readObservation(root2, outcomes[0].AttemptID); seen {
		t.Fatal("a live child did not clear the observation")
	}
}

func TestLiveAttemptsReadTheLauncherLiveness(t *testing.T) {
	now := time.Date(2026, 9, 12, 12, 0, 0, 0, time.UTC)
	root, attempt, _ := reconcileFixture(t, now)
	probe := &mapProbe{states: map[int64]identity.Liveness{1001: identity.Dead}}
	live, err := LiveAttempts(root, probe)
	if err != nil || len(live) != 1 || live[0].AttemptID != attempt.AttemptID || live[0].LauncherLiveness != "dead" || live[0].ProcessRecords != 1 || live[0].GoalID != "goal-a" {
		t.Fatalf("live attempts = %+v, %v", live, err)
	}
	if _, err := ReconcileAttempts(root, reconcileOptions(&mapProbe{states: map[int64]identity.Liveness{}}, now.Add(time.Hour), nil)); err != nil {
		t.Fatal(err)
	}
	if live, err := LiveAttempts(root, probe); err != nil || len(live) != 0 {
		t.Fatalf("live attempts after reconciliation = %+v, %v", live, err)
	}
}
