package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/hooks"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/hostload"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/launch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testprovider"
)

// launchedChildEnvironment launches a read through the real launcher and
// process start, from a parent whose own session is a seat, and returns the
// environment the child process saw.
func launchedChildEnvironment(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	written := filepath.Join(dir, "child.env")
	binary := filepath.Join(dir, "claude")
	script := "#!/bin/sh\nenv > '" + written + "'\n" +
		`printf '%s\n' '{"session_id":"launched-read","is_error":false,"num_turns":1,"result":"done"}'` + "\n"
	if err := testexec.WriteFile(binary, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	brief := filepath.Join(dir, "brief.md")
	if err := os.WriteFile(brief, []byte("read\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	diff := filepath.Join(dir, "change.diff")
	if err := os.WriteFile(diff, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	clock := &workClock{now: time.Date(2026, 10, 30, 10, 0, 0, 0, time.UTC)}
	manager := &launch.Manager{Store: launch.Store{Root: filepath.Join(dir, "launch")},
		Adapters:  map[string]launch.Adapter{"claude-headless": launch.ClaudeHeadless{Binary: binary}},
		Processes: &seatParentProcesses{OSProcesses: launch.OSProcesses{Prober: identity.KernelProber{}}},
		Prober:    identity.KernelProber{}, Now: clock.Now, Sleep: clock.Sleep, Grace: time.Second, Poll: time.Second}
	manager.Settings = launch.DefaultSettings()
	manager.Settings.ReadRuntime, manager.Settings.ReadModel = "claude", "fixture-read-model"
	manager.CapacityHome = testprovider.Register(t, t.TempDir())
	manager.CapacitySources.Load = func(at time.Time) hostload.Sample {
		return hostload.Sample{At: at.Format(time.RFC3339Nano), Available: true, Load1m: 0}
	}
	manager.Supervisor = supervisingInPlace{t: t, m: manager}
	if _, err := manager.Start(launch.StartSpec{ID: "launched-read", Kind: "read", Brief: brief, DiffFile: diff, WorkingDirectory: t.TempDir()}); err != nil {
		t.Fatalf("the read launch did not start: %v", err)
	}
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatalf("the launched child wrote no environment: %v", err)
	}
	environment := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		if name, value, found := strings.Cut(line, "="); found {
			environment[name] = value
		}
	}
	return environment
}

// supervisingInPlace runs the launch's supervisor in the caller, the way the
// detached `metasystem launch supervise --id` would.
type supervisingInPlace struct {
	t *testing.T
	m *launch.Manager
}

func (s supervisingInPlace) StartSupervisor(id, _ string) (identity.Ref, error) {
	record, err := s.m.Supervise(id)
	if err != nil {
		s.t.Logf("supervise %s: %v", id, err)
	}
	if record.Supervisor == nil {
		return identity.Ref{}, errors.New("the supervisor recorded no identity")
	}
	return *record.Supervisor, nil
}

// seatParentProcesses starts the real child with the launch kind a seat's
// session hands down ahead of the launcher's own entries.
type seatParentProcesses struct {
	launch.OSProcesses
}

func (p *seatParentProcesses) StartChild(spec launch.Command) (launch.Child, identity.Ref, error) {
	spec.Environment = append([]string{launch.KindEnv + "=seat"}, spec.Environment...)
	return p.OSProcesses.StartChild(spec)
}

// startHookIn runs the real start hook, with the real arming owner, in the
// given environment; it returns the arming count, the output and the
// checkout's lease and announcements before and after.
func startHookIn(t *testing.T, environment map[string]string) (int, string, map[string]string, map[string]string) {
	t.Helper()
	b := newGoalCLIBed(t, goalCLISeed{})
	attempts := 0
	r := freshCommandRepository{Repository: b.repo, attempts: &attempts, capture: func(context.Context, string) (string, error) { return "", errors.New("remote transport unavailable") }}
	inputs := freshUpFixture(t, b, freshDependencies(b, r))
	mains := func() map[string]string {
		files := map[string]string{}
		entries, _ := os.ReadDir(filepath.Join(b.root, "artifacts/agents/mains"))
		for _, entry := range entries {
			data, _ := os.ReadFile(filepath.Join(b.root, "artifacts/agents/mains", entry.Name()))
			files[entry.Name()] = string(data)
		}
		return files
	}
	before := mains()
	var output, diagnostic bytes.Buffer
	ops := &freshHookOperations{hookOwners: hookOwners{diagnostics: &diagnostic, upInputs: inputs, repositoryTop: fakeTop(b.root)}, b: b}
	invocation := hooks.Invocation{Installation: b.root, Stdin: strings.NewReader(`{"session_id":"goal-cli-fixture","source":"startup","cwd":"` + b.root + `"}`),
		Pid: os.Getpid(), Ppid: os.Getppid(), Now: b.clock, Monotonic: func() time.Duration { return 0 }, After: func(time.Duration) <-chan time.Time { return make(chan time.Time) },
		Lookup:       func(name string) (string, bool) { value, ok := environment[name]; return value, ok },
		IsExecutable: func(string) bool { return true }, Environ: func() []string { return nil }}
	code := runHookEntryWithInputs([]string{"claude", "start"}, &output, &diagnostic, func(string, string, chan os.Signal, io.Writer) hookEntryInputs {
		inputs := defaultHookEntryInputs(b.root, "start", nil, &diagnostic)
		inputs.invocation, inputs.operations = invocation, ops
		return inputs
	})
	if code != 0 || ops.prepared != 1 || ops.recovered != 1 || !strings.Contains(output.String(), "role packet") {
		t.Fatalf("the start lost its context or wait rows: exit=%d prepared=%d recovered=%d out=%s err=%s", code, ops.prepared, ops.recovered, &output, &diagnostic)
	}
	return ops.armed, output.String(), before, mains()
}

// A read the launcher starts from a seat's session carries its own kind,
// and its start hook never reaches the lease; a seat's start arms.
func TestALaunchedJobsStartNeverReachesTheLease(t *testing.T) {
	t.Parallel()
	child := launchedChildEnvironment(t)
	if child[launch.KindEnv] != "read" {
		t.Errorf("the launched read's environment names kind %q", child[launch.KindEnv])
	}
	armed, output, before, after := startHookIn(t, child)
	if armed != 0 {
		t.Fatalf("a launched read's start armed %d times: %s", armed, output)
	}
	if len(before) == 0 || len(after) != len(before) {
		t.Fatalf("a launched read's start changed the announcements: before %v after %v", before, after)
	}
	for name, data := range before {
		if after[name] != data {
			t.Fatalf("a launched read's start changed %s:\nbefore %s\nafter  %s", name, data, after[name])
		}
	}

	if armed, output, _, _ := startHookIn(t, map[string]string{launch.KindEnv: "seat"}); armed != 1 {
		t.Fatalf("a seat's start armed %d times: %s", armed, output)
	}
}
