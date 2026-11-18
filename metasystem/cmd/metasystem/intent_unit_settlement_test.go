package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

const classificationReviseImpact = "Impact: the red's comparison or rerun stops now; the failed check stays unclassified.\nWhat it already ran is still charged. A correction starts at once.\nStopping that correction later does not resume the comparison."

func TestAPersonsReviseEndsTheClassificationItSupersedes(t *testing.T) {
	t.Parallel()
	for _, form := range []string{"named", "run"} {
		t.Run(form, func(t *testing.T) {
			t.Parallel()
			reviseHeldClassification(t, form, false)
		})
	}
}

func TestAPersonsReviseSettlesAComparisonLeftPending(t *testing.T) {
	t.Parallel()
	reviseHeldClassification(t, "named", true)
}

func reviseHeldClassification(t *testing.T, form string, pending bool) {
	t.Helper()
	bed, args := comparisonAccountingBed(t, 240, func(file *goal.GoalFile) { file.Budget.ActiveJobLimit = 1 })
	bed.manager.Supervisor = callerComparisonStarter{unitProofStarter: unitProofStarter{bed: bed, t: t}, hold: true}
	prober := &settlementProber{Prober: bed.manager.Prober}
	bed.manager.Prober = prober
	code, held, _ := bed.work(args...)
	if code != 3 || !strings.Contains(resultWords(held), "comparing the red with the base") {
		t.Fatalf("comparison not held: %d %+v", code, held)
	}
	run := resultData(t, held)["run"].(string)
	runner := &launch.UnitRunner{Root: bed.unitRoot}
	before, err := runner.Status(run)
	if err != nil {
		t.Fatal(err)
	}
	base := before.Rounds[0].Steps[1].Comparison.LaunchID
	if pending {
		state, err := bed.manager.Store.StateDir(base)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.RemoveAll(state); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Remove(filepath.Join(bed.worktree, "red")); err != nil {
		t.Fatal(err)
	}
	brief, reason := bed.brief("correction.md", "Correct the unit.\n"), "start over; the base can wait"
	args = []string{"work", "revise", bed.id, "--work", "comparison", "--after", "1", "--brief", brief, "--reason", reason, "--by", "Wido"}
	if form == "run" {
		args = []string{"work", "revise", "run:" + run, "--brief", brief, "--reason", reason, "--by", "Wido"}
	}
	owners := bed.workOwners()
	owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	owners.commandNow = func(string) (time.Time, error) { return bed.manager.Now(), nil }
	overrides := filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides")
	assertImpact := func() {
		t.Helper()
		paths, _ := filepath.Glob(filepath.Join(overrides, "*.json"))
		if len(paths) != 1 {
			t.Fatalf("impact not kept before its effect: %v", paths)
		}
		data, err := os.ReadFile(paths[0])
		var record struct{ Kind, Impact, Who, Reason string }
		if err != nil || json.Unmarshal(data, &record) != nil || record.Kind != "work-revise" || record.Impact != classificationReviseImpact || record.Who != "Wido" || record.Reason != reason {
			t.Fatalf("wrong impact evidence: %s %v", data, err)
		}
	}
	var out bytes.Buffer
	printed, cancels := false, 0
	stderr := &unitImpactWriter{observe: func(text string) {
		if strings.Contains(text, "Impact:") {
			assertImpact()
			printed = true
		}
	}}
	sleep := bed.manager.Sleep
	bed.manager.Sleep = func(d time.Duration) {
		if !prober.dead {
			assertImpact()
			if !printed {
				t.Fatal("classification cancelled before impact was printed")
			}
			cancels++
			prober.dead = true
		}
		sleep(d)
	}
	command, rest, ok := resolveIntentArgv(args)
	if !ok {
		t.Fatal("revise command missing")
	}
	code = runIntentIn(command, append([]string{"--json"}, rest...), &out, stderr, bed.root(), owners)
	after, err := runner.Status(run)
	if (code != 0 && code != 3) || err != nil || len(after.Rounds) != 2 || len(after.Revisions) != 1 || after.Revisions[0].Person != "Wido" || after.Revisions[0].Impact != classificationReviseImpact || !strings.Contains(stderr.String(), classificationReviseImpact+" Reason: "+reason) {
		t.Fatalf("person's revision did not continue: %d %s %s %+v %v", code, out.String(), stderr.String(), after, err)
	}
	round := after.Rounds[0]
	if round.Outcome != "proof-red" || round.Cause != "unclassified" || round.Stop == nil || round.Steps[1].Reason != "the red is not classified: Wido's revise stopped its classification" || len(after.Rounds[1].Steps[0].LaunchIDs) == 0 {
		t.Fatalf("classification did not end with a correction: %+v", after)
	}
	for _, step := range round.Steps[2:] {
		if step.State != launch.StepSkipped {
			t.Fatalf("later step not skipped: %+v", step)
		}
	}
	reservation := filepath.Join(bed.root(), "artifacts", "agents", "jobs", base+".json")
	job, err := dispatchcore.ReadRecordObject(reservation)
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if err != nil || job["status"] != "cancelled" || job["unitExecuted"] != !pending || projection.Status != dispatchcore.BudgetKnown {
		t.Fatalf("comparison not settled: %v %+v %v", job, projection, err)
	}
	if !pending {
		execution, err := bed.manager.Store.Read(base)
		if err != nil || execution.State != launch.Cancelled || cancels != 1 {
			t.Fatalf("comparison not cancelled: %+v %v cancels=%d", execution, err, cancels)
		}
	}
	files := workIdemSnapshot(t, overrides)
	settled, _ := os.ReadFile(reservation)
	sleep(time.Minute)
	code, _, replay := bed.run(owners, args...)
	again, err := runner.Status(run)
	bytesAfter, _ := os.ReadFile(reservation)
	if (code != 0 && code != 3) || err != nil || len(again.Rounds) != 2 || len(again.Revisions) != 1 || strings.Contains(replay, "Impact:") || !bytes.Equal(settled, bytesAfter) || !pending && cancels != 1 {
		t.Fatalf("revision replay changed the request: %d %s %+v %v", code, replay, again, err)
	}
	workIdemSameFiles(t, "revision replay", files, workIdemSnapshot(t, overrides))
}

func TestOnlyAPersonsReviseEndsAClassification(t *testing.T) {
	t.Parallel()
	for _, classification := range []bool{true, false} {
		t.Run(map[bool]string{true: "machine-classification", false: "person-build"}[classification], func(t *testing.T) {
			t.Parallel()
			bed, args := comparisonAccountingBed(t, 240)
			if classification {
				bed.manager.Supervisor = callerComparisonStarter{unitProofStarter: unitProofStarter{bed: bed, t: t}, hold: true}
			} else {
				bed.starter.hold = "build"
			}
			_, held, _ := bed.work(args...)
			run := resultData(t, held)["run"].(string)
			args = []string{"work", "revise", bed.id, "--work", "comparison", "--after", "1", "--brief", bed.brief("next.md", "Correct the unit.\n")}
			owners := bed.workOwners()
			owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
			if !classification {
				args = append(args, "--reason", "start over", "--by", "Wido")
			}
			code, _, stderr := bed.run(owners, args...)
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
			if code != 1 || !strings.Contains(stderr, "wait for the running attempt to finish") || err != nil || len(record.Rounds) != 1 || len(record.Revisions) != 0 || record.State != "running" || projection.ActiveJobs != 1 {
				t.Fatalf("running work was superseded: %d %s %+v %+v %v", code, stderr, record, projection, err)
			}
			paths, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
			if classification && len(paths) != 0 || !classification && !strings.Contains(stderr, "Impact: this admits one correction despite the recorded read.") {
				t.Fatalf("wrong refusal impact: %s records=%v", stderr, paths)
			}
		})
	}
}

func TestAStoppedRunFreesTheSlotItsUnstartedJobHeld(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	conf := filepath.Join(bed.root(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("host.builds=person\n"), 0600); err != nil {
		t.Fatal(err)
	}
	owners := bed.workOwners()
	owners.processes.ask = func(string, channelAskInput) (channel.Question, []string, int, error) {
		return channel.Question{}, nil, 0, nil
	}
	owners.commandNow = func(string) (time.Time, error) { return bed.manager.Now(), nil }
	before := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	brief := bed.brief("build.md", "Build the unit.\n")
	code, held := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "--work", "A", "--brief", brief, "--lines", "5")
	if code != 1 || held.Outcome != intentRefused {
		t.Fatalf("the holder's build was not refused: %d %+v", code, held)
	}
	paths, _ := filepath.Glob(filepath.Join(bed.unitRoot, "*", "run.json"))
	if len(paths) != 1 {
		t.Fatalf("refused build did not retain one run: %v", paths)
	}
	runner := &launch.UnitRunner{Root: bed.unitRoot}
	run, err := runner.Status(filepath.Base(filepath.Dir(paths[0])))
	if err != nil {
		t.Fatal(err)
	}
	step := run.Rounds[0].Steps[0]
	if step.State != launch.StepStarting || step.LaunchID == "" || slices.Contains(step.LaunchIDs, step.LaunchID) {
		t.Fatalf("refused build lost its starting id: %+v", step)
	}
	if _, err := bed.manager.Store.Read(step.LaunchID); !os.IsNotExist(err) {
		t.Fatalf("refused build created a launch: %v", err)
	}
	reservation := filepath.Join(bed.root(), "artifacts", "agents", "jobs", step.LaunchID+".json")
	job, err := dispatchcore.ReadRecordObject(reservation)
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if err != nil || job["status"] != "pending" || projection.ActiveJobs != 1 {
		t.Fatalf("refused build did not retain its slot: %v %+v %v", job, projection, err)
	}
	bed.lineage = ""
	owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	target, at := "run:"+run.ID, bed.manager.Now()
	code, _, stderr := bed.run(owners, "work", "stop", target, "--reason", "finding 518")
	if code != 0 || !strings.Contains(stderr, "Impact: "+target+" stops and is not restarted. Reason: finding 518") {
		t.Fatalf("person's stop failed or lost its impact: %d %s", code, stderr)
	}
	job, err = dispatchcore.ReadRecordObject(reservation)
	projection = dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	if err != nil || job["status"] != "cancelled" || job["unitExecuted"] != false || projection.Status != dispatchcore.BudgetKnown || projection.ActiveJobs != 0 || projection.ReservedJobMinutes != before.ReservedJobMinutes || projection.ObservedJobMinutes != before.ObservedJobMinutes {
		t.Fatalf("stop did not settle the unstarted reservation uncharged: %v %+v %v", job, projection, err)
	}
	impacts, _ := filepath.Glob(filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides", "*.json"))
	if len(impacts) != 1 {
		t.Fatalf("stop kept %d override records", len(impacts))
	}
	data, err := os.ReadFile(impacts[0])
	var impact struct {
		Goal, Kind, Reason, Who, Impact string
		At                              time.Time
	}
	if err != nil || json.Unmarshal(data, &impact) != nil || impact.Goal != bed.id || impact.Kind != "work-stop" || impact.Reason != "finding 518" || impact.Who != "Wido" || impact.Impact != "Impact: "+target+" stops and is not restarted." || !impact.At.Equal(at) {
		t.Fatalf("stop decision lost its evidence: %s %v", data, err)
	}
	if err := os.WriteFile(conf, []byte("host.builds=auto\n"), 0600); err != nil {
		t.Fatal(err)
	}
	code, admitted := fleetWorkJSON(t, bed, owners, "work", "build", bed.id, "--work", "B", "--brief", brief, "--lines", "5")
	if code != 0 || len(bed.starter.launched()) == 0 || bed.starter.launched()[0] != "build" {
		t.Fatalf("the next build was not admitted: %d %+v", code, admitted)
	}
	files := workIdemSnapshot(t, bed.root())
	bed.manager.Sleep(time.Minute)
	if code, stopped := bed.runJSON(owners, "work", "stop", target, "--reason", "finding 518"); code != 0 {
		t.Fatalf("repeated stop: %d %+v", code, stopped)
	}
	workIdemSameFiles(t, "repeated run stop", files, workIdemSnapshot(t, bed.root()))
}

type settlementProber struct {
	identity.Prober
	dead bool
}

func (p *settlementProber) Probe(pid int64) (identity.Exact, identity.Liveness, error) {
	if p.dead && (pid == 10 || pid == 20) {
		return identity.Exact{}, identity.Dead, nil
	}
	return p.Prober.Probe(pid)
}

func TestAStoppedRunEndsItsComparison(t *testing.T) {
	t.Parallel()
	bed, args := comparisonAccountingBed(t, 240)
	bed.manager.Supervisor = callerComparisonStarter{unitProofStarter: unitProofStarter{bed: bed, t: t}, hold: true}
	prober := &settlementProber{Prober: bed.manager.Prober}
	bed.manager.Prober = prober
	code, held, _ := bed.work(args...)
	if code != 3 || !strings.Contains(resultWords(held), "comparing the red with the base") {
		t.Fatalf("base comparison was not held: %d %+v", code, held)
	}
	run := resultData(t, held)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	base := record.Rounds[0].Steps[1].Comparison.LaunchID
	sleep := bed.manager.Sleep
	bed.manager.Sleep = func(d time.Duration) { prober.dead = true; sleep(d) }
	owners := bed.workOwners()
	owners.prove = enrolledPersonProver(t, bed.root(), bed.manager.Now())
	if code, stopped := bed.runJSON(owners, "work", "stop", "run:"+run); code != 0 {
		t.Fatalf("run stop failed: %d %+v", code, stopped)
	}
	execution, err := bed.manager.Store.Read(base)
	if err != nil || execution.State != launch.Cancelled || !prober.dead {
		t.Fatalf("comparison did not end: %+v %v", execution, err)
	}
	job, err := dispatchcore.ReadRecordObject(filepath.Join(bed.root(), "artifacts", "agents", "jobs", base+".json"))
	projection := dispatchcore.ProjectBudget(bed.root(), bed.goalFile(bed.id), bed.manager.Now())
	record, readErr := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil || readErr != nil || job["status"] != "cancelled" || projection.Status != dispatchcore.BudgetKnown || projection.ActiveJobs != 0 || record.State != "cancelled" {
		t.Fatalf("comparison reservation was not settled: %v %+v %s %v %v", job, projection, record.State, err, readErr)
	}
}

func TestAReasonThatCannotBeKeptDoesNotStopALaunch(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	if err := bed.manager.Store.Create(launch.Record{ID: "standalone", Goal: bed.id, Kind: "read", State: launch.Starting}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(bed.root(), "artifacts", "agents", "channel", "unit-stop-overrides")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0600); err != nil {
		t.Fatal(err)
	}
	owners := bed.workOwners()
	owners.processes.launches = func() *launch.Manager { return bed.manager }
	code, result := bed.runJSON(owners, "work", "stop", "j1:standalone", "--reason", "stop this read")
	record, err := bed.manager.Store.Read("standalone")
	if code != 1 || result.Summary != "the stop's reason could not be kept; nothing was stopped" || len(result.Details) == 0 || result.Next == nil || err != nil || record.State != launch.Starting || record.Reason != "" {
		t.Fatalf("an unrecorded reason reached cancellation: %d %+v %+v %v", code, result, record, err)
	}
}
