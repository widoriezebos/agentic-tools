package hooks

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// The plumbing stub scripts/agents/supervision-hook.sh: it locates the
// engine, execs `internal hook`, starts one detached rebuild when no engine
// serves the entry, and answers with the fixed degraded response. Git and the
// engine are fixture programs on the stub's PATH and path.

const stubFakeEngine = `#!/usr/bin/env bash
if [[ ${1-} == internal && ${2-} == hook && ${3-} == --accepts ]]; then
  exit "${STUB_ACCEPTS_STATUS:-0}"
fi
{
  printf 'argv=%s\n' "$*"
  printf 'script=%s\n' "${METASYSTEM_HOOK_SCRIPT-}"
  printf 'self=%s\n' "$0"
  while IFS= read -r line || [[ -n "$line" ]]; do printf 'stdin=%s\n' "$line"; done
} >>"${STUB_ENGINE_RECORD:?}"
printf 'engine stdout\n'
exit "${STUB_ENGINE_STATUS:-0}"
`

type stubBed struct {
	root, script, record string
	toolDir              string
}

func newStubBed(t *testing.T, engine string) stubBed {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"scripts/agents", "bin"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	source, err := os.ReadFile(filepath.Join("..", "..", RuntimeHookScript))
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(root, RuntimeHookScript)
	if err := testexec.WriteFile(script, source, 0o755); err != nil {
		t.Fatal(err)
	}
	if engine != "" {
		if err := testexec.WriteFile(filepath.Join(root, "bin", "metasystem"), []byte(engine), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	toolDir := t.TempDir()
	// The fixture Git answers only the stub's identification queries.
	if err := testexec.WriteFile(filepath.Join(toolDir, "git"), []byte(`#!/usr/bin/env bash
case "${STUB_GIT_MODE:-ordinary}:$*" in
  ordinary:"-C "*" rev-parse --path-format=absolute --git-dir --git-common-dir")
    printf '%s/.git\n%s/.git\n' "$2" "$2" ;;
  worktree:"-C "*" rev-parse --path-format=absolute --git-dir --git-common-dir")
    printf '%s/.git/worktrees/wt\n%s/.git\n' "${STUB_PRIMARY:?}" "${STUB_PRIMARY:?}" ;;
  worktree:"-C "*" rev-parse --show-toplevel")
    printf '%s\n' "${STUB_WORKTREE_TOP:?}" ;;
  *) exit 128 ;;
esac
`), 0o755); err != nil {
		t.Fatal(err)
	}
	return stubBed{root: root, script: script, record: filepath.Join(t.TempDir(), "engine-record"), toolDir: toolDir}
}

func (b stubBed) run(t *testing.T, script, runtime, event, input string, extra ...string) (int, string, string) {
	t.Helper()
	command := exec.Command("bash", script, runtime, event)
	var environment []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "METASYSTEM_") && !strings.HasPrefix(entry, "PATH=") && !strings.HasPrefix(entry, "GIT_") {
			environment = append(environment, entry)
		}
	}
	command.Env = append(environment, "PATH="+b.toolDir+":/usr/bin:/bin", "STUB_ENGINE_RECORD="+b.record)
	command.Env = append(command.Env, extra...)
	command.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	status := 0
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		status = exitError.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return status, stdout.String(), stderr.String()
}

