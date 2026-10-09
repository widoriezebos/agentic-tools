package supervisor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/verbresult"
)

// The fake runtime's test installation: a real artifacts tree in
// t.TempDir(), a job record carrying a minted launch capability, an
// engine stand-in script that behaves like `util hold` and records
// `internal delegate` calls, and a recording Dispatcher standing in for
// the delegate lifecycle's lease-held callbacks.

// engineStandIn is the engine binary the fake's children run. `util hold`
// records its argv under FAKE_HOLD_DIR/<pid>, writes its ready file, and on
// SIGTERM writes its stopped file and exits (or, with --ignore-term, writes
// the observed file and keeps holding). `internal delegate` appends its argv
// and delegate environment to FAKE_DELEGATE_LOG and exits
// FAKE_DELEGATE_STATUS.
const engineStandIn = `#!/bin/sh
if [ "$1" = util ] && [ "$2" = hold ]; then
  shift 2
  printf '%s\n' "$@" >"$FAKE_HOLD_DIR/.$$" && mv "$FAKE_HOLD_DIR/.$$" "$FAKE_HOLD_DIR/$$"
  stopped= ready= ignore= observed=
  while [ $# -gt 0 ]; do
    case "$1" in
      --stopped-file) stopped=$2; shift 2 ;;
      --ready-file) ready=$2; shift 2 ;;
      --term-observed-file) observed=$2; shift 2 ;;
      --ignore-term) ignore=1; shift ;;
      *) shift ;;
    esac
  done
  sleeper=
  if [ -n "$ignore" ]; then
    trap '[ -z "$observed" ] || : >"$observed"' TERM
  else
    trap '[ -z "$sleeper" ] || kill "$sleeper" 2>/dev/null; [ -z "$stopped" ] || : >"$stopped"; exit 0' TERM
  fi
  [ -z "$ready" ] || echo "$$" >"$ready"
  while :; do sleep 0.2 & sleeper=$!; wait "$sleeper"; done
fi
if [ "$1" = internal ] && [ "$2" = delegate ]; then
  shift 2
  printf '%s|root=%s|internal=%s\n' "$*" "${METASYSTEM_DELEGATE_ROOT-}" "${METASYSTEM_DELEGATE_SELFTEST_INTERNAL-}" >>"$FAKE_DELEGATE_LOG"
  exit "${FAKE_DELEGATE_STATUS:-0}"
fi
echo "engine stand-in: unexpected argv: $*" >&2
exit 64
`

// syncBuffer is a writer children and the supervisor share.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// fakeRecordingDispatcher appends each callback's argv, one JSON array per
// line, to a file, and answers with a configured status. A successful
// __handshake records the session in the job record, as the lifecycle's
// handshake does, and status answers the record's status.
type fakeRecordingDispatcher struct {
	Root     string         `json:"root"`
	Log      string         `json:"log"`
	Statuses map[string]int `json:"statuses"`
	// KeepSession leaves the record's sessionId untouched on handshake.
	KeepSession bool `json:"keepSession"`
}

func (r fakeRecordingDispatcher) Run(stdout, _ io.Writer, args ...string) int {
	line, _ := json.Marshal(args)
	file, err := os.OpenFile(r.Log, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err == nil {
		file.Write(append(line, '\n'))
		file.Close()
	}
	status := r.Statuses[args[0]]
	if status == 0 && args[0] == "status" {
		// The lifecycle's status --json: an envelope whose data is the job
		// record's status, "unknown" when there is no record (a self-test's
		// poll).
		answer := "unknown"
		data, readErr := os.ReadFile(filepath.Join(r.Root, "artifacts", "agents", "jobs", flagValue(args, "--job")+".json"))
		var record map[string]any
		if readErr == nil && json.Unmarshal(data, &record) == nil {
			if value, ok := record["status"].(string); ok {
				answer = value
			}
		}
		_ = verbresult.Write(stdout, verbresult.Result{Verb: delegateStatusVerb, Outcome: verbresult.Confirmed,
			Summary: "job status", Data: json.RawMessage(`{"status":"` + answer + `"}`)})
		return 0
	}
	if status == 0 && args[0] == "__handshake" && !r.KeepSession {
		job, session := flagValue(args, "--job"), flagValue(args, "--session")
		path := filepath.Join(r.Root, "artifacts", "agents", "jobs", job+".json")
		editRecord(path, func(record map[string]any) { record["sessionId"] = session })
	}
	return status
}

func editRecord(path string, edit func(map[string]any)) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var record map[string]any
	if json.Unmarshal(data, &record) != nil {
		return
	}
	edit(record)
	data, _ = json.Marshal(record)
	os.WriteFile(path, data, 0o644)
}

