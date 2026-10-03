package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/behaviorsurface"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testexec"
)

const fixtureModule = "github.com/widoriezebos/agentic-tools/metasystem"

// gateWorld is one test's gate installation: a module tree outside any Git
// repository, a stubbed Go toolchain, gofmt, Git and trusted engine that
// record every call, and the real owners for everything a stub cannot prove
// (the behavior-surface policy, the frozen export and its manifest, process
// ancestry and the gate markers). Stubbed owners are the audits, the native
// test selection and the proof-attempt coverage custody.
type gateWorld struct {
	t    *testing.T
	root string
	tmp  string
	env  []string

	gitInside bool
	gitFiles  []string

	// statuses maps a stage key to the exit status its stub returns;
	// outputs maps it to the text the stub prints.
	statuses map[string]int
	outputs  map[string]string

	mu       sync.Mutex
	calls    []worldCall
	gofmtDir []string

	// buildHook runs after a stubbed `go build -o` writes its output.
	buildHook func(root, out string)

	owners owners

	nativeOutput string
	nativeCalls  []proofrun.GoGateTestRequest
	coverage     []string
	// nativeNoLog leaves the native log unwritten; nativeReruns are the
	// diagnostic reruns the selection reports; failBuildAfterNative fails
	// every -o build once the native selection has run.
	nativeNoLog          bool
	nativeReruns         []proofrun.RerunFinding
	failBuildAfterNative bool
	hostLeases           []string
	hostReleases         int

	stdout bytes.Buffer
	stderr bytes.Buffer
}

type worldCall struct {
	dir  string
	name string
	args []string
	env  []string
}

func (c worldCall) String() string { return c.name + " " + strings.Join(c.args, " ") }

