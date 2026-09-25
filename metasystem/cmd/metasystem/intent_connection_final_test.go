package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	dispatchcore "github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// TestIntentCriticDelegateSelectedRoster (CONN-R2-F1): a critic dispatched
// from a generated goal worktree resolves the selected installation's
// configured code-critic, although the worktree's tracked roster is the
// template and it has no local overlay. The real delegate entrypoint
// (readDelegate) runs a fake delegate executable that records the process
// environment it receives; the real roster owner that dispatch's
// `job resolve-roster` calls then resolves the worktree's configuration
// under exactly that environment. The selected pair is the roster default,
// not an override (no escalation); an explicit model still escalates by the
// unchanged policy; without the selected installation the worktree refuses.
// Declared fakes: the delegate executable (no dispatch.sh, lease, brain or
// model), and the synthetic configuration files.
func TestIntentCriticDelegateSelectedRoster(t *testing.T) {
	selected, worktree := t.TempDir(), t.TempDir()
	tracked := "metasystem.runtimes=claude,codex\nrole.default.runtime=codex\nrole.code-critic.runtime=<runtime>\nrole.code-critic.model.<runtime>=<model>\n"
	for _, dir := range []string{selected, worktree} {
		if err := os.WriteFile(filepath.Join(dir, "metasystem.conf"), []byte(tracked), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	// Synthetic local overlay of the selected checkout only.
	os.WriteFile(filepath.Join(selected, "metasystem.conf.local"), []byte("role.code-critic.runtime=claude\nrole.code-critic.model.claude=fixture-critic-model\nmode.review.role.code-critic.model.claude=review-mode-critic\n"), 0o600)
	brief := filepath.Join(worktree, "brief.md")
	os.WriteFile(brief, []byte("# Read\n\nWorking Mode: review\n"), 0o600)
	other := filepath.Join(worktree, "other-brief.md")
	os.WriteFile(other, []byte("# Read\n\nWorking Mode: implementation\n"), 0o600)
	modeless := filepath.Join(worktree, "modeless-brief.md")
	os.WriteFile(modeless, []byte("# Read\n"), 0o600)
	if _, err := criticDelegateEnvironment(selected, worktree, modeless); err == nil {
		t.Fatal("a brief without a working mode must be refused, not resolved as the base mode")
	}
	if base, err := criticDelegateEnvironment(selected, worktree, other); err != nil || !slices.Contains(base, "METASYSTEM_ROLE_CODE_CRITIC_MODEL_CLAUDE=fixture-critic-model") {
		t.Fatalf("a mode without its own critic carries the base critic: %v %v", base, err)
	}

	if _, err := dispatchcore.ResolveRoster(dispatchcore.RosterParams{ConfPath: filepath.Join(worktree, "metasystem.conf"), Role: "code-critic"}); err == nil {
		t.Fatal("the generated worktree's template roster must not resolve on its own")
	}
	environment, err := criticDelegateEnvironment(selected, worktree, brief)
	if err != nil {
		t.Fatal(err)
	}
	record := filepath.Join(t.TempDir(), "delegate-environment")
	binary := filepath.Join(t.TempDir(), "metasystem")
	script := "#!/usr/bin/env bash\nset -euo pipefail\nenv >\"$DELEGATE_RECORD\"\nprintf '%s\\n' \"$*\" >>\"$DELEGATE_RECORD.args\"\nprintf '{\"outcome\":\"WON\",\"headline\":\"started\",\"jobId\":\"critic-fixture\"}\\n'\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DELEGATE_RECORD", record)
	if job, err := readDelegate(binary, worktree, brief, "goal-a", strings.Repeat("a", 40), "", "", environment...); err != nil || job != "critic-fixture" {
		t.Fatalf("delegate: job=%q err=%v", job, err)
	}
	args, _ := os.ReadFile(record + ".args")
	if strings.Contains(string(args), "--runtime") || strings.Contains(string(args), "--model") {
		t.Fatalf("the selected critic must not be passed as an override: %s", args)
	}
	seen, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(seen), "\n")
	if len(environment) != 4 || !slices.Contains(environment, "METASYSTEM_RUNTIME_CLAUDE_MAXIMAL_MODELS=") {
		t.Fatalf("exactly the resolved roster values are carried: %v", environment)
	}
	for _, entry := range environment {
		if !slices.Contains(lines, entry) {
			t.Fatalf("the delegate process did not receive %s", entry)
		}
		name, value, _ := strings.Cut(entry, "=")
		t.Setenv(name, value)
	}
	// The selected installation names no maximal model, so its critic is
	// refused for a design-bearing read even where the worktree would admit it.
	os.WriteFile(filepath.Join(worktree, "metasystem.conf.local"), []byte("runtime.claude.maximal-models=review-mode-critic\n"), 0o600)
	if err := dispatchcore.ValidateRuntimeHazardConfiguration(worktree, "claude", "review-mode-critic", dispatchcore.HazardDesignBearing); err == nil {
		t.Fatal("a selected critic the selected installation does not authorize as maximal must be refused")
	}
	os.Remove(filepath.Join(worktree, "metasystem.conf.local"))
	if !slices.Contains(lines, "METASYSTEM_DELEGATE_ROOT="+worktree) {
		t.Fatal("records and workspace stay the worktree's")
	}
	conf := filepath.Join(worktree, "metasystem.conf")
	resolution, err := dispatchcore.ResolveRoster(dispatchcore.RosterParams{ConfPath: conf, Role: "code-critic", Mode: "review"})
	if err != nil || resolution.Runtime != "claude" || resolution.Model != "review-mode-critic" || resolution.EscalationRequired || resolution.Overridden {
		t.Fatalf("the delegate must resolve the selected critic as its default: %+v %v", resolution, err)
	}
	explicit, err := dispatchcore.ResolveRoster(dispatchcore.RosterParams{ConfPath: conf, Role: "code-critic", Mode: "review", ModelOverride: "another-model"})
	if err == nil && !explicit.EscalationRequired {
		t.Fatalf("an explicit model must still escalate: %+v", explicit)
	}
	// A selected installation whose roster does not resolve is refused.
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		os.Unsetenv(name) // t.Setenv above restores the test's environment
	}
	os.Remove(filepath.Join(selected, "metasystem.conf.local"))
	if _, err := criticDelegateEnvironment(selected, worktree, brief); err == nil {
		t.Fatal("an unresolvable selected roster must be refused, not dropped")
	}
}