// calls reads the recorded callback argvs.
func (r fakeRecordingDispatcher) calls(t *testing.T) [][]string {
	t.Helper()
	data, err := os.ReadFile(r.Log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var call []string
		if err := json.Unmarshal([]byte(line), &call); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, call)
	}
	return calls
}

// stepClock is a fake clock: each Sleep advances Now by the slept duration
// and runs onSleep with the sleep count.
type stepClock struct {
	mu      sync.Mutex
	now     time.Time
	sleeps  int
	onSleep func(int)
}

func (c *stepClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *stepClock) Sleep(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.sleeps++
	count, hook := c.sleeps, c.onSleep
	c.mu.Unlock()
	if hook != nil {
		hook(count)
	}
}

type fakeInstall struct {
	t                          *testing.T
	root, workspace            string
	job, rootJob, tag, verb    string
	capability, gate, round    string
	engine, holdDir, delegates string
	dispatcher                 fakeRecordingDispatcher
	env                        map[string]string
	clock                      Clock
	stdout, stderr             *syncBuffer
}

type installOptions struct {
	verb      string
	round     int
	prompt    string
	staged    string
	sessionID any
	// parent is the chain root's job id for a round above 1.
	parent string
	role   string
	// writeRoots overrides the requested write roots (default: the
	// workspace).
	writeRoots []string
}

func metasystemRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, path, string(data)+"\n")
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal([]byte(readText(t, path)), &value); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return value
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// newFakeInstall lays out one pending fake round ready for its supervisor.
func newFakeInstall(t *testing.T, opts installOptions) *fakeInstall {
	t.Helper()
	if opts.verb == "" {
		opts.verb = "dispatch"
	}
	if opts.round == 0 {
		opts.round = 1
	}
	if opts.role == "" {
		opts.role = "verifier"
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeInstall{
		t: t, root: root, workspace: filepath.Join(root, "ws"),
		job: "fake-job-1", tag: "fake-tag-1", verb: opts.verb, round: strconv.Itoa(opts.round),
		capability: strings.Repeat("ab", 32), gate: filepath.Join(root, "gate"),
		holdDir: filepath.Join(root, "holds"), delegates: filepath.Join(root, "delegate.log"),
		env: map[string]string{}, clock: SystemClock(), stdout: &syncBuffer{}, stderr: &syncBuffer{},
	}
	f.rootJob = f.job
	if opts.parent != "" {
		f.rootJob = opts.parent
		f.job = "fake-job-2"
	}
	f.dispatcher = fakeRecordingDispatcher{Root: root, Log: filepath.Join(root, "dispatch.log"), Statuses: map[string]int{}}
	for _, dir := range []string{f.workspace, f.holdDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	schema := filepath.Join(metasystemRoot(t), "internal", "protocol", "schemas", opts.role+".schema.json")
	mustWrite(t, filepath.Join(root, "internal", "protocol", "schemas", opts.role+".schema.json"), readText(t, schema))
	f.engine = filepath.Join(root, "bin", "engine-standin")
	mustWrite(t, f.engine, engineStandIn)
	if err := os.Chmod(f.engine, 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, f.gate, "")

	jobs := filepath.Join(root, "artifacts", "agents", "jobs")
	if opts.parent != "" {
		mustJSON(t, filepath.Join(jobs, opts.parent+".json"), map[string]any{
			"jobId": opts.parent, "round": 1, "parentJob": nil, "role": opts.role, "status": "completed",
		})
	}
	writeRoots := []string{f.workspace}
	if opts.writeRoots != nil {
		writeRoots = opts.writeRoots
	}
	digest := sha256.Sum256([]byte(f.capability))
	var parent any
	if opts.parent != "" {
		parent = opts.parent
	}
	mustJSON(t, filepath.Join(jobs, f.job+".json"), map[string]any{
		"jobId": f.job, "operationId": f.job, "status": "pending", "round": opts.round,
		"parentJob": parent, "role": opts.role, "runtime": "fake", "sessionId": opts.sessionID,
		"requestedModel": "fake-model", "effectiveModel": nil, "workspaceRoot": f.workspace,
		"sessionEstablishedSignal": "fake-signal", "instanceTag": f.tag,
		"permissions": map[string]any{"requested": map[string]any{
			"readRoots": []string{}, "writeRoots": writeRoots, "network": "deny",
		}},
		"launchCapability": map[string]any{
			"digest": hex.EncodeToString(digest[:]), "jobId": f.job, "operationId": f.job,
			"instanceTag": f.tag, "adapterVerb": opts.verb, "status": "minted",
			"mintedAt": time.Now().UTC().Format(time.RFC3339),
		},
	})
	prompt := opts.prompt
	if prompt == "" {
		prompt = "Working Mode: implement\n\nDo the fake thing.\n"
	}
	mustWrite(t, filepath.Join(f.roundDir(), "prompt.md"), prompt)
	if opts.staged != "" {
		mustWrite(t, filepath.Join(f.roundDir(), "staged", "task-direction.md"), opts.staged)
	}
	mustWrite(t, filepath.Join(f.roundDir(), "composition.json"), `{"references":[]}`+"\n")
	return f
}

func (f *fakeInstall) agents() string { return filepath.Join(f.root, "artifacts", "agents") }
func (f *fakeInstall) roundDir() string {
	return filepath.Join(f.agents(), f.rootJob, "rounds", f.round)
}
func (f *fakeInstall) roundFile(name string) string {
	return filepath.Join(f.roundDir(), name)
}
func (f *fakeInstall) logText() string {
	return readText(f.t, filepath.Join(f.agents(), "jobs", f.job+".log"))
}
func (f *fakeInstall) heartbeat() string {
	return filepath.Join(f.agents(), "hb", f.job)
}

// environ is the supervisor's environment: the test process's without any
// METASYSTEM_ variable, plus the install's overrides and the stand-in's.
func (f *fakeInstall) environ() []string {
	var out []string
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "METASYSTEM_") || strings.HasPrefix(entry, "FAKE_") {
			continue
		}
		out = append(out, entry)
	}
	out = append(out, "FAKE_HOLD_DIR="+f.holdDir, "FAKE_DELEGATE_LOG="+f.delegates)
	for name, value := range f.env {
		out = withEnv(out, name+"="+value)
	}
	return out
}

