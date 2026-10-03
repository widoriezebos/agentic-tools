package hooks

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stateroot/stateroottest"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

// U9: the runtime settings run the engine's `internal hook RUNTIME EVENT`
// entry directly (plans/designs/verbs-object-action.md 3.3, VOA-18, VOA-19).
// These witnesses run the rendered commands through a shell in a real Git
// checkout whose installation carries no scripts/agents/supervision-hook.sh.

const directFakeEngine = `#!/bin/sh
{
  printf 'argv=%s\n' "$*"
  printf 'pwd=%s\n' "$(pwd -P)"
  printf 'self=%s\n' "$0"
  while IFS= read -r line || [ -n "$line" ]; do printf 'stdin=%s\n' "$line"; done
} >>"${DIRECT_ENGINE_RECORD:?}"
printf 'engine stdout\n'
exit "${DIRECT_ENGINE_STATUS:-0}"
`

// directBed is a Git checkout whose metasystem installation sits at
// metasystem/, as in the template repository.
type directBed struct {
	repo, installation, record string
	path                       string
}

func newDirectBed(t *testing.T, engine string) directBed {
	t.Helper()
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	installation := filepath.Join(repo, "metasystem")
	if err := os.MkdirAll(installation, 0o755); err != nil {
		t.Fatal(err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git is not installed")
	}
	bed := directBed{repo: repo, installation: installation, record: filepath.Join(t.TempDir(), "engine-record"),
		path: filepath.Dir(gitPath) + ":/usr/bin:/bin"}
	if err := os.WriteFile(filepath.Join(installation, "README"), []byte("installation\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bed.git(t, repo, "init", "-q")
	bed.git(t, repo, "add", "metasystem/README")
	bed.git(t, repo, "-c", "user.name=fixture", "-c", "user.email=fixture@invalid", "commit", "-qm", "fixture")
	if engine != "" {
		bed.installEngine(t, installation, engine)
	}
	return bed
}

func (b directBed) installEngine(t *testing.T, installation, engine string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := testexec.WriteFile(filepath.Join(installation, "bin", "metasystem"), []byte(engine), 0o755); err != nil {
		t.Fatal(err)
	}
}

func (b directBed) git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	command.Env = []string{"PATH=" + b.path, "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1"}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

// renderedCommands are the commands runtime setup writes for a runtime.
func renderedCommands(t *testing.T, shipped, runtime, installationRel string) map[string]string {
	t.Helper()
	merged, err := MergeSettings([]byte("{}\n"), []byte(shipped), runtime, installationRel, false)
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(merged, &settings); err != nil {
		t.Fatal(err)
	}
	commands := map[string]string{}
	events := map[string]string{"SessionStart": "start", "PreToolUse": "tool", "Stop": "stop", "SessionEnd": "end"}
	for event, groups := range settings.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				commands[events[event]] = handler.Command
			}
		}
	}
	return commands
}

// run runs one rendered command the way a runtime does: through a shell,
// from the directory the runtime runs in, with the payload on stdin.
func (b directBed) run(t *testing.T, dir, command, input string, extra ...string) (int, string, string) {
	t.Helper()
	shell := exec.Command("/bin/sh", "-c", command)
	shell.Dir = dir
	shell.Env = append([]string{"PATH=" + b.path, "HOME=" + dir, "GIT_CONFIG_NOSYSTEM=1", "DIRECT_ENGINE_RECORD=" + b.record}, extra...)
	shell.Stdin = strings.NewReader(input)
	var stdout, stderr bytes.Buffer
	shell.Stdout, shell.Stderr = &stdout, &stderr
	err := shell.Run()
	status := 0
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		status = exitError.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return status, stdout.String(), stderr.String()
}

func (b directBed) recorded(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(b.record)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(data)
}

