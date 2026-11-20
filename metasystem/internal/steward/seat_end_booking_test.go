package steward

import (
	"bytes"
	"encoding/json"
	"errors"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestTickBooksEndedReservationWithoutAHeldGoal(t *testing.T) {
	t.Parallel()
	root, home := t.TempDir(), t.TempDir()
	now := time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)
	start, end := now.Add(-10*time.Minute).Format(time.RFC3339Nano), now.Add(-6*time.Minute).Format(time.RFC3339Nano)
	stamp := map[string]json.RawMessage{}
	stamp["unitRun"], _ = json.Marshal("run-1")
	stamp["unitInstallation"], _ = json.Marshal(root)
	exit := 0
	if err := (launch.Store{Root: filepath.Join(home, "launch")}).Create(launch.Record{ID: "launch-1", Kind: "proof", Goal: "paused-goal", State: launch.Completed, StartedAt: start, FinishedAt: end, ExitCode: &exit, AdapterData: stamp}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "artifacts/agents/jobs/launch-1.json")
	writeStewardRecord(t, path, map[string]any{"jobId": "launch-1", "operationId": "unit-launch:launch-1", "unitRun": "run-1", "goalId": "paused-goal", "goalRevision": 1, "status": "pending", "createdAt": start, "capMin": 20})
	seatBusyReader(home, 5*time.Minute, seatBusyDeadProber{})(root, goal.ClaimableBudgetedWork{}, now)
	data, err := os.ReadFile(path)
	var job map[string]any
	if err != nil || json.Unmarshal(data, &job) != nil {
		t.Fatal("job unreadable", err)
	}
	if job["status"] != "completed" || job["startedAt"] != start || job["endedAt"] != end || job["unitExecuted"] != true {
		t.Fatalf("finished launch still holds its reservation: %s", data)
	}
	if _, open := OpenJob(root); open {
		t.Fatal("finished job still blocks engine follow")
	}
}

// Only resolving the tree's identity is allowed during booking. In particular,
// snapshots, index writes and restoring excluded files would fail this fixture.
type endBookingGit struct{ t *testing.T }

func (g endBookingGit) Run(dir string, _ []string, args ...string) ([]byte, error) {
	if strings.Join(args, " ") != "rev-parse --show-toplevel" {
		g.t.Fatalf("booking touched the worktree: %v", args)
	}
	return []byte(dir), nil
}

type endBookingProber struct{ state identity.Liveness }

func (p endBookingProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	return identity.Exact{Pid: pid, StartedAt: time.Unix(pid, 0)}, p.state, nil
}

type endBookingProcesses struct {
	launch.OSProcesses
	group bool
	err   error
}

func (p endBookingProcesses) GroupAlive(int64) (bool, error) { return p.group, p.err }
func (p endBookingProcesses) StartChild(launch.Command) (launch.Child, identity.Ref, error) {
	panic("booking started a child")
}
func (p endBookingProcesses) SignalGroup(int64, syscall.Signal) error {
	panic("booking signalled a child")
}

type endBookingBed struct {
	t                            *testing.T
	root, home, runPath, jobPath string
	now                          time.Time
	runner                       *launch.UnitRunner
	run                          launch.UnitRunRecord
	execution                    launch.Record
	job                          map[string]any
}

