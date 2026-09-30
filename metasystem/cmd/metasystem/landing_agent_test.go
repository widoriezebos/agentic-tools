package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/outage"
)

// recordingSupervisor stands in for the detached launch supervisor: it
// records the child as started, so Start returns with a running record.
type recordingSupervisor struct{ store launch.Store }

func (s recordingSupervisor) StartSupervisor(id, _ string) (identity.Ref, error) {
	_, err := s.store.Update(id, func(record *launch.Record) error {
		child := identity.Ref{Pid: 20, StartedAtSec: 20}
		record.Child, record.ProcessGroup, record.State = &child, &child, launch.Running
		return nil
	})
	return identity.Ref{Pid: 10, StartedAtSec: 10}, err
}

// TestLandingAgentStartsOnTheLaneWithItsRoster (A-a): the steward of the
// registered lane checkout wakes the landing agent through the real launch
// manager and its lane guard: the landing kind, in the nested lane
// checkout, bound to its module's fence, on the runtime and model the lane
// installation's metasystem.conf.local names (D2), with a brief naming why
// it was woken; one at a time. Another checkout's steward starts none, and a
// provider outage recorded from an ended agent holds the next start.
func TestLandingAgentStartsOnTheLaneWithItsRoster(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	home, checkout := filepath.Join(base, "home"), filepath.Join(base, "landing")
	module := filepath.Join(checkout, "metasystem")
	for _, dir := range []string{home, module} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		filepath.Join(module, "go.mod"):                "module fixture\n",
		filepath.Join(module, "metasystem.conf"):       "# overrides only\n",
		filepath.Join(module, "metasystem.conf.local"): "launch.landing.runtime=claude\nlaunch.landing.model=claude-roster-model\nlaunch.landing.effort=high\n",
	}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := lane.Register(home, checkout, "a-person", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	checkout, module = resolvedPath(checkout), resolvedPath(module)
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	store := launch.Store{Root: filepath.Join(base, "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude"}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return home, nil })}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, settings: installationSettings, now: func() time.Time { return now },
		nonce: func() (string, error) { return "0011223344556677", nil },
		gateSettings: func(asked string) (string, error) {
			if asked != module {
				t.Errorf("the gate settings were asked for %s, want the lane installation %s", asked, module)
			}
			return filepath.Join(base, "gate", "landing-settings.json"), nil
		}}
	keeper := newLandingAgentKeeper(module, home, agent)
	keeper.Sources.Validation = func(string, time.Time) (bool, error) { return true, nil }

	line := keeper.Step()
	records, err := store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("after a due validation: line %q, launches %+v %v; want one", line, records, err)
	}
	record := records[0]
	var model, effort, fence, briefPath, gate string
	for key, into := range map[string]*string{"model": &model, "effort": &effort, "fenceRoot": &fence, "brief": &briefPath, "settings": &gate} {
		_ = json.Unmarshal(record.AdapterData[key], into)
	}
	if record.Kind != launch.LandingKind || record.WorkingDirectory != checkout || fence != module || record.Adapter != "claude-headless" ||
		model != "claude-roster-model" || effort != "high" {
		t.Fatalf("landing launch = kind %q dir %q fence %q adapter %q model %q effort %q; want landing in %s fenced to %s on the roster",
			record.Kind, record.WorkingDirectory, fence, record.Adapter, model, effort, checkout, module)
	}
	if gate != filepath.Join(base, "gate", "landing-settings.json") {
		t.Fatalf("the landing launch record carries settings %q; want the tool gate's settings file", gate)
	}
	brief, err := os.ReadFile(briefPath)
	if err != nil || !strings.Contains(string(brief), lane.WakeValidationDue) || !strings.HasPrefix(briefPath, module+string(filepath.Separator)) {
		t.Fatalf("brief %s = %q %v; want it in the lane installation naming %s", briefPath, brief, err, lane.WakeValidationDue)
	}
	if !strings.Contains(line, record.ID) {
		t.Fatalf("keeper line %q does not name the launch %s", line, record.ID)
	}
	if line := keeper.Step(); !strings.Contains(line, "running") {
		t.Fatalf("second step: %q; want the running agent named", line)
	}
	if records, _ := store.List(); len(records) != 1 {
		t.Fatalf("a second landing agent started: %d launches", len(records))
	}

	// Another checkout's steward keeps no agent.
	other := newLandingAgentKeeper(t.TempDir(), home, agent)
	other.Sources.Validation = keeper.Sources.Validation
	if _, err := store.Update(record.ID, func(r *launch.Record) error { r.State = launch.Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	other.Step()
	if records, _ := store.List(); len(records) != 1 {
		t.Fatalf("another checkout's steward started a landing agent: %d launches", len(records))
	}

	// The agent ended on a provider limit: the reap records the outage at
	// the lane installation, and the outage holds the next start.
	dir, _ := store.StateDir(record.ID)
	if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte(`{"is_error":true,"result":"API Error: 529 overloaded_error"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	line = keeper.Step()
	if _, standing := outage.StandingAt(module, now); !standing || !strings.Contains(line, "provider") {
		t.Fatalf("after a provider-limited end: line %q, outage standing %t; want the start held", line, standing)
	}
	if records, _ := store.List(); len(records) != 1 {
		t.Fatalf("a landing agent started into a provider outage: %d launches", len(records))
	}

	// A start the launcher refuses leaves no brief behind (critique F-2).
	briefs := func() int {
		matches, _ := filepath.Glob(filepath.Join(module, "artifacts", "agents", "landing-agent", "*.brief.md"))
		return len(matches)
	}
	before := briefs()
	refused := agent
	refused.nonce = func() (string, error) { return "8899aabbccddeeff", nil }
	manager.Lane = func() (launch.LaneCheckout, error) { return launch.LaneCheckout{}, nil }
	if _, err := refused.start(checkout, lane.Wake{Reasons: []string{lane.WakeQueued}}); err == nil || briefs() != before {
		t.Fatalf("a refused start = %v, briefs %d -> %d; want refused and no brief left", err, before, briefs())
	}
}

func resolvedPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}

// TestNoOwnerStartsWhileTheAgentStarts (A-a, re-review 1): the read every
// batch owner start makes names a landing agent whose start the keeper has
// claimed and not finished, as it names one that runs.
func TestNoOwnerStartsWhileTheAgentStarts(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	home := filepath.Join(base, "home")
	if err := os.MkdirAll(lane.HostDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	manager := &launch.Manager{Store: launch.Store{Root: filepath.Join(base, "launches")}}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, now: func() time.Time { return now }}
	live := agent.liveOrStarting(func() (string, error) { return home, nil })
	if _, running, err := live(); err != nil || running {
		t.Fatalf("no agent: running=%t err=%v", running, err)
	}
	claim := `{"startingAt":"` + now.Add(-time.Minute).Format(time.RFC3339) + `"}`
	if err := os.WriteFile(filepath.Join(lane.HostDir(home), "landing-agent-keeper.json"), []byte(claim), 0o600); err != nil {
		t.Fatal(err)
	}
	if id, running, err := live(); err != nil || !running || !strings.Contains(id, "starting") || strings.Contains(id, "T1") {
		t.Fatalf("a claimed start: %q running=%t err=%v; want it named, with no raw stamp", id, running, err)
	}
	// Unknown holds: an unreadable keeper record, or no lane home.
	if err := os.WriteFile(filepath.Join(lane.HostDir(home), "landing-agent-keeper.json"), []byte("{torn"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := live(); err == nil || !strings.Contains(err.Error(), "metasystem landing start") {
		t.Fatalf("an unreadable keeper record: err %v; want a hold naming the repair", err)
	}
	homeless := agent.liveOrStarting(func() (string, error) { return "", errors.New("no home") })
	if _, _, err := homeless(); err == nil {
		t.Fatal("no lane home read as no agent")
	}
}