// The rendered settings name no script: each lifecycle command runs the
// engine's hook entry, and the stub-era command is recognized and replaced.
func TestSettingsRenderTheDirectEngineCommand(t *testing.T) {
	t.Parallel()
	for runtime, shipped := range map[string]string{"claude": claudeShipped, "codex": codexShipped, "devin": devinShipped} {
		commands := renderedCommands(t, shipped, runtime, "metasystem")
		if len(commands) == 0 {
			t.Fatalf("%s rendered no commands", runtime)
		}
		for action, command := range commands {
			if strings.Contains(command, "supervision-hook.sh") || !strings.Contains(command, ` internal hook `+runtime+` `+action+`;`) {
				t.Fatalf("%s %s command is not the direct engine command: %s", runtime, action, command)
			}
		}
	}
	// A checkout still on the stub-era command switches in place: one owned
	// handler per event, the stub named nowhere.
	var stubEra map[string]any
	if err := json.Unmarshal([]byte(claudeShipped), &stubEra); err != nil {
		t.Fatal(err)
	}
	for event, raw := range stubEra["hooks"].(map[string]any) {
		handler := raw.([]any)[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
		command := handler["command"].(string)
		match := lifecycleCommand.FindStringSubmatch(command)
		if len(match) != 3 {
			t.Fatalf("%s template is not a lifecycle command: %s", event, command)
		}
		handler["command"] = renderStubCommand("claude", match[2], "metasystem", shippedFallback(command))
	}
	live, err := json.Marshal(stubEra)
	if err != nil {
		t.Fatal(err)
	}
	if CheckSettings(live, []byte(claudeShipped), "claude", "metasystem", false) == nil {
		t.Fatal("stub-era settings passed the direct-command readiness check")
	}
	merged, err := MergeSettings(live, []byte(claudeShipped), "claude", "metasystem", false)
	if err != nil {
		t.Fatalf("stub-era settings were not recognized: %v", err)
	}
	if err := CheckSettings(merged, []byte(claudeShipped), "claude", "metasystem", false); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(merged), "supervision-hook.sh") || strings.Count(string(merged), " internal hook claude ") != 4 {
		t.Fatalf("switched settings:\n%s", merged)
	}
}

// Every event runs the engine entry in the installation directory with the
// runtime's payload, answers with the engine's output and exit status, and
// needs no stub.
func TestDirectCommandRunsTheEngineEntry(t *testing.T) {
	t.Parallel()
	bed := newDirectBed(t, directFakeEngine)
	commands := renderedCommands(t, claudeShipped, "claude", "metasystem")
	engine := filepath.Join(bed.installation, "bin", "metasystem")
	for _, event := range []string{"start", "tool", "stop", "end"} {
		if err := os.Remove(bed.record); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		status, stdout, stderr := bed.run(t, bed.repo, commands[event], "payload one\npayload two\n", "DIRECT_ENGINE_STATUS=0")
		want := "argv=internal hook claude " + event + "\npwd=" + bed.installation + "\nself=" + engine + "\nstdin=payload one\nstdin=payload two\n"
		if status != 0 || stdout != "engine stdout\n" || stderr != "" || bed.recorded(t) != want {
			t.Fatalf("%s = status %d stdout %q stderr %q record:\n%s\nwant:\n%s", event, status, stdout, stderr, bed.recorded(t), want)
		}
	}
	// A subdirectory of the checkout is the same checkout.
	if status, stdout, _ := bed.run(t, bed.installation, commands["end"], "{}", "DIRECT_ENGINE_STATUS=0"); status != 0 || stdout != "engine stdout\n" {
		t.Fatalf("end from a subdirectory = %d %q", status, stdout)
	}
	// The engine's status is the hook's; a failed Claude Stop launch falls
	// back to its allowed bootstrap form, and the tool gate never refuses.
	if status, _, _ := bed.run(t, bed.repo, commands["end"], "{}", "DIRECT_ENGINE_STATUS=2"); status != 2 {
		t.Fatalf("end status = %d, want the engine's 2", status)
	}
	if status, stdout, _ := bed.run(t, bed.repo, commands["stop"], "{}", "DIRECT_ENGINE_STATUS=2"); status != 0 ||
		stdout != "engine stdout\n"+mustForm(t, "allowed", "bootstrap-failed")+"\n" {
		t.Fatalf("failed stop = %d %q", status, stdout)
	}
	if status, _, _ := bed.run(t, bed.repo, commands["tool"], "{}", "DIRECT_ENGINE_STATUS=2"); status != 0 {
		t.Fatalf("failed tool gate = %d", status)
	}
	if _, err := os.Stat(filepath.Join(bed.installation, "scripts", "agents", "supervision-hook.sh")); !os.IsNotExist(err) {
		t.Fatalf("the bed carries the stub: %v", err)
	}
}