func (b stubBed) recorded(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(b.record)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

func TestStubExecsTheEngineEntry(t *testing.T) {
	t.Parallel()
	bed := newStubBed(t, stubFakeEngine)
	status, stdout, stderr := bed.run(t, bed.script, "claude", "stop", "payload one\npayload two\n", "STUB_ENGINE_STATUS=23")
	if status != 23 || stdout != "engine stdout\n" || stderr != "" {
		t.Fatalf("stub = status %d stdout %q stderr %q", status, stdout, stderr)
	}
	want := "argv=internal hook claude stop\nscript=" + bed.script + "\nself=" + filepath.Join(bed.root, "bin", "metasystem") +
		"\nstdin=payload one\nstdin=payload two\n"
	if got := bed.recorded(t); got != want {
		t.Fatalf("engine record:\n got %q\nwant %q", got, want)
	}
	// Relative invocation keeps the stub's own spelling for the tool cache.
	command := exec.Command("bash", "scripts/agents/supervision-hook.sh", "claude", "tool")
	command.Dir = bed.root
	command.Env = []string{"PATH=" + bed.toolDir + ":/usr/bin:/bin", "STUB_ENGINE_RECORD=" + bed.record}
	if output, err := command.CombinedOutput(); err != nil || string(output) != "engine stdout\n" {
		t.Fatalf("relative tool invocation = %q, %v", output, err)
	}
	if !strings.Contains(bed.recorded(t), "argv=internal hook claude tool\nscript=scripts/agents/supervision-hook.sh\n") {
		t.Fatalf("relative invocation record = %q", bed.recorded(t))
	}
}

// The override engine runs the turn only while the installation carries its
// own engine: a missing canonical engine is never waived.
func TestStubOverrideNeedsTheInstallationEngine(t *testing.T) {
	t.Parallel()
	bed := newStubBed(t, stubFakeEngine)
	override := filepath.Join(t.TempDir(), "override")
	if err := testexec.WriteFile(override, []byte(stubFakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stdout, _ := bed.run(t, bed.script, "codex", "end", "{}", "METASYSTEM_BIN="+override); stdout != "engine stdout\n" ||
		!strings.Contains(bed.recorded(t), "self="+override+"\n") {
		t.Fatalf("override = stdout %q record %q", stdout, bed.recorded(t))
	}
	if err := os.Remove(filepath.Join(bed.root, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	status, stdout, _ := bed.run(t, bed.script, "claude", "stop", "{}", "METASYSTEM_BIN="+override)
	if status != 0 || stdout != mustForm(t, "allowed", "engine-missing")+"\n" {
		t.Fatalf("override without the installation engine = status %d stdout %q", status, stdout)
	}
}

// With no engine and no way to build one, every event answers with its
// fixed degraded response and nothing is written.
func TestStubDegradedFallbacks(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		event, stdout string
	}{
		{"stop", mustForm(t, "allowed", "engine-missing") + "\n"},
		{"start", StartEngineMissingNotice() + "\n"},
		{"end", ""}, {"receipt", ""}, {"tool", ""},
	} {
		t.Run(test.event, func(t *testing.T) {
			bed := newStubBed(t, "")
			status, stdout, stderr := bed.run(t, bed.script, "claude", test.event, "{}")
			if status != 0 || stdout != test.stdout || stderr != "" {
				t.Fatalf("%s = status %d stdout %q stderr %q", test.event, status, stdout, stderr)
			}
			if _, err := os.Stat(filepath.Join(bed.root, "artifacts")); !os.IsNotExist(err) {
				t.Fatalf("a degraded %s wrote state: %v", test.event, err)
			}
		})
	}
}

// Without an engine the stub still refuses what the hook never serves.
func TestStubDegradedRefusesInvalidInvocations(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ runtime, event string }{
		{"codex", "tool"}, {"devin", "tool"}, {"Bad", "stop"}, {"claude", "bogus"}, {"claude", ""},
	} {
		bed := newStubBed(t, "")
		status, stdout, stderr := bed.run(t, bed.script, test.runtime, test.event, "{}")
		if status != 2 || stdout != "" || stderr != "" {
			t.Fatalf("%s %s = status %d stdout %q stderr %q", test.runtime, test.event, status, stdout, stderr)
		}
	}
}

// An engine older than the entry gets one detached rebuild under the fence
// while the event answers at once with its degraded response; the event
// after the build uses the rebuilt engine. Builds hand off through FIFOs, so
// nothing here waits on elapsed time.
func TestStubRebuildsDetachedForAnEngineOlderThanTheEntry(t *testing.T) {
	t.Parallel()
	old := strings.Replace(stubFakeEngine, `exit "${STUB_ACCEPTS_STATUS:-0}"`, `echo 'metasystem: unknown command "hook"' >&2; exit 2`, 1)
	build := `#!/usr/bin/env bash
printf 'build %s pid=%s\n' "$PWD" "$$" >>"${STUB_BUILD_RECORD:?}"
[[ -z "${STUB_BUILD_RELEASE:-}" ]] || { read -r _ <"$STUB_BUILD_RELEASE" || true; }
status=0
[[ ${STUB_BUILD_FAILS:-0} == 0 ]] || { echo "go-build: no go toolchain" >&2; status=1; }
[[ $status != 0 ]] || cp "${STUB_NEW_ENGINE:?}" bin/metasystem
[[ -z "${STUB_BUILD_DONE:-}" ]] || printf 'done\n' >"$STUB_BUILD_DONE"
exit "$status"
`
	newEngine := filepath.Join(t.TempDir(), "new-engine")
	if err := testexec.WriteFile(newEngine, []byte(stubFakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	setup := func(t *testing.T) (stubBed, string) {
		bed := newStubBed(t, old)
		if err := testexec.WriteFile(filepath.Join(bed.root, "scripts", "agents", "go-build.sh"), []byte(build), 0o755); err != nil {
			t.Fatal(err)
		}
		return bed, filepath.Join(t.TempDir(), "builds")
	}
	awaitBuild := func(t *testing.T, done string) {
		t.Helper()
		if err := awaitFIFO(done); err != nil {
			t.Fatal(err)
		}
	}
	fenceHolder := func(t *testing.T, bed stubBed) int {
		t.Helper()
		target, err := os.Readlink(filepath.Join(bed.root, filepath.FromSlash(BootstrapFencePath)))
		if err != nil {
			t.Fatal(err)
		}
		pid, err := strconv.Atoi(target)
		if err != nil {
			t.Fatal(err)
		}
		return pid
	}
	bootstrapForm := mustForm(t, "allowed", "bootstrap-failed") + "\n"

	t.Run("start and end answer while the build runs, the next event uses the rebuilt engine", func(t *testing.T) {
		bed, builds := setup(t)
		release, done := makeFIFO(t, "release"), makeFIFO(t, "done")
		t.Cleanup(func() { unblockFIFO(release) })
		env := []string{"STUB_BUILD_RECORD=" + builds, "STUB_NEW_ENGINE=" + newEngine, "STUB_BUILD_RELEASE=" + release, "STUB_BUILD_DONE=" + done}
		status, stdout, _ := bed.run(t, bed.script, "claude", "start", "{}", env...)
		if status != 0 || stdout != StartEngineMissingNotice()+"\n" {
			t.Fatalf("start during the build = status %d stdout %q", status, stdout)
		}
		builder := fenceHolder(t, bed)
		if group, err := syscall.Getpgid(builder); err != nil || group != builder {
			t.Fatalf("the builder %d does not lead its own process group: %d %v", builder, group, err)
		}
		// A second event while the build runs neither builds nor waits;
		// SessionEnd never starts a build.
		if status, stdout, _ := bed.run(t, bed.script, "claude", "stop", "{}", env...); status != 0 || stdout != bootstrapForm {
			t.Fatalf("stop during the build = status %d stdout %q", status, stdout)
		}
		if status, stdout, _ := bed.run(t, bed.script, "claude", "end", "{}", env...); status != 0 || stdout != "" {
			t.Fatalf("end during the build = status %d stdout %q", status, stdout)
		}
		releaseFIFO(t, release)
		awaitBuild(t, done)
		record, _ := os.ReadFile(builds)
		if string(record) != "build "+bed.root+" pid="+strconv.Itoa(builder)+"\n" {
			t.Fatalf("builds = %q, want one by the fence holder %d", record, builder)
		}
		if bed.recorded(t) != "" {
			t.Fatalf("an event ran an engine that refuses the entry: %q", bed.recorded(t))
		}
		status, stdout, _ = bed.run(t, bed.script, "claude", "stop", "{}", env...)
		if status != 0 || stdout != "engine stdout\n" || !strings.Contains(bed.recorded(t), "argv=internal hook claude stop\n") {
			t.Fatalf("after the build = status %d stdout %q record %q", status, stdout, bed.recorded(t))
		}
	})
	t.Run("a stub killed mid-bootstrap leaves no blocking fence", func(t *testing.T) {
		bed, builds := setup(t)
		entered, hold := makeFIFO(t, "ln-entered"), makeFIFO(t, "ln-hold")
		t.Cleanup(func() { unblockFIFO(hold) })
		// The fixture ln stops the stub right after it claimed the fence,
		// before any builder exists; unset, it is the real ln.
		if err := testexec.WriteFile(filepath.Join(bed.toolDir, "ln"), []byte(`#!/usr/bin/env bash
/bin/ln "$@" || exit
[[ -n "${STUB_LN_ENTERED:-}" ]] || exit 0
printf 'claimed\n' >"$STUB_LN_ENTERED"
read -r _ <"$STUB_LN_HOLD" || true
`), 0o755); err != nil {
			t.Fatal(err)
		}
		command := exec.Command("bash", bed.script, "claude", "stop")
		command.Env = []string{"PATH=" + bed.toolDir + ":/usr/bin:/bin", "STUB_ENGINE_RECORD=" + bed.record,
			"STUB_LN_ENTERED=" + entered, "STUB_LN_HOLD=" + hold}
		command.Stdin = strings.NewReader("{}")
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
		if err := awaitFIFO(entered); err != nil {
			t.Fatal(err)
		}
		if holder := fenceHolder(t, bed); holder != command.Process.Pid {
			t.Fatalf("the fence names %d, not the stub %d", holder, command.Process.Pid)
		}
		if err := command.Process.Kill(); err != nil {
			t.Fatal(err)
		}
		_ = command.Wait()
		releaseFIFO(t, hold)
		done := makeFIFO(t, "done")
		status, stdout, _ := bed.run(t, bed.script, "claude", "start", "{}", "STUB_BUILD_RECORD="+builds, "STUB_NEW_ENGINE="+newEngine, "STUB_BUILD_DONE="+done)
		if status != 0 || stdout != StartEngineMissingNotice()+"\n" {
			t.Fatalf("start after the killed stub = status %d stdout %q", status, stdout)
		}
		awaitBuild(t, done)
		if record, _ := os.ReadFile(builds); !strings.HasPrefix(string(record), "build "+bed.root+" pid=") {
			t.Fatalf("the dead stub's fence blocked the rebuild: builds %q", record)
		}
	})
	t.Run("a failed build is logged and the next event retries", func(t *testing.T) {
		bed, builds := setup(t)
		for attempt := 1; attempt <= 2; attempt++ {
			done := makeFIFO(t, "done")
			status, stdout, _ := bed.run(t, bed.script, "claude", "stop", "{}", "STUB_BUILD_RECORD="+builds, "STUB_NEW_ENGINE="+newEngine,
				"STUB_BUILD_FAILS=1", "STUB_BUILD_DONE="+done)
			if status != 0 || stdout != bootstrapForm || bed.recorded(t) != "" {
				t.Fatalf("failed rebuild %d = status %d stdout %q record %q", attempt, status, stdout, bed.recorded(t))
			}
			awaitBuild(t, done)
		}
		record, _ := os.ReadFile(builds)
		log, _ := os.ReadFile(filepath.Join(bed.root, filepath.FromSlash(BootstrapLogPath)))
		if strings.Count(string(record), "build ") != 2 || strings.Count(string(log), "go-build: no go toolchain") != 2 {
			t.Fatalf("builds %q log %q", record, log)
		}
	})
	t.Run("the tool gate stays on the enrolled engine and never builds", func(t *testing.T) {
		bed, builds := setup(t)
		status, stdout, _ := bed.run(t, bed.script, "claude", "tool", "payload\n", "STUB_BUILD_RECORD="+builds, "STUB_NEW_ENGINE="+newEngine)
		if _, err := os.Stat(builds); status != 0 || stdout != "engine stdout\n" || !os.IsNotExist(err) ||
			!strings.Contains(bed.recorded(t), "argv=adapter claude-tool-gate --root "+bed.root+"\n") ||
			!strings.Contains(bed.recorded(t), "stdin=payload\n") {
			t.Fatalf("tool = status %d stdout %q build %v record %q", status, stdout, err, bed.recorded(t))
		}
		delegate, _ := setup(t)
		status, stdout, _ = delegate.run(t, delegate.script, "claude", "tool", "{}", "METASYSTEM_HOOK_DELEGATE_JOB=job-1")
		if status != 0 || stdout != "" || delegate.recorded(t) != "" {
			t.Fatalf("delegate tool = status %d stdout %q record %q", status, stdout, delegate.recorded(t))
		}
	})
}

// A linked worktree's hook runs its primary checkout's engine; a hook copied
// or linked outside an installation is governed by nothing.
func TestStubResolvesTheInstallationEngine(t *testing.T) {
	t.Parallel()
	primary := newStubBed(t, stubFakeEngine)
	worktreeTop, _ := filepath.EvalSymlinks(t.TempDir())
	source, _ := os.ReadFile(primary.script)
	linked := filepath.Join(worktreeTop, RuntimeHookScript)
	if err := os.MkdirAll(filepath.Dir(linked), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(linked, source, 0o755); err != nil {
		t.Fatal(err)
	}
	status, stdout, _ := primary.run(t, linked, "codex", "end", "{}", "STUB_GIT_MODE=worktree", "STUB_PRIMARY="+primary.root, "STUB_WORKTREE_TOP="+worktreeTop)
	if status != 0 || stdout != "engine stdout\n" || !strings.Contains(primary.recorded(t), "self="+filepath.Join(primary.root, "bin", "metasystem")+"\n") {
		t.Fatalf("linked worktree = status %d stdout %q record %q", status, stdout, primary.recorded(t))
	}

	candidates := filepath.Join(primary.root, "development", "sub", "scripts", "agents")
	if err := os.MkdirAll(candidates, 0o755); err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(candidates, "supervision-hook.sh")
	if err := testexec.WriteFile(copied, source, 0o755); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(candidates, "hook-link.sh")
	if err := os.Symlink(primary.script, symlink); err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{copied, symlink} {
		for _, override := range []string{"", "METASYSTEM_BIN=" + filepath.Join(primary.root, "bin", "metasystem")} {
			status, stdout, _ := primary.run(t, candidate, "claude", "stop", "{}", override)
			if status != 0 || stdout != mustForm(t, "allowed", "engine-missing")+"\n" {
				t.Fatalf("candidate %s (%s) = status %d stdout %q", candidate, override, status, stdout)
			}
		}
	}
	if _, err := os.Stat(filepath.Join(primary.root, "development", "sub", "artifacts")); !os.IsNotExist(err) {
		t.Fatalf("an unproven hook candidate wrote state: %v", err)
	}
}

// The shipped Claude Stop launcher, run where the hook cannot start at all,
// allows the Stop with the bootstrap form and never blocks.
func TestShippedStopLauncherAllowsWhenTheHookCannotStart(t *testing.T) {
	t.Parallel()
	command := degradedClaudeStopCommand(t, filepath.Join(degradedModuleRoot(t), degradedTemplate))
	run := exec.Command("bash", "-c", command)
	run.Dir = t.TempDir()
	run.Env = []string{"PATH=/usr/bin:/bin"}
	output, err := run.Output()
	if err != nil || string(output) != mustForm(t, "allowed", "bootstrap-failed")+"\n" {
		t.Fatalf("launcher fallback = %q, %v", output, err)
	}
}
