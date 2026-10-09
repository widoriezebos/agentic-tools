package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot"
)

// TestWorkStopGoalCompletesItsRecordedStop: a breach-stopped goal's fence
// lifts only once its stop batch is COMPLETE. The refusal names work stop G,
// which stops the goal's running jobs and advances the stop from the job
// records, as the steward's pass does; a repeat stops nothing, and the
// goal's resume then succeeds.
func TestWorkStopGoalCompletesItsRecordedStop(t *testing.T) {
	t.Parallel()
	bed := newGoalCLIBed(t, goalCLISeed{})
	bed.announceHolder()
	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "4h/6/600m/1/2", "--by", "Wido", gcliForgivingFixture)
	bed.setNow(gcliForgivingBreachAt)
	stopID := gcliForgivingOpenStop(t, bed, "ship-widget", "01ARZ3NDEKTSV4RRFFQ69G7S03")

	code, stdout, stderr := gcliForgivingPublic(bed, "goal", "budget", "ship-widget", "keep", "--by", "Wido", gcliForgivingFixture)
	if code == 0 || !strings.Contains(stdout+stderr, "metasystem work stop ship-widget") {
		t.Fatalf("the fenced goal's resume did not name work stop G: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	gcliForgivingMust(t, bed, "work", "stop", "ship-widget")
	batch, err := goal.ReadStopBatch(bed.root, stopID)
	if err != nil || batch.State != goal.StopBatchComplete {
		t.Fatalf("work stop G did not complete stop batch %s: %+v %v", stopID, batch, err)
	}
	tip := bed.tip()
	if out := gcliForgivingMust(t, bed, "work", "stop", "ship-widget"); !strings.Contains(out, "nothing to stop") || bed.tip() != tip {
		t.Fatalf("a repeated work stop G was not a no-op: %q", out)
	}

	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "keep", "--by", "Wido", gcliForgivingFixture)
	if resumed := bed.goalRecord("ship-widget"); goalCLILine(resumed, "- StopFence:") != "" {
		t.Fatalf("the goal's fence did not lift once its stop completed:\n%s", resumed)
	}

	code, stdout, stderr = gcliForgivingPublic(bed, "work", "stop", "ship-widget")
	if code != 0 || !strings.Contains(stdout+stderr, "No job of goal ship-widget is running; nothing to stop") {
		t.Fatalf("work stop G with nothing running is not a quiet success: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestWorkStopGoalPreservesFinishedRuns(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"completed", "awaiting-judgement"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			b := newWorkBed(t)
			b.starter.hold = "build"
			argv := append([]string{"work", "build", b.id, "--work", "live", "--brief", b.brief("live.md", "Build the unit.\n"), "--lines", "5"}, workCheck...)
			code, built, _ := pendingWork(t, b, argv...)
			if code != 3 {
				t.Fatalf("live build: %d %s", code, built.Summary)
			}
			live := resultData(t, built)["run"].(string)
			const finished = "000-finished"
			retained := launch.UnitRunRecord{ID: finished, Unit: "finished", Goal: b.id, Worktree: b.worktree, State: state}
			if state == "awaiting-judgement" {
				retained.Rounds = []launch.UnitRound{{Number: 1, Outcome: "build-size"}}
			}
			before, err := json.MarshalIndent(retained, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(b.unitRoot, finished)
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "run.json")
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			runs, unknown, err := (&launch.UnitRunner{Root: b.unitRoot}).GoalRuns(b.id)
			if err != nil || len(unknown) != 0 || len(runs) != 2 || runs[0].Run != finished {
				t.Fatalf("goal runs did not list finished work before live work: %+v unknown=%v error=%v", runs, unknown, err)
			}
			alive := true
			b.manager.Prober = unitStopProber{&alive}
			sleep := b.manager.Sleep
			b.manager.Sleep = func(d time.Duration) { sleep(d); alive = false }
			for range 2 {
				code, stopped, _ := pendingWork(t, b, "work", "stop", b.id)
				if code != 0 {
					t.Errorf("goal stop: %d %s", code, stopped.Summary)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("finished run changed: error=%v before=%s after=%s", err, before, after)
				}
				if _, err := os.Stat(filepath.Join(dir, "cancelled")); !os.IsNotExist(err) {
					t.Fatalf("finished run received cancellation marker: %v", err)
				}
				if refs := jsonText(resultData(t, stopped)["stopped"]); strings.Contains(refs, finished) || !strings.Contains(refs, live) {
					t.Fatalf("stop selected finished work or missed live work: %s", refs)
				}
			}
			record, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(live)
			if err != nil || record.State != "cancelled" {
				t.Fatalf("live run was not cancelled: state=%s error=%v", record.State, err)
			}
		})
	}
}