func newGateWorld(t *testing.T) *gateWorld {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w := &gateWorld{t: t, root: filepath.Join(base, "metasystem"), tmp: filepath.Join(base, "tmp"),
		statuses: map[string]int{}, outputs: map[string]string{}}
	for _, dir := range []string{w.tmp, filepath.Join(w.root, "internal", "fixture"), filepath.Join(w.root, "cmd", "metasystem"),
		filepath.Join(w.root, "docs"), filepath.Join(w.root, "bin"), filepath.Join(w.root, "scripts", "agents")} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	w.write("go.mod", "module "+fixtureModule+"\n\ngo 1.24\n")
	w.write("metasystem.conf", "metasystem.runtimes=fake\n")
	w.write("wow.md", "# ways of working\n")
	w.write("internal/fixture/fixture.go", "package fixture\n")
	w.write("cmd/metasystem/main.go", "package main\n")
	w.write("docs/payload.md", "payload baseline\n")
	w.write("testing-coverage-floors.json", `{"floors":{"internal/fixture":50.0},"exempt":{}}`+"\n")
	w.write("testing-coverage-floors-linux.json", `{"floors":{"internal/fixture":50.0},"exempt":{}}`+"\n")
	w.writeMode("bin/metasystem", "#!/bin/sh\nexit 0\n", 0o755)
	// The trusted authentication engine a proof launcher hands its worker;
	// the stubbed tool answers its worker-authorized question.
	auth := filepath.Join(base, "auth-engine")
	if err := testexec.WriteFile(auth, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	w.env = []string{"PATH=/fixture/bin", "HOME=" + base, "TMPDIR=" + w.tmp, "METASYSTEM_TEST_WORKERS=3", "METASYSTEM_PROOF_AUTH_BIN=" + auth}
	w.nativeOutput = "ok  \t" + fixtureModule + "/internal/fixture\t0.1s\tcoverage: 85.0% of statements\n"
	// Every full gate in this world runs as an authenticated proof worker
	// unless a test says otherwise; a standalone run relaunches.
	w.statuses["worker"] = 0
	w.statuses["candidate-worker"] = 1
	w.owners = nativeOwners()
	pass := func(string) (string, bool) { return "", true }
	w.owners.dependencyRatchet = func(string) (string, bool) { return "dependency ratchet passed\n", true }
	w.owners.parallelRatchet = func(string) (string, bool) { return "parallel ratchet passed\n", true }
	w.owners.rootAuditRatchet = func(string) (string, bool) { return "run-state audit passed: 0 open site(s)\n", true }
	w.owners.hookStartExits = pass
	w.owners.stopSurface = func(string) (string, bool) {
		return "stop decision surface: fixture; added 0, moved 0, removed 0\n", true
	}
	w.owners.projectCheck = pass
	w.owners.hostResources = func(ctx context.Context, controlRoot string) (func(), context.Context, error) {
		w.mu.Lock()
		w.hostLeases = append(w.hostLeases, controlRoot)
		w.mu.Unlock()
		return func() {
			w.mu.Lock()
			w.hostReleases++
			w.mu.Unlock()
		}, ctx, nil
	}
	w.owners.goGateTests = func(_ context.Context, request proofrun.GoGateTestRequest) (proofrun.GoGateTestResult, int, error) {
		w.mu.Lock()
		w.nativeCalls = append(w.nativeCalls, request)
		w.mu.Unlock()
		// The native custodians run as the candidate engine the static stage
		// built, so that build must still exist.
		if scratch, _ := filepath.Glob(filepath.Join(w.tmp, "metasystem-gate-collect.*")); len(scratch) != 1 {
			w.t.Errorf("native selection ran without the collected candidate engine: %v", scratch)
		}
		if err := os.MkdirAll(request.LogRoot, 0o700); err != nil {
			return proofrun.GoGateTestResult{}, 1, err
		}
		logPath := filepath.Join(request.LogRoot, "go-gate-native.log")
		if !w.nativeNoLog {
			if err := os.WriteFile(logPath, []byte("native census and diagnostic\n"), 0o600); err != nil {
				return proofrun.GoGateTestResult{}, 1, err
			}
		}
		if status := w.statuses["native"]; status != 0 {
			return proofrun.GoGateTestResult{LogPath: logPath, Output: []byte("native red output\n"), Reruns: w.nativeReruns}, status, errors.New("native selection failed")
		}
		return proofrun.GoGateTestResult{LogPath: logPath, Output: []byte(w.nativeOutput), Reruns: w.nativeReruns}, 0, nil
	}
	w.owners.coverageEligible = func(options proofrun.CoverageBeginOptions) (bool, error) {
		w.record("coverage-eligible", options)
		if w.statuses["coverage-eligible"] == 3 {
			return false, nil
		}
		if w.statuses["coverage-eligible"] != 0 {
			return false, errors.New("unauthenticated")
		}
		return true, nil
	}
	w.owners.coverageBegin = func(options proofrun.CoverageBeginOptions) error {
		w.record("coverage-begin", options)
		if w.statuses["coverage-begin"] != 0 {
			return errors.New("slot owned")
		}
		return nil
	}
	w.owners.coverageComplete = func(options proofrun.CoverageCompleteOptions) (proofrun.CoverageEvidence, error) {
		w.record("coverage-complete", options.CoverageBeginOptions)
		if w.statuses["coverage-complete"] != 0 {
			return proofrun.CoverageEvidence{}, errors.New("publication refused")
		}
		return proofrun.CoverageEvidence{AttemptID: options.AttemptID}, nil
	}
	return w
}

func (w *gateWorld) record(what string, options proofrun.CoverageBeginOptions) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.coverage = append(w.coverage, fmt.Sprintf("%s producer=%d caller=%d attempt=%s baseline=%s root=%s",
		what, options.ProducerPID, options.CallerPID, options.AttemptID, filepath.Base(options.BaselinePath), options.ExecutionRoot))
}

func (w *gateWorld) write(rel, content string) { w.writeMode(rel, content, 0o644) }

func (w *gateWorld) writeMode(rel, content string, mode os.FileMode) {
	w.t.Helper()
	path := filepath.Join(w.root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		w.t.Fatal(err)
	}
	if err := testexec.WriteFile(path, []byte(content), mode); err != nil {
		w.t.Fatal(err)
	}
}

