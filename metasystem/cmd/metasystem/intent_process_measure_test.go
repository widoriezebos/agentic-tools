package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/channel"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/processmeasure"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy/adapter/fakeadapter"
)

type processCostStarter struct {
	*workStarter
	clock *workClock
}

func (s processCostStarter) StartSupervisor(id, root string) (identity.Ref, error) {
	pending := s.clock.Now()
	ref, err := s.workStarter.StartSupervisor(id, root)
	if err != nil {
		return ref, err
	}
	start := pending.Add(2 * time.Minute)
	end := start.Add(time.Minute)
	s.clock.Sleep(23 * time.Minute)
	_, err = s.m.Store.Update(id, func(record *launch.Record) error {
		record.StartedAt, record.FinishedAt = start.Format(time.RFC3339Nano), end.Format(time.RFC3339Nano)
		record.Measurement = launch.Measurement{UsageRead: true, InputTokens: 10, CacheReadTokens: 20, CacheCreationTokens: 30, OutputTokens: 40}
		return nil
	})
	return ref, err
}

func TestProcessStepCostPublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	bed.lineage = "builder"
	processEstimatePage(t, bed, "70", "10")
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	clock := &workClock{now: start}
	bed.manager.Now = clock.Now
	bed.manager.Supervisor = processCostStarter{bed.starter, clock}
	full := []string{"selected-tool", "--full"}
	declaredFull := " selected-tool   --full"
	gitHook := bed.workOwnersHook
	bed.workOwnersHook = func(owners *intentWorkOwners) {
		gitHook(owners)
		git := owners.git
		owners.git = func(root string, args ...string) ([]byte, error) {
			if len(args) == 2 && args[0] == "show" && strings.HasSuffix(args[1], ":metasystem.conf") {
				return []byte("proof.full=" + declaredFull + "\n"), nil
			}
			return git(root, args...)
		}
	}
	fake := fakeadapter.New()
	fake.Steps = []adapter.GateStep{{Name: "unrelated-display-name", Args: full}}
	fake.Scripted.Root = t.TempDir()
	bed.testingAdapter = workTestingAdapter{fake, func(string, string, string) (adapter.Closure, error) { return fake.Scripted, nil }}
	brief := bed.brief("cost.md", "Read each round: yes\nBuild the unit.\n")
	check := slices.Insert(workArgv, 2, "-timeout", "30m")
	args := append([]string{"work", "build", bed.id, "--work", "evidence", "--brief", brief, "--lines", "250", "--read-tool-calls", "12", "--check"}, check...)
	code, built, output := bed.work(args...)
	if code != 0 {
		t.Fatalf("build exit %d: %+v %s", code, built, output)
	}
	run := resultData(t, built)["run"].(string)
	declaredFull = "different-tool --full"
	fix := bed.brief("cost-fix.md", "Correct the unit.\n")
	if code, result, output := bed.work("work", "revise", bed.id, "--work", "evidence", "--after", "1", "--brief", filepath.Join(bed.root(), fix)); code != 0 {
		t.Fatalf("correction exit %d: %+v %s", code, result, output)
	}
	for _, question := range []channel.Question{
		{ID: "closed", Goal: bed.id, State: "answered", OpenedAt: start.Add(5 * time.Minute), Answer: &channel.Answer{At: start.Add(15 * time.Minute)}},
		{ID: "open", Goal: bed.id, State: "open", OpenedAt: start.Add(10 * time.Minute)},
		{ID: "another-goal", Goal: "elsewhere", State: "open", OpenedAt: start},
		// An earlier unit's question of the same goal, answered before this run began, is not this unit's wait.
		{ID: "earlier-unit", Goal: bed.id, State: "answered", OpenedAt: start.Add(-10 * time.Hour), Answer: &channel.Answer{At: start.Add(-9 * time.Hour)}},
	} {
		path := filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", question.ID+".json")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(question)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	status := func() processmeasure.Measures {
		t.Helper()
		code, result, output := bed.work("work", "status", bed.id, "--work", "evidence")
		if code != 0 {
			t.Fatalf("status exit %d: %+v %s", code, result, output)
		}
		work := resultData(t, result)["work"].([]any)[0].(map[string]any)
		if work["state"] != "awaiting-judgement" {
			t.Fatalf("current state lost: %+v", work)
		}
		data, err := json.Marshal(work["measures"])
		if err != nil {
			t.Fatal(err)
		}
		var measures processmeasure.Measures
		if err := json.Unmarshal(data, &measures); err != nil {
			t.Fatal(err)
		}
		return measures
	}
	// The run is finished: its window ends at the last collection, so the open
	// question is clipped there and the person time is exact, not a lower bound.
	for repeat := 0; repeat < 2; repeat++ {
		m := status()
		for kind, minutes := range map[string]float64{"build": 1, "attest": 4, "read": 2, "correction": 1, "pending": 16, "collection": 160, "person": 179} {
			if m.Hours[kind] == nil || fmt.Sprintf("%.6f", *m.Hours[kind]) != fmt.Sprintf("%.6f", minutes/60) {
				t.Fatalf("%s time: %+v, want %g hours", kind, m, minutes/60)
			}
		}
		if m.SuiteMinutes == nil || *m.SuiteMinutes != 2 || m.Run != run || m.Corrections != 1 || m.PersonLowerBound || m.WorkLowerBound || m.EstimateMinutes == nil || *m.EstimateMinutes != 70 || m.ElapsedFinish != nil || len(m.Sources) != 8 || m.UsageKnown != 4 || m.UsageExpected != 4 || m.Tokens == nil || *m.Tokens != (processmeasure.Tokens{Input: 40, CacheRead: 80, CacheCreation: 120, Output: 160}) {
			t.Fatalf("cost/coverage: %+v", m)
		}
	}
	m := status()
	if _, err := bed.manager.Store.Update(m.Sources[0], func(record *launch.Record) error { record.Measurement.UsageRead = false; return nil }); err != nil {
		t.Fatal(err)
	}
	missing := status()
	if missing.Tokens != nil || missing.UsageKnown != 3 || missing.UsageExpected != 4 || missing.ReportedTokens.Input != 30 {
		t.Fatalf("missing usage became zero: %+v", missing)
	}
	command, rest, ok := resolveIntentArgv([]string{"work", "status", bed.id, "--work", "evidence"})
	if !ok {
		t.Fatal("status did not resolve")
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, bed.root(), bed.workOwners()); code != 0 {
		t.Fatalf("text status exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	for _, fragment := range []string{"hours: build", "estimate 70 minutes", "tokens unavailable", "full suite minutes 2", "elapsed finish unavailable"} {
		if !strings.Contains(strings.Join(strings.Fields(stdout.String()), " "), fragment) {
			t.Fatalf("text status missing %q: %s", fragment, stdout.String())
		}
	}
	// Duplicate retained references must not add a second physical execution.
	path := filepath.Join(bed.unitRoot, run, "run.json")
	record, err := (&launch.UnitRunner{Manager: bed.manager, Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	record.Rounds[0].Steps = append(record.Rounds[0].Steps, record.Rounds[0].Steps[0])
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := status(); len(got.Sources) != 8 || *got.Hours["build"] != *m.Hours["build"] {
		t.Fatalf("duplicate execution grew cost: %+v", got)
	}
	if _, err := bed.manager.Store.Update(m.Sources[0], func(record *launch.Record) error {
		record.FinishedAt = start.Format(time.RFC3339Nano)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got := status(); got.Hours["build"] != nil {
		t.Fatalf("negative execution interval became a value: %+v", got)
	}
	closedPath := filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", "closed.json")
	ended := channel.Question{ID: "closed", Goal: bed.id, State: "resolved", OpenedAt: start.Add(5 * time.Minute)}
	data, err = json.Marshal(ended)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(closedPath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := status(); got.Hours["person"] != nil {
		t.Fatalf("missing question closing time became a value: %+v", got)
	}
	// Missing rows are advisory even for an agent.
	noRow := newWorkBed(t)
	noRow.lineage = "builder"
	page, accepted := designGatePage(t, noRow, "- Critique: ruled by Wido 2026-10-07, accepted\n")
	processCommittedPage(t, noRow, page, accepted)
	if code, result, output := processEvidenceBuild(noRow); code != 0 || !strings.Contains(output, "estimate unavailable") {
		t.Fatalf("missing row held build: %d %+v %s", code, result, output)
	}
}

func processCostStatus(t *testing.T, bed *workBed) processmeasure.Measures {
	t.Helper()
	code, result, output := bed.work("work", "status", bed.id, "--work", "evidence")
	if code != 0 {
		t.Fatalf("status exit %d: %+v %s", code, result, output)
	}
	work := resultData(t, result)["work"].([]any)[0].(map[string]any)
	data, err := json.Marshal(work["measures"])
	if err != nil {
		t.Fatal(err)
	}
	var measures processmeasure.Measures
	if err := json.Unmarshal(data, &measures); err != nil {
		t.Fatal(err)
	}
	return measures
}

func processCostQuestion(t *testing.T, bed *workBed, question channel.Question) {
	t.Helper()
	path := filepath.Join(bed.root(), "artifacts", "agents", "channel", "questions", question.ID+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(question)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestProcessFinishedRunPersonTimePublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	start := time.Date(2026, 10, 7, 10, 0, 0, 0, time.UTC)
	clock := &workClock{now: start}
	bed.manager.Now = clock.Now
	bed.manager.Supervisor = processCostStarter{bed.starter, clock}
	code, built, output := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("build exit %d: %+v %s", code, built, output)
	}
	run := resultData(t, built)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	var lastCollection time.Time
	for _, round := range record.Rounds {
		for _, step := range round.Steps {
			at, err := time.Parse(time.RFC3339Nano, step.FinishedAt)
			if err != nil || (step.State != launch.StepPassed && step.State != launch.StepFailed) {
				t.Fatalf("step is not finished with a collection time: %+v", step)
			}
			if at.After(lastCollection) {
				lastCollection = at
			}
		}
	}
	if lastCollection.IsZero() {
		t.Fatal("finished run has no collection time")
	}
	processCostQuestion(t, bed, channel.Question{ID: "this-unit", Goal: bed.id, State: "answered",
		OpenedAt: start.Add(5 * time.Minute), Answer: &channel.Answer{At: start.Add(15 * time.Minute)}})
	before := processCostStatus(t, bed)
	if before.Run != run || before.Hours["person"] == nil || *before.Hours["person"] != (10*time.Minute).Hours() || before.PersonLowerBound {
		t.Fatalf("finished run person time: %+v", before)
	}
	clock.Sleep(time.Hour)
	// A later unit's question belongs to the same goal, outside this run's window.
	for _, offset := range []time.Duration{time.Minute, 0} {
		opened := lastCollection.Add(offset)
		processCostQuestion(t, bed, channel.Question{ID: "later-unit", Goal: bed.id, State: "answered",
			OpenedAt: opened, Answer: &channel.Answer{At: opened.Add(10 * time.Minute)}})
		after := processCostStatus(t, bed)
		if after.Run != run || after.Hours["person"] == nil || *after.Hours["person"] != *before.Hours["person"] || after.PersonLowerBound {
			t.Fatalf("question opened %s after collection changed finished run person hours: before %s, after %s (lower bound %t)", offset, measureNumber(before.Hours["person"]), measureNumber(after.Hours["person"]), after.PersonLowerBound)
		}
	}
	clock.Sleep(time.Hour)
	if after := processCostStatus(t, bed); after.Hours["person"] == nil || *after.Hours["person"] != *before.Hours["person"] || after.PersonLowerBound {
		t.Fatalf("later observation changed finished run person time: %+v", after)
	}
}

func TestProcessUnstartedRunPersonTimePublicStatus(t *testing.T) {
	t.Parallel()
	bed := newWorkBed(t)
	code, built, output := processEvidenceBuild(bed)
	if code != 0 {
		t.Fatalf("build exit %d: %+v %s", code, built, output)
	}
	run := resultData(t, built)["run"].(string)
	record, err := (&launch.UnitRunner{Root: bed.unitRoot}).Status(run)
	if err != nil {
		t.Fatal(err)
	}
	// Retain the named run's binding with only its initial, unstarted build step.
	record.State = "running"
	record.Rounds = []launch.UnitRound{{Number: 1, Steps: []launch.UnitStep{{Name: "build", Kind: "build", State: launch.StepPending}}}}
	data, err := json.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.unitRoot, run, "run.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	measures := processCostStatus(t, bed)
	if measures.Run != run || measures.Hours["person"] != nil || !slices.Contains(measures.Unknown, "person time unavailable") {
		t.Fatalf("unstarted run must report person time unavailable: %+v", measures)
	}
	command, rest, ok := resolveIntentArgv([]string{"work", "status", bed.id, "--work", "evidence"})
	if !ok {
		t.Fatal("status did not resolve")
	}
	var stdout, stderr bytes.Buffer
	if code := runIntentIn(command, rest, &stdout, &stderr, bed.root(), bed.workOwners()); code != 0 {
		t.Fatalf("text status exit %d: %s %s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(strings.Join(strings.Fields(stdout.String()), " "), "person unavailable") {
		t.Fatalf("text status must report person time unavailable: %s", stdout.String())
	}
}
