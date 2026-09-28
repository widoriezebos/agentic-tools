package hostturn

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/adapter/supervisor"
)

// holdStandIn stands in for `ENGINE util hold`: it records its pid and argv
// in its ready file and, on SIGTERM, writes its stopped file and exits. A
// shell that starts with SIGTERM ignored cannot trap it, so an inherited
// ignore disposition shows as a hold that survives SIGTERM.
const holdStandIn = `#!/bin/sh
[ "$1" = util ] && [ "$2" = hold ] || { echo "hold stand-in: unexpected argv: $*" >&2; exit 64; }
shift 2
argv="$*"
stopped= ready=
while [ $# -gt 0 ]; do
  case "$1" in
    --stopped-file) stopped=$2; shift 2 ;;
    --ready-file) ready=$2; shift 2 ;;
    --ignore-term) shift ;;
    *) shift 2 ;;
  esac
done
sleeper=
trap '[ -z "$sleeper" ] || kill "$sleeper" 2>/dev/null; : >"$stopped"; exit 0' TERM
printf '%s\n%s\n' "$$" "$argv" >"$ready.tmp" && mv "$ready.tmp" "$ready"
while :; do sleep 0.2 & sleeper=$!; wait "$sleeper"; done
`

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type hostInstall struct {
	t                     *testing.T
	root, turnDir, engine string
	env                   map[string]string
	stderr                *lockedBuffer
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func decode(t *testing.T, path string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(readFile(t, path)), &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func present(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// newHostInstall lays out one fake host turn: the mission state with an
// active stream, the turn record, and the prompt.
func newHostInstall(t *testing.T, prompt string, fixtureRoot bool) *hostInstall {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := &hostInstall{t: t, root: root, env: map[string]string{}, stderr: &lockedBuffer{},
		turnDir: filepath.Join(root, "artifacts", "agents", "missions", "m1", "turns", "t1"),
		engine:  filepath.Join(root, "bin", "hold-standin")}
	if fixtureRoot {
		writeFile(t, filepath.Join(root, "metasystem.conf"), "# fixture root\nmetasystem.runtimes = fake\n")
	}
	writeFile(t, h.engine, holdStandIn)
	if err := os.Chmod(h.engine, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "artifacts", "agents", "missions", "m1", "state.json"),
		`{"streams":{"a-stream":{"state":"parked"},"b-stream":{"state":"active"}}}`)
	writeFile(t, h.path("turn.json"), `{"turnId":"t1","missionId":"m1","cycle":4,"model":"fake-model","hostSession":null,"startedAt":"2026-09-27T10:00:00Z"}`)
	writeFile(t, h.path("prompt.md"), prompt)
	return h
}

func (h *hostInstall) path(name string) string { return filepath.Join(h.turnDir, name) }

func (h *hostInstall) environ() []string {
	var out []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "METASYSTEM_") && !strings.HasPrefix(entry, "FAKE_") {
			out = append(out, entry)
		}
	}
	for name, value := range h.env {
		out = append(out, name+"="+value)
	}
	return out
}

func (h *hostInstall) deps() supervisor.Deps {
	environ := h.environ()
	return supervisor.Deps{
		Root: h.root, Engine: h.engine, Environ: environ,
		Getenv: func(name string) string {
			for _, entry := range environ {
				if key, value, ok := strings.Cut(entry, "="); ok && key == name {
					return value
				}
			}
			return ""
		},
		Pid: os.Getpid(), Stdout: &lockedBuffer{}, Stderr: h.stderr,
		Clock: supervisor.SystemClock(), LookPath: exec.LookPath,
	}
}

func (h *hostInstall) args(extra ...string) []string {
	return append([]string{"fake", "start-turn", "--root", h.root, "--mission", "m1", "--turn-id", "t1",
		"--prompt", h.path("prompt.md"), "--result", h.path("result.json"), "--instance-tag", "host-tag-1"}, extra...)
}

func (h *hostInstall) run(extra ...string) int {
	d := h.deps()
	return Main(h.args(extra...), func(string) supervisor.Deps { return d })
}