func newEndBookingBed(t *testing.T) *endBookingBed {
	t.Helper()
	b := &endBookingBed{t: t, root: t.TempDir(), home: t.TempDir(), now: time.Date(2026, 11, 20, 8, 0, 0, 0, time.UTC)}
	b.runner = seatEndRunner(b.root, b.home, b.now, seatBusyDeadProber{})
	b.runner.Git = endBookingGit{t}
	b.runner.Manager.Now = func() time.Time { return b.now }
	b.runner.Manager.Processes = endBookingProcesses{}
	start, end := b.now.Add(-10*time.Minute).Format(time.RFC3339Nano), b.now.Add(-6*time.Minute).Format(time.RFC3339Nano)
	worktree := t.TempDir()
	b.runPath = filepath.Join(b.home, "unit/run-1/run.json")
	b.jobPath = filepath.Join(b.root, "artifacts/agents/jobs/launch-1.json")
	b.run = launch.UnitRunRecord{ID: "run-1", Goal: "held", Unit: "U", Worktree: worktree, State: "running", Plan: filepath.Join(b.home, "legacy-plan.json"), Rounds: []launch.UnitRound{{Number: 1, Directory: filepath.Join(b.home, "unit/run-1/round-1"), Steps: []launch.UnitStep{{Name: "build", Kind: "build", State: launch.StepRunning, LaunchID: "launch-1", LaunchIDs: []string{"launch-1"}, StartedAt: start}, {Name: "build:next", State: launch.StepPending}}}}}
	// An unreadable legacy plan cannot prevent booking facts.
	if err := os.WriteFile(b.run.Plan, []byte(`{"proof":[{"env":["LEGACY_FIXTURE"]}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, "uncommitted.go"), []byte("builder changes\n"), 0600); err != nil {
		t.Fatal(err)
	}
	stamp := map[string]json.RawMessage{}
	stamp["unitRun"], _ = json.Marshal(b.run.ID)
	stamp["unitInstallation"], _ = json.Marshal(b.root)
	exit := 0
	b.execution = launch.Record{ID: "launch-1", Kind: "build", Goal: b.run.Goal, State: launch.Completed, StartedAt: start, FinishedAt: end, ExitCode: &exit, AdapterData: stamp}
	b.job = map[string]any{"jobId": "launch-1", "operationId": "unit-launch:launch-1", "unitRun": b.run.ID, "goalId": b.run.Goal, "goalRevision": 1, "status": "pending", "createdAt": start, "capMin": 20, "unitRound": 1}
	b.save()
	return b
}
func (b *endBookingBed) save() {
	b.t.Helper()
	writeBookingJSON(b.t, b.runPath, b.run)
	writeStewardRecord(b.t, b.jobPath, b.job)
	writeBookingJSON(b.t, filepath.Join(b.home, "launch/launch-1/record.json"), b.execution)
}
func (b *endBookingBed) tick() {
	b.t.Helper()
	seatBusyOrphanReader(b.home, 5*time.Minute, b.runner.Manager.Prober, nil, b.runner)(b.root, goal.ClaimableBudgetedWork{}, b.now)
}
func bookingBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestLightCycleBooksFinishedWorkBehindScheduledGates(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"allowed", "first tick", "collection held", "seat held", "unreadable prerequisite", "live child", "young finish"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			b := newEndBookingBed(t)
			cfg := TickConfig{Now: b.now, scheduledProgress: &scheduledProgress{collection: true, seat: true}}
			switch name {
			case "first tick":
				cfg.scheduledProgress = nil
			case "collection held":
				cfg.scheduledProgress.collection = false
			case "seat held":
				cfg.scheduledProgress.seat = false
			case "unreadable prerequisite":
				writeLightPoison(t, validationWindowPath(b.root))
			case "live child":
				b.execution.Child = &identity.Ref{Pid: 41, StartedAtSec: 41}
				b.runner.Manager.Prober = endBookingProber{identity.Alive}
			case "young finish":
				b.execution.FinishedAt = b.now.Add(-4 * time.Minute).Format(time.RFC3339Nano)
			}
			b.save()
			work := openWorkDependencies{
				NewWorld: func(string) bool { return true },
				ReadClaimableBudgetedWork: func(string, time.Time) (goal.ClaimableBudgetedWork, error) {
					return goal.ClaimableBudgetedWork{}, nil
				},
				Busy: seatBusyOrphanReader(b.home, 5*time.Minute, b.runner.Manager.Prober, nil, b.runner),
			}
			runBefore, jobBefore := bookingBytes(t, b.runPath), bookingBytes(t, b.jobPath)
			if _, err := runLightDecisionsWith(b.root, cfg, deadCensus(), work, ReapContinuations); err != nil {
				t.Fatal(err)
			}
			if name != "allowed" {
				if !bytes.Equal(runBefore, bookingBytes(t, b.runPath)) || !bytes.Equal(jobBefore, bookingBytes(t, b.jobPath)) {
					t.Fatal("light booking crossed a scheduled, age or custody boundary")
				}
				return
			}
			run, err := b.runner.Status(b.run.ID)
			if err != nil {
				t.Fatal(err)
			}
			var job map[string]any
			if err := json.Unmarshal(bookingBytes(t, b.jobPath), &job); err != nil {
				t.Fatal(err)
			}
			if run.State != launch.UnitStepEnded || run.Rounds[0].Steps[0].EndBooking == nil || run.Rounds[0].Outcome != "" || run.Rounds[0].Steps[1].State != launch.StepPending || job["status"] != "completed" {
				t.Fatalf("light collection did not retain only the finished facts: run=%+v job=%+v", run, job)
			}
			if string(bookingBytes(t, filepath.Join(b.run.Worktree, "uncommitted.go"))) != "builder changes\n" {
				t.Fatal("light collection changed builder output")
			}
		})
	}
}

func TestTickBooksOnlyFinishedCheckoutWork(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		change           func(*endBookingBed)
		wantRun, wantJob bool
	}{
		{"completed", func(*endBookingBed) {}, true, true},
		{"no proof or read", func(b *endBookingBed) {
			b.run.Rounds[0].Steps = b.run.Rounds[0].Steps[:1]
			writeBookingJSON(b.t, b.run.Plan, map[string]any{"unit": "U", "goal": "held", "worktree": b.run.Worktree, "base": "base", "build": map[string]any{"brief": "build.md"}, "proof": []any{}})
		}, true, true},
		{"failed", func(b *endBookingBed) { b.execution.State = launch.Failed }, true, true},
		{"cancelled launch", func(b *endBookingBed) { b.execution.State = launch.Cancelled }, true, true},
		{"already completed job", func(b *endBookingBed) { b.job["status"] = "completed" }, true, false},
		{"old unit with checkout reservation", func(b *endBookingBed) { b.execution.AdapterData = nil }, true, true},
		{"paused goal", func(b *endBookingBed) { b.run.Goal = "paused"; b.execution.Goal = "paused"; b.job["goalId"] = "paused" }, true, true},
		{"other checkout same goal", func(b *endBookingBed) { b.execution.AdapterData["unitInstallation"], _ = json.Marshal(b.t.TempDir()) }, false, false},
		{"person cancelled run", func(b *endBookingBed) { b.run.State = "cancelled"; b.job["status"] = "cancelled" }, false, false},
		{"nonterminal dead supervisor", func(b *endBookingBed) { b.execution.State = launch.Running }, false, false},
		{"alive child", func(b *endBookingBed) {
			b.execution.Child = &identity.Ref{Pid: 41, StartedAtSec: 41}
			b.runner.Manager.Prober = endBookingProber{identity.Alive}
		}, false, false},
		{"unknown child", func(b *endBookingBed) {
			b.execution.Child = &identity.Ref{Pid: 41, StartedAtSec: 41}
			b.runner.Manager.Prober = endBookingProber{identity.Unknown}
		}, false, false},
		{"alive supervisor", func(b *endBookingBed) {
			b.execution.Supervisor = &identity.Ref{Pid: 41, StartedAtSec: 41}
			b.runner.Manager.Prober = endBookingProber{identity.Alive}
		}, false, false},
		{"alive group", func(b *endBookingBed) {
			b.execution.ProcessGroup = &identity.Ref{Pid: 41, StartedAtSec: 41}
			b.runner.Manager.Processes = endBookingProcesses{group: true}
		}, false, false},
		{"unknown group", func(b *endBookingBed) {
			b.execution.ProcessGroup = &identity.Ref{Pid: 41, StartedAtSec: 41}
			b.runner.Manager.Processes = endBookingProcesses{err: errors.New("unreadable group")}
		}, false, false},
		{"young finish", func(b *endBookingBed) { b.execution.FinishedAt = b.now.Add(-4 * time.Minute).Format(time.RFC3339Nano) }, false, false},
		{"exact interval", func(b *endBookingBed) { b.execution.FinishedAt = b.now.Add(-5 * time.Minute).Format(time.RFC3339Nano) }, false, false},
		{"missing time", func(b *endBookingBed) { b.execution.FinishedAt = "" }, false, false},
		{"missing kind", func(b *endBookingBed) { b.execution.Kind = "" }, false, false},
		{"missing goal", func(b *endBookingBed) { b.execution.Goal = "" }, false, false},
		{"missing successful exit", func(b *endBookingBed) { b.execution.ExitCode = nil }, false, false},
		{"regressed time", func(b *endBookingBed) { b.execution.FinishedAt = b.now.Add(-20 * time.Minute).Format(time.RFC3339Nano) }, false, false},
		{"unknown format", func(b *endBookingBed) { b.execution.State = "future-state" }, false, false},
		{"unstamped publication", func(b *endBookingBed) {
			b.execution.Kind = "proof"
			b.execution.AdapterData = nil
			b.run.Rounds[0].Steps = nil
			b.run.Subjects = []launch.UnitSubject{{Round: 1, GateLaunches: []string{"launch-1"}}}
		}, false, false},
		{"stamped publication", func(b *endBookingBed) {
			b.execution.Kind = "proof"
			b.run.Rounds[0].Steps = nil
			b.run.Subjects = []launch.UnitSubject{{Round: 1, GateLaunches: []string{"launch-1"}}}
		}, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := newEndBookingBed(t)
			tc.change(b)
			b.save()
			runBefore, jobBefore := bookingBytes(t, b.runPath), bookingBytes(t, b.jobPath)
			b.tick()
			runAfter, jobAfter := bookingBytes(t, b.runPath), bookingBytes(t, b.jobPath)
			if bytes.Equal(runBefore, runAfter) == tc.wantRun || bytes.Equal(jobBefore, jobAfter) == tc.wantJob {
				t.Fatalf("booking changed run=%v job=%v; want %v %v", !bytes.Equal(runBefore, runAfter), !bytes.Equal(jobBefore, jobAfter), tc.wantRun, tc.wantJob)
			}
			if tc.wantRun {
				run, err := b.runner.Status(b.run.ID)
				if err != nil {
					t.Fatal(err)
				}
				step := run.Rounds[0].Steps[0]
				want := launch.StepPassed
				if b.execution.State != launch.Completed {
					want = launch.StepFailed
				}
				if run.State != launch.UnitStepEnded || step.State != want || step.ExecutionEndedAt != b.execution.FinishedAt || step.EndBooking == nil || step.EndBooking.By != "steward" || step.EndBooking.Launch != b.execution.ID || step.EndBooking.State != b.execution.State || step.EndBooking.At != b.now.Format(time.RFC3339Nano) || run.Rounds[0].Outcome != "" || run.Rounds[0].Stop != nil || len(run.Rounds[0].Steps) > 1 && run.Rounds[0].Steps[1].State != launch.StepPending {
					t.Fatalf("wrong booked facts: %+v %+v", run, step)
				}
			}
			if string(bookingBytes(t, filepath.Join(b.run.Worktree, "uncommitted.go"))) != "builder changes\n" {
				t.Fatal("builder changes lost")
			}
			var logBefore []byte
			logBefore, _ = os.ReadFile(runnerLogPath(b.root))
			runInfo, _ := os.Stat(b.runPath)
			jobInfo, _ := os.Stat(b.jobPath)
			b.tick()
			if !bytes.Equal(runAfter, bookingBytes(t, b.runPath)) || !bytes.Equal(jobAfter, bookingBytes(t, b.jobPath)) {
				t.Fatal("repeat changed records")
			}
			logAfter, _ := os.ReadFile(runnerLogPath(b.root))
			if !bytes.Equal(logBefore, logAfter) {
				t.Fatal("repeat duplicated log")
			}
			runInfoAfter, _ := os.Stat(b.runPath)
			jobInfoAfter, _ := os.Stat(b.jobPath)
			if !runInfo.ModTime().Equal(runInfoAfter.ModTime()) || !jobInfo.ModTime().Equal(jobInfoAfter.ModTime()) {
				t.Fatal("repeat rewrote records")
			}
		})
	}
}

func TestTickBookingRetriesAfterIntervalAndSkipsLockedOrBadRecords(t *testing.T) {
	t.Parallel()
	t.Run("next interval", func(t *testing.T) {
		t.Parallel()
		b := newEndBookingBed(t)
		b.execution.FinishedAt = b.now.Add(-5 * time.Minute).Format(time.RFC3339Nano)
		b.save()
		b.tick()
		b.now = b.now.Add(time.Nanosecond)
		b.tick()
		run, _ := b.runner.Status(b.run.ID)
		if run.State != launch.UnitStepEnded {
			t.Fatal("next eligible tick did not book")
		}
	})
	for _, broken := range []string{"run locked", "run lock unopenable", "corrupt launch", "missing launch", "unrelated corrupt run", "log unavailable"} {
		t.Run(broken, func(t *testing.T) {
			t.Parallel()
			b := newEndBookingBed(t)
			switch broken {
			case "run locked":
				held, err := lock.File(filepath.Join(filepath.Dir(b.runPath), ".lock"), 0600, lock.TryExclusive)
				if err != nil {
					t.Fatal(err)
				}
				defer held.Release()
			case "run lock unopenable":
				if err := os.Mkdir(filepath.Join(filepath.Dir(b.runPath), ".lock"), 0700); err != nil {
					t.Fatal(err)
				}
			case "corrupt launch":
				if err := os.WriteFile(filepath.Join(b.home, "launch/launch-1/record.json"), []byte("{"), 0600); err != nil {
					t.Fatal(err)
				}
			case "missing launch":
				if err := os.Remove(filepath.Join(b.home, "launch/launch-1/record.json")); err != nil {
					t.Fatal(err)
				}
			case "unrelated corrupt run":
				writeBookingJSON(t, filepath.Join(b.home, "unit/bad/run.json"), "bad")
			case "log unavailable":
				if err := os.MkdirAll(runnerLogPath(b.root), 0700); err != nil {
					t.Fatal(err)
				}
			}
			before := bookingBytes(t, b.runPath)
			b.tick()
			run, err := b.runner.Status(b.run.ID)
			if err != nil {
				t.Fatal("partial run write", err)
			}
			if broken == "unrelated corrupt run" || broken == "log unavailable" {
				if run.State != launch.UnitStepEnded {
					t.Fatal("other failure blocked valid booking")
				}
			} else if !bytes.Equal(before, bookingBytes(t, b.runPath)) {
				t.Fatal("blocked run changed")
			}
			if broken == "run locked" {
				data, _ := os.ReadFile(runnerLogPath(b.root))
				if strings.Contains(string(data), "end booking left run") {
					t.Fatal("busy lock logged as failure")
				}
			}
		})
	}
}

func writeBookingJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}
