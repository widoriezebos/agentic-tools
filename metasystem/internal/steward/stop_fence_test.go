package steward

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

func closeProcessFence(t *testing.T, root string, generation int64) {
	t.Helper()
	if err := stopfence.Write(root, stopfence.Record{
		State: stopfence.StateClosed, Phase: stopfence.PhaseStopped,
		Generation: generation, ChangedAt: "2026-09-07T08:00:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 75}},
	}); err != nil {
		t.Fatal(err)
	}
}

type countingFenceCensus struct{ calls int }

func (c *countingFenceCensus) Workers(string) (Workers, error) {
	c.calls++
	return Workers{Live: 1, CensusComplete: true}, nil
}

func TestRunnerCreationReadersReturnFencedWithoutCreating(t *testing.T) {
	root := t.TempDir()
	closeProcessFence(t, root, 3)

	ensured, err := EnsureRunner(root, nil, 1000)
	if err != nil || ensured.Action != "FENCED" || ensured.Generation != 3 {
		t.Fatalf("ensure did not return FENCED: %+v %v", ensured, err)
	}
	repaired, err := RepairEnrolledRunner(root)
	if err != nil || repaired.Status != "FENCED" || repaired.Generation != 3 {
		t.Fatalf("repair did not return FENCED: %+v %v", repaired, err)
	}
	for name, call := range map[string]func() error{
		"arm":     func() error { _, err := Arm(root, "/bin/true"); return err },
		"restart": func() error { _, err := Restart(root, "/bin/true"); return err },
		"run":     func() error { return RunLoop(root, fakeCensus{}, nil, time.Hour) },
	} {
		err := call()
		var stopped *StoppedError
		if !errors.As(err, &stopped) || !strings.Contains(err.Error(), "since 2026-09-07T08:00:00Z, by stop pid 75") {
			t.Fatalf("%s did not return the stopped refusal: %v", name, err)
		}
	}
	if _, err := os.Stat(runnerDir(root)); !os.IsNotExist(err) {
		t.Fatalf("a fenced reader created the runner directory: %v", err)
	}
}

func TestRunLoopClosesItsCreationClaimAfterASecondFenceRead(t *testing.T) {
	root := gitRepoWithCurrentGoal(t)
	census := &countingFenceCensus{}
	called := false
	runnerAfterRecordPublished = func() {
		called = true
		claims, err := stopfence.Claims(root, 0)
		if err != nil || len(claims) != 1 || claims[0].Verb != "steward-run" {
			t.Fatalf("runner publication was not covered by one creation claim: %+v %v", claims, err)
		}
		closeProcessFence(t, root, 1)
	}
	t.Cleanup(func() { runnerAfterRecordPublished = nil })

	err := RunLoop(root, census, nil, time.Hour)
	var stopped *StoppedError
	if !called || !errors.As(err, &stopped) || !stopped.Raced {
		t.Fatalf("runner did not end itself after the fence changed: called=%t err=%v", called, err)
	}
	if _, err := os.Stat(runnerRecordPath(root)); !os.IsNotExist(err) {
		t.Fatalf("raced runner record survived: %v", err)
	}
	claims, err := stopfence.Claims(root, 1)
	if err != nil || len(claims) != 0 {
		t.Fatalf("raced runner creation claim survived: %+v %v", claims, err)
	}
	if _, err := os.Stat(EvidencePath(root)); !os.IsNotExist(err) {
		t.Fatalf("raced runner completed a first tick: %v", err)
	}
	if census.calls != 0 {
		t.Fatalf("raced runner reached its first worker census: calls=%d", census.calls)
	}
}

