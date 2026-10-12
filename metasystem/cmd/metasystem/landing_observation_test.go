package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
)

func TestLandingHealthUnreadableShadowStillReportsFactsAndStewardRecovers(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	owners := bed.owners()
	if _, err := lane.SetPauseBecause(bed.home, "fixture", "maintenance", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.RefreshShadow(bed.home, laneTestNow, "fixture", bed.plainProve); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(plain.Dir(bed.landingA), "state.json")
	corrupt := func() {
		t.Helper()
		if err := os.WriteFile(path, []byte(`{not json`), 0600); err != nil {
			t.Fatal(err)
		}
	}
	corrupt()
	seams := bed.plainProve
	seams.Steward = &plain.StewardEffects{Owner: func(string) (lane.OwnerProbe, error) { return lane.OwnerProbe{}, nil }}
	if _, err := plain.StewardStep(bed.home, laneTestNow, seams); err != nil {
		t.Fatalf("steward failed on unreadable shadow: %v", err)
	}
	for _, action := range []string{"health", "status"} {
		corrupt()
		command, _ := findIntentAction("landing", action)
		var out, problem bytes.Buffer
		code := runIntentIn(command, nil, &out, &problem, bed.cwd, owners)
		if code != 0 || !strings.Contains(out.String(), "the lane shadow cannot be read:") || !strings.Contains(out.String(), "the next refresh replaces it") {
			t.Fatalf("%s hid finding or failed: code=%d stdout=%s stderr=%s", action, code, &out, &problem)
		}
		if !strings.Contains(out.String(), "maintenance") {
			t.Fatalf("%s lost lane facts: %s", action, &out)
		}
		rebuilt, exists, err := state.Read(plain.Dir(bed.landingA))
		if err != nil || !exists || rebuilt.Lane.Word != "paused" {
			t.Fatalf("exit refresh did not repair shadow: %+v %v %v", rebuilt, exists, err)
		}
	}
	corrupt()
	command, _ := findIntentAction("landing", "status")
	var out, problem bytes.Buffer
	code := runIntentIn(command, []string{"--json"}, &out, &problem, bed.cwd, owners)
	var result struct{ Data plain.Status }
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 0 || result.Data.Root == nil || *result.Data.Root != bed.landingA || result.Data.Shadow != nil || result.Data.Health == nil {
		t.Fatalf("status lost lane with unreadable shadow: code=%d data=%+v stderr=%s", code, result.Data, &problem)
	}
	data, err := os.ReadFile(filepath.Join(plain.Dir(bed.landingA), "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var event state.Event
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatal(err)
	}
	if event.Kind != "rebuild" || !event.Rebuild || !strings.Contains(event.Evidence, "previous record unreadable: invalid character") {
		t.Fatalf("repair omitted unreadable predecessor: %+v", event)
	}
}

func TestLandingHealthCapturesBeforeViewAndRetakesResume(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	owners := bed.owners()
	if _, err := lane.SetPause(bed.home, "fixture", laneTestNow); err != nil {
		t.Fatal(err)
	}
	if _, err := plain.RefreshShadow(bed.home, laneTestNow, "fixture", bed.plainProve); err != nil {
		t.Fatal(err)
	}
	views, observations := 0, 0
	owners.landing.plainProve.Observed = func() { observations++ }
	owners.landing.view = func(home string) lane.View {
		views++
		if views == 1 {
			if _, err := lane.ClearPause(home); err != nil {
				t.Fatal(err)
			}
			current, _, _ := state.Read(plain.Dir(bed.landingA))
			current.Lane.Word = "running"
			if _, err := state.Replace(plain.Dir(bed.landingA), laneTestNow, current, "resume", current.Version, state.Fingerprint{Current: func() string { return "" }}); err != nil {
				t.Fatal(err)
			}
		}
		return lane.BuildView(lane.ViewSources{Home: home, Now: laneTestNow, Owner: owners.landing.probe})
	}
	command, _ := findIntentAction("landing", "health")
	var out, problem bytes.Buffer
	code := runIntentIn(command, []string{"--json"}, &out, &problem, bed.cwd, owners)
	var result struct{ Data plain.Health }
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if code != 0 || views != 2 || observations < 2 || result.Data.Shadow == nil || result.Data.Shadow.Lane.Word != "running" || result.Data.Paused != nil {
		t.Fatalf("health compared a mixed read: code=%d views=%d observations=%d data=%+v stderr=%s", code, views, observations, result.Data, &problem)
	}
}

func TestLandingObservationDeadSessionDoesNotWriteLaunch(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	store := launch.Store{Root: t.TempDir()}
	supervisor := identity.Ref{Pid: 10, StartedAtSec: 10}
	child := identity.Ref{Pid: 20, StartedAtSec: 20}
	record := launch.Record{ID: "dead-session", Kind: launch.LandingKind, State: launch.Running, StartedAt: laneTestNow.Format(time.RFC3339), WorkingDirectory: bed.landingA, Supervisor: &supervisor, Child: &child}
	if err := store.Create(record); err != nil {
		t.Fatal(err)
	}
	manager := &launch.Manager{Store: store, Prober: &treeProber{dead: true}}
	agent := newTestLandingAgent(func(a *landingAgent) { a.manager = func() *launch.Manager { return manager } })
	seam := bed.plainProve
	seam.AgentProbe = agent.probe
	dir, err := store.StateDir(record.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.RefreshShadow(bed.home, laneTestNow, "keeper", seam); err != nil {
		t.Fatal(err)
	}
	registered, _, _ := lane.Read(bed.home)
	status := plain.ObserveStatus(bed.home, registered, seam, nil)
	after, err := os.ReadFile(filepath.Join(dir, "record.json"))
	if err != nil {
		t.Fatal(err)
	}
	shadow, _, err := state.Read(plain.Dir(bed.landingA))
	if err != nil || shadow.Agent.Word != "ended" || status.AgentAlive || len(status.Problems) != 0 || !reflect.DeepEqual(before, after) {
		t.Fatalf("dead session treated as live or persisted: agent=%+v problems=%v error=%v", shadow.Agent, status.Problems, err)
	}
}

func TestLandingHealthWakeUsesCapturedMain(t *testing.T) {
	t.Parallel()
	bed, _, _, _, _ := landingRestartBed(t)
	owners := bed.owners()
	original := owners.landing.plainProve.Git
	reads := 0
	owners.landing.plainProve.Git = func(dir string, args ...string) (string, error) {
		if args[len(args)-1] == "refs/remotes/origin/main^{commit}" {
			reads++
		}
		return original(dir, args...)
	}
	command, _ := findIntentAction("landing", "health")
	var out, problem bytes.Buffer
	code := runIntentIn(command, []string{"--json"}, &out, &problem, bed.cwd, owners)
	// Health captures and validates once; the exit refresh additionally validates publication.
	if code != 0 || reads != 5 {
		t.Fatalf("wake reread a live main reference: code=%d reads=%d stderr=%s", code, reads, &problem)
	}
}
