package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
)

func TestResolutionLaunchFailureHasNoJobAndStatusNamesItsTime(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Attempt: "resolution", State: "resolving", Goal: "A", Commit: "main", Tip: "tip", Units: []string{"merge-A"}, Paths: []conflict.Path{{Path: "code.go"}}}
	err := LaunchResolution(b.install, fix, bedNow, "agent", func() (string, error) {
		saved, err := FixForAttempt(b.install, fix.Attempt)
		if err != nil || saved.Job != "" || saved.Launch == nil || saved.Launch.At != bedNow.Format(time.RFC3339Nano) || saved.Launch.Attempt != fix.Attempt {
			t.Fatalf("request missing before launch: %+v %v", saved, err)
		}
		return "unlaunched-job", errors.New("launcher unavailable")
	})
	if err == nil {
		t.Fatal("launch failure vanished")
	}
	saved, err := FixForAttempt(b.install, fix.Attempt)
	if err != nil || saved.Job != "" || saved.Launch.Error != "launcher unavailable" {
		t.Fatalf("false job: %+v %v", saved, err)
	}
	s := shadowSeams()
	s.Git = func(_ string, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "MERGE_HEAD") {
			return "tip", nil
		}
		if len(args) > 0 && args[0] == "diff" {
			return "code.go", nil
		}
		return "main", nil
	}
	status := ReadStatus(b.home, b.record, b.view, s)
	if !strings.Contains(status.Summary, "launch of the resolution failed at ") || !strings.Contains(status.Summary, bedNow.Local().Format("15:04")) || !strings.Contains(status.Summary, "launcher unavailable") {
		t.Fatalf("status: %+v", status)
	}
}

func TestEngineIncidentHealthListsUntilCleared(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	incident, err := RecordEngineIncident(b.install, "merge", []string{"A"}, bedNow, "nil error in landingLaneFailure", "stack frame")
	if err != nil {
		t.Fatal(err)
	}
	status := ReadStatus(b.home, b.record, b.view, shadowSeams())
	if status.Health == nil {
		t.Fatal(status.Problems)
	}
	text := status.Health.Text()
	if !strings.Contains(text, "an engine incident at "+bedNow.Local().Format("15:04")+" in landing merge: nil error in landingLaneFailure") {
		t.Fatal(text)
	}
	if err := os.Remove(filepath.Join(Dir(b.install), "incidents", incident.ID+".json")); err != nil {
		t.Fatal(err)
	}
	status = ReadStatus(b.home, b.record, b.view, shadowSeams())
	if strings.Contains(status.Health.Text(), "engine incident") {
		t.Fatal(status.Health.Text())
	}
}

func TestProofLaunchRequestPrecedesProcessAndRetainsFailure(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	b.seams.Person = &ActProvenance{Person: "Wido"}
	b.seams.Launch = func([]string, string, string) (int64, error) {
		saved, recorded, _, err := ReadRunning(b.install, b.seams)
		if err != nil || !recorded || saved.Pid != 0 || saved.Process != "" || saved.Launch == nil || saved.Launch.By != "Wido" {
			t.Fatalf("request before process: %+v %v", saved, err)
		}
		return 0, errors.New("launch failed")
	}
	_, _, err := Start(b.install, b.checkout, b.seams)
	if err == nil {
		t.Fatal("launch failure vanished")
	}
	saved, recorded, _, err := ReadRunning(b.install, b.seams)
	if err != nil || !recorded || saved.Pid != 0 || saved.Launch.Error != "launch failed" {
		t.Fatalf("failed request: %+v %v", saved, err)
	}
}

func TestLandingPlainSourcesHaveNoUnrecordedWait(t *testing.T) {
	t.Parallel()
	sources, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range sources {
		if strings.HasSuffix(source, "_test.go") {
			continue
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "unrecorded") {
			t.Errorf("%s still describes a wait without its start", source)
		}
	}
}