func TestDisarmReportsRunnerThatSurvivesKill(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.runtimes=fake\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("METASYSTEM_FIXTURE_CAP_SCALE_MILLI", "1")
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("read test identity: %v %s", err, state)
	}
	if err := writeJSONAtomic(runnerRecordPath(root), RunnerRecord{
		Pid: self.Pid, PidStartedAt: self.StartedAt.Unix(), StartTicks: self.StartTicks,
		BootID: self.BootID, StartedAt: time.Now().UTC().Format(time.RFC3339), FenceGeneration: 2,
	}); err != nil {
		t.Fatal(err)
	}
	originalSignal := runnerSignal
	var signals []syscall.Signal
	runnerSignal = func(_ int, signal syscall.Signal) error {
		signals = append(signals, signal)
		return nil
	}
	t.Cleanup(func() { runnerSignal = originalSignal })

	outcome, err := Disarm(root)
	if err != nil || outcome.Result != "not-stopped" || outcome.Signal != "kill" ||
		outcome.Record.FenceGeneration != 2 || !strings.Contains(outcome.Reason, "remained alive") {
		t.Fatalf("surviving runner outcome is not honest: %+v %v", outcome, err)
	}
	if len(signals) != 2 || signals[0] != syscall.SIGTERM || signals[1] != syscall.SIGKILL {
		t.Fatalf("disarm did not use TERM then KILL: %v", signals)
	}
	if current, alive := liveRunner(root); !alive || !sameRunner(outcome.Record, current) {
		t.Fatalf("the real recorded runner did not survive the no-op KILL seam: current=%+v alive=%t", current, alive)
	}
}

func TestStoppedHealthDoesNotAdvanceFailureCounters(t *testing.T) {
	root := t.TempDir()
	closeProcessFence(t, root, 1)
	first, err := ObserveHealth(root, time.Now(), nil)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ObserveHealth(root, time.Now().Add(time.Second), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(first.Line(), "HEALTH STOPPED ") ||
		!strings.HasPrefix(second.Line(), "HEALTH STOPPED ") || first.ShouldAlert || second.ShouldAlert {
		t.Fatalf("closed-fence health was not quiet and visibly stopped: %q %+v", first.Line(), second)
	}
	if first.State.Sequence != second.State.Sequence {
		t.Fatalf("stopped health advanced observation state: %d -> %d", first.State.Sequence, second.State.Sequence)
	}
	for _, role := range second.Roles {
		if role.ConsecutiveFailures != 0 || role.ConsecutiveUnknown != 0 || role.FailureEscalation != "" {
			t.Fatalf("stopped health advanced %s counters: %+v", role.Role, role)
		}
		if strings.Contains(role.Remedy, "metasystem up") {
			t.Fatalf("stopped health retained an up remedy: %+v", role)
		}
	}
}

func TestStoppedHealthRemedyPrintsPathWithoutQuotes(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout with spaces")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	closeProcessFence(t, root, 1)
	roles := []RoleVerdict{{Role: RoleSupervisionOwner, Status: HealthDead, Remedy: "metasystem up --repo obsolete"}}
	verdict, err := healthStopped(root, time.Now(), roles, SpendObservation{}, HealthObservationState{})
	if err != nil || verdict == nil || len(verdict.Roles) != 1 {
		t.Fatalf("stopped health = %#v err=%v", verdict, err)
	}
	want := "metasystem arm --repo " + root
	if got := verdict.Roles[0].Remedy; got != want || strings.Contains(got, `"`) {
		t.Fatalf("stopped remedy = %q, want %q", got, want)
	}
}

func TestStoppedHealthLineNamesIncompleteAndUnfinishedPhases(t *testing.T) {
	for _, test := range []struct {
		phase      string
		unresolved int
		want       string
	}{
		{phase: stopfence.PhaseStopIncomplete, want: "HEALTH STOP INCOMPLETE "},
		{phase: stopfence.PhaseStopping, want: "HEALTH STOP UNFINISHED "},
		{phase: stopfence.PhaseStopped, unresolved: 1, want: "HEALTH STOP INCOMPLETE "},
	} {
		verdict := HealthVerdict{Stopped: true, StopPhase: test.phase, StopUnresolved: test.unresolved, Aggregate: "healthy"}
		if line := verdict.Line(); !strings.HasPrefix(line, test.want) {
			t.Fatalf("phase %s health line = %q, want prefix %q", test.phase, line, test.want)
		}
	}
}
