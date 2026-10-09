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

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel/phase"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

func TestDriverPendingWorkPublicBuildStatus(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"load", "build cap", "person"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			personProof := bed.personProof
			bed.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			conf := filepath.Join(bed.root(), "metasystem.conf")
			settings, err := os.ReadFile(conf)
			if err != nil {
				t.Fatal(err)
			}
			cap := "1"
			if reason == "person" {
				cap = "person"
			}
			if err := os.WriteFile(conf, append(settings, []byte("\nhost.builds="+cap+"\nhost.load-max=8\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			waiting := true
			bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				load := 0.0
				if waiting && reason == "load" {
					load = 9
				}
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: load}
			}
			if reason == "build cap" {
				if err := bed.manager.Store.Create(launch.Record{ID: "other-build", Goal: "other", Kind: "build", State: launch.Starting}); err != nil {
					t.Fatal(err)
				}
			}

			brief := bed.brief("pending.md", "Build this unit.\n")
			argv := append([]string{"work", "build", bed.id, "--work", "pending", "--brief", brief, "--lines", "5"}, workCheck...)
			code, first, _ := pendingWork(t, bed, argv...)
			if code != 1 || first.Outcome != intentRefused {
				t.Fatalf("initial hold: code=%d outcome=%s summary=%s", code, first.Outcome, first.Summary)
			}
			run := resultData(t, first)["run"].(string)
			runner := &launch.UnitRunner{Root: bed.unitRoot}
			read := func() launch.UnitRunRecord {
				t.Helper()
				record, err := runner.Status(run)
				if err != nil {
					t.Fatal(err)
				}
				return record
			}
			record := read()
			act := launch.PendingWork(record)
			if act == nil || act.Request == nil || act.Request.Actor == "person" || act.Request.ClaimSubject == "" || act.Request.InputDigest == "" || act.Operation != record.Rounds[0].Steps[0].LaunchID || act.Worktree != bed.worktree || act.Base != bed.head || act.Goal != bed.id || act.Unit != "pending" || act.Registration.Install == "" {
				t.Fatalf("capacity adapter lost the operation: %+v", act)
			}
			command := append([]string{"metasystem", "work", "build", "--json"}, argv[2:]...)
			if !slices.Equal(act.Request.Argv, command) || act.Request.CallerDirectory != bed.root() {
				t.Fatalf("retained argv=%v want %v", act.Request.Argv, command)
			}
			operation, start := act.Operation, act.Waits[0].StartedAt
			bed.manager.Sleep(90 * time.Second)
			for _, statusArgv := range [][]string{{"work", "status", bed.id}, {"work", "status", bed.id, "--work", "pending"}, {"work", "status", "run:" + run}} {
				code, status, _ := pendingWork(t, bed, statusArgv...)
				data, _ := json.Marshal(status.Data)
				if code != 0 || !strings.Contains(string(data), `"pendingAct"`) || !strings.Contains(string(data), `"queueDurationMs":90000`) || !strings.Contains(string(data), operation) {
					t.Fatalf("public status hid waiting work: %d %+v", code, status)
				}
				if status.Next == nil || !slices.Equal(status.Next.Argv, command) || !strings.Contains(status.Next.Reason, bed.root()) {
					t.Fatalf("status remedy changed: %+v", status.Next)
				}
				if reason == "person" && (!strings.Contains(status.Next.Reason, humanauthority.PersonActRemedy(shellCommand(command))) || strings.Contains(status.Next.Reason, "when capacity is available")) {
					t.Fatalf("person wait offered an agent capacity retry: %+v", status.Next)
				}
			}
			code, repeated, _ := pendingWork(t, bed, argv...)
			if code != 1 || resultData(t, repeated)["run"] != run {
				t.Fatalf("repeated hold: %d %+v", code, repeated)
			}
			record = read()
			step := record.Rounds[0].Steps[0]
			if record.State != "running" || len(record.Rounds) != 1 || record.Rounds[0].Stop != nil || record.Rounds[0].UnknownRetries != 0 || step.Cause != "" || step.FinishedAt != "" || step.ExecutionStartedAt != "" || step.ExecutionEndedAt != "" || len(step.LaunchIDs) != 1 || len(step.PendingAct.Waits) != 1 || step.PendingAct.Waits[0].StartedAt != start || len(bed.starter.launched()) != 0 {
				t.Fatalf("wait became a retry or execution: %+v", record)
			}
			if _, err := bed.manager.Store.Read(operation); !os.IsNotExist(err) {
				t.Fatalf("refusal wrote a launch: %v", err)
			}
			waiting = false
			if reason == "build cap" {
				if _, err := bed.manager.Store.Update("other-build", func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if reason == "person" {
				bed.personProof = personProof
			}
			code, built, _ := pendingWork(t, bed, argv...)
			if code != 0 || resultData(t, built)["run"] != run {
				t.Fatalf("same command did not resume: code=%d outcome=%s summary=%s", code, built.Outcome, built.Summary)
			}
			record = read()
			step = record.Rounds[0].Steps[0]
			if len(record.Rounds) != 1 || len(step.LaunchIDs) != 1 || step.LaunchID != operation || step.ExecutionStartedAt != step.PendingAct.Waits[0].EndedAt || step.PendingAct.Waits[0].EndReason != "admitted" || step.CollectedAt == "" || strings.Count(strings.Join(bed.starter.launched(), ","), "build") != 1 {
				t.Fatalf("admission changed identity or timing: %+v launches=%v", step, bed.starter.launched())
			}
			if duration, known := step.PendingAct.QueueDuration(bed.manager.Now()); !known || duration != 90*time.Second {
				t.Fatalf("wait duration=%s known=%t", duration, known)
			}
			children := len(bed.starter.launched())
			code, _, _ = pendingWork(t, bed, argv...)
			if code != 0 || len(read().Rounds) != 1 || len(bed.starter.launched()) != children {
				t.Fatal("re-entry created a correction")
			}
			step.PendingAct = nil
			legacy := record
			legacy.Rounds[0].Steps[0] = step
			body, err := json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(bed.unitRoot, run, "run.json"), body, 0600); err != nil {
				t.Fatal(err)
			}
			code, status, _ := pendingWork(t, bed, "work", "status", "run:"+run)
			if data := resultData(t, status); code != 0 || data["queueTiming"] != "unavailable" || data["queueDurationMs"] != nil {
				t.Fatalf("legacy public status invented timing: %d %+v", code, data)
			}
			body, _ = json.Marshal(status.Data)
			if strings.Contains(string(body), `"pendingAct"`) {
				t.Fatalf("legacy fixture did not remove the evidence: %s", body)
			}
		})
	}
}