// TestIntentGoalWorktreeIsolationRetry (CONN-R2-F2): the adapters' declared
// local settings are completed on every worktree handed to a build. A
// preparation whose isolation fails once keeps the worktree and the repeat
// completes it; a truncated earlier copy is refused with its recovery; a
// staging entry of an interrupted copy is replaced; a user's own edit is
// kept. The manifest comes from the real claude.sh entrypoint and the copy
// from the real session-isolation owner.
func TestIntentGoalWorktreeIsolationRetry(t *testing.T) {
	c := newConnectionBed(t)
	settings := c.adapterFixture()
	owners := c.connectionOwners()
	layout, err := owners.resolver.ResolveLayout(c.root())
	if err != nil {
		t.Fatal(err)
	}
	real := (&intentInvocation{owners: owners, layout: layout, cwd: c.root()}).isolateAdapterConfiguration
	calls := 0
	owners.connection.isolate = func(source, destination string) error {
		calls++
		if calls == 1 {
			return errors.New("fixture: the copy failed once")
		}
		return real(source, destination)
	}
	local := filepath.Join(c.worktree, ".claude", "settings.local.json")
	if path, result := c.prepare(owners); path != "" || result == nil || !strings.Contains(result.Summary, "is kept") {
		t.Fatalf("first preparation: %q %+v", path, result)
	}
	if _, err := os.Stat(local); !os.IsNotExist(err) {
		t.Fatal("the failed copy wrote the settings")
	}
	read := func() string { data, _ := os.ReadFile(local); return string(data) }
	if path, result := c.prepare(owners); result != nil || path != c.worktree || read() != settings || calls != 2 {
		t.Fatalf("retry must complete the settings: %q %+v calls=%d settings=%q", path, result, calls, read())
	}
	// A truncated earlier copy is neither kept as complete nor overwritten.
	partial := settings[:len(settings)/2]
	os.WriteFile(local, []byte(partial), 0o600)
	if path, result := c.prepare(owners); path != "" || result == nil || !strings.Contains(result.Summary, "interrupted copy") || !strings.Contains(result.Summary, "remove") || read() != partial {
		t.Fatalf("partial copy: %q %+v", path, result)
	}
	os.Remove(local)
	// An interrupted staged copy is this owner's own and is replaced.
	os.WriteFile(local+".metasystem-isolation-staging", []byte("half"), 0o600)
	if path, result := c.prepare(owners); result != nil || path == "" || read() != settings {
		t.Fatalf("staged copy: %q %+v", path, result)
	}
	if _, err := os.Stat(local + ".metasystem-isolation-staging"); !os.IsNotExist(err) {
		t.Fatal("the staging entry outlived the completed copy")
	}
	// The user's own settings stay theirs.
	os.WriteFile(local, []byte(`{"mine":true}`), 0o600)
	if path, result := c.prepare(owners); result != nil || path == "" || read() != `{"mine":true}` {
		t.Fatalf("user file: %q %+v", path, result)
	}
}