// With no engine installed every event answers at once with its fixed
// degraded response naming the build, exit 0, and writes nothing.
func TestDirectCommandWithoutAnEngineAnswersDegraded(t *testing.T) {
	t.Parallel()
	bed := newDirectBed(t, "")
	commands := renderedCommands(t, claudeShipped, "claude", "metasystem")
	for _, test := range []struct{ event, stdout string }{
		{"stop", mustForm(t, "allowed", "engine-missing") + "\n"},
		{"start", StartEngineMissingNotice() + "\n"},
		{"end", ""}, {"tool", ""},
	} {
		status, stdout, stderr := bed.run(t, bed.repo, commands[test.event], "{}")
		if status != 0 || stdout != test.stdout || stderr != "" {
			t.Fatalf("%s = status %d stdout %q stderr %q", test.event, status, stdout, stderr)
		}
		if test.stdout != "" && !strings.Contains(stdout, "go run ./cmd/devgate build") {
			t.Fatalf("%s degraded answer names no build: %q", test.event, stdout)
		}
	}
	for _, runtime := range []string{"codex", "devin"} {
		shipped := map[string]string{"codex": codexShipped, "devin": devinShipped}[runtime]
		status, stdout, _ := bed.run(t, bed.repo, renderedCommands(t, shipped, runtime, "metasystem")["stop"], "{}")
		if status != 0 || stdout != mustForm(t, "allowed", "engine-missing")+"\n" {
			t.Fatalf("%s stop without an engine = %d %q", runtime, status, stdout)
		}
	}
	// An override engine is never used without the installation's own.
	override := filepath.Join(t.TempDir(), "override")
	if err := testexec.WriteFile(override, []byte(directFakeEngine), 0o755); err != nil {
		t.Fatal(err)
	}
	if status, stdout, _ := bed.run(t, bed.repo, commands["stop"], "{}", "METASYSTEM_BIN="+override); status != 0 ||
		stdout != mustForm(t, "allowed", "engine-missing")+"\n" || bed.recorded(t) != "" {
		t.Fatalf("override without the installation engine = %d %q record %q", status, stdout, bed.recorded(t))
	}
	entries, err := os.ReadDir(bed.installation)
	if err != nil || len(entries) != 1 {
		t.Fatalf("a degraded answer wrote into the installation: %v %v", entries, err)
	}
	bed.installEngine(t, bed.installation, directFakeEngine)
	if _, stdout, _ := bed.run(t, bed.repo, commands["end"], "{}", "METASYSTEM_BIN="+override); stdout != "engine stdout\n" ||
		!strings.Contains(bed.recorded(t), "self="+override+"\n") {
		t.Fatalf("override = %q record %q", stdout, bed.recorded(t))
	}
}