func (w *gateWorld) setenv(pairs ...string) {
	for _, pair := range pairs {
		name, _, _ := strings.Cut(pair, "=")
		w.env = slices.DeleteFunc(w.env, func(entry string) bool { return strings.HasPrefix(entry, name+"=") })
		w.env = append(w.env, pair)
	}
}

func (w *gateWorld) note(call worldCall) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, call)
}

// called reports the recorded calls whose rendering contains fragment.
func (w *gateWorld) called(fragment string) []worldCall {
	w.mu.Lock()
	defer w.mu.Unlock()
	var found []worldCall
	for _, call := range w.calls {
		if strings.Contains(call.String(), fragment) {
			found = append(found, call)
		}
	}
	return found
}

func envValue(env []string, name string) string {
	value := ""
	for _, entry := range env {
		if strings.HasPrefix(entry, name+"=") {
			value = entry[len(name)+1:]
		}
	}
	return value
}

func (w *gateWorld) respond(key string, stdout io.Writer) error {
	if out := w.outputs[key]; out != "" && stdout != nil {
		_, _ = io.WriteString(stdout, out)
	}
	if status := w.statuses[key]; status != 0 {
		return exitCode(status)
	}
	return nil
}

// exitCode is a stub child's exit status.
type exitCode int

func (e exitCode) Error() string { return "exit status " + strconv.Itoa(int(e)) }
func (e exitCode) ExitCode() int { return int(e) }

