package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
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
	registerLane(t, home, checkout, "a-person", time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC))
	checkout, module = resolvedPath(checkout), resolvedPath(module)
	now := time.Date(2026, 9, 30, 13, 0, 0, 0, time.UTC)
	store := launch.Store{Root: filepath.Join(base, "launches")}
	manager := &launch.Manager{Store: store, Adapters: map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: "/fixture/bin/claude"}},
		Supervisor: recordingSupervisor{store}, Now: func() time.Time { return now }, Sleep: func(time.Duration) {}, Poll: time.Second, StartCap: time.Minute,
		Lane: landingLaneCheckout(func() (string, error) { return home, nil })}
	agent := landingAgent{manager: func() *launch.Manager { return manager }, settings: installationSettings, now: func() time.Time { return now },
		nonce: func() (string, error) { return "0011223344556677", nil },
		gateSettings: func(_ launch.Store, _, _, asked string) (string, error) {
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

// TestLandingAgentWritesItsToolGateIntoTheLaunchState (integration of A-a
// and A-b): the production landing agent passes the tool gate to every
// landing launch. Its settings file is written by agentgate in the launch's
// own state directory, outside the lane checkout the agent may edit, and
// its hook runs the lane installation's engine in the lane checkout.
func TestLandingAgentWritesItsToolGateIntoTheLaunchState(t *testing.T) {
	t.Parallel()
	base := resolvedPath(t.TempDir())
	checkout := filepath.Join(base, "landing")
	module := filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	store := launch.Store{Root: filepath.Join(base, "launches")}
	agent := newLandingAgent()
	if agent.gateSettings == nil {
		t.Fatal("the production landing agent passes no tool gate, so the launcher refuses every landing session")
	}
	path, err := agent.gateSettings(store, "landing-0011223344556677", checkout, module)
	if err != nil {
		t.Fatal(err)
	}
	dir, err := store.StateDir("landing-0011223344556677")
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(path) != dir {
		t.Fatalf("the gate settings are at %s; want them in the launch's state directory %s", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	engine := filepath.Join(module, "bin", "metasystem")
	if !strings.Contains(string(data), "PreToolUse") || !strings.Contains(string(data), "'"+engine+"' internal hook claude tool") || !strings.Contains(string(data), "cd '"+checkout+"'") {
		t.Fatalf("gate settings = %s; want a PreToolUse hook running %s in %s", data, engine, checkout)
	}
	if _, err := agent.gateSettings(store, "landing-0011223344556677", checkout, module); err != nil {
		t.Fatalf("a second write over the same launch: %v", err)
	}
	if _, err := agent.gateSettings(launch.Store{Root: filepath.Join(module, "launches")}, "landing-0011223344556677", checkout, module); err == nil {
		t.Fatal("gate settings were written inside the lane checkout, where the agent may edit them")
	}
}

// The landing-agent skill (A-b) is the agent's brief: every metasystem
// landing or agent command it spells is a command this engine declares,
// with options that command takes, and a kernel verb over a batch names
// it with --batch. A brief that spells a form the engine refuses sends the
// agent into a denial or a usage error at the step it describes.
func TestLandingAgentSkillUsesTheVerbsAsDeclared(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile(filepath.Join("..", "..", "skills", "landing-agent", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	spans := strings.Split(string(data), "`")
	checked := 0
	for index := 1; index < len(spans); index += 2 {
		words := strings.Fields(spans[index])
		if len(words) < 3 || words[0] != "metasystem" || (words[1] != "landing" && words[1] != "agent") {
			continue
		}
		command, ok := findIntentAction(words[1], words[2])
		if !ok {
			t.Errorf("the skill spells %q, which this engine does not declare", spans[index])
			continue
		}
		checked++
		named := map[string]bool{}
		for _, word := range words[3:] {
			if !strings.HasPrefix(word, "--") {
				continue
			}
			name, _, _ := strings.Cut(strings.TrimPrefix(word, "--"), "=")
			named[name] = true
			if _, ok := command.lookupFlag(name); !ok {
				t.Errorf("the skill spells %q, but %s takes no --%s", spans[index], command.name, name)
			}
		}
		if slices.Contains([]string{"begin", "prove", "publish"}, words[2]) && len(named) > 0 && !named["batch"] {
			t.Errorf("the skill spells %q without the --batch it acts on", spans[index])
		}
	}
	if checked == 0 {
		t.Fatal("the skill spells no landing or agent command")
	}
}
