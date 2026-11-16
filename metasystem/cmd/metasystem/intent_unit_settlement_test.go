package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

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