// connectionRuntimeBuilder hosts the real launch supervisor for the build
// (Manager.Supervise with OSProcesses and ClaudeHeadless) synchronously in
// the test; proof and read stay the connection bed's declared fakes.
type connectionRuntimeBuilder struct {
	c         *connectionBed
	built     int
	deadChild identity.Ref
	build     launch.Record
}

func (s *connectionRuntimeBuilder) StartSupervisor(id, stateDir string) (identity.Ref, error) {
	record, err := s.c.manager.Store.Read(id)
	if err != nil {
		return identity.Ref{}, err
	}
	if record.Kind == "build" {
		s.built++
		result, err := s.c.manager.Supervise(id)
		s.build = result
		if err != nil {
			return identity.Ref{}, err
		}
		if result.Child == nil || result.Supervisor == nil {
			return identity.Ref{}, fmt.Errorf("the real builder recorded no child and supervisor")
		}
		s.deadChild = *result.Child
		return *result.Supervisor, nil
	}
	if s.deadChild.Pid == 0 {
		return identity.Ref{}, fmt.Errorf("%s preceded the builder", record.Kind)
	}
	_, err = s.c.StartSupervisor(id, stateDir)
	return s.deadChild, err
}

// connectionReleasedProcesses starts the builder as a real OS child and
// releases its held exit only when the supervisor joins it, after the exact
// identity was probed and Running recorded.
type connectionReleasedProcesses struct {
	launch.OSProcesses
	release func() error
}

func (p connectionReleasedProcesses) StartChild(spec launch.Command) (launch.Child, identity.Ref, error) {
	child, ref, err := p.OSProcesses.StartChild(spec)
	if err != nil {
		return child, ref, err
	}
	return connectionReleasedChild{Child: child, release: p.release}, ref, nil
}

type connectionReleasedChild struct {
	launch.Child
	release func() error
}

func (c connectionReleasedChild) Wait() (int, error) {
	if err := c.release(); err != nil {
		return 1, err
	}
	return c.Child.Wait()
}