func (w *gateWorld) deps() deps {
	return deps{
		git: func(_ context.Context, root string, args ...string) ([]byte, error) {
			w.note(worldCall{dir: root, name: "git", args: args})
			joined := strings.Join(args, " ")
			switch {
			case joined == "rev-parse --is-inside-work-tree":
				if !w.gitInside {
					return nil, errors.New("not a git repository")
				}
				return []byte("true\n"), nil
			case strings.HasPrefix(joined, "ls-files -z --cached --others --exclude-standard -- "):
				pattern := args[len(args)-1]
				return nulList(slices.DeleteFunc(slices.Clone(w.gitFiles), func(name string) bool {
					matched, _ := filepath.Match(pattern, filepath.Base(name))
					return !matched
				})), nil
			case joined == "rev-parse HEAD":
				if !w.gitInside {
					return nil, errors.New("not a git repository")
				}
				return []byte(fixtureCommit + "\n"), nil
			case joined == "rev-parse --short HEAD":
				if !w.gitInside {
					return nil, errors.New("not a git repository")
				}
				return []byte(fixtureCommit[:9] + "\n"), nil
			case joined == "rev-parse --show-prefix":
				return []byte("\n"), nil
			case strings.HasPrefix(joined, "diff ") || strings.HasPrefix(joined, "ls-files --others"):
				return nil, nil
			}
			w.t.Errorf("unexpected git %q", args)
			return nil, errors.New("unexpected git")
		},
		lookGo: func() error { return nil },
		goTool: func(_ context.Context, root string, env, args []string, stdout, stderr io.Writer) error {
			w.note(worldCall{dir: root, name: "go", args: args, env: env})
			if stdout == nil {
				stdout = io.Discard
			}
			switch args[0] {
			case "version":
				_, _ = io.WriteString(stdout, "go version go1.fixture darwin/arm64\n")
				return nil
			case "env":
				if len(args) == 2 && args[1] == "GOTOOLCHAIN" {
					_, _ = io.WriteString(stdout, "auto\n")
					return nil
				}
				_, _ = io.WriteString(stdout, "darwin\narm64\n-mod=readonly -trimpath\noff\n\n0\nauto\n")
				return nil
			case "vet":
				return w.respond("vet", stdout)
			case "install":
				return w.respond("deadcode-install", stdout)
			case "run":
				switch {
				case slices.Contains(args, staticcheckModule):
					return w.respond("staticcheck", stdout)
				case slices.Contains(args, govulncheckModule):
					return w.respond("govulncheck", stdout)

				}
			case "test":
				return w.respond("refusal", stdout)
			case "list":
				if slices.Contains(args, "-f") {
					dirs := w.outputs["list-dirs"]
					if dirs == "" {
						dirs = root + "/internal/fixture\n" + root + "/cmd/metasystem\n"
					}
					_, _ = io.WriteString(stdout, dirs)
					return nil
				}
				_, _ = io.WriteString(stdout, fixtureModule+"/internal/fixture\n")
				return w.respond("list", nil)
			case "build":
				index := slices.Index(args, "-o")
				if index < 0 {
					return w.respond(envValue(env, "GOOS")+"/"+envValue(env, "GOARCH"), stdout)
				}
				if status := w.statuses["build"]; status != 0 {
					return exitCode(status)
				}
				w.mu.Lock()
				nativeRan := len(w.nativeCalls) > 0
				w.mu.Unlock()
				if w.failBuildAfterNative && nativeRan {
					return exitCode(1)
				}
				out := args[index+1]
				if !filepath.IsAbs(out) {
					out = filepath.Join(root, out)
				}
				engine := "engine built in " + root + " stamp " + stampOf(args) + "\n"
				if err := testexec.WriteFile(out, []byte(engine), 0o755); err != nil {
					return err
				}
				if w.buildHook != nil {
					w.buildHook(root, out)
				}
				return nil
			}
			w.t.Errorf("unexpected go %q", args)
			return errors.New("unexpected go")
		},
		tool: func(_ context.Context, call toolCall) error {
			w.note(worldCall{dir: call.dir, name: call.name, args: call.args, env: call.env})
			switch {
			case filepath.Base(call.name) == "deadcode":
				return w.respond("deadcode-"+envValue(call.env, "GOOS"), call.stdout)
			case call.name == "gofmt":
				w.mu.Lock()
				w.gofmtDir = append(w.gofmtDir, call.dir)
				w.mu.Unlock()
				return w.respond("gofmt", call.stdout)
			case call.name == "bash":
				// The shell parse is judged by the real bash: the claim is
				// that an unparsable script refuses the gate.
				if _, stubbed := w.statuses["bash"]; stubbed {
					return w.respond("bash", call.stdout)
				}
				command := exec.Command("bash", call.args...)
				command.Dir = call.dir
				command.Stdout, command.Stderr = call.stdout, call.stderr
				return command.Run()
			case call.name == "env" && slices.Contains(call.args, "worker-authorized"):
				return w.respond("candidate-worker", call.stdout)
			case slices.Contains(call.args, "worker-authorized"):
				return w.respond("worker", call.stdout)
			case len(call.args) > 2 && call.args[1] == "proof-run" && call.args[2] == "banner":
				_, _ = io.WriteString(call.stdout, "suite-cost suite=go-gate witness=unarmed duration=full-gate\n")
				return w.respond("banner", nil)
			case len(call.args) > 2 && call.args[1] == "proof-run" && call.args[2] == "launch":
				return w.respond("launch", call.stdout)
			}
			w.t.Errorf("unexpected tool %s %q", call.name, call.args)
			return errors.New("unexpected tool")
		},
		fence:   defaultFence,
		selfPid: int64(os.Getpid()),
		getenv:  func(name string) string { return envValue(w.env, name) },
		environ: func() []string { return append([]string(nil), w.env...) },
		stdout:  &w.stdout,
		stderr:  &w.stderr,
		now:     func() time.Time { return time.Date(2026, 9, 27, 4, 30, 0, 0, time.UTC) },
		owners:  w.owners,
	}
}

var defaultFence = nativeDeps().fence

func (w *gateWorld) static(args ...string) int {
	return run(context.Background(), append([]string{"static"}, args...), w.root, w.deps())
}

func (w *gateWorld) gate(args ...string) int {
	return run(context.Background(), append([]string{"gate"}, args...), w.root, w.deps())
}

func (w *gateWorld) gateAt(root string, args ...string) int {
	return run(context.Background(), append([]string{"gate"}, args...), root, w.deps())
}

func (w *gateWorld) output() string { return w.stdout.String() + w.stderr.String() }