// TestFakeHostBehaviors is the FAKEHOST behavior table: each behavior's
// exit status, return, and result envelope.
func TestFakeHostBehaviors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		behavior   string
		exit       int
		outcome    string
		returnPath bool
		check      func(t *testing.T, h *hostInstall)
	}{
		{behavior: "", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			ret := decode(t, h.path("return.json"))
			if ret["turnId"] != "t1" || ret["identity"].(map[string]any)["runtime"] != "fake" || len(ret["dispatched"].([]any)) != 0 {
				t.Fatalf("return = %v", ret)
			}
		}},
		{behavior: "return-ok", exit: 0, outcome: "completed", returnPath: true},
		{behavior: "return-malformed", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			if got := readFile(t, h.path("return.json")); got != "{malformed\n" {
				t.Fatalf("return = %q", got)
			}
		}},
		{behavior: "dispatch-ghost", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			dispatched := decode(t, h.path("return.json"))["dispatched"].([]any)
			if entry := dispatched[0].(map[string]any); entry["jobId"] != "ghost-4" || entry["stream"] != "b-stream" {
				t.Fatalf("dispatched = %v", dispatched)
			}
		}},
		{behavior: "dispatch-terminal", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			record := decode(t, filepath.Join(h.root, "artifacts", "agents", "jobs", "verifier-m1.json"))
			if record["status"] != "completed" || record["turnId"] != "t1" {
				t.Fatalf("job record = %v", record)
			}
		}},
		{behavior: "solo-build", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			if got := readFile(t, filepath.Join(h.root, "solo.go")); got != "package solo\n" {
				t.Fatalf("solo.go = %q", got)
			}
		}},
		{behavior: "close-stream", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			update := decode(t, h.path("return.json"))["streamUpdatesRequested"].([]any)[0].(map[string]any)
			if update["requestedState"] != "done" || update["streamId"] != "b-stream" {
				t.Fatalf("update = %v", update)
			}
		}},
		{behavior: "park-request", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			ret := decode(t, h.path("return.json"))
			if len(ret["askCandidates"].([]any)) != 1 || ret["streamUpdatesRequested"].([]any)[0].(map[string]any)["requestedState"] != "parked-reserved" {
				t.Fatalf("return = %v", ret)
			}
		}},
		{behavior: "exit-nonzero", exit: 3, outcome: "failed", check: func(t *testing.T, h *hostInstall) {
			if present(h.path("return.json")) || present(h.path("host.log")) {
				t.Fatal("exit-nonzero wrote a return or host log")
			}
		}},
		{behavior: "exit-overloaded", exit: 3, outcome: "failed", check: func(t *testing.T, h *hostInstall) {
			if got := readFile(t, h.path("host.log")); got != `API Error: 529 {"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`+"\n" {
				t.Fatalf("host.log = %q", got)
			}
		}},
		{behavior: "overloaded-result", exit: 0, outcome: "completed", check: func(t *testing.T, h *hostInstall) {
			if got := readFile(t, h.path("provider-result.json")); got != `{"is_error":true,"result":"API Error: 529 Overloaded"}`+"\n" {
				t.Fatalf("provider-result = %q", got)
			}
			if present(h.path("return.json")) {
				t.Fatal("overloaded-result wrote a return")
			}
		}},
		{behavior: "no-return", exit: 0, outcome: "completed", returnPath: true, check: func(t *testing.T, h *hostInstall) {
			if present(h.path("return.json")) {
				t.Fatal("no-return wrote a return")
			}
		}},
	}
	for _, tc := range cases {
		name := tc.behavior
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
			prompt := "Host turn.\n"
			if tc.behavior != "" {
				prompt += "Behavior: FAKEHOST:" + tc.behavior + " please\n"
			}
			h := newHostInstall(t, prompt, false)
			if code := h.run(); code != tc.exit {
				t.Fatalf("exit %d, want %d; stderr %s", code, tc.exit, h.stderr.String())
			}
			shown := tc.behavior
			if shown == "" {
				shown = "return-ok"
			}
			if got := readFile(t, h.path("raw.out")); got != "fake host behavior="+shown+" instance=host-tag-1\n" {
				t.Fatalf("raw = %q", got)
			}
			result := decode(t, h.path("result.json"))
			if result["outcome"] != tc.outcome || result["sessionId"] != "fake-host-session-m1" || result["rawPath"] != h.path("raw.out") {
				t.Fatalf("result = %v", result)
			}
			if tc.returnPath != (result["returnPath"] == h.path("return.json")) || (!tc.returnPath && result["returnPath"] != nil) {
				t.Fatalf("result returnPath = %v, want present=%v", result["returnPath"], tc.returnPath)
			}
			if tc.check != nil {
				tc.check(t, h)
			}
		})
	}
}