func (f *fakeInstall) deps() Deps {
	environ := f.environ()
	return Deps{
		Root: f.root, Engine: f.engine, Environ: environ,
		Getenv: func(name string) string {
			for _, entry := range environ {
				if key, value, ok := strings.Cut(entry, "="); ok && key == name {
					return value
				}
			}
			return ""
		},
		Pid: os.Getpid(), Stdout: f.stdout, Stderr: f.stderr, Clock: f.clock,
		Dispatch:     f.dispatcher,
		GroupMembers: func(int, ...int) ([]int, error) { return nil, nil },
		LookPath:     exec.LookPath,
		UserCacheDir: func() (string, error) { return filepath.Join(f.root, "user-cache"), nil },
	}
}

func (f *fakeInstall) superviseArgs() []string {
	return []string{"fake", f.verb, "--root", f.root, "--job", f.job, "--start-gate", f.gate,
		"--instance-tag", f.tag, "--launch-capability", f.capability}
}

// run drives `delegate-supervisor fake VERB` in process.
func (f *fakeInstall) run(args ...string) int {
	if len(args) == 0 {
		args = f.superviseArgs()
	}
	d := f.deps()
	return Main(args, func(string) Deps { return d })
}

// holds returns the recorded argv of every hold child, by pid.
func (f *fakeInstall) holds() map[int][]string {
	entries, _ := os.ReadDir(f.holdDir)
	out := map[int][]string{}
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		data, _ := os.ReadFile(filepath.Join(f.holdDir, entry.Name()))
		out[pid] = strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	}
	return out
}

// waitFor polls cond until it holds: the fact is the event, bounded only by
// the test binary's deadline.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	testenv.Await(t, what, cond)
}

// waitPidFile waits for a round pid file to be written whole (the writer
// creates it before its content lands) and returns the pid.
func (f *fakeInstall) waitPidFile(name string) int {
	f.t.Helper()
	var pid int
	waitFor(f.t, name, func() bool {
		data, err := os.ReadFile(f.roundFile(name))
		if err != nil || !strings.HasSuffix(string(data), "\n") {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(data)))
		return err == nil
	})
	return pid
}

func processAlive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

