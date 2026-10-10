package steward

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

type seatBusyDeadProber struct{}

func (seatBusyDeadProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid}, identity.Dead, nil
}

func seatBusyFixture(t *testing.T) (*seatBed, string, goal.ClaimableBudgetedWork, SeatBusyOptions) {
	t.Helper()
	bed := newSeatBed(t, seatClaimedGoal("held", SeatLineage), seatReadyGoal("ready", "Build it."))
	bed.now = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	work, err := goal.ClaimableWorkFromProjection(bed.projection(bed.now), seatBedMachine, seatBusyDeadProber{})
	if err != nil {
		t.Fatal(err)
	}
	options := SeatBusyOptions{Now: bed.now, Alive: func(ref identity.Ref) bool { return ref.Pid == 41 && ref.StartedAtSec == 100 }}
	return bed, filepath.Join(t.TempDir(), "unit"), work, options
}

func seatBusyRun(t *testing.T, units string, now time.Time, pid int64) {
	t.Helper()
	writeStewardRecord(t, filepath.Join(units, "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "build", LaunchID: "launch-1", State: launch.StepRunning, StartedAt: now.Add(-time.Minute).Format(time.RFC3339)}}}}})
	store := launch.Store{Root: filepath.Join(filepath.Dir(units), "launch")}
	if err := store.Create(launch.Record{ID: "launch-1", State: launch.Running, Supervisor: &identity.Ref{Pid: pid, StartedAtSec: 100}}); err != nil {
		t.Fatal(err)
	}
}

func seatBusyDependencies(bed *seatBed, units string, work goal.ClaimableBudgetedWork, options SeatBusyOptions) openWorkDependencies {
	deps := bed.dependencies()
	deps.ReadClaimableBudgetedWork = func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return work, nil }
	deps.Busy = func(root string, work goal.ClaimableBudgetedWork, now time.Time) (bool, string, int) {
		options.Now = now
		return SeatBusyAt(root, units, work, options)
	}
	return deps
}

func TestSeatBusyPreventsIdleVerdict(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	seatBusyRun(t, units, bed.now, 41)
	deps := seatBusyDependencies(bed, units, work, options)
	d, selection, _, err := decideNowWithSeat(bed.root, TickConfig{Now: bed.now}, fakeCensus{workers: deadWorkers}, &Evidence{}, func(runtime string, err error) (outage.Mark, bool) {
		return standingProviderOutage(runtime, bed.now, nil, testprovider.Home(bed.root))
	}, deps, &seatTickState{})
	if err != nil || d.Verdict != VerdictHealthy || d.Action != ActNone || selection != nil {
		t.Fatalf("live unit declared idle: %+v %v", d, err)
	}
	busy, reason, skipped := SeatBusyAt(bed.root, units, goal.ClaimableBudgetedWork{Claimed: []string{"foreign"}}, options)
	if busy || reason != "" || skipped != 0 {
		t.Fatalf("foreign work counted: %v %q %d", busy, reason, skipped)
	}
}

func TestSeatBusyOrphanDoesNotHoldTheSeat(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	seatBusyRun(t, units, bed.now.Add(-time.Hour), 99)
	deps := seatBusyDependencies(bed, units, work, options)
	for tick := 0; tick < 2; tick++ {
		d, selection, _, err := decideNowWithSeat(bed.root, TickConfig{Now: bed.now}, fakeCensus{workers: deadWorkers}, &Evidence{}, func(runtime string, err error) (outage.Mark, bool) {
			return standingProviderOutage(runtime, bed.now, nil, testprovider.Home(bed.root))
		}, deps, &seatTickState{})
		if err != nil || d.Verdict == VerdictDegraded || selection == nil {
			t.Fatalf("an orphan prevented a seat launch: %+v %+v %v", d, selection, err)
		}
		bed.now = bed.now.Add(time.Minute)
	}
	data, err := os.ReadFile(runnerLogPath(bed.root))
	if err != nil {
		t.Fatal(err)
	}
	message := "orphan run run-1 of goal held (step build since 2026-09-01T08:59:00Z)"
	if strings.Count(string(data), message) != 1 {
		t.Fatalf("orphan must be reported once: %s", data)
	}
}

func TestSeatBusyForeignMalformedRecordDoesNotDegrade(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	path := filepath.Join(units, "foreign_invalid_name", "run.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"goal":"foreign","rounds":broken}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writeStewardRecord(t, filepath.Join(bed.root, "artifacts", "agents", "jobs", "code-critic-foreign.json"), map[string]any{"goalId": "foreign", "status": "running", "pid": 41, "pidStartedAt": 100})
	deps := seatBusyDependencies(bed, units, work, options)
	answer, reason, _, err := convertedOpenWorkWithDependencies(bed.root, deps)
	if err != nil || answer != WorkClaimable || strings.Contains(reason, "unreadable") {
		t.Fatalf("a foreign record degraded or occupied the seat: %s %q %v", answer, reason, err)
	}
}

func TestSeatBusyUnreadableRecordIsCounted(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	path := filepath.Join(units, "broken", "run.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"goal":"held","rounds":broken}`), 0o644); err != nil {
		t.Fatal(err)
	}
	busy, reason, skipped := SeatBusyAt(bed.root, units, work, options)
	if !busy || skipped != 1 || !strings.Contains(reason, "custody is unknown") {
		t.Fatalf("bad record was not skipped: %v %q %d", busy, reason, skipped)
	}
	deps := seatBusyDependencies(bed, units, work, options)
	answer, reason, _, err := convertedOpenWorkWithDependencies(bed.root, deps)
	if err != nil || answer != WorkInFlight || !strings.Contains(reason, "custody is unknown") {
		t.Fatalf("bad record degraded the tick: %s %q %v", answer, reason, err)
	}
}

func TestSeatBusyCriticRequiresLiveProcess(t *testing.T) {
	t.Parallel()
	for _, pid := range []int64{41, 99} {
		t.Run(time.Unix(pid, 0).Format("05"), func(t *testing.T) {
			t.Parallel()
			bed, units, work, options := seatBusyFixture(t)
			writeStewardRecord(t, filepath.Join(bed.root, "artifacts", "agents", "jobs", "code-critic-1.json"), map[string]any{"goalId": "held", "status": "running", "pid": pid, "pidStartedAt": 100})
			busy, reason, skipped := SeatBusyAt(bed.root, units, work, options)
			if busy != (pid == 41) || skipped != 0 {
				t.Fatalf("critic liveness ignored: %v %q %d", busy, reason, skipped)
			}
		})
	}
}

func TestSeatBusyRunRemainsBusyPastGoalBudget(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	file, ok := work.OwnedClaim("held")
	if !ok {
		t.Fatal("fixture has no held goal")
	}
	limit, _ := goal.ParseWorkingDuration(file.Budget.ElapsedLimit)
	seatBusyRun(t, units, bed.now.Add(-limit+time.Minute), 41)
	busy, reason, skipped := SeatBusyAt(bed.root, units, work, options)
	if !busy || skipped != 0 {
		t.Fatalf("live run past its budget is idle: %v %q %d", busy, reason, skipped)
	}
}

func TestSeatBusyUsesOnlyTheCurrentRound(t *testing.T) {
	t.Parallel()
	bed, units, work, options := seatBusyFixture(t)
	seatBusyRun(t, units, bed.now, 41)
	runner := launch.UnitRunner{Root: units}
	run, err := runner.Status("run-1")
	if err != nil {
		t.Fatal(err)
	}
	run.Rounds = append(run.Rounds, launch.UnitRound{Steps: []launch.UnitStep{{Name: "read", State: launch.StepPassed, StartedAt: bed.now.Format(time.RFC3339)}}})
	writeStewardRecord(t, filepath.Join(units, run.ID, "run.json"), map[string]any{"id": run.ID, "goal": run.Goal, "state": run.State, "rounds": run.Rounds})
	busy, reason, skipped := SeatBusyAt(bed.root, units, work, options)
	if busy || skipped != 0 {
		t.Fatalf("historical step counted as live work: %v %q %d", busy, reason, skipped)
	}
}
