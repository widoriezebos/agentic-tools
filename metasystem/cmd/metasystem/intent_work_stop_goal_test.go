package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
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
	if out := gcliForgivingMust(t, bed, "work", "stop", "ship-widget"); !strings.Contains(out, "nothing was stopped") || bed.tip() != tip {
		t.Fatalf("a repeated work stop G was not a no-op: %q", out)
	}

	gcliForgivingMust(t, bed, "goal", "budget", "ship-widget", "keep", "--by", "Wido", gcliForgivingFixture)
	if resumed := bed.goalRecord("ship-widget"); goalCLILine(resumed, "- StopFence:") != "" {
		t.Fatalf("the goal's fence did not lift once its stop completed:\n%s", resumed)
	}

	code, stdout, stderr = gcliForgivingPublic(bed, "work", "stop", "ship-widget")
	if code != 0 || !strings.Contains(stdout+stderr, "no job of goal ship-widget is running; nothing was stopped") {
		t.Fatalf("work stop G with nothing running is not a quiet success: code=%d stdout=%q stderr=%q", code, stdout, stderr)
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