func TestDriverProviderStateDoesNotGatePublicBuild(t *testing.T) {
	t.Parallel()
	for _, state := range []string{"standing", "unreadable"} {
		t.Run(state, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			bed.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			if _, err := outage.Observe(bed.manager.CapacityHome, bed.manager.Settings.BuildRuntime, "fixture", "overloaded", "synthetic overload", "observation", bed.manager.Now()); err != nil {
				t.Fatal(err)
			}
			if state == "unreadable" {
				path := testprovider.Path(filepath.Dir(bed.manager.CapacityHome))
				if err := os.WriteFile(path, []byte("{broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			brief := bed.brief("provider.md", "Build this unit.\n")
			argv := append([]string{"work", "build", bed.id, "--work", "provider", "--brief", brief, "--lines", "5"}, workCheck...)
			code, built, _ := pendingWork(t, bed, argv...)
			if code != 0 || resultData(t, built)["state"] != "awaiting-judgement" || strings.Count(strings.Join(bed.starter.launched(), ","), "build") != 1 {
				t.Fatalf("provider state gated the build: code=%d outcome=%s summary=%s launches=%v", code, built.Outcome, built.Summary, bed.starter.launched())
			}
			run := resultData(t, built)["run"].(string)
			record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
			if err != nil || launch.PendingWork(record) != nil || record.Rounds[0].Steps[0].PendingAct != nil {
				t.Fatalf("provider state retained a capacity wait: error=%v record=%+v", err, record)
			}
		})
	}
}

func TestDriverPendingRunBuildRemedy(t *testing.T) {
	t.Parallel()
	for _, reason := range []string{"load", "person"} {
		t.Run(reason, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			personProof := bed.personProof
			bed.personProof = func(string, int64, humanauthority.Reader, string, string, time.Time) (humanauthority.Proof, error) {
				return humanauthority.Proof{}, errors.New("agent invocation")
			}
			if reason == "person" {
				conf := filepath.Join(bed.root(), "metasystem.conf")
				settings, err := os.ReadFile(conf)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(conf, append(settings, []byte("\nhost.builds=person\n")...), 0600); err != nil {
					t.Fatal(err)
				}
			}
			waiting := true
			bed.manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
				load := 0.0
				if waiting && reason == "load" {
					load = 100
				}
				return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: load}
			}
			brief := bed.brief("pending-run.md", "Build this unit.\n")
			argv := append([]string{"work", "build", bed.id, "--work", "pending-run", "--brief", brief, "--lines", "5"}, workCheck...)
			code, held, _ := pendingWork(t, bed, argv...)
			if code != 1 || held.Outcome != intentRefused {
				t.Fatalf("initial hold: code=%d outcome=%s summary=%s", code, held.Outcome, held.Summary)
			}
			run := resultData(t, held)["run"].(string)
			code, held, _ = pendingWork(t, bed, "work", "build", "run:"+run)
			want := []string{"metasystem", "work", "build", "run:" + run}
			if code != 1 || held.Outcome != intentRefused || len(bed.starter.launched()) != 0 {
				t.Fatalf("run build did not stay pending: code=%d next=%+v launches=%v", code, held.Next, bed.starter.launched())
			}
			if reason == "person" {
				question, err := channel.ReadQuestion(bed.root(), targetID(held.Targets, "question", ""))
				facts := strings.Join(question.Facts, "\n")
				if err != nil || !strings.Contains(facts, shellCommand(want)) || strings.Contains(facts, "metasystem work wait run:") {
					t.Fatalf("person question did not name the run build: error=%v facts=%s", err, facts)
				}
				bed.personProof = personProof
			} else if held.Next == nil || !slices.Equal(held.Next.Argv, want) {
				t.Fatalf("run build did not retain its build remedy: %+v", held.Next)
			}
			waiting = false
			code, built, _ := pendingWork(t, bed, want[1:]...)
			if code != 0 || resultData(t, built)["run"] != run || strings.Count(strings.Join(bed.starter.launched(), ","), "build") != 1 {
				t.Fatalf("run remedy did not resume the same build: code=%d summary=%s launches=%v", code, built.Summary, bed.starter.launched())
			}
		})
	}
}

func pendingWork(t *testing.T, bed *workBed, argv ...string) (int, intentResult, string) {
	t.Helper()
	owners := bed.workOwners()
	owners.processes.ask = func(root string, in channelAskInput) (channel.Question, []string, int, error) {
		return askChannelQuestionVia(root, in, channelAskSurface{
			identity: func(string) (string, string, error) { return "fixture-machine", "fixture-lineage", nil },
			load:     func(string) (phase.Loaded, error) { return phase.Loaded{}, nil },
			cursor:   func(string) (string, bool, error) { return "", false, nil },
		})
	}
	command, rest, ok := resolveIntentArgv(argv)
	if !ok {
		t.Fatalf("no public command %q", argv)
	}
	var out, stderr bytes.Buffer
	code := runIntentIn(command, append([]string{"--json"}, rest...), &out, &stderr, bed.root(), owners)
	var result intentResult
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("%v printed no JSON result: %v; stdout=%q stderr=%q", argv, err, out.String(), stderr.String())
	}
	bed.recordReadDirs(result)
	return code, result, stderr.String()
}
