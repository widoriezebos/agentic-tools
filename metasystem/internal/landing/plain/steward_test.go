package plain

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/enginebuild"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"golang.org/x/sys/unix"
)

type stewardBed struct {
	*statusBed
	seams    ProveSeams
	calls    []string
	question channel.Question
}

func newStewardBed(t *testing.T) *stewardBed {
	t.Helper()
	b := &stewardBed{statusBed: newStatusBed(t), seams: healthSeams(bedNow)}
	b.record.CustodyEpoch = 1
	if err := os.MkdirAll(b.install, 0755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(b.record)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(filepath.Dir(lane.RecordPath(b.home)), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(lane.RecordPath(b.home), data, 0600); err != nil {
		t.Fatal(err)
	}
	b.seams.Steward = &StewardEffects{
		UsageStore: &launch.Store{Root: t.TempDir()},
		Owner:      func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil },
		Withdraw: func(install string, q channel.Question, seams ProveSeams) (bool, error) {
			b.calls = append(b.calls, "withdraw")
			return WithdrawStaleLaneQuestion(install, q, seams)
		},
		Trunk: func(install, checkout string, seams ProveSeams) (string, error) {
			if install != b.install || checkout != b.checkout || seams.Person != nil {
				t.Fatalf("foreign proof: %s %s %+v", install, checkout, seams.Person)
			}
			b.calls = append(b.calls, "trunk")
			return "proving main", nil
		},
		Settle: func(lane.Record) (string, error) { b.calls = append(b.calls, "settle"); return "ended", nil },
		Wake:   func(lane.Record) (string, error) { b.calls = append(b.calls, "wake"); return "keeper woken", nil },
	}
	return b
}

func (b *stewardBed) staleQuestion() {
	b.t.Helper()
	q, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Facts: []string{"The checkout is not on main."}, Now: bedNow.Add(-time.Hour)})
	if err != nil {
		b.t.Fatal(err)
	}
	b.question = q
}

func (b *stewardBed) incidents() {
	b.seams.Incidents = func(string, string, string) ([]goal.TrunkRedEntry, error) {
		return []goal.TrunkRedEntry{{ID: "incident", Class: goal.TrunkRedClassTrunkRed, Sightings: []goal.TrunkRedSighting{{BaseCommit: "old-main"}}}}, nil
	}
}

func (b *stewardBed) step(now time.Time) StewardRun {
	b.t.Helper()
	run, err := StewardStep(b.home, now, b.seams)
	if err != nil {
		b.t.Fatal(err)
	}
	return run
}

func TestLandingStewardStaleQuestionBoundAndHealth(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.staleQuestion()
	run := b.step(bedNow)
	if !reflect.DeepEqual(b.calls, []string{"withdraw", "trunk"}) || len(run.Interventions) != 1 || run.Interventions[0].Row != "stale-question" {
		t.Fatalf("%+v calls=%v", run, b.calls)
	}
	closed, err := channel.ReadQuestion(b.install, b.question.ID)
	if err != nil || closed.State != "closed" {
		t.Fatalf("question=%+v %v", closed, err)
	}
	// Restore the same situation, without changing the bound's subject.
	closed.State, closed.ClosedBecause = "open", ""
	data, err := json.Marshal(closed)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(b.install, "artifacts", "agents", "channel", "questions", closed.ID+".json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	b.calls = nil
	if bounded := b.step(bedNow.Add(59 * time.Minute)); len(b.calls) != 0 || !strings.Contains(bounded.Line, "bounded") {
		t.Fatalf("%+v calls=%v", bounded, b.calls)
	}
	if retry := b.step(bedNow.Add(61 * time.Minute)); len(retry.Interventions) != 1 || len(b.calls) != 2 {
		t.Fatalf("%+v calls=%v", retry, b.calls)
	}
	status := ReadStatus(b.home, b.record, b.view, b.seams)
	if status.Health == nil || len(status.Health.Interventions) != 2 || !strings.Contains(status.Health.Line(), "; steward:") || !strings.Contains(status.Health.Text(), "interventions:") {
		t.Fatalf("health=%+v problems=%v", status.Health, status.Problems)
	}
}

func TestLandingStewardIncidentsAndContention(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"incidents", "contention", "running", "person", "paused", "young-step"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.incidents()
			if scenario == "contention" {
				b.lines("results.jsonl", Result{Attempt: "red", Commit: "old-main", Result: Red, LoadStart: &hostload.Sample{Available: true, Cores: 2, Load1m: 10}})
				b.seams.Incidents = healthSeams(bedNow).Incidents
			}
			if scenario == "running" || scenario == "person" {
				running := Running{Attempt: "proof", Commit: "main", Tree: "tree", Since: bedNow.Add(-time.Hour).Format(time.RFC3339), Pid: 42}
				if scenario == "person" {
					running.Person = &ActProvenance{Person: "a person"}
				}
				b.lines("running.json", running)
				b.seams.Alive = func(Running) bool { return true }
			}
			if scenario == "paused" {
				if _, err := lane.SetPause(b.home, "a person", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "young-step" {
				b.lines("agent-step.json", AgentStep{Verb: "work review", StartedAt: bedNow.Add(-time.Minute).Format(time.RFC3339), PID: 42})
				b.seams.Alive = func(Running) bool { return false }
			}
			run := b.step(bedNow)
			want := scenario == "incidents" || scenario == "contention"
			if (len(b.calls) == 1) != want || want && b.calls[0] != "trunk" || !want && len(run.Interventions) != 0 {
				t.Fatalf("%s: %+v calls=%v", scenario, run, b.calls)
			}
		})
	}
}

