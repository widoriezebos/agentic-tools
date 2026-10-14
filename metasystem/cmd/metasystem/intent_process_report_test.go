package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processchange"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
)

func TestUnitCarryPublicCostReport(t *testing.T) {
	t.Parallel()
	for _, key := range []string{"commit", "unit"} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()
			bed := newWorkBed(t)
			start := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
			bed.manager.Now = func() time.Time { return start.Add(10 * time.Minute) }
			act := processchange.ProcessAct{ID: "carry-act", Goal: bed.id, Actor: "agent", AppliedAt: start}
			writeProcessRootJSON(t, filepath.Join(bed.root(), "process", "acts", act.ID+".json"), act)
			for index, unit := range []string{"first", "second"} {
				run, commit := "carry-"+unit, unit+"-commit"
				plan := filepath.Join(bed.unitRoot, run, "plan.json")
				writeProcessRootJSON(t, plan, launch.UnitPlan{FullArgv: []string{"/bin/sh", "-c", "full"}})
				record := launch.UnitRunRecord{ID: run, Goal: bed.id, Unit: unit, Worktree: bed.worktree, Plan: plan, State: "awaiting-judgement",
					Rounds: []launch.UnitRound{{Number: 1}}, Subjects: []launch.UnitSubject{{Round: 1, Commit: commit}}}
				writeProcessRootJSON(t, filepath.Join(bed.unitRoot, run, "run.json"), record)
				subject := commit
				if key == "unit" {
					subject = unit
				}
				observation := launch.CheckExecution{ExecutionID: "check-" + unit, Goal: bed.id, Run: subject, Role: "carry"}
				for command, name := range []string{"cheap", "audits"} {
					from := start.Add(time.Duration(command) * time.Minute)
					to := from.Add(time.Duration(1+index*command) * time.Minute)
					observation.Steps = append(observation.Steps, processmeasure.Step{
						ID: subject + "/" + observation.ExecutionID + ":" + name, Kind: "attest", Act: act.ID,
						Start: from.Format(time.RFC3339Nano), End: to.Format(time.RFC3339Nano), Terminal: true, Outcome: "passed", Coverage: "unknown",
						Argv: []string{"/bin/sh", "-c", name}, FullArgv: []string{"/bin/sh", "-c", "full"}})
				}
				writeProcessRootJSON(t, filepath.Join(bed.root(), "artifacts", "unit-checks", "carry", subject, observation.ExecutionID, "observation.json"), observation)
			}
			owners := bed.workOwners()
			owners.processes.launches = func() *launch.Manager { return bed.manager }
			for _, want := range []struct {
				target   string
				minutes  float64
				children int
			}{{"run:carry-first", 2, 2}, {"run:carry-second", 3, 2}, {bed.id, 5, 4}} {
				code, result := bed.runJSON(owners, "work", "status", want.target)
				if code != 0 {
					t.Fatalf("status %s: %d %+v", want.target, code, result)
				}
				body, err := json.Marshal(resultData(t, result)["processReport"])
				if err != nil {
					t.Fatal(err)
				}
				var report processmeasure.Projection
				if err := json.Unmarshal(body, &report); err != nil {
					t.Fatal(err)
				}
				if report.Measures == nil || report.Measures.Hours["attest"] == nil || math.Abs(*report.Measures.Hours["attest"]*60-want.minutes) > 1e-8 || len(report.Measures.Children) != want.children {
					t.Fatalf("%s carry cost: %+v; want %g minutes and %d children", want.target, report.Measures, want.minutes, want.children)
				}
				if !strings.Contains(strings.Join(report.Lines, "\n"), fmt.Sprintf("Process act %s: own process cost; actor agent;", act.ID)) || !strings.Contains(strings.Join(report.Lines, "\n"), fmt.Sprintf("observed %g minutes;", want.minutes)) {
					t.Fatalf("%s consumed act cost: %v", want.target, report.Lines)
				}
			}
		})
	}
}