// TestIntentBuilderChildInGeneratedWorktree (VMI-CONN-05, CONN-R2-F2): the
// public build's first preparation fails its settings copy once; the
// repeated public build completes the settings and launches the real
// ClaudeHeadless builder command as an OS child (a fake absolute executable,
// no model) in the generated worktree. The child itself checks its cwd, the
// declared settings bytes, the absent local configuration, its argv and
// brief, writes the result bytes and a fixture transcript under an external
// projects root. A repeat reuses the run and starts no second child.
func TestIntentBuilderChildInGeneratedWorktree(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for the fixture builder child")
	}
	c := newConnectionBed(t)
	settings := c.adapterFixture()
	evidence, projects := t.TempDir(), t.TempDir()
	child := filepath.Join(t.TempDir(), "claude")
	body := "#!" + python + `
import json, os, pathlib, sys
cwd = pathlib.Path.cwd().resolve()
assert cwd == pathlib.Path(os.environ["VMI_PROBE_WORKTREE"]).resolve(), cwd
settings = (cwd / ".claude/settings.local.json").read_text()
assert settings == os.environ["VMI_PROBE_SETTINGS"]
assert not (cwd / "metasystem.conf.local").exists()
args = sys.argv[1:]
assert args[:3] == ["-p", "--model", os.environ["VMI_PROBE_MODEL"]], args
assert "--dangerously-skip-permissions" in args
assert args[args.index("--output-format") + 1] == "json"
assert args[args.index("--name") + 1].startswith("build-")
assert os.environ["CLAUDE_CODE_AUTO_COMPACT_WINDOW"] == "12345"
brief = sys.stdin.read()
assert "Build through a real adapter child." in brief
evidence = pathlib.Path(os.environ["VMI_PROBE_EVIDENCE"])
with (evidence / "child-invocations.jsonl").open("a") as f:
    f.write(json.dumps({"pid": os.getpid(), "cwd": str(cwd), "argv": args, "settings": settings}) + "\n")
with (cwd / "builder-result.txt").open("x") as f:
    f.write("produced by the actual fixture builder child\n")
session = "vmi-conn-05-fixture"
projects = pathlib.Path(os.environ["VMI_PROBE_PROJECTS"])
row = {"type": "assistant", "message": {"id": "fixture-answer-1", "content": [{"type": "text"}], "usage": {"input_tokens": 7, "output_tokens": 3}}}
(projects / (session + ".jsonl")).write_text(json.dumps(row) + "\n")
print(json.dumps({"session_id": session, "is_error": False, "num_turns": 1, "result": "Fixture build completed."}), flush=True)
with open(os.environ["VMI_PROBE_RELEASE"]) as gate:
    assert gate.readline() == "release\n"
`
	if err := testexec.WriteFile(child, []byte(body), 0o700); err != nil {
		t.Fatal(err)
	}
	// The child holds its exit on a release FIFO. The test keeps it open for
	// reading and writing so the release never races the child's open, and
	// the release is sent once, at the supervisor's join or at cleanup.
	gate := filepath.Join(t.TempDir(), "release")
	makeFixtureFIFO(t, gate)
	held, err := os.OpenFile(gate, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	var releaseOnce sync.Once
	var releaseErr error
	release := func() error {
		releaseOnce.Do(func() { releaseErr = writeFixtureEvent(context.Background(), gate) })
		return releaseErr
	}
	t.Cleanup(func() {
		_ = release()
		held.Close()
	})
	m := c.manager
	prober := identity.KernelProber{}
	processes := launch.OSProcesses{Prober: prober}
	m.Processes, m.Signaler, m.Prober = connectionReleasedProcesses{OSProcesses: processes, release: release}, processes, prober
	m.Grace, m.Poll, m.StartCap = 100*time.Millisecond, 5*time.Millisecond, 5*time.Second
	m.Settings.WaitCapSeconds = 5
	m.Settings.BuildWindow = 12345
	m.Adapters["claude-headless"] = launch.ClaudeHeadless{Binary: child, ProjectsRoot: projects}
	starter := &connectionRuntimeBuilder{c: c}
	m.Supervisor = starter
	for key, value := range map[string]string{"VMI_PROBE_WORKTREE": c.worktree, "VMI_PROBE_SETTINGS": settings,
		"VMI_PROBE_MODEL": m.Settings.BuildModel, "VMI_PROBE_EVIDENCE": evidence, "VMI_PROBE_PROJECTS": projects, "VMI_PROBE_RELEASE": gate} {
		t.Setenv(key, value)
	}
	failOnce := true
	c.isolate = func(real func(string, string) error, source, destination string) error {
		if failOnce {
			failOnce = false
			return errors.New("fixture: the settings copy failed once")
		}
		return real(source, destination)
	}
	base := c.endpointTip()
	brief := c.brief("runtime-child-brief.md", "Build through a real adapter child.\n")
	args := append([]string{"build", c.id, "runtime-child", "--brief", brief, "--lines", "10"}, workCheck...)
	if code, result := c.do(args...); result.Outcome != intentRefused || starter.built != 0 {
		t.Fatalf("a failed settings copy must launch nothing: code=%d %+v", code, result)
	}
	code, result := c.do(args...)
	if code != 0 || result.Outcome != intentConfirmed {
		t.Fatalf("build: code=%d %+v", code, result)
	}
	run := resultData(t, result)["run"].(string)
	build := starter.build
	if starter.built != 1 || build.State != launch.Completed || build.ExitCode == nil || *build.ExitCode != 0 || build.Child == nil ||
		!build.Measured || build.Adapter != "claude-headless" || build.WorkingDirectory != c.worktree {
		t.Fatalf("real builder record: built=%d %+v", starter.built, build)
	}
	invocations, _ := os.ReadFile(filepath.Join(evidence, "child-invocations.jsonl"))
	lines := strings.Split(strings.TrimSpace(string(invocations)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], fmt.Sprintf(`"pid": %d`, build.Child.Pid)) || build.Child.Pid == int64(os.Getpid()) {
		t.Fatalf("one child observation with the launch's pid %d: %q", build.Child.Pid, invocations)
	}
	if data, err := os.ReadFile(filepath.Join(c.worktree, "builder-result.txt")); err != nil || string(data) != "produced by the actual fixture builder child\n" {
		t.Fatalf("built bytes: %q %v", data, err)
	}
	record := c.runRecord(run)
	if record.Rounds[0].Steps[0].LaunchID != build.ID || !strings.Contains(record.Rounds[0].Outcome, "green") {
		t.Fatalf("the run's build step is the real launch: %+v", record.Rounds[0])
	}
	if connectionGit(t, c.worktree, "rev-parse", "HEAD") != base || c.endpointTip() != base {
		t.Fatal("build alone commits and publishes nothing")
	}
	if code, result = c.do(args...); code != 0 || resultData(t, result)["run"] != run || starter.built != 1 {
		t.Fatalf("repeat: code=%d %+v built=%d", code, result, starter.built)
	}
}