func TestLandingStewardSilentAgentAndPersonBounds(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"silent", "young-step", "person-proof", "person-stop"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.staleQuestion()
			b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) {
				return lane.OwnerProbe{Alive: true, Since: bedNow.Add(-55 * time.Minute)}, nil
			}
			if scenario == "young-step" {
				b.lines("agent-step.json", AgentStep{Verb: "work wait", StartedAt: bedNow.Add(-time.Minute).Format(time.RFC3339), PID: 42})
				b.seams.Alive = func(Running) bool { return true }
			}
			if scenario == "person-proof" {
				b.lines("running.json", Running{Attempt: "person", Person: &ActProvenance{Person: "a person"}, Since: bedNow.Format(time.RFC3339), Pid: 42})
				b.seams.Alive = func(Running) bool { return true }
			}
			if scenario == "person-stop" {
				if _, err := lane.SetPause(b.home, "a person", bedNow); err != nil {
					t.Fatal(err)
				}
			}
			// The silent session is observed without another actionable row.
			if scenario == "silent" {
				if _, err := channel.Withdraw(b.install, b.question.ID, "fixture closed", nil, channel.DestinationConfig{}); err != nil {
					t.Fatal(err)
				}
			}
			run := b.step(bedNow)
			if scenario == "silent" {
				if !reflect.DeepEqual(b.calls, []string{"settle", "wake"}) || len(run.Interventions) != 1 {
					t.Fatalf("%+v calls=%v", run, b.calls)
				}
				bounded := b.step(bedNow.Add(time.Minute))
				if len(b.calls) != 2 || !strings.Contains(bounded.Line, "bounded") {
					t.Fatalf("silent session restarted twice: %+v %v", bounded, b.calls)
				}
				stops, err := OpenStops(b.install)
				if err != nil || len(stops) != 0 {
					t.Fatalf("invented person stop: %+v %v", stops, err)
				}
			} else if len(b.calls) != 0 {
				t.Fatalf("person or agent interrupted: %+v calls=%v", run, b.calls)
			}
		})
	}
}