func TestProcessCostStatusAndChannelPublicReport(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	clock := &workClock{now: time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)}
	bed.manager.Now, bed.manager.Supervisor = clock.Now, processCostStarter{bed.starter, clock}
	processEstimatePage(t, bed, "1", "100")
	code, built, output := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("build: %d %+v %s", code, built, output)
	}
	run := resultData(t, built)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	source := record.Rounds[0].Steps[0].LaunchID
	unconsumed := record.Rounds[0].Steps[1].LaunchID
	seed := func(act processchange.ProcessAct) {
		t.Helper()
		path := filepath.Join(bed.root(), "process", "acts", act.ID+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(act)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// Serialized historical evidence uses the production record shapes, including a grant.
	seed(processchange.ProcessAct{ID: "helm-act", Goal: bed.id, Status: "applied", Actor: "direct-person", Lineage: "builder", Key: launch.CodexSandboxKey, After: "workspace-write", AppliedAt: clock.Now().Add(-24 * time.Hour), Proof: humanauthority.Proof{Helm: &humanauthority.HelmGrant{By: "Wido", Grant: "attorney-42"}}})
	seed(processchange.ProcessAct{ID: "person-act", Goal: bed.id, Status: "applied", Actor: "direct-person", AppliedAt: clock.Now().Add(-25 * time.Hour), Key: "launch.read.model", After: "selected-model"})
	seed(processchange.ProcessAct{ID: "", Goal: bed.id, Status: "applied", Actor: "agent", AppliedAt: clock.Now()})
	active, err := bed.manager.Store.Read(source)
	if err != nil {
		t.Fatal(err)
	}
	if active.AdapterData == nil {
		active.AdapterData = map[string]json.RawMessage{}
	}
	active.AdapterData["sandboxAct"] = json.RawMessage(`"helm-act"`)
	active.StartedAt, active.FinishedAt, active.State = clock.Now().Add(-time.Minute).Format(time.RFC3339Nano), "", launch.Running
	dir, err := bed.manager.Store.StateDir(source)
	if err != nil {
		t.Fatal(err)
	}
	activeBody, err := json.Marshal(active)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "record.json"), activeBody, 0600); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var sent []string
	failing := true
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.URL.Path != "/chat.postMessage" {
			t.Errorf("unexpected provider method: %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		sent = append(sent, r.Form.Get("text"))
		w.Header().Set("Content-Type", "application/json")
		if failing {
			fmt.Fprint(w, `{"ok":false,"error":"internal_error"}`)
		} else {
			fmt.Fprint(w, `{"ok":true,"ts":"123.000001"}`)
		}
	}))
	t.Cleanup(provider.Close)
	fakeDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(fakeDir, "base-url"), []byte(provider.URL), 0600); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(bed.root(), "metasystem.conf")
	body, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	body = append(body, []byte("\nchannel.destination.fleet.adapter=fake\nchannel.destination.fleet.fake.dir="+fakeDir+"\n")...)
	if err := os.WriteFile(conf, body, 0600); err != nil {
		t.Fatal(err)
	}
	invoke := func(args ...string) (int, intentResult, string) {
		t.Helper()
		command, rest, ok := resolveIntentArgv(args)
		if !ok {
			t.Fatalf("unresolved public command: %v", args)
		}
		var out, problem bytes.Buffer
		owners := bed.workOwners()
		owners.processes.launches = func() *launch.Manager { return bed.manager }
		code := runIntentIn(command, rest, &out, &problem, bed.root(), owners)
		var result intentResult
		if len(rest) > 0 && rest[len(rest)-1] == "--json" {
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("decode: %v %s %s", err, &out, &problem)
			}
		}
		return code, result, out.String() + problem.String()
	}
	assertReport := func(lines []string) {
		t.Helper()
		text := strings.Join(lines, "\n")
		if len(lines) == 0 || !strings.HasPrefix(lines[0], "Automatic process changes are held:") {
			t.Fatalf("drift is not first: %s", text)
		}
		for _, fragment := range []string{"Process act helm-act: own process cost", "grant attorney-42", "actor agent; lineage builder", "observed 1 minutes", "Recorded work: build 0.0166667 hours", "Process act person-act: person-directed process cost", "counterfactual unknown", "External delay: collection", "Unknown: nested checks unavailable", "Unknown: report boundary"} {
			if !strings.Contains(text, fragment) {
				t.Fatalf("missing %q: %s", fragment, text)
			}
		}
	}
	readStatus := func(args ...string) processmeasure.Projection {
		t.Helper()
		code, result, out := invoke(append(args, "--json")...)
		if code != 0 {
			t.Fatalf("status: %d %s", code, out)
		}
		body, err := json.Marshal(resultData(t, result)["processReport"])
		if err != nil {
			t.Fatal(err)
		}
		var report processmeasure.Projection
		if err := json.Unmarshal(body, &report); err != nil {
			t.Fatal(err)
		}
		if _, attributed := report.Watermark[unconsumed]; attributed {
			t.Fatal("unconsumed work was attributed through an empty act identity")
		}
		return report
	}
	for _, args := range [][]string{{"goal", "status", bed.id}, {"work", "status", bed.id, "--work", "evidence"}, {"work", "status", "run:" + run}} {
		assertReport(readStatus(args...).Lines)
	}
	if state := channel.LoadStatusState(bed.root()); state.ProcessWatermark != nil || state.Pending != nil {
		t.Fatalf("status read advanced delivery state: %+v", state)
	}
	_, _, text := invoke("goal", "status", bed.id)
	if strings.Index(text, "Automatic process changes are held:") > strings.Index(text, "Process act helm-act:") {
		t.Fatalf("visible status order: %s", text)
	}
	code, _, text = invoke("channel", "status")
	if code != 0 || !strings.Contains(text, "Process act helm-act: own process cost") {
		t.Fatalf("channel read: %d %s", code, text)
	}
	if state := channel.LoadStatusState(bed.root()); state.ProcessWatermark != nil || state.Pending != nil {
		t.Fatalf("channel read advanced delivery state: %+v", state)
	}
	code, _, text = invoke("channel", "status", "--post")
	if code != 1 {
		t.Fatalf("failed delivery: %d %s", code, text)
	}
	failed := channel.LoadStatusState(bed.root())
	if failed.ProcessWatermark != nil || !failed.LastPost.IsZero() || failed.Pending == nil {
		t.Fatalf("failed delivery advanced boundary: %+v", failed)
	}
	channelLines := strings.Split(failed.Pending.Text, "\n")
	if len(channelLines) > 12 || !strings.HasPrefix(channelLines[0], "Automatic process changes are held:") || !strings.Contains(failed.Pending.Text, "observed 1 minutes") {
		t.Fatalf("channel priority or line budget: %s", failed.Pending.Text)
	}
	if failed.Pending.Watermark["act:helm-act"].Revision == "" || failed.Pending.Watermark[source].Revision == "" {
		t.Fatal("included act or observation revision missing")
	}
	seed(processchange.ProcessAct{ID: "next-act", Goal: bed.id, Status: "applied", Actor: "agent", AppliedAt: clock.Now(), Key: "launch.read.model"})
	clock.Sleep(2 * time.Minute)
	mu.Lock()
	failing = false
	mu.Unlock()
	code, _, text = invoke("channel", "status", "--post")
	if code != 0 {
		t.Fatalf("retry: %d %s", code, text)
	}
	delivered := channel.LoadStatusState(bed.root())
	if delivered.Pending != nil || !reflect.DeepEqual(delivered.ProcessWatermark, failed.Pending.Watermark) {
		t.Fatalf("retry changed candidate boundary: %+v", delivered)
	}
	mu.Lock()
	if len(sent) != 2 || sent[0] != sent[1] || strings.Contains(sent[1], "next-act") {
		t.Fatalf("failed message not retained: %+v", sent)
	}
	mu.Unlock()
	for _, args := range [][]string{{"goal", "status", bed.id}, {"work", "status", bed.id, "--work", "evidence"}, {"work", "status", "run:" + run}} {
		full := readStatus(args...)
		if !strings.Contains(strings.Join(full.Lines, "\n"), "observed 3 minutes") {
			t.Fatalf("status lost full cost after delivery: %+v", full)
		}
	}
	if !reflect.DeepEqual(channel.LoadStatusState(bed.root()).ProcessWatermark, delivered.ProcessWatermark) {
		t.Fatal("status advanced successful watermark")
	}
	code, _, text = invoke("channel", "status", "--post")
	if code != 0 || !strings.Contains(text, "observed 2 minutes") || !strings.Contains(text, "next-act") {
		t.Fatalf("next report: %d %s", code, text)
	}

	if err := os.WriteFile(filepath.Join(bed.root(), "artifacts", "agents", "channel", "status.json"), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	unknown := readStatus("goal", "status", bed.id)
	if text := strings.Join(unknown.Lines, "\n"); !strings.Contains(text, "Unknown: report boundary") || !strings.Contains(text, "observed 3 minutes") {
		t.Fatalf("unreadable watermark hid retained cost: %s", text)
	}

	actsPath := filepath.Join(bed.root(), "process", "acts")
	if err := os.Rename(actsPath, actsPath+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(actsPath, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	damaged := readStatus("goal", "status", bed.id)
	if damaged.Measures.Hours["build"] == nil || !strings.Contains(strings.Join(damaged.Lines, "\n"), "process history unavailable:") {
		t.Fatalf("unreadable history became zero coverage: %+v", damaged)
	}

	if err := os.Rename(bed.unitRoot, bed.unitRoot+"-retained"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bed.unitRoot, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	code, _, text = invoke("channel", "status")
	if code != 0 || !strings.Contains(text, "process measurements unavailable:") || !strings.Contains(text, "Recorded work: build unavailable") {
		t.Fatalf("unreadable runs became zero cost: %d %s", code, text)
	}
}
