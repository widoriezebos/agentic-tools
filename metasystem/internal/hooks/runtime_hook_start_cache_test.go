package hooks

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// Start legs of the brain-lifecycle bed: the tool engine cache, the nested
// installation layout and the bootstrap of a generation cutover.

func readEngineCache(t *testing.T, installation hookInstallation) (string, os.FileInfo) {
	t.Helper()
	path := filepath.Join(installation.root, "artifacts", "agents", "context", "engine-path")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("start wrote no tool engine cache: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), info
}

// An unregistered runtime is refused (status 2) but still writes the tool
// engine cache first, replaced by rename; an unwritable cache directory
// fails open and changes nothing about the response.
func TestHookStartToolEngineCache(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.runtimeNames = func() (string, int) { return "fake\n", 0 }
	override := filepath.Join(t.TempDir(), "override-engine")
	if err := testexec.WriteFile(override, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	call := hookCall{runtime: "claude", event: "start", payload: `{"session_id":"tool-engine-cache","source":"startup"}`, env: map[string]string{"METASYSTEM_BIN": override}}
	first := runHook(t, installation, ops, call)
	if first.status != 2 || first.stdout != noticeOf("runtime-unregistered") {
		t.Fatalf("unregistered start = status %d stdout %q", first.status, first.stdout)
	}
	cache, before := readEngineCache(t, installation)
	if cache != override+"\n"+installation.root+"\n" {
		t.Fatalf("start cached %q; want the running engine and the installation", cache)
	}

	second := runHook(t, installation, ops, call)
	_, after := readEngineCache(t, installation)
	if second.status != 2 || os.SameFile(before, after) {
		t.Fatalf("the cache was not replaced by rename: status %d same inode %t", second.status, os.SameFile(before, after))
	}
	if leftovers, _ := filepath.Glob(filepath.Join(installation.root, "artifacts", "agents", "context", "engine-path.*")); len(leftovers) != 0 {
		t.Fatalf("rename left temporary files: %v", leftovers)
	}

	directory := filepath.Join(installation.root, "artifacts", "agents", "context")
	if err := os.Chmod(directory, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(directory, 0o755)
	third := runHook(t, installation, ops, call)
	_, unchanged := readEngineCache(t, installation)
	if third.status != second.status || third.stdout != second.stdout || !os.SameFile(after, unchanged) {
		t.Fatalf("an unwritable cache changed the start: status %d stdout %q", third.status, third.stdout)
	}
}

// A session owned by another runtime leaves no cache anywhere.
func TestHookStartForeignRuntimeWritesNoCache(t *testing.T) {
	t.Parallel()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.identityRuntime = "devin"
	run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"s"}`})
	if run.status != 0 || run.stdout != "" {
		t.Fatalf("foreign start = status %d stdout %q", run.status, run.stdout)
	}
	if _, err := os.Stat(filepath.Join(installation.root, "artifacts", "agents", "context", "engine-path")); !os.IsNotExist(err) {
		t.Fatalf("a foreign runtime's start wrote the tool cache: %v", err)
	}
}

// A linked worktree's hook maps to the same installation under its primary
// checkout; a failed identification never passes as an ordinary checkout.
func TestHookWorldInstallationMapsLinkedWorktrees(t *testing.T) {
	t.Parallel()
	primary := newHookInstallation(t)
	worktree := t.TempDir()
	worktree, _ = filepath.EvalSymlinks(worktree)
	harness := filepath.Join(worktree, "metasystem")
	if err := os.MkdirAll(filepath.Join(primary.root, "metasystem", "scripts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(harness, 0o755); err != nil {
		t.Fatal(err)
	}
	ops := newFakeOps(t, primary)
	ops.git = func(args ...string) (string, error) {
		switch strings.Join(args[2:], " ") {
		case "rev-parse --path-format=absolute --git-dir --git-common-dir":
			return primary.root + "/.git/worktrees/linked\n" + primary.root + "/.git\n", nil
		case "rev-parse --show-toplevel":
			return worktree + "\n", nil
		}
		return "", errors.New("unexpected")
	}
	world, ok := worldInstallation(ops, harness)
	if !ok || world != primary.root+"/metasystem" {
		t.Fatalf("linked worktree mapped to %q (%t)", world, ok)
	}
	ops.git = func(args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "--git-common-dir") {
			return primary.root + "/.git/worktrees/linked\n" + primary.root + "/other-git-dir\n", nil
		}
		return worktree + "\n", nil
	}
	if _, ok := worldInstallation(ops, harness); ok {
		t.Fatal("a common directory that is not .git mapped to an installation")
	}
	ops.git = func(...string) (string, error) { return primary.root + "/.git\n", nil }
	if _, ok := worldInstallation(ops, harness); ok {
		t.Fatal("a one-line identification passed as an ordinary checkout")
	}
}

// A template layout: the hook's installation is nested under the Git top,
// and an ordinary checkout maps to itself.
func TestHookStartNestedInstallationReachesArming(t *testing.T) {
	t.Parallel()
	outer := t.TempDir()
	installation := newHookInstallation(t)
	ops := newFakeOps(t, installation)
	ops.identityRuntime = "fake"
	ops.git = func(args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "--git-common-dir") {
			return outer + "/.git\n" + outer + "/.git\n", nil
		}
		return "", errors.New("unexpected")
	}
	run := runHook(t, installation, ops, hookCall{runtime: "fake", event: "start", payload: `{"session_id":"nested-installation"}`})
	if run.status != 0 || strings.Contains(run.stdout, "could not identify the immediate") || !ops.called("up runtime=fake") {
		t.Fatalf("nested installation = status %d stdout %q trace %s", run.status, run.stdout, ops.trace())
	}
	if !ops.called("proc find-ancestor pid=777 runtime=fake all-hosts=false") {
		t.Fatalf("the fake runtime did not keep its exact signature path: %s", ops.trace())
	}
}

// The start boundary checks engine provenance before any owner call: a
// missing installation engine, or a missing override, is the one notice.
func TestHookStartMissingEngine(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name     string
		override string
		remove   bool
	}{{"override missing", "/metasystem-does-not-exist", false}, {"installation engine missing", "", true}} {
		t.Run(test.name, func(t *testing.T) {
			installation := newHookInstallation(t)
			if test.remove {
				_ = os.Remove(filepath.Join(installation.root, "bin", "metasystem"))
			}
			env := map[string]string{}
			if test.override != "" {
				env["METASYSTEM_BIN"] = test.override
			}
			ops := newFakeOps(t, installation)
			run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: "{}", env: env})
			if run.status != 0 || run.stdout != noticeOf("engine-missing") || strings.Contains(run.stdout, "hookSpecificOutput") {
				t.Fatalf("missing engine = status %d stdout %q", run.status, run.stdout)
			}
			if ops.called("runtime list") {
				t.Fatalf("a missing engine reached the owners: %s", ops.trace())
			}
		})
	}
}

// The hook side of a generation cutover: an engine behind the checkout's
// landed sources gets a detached rebuild and the start ends with its notice,
// never waiting on the build and never arming the engine behind; a retained
// plan holds the rebuild; a rebuild that cannot start is a notice and the
// start continues on the enrolled engine.
func TestHookStartBootstrapsAnEngineBehindTheLandedSources(t *testing.T) {
	t.Parallel()
	t.Run("detached rebuild ends the start", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.engineBehind = func() (bool, error) { return true, nil }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{"session_id":"cutover"}`})
		if run.status != 0 || run.stdout != noticeOf("engine-rebuilding") || !ops.called("start engine rebuild") ||
			ops.called("up ") || ops.called("brain boot") {
			t.Fatalf("rebuild = status %d stdout %q trace %s", run.status, run.stdout, ops.trace())
		}
	})
	t.Run("retained plan holds the rebuild", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.engineBehind = func() (bool, error) { return true, nil }
		ops.unmigratable = func() ([]string, error) { return []string{"run:held"}, nil }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{}`})
		if ops.called("start engine rebuild") || !strings.Contains(run.stdout, "retained plans run:held need it") || !ops.called("up ") {
			t.Fatalf("held rebuild = stdout %q trace %s", run.stdout, ops.trace())
		}
	})
	t.Run("a rebuild that cannot start continues on the enrolled engine", func(t *testing.T) {
		installation := newHookInstallation(t)
		ops := newFakeOps(t, installation)
		ops.engineBehind = func() (bool, error) { return true, nil }
		ops.rebuild = func() error { return errors.New("the proof mutation lock is held") }
		run := runHook(t, installation, ops, hookCall{runtime: "claude", event: "start", payload: `{}`})
		if !strings.Contains(run.stdout, "could not start a rebuild of the engine behind this checkout's sources: the proof mutation lock is held") || !ops.called("up ") {
			t.Fatalf("refused rebuild = stdout %q trace %s", run.stdout, ops.trace())
		}
	})
}

func containsEnv(environment []string, entry string) bool {
	for _, candidate := range environment {
		if candidate == entry {
			return true
		}
	}
	return false
}