// A linked worktree without its own engine runs its primary checkout's,
// in the worktree's installation; Claude's tool gate prefers a local engine.
func TestDirectCommandLinkedWorktreeRunsThePrimaryEngine(t *testing.T) {
	t.Parallel()
	bed := newDirectBed(t, directFakeEngine)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	worktree := filepath.Join(parent, "linked")
	bed.git(t, bed.repo, "worktree", "add", "-q", worktree)
	commands := renderedCommands(t, claudeShipped, "claude", "metasystem")
	primary := filepath.Join(bed.installation, "bin", "metasystem")
	linkedInstallation := filepath.Join(worktree, "metasystem")
	for _, event := range []string{"start", "stop", "end", "tool"} {
		if err := os.Remove(bed.record); err != nil && !os.IsNotExist(err) {
			t.Fatal(err)
		}
		status, stdout, _ := bed.run(t, worktree, commands[event], "{}")
		if status != 0 || stdout != "engine stdout\n" || !strings.Contains(bed.recorded(t), "self="+primary+"\n") ||
			!strings.Contains(bed.recorded(t), "pwd="+linkedInstallation+"\n") {
			t.Fatalf("linked %s = %d %q record %q", event, status, stdout, bed.recorded(t))
		}
	}
	local := strings.Replace(directFakeEngine, "engine stdout", "local engine", 1)
	bed.installEngine(t, linkedInstallation, local)
	if _, stdout, _ := bed.run(t, worktree, commands["tool"], "{}"); stdout != "local engine\n" {
		t.Fatalf("tool with a local engine = %q", stdout)
	}
	if _, stdout, _ := bed.run(t, worktree, commands["stop"], "{}"); stdout != "engine stdout\n" {
		t.Fatalf("stop with a local engine = %q, want the primary's", stdout)
	}
	// The Go selection the switch validates is the command's.
	engine, err := DirectEngine(stateroottest.Installation(t, linkedInstallation), func(args ...string) (string, error) {
		command := exec.Command("git", args...)
		command.Env = []string{"PATH=" + bed.path, "HOME=" + parent}
		out, err := command.Output()
		return string(out), err
	})
	if err != nil || engine != primary {
		t.Fatalf("DirectEngine = %q, %v; want %q", engine, err, primary)
	}
}

// directTestEngineEnv makes this test binary the installation's engine: it
// serves `internal hook RUNTIME EVENT` with the production launcher and a
// fixture owner set, so a Stop runs end to end through the direct command,
// the deadline parent and its directly launched worker.
const (
	directTestEngineEnv    = "METASYSTEM_HOOK_TEST_ENGINE"
	directTestEngineRecord = "METASYSTEM_HOOK_TEST_ENGINE_RECORD"
)

func runHookTestEngine() int {
	args := os.Args[1:]
	if len(args) != 4 || args[0] != "internal" || args[1] != "hook" {
		fmt.Fprintf(os.Stderr, "test engine: unexpected argv %q\n", args)
		return 2
	}
	installation, _ := os.Getwd()
	if record := os.Getenv(directTestEngineRecord); record != "" {
		if file, err := os.OpenFile(record, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			fmt.Fprintf(file, "pid=%d ppid=%d argv=%s pwd=%s deadline-parent=%s\n", os.Getpid(), os.Getppid(), strings.Join(args, " "), installation, os.Getenv(stopDeadlineParentEnv))
			_ = file.Close()
		}
	}
	ops := &fakeOps{root: installation, identityRuntime: "claude", identityPid: 4321,
		verdict: `{"schemaVersion":1,"class":"seat-actionable","shouldBlock":false,"ledgerStatus":"ok","display":"fixture verdict","surfaceWatchdog":false,"idleRefusal":false,"brainStatusDue":false}`}
	switch os.Getenv(directTestEngineEnv) {
	case "block":
		ops.verdict = strings.Replace(ops.verdict, `"shouldBlock":false`, `"shouldBlock":true`, 1)
	case "behind":
		ops.engineBehind = func() (bool, error) { return true, nil }
		ops.rebuild = func() error {
			return os.WriteFile(filepath.Join(installation, "rebuild-started"), []byte("rebuild\n"), 0o600)
		}
	}
	// The monotonic clock is artificial, a millisecond further at each
	// read: the cost trace it feeds is never asserted on these beds, and
	// nothing the bed proves may rest on how long the host took.
	var monotonic atomic.Int64
	return RunRuntimeHook(Invocation{
		Runtime: args[2], Event: args[3], Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
		Lookup: os.LookupEnv, Pid: os.Getpid(), Ppid: os.Getppid(),
		Installation: installation,
		Now:          time.Now, Monotonic: func() time.Duration { return time.Duration(monotonic.Add(int64(time.Millisecond))) },
		// No deadline fires: the worker's exit is the event the parent waits
		// on, so nothing here waits on wall time.
		After: func(time.Duration) <-chan time.Time { return nil }, Sleep: func(time.Duration) { runtime.Gosched() },
		Exec: syscall.Exec, Environ: os.Environ,
		StartWorker: LaunchEngineWorker, TempDir: os.TempDir(),
		Deadline: DeadlineDeps{BootClock: identity.BootClock, Prober: identity.KernelProber{}, ParentPid: identity.ParentPid,
			EventInterval: 10 * time.Millisecond},
	}, ops)
}