// TestFakeHostMarkerGrammar covers the sed-and-sort-u parsing: one line's
// last marker, duplicates collapsing, multiple behaviors and unknown ones
// refused, and a marker that is not lowercase ignored.
func TestFakeHostMarkerGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, prompt, want, refusal string
	}{
		{name: "last marker on a line", prompt: "FAKEHOST:park-request then FAKEHOST:close-stream\n", want: "close-stream"},
		{name: "duplicates collapse", prompt: "FAKEHOST:no-return\nagain FAKEHOST:no-return\n", want: "no-return"},
		{name: "uppercase is no marker", prompt: "FAKEHOST:Close-stream\n", want: "return-ok"},
		{name: "multiple behaviors", prompt: "FAKEHOST:no-return\nFAKEHOST:close-stream\n", refusal: "fake host prompt contains multiple behaviors\n"},
		{name: "unknown behavior", prompt: "FAKEHOST:bogus-thing\n", refusal: "unknown fake host behavior: bogus-thing\n"},
		{name: "marker without newline", prompt: "FAKEHOST:solo-build", want: "solo-build"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHostInstall(t, tc.prompt, false)
			code := h.run()
			if tc.refusal != "" {
				if code != 3 || h.stderr.String() != tc.refusal || present(h.path("raw.out")) {
					t.Fatalf("exit %d stderr %q, want 3 and %q", code, h.stderr.String(), tc.refusal)
				}
				return
			}
			if code != 0 {
				t.Fatalf("exit %d, stderr %s", code, h.stderr.String())
			}
			if got := readFile(t, h.path("raw.out")); got != "fake host behavior="+tc.want+" instance=host-tag-1\n" {
				t.Fatalf("raw = %q", got)
			}
		})
	}
}

// TestFakeHostSetup covers the checks before any behavior: the turn
// record, the unverified start, the start gate, the resume session, and the
// fixture-mode guard on the hold controls.
func TestFakeHostSetup(t *testing.T) {
	t.Parallel()
	t.Run("missing turn record", func(t *testing.T) {
		h := newHostInstall(t, "x\n", false)
		os.Remove(h.path("turn.json"))
		if code := h.run(); code != 3 || h.stderr.String() != "fake host turn record is missing: "+h.path("turn.json")+"\n" {
			t.Fatalf("exit %d stderr %q", code, h.stderr.String())
		}
	})
	t.Run("start unverified exits before the gate", func(t *testing.T) {
		h := newHostInstall(t, "x\n", false)
		h.env["METASYSTEM_FAKE_HOST_START_UNVERIFIED"] = "1"
		h.env["METASYSTEM_HOST_START_GATE"] = filepath.Join(h.root, "never")
		if code := h.run(); code != 0 || present(h.path("raw.out")) || present(h.path("result.json")) {
			t.Fatalf("exit %d", code)
		}
	})
	t.Run("gate opened", func(t *testing.T) {
		h := newHostInstall(t, "x\n", false)
		writeFile(t, filepath.Join(h.root, "gate"), "")
		h.env["METASYSTEM_HOST_START_GATE"] = filepath.Join(h.root, "gate")
		if code := h.run(); code != 0 {
			t.Fatalf("exit %d stderr %s", code, h.stderr.String())
		}
	})
	t.Run("gate never released", func(t *testing.T) {
		h := newHostInstall(t, "x\n", false)
		h.env["METASYSTEM_HOST_START_GATE"] = filepath.Join(h.root, "never")
		h.env["METASYSTEM_HOST_START_GATE_TIMEOUT_SEC"] = "1"
		if code := h.run(); code != 3 || h.stderr.String() != "fake host start gate was not released within 1s\n" {
			t.Fatalf("exit %d stderr %q", code, h.stderr.String())
		}
		if present(h.path("raw.out")) {
			t.Fatal("a gated turn ran")
		}
	})
	t.Run("resume session", func(t *testing.T) {
		h := newHostInstall(t, "x\n", false)
		if code := h.run("--resume-session", "sess-9"); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if got := decode(t, h.path("result.json"))["sessionId"]; got != "sess-9" {
			t.Fatalf("session = %v", got)
		}
	})
	for _, name := range []string{"METASYSTEM_FAKE_HOST_HOLD", "METASYSTEM_FAKE_HOST_IGNORE_TERM"} {
		t.Run(name+" outside a fixture root", func(t *testing.T) {
			h := newHostInstall(t, "x\n", false)
			writeFile(t, filepath.Join(h.root, "metasystem.conf"), "metasystem.runtimes = claude\n")
			h.env[name] = "1"
			if code := h.run(); code != 3 || h.stderr.String() != "METASYSTEM_FAKE_HOST_HOLD and METASYSTEM_FAKE_HOST_IGNORE_TERM are available only in a fixture-mode root\n" {
				t.Fatalf("exit %d stderr %q", code, h.stderr.String())
			}
		})
	}
	t.Run("ignore-term alone runs the turn", func(t *testing.T) {
		h := newHostInstall(t, "x\n", true)
		h.env["METASYSTEM_FAKE_HOST_IGNORE_TERM"] = "1"
		if code := h.run(); code != 0 || !present(h.path("result.json")) {
			t.Fatalf("exit %d stderr %s", code, h.stderr.String())
		}
	})
}