// reachedFullProof reports that the gate ran the proof a witness would have
// omitted: every refusal world breaks gofmt, so an accidental acceptance
// returns green while the lawful full fallback fails right here.
func (w *gateWorld) reachedFullProof(code int) bool {
	return code == 1 && strings.Contains(w.stderr.String(), "gofmt itself failed (status 79)")
}

// payloadDigests is the real policy's projections of a tree.
func payloadDigests(t *testing.T, tree string, manifest string) (int, string, string) {
	t.Helper()
	policy, err := behaviorsurface.Load()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := policy.ListPaths(tree, behaviorsurface.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var list bytes.Buffer
	for _, path := range paths {
		list.WriteString(path)
		list.WriteByte(0)
	}
	if err := os.WriteFile(manifest, list.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	engine, err := policy.DigestWithPrefix(tree, behaviorsurface.Engine, "")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := policy.DigestListed(tree, behaviorsurface.Payload, paths)
	if err != nil {
		t.Fatal(err)
	}
	return policy.Version, engine, payload
}

// witness is one controller's published witness for a tree.
type witness struct {
	path, root, run string
}

// writeWitness records what a controller would have proven for tree: the
// complete-project manifest of a private freeze, the projections, the
// toolchain identity this world's Go reports, and the controller's exact
// start identity.
func (w *gateWorld) writeWitness(name, tree string, controller int64) witness {
	w.t.Helper()
	state := filepath.Join(w.tmp, "witness-"+name)
	if err := os.MkdirAll(state, 0o700); err != nil {
		w.t.Fatal(err)
	}
	if err := os.Chmod(state, 0o700); err != nil {
		w.t.Fatal(err)
	}
	frozen, err := proofrun.Freeze(tree)
	if err != nil {
		w.t.Fatal(err)
	}
	if err := proofrun.CleanupFrozenExport(frozen.SnapshotRoot); err != nil {
		w.t.Fatal(err)
	}
	version, engine, payload := payloadDigests(w.t, tree, filepath.Join(state, "payload-paths.nul"))
	g := &gateRun{ctx: context.Background(), root: tree, env: newEnvironment(w.env), d: w.deps(), workers: "3"}
	toolchain, err := g.toolchainIdentity()
	if err != nil {
		w.t.Fatal(err)
	}
	ref, err := processPair(controller)
	if err != nil {
		w.t.Fatal(err)
	}
	run := "run-" + name
	content := fmt.Sprintf(`{"policyVersion":%d,"engineDigest":"%s","manifestDigest":"%s","payloadDigest":"%s","payloadManifest":"payload-paths.nul","toolchainIdentity":"%s","runId":"%s","controller":{"pid":%d,"startedAtSec":%d,"startTicks":%d,"bootId":"%s"}}`+"\n",
		version, engine, frozen.Digest, payload, toolchain, run, ref.Pid, ref.StartedAtSec, ref.StartTicks, ref.BootID)
	path := filepath.Join(state, "witness.json")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		w.t.Fatal(err)
	}
	return witness{path: path, root: state, run: run}
}

func (wit witness) env(scope string) []string {
	pairs := []string{"METASYSTEM_GATE_WITNESS=" + wit.path, "METASYSTEM_GATE_WITNESS_ROOT=" + wit.root, "METASYSTEM_GATE_WITNESS_RUN=" + wit.run}
	if scope != "" {
		pairs = append(pairs, "METASYSTEM_GATE_WITNESS_CONSUMER_SCOPE="+scope)
	}
	return pairs
}

// editWitness rewrites the witness text, keeping it 0600.
func editWitness(t *testing.T, path string, edit func(string) string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(edit(string(data))), 0o600); err != nil {
		t.Fatal(err)
	}
}

// copyTree copies a frozen export to a consumer directory byte for byte.
func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.Walk(from, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm()|0o700)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	})
	if err != nil {
		t.Fatal(err)
	}
}

// foreignController is a live process that is not this test's ancestor.
func foreignController(t *testing.T) int64 {
	t.Helper()
	sleeper := exec.Command("sleep", "60")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sleeper.Process.Kill()
		_ = sleeper.Wait()
	})
	return int64(sleeper.Process.Pid)
}