func TestLandingStewardReportOnlyAndLastFive(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.lines("queue.jsonl", Line{Goal: "late", SHA: "sha", At: bedNow.Add(-2 * time.Hour).Format(time.RFC3339)})
	b.seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "merge-base" {
			return "", exec.Command("false").Run()
		}
		return "main", nil
	}
	if _, err := channel.Ask(channel.AskRequest{RepoRoot: b.install, About: "lane", Machine: "landing", Lineage: lane.AgentLineage, Kind: "other", Facts: []string{"Unknown premise"}, Now: bedNow.Add(-time.Hour)}); err != nil {
		t.Fatal(err)
	}
	run := b.step(bedNow)
	if len(b.calls) != 0 || !strings.Contains(run.Line, "report only") || !strings.Contains(run.Line, "overdue-goal") || !strings.Contains(run.Line, "question-open") {
		t.Fatalf("%+v calls=%v", run, b.calls)
	}
	if _, err := lane.SetPause(b.home, "a person", bedNow.Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if paused := b.step(bedNow); !strings.Contains(paused.Line, "paused") || len(b.calls) != 0 {
		t.Fatalf("%+v %v", paused, b.calls)
	}
	for i := 0; i < 7; i++ {
		if err := withLock(b.install, func() error {
			return appendLine(interventionsPath(b.install), Intervention{At: bedNow.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339), What: "act"})
		}); err != nil {
			t.Fatal(err)
		}
	}
	status := ReadStatus(b.home, b.record, b.view, b.seams)
	if status.Health == nil || len(status.Health.Interventions) != 5 || !strings.Contains(status.Health.Line(), "act 6 min ago") {
		t.Fatalf("health=%+v problems=%v", status.Health, status.Problems)
	}
}

func TestLandingStewardConcurrentTicksAndFailureRecorded(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.incidents()
	b.seams.Steward.Trunk = func(string, string, ProveSeams) (string, error) {
		b.calls = append(b.calls, "trunk")
		return "", errors.New("launch failed")
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Go(func() { _, _ = StewardStep(b.home, bedNow, b.seams) })
	}
	wg.Wait()
	history, err := readLines[Intervention](interventionsPath(b.install))
	if err != nil || len(history) != 1 || len(b.calls) != 1 || history[0].Outcome != "failed: launch failed" {
		t.Fatalf("history=%+v calls=%v err=%v", history, b.calls, err)
	}
}

func TestLandingStewardStaleEngineReportsWithoutActing(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	if err := os.MkdirAll(filepath.Join(b.install, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	command := testenv.Go("build", "-buildvcs=false", "-ldflags", enginebuild.StampLinkerFlags("old-main"), "-o", filepath.Join(b.install, "bin", "metasystem"), "../../enginebuild/testdata/stampprobe")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("stamp build: %v %s", err, out)
	}
	b.seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "diff" {
			return "", exec.Command("false").Run()
		}
		return "main", nil
	}
	for _, now := range []time.Time{bedNow, bedNow.Add(time.Minute), bedNow.Add(2 * time.Hour)} {
		run := b.step(now)
		if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, "stale-engine") || !strings.Contains(run.Line, "report only") {
			t.Fatalf("stale engine acted: %+v calls=%v", run, b.calls)
		}
	}
	status := ReadStatus(b.home, b.record, b.view, b.seams)
	row := healthRow(status.Health, "stale-engine")
	if row == nil || row.Act != "redeploy the engine from main (deploy now); the steward acts on it once landing-deploys-the-engine rebuilds the install's engine" {
		t.Fatalf("stale engine prescription: %+v", row)
	}
}

func TestLandingStewardContentionBoundUsesCommit(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	red := Result{Attempt: "red-1", Commit: "old-main", Tree: "old-tree", Result: Red, LoadStart: &hostload.Sample{Available: true, Cores: 2, Load1m: 10}}
	b.lines("results.jsonl", red)
	run := b.step(bedNow)
	if len(b.calls) != 1 || len(run.Interventions) != 1 || run.Interventions[0].Row != "suspected-contention" {
		t.Fatalf("first red: %+v calls=%v", run, b.calls)
	}
	red.Attempt = "red-2"
	b.lines("results.jsonl", red)
	bounded := b.step(bedNow.Add(time.Minute))
	if len(b.calls) != 1 || len(bounded.Interventions) != 0 || !strings.Contains(bounded.Line, "bounded: already acted within 60 minutes") {
		t.Fatalf("same main re-proved: %+v calls=%v", bounded, b.calls)
	}
	red.Attempt, red.Commit, red.Tree = "red-3", "moved-main", "moved-tree"
	b.lines("results.jsonl", red)
	moved := b.step(bedNow.Add(2 * time.Minute))
	if len(b.calls) != 2 || len(moved.Interventions) != 1 || moved.Interventions[0].Subject != red.Commit {
		t.Fatalf("moved main not proved: %+v calls=%v", moved, b.calls)
	}
}