// TestFakeHostHelperProcess is the subprocess body for the hold, which
// replaces the process. It is not a test.
func TestFakeHostHelperProcess(t *testing.T) {
	t.Parallel()
	if os.Getenv("FAKE_HOST_HELPER") != "1" {
		t.Skip("subprocess helper")
	}
	var args []string
	if err := json.Unmarshal([]byte(os.Getenv("FAKE_HOST_ARGS")), &args); err != nil {
		os.Exit(90)
	}
	d := supervisor.ProcessDeps(args[3])
	d.Engine = os.Getenv("FAKE_HOST_ENGINE")
	os.Exit(Main(args, func(string) supervisor.Deps { return d }))
}

// TestFakeHostHold: the hold replaces the host process (the same pid the
// runner verified) with the engine's util hold and its turn files; with
// IGNORE_TERM the ignored SIGTERM disposition survives the exec.
func TestFakeHostHold(t *testing.T) {
	t.Parallel()
	for _, ignore := range []bool{false, true} {
		t.Run("ignore-term="+strconv.FormatBool(ignore), func(t *testing.T) {
			h := newHostInstall(t, "x\n", true)
			h.env["METASYSTEM_FAKE_HOST_HOLD"] = "1"
			if ignore {
				h.env["METASYSTEM_FAKE_HOST_IGNORE_TERM"] = "1"
			}
			args, _ := json.Marshal(h.args())
			command := exec.Command(os.Args[0], "-test.run=^TestFakeHostHelperProcess$")
			command.Env = append(h.environ(), "FAKE_HOST_HELPER=1", "FAKE_HOST_ARGS="+string(args), "FAKE_HOST_ENGINE="+h.engine)
			output, err := os.Create(filepath.Join(h.root, "host.out"))
			if err != nil {
				t.Fatal(err)
			}
			defer output.Close()
			command.Stdout, command.Stderr = output, output
			if err := command.Start(); err != nil {
				t.Fatal(err)
			}
			pid := command.Process.Pid
			t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })
			deadline := time.Now().Add(20 * time.Second)
			for !present(h.path("host-ready")) {
				if time.Now().After(deadline) {
					t.Fatalf("the hold never became ready; output %s", readFile(t, filepath.Join(h.root, "host.out")))
				}
				time.Sleep(10 * time.Millisecond)
			}
			lines := strings.Split(strings.TrimSpace(readFile(t, h.path("host-ready"))), "\n")
			if lines[0] != strconv.Itoa(pid) {
				t.Fatalf("hold pid %s, host pid %d: the hold did not replace the host", lines[0], pid)
			}
			want := "--tag host-tag-1 --ready-file " + h.path("host-ready") + " --stopped-file " + h.path("host-stopped")
			if ignore {
				want += " --ignore-term --term-observed-file " + h.path("host-term-observed")
			}
			if lines[1] != want {
				t.Fatalf("hold argv %q, want %q", lines[1], want)
			}
			_ = syscall.Kill(pid, syscall.SIGTERM)
			if ignore {
				time.Sleep(300 * time.Millisecond)
				if present(h.path("host-stopped")) || syscall.Kill(pid, 0) != nil {
					t.Fatal("an ignore-term hold stopped on SIGTERM")
				}
				_ = syscall.Kill(pid, syscall.SIGKILL)
				command.Wait()
				return
			}
			if err := command.Wait(); err != nil {
				t.Fatalf("hold ended %v, want exit 0", err)
			}
			if !present(h.path("host-stopped")) {
				t.Fatal("the hold wrote no stopped file")
			}
			if present(h.path("raw.out")) || present(h.path("result.json")) {
				t.Fatal("a holding host ran its behavior")
			}
		})
	}
}
