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
	"github.com/widoriezebos/agentic-tools/metasystem/internal/seat"
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

func seatBusyReadBed(t *testing.T, state launch.UnitStepState, age time.Duration) (*seatBed, TickConfig, openWorkDependencies) {
	t.Helper()
	bed, units, work, _ := seatBusyFixture(t)
	if state != "" {
		writeStewardRecord(t, filepath.Join(units, "run-1", "run.json"), map[string]any{"id": "run-1", "goal": "held", "state": "running", "rounds": []launch.UnitRound{{Steps: []launch.UnitStep{{Name: "r1-s3", Kind: "read", LaunchID: "launch-r1-s3", State: state, StartedAt: bed.now.Add(-age).Format(time.RFC3339)}}}}})
	}
	deps := bed.dependencies()
	deps.ReadClaimableBudgetedWork = func(string, time.Time) (goal.ClaimableBudgetedWork, error) { return work, nil }
	cfg := TickConfig{WorkStateRoot: filepath.Dir(units), Prober: seatBusyDeadProber{}, Runner: &seat.RunnerContext{TickSeconds: 300}}
	return bed, cfg, deps
}

func assertSeatBusyRead(t *testing.T, bed *seatBed, cfg TickConfig, deps openWorkDependencies, wantBusy bool, wantReason string) {
	t.Helper()
	cfg.Now = bed.now
	d, selection, reason, err := decideNowWithSeat(bed.root, cfg, fakeCensus{workers: deadWorkers}, &Evidence{}, func(runtime string, err error) (outage.Mark, bool) {
		return standingProviderOutage(runtime, bed.now, nil, testprovider.Home(bed.root))
	}, deps, &seatTickState{})
	if err != nil || d.Verdict == VerdictDegraded {
		t.Fatalf("seat decision failed: %+v %v", d, err)
	}
	if wantBusy && (selection != nil || d.Verdict != VerdictHealthy || d.Action != ActNone || reason != wantReason) || !wantBusy && (selection == nil || selection.Goal != "held") {
		t.Errorf("seat decision: %+v selection=%+v reason=%q; want busy=%v reason=%q", d, selection, reason, wantBusy, wantReason)
	}
	work, err := deps.ReadClaimableBudgetedWork(bed.root, bed.now)
	if err != nil {
		t.Fatal(err)
	}
	busy, reason, skipped := defaultTickContinuationDependencies(cfg).openWork.Busy(bed.root, work, bed.now)
	if busy != wantBusy || skipped != 0 || wantBusy && reason != wantReason {
		t.Errorf("continuation reader: busy=%v reason=%q skipped=%d; want busy=%v reason=%q", busy, reason, skipped, wantBusy, wantReason)
	}
}

func TestAStartThatNeverLaunchedDoesNotHoldTheSeat(t *testing.T) {
	t.Parallel()
	bed, cfg, deps := seatBusyReadBed(t, launch.StepStarting, 6*time.Minute)
	message := "start never launched: run run-1 of goal held (step r1-s3 since " + bed.now.Add(-6*time.Minute).Format(time.RFC3339) + ", no launch record)"
	for tick := 0; tick < 2; tick++ {
		assertSeatBusyRead(t, bed, cfg, deps, false, "")
		bed.now = bed.now.Add(time.Minute)
	}
	data, err := os.ReadFile(runnerLogPath(bed.root))
	if err != nil || strings.Count(string(data), message) != 1 {
		t.Fatalf("unlaunched start must be reported once: %s %v", data, err)
	}
}

func TestAStartYoungerThanOneTickStillHoldsTheSeat(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		age             time.Duration
		manual, present bool
		busy            bool
	}{
		{"expired-control", 6 * time.Minute, false, false, false},
		{"young", 4 * time.Minute, false, false, true},
		{"manual-tick", 6 * time.Minute, true, false, true},
		{"launch-present", 6 * time.Minute, false, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed, cfg, deps := seatBusyReadBed(t, launch.StepStarting, tc.age)
			if tc.manual {
				cfg.Runner = nil
			}
			if tc.present {
				if err := (launch.Store{Root: filepath.Join(cfg.WorkStateRoot, "launch")}).Create(launch.Record{ID: "launch-r1-s3", State: launch.Starting}); err != nil {
					t.Fatal(err)
				}
			}
			assertSeatBusyRead(t, bed, cfg, deps, tc.busy, "unit run-1 step r1-s3 for goal held is starting")
			if data, err := os.ReadFile(runnerLogPath(bed.root)); tc.busy && (!os.IsNotExist(err) || len(data) != 0) {
				t.Fatalf("held start logged as orphan: %s %v", data, err)
			}
		})
	}
}

func TestAnEndedUnitLaunchReservationDoesNotHoldTheSeat(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		state        launch.State
		age          time.Duration
		operation    string
		manual, busy bool
	}{
		{"ended", launch.Completed, time.Minute, "unit-launch:launch-r1-s3", false, false},
		{"never-started", "", 6 * time.Minute, "unit-launch:launch-r1-s3", false, false},
		{"still-running", launch.Running, 30 * time.Minute, "unit-launch:launch-r1-s3", false, true},
		{"young", "", 4 * time.Minute, "unit-launch:launch-r1-s3", false, true},
		{"other-operation", "", 6 * time.Hour, "reserve-two", false, true},
		{"ended-zero-tick", launch.Completed, 6 * time.Hour, "unit-launch:launch-r1-s3", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			bed, cfg, deps := seatBusyReadBed(t, "", 0)
			writeStewardRecord(t, filepath.Join(bed.root, "artifacts", "agents", "jobs", "launch-r1-s3.json"), map[string]any{
				"jobId": "launch-r1-s3", "operationId": tc.operation, "unitRun": "run-1", "unitRound": 1,
				"goalId": "held", "goalRevision": 1, "machineId": seatBedMachine, "claimEpoch": 1,
				"capMin": 20, "status": "pending", "createdAt": bed.now.Add(-tc.age).Format(time.RFC3339Nano),
			})
			if tc.state != "" {
				if err := (launch.Store{Root: filepath.Join(cfg.WorkStateRoot, "launch")}).Create(launch.Record{ID: "launch-r1-s3", State: tc.state, Supervisor: &identity.Ref{Pid: 41, StartedAtSec: 100}}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.manual {
				cfg.Runner = nil
			}
			for tick := 0; tick < 2; tick++ {
				assertSeatBusyRead(t, bed, cfg, deps, tc.busy, "dependent job launch-r1-s3.json is starting")
				bed.now = bed.now.Add(time.Minute)
			}
			data, err := os.ReadFile(runnerLogPath(bed.root))
			if tc.busy {
				if !os.IsNotExist(err) || len(data) != 0 {
					t.Fatalf("held reservation logged as ended: %s %v", data, err)
				}
			} else {
				ended := string(tc.state)
				if ended == "" {
					ended = "no launch record"
				}
				message := "ended reservation launch-r1-s3.json of goal held (launch launch-r1-s3: " + ended + ")"
				if err != nil || strings.Count(string(data), message) != 1 {
					t.Fatalf("ended reservation must be reported once: %s %v", data, err)
				}
			}
		})
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