// helperConfig is the subprocess supervisor's configuration.
type helperConfig struct {
	Args       []string                `json:"args"`
	Root       string                  `json:"root"`
	Engine     string                  `json:"engine"`
	Dispatcher fakeRecordingDispatcher `json:"dispatcher"`
}

// runFakeSupervisorHelper is the subprocess body for the behaviors that end
// or hold the supervisor process itself (kill -KILL $$, the forever
// heartbeat loops, the SIGTERM trap). TestMain runs it in place of the test
// environment: the subprocess stands in for the product supervisor, so it
// starts no fixture custodian, whose reaping of the owner's descendants at
// exit would end the children the supervisor leaves to the dispatcher. The
// parent test's custodian and cleanups own those children. The helper's
// markers are dropped so no child it starts is taken for the helper.
func runFakeSupervisorHelper() int {
	encoded := os.Getenv("FAKE_SUPERVISOR_CONFIG")
	_ = os.Unsetenv("FAKE_SUPERVISOR_HELPER")
	_ = os.Unsetenv("FAKE_SUPERVISOR_CONFIG")
	var config helperConfig
	if err := json.Unmarshal([]byte(encoded), &config); err != nil {
		return 90
	}
	d := ProcessDeps(config.Root)
	d.Engine = config.Engine
	d.Dispatch = config.Dispatcher
	return Main(config.Args, func(string) Deps { return d })
}

// fakeSupervisorHelperCommand is the supervisor subprocess command.
func fakeSupervisorHelperCommand(config []byte, env ...string) *exec.Cmd {
	command := exec.Command(os.Args[0])
	command.Env = append(env, "FAKE_SUPERVISOR_HELPER=1", "FAKE_SUPERVISOR_CONFIG="+string(config))
	return command
}

// fixtureCustodiansOf returns the live children of pid that are fixture
// custodians (they carry the custodian's environment marker).
func fixtureCustodiansOf(t *testing.T, pid int) []identity.Ref {
	t.Helper()
	census, err := identity.TakeProcessCensus()
	if err != nil {
		t.Fatalf("process census: %v", err)
	}
	var custodians []identity.Ref
	for _, child := range census.Pids() {
		if parent, known := census.Parent(child); !known || parent != int64(pid) {
			continue
		}
		exact, state, err := identity.KernelProber{}.Probe(child)
		if err != nil || state != identity.Alive || !exact.EnvironKnown {
			continue
		}
		for _, entry := range exact.Environ {
			if entry == identity.FixtureCustodianEnv+"=1" {
				custodians = append(custodians, exact.Ref())
				break
			}
		}
	}
	return custodians
}

// startSubprocess runs the supervisor in its own process (and group).
func (f *fakeInstall) startSubprocess() *exec.Cmd {
	f.t.Helper()
	config, _ := json.Marshal(helperConfig{Args: f.superviseArgs(), Root: f.root, Engine: f.engine, Dispatcher: f.dispatcher})
	command := fakeSupervisorHelperCommand(config, f.environ()...)
	// Files, not pipes: an orphaned hold child keeps an inherited pipe
	// open, and Wait would block on its copy until the hold exits.
	output, err := os.Create(filepath.Join(f.root, "subprocess.out"))
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { output.Close() })
	command.Stdout = output
	command.Stderr = output
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		f.t.Fatal(err)
	}
	pid := command.Process.Pid
	f.t.Cleanup(func() {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		if command.ProcessState == nil {
			_ = command.Wait()
		}
		// The group includes holds that have not published their argv yet.
		testenv.Await(f.t, "the supervisor fixture group to exit", func() bool {
			return syscall.Kill(-pid, 0) == syscall.ESRCH
		})
	})
	return command
}

// subprocessOutput is the subprocess supervisor's combined output.
func (f *fakeInstall) subprocessOutput() string {
	data, _ := os.ReadFile(filepath.Join(f.root, "subprocess.out"))
	return string(data)
}

// signalOfExit returns the signal that ended a waited command, or 0.
func signalOfExit(err error) syscall.Signal {
	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			return status.Signal()
		}
	}
	return 0
}

// callNames lists the dispatcher callback verbs in order.
func callNames(calls [][]string) []string {
	var names []string
	for _, call := range calls {
		names = append(names, call[0])
	}
	return names
}

// casCall returns the first __record-cas call, or nil.
func casCall(calls [][]string) []string {
	for _, call := range calls {
		if call[0] == "__record-cas" {
			return call
		}
	}
	return nil
}