func TestRegenerationRequestPrecedesProcessAndRetainsFailure(t *testing.T) {
	t.Parallel()
	b := newResolveFixture(t)
	log, err := os.CreateTemp(b.install, "regeneration-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	running := RunningRegeneration{Launch: &LaunchRecord{By: "Wido"}, Goal: "goal", SHA: "goal-sha", Since: bedNow.Format(time.RFC3339Nano), Log: log.Name()}
	err = runRegeneration(b.home, b.install, running, []string{"compile"}, b.install, log, func(_ []string, _ string, _ *os.File, _ func(int64) error) error {
		saved, err := ReadRunningRegeneration(b.install, ProveSeams{})
		if err != nil || saved == nil || saved.Pid != 0 || saved.Launch == nil || saved.Launch.By != "Wido" {
			t.Fatalf("missing request: %+v %v", saved, err)
		}
		return errors.New("cannot start compiler")
	})
	saved, readErr := ReadRunningRegeneration(b.install, ProveSeams{})
	if err == nil || readErr != nil || saved == nil || saved.Pid != 0 || saved.Launch.Error != "cannot start compiler" || saved.State != "launch-failed" {
		t.Fatalf("failed request: %+v %v %v", saved, err, readErr)
	}
	if err := StopRegeneration(b.install); err != nil {
		t.Fatalf("stop after failed launch: %v", err)
	}
	if active, err := drainExecution(b.install, ProveSeams{}); active || err != nil {
		t.Fatalf("failed launch blocks drain: %t %v", active, err)
	}
	saved, readErr = ReadRunningRegeneration(b.install, ProveSeams{})
	if readErr != nil || saved == nil || saved.Launch.Error != "cannot start compiler" {
		t.Fatalf("stop discarded failure: %+v %v", saved, readErr)
	}
}

func TestLegacySealWaitUsesItsSavedObservation(t *testing.T) {
	t.Parallel()
	shadow := state.Record{Batch: state.Batch{Admission: state.Fact{Since: state.Since{At: bedNow, Observed: true}}}}
	health := Health{Shadow: &shadow, Batch: &BatchHealth{ID: "legacy"}, observedAt: bedNow.Add(time.Hour)}
	text := health.Text()
	if !strings.Contains(text, "seen since "+bedNow.Local().Format("15:04")+" (the record carries no transition time)") {
		t.Fatal(text)
	}
}

func TestEngineIncidentRecurrenceKeepsOneOpenRecord(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	first, err := RecordEngineIncident(b.install, "merge", nil, bedNow, "panic", "first stack")
	if err != nil {
		t.Fatal(err)
	}
	again, err := RecordEngineIncident(b.install, "merge", []string{"A"}, bedNow.Add(time.Minute), "panic", "second stack")
	if err != nil {
		t.Fatal(err)
	}
	incidents, err := EngineIncidents(b.install)
	if err != nil || len(incidents) != 1 || again.ID != first.ID {
		t.Fatalf("recurrence: %+v %v", incidents, err)
	}
	data, err := os.ReadFile(filepath.Join(Dir(b.install), "incidents", first.ID+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var record struct {
		Count  int       `json:"count"`
		LastAt time.Time `json:"last_at"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatal(err)
	}
	if record.Count != 2 || !record.LastAt.Equal(bedNow.Add(time.Minute)) {
		t.Fatalf("recurrence fields: %+v", record)
	}
	for _, different := range []struct{ verb, message string }{{"push", "panic"}, {"merge", "other panic"}} {
		if _, err := RecordEngineIncident(b.install, different.verb, nil, bedNow.Add(2*time.Minute), different.message, "stack"); err != nil {
			t.Fatal(err)
		}
	}
	incidents, err = EngineIncidents(b.install)
	if err != nil || len(incidents) != 3 {
		t.Fatalf("different incidents: %+v %v", incidents, err)
	}
}

func TestWaitWithoutRecordedTimeStaysUnknown(t *testing.T) {
	t.Parallel()
	if got := waitSince("", state.Since{}, bedNow); got != "an unknown time" {
		t.Fatal(got)
	}
	status := Status{RunningProof: &RunningProof{State: "running"}}
	shadow := ShadowOf(status, RawFacts{Now: bedNow, Fix: Fix{Attempt: "fix", State: "resolving"}})
	if !shadow.Proof.Since.At.IsZero() || !shadow.Resolution.Since.At.IsZero() {
		t.Fatalf("invented time: %+v", shadow)
	}
}

func TestAgentProofLaunchNamesLanding(t *testing.T) {
	t.Parallel()
	b := newRepeatBed(t)
	b.seams.Launch = func([]string, string, string) (int64, error) {
		saved, _, _, err := ReadRunning(b.install, b.seams)
		if err != nil || saved.Launch == nil || saved.Launch.By != "landing" {
			t.Fatalf("agent launch author: %+v %v", saved, err)
		}
		return 0, errors.New("fixture launch unavailable")
	}
	if _, _, err := Start(b.install, b.checkout, b.seams); err == nil {
		t.Fatal("failure lost")
	}
}

func TestResolutionAdoptionKeepsLaunchIdentityAndStart(t *testing.T) {
	t.Parallel()
	b := newStatusBed(t)
	fix := &Fix{Attempt: "resolution", State: "resolving", Launch: &LaunchRecord{Job: "job-id"}}
	if err := LaunchResolution(b.install, fix, bedNow, "landing", func() (string, error) {
		saved, err := FixForAttempt(b.install, fix.Attempt)
		if err != nil || saved.Launch == nil || saved.Launch.Job != "job-id" || saved.Launch.Attempt != fix.Attempt || saved.Job != "" {
			t.Fatalf("request identity: %+v %v", saved, err)
		}
		return "job-id", nil
	}); err != nil {
		t.Fatal(err)
	}
	fix.Job = ""
	if err := LaunchResolution(b.install, fix, bedNow.Add(time.Hour), "landing", func() (string, error) { return "job-id", nil }); err != nil {
		t.Fatal(err)
	}
	saved, err := FixForAttempt(b.install, fix.Attempt)
	if err != nil || saved.Job != "job-id" || saved.Launch.Job != "job-id" || saved.Launch.At != bedNow.Format(time.RFC3339Nano) {
		t.Fatalf("adoption moved identity/time: %+v %v", saved, err)
	}
}