func TestLandingStewardContentionSkipsProvenCurrentMain(t *testing.T) {
	t.Parallel()
	for _, verdict := range []string{Red, Green} {
		t.Run(verdict, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			b.lines("results.jsonl", Result{Attempt: "trunk", Commit: "main", Trunk: true, Result: verdict},
				Result{Attempt: "red", Commit: "main", Result: Red, LoadStart: &hostload.Sample{Available: true, Cores: 2, Load1m: 10}})
			run := b.step(bedNow)
			if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, "suspected-contention") || !strings.Contains(run.Line, "bounded: current main already proved") {
				t.Fatalf("unchanged main re-proved: %+v calls=%v", run, b.calls)
			}
			b.seams.Git = func(string, ...string) (string, error) { return "new-main", nil }
			moved := b.step(bedNow.Add(time.Minute))
			if len(b.calls) != 1 || len(moved.Interventions) != 1 {
				t.Fatalf("moved main not proved: %+v calls=%v", moved, b.calls)
			}
		})
	}
}

func TestLandingStewardOpenLoopStopBoundsActs(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.staleQuestion()
	b.incidents()
	b.seams.Steward.Owner = func(string) (lane.OwnerProbe, error) {
		return lane.OwnerProbe{Alive: true, Since: bedNow.Add(-55 * time.Minute)}, nil
	}
	stop := Stop{Loop: "lane-proof", Subject: "main", Decision: "stop", Evidence: "a person must decide", At: bedNow.Add(-time.Minute).Format(time.RFC3339)}
	b.lines("stops.jsonl", stop)
	run := b.step(bedNow)
	if len(b.calls) != 0 || len(run.Interventions) != 0 || strings.Count(run.Line, "bounded: a person's stop is open") != 3 {
		t.Fatalf("acted during open stop: %+v calls=%v", run, b.calls)
	}
	stop.Decision = "close"
	b.lines("stops.jsonl", stop)
	closed := b.step(bedNow.Add(time.Minute))
	if len(b.calls) == 0 || len(closed.Interventions) == 0 {
		t.Fatalf("closed stop still bounded: %+v calls=%v", closed, b.calls)
	}
}

func TestLandingStewardTrunkUsesVerbAdmission(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.incidents()
	b.seams.Steward.Trunk = nil
	b.seams.Executable = func() (string, error) { return "/fixture/engine", nil }
	b.seams.NewID = func() string { return "steward-proof" }
	b.seams.Lane = func() (lane.Record, error) { return b.record, nil }
	b.seams.CommandForCommit = func(commit string) (string, error) {
		if commit != "main" {
			t.Fatal(commit)
		}
		return "the committed proof command", nil
	}
	fetched, launched := 0, 0
	b.seams.Git = func(_ string, args ...string) (string, error) {
		if args[0] == "fetch" {
			fetched++
		}
		return "main", nil
	}
	b.seams.Launch = func(argv []string, dir, log string) (int64, error) {
		launched++
		if strings.Join(argv, " ") != "/fixture/engine landing prove --wait --attempt steward-proof --trunk" || dir != b.checkout || !strings.HasPrefix(log, Dir(b.install)) {
			t.Fatalf("foreign admission: %v %s %s", argv, dir, log)
		}
		return int64(os.Getpid()), nil
	}
	b.step(bedNow)
	running, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || !running.Trunk || running.Person != nil || running.Admission == nil || running.Admission.Command != "the committed proof command" || fetched != 1 || launched != 1 {
		t.Fatalf("running=%+v err=%v fetches=%d launches=%d", running, err, fetched, launched)
	}
}

