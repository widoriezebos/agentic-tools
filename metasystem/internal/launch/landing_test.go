package launch

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// nestedLane makes a landing checkout whose module root is below its top,
// the layout the lane runs in (checkout root != module root), and returns
// both.
func nestedLane(t *testing.T) (checkout, module string) {
	t.Helper()
	checkout = filepath.Join(t.TempDir(), "landing")
	module = filepath.Join(checkout, "metasystem")
	if err := os.MkdirAll(module, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return checkout, module
}

func landingBrief(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "landing-brief.md")
	if err := os.WriteFile(path, []byte("You are this computer's landing agent; metasystem landing status --json names why you were woken.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// laneManager is a manager whose host registers the nested lane checkout.
func laneManager(t *testing.T, checkout, module string) (*Manager, *fakeProcesses) {
	t.Helper()
	m, processes, _, _ := manager(t)
	m.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	m.Supervisor = supervisingStarter(t, m)
	settings := seatOn()
	settings.LandingRuntime, settings.LandingModel, settings.LandingEffort = "claude", "claude-opus-5-5", "xhigh"
	m.Settings = settings
	m.Lane = func() (LaneCheckout, error) {
		return LaneCheckout{Registered: true, Checkout: checkout, Module: module}, nil
	}
	return m, processes
}

// TestSeatStillRefusedOnLane (A-a; Amendment 1 kept): the registered lane
// checkout starts the landing kind and never a seat, whatever its settings
// (the seat is on here). The guard is the launcher's own, so no caller of
// Start can get past it: a seat in the lane's checkout, or bound to its
// module's fence, is refused and leaves no record; the landing kind there is
// admitted and runs under the landing-agent lineage bound to the lane's
// fence; the landing kind anywhere else is refused, as is either kind when
// the lane cannot be read.
func TestSeatStillRefusedOnLane(t *testing.T) {
	t.Parallel()
	checkout, module := nestedLane(t)
	m, processes := laneManager(t, checkout, module)
	for _, spec := range []StartSpec{
		{Kind: "seat", WorkingDirectory: checkout, FenceRoot: module, Brief: seatBrief(t), Tag: "n0nce"},
		{Kind: "seat", WorkingDirectory: checkout + string(filepath.Separator), FenceRoot: checkout, Brief: seatBrief(t), Tag: "n0nce"},
		{Kind: "seat", WorkingDirectory: t.TempDir(), FenceRoot: module, Brief: seatBrief(t), Tag: "n0nce"},
	} {
		_, err := m.Start(spec)
		if err == nil || !strings.Contains(err.Error(), "landing lane never starts a seat") {
			t.Fatalf("a seat in the lane checkout (%s, fence %s) = %v; want refused", spec.WorkingDirectory, spec.FenceRoot, err)
		}
	}
	if records, _ := m.Store.List(); len(records) != 0 || processes.command.Program != "" {
		t.Fatalf("a refused seat left records %+v or a command %+v", records, processes.command)
	}

	record, err := m.Start(StartSpec{Kind: "landing", WorkingDirectory: checkout, FenceRoot: module, Brief: landingBrief(t), Tag: "w4ke"})
	if err != nil {
		t.Fatalf("the landing kind on the lane checkout was refused: %v", err)
	}
	if record.Kind != "landing" || record.Adapter != "claude-headless" || record.Child == nil || readString(record.AdapterData, "fenceRoot") != module {
		t.Fatalf("landing record = %+v", record)
	}
	absCheckout, _ := filepath.Abs(checkout)
	if processes.command.Directory != absCheckout {
		t.Fatalf("the landing agent runs in %s, want the lane checkout %s", processes.command.Directory, absCheckout)
	}
	for key, want := range map[string]string{"METASYSTEM_OWNER_LINEAGE": LandingOwnerLineage, "METASYSTEM_DELEGATE_ROOT": "", "METASYSTEM_SESSION_ID": ""} {
		if got, set := environmentValue(processes.command.Environment, key); !set || got != want {
			t.Fatalf("landing environment %s = %q set=%t, want %q", key, got, set, want)
		}
	}
	if LandingOwnerLineage != "landing-agent" {
		t.Fatalf("landing lineage = %q", LandingOwnerLineage)
	}

	// The landing kind anywhere but the registered lane checkout is refused.
	elsewhere := t.TempDir()
	if _, err := m.Start(StartSpec{Kind: "landing", WorkingDirectory: elsewhere, FenceRoot: elsewhere, Brief: landingBrief(t), Tag: "w4ke"}); err == nil ||
		!strings.Contains(err.Error(), "only on this computer's landing lane") {
		t.Fatalf("the landing kind outside the lane = %v; want refused", err)
	}
	// Nothing registered: the landing kind has nowhere to run; a seat elsewhere starts.
	none, _ := laneManager(t, checkout, module)
	none.Lane = func() (LaneCheckout, error) { return LaneCheckout{}, nil }
	if _, err := none.Start(StartSpec{Kind: "landing", WorkingDirectory: checkout, FenceRoot: module, Brief: landingBrief(t), Tag: "w4ke"}); err == nil {
		t.Fatal("the landing kind started with no lane registered")
	}
	if _, err := none.Start(StartSpec{Kind: "seat", WorkingDirectory: checkout, FenceRoot: module, Brief: seatBrief(t), Tag: "n0nce"}); err != nil {
		t.Fatalf("a seat with no lane registered was refused: %v", err)
	}
	// An unreadable lane record fails closed for both kinds.
	unreadable, _ := laneManager(t, checkout, module)
	unreadable.Lane = func() (LaneCheckout, error) { return LaneCheckout{}, errors.New("landing-lane.json is unreadable") }
	for _, kind := range []string{"seat", "landing"} {
		if _, err := unreadable.Start(StartSpec{Kind: kind, WorkingDirectory: t.TempDir(), FenceRoot: module, Brief: seatBrief(t), Tag: "n0nce"}); err == nil ||
			!strings.Contains(err.Error(), "unreadable") {
			t.Fatalf("a %s start with an unreadable lane = %v; want refused", kind, err)
		}
	}
	// No lane reader at all is no answer: the landing kind is refused.
	unwired, _ := laneManager(t, checkout, module)
	unwired.Lane = nil
	if _, err := unwired.Start(StartSpec{Kind: "landing", WorkingDirectory: checkout, FenceRoot: module, Brief: landingBrief(t), Tag: "w4ke"}); err == nil {
		t.Fatal("the landing kind started with no lane reader")
	}
}

// TestLandingLaunchOneAtATime (A-a, D1): one landing agent per computer. A
// landing launch that has not ended refuses the next; once it ended the next
// is admitted. The landing kind binds to the lane's process-creation fence
// as a seat does: a closed fence refuses it.
func TestLandingLaunchOneAtATime(t *testing.T) {
	t.Parallel()
	checkout, module := nestedLane(t)
	m, _ := laneManager(t, checkout, module)
	running := seed(t, m, "landing-running", Running)
	if _, err := m.Store.Update(running.ID, func(r *Record) error { r.Kind = "landing"; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(StartSpec{Kind: "landing", WorkingDirectory: checkout, FenceRoot: module, Brief: landingBrief(t), Tag: "w4ke"}); err == nil ||
		!strings.Contains(err.Error(), "landing-running") {
		t.Fatalf("a second landing agent = %v; want refused naming the running one", err)
	}
	if _, err := m.Store.Update(running.ID, func(r *Record) error { r.State = Completed; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(StartSpec{Kind: "landing", WorkingDirectory: checkout, FenceRoot: module, Brief: landingBrief(t), Tag: "w4ke"}); err != nil {
		t.Fatalf("a landing agent after the last one ended was refused: %v", err)
	}

	closedCheckout, closedModule := nestedLane(t)
	closed := writeFence(t, closedModule, stopfence.StateClosed, stopfence.PhaseStopped, 2)
	want, _ := stopfence.ClosedDescription(closed, closedModule)
	fenced, _ := laneManager(t, closedCheckout, closedModule)
	if _, err := fenced.Start(StartSpec{Kind: "landing", WorkingDirectory: closedCheckout, FenceRoot: closedModule, Brief: landingBrief(t), Tag: "w4ke"}); err == nil || err.Error() != want {
		t.Fatalf("a landing agent under a closed fence = %v; want %q", err, want)
	}
}

// TestLandingSettingsAreRosterKeys (A-a, D2): launch.landing.runtime, .model
// and .effort resolve as a lane's from the layered settings, with the
// documented default: the first runtime on PATH, the build lane's model for
// it, effort xhigh; metasystem.conf.local names them.
func TestLandingSettingsAreRosterKeys(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, name := range []string{"codex", "claude"} {
		if err := testexec.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	lookup := func(key string) (string, bool) {
		if key == "PATH" {
			return dir, true
		}
		return "", false
	}
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("# overrides only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := ResolveSettings(conf, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if settings.LandingRuntime != "claude" || settings.LandingModel != settings.BuildModel || settings.LandingEffort != "xhigh" {
		t.Fatalf("default landing settings: runtime=%q model=%q effort=%q (build model %q)", settings.LandingRuntime, settings.LandingModel, settings.LandingEffort, settings.BuildModel)
	}
	if model, effort, window := settings.launchValues("landing"); model != settings.LandingModel || effort != "xhigh" || window != 0 {
		t.Fatalf("launchValues(landing) = %q %q %d", model, effort, window)
	}
	sources := map[string]string{}
	for _, value := range settings.Values {
		sources[value.Key] = value.Source
	}
	if sources[LandingModelKey] != "default via "+BuildModelKey+".claude" {
		t.Fatalf("landing model source = %q", sources[LandingModelKey])
	}
	if err := os.WriteFile(conf+".local", []byte(LandingRuntimeKey+"=codex\n"+LandingModelKey+"=gpt-6-sol\n"+LandingEffortKey+"=high\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err = ResolveSettings(conf, lookup)
	if err != nil {
		t.Fatal(err)
	}
	if settings.LandingRuntime != "codex" || settings.LandingModel != "gpt-6-sol" || settings.LandingEffort != "high" ||
		adapterForLane("landing", settings.launchRuntime("landing")) != "codex-exec" {
		t.Fatalf("roster landing settings = %q %q %q", settings.LandingRuntime, settings.LandingModel, settings.LandingEffort)
	}
}
