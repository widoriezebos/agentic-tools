package launch

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// seatBrief writes the brief a steward stages for a seat: prose, no units
// table, since a seat is a session and not a sized build.
func seatBrief(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "seat-brief.md")
	if err := os.WriteFile(path, []byte("You are this seat's session; metasystem goal claim g-one takes the work named here.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// supervisingStarter runs the supervisor in the caller, the way the detached
// `metasystem launch supervise --id` would, over the fake process system.
func supervisingStarter(t *testing.T, m *Manager) fakeStarter {
	return fakeStarter{func(id string) {
		if _, err := m.Supervise(id); err != nil {
			t.Logf("supervise %s: %v", id, err)
		}
	}}
}

func environmentValue(environment []string, key string) (string, bool) {
	for _, entry := range environment {
		if name, value, found := strings.Cut(entry, "="); found && name == key {
			return value, true
		}
	}
	return "", false
}

// TestSeatLaunchIsSessionShapedAndNamesItsLineage (D-seat, SW-13): the seat
// kind is admitted without a goal and runs the lane's command in the checkout
// root with the brief on stdin, no --settings and no --json-schema; its
// environment names the seat lineage, empties the delegate root and the
// session id, and inherits the rest; the record carries the supervisor, the
// child and the process group.
func TestSeatLaunchIsSessionShapedAndNamesItsLineage(t *testing.T) {
	t.Parallel()
	m, processes, _, _ := manager(t)
	m.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	m.Supervisor = supervisingStarter(t, m)
	m.Settings = seatOn()
	checkout := t.TempDir()
	briefPath := seatBrief(t)
	record, err := m.Start(StartSpec{Kind: "seat", WorkingDirectory: checkout, FenceRoot: checkout, Brief: briefPath, Tag: "n0nce"})
	if err != nil {
		t.Fatalf("a seat launch without a goal was refused: %v", err)
	}
	if record.Kind != "seat" || record.Goal != "" || record.Adapter != "claude-headless" ||
		record.Supervisor == nil || record.Child == nil || record.ProcessGroup == nil {
		t.Fatalf("seat record = %+v", record)
	}
	command := processes.command
	absCheckout, _ := filepath.Abs(checkout)
	wantBrief, _ := os.ReadFile(briefPath)
	if command.Program != "/fixture/bin/claude" || command.Directory != absCheckout || command.Stdin != string(wantBrief) {
		t.Fatalf("seat command = %+v", command)
	}
	for _, refused := range []string{"--settings", "--json-schema"} {
		if slices.Contains(command.Args, refused) {
			t.Fatalf("seat command carries %s: %q", refused, command.Args)
		}
	}
	if !slices.Contains(command.Args, "--name") || command.Args[slices.Index(command.Args, "--name")+1] != "seat-n0nce" ||
		command.Args[0] != "-p" || !slices.Contains(command.Args, "--dangerously-skip-permissions") {
		t.Fatalf("seat args = %q", command.Args)
	}
	for key, want := range map[string]string{
		"METASYSTEM_OWNER_LINEAGE": SeatOwnerLineage,
		"METASYSTEM_DELEGATE_ROOT": "",
		"METASYSTEM_SESSION_ID":    "",
	} {
		if got, set := environmentValue(command.Environment, key); !set || got != want {
			t.Fatalf("seat environment %s = %q set=%t, want %q; environment=%q", key, got, set, want, command.Environment)
		}
	}
	if SeatOwnerLineage != "steward-seat" {
		t.Fatalf("seat lineage = %q", SeatOwnerLineage)
	}
	parent := []string{"PATH=/fixture/bin", "HOME=/fixture/home", "METASYSTEM_OWNER_LINEAGE=a-person",
		"METASYSTEM_DELEGATE_ROOT=/fixture/delegate", "METASYSTEM_SESSION_ID=person-session"}
	child := childEnvironment(parent, command.Environment)
	for key, want := range map[string]string{"PATH": "/fixture/bin", "HOME": "/fixture/home",
		"METASYSTEM_OWNER_LINEAGE": SeatOwnerLineage, "METASYSTEM_DELEGATE_ROOT": "", "METASYSTEM_SESSION_ID": ""} {
		if got, set := environmentValue(child, key); !set || got != want {
			t.Fatalf("child environment %s = %q set=%t, want %q; environment=%q", key, got, set, want, child)
		}
	}
	for _, key := range []string{"METASYSTEM_OWNER_LINEAGE", "METASYSTEM_DELEGATE_ROOT", "METASYSTEM_SESSION_ID"} {
		count := 0
		for _, entry := range child {
			if strings.HasPrefix(entry, key+"=") {
				count++
			}
		}
		if count != 1 {
			t.Fatalf("child environment carries %s %d times: %q", key, count, child)
		}
	}

	// A build launch keeps the launcher's lineage: only the seat kind names one.
	m2, processes2, _, _ := manager(t)
	m2.Adapters["claude-headless"] = ClaudeHeadless{Binary: "/fixture/bin/claude"}
	m2.Supervisor = supervisingStarter(t, m2)
	if _, err := m2.Start(StartSpec{ID: "build-env", Kind: "build", Brief: brief(t), WorkingDirectory: t.TempDir()}); err != nil {
		t.Fatal(err)
	}
	if _, set := environmentValue(processes2.command.Environment, "METASYSTEM_OWNER_LINEAGE"); set {
		t.Fatalf("a build launch named a lineage: %q", processes2.command.Environment)
	}
}

// TestSeatLaunchSettingsResolveLikeALane (D-seat): launch.seat.runtime,
// launch.seat.model and launch.seat.effort resolve as launch.build.* do: auto
// takes the first listed runtime on PATH, the model follows the resolved
// runtime unless the runtime-independent key pins one, every layer's source
// is named; an unknown runtime refuses at the start.
func TestSeatLaunchSettingsResolveLikeALane(t *testing.T) {
	t.Parallel()
	onPath := func(names ...string) func(string) (string, bool) {
		dir := t.TempDir()
		for _, name := range names {
			if err := testexec.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		return func(key string) (string, bool) {
			if key == "PATH" {
				return dir, true
			}
			return "", false
		}
	}
	// A seat is opt-in (Amendment 1): this installation turned it on as auto.
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte(SeatRuntimeKey+"=auto\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, runtime, model string
		lookup               func(string) (string, bool)
	}{
		{"claude first", "claude", "claude-opus-5-5", onPath("devin", "codex", "claude")},
		{"codex before devin", "codex", "gpt-6-sol", onPath("devin", "codex")},
	} {
		settings, err := ResolveSettings(conf, test.lookup)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		if settings.SeatRuntime != test.runtime || settings.SeatRuntime != settings.BuildRuntime ||
			settings.SeatModel != test.model || settings.SeatModel != settings.BuildModel || settings.SeatEffort != "xhigh" {
			t.Fatalf("%s: seat runtime=%q model=%q effort=%q build runtime=%q model=%q", test.name,
				settings.SeatRuntime, settings.SeatModel, settings.SeatEffort, settings.BuildRuntime, settings.BuildModel)
		}
		if runtime := settings.launchRuntime("seat"); runtime != test.runtime {
			t.Fatalf("%s: launchRuntime(seat) = %q", test.name, runtime)
		}
		if model, effort, window := settings.launchValues("seat"); model != test.model || effort != "xhigh" || window != settings.SeatWindow {
			t.Fatalf("%s: launchValues(seat) = %q %q %d", test.name, model, effort, window)
		}
		sources := map[string]string{}
		for _, value := range settings.Values {
			sources[value.Key] = value.Source
		}
		if !strings.HasSuffix(sources[SeatRuntimeKey], "; auto: first of claude,codex,devin on PATH") ||
			sources[SeatModelKey] != "default via "+BuildModelKey+"."+test.runtime || sources[SeatEffortKey] != "default" {
			t.Fatalf("%s: sources=%v", test.name, sources)
		}
	}
	if err := os.WriteFile(conf, []byte(SeatRuntimeKey+"=devin\n"+SeatModelKey+"=pinned-seat\n"+SeatEffortKey+"=high\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := ResolveSettings(conf, onPath("claude"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.SeatRuntime != "devin" || settings.SeatModel != "pinned-seat" || settings.SeatEffort != "high" || settings.BuildRuntime != "claude" {
		t.Fatalf("pinned seat settings = %+v", settings)
	}
	if adapterForLane("seat", settings.launchRuntime("seat")) != "devin-print" {
		t.Fatalf("a seat on devin names adapter %q", adapterForLane("seat", settings.launchRuntime("seat")))
	}
	if err := os.WriteFile(conf, []byte(SeatEffortKey+"=   \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveSettings(conf, onPath("claude")); err == nil || !strings.HasPrefix(ErrorDetail(err), "LAUNCH_SETTING_INVALID key="+SeatEffortKey) {
		t.Fatalf("an empty seat effort resolved: %v", err)
	}

	m, _, _, _ := manager(t)
	m.Supervisor = supervisingStarter(t, m)
	unknown := DefaultSettings()
	unknown.SeatRuntime = "nonesuch"
	m.Settings = unknown
	if _, err := m.Start(StartSpec{Kind: "seat", WorkingDirectory: t.TempDir(), Brief: seatBrief(t), Tag: "n0nce"}); err == nil ||
		err.Error() != `launch kind "seat" is not available` {
		t.Fatalf("a seat on an unknown runtime started: %v", err)
	}
	if records, _ := m.Store.List(); len(records) != 0 {
		t.Fatalf("a refused seat left a record: %+v", records)
	}
}

func writeFence(t *testing.T, root, state, phase string, generation int64) stopfence.Record {
	t.Helper()
	record := stopfence.Record{State: state, Phase: phase, Generation: generation, ChangedAt: "2026-09-30T12:00:00Z", Checkout: root,
		By: stopfence.Actor{Verb: "stop", Process: stopfence.Process{Pid: 71, PidStartedAt: 70}}}
	if state == stopfence.StateOpen {
		record.By.Verb = "arm"
	}
	if err := stopfence.Write(root, record); err != nil {
		t.Fatal(err)
	}
	record, err := stopfence.Read(root)
	if err != nil {
		t.Fatal(err)
	}
	return record
}

// TestSeatStartDuringStopIsEnded (D-fence, SW-12): the seat supervisor holds
// a creation claim across the child's start and re-reads the fence after the
// child is recorded; a stop that closed the fence meanwhile ends the group
// and the record fails with the fence's own description. A fence already
// closed refuses the start itself and the supervisor alike.
func TestSeatStartDuringStopIsEnded(t *testing.T) {
	t.Parallel()
	m, processes, _, _ := manager(t)
	m.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	m.Supervisor = supervisingStarter(t, m)
	m.Settings = seatOn()
	processes.group = true
	// A vendored layout: the session works in the checkout, the fence is
	// the installation state root's below it.
	checkout := t.TempDir()
	stateRoot1 := filepath.Join(checkout, "metasystem")
	writeFence(t, stateRoot1, stopfence.StateOpen, stopfence.PhaseArmed, 4)
	var closedFence stopfence.Record
	claimed := 0
	processes.onStart = func(Command) {
		claims, err := stopfence.Claims(stateRoot1, 4)
		if err == nil {
			for _, claim := range claims {
				if claim.Verb == "seat-launch" && claim.Generation == 4 && claim.Creator.Pid == 10 {
					claimed++
				}
			}
		}
		closedFence = writeFence(t, stateRoot1, stopfence.StateClosed, stopfence.PhaseStopped, 5)
	}
	record, err := m.Start(StartSpec{Kind: "seat", WorkingDirectory: checkout, FenceRoot: stateRoot1, Brief: seatBrief(t), Tag: "n0nce"})
	if err != nil {
		t.Fatal(err)
	}
	if claimed != 1 {
		t.Fatalf("the child started outside one seat-launch creation claim (claims seen %d)", claimed)
	}
	want, _ := stopfence.ClosedDescription(closedFence, stateRoot1)
	record, _ = m.Store.Read(record.ID)
	if record.State != Failed || record.Reason != want || record.Child == nil || processes.group ||
		!slices.Contains(processes.signals, syscall.SIGTERM) {
		t.Fatalf("a seat started during a stop = %+v group=%t signals=%v, want failed %q", record, processes.group, processes.signals, want)
	}
	if claims, err := stopfence.Claims(stateRoot1, 5); err != nil || len(claims) != 0 {
		t.Fatalf("the seat supervisor left its creation claim: %+v %v", claims, err)
	}

	// Already closed: the start refuses with the fence's description and
	// starts no supervisor; a supervisor handed such a record starts no child.
	m2, processes2, _, _ := manager(t)
	m2.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	m2.Settings = seatOn()
	started := false
	m2.Supervisor = fakeStarter{func(string) { started = true }}
	closedCheckout := t.TempDir()
	closed := writeFence(t, closedCheckout, stopfence.StateClosed, stopfence.PhaseStopped, 2)
	want, _ = stopfence.ClosedDescription(closed, closedCheckout)
	if _, err := m2.Start(StartSpec{Kind: "seat", WorkingDirectory: closedCheckout, FenceRoot: closedCheckout, Brief: seatBrief(t), Tag: "n0nce"}); err == nil || err.Error() != want || started {
		t.Fatalf("a seat start under a closed fence = %v started=%t, want %q", err, started, want)
	}
	// The fence lives at the installation's state root, which need not be
	// the session's working directory (a vendored metasystem/ below the
	// checkout): the start names it and the fence there binds.
	vendored := t.TempDir()
	stateRoot := filepath.Join(vendored, "metasystem")
	vendoredFence := writeFence(t, stateRoot, stopfence.StateClosed, stopfence.PhaseStopped, 3)
	wantVendored, _ := stopfence.ClosedDescription(vendoredFence, stateRoot)
	if _, err := m2.Start(StartSpec{Kind: "seat", WorkingDirectory: vendored, FenceRoot: stateRoot, Brief: seatBrief(t), Tag: "n0nce"}); err == nil || err.Error() != wantVendored || started {
		t.Fatalf("a seat start under a closed state-root fence = %v started=%t, want %q", err, started, wantVendored)
	}
	seeded := seed(t, m2, "seat-closed", Starting)
	if _, err := m2.Store.Update(seeded.ID, func(r *Record) error {
		r.Kind, r.Adapter, r.WorkingDirectory = "seat", "claude-headless", closedCheckout
		setString(r.AdapterData, "brief", seatBrief(t))
		setString(r.AdapterData, "fenceRoot", closedCheckout)
		setString(r.AdapterData, "model", "claude-opus-5-5")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	got, err := m2.Supervise(seeded.ID)
	if err == nil || got.State != Failed || got.Reason != want || got.Child != nil || processes2.command.Program != "" {
		t.Fatalf("a seat supervisor under a closed fence = %+v err=%v command=%+v", got, err, processes2.command)
	}
	if claims, err := stopfence.Claims(closedCheckout, 2); err != nil || len(claims) != 0 {
		t.Fatalf("a refused seat supervisor left a creation claim: %+v %v", claims, err)
	}
}

// TestSeatStartWithoutAFenceRootIsRefused (SOL-B-01): a seat binds to the
// fence of the state root its start names; with none named it would read the
// working directory, where a vendored installation keeps no fence, and take
// a closed fence for an open one. The start refuses and leaves no record, and
// a supervisor handed a seat record naming no fence root starts no child.
func TestSeatStartWithoutAFenceRootIsRefused(t *testing.T) {
	t.Parallel()
	m, processes, _, _ := manager(t)
	m.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	m.Settings = seatOn()
	started := false
	m.Supervisor = fakeStarter{func(string) { started = true }}
	checkout := t.TempDir()
	writeFence(t, filepath.Join(checkout, "metasystem"), stopfence.StateClosed, stopfence.PhaseStopped, 2)
	if _, err := m.Start(StartSpec{Kind: "seat", WorkingDirectory: checkout, Brief: seatBrief(t), Tag: "n0nce"}); err == nil || started {
		t.Fatalf("a seat start naming no fence root = %v started=%t", err, started)
	}
	if records, _ := m.Store.List(); len(records) != 0 {
		t.Fatalf("a seat start naming no fence root left a record: %+v", records)
	}
	seeded := seed(t, m, "seat-unfenced", Starting)
	if _, err := m.Store.Update(seeded.ID, func(r *Record) error {
		r.Kind, r.Adapter, r.WorkingDirectory = "seat", "claude-headless", checkout
		setString(r.AdapterData, "brief", seatBrief(t))
		setString(r.AdapterData, "model", "claude-opus-5-5")
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if got, err := m.Supervise(seeded.ID); err == nil || got.State != Failed || got.Child != nil || processes.command.Program != "" {
		t.Fatalf("a seat supervisor naming no fence root = %+v err=%v command=%+v", got, err, processes.command)
	}
}

// seatOn is the settings of an installation that turned its seat on.
func seatOn() Settings {
	settings := DefaultSettings()
	settings.SeatRuntime, settings.SeatModel = "claude", "claude-opus-5-5"
	return settings
}

// TestSeatIsOffUntilASettingTurnsItOn (Amendment 1): launch.seat.runtime
// resolves to off by default, the lanes' settings still resolve, and a seat
// start is refused naming the key; metasystem.conf.local or the environment
// turns it on.
func TestSeatIsOffUntilASettingTurnsItOn(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := testexec.WriteFile(filepath.Join(dir, "claude"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	lookup := func(env map[string]string) func(string) (string, bool) {
		return func(key string) (string, bool) {
			if key == "PATH" {
				return dir, true
			}
			value, ok := env[key]
			return value, ok
		}
	}
	conf := filepath.Join(t.TempDir(), "metasystem.conf")
	if err := os.WriteFile(conf, []byte("# overrides only\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := ResolveSettings(conf, lookup(nil))
	if err != nil || settings.SeatRuntime != "off" || settings.BuildRuntime != "claude" {
		t.Fatalf("by default the seat is off and the lanes resolve: seat=%q build=%q %v", settings.SeatRuntime, settings.BuildRuntime, err)
	}
	m, _, _, _ := manager(t)
	m.Adapters = map[string]Adapter{"claude-headless": ClaudeHeadless{Binary: "/fixture/bin/claude"}}
	started := false
	m.Supervisor = fakeStarter{func(string) { started = true }}
	m.Settings = settings
	checkout := t.TempDir()
	if _, err := m.Start(StartSpec{Kind: "seat", WorkingDirectory: checkout, FenceRoot: checkout, Brief: seatBrief(t), Tag: "n0nce"}); err == nil ||
		!strings.Contains(err.Error(), SeatRuntimeKey+"=off") || started {
		t.Fatalf("a seat start while the seat is off = %v started=%t", err, started)
	}
	if records, _ := m.Store.List(); len(records) != 0 {
		t.Fatalf("a refused seat left a record: %+v", records)
	}
	if err := os.WriteFile(conf+".local", []byte(SeatRuntimeKey+"=claude\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if settings, err := ResolveSettings(conf, lookup(nil)); err != nil || settings.SeatRuntime != "claude" || settings.SeatModel != "claude-opus-5-5" {
		t.Fatalf("metasystem.conf.local turns the seat on: %q %q %v", settings.SeatRuntime, settings.SeatModel, err)
	}
	if err := os.Remove(conf + ".local"); err != nil {
		t.Fatal(err)
	}
	if settings, err := ResolveSettings(conf, lookup(map[string]string{"METASYSTEM_LAUNCH_SEAT_RUNTIME": "auto"})); err != nil || settings.SeatRuntime != "claude" {
		t.Fatalf("the environment turns the seat on: %q %v", settings.SeatRuntime, err)
	}
}