func TestLandingStewardRemainingReportOnlyRows(t *testing.T) {
	t.Parallel()
	for _, row := range []string{"batch-of-one", "proof-drift", "abandoned-resolution", "confirmed-flake"} {
		t.Run(row, func(t *testing.T) {
			t.Parallel()
			b := newStewardBed(t)
			exitOne := exec.Command("false").Run()
			missingMerge := exec.Command("sh", "-c", "exit 128").Run()
			b.seams.Git = func(_ string, args ...string) (string, error) {
				if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
					return "", missingMerge
				}
				if args[0] == "merge-base" {
					return "", exitOne
				}
				return "main", nil
			}
			switch row {
			case "batch-of-one":
				b.lines("queue.jsonl", Line{Goal: "waiting", SHA: "sha", At: bedNow.Add(-10 * time.Minute).Format(time.RFC3339)})
				if err := writeBatch(b.install, &Batch{ID: "singleton", Lane: b.record, Base: "main", State: BatchRunning, Selector: PolicyValue{Value: "auto"}, Members: []GoalSHA{{Goal: "member", SHA: "member-tip"}}, SealedAt: bedNow.Add(-5 * time.Minute).Format(time.RFC3339)}); err != nil {
					t.Fatal(err)
				}
			case "proof-drift":
				minutes := 40.
				b.lines("results.jsonl", Result{Attempt: "prior", Scope: "full", Result: Green, Minutes: &minutes})
				b.lines("running.json", Running{Attempt: "slow", Since: bedNow.Add(-70 * time.Minute).Format(time.RFC3339), Pid: 42})
				b.seams.Alive = func(Running) bool { return true }
			case "abandoned-resolution":
				if err := WriteFix(b.install, &Fix{Attempt: "merge", Commit: "main", Tip: "old", State: "resolving", Goal: "g", Units: []string{"unit"}, Paths: []conflict.Path{{Path: "x"}}}); err != nil {
					t.Fatal(err)
				}
			case "confirmed-flake":
				b.lines("results.jsonl", Result{Attempt: "flaky", Result: Red, RepeatComplete: true, FlakeRepeats: []Running{{Attempt: "repeat"}}})
			}
			run := b.step(bedNow)
			if len(b.calls) != 0 || len(run.Interventions) != 0 || !strings.Contains(run.Line, row) || !strings.Contains(run.Line, "report only") {
				t.Fatalf("%s: %+v calls=%v", row, run, b.calls)
			}
		})
	}
}

func TestLandingStewardAppendHoldsInstallLock(t *testing.T) {
	t.Parallel()
	b := newStewardBed(t)
	b.incidents()
	ready := make(chan error, 1)
	b.seams.Steward.Trunk = func(string, string, ProveSeams) (string, error) {
		err := unix.Mkfifo(interventionsPath(b.install), 0600)
		ready <- err
		return "proving main " + strings.Repeat("x", 1024*1024), err
	}
	done := make(chan error, 1)
	go func() { _, err := StewardStep(b.home, bedNow, b.seams); done <- err }()
	if err := <-ready; err != nil {
		t.Fatal(err)
	}
	// A record larger than the pipe buffer keeps its append blocked until read.
	var pipe *os.File
	finished := false
	t.Cleanup(func() {
		if pipe == nil {
			pipe, _ = os.OpenFile(interventionsPath(b.install), os.O_RDONLY, 0600)
		}
		if pipe != nil {
			if !finished {
				<-done
			}
			_ = pipe.Close()
		}
	})
	// The append holds the installation lock while it blocks on the pipe. The lock is
	// polled a bounded number of times with no wall clock (R-104); the test's own
	// deadline bounds a hang.
	locked := false
	for i := 0; i < 2_000_000 && !locked; i++ {
		held, err := lock.File(lockPath(b.install), 0600, lock.TryExclusive)
		if lock.Busy(err) {
			locked = true
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if err = held.Release(); err != nil {
			t.Fatal(err)
		}
		runtime.Gosched()
	}
	if !locked {
		t.Fatal("the intervention append did not hold the installation lock")
	}
	var err error
	pipe, err = os.OpenFile(interventionsPath(b.install), os.O_RDONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	first := make([]byte, 1)
	if _, err := io.ReadFull(pipe, first); err != nil {
		t.Fatal(err)
	}
	held, err := lock.File(lockPath(b.install), 0600, lock.TryExclusive)
	if !lock.Busy(err) {
		if held != nil {
			_ = held.Release()
		}
		t.Fatalf("the pending intervention append did not hold the installation lock: %v", err)
	}
	line, err := bufio.NewReader(pipe).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var intervention Intervention
	if err = json.Unmarshal(append(first, line...), &intervention); err != nil {
		t.Fatal(err)
	}
	if intervention.Row != "unproven-incidents" || !strings.HasPrefix(intervention.Outcome, "proving main ") {
		t.Fatal(intervention)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	finished = true
}