func TestWorkStopGoalAgentCancelsLiveRunAndJob(t *testing.T) {
	t.Parallel()
	b := newWorkBed(t)
	b.starter.hold = "build"
	argv := append([]string{"work", "build", b.id, "--work", "agent-stop", "--brief", b.brief("agent-stop.md", "Build the unit.\n"), "--lines", "5"}, workCheck...)
	code, built, _ := pendingWork(t, b, argv...)
	if code != 3 {
		t.Fatalf("live build: %d %s", code, built.Summary)
	}
	run := resultData(t, built)["run"].(string)
	unit, err := (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	operation := unit.Rounds[0].Steps[0].LaunchID
	job := "agent-stop-job"
	path := filepath.Join(b.root(), "artifacts", "agents", "jobs", job+".json")
	transferWriteJSON(t, path, map[string]any{"status": "running", "jobId": job, "goalId": b.id})
	b.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
		return humanauthority.Proof{}, errors.New("agent invocation")
	}
	alive := true
	b.manager.Prober = unitStopProber{&alive}
	sleep := b.manager.Sleep
	b.manager.Sleep = func(d time.Duration) { sleep(d); alive = false }
	code, stopped, _ := pendingWork(t, b, "work", "stop", b.id)
	if code != 0 || stopped.Outcome != intentConfirmed {
		t.Fatalf("agent goal stop: %d %s", code, stopped.Summary)
	}
	unit, err = (&launch.UnitRunner{Root: b.unitRoot}).Status(run)
	if err != nil || unit.State != "cancelled" {
		t.Fatalf("agent did not cancel the run: state=%s error=%v", unit.State, err)
	}
	execution, err := b.manager.Store.Read(operation)
	if err != nil || execution.State != launch.Cancelled {
		t.Fatalf("agent did not cancel the launch: state=%s error=%v", execution.State, err)
	}
	dispatch, err := dispatchcore.ReadRecordObject(path)
	if err != nil || dispatch["status"] != "cancelled" {
		t.Fatalf("agent did not cancel the dispatch job: %+v error=%v", dispatch, err)
	}
}

// TestWorkStopGoalStopsEveryRunningJobOfTheGoal: work stop G stops every
// running job of goal G and no other goal's (U9b, B2); a repeat with none
// running is success and stops nothing.
func TestWorkStopGoalStopsEveryRunningJobOfTheGoal(t *testing.T) {
	t.Parallel()
	b := newProcessBed(t)
	jobs := filepath.Join(b.root(), "artifacts", "agents", "jobs")
	if err := os.MkdirAll(jobs, 0o755); err != nil {
		t.Fatal(err)
	}
	for id, record := range map[string]string{
		"job-g1":    `{"status":"running","jobId":"job-g1","goalId":"` + bedGoal + `"}`,
		"job-g2":    `{"status":"running","jobId":"job-g2","goalId":"` + bedGoal + `"}`,
		"job-other": `{"status":"running","jobId":"job-other","goalId":"another-goal"}`,
		"job-ended": `{"status":"completed","jobId":"job-ended","goalId":"` + bedGoal + `"}`,
	} {
		if err := os.WriteFile(filepath.Join(jobs, id+".json"), []byte(record), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	owners := b.owners()
	unitRoot := t.TempDir()
	owners.work.units = func(stateroot.Layout) *launch.UnitRunner {
		return &launch.UnitRunner{Root: unitRoot}
	}
	var cancelled []string
	owners.processes.cancelDispatch = func(_, job string) (map[string]any, int, error) {
		cancelled = append(cancelled, job)
		if err := os.WriteFile(filepath.Join(jobs, job+".json"), []byte(`{"status":"cancelled","jobId":"`+job+`","goalId":"`+bedGoal+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		return map[string]any{"outcome": "CANCELLED", "jobId": job}, 0, nil
	}
	code, result := b.runJSON(owners, "work", "stop", bedGoal)
	slices.Sort(cancelled)
	if code != 0 || result.Outcome != intentConfirmed || !slices.Equal(cancelled, []string{"job-g1", "job-g2"}) {
		t.Fatalf("work stop G = %d %+v, cancelled %v", code, result, cancelled)
	}
	code, result = b.runJSON(owners, "work", "stop", bedGoal)
	if code != 0 || result.Outcome != intentUnchanged || len(cancelled) != 2 || !strings.Contains(result.Summary, "no job of goal "+bedGoal+" is running") {
		t.Fatalf("repeated work stop G = %d %+v, cancelled %v", code, result, cancelled)
	}
}