func newDirectEngineBed(t *testing.T) directBed {
	t.Helper()
	bed := newDirectBed(t, "")
	if err := os.MkdirAll(filepath.Join(bed.installation, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(self, filepath.Join(bed.installation, "bin", "metasystem")); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(bed.installation, "artifacts", "agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bed.installation, "metasystem.conf"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return bed
}

// An allowed and a blocked Stop end to end with no stub: the settings
// command runs the engine, the engine is the deadline parent, and its worker
// is the engine's own entry launched directly with the parent's identity.
func TestDirectStopEndToEndWithoutTheStub(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"allow", "block"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			bed := newDirectEngineBed(t)
			command := renderedCommands(t, claudeShipped, "claude", "metasystem")["stop"]
			status, stdout, stderr := bed.run(t, bed.repo, command, `{"session_id":"direct-stop","hook_event_name":"Stop"}`,
				directTestEngineEnv+"="+mode, directTestEngineRecord+"="+bed.record, "TMPDIR="+t.TempDir())
			var answer map[string]string
			if status != 0 || json.Unmarshal([]byte(stdout), &answer) != nil {
				t.Fatalf("stop = status %d stdout %q stderr %q", status, stdout, stderr)
			}
			if mode == "allow" && (answer["decision"] != "" || !strings.HasPrefix(answer["systemMessage"], "presented: ")) {
				t.Fatalf("allowed stop = %q", stdout)
			}
			if mode == "block" && answer["decision"] != "block" {
				t.Fatalf("blocked stop = %q", stdout)
			}
			lines := strings.Split(strings.TrimSpace(bed.recorded(t)), "\n")
			if len(lines) != 2 {
				t.Fatalf("engine processes = %q, want the deadline parent and its worker", lines)
			}
			parent, worker := lines[0], lines[1]
			parentPid := strings.TrimPrefix(strings.Fields(parent)[0], "pid=")
			want := "argv=internal hook claude stop pwd=" + bed.installation
			if !strings.Contains(parent, want+" deadline-parent=") || !strings.HasSuffix(parent, "deadline-parent=") ||
				!strings.Contains(worker, " ppid="+parentPid+" ") || !strings.Contains(worker, want+" deadline-parent="+parentPid) {
				t.Fatalf("parent %q worker %q", parent, worker)
			}
			if _, err := os.Stat(filepath.Join(bed.installation, "scripts", "agents", "supervision-hook.sh")); !os.IsNotExist(err) {
				t.Fatalf("the bed carries the stub: %v", err)
			}
		})
	}
}

// A checkout whose sources are ahead of its engine: SessionStart through
// the direct command starts the rebuild detached and answers with the
// rebuilding notice, as the stub's bootstrap did.
func TestDirectStartRebuildsAnEngineBehindItsSources(t *testing.T) {
	t.Parallel()
	bed := newDirectEngineBed(t)
	command := renderedCommands(t, claudeShipped, "claude", "metasystem")["start"]
	status, stdout, stderr := bed.run(t, bed.repo, command, `{"session_id":"direct-start","source":"startup"}`,
		directTestEngineEnv+"=behind", "TMPDIR="+t.TempDir())
	if status != 0 || stdout != StartOutcomeNotices["engine-rebuilding"].text+"\n" {
		t.Fatalf("start = status %d stdout %q stderr %q", status, stdout, stderr)
	}
	if data, err := os.ReadFile(filepath.Join(bed.installation, "rebuild-started")); err != nil || string(data) != "rebuild\n" {
		t.Fatalf("no rebuild started: %q %v", data, err)
	}
}
