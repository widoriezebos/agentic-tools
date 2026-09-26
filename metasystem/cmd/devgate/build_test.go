package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

const fixtureCommit = "0123456789abcdef0123456789abcdef01234567"

// buildFixture is one per-test world: a module directory, a stubbed Git that
// answers from fields, a stubbed Go toolchain that records every call and
// writes the -o file, and the real gate fence unless a test replaces it.
type buildFixture struct {
	t         *testing.T
	root      string
	env       map[string]string
	head      string
	headErr   error
	prefix    string
	changed   []string
	untracked []string
	gitCalls  [][]string
	goCalls   []goCall
	goFails   bool
	noGo      bool
	fenceHits int
	fence     func(string, int64) []gaterun.Holder
	selfPid   int64
	stdout    bytes.Buffer
	stderr    bytes.Buffer
}

type goCall struct {
	args []string
	env  []string
}

func newBuildFixture(t *testing.T) *buildFixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &buildFixture{
		t: t, root: root, env: map[string]string{}, head: fixtureCommit + "\n", prefix: "metasystem/\n",
		fence: gaterun.Fence, selfPid: int64(os.Getpid()),
	}
}

func nulList(names []string) []byte {
	var out []byte
	for _, name := range names {
		out = append(out, name...)
		out = append(out, 0)
	}
	return out
}

func (f *buildFixture) deps() deps {
	return deps{
		git: func(_ context.Context, root string, args ...string) ([]byte, error) {
			if root != f.root {
				f.t.Fatalf("git ran against %s, want %s", root, f.root)
			}
			f.gitCalls = append(f.gitCalls, args)
			switch strings.Join(args, " ") {
			case "rev-parse HEAD":
				return []byte(f.head), f.headErr
			case "rev-parse --show-prefix":
				return []byte(f.prefix), nil
			case "diff --name-only --no-renames -z HEAD --":
				return nulList(f.changed), nil
			case "ls-files --others --exclude-standard --full-name -z":
				return nulList(f.untracked), nil
			}
			f.t.Fatalf("unexpected git %q", args)
			return nil, nil
		},
		lookGo: func() error {
			if f.noGo {
				return errors.New("not found")
			}
			return nil
		},
		goTool: func(_ context.Context, root string, env, args []string, _, _ io.Writer) error {
			f.goCalls = append(f.goCalls, goCall{args: args, env: env})
			if f.goFails {
				return errors.New("exit status 1")
			}
			index := slices.Index(args, "-o")
			if index < 0 || index+1 >= len(args) {
				f.t.Fatalf("go %q has no -o", args)
			}
			out := args[index+1]
			if !filepath.IsAbs(out) {
				out = filepath.Join(root, out)
			}
			return os.WriteFile(out, []byte("engine "+strings.Join(args, " ")), 0o755)
		},
		fence: func(root string, selfPid int64) []gaterun.Holder {
			f.fenceHits++
			return f.fence(root, selfPid)
		},
		selfPid: f.selfPid,
		getenv:  func(key string) string { return f.env[key] },
		environ: func() []string { return []string{"PATH=/fixture/bin", "GOMAXPROCS=99"} },
		stdout:  &f.stdout,
		stderr:  &f.stderr,
	}
}

func (f *buildFixture) run(args ...string) int {
	return run(context.Background(), append([]string{"build"}, args...), f.root, f.deps())
}

func (f *buildFixture) installEngine() string {
	f.t.Helper()
	engine := filepath.Join(f.root, "bin", "metasystem")
	if err := os.MkdirAll(filepath.Dir(engine), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(engine, []byte("previous engine"), 0o755); err != nil {
		f.t.Fatal(err)
	}
	return engine
}

func (f *buildFixture) onlyGoCall() goCall {
	f.t.Helper()
	if len(f.goCalls) != 1 {
		f.t.Fatalf("go calls = %+v, want exactly one; stderr:\n%s", f.goCalls, f.stderr.String())
	}
	return f.goCalls[0]
}

func stampOf(args []string) string {
	index := slices.Index(args, "-ldflags")
	if index < 0 || index+1 >= len(args) {
		return ""
	}
	return strings.TrimPrefix(args[index+1], buildStampFlag)
}

func TestBuildCleanTreeInstallsEngineStampedWithHead(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	f.env["METASYSTEM_TEST_WORKERS"] = "3"
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, f.stderr.String())
	}
	call := f.onlyGoCall()
	staging := filepath.Join("bin", ".metasystem.build."+itoa(f.selfPid))
	want := []string{"build", "-p=3", "-buildvcs=false", "-ldflags", buildStampFlag + fixtureCommit, "-o", staging, "./cmd/metasystem"}
	if !slices.Equal(call.args, want) {
		t.Fatalf("go args = %q, want %q", call.args, want)
	}
	if !slices.Contains(call.env, "CGO_ENABLED=0") || call.env[len(call.env)-2] != "GOMAXPROCS=3" {
		t.Fatalf("go env = %q, want the inherited environment then GOMAXPROCS=3 and CGO_ENABLED=0", call.env)
	}
	data, err := os.ReadFile(filepath.Join(f.root, "bin", "metasystem"))
	if err != nil || !strings.Contains(string(data), fixtureCommit) {
		t.Fatalf("bin/metasystem = %q, %v; want the staged build renamed over it", data, err)
	}
	if _, err := os.Stat(filepath.Join(f.root, staging)); !os.IsNotExist(err) {
		t.Fatalf("staging file survived: %v", err)
	}
	if got := f.stdout.String(); got != "go-build: bin/metasystem @ "+fixtureCommit+" (CGO_ENABLED=0)\n" {
		t.Fatalf("stdout = %q", got)
	}
	if f.fenceHits != 0 {
		t.Fatalf("fence ran with no bin/metasystem present: %d", f.fenceHits)
	}
}

func TestBuildStampClassifiesChangesAgainstTheEngineProjection(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name      string
		changed   []string
		untracked []string
		want      string
	}{
		{name: "dirty engine path", changed: []string{"metasystem/internal/gaterun/fence.go"}, want: "dev-" + fixtureCommit + "-dirty"},
		{name: "untracked engine path", untracked: []string{"metasystem/cmd/devgate/new.go"}, want: "dev-" + fixtureCommit + "-dirty"},
		{name: "dirty non-engine path", changed: []string{"metasystem/plans/designs/verbs-object-action.md", "README.md"}, untracked: []string{"metasystem/records/note.md"}, want: fixtureCommit},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newBuildFixture(t)
			f.changed, f.untracked = test.changed, test.untracked
			if code := f.run("--out", "proof"); code != 0 {
				t.Fatalf("exit %d; stderr:\n%s", code, f.stderr.String())
			}
			if got := stampOf(f.onlyGoCall().args); got != test.want {
				t.Fatalf("stamp = %q, want %q", got, test.want)
			}
		})
	}
}

func TestBuildStampWithoutHeadIsUnknownAndOverrideSkipsGit(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	f.head, f.headErr = "", errors.New("not a git repository")
	if code := f.run("--out", "proof"); code != 0 || stampOf(f.onlyGoCall().args) != "unknown" {
		t.Fatalf("exit %d stamp %q, want unknown", code, stampOf(f.onlyGoCall().args))
	}

	override := newBuildFixture(t)
	override.env["METASYSTEM_BUILD_STAMP"] = "witness-abcdef012345"
	override.changed = []string{"metasystem/internal/dirty.go"}
	if code := override.run("--out", "proof"); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, override.stderr.String())
	}
	if got := stampOf(override.onlyGoCall().args); got != "witness-abcdef012345" || len(override.gitCalls) != 0 {
		t.Fatalf("stamp %q after git %q; want the override and no Git", got, override.gitCalls)
	}
	if got := override.stdout.String(); got != "go-build: proof engine @ witness-abcdef012345 (CGO_ENABLED=0); bin/metasystem untouched\n" {
		t.Fatalf("stdout = %q", got)
	}
}

func TestBuildOutLeavesTheInstalledEngineAndSkipsTheFence(t *testing.T) {
	t.Parallel()
	for _, trimpath := range []bool{false, true} {
		f := newBuildFixture(t)
		engine := f.installEngine()
		f.fence = func(string, int64) []gaterun.Holder { return []gaterun.Holder{{Pid: 1, Gate: "foreign"}} }
		out := filepath.Join(t.TempDir(), "proof-engine")
		args := []string{"--out", out}
		if trimpath {
			args = append([]string{"--trimpath"}, args...)
		}
		if code := f.run(args...); code != 0 {
			t.Fatalf("trimpath=%v exit %d; stderr:\n%s", trimpath, code, f.stderr.String())
		}
		want := []string{"build", "-p=1", "-buildvcs=false"}
		if trimpath {
			want = append(want, "-trimpath")
		}
		want = append(want, "-ldflags", buildStampFlag+fixtureCommit, "-o", out, "./cmd/metasystem")
		if got := f.onlyGoCall().args; !slices.Equal(got, want) {
			t.Fatalf("trimpath=%v go args = %q, want %q", trimpath, got, want)
		}
		if data, err := os.ReadFile(engine); err != nil || string(data) != "previous engine" {
			t.Fatalf("--out touched bin/metasystem: %q %v", data, err)
		}
		if _, err := os.Stat(out); err != nil {
			t.Fatalf("--out produced nothing: %v", err)
		}
		if f.fenceHits != 0 {
			t.Fatalf("--out consulted the fence")
		}
	}
}

func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	sleeper := exec.Command("sleep", "60")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = sleeper.Process.Kill()
		_ = sleeper.Wait()
	})
	return sleeper
}

func TestBuildFenceRefusesAForeignLiveGate(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	engine := f.installEngine()
	sleeper := startSleeper(t)
	if _, err := gaterun.Register(f.root, int64(sleeper.Process.Pid), "go-gate"); err != nil {
		t.Fatal(err)
	}
	if code := f.run(); code != 1 {
		t.Fatalf("exit %d, want 1; stderr:\n%s", code, f.stderr.String())
	}
	if len(f.goCalls) != 0 || len(f.gitCalls) != 0 {
		t.Fatalf("a refused build reached git %q or go %+v", f.gitCalls, f.goCalls)
	}
	if !strings.Contains(f.stderr.String(), "gate go-gate is running as pid "+itoa(int64(sleeper.Process.Pid))) ||
		!strings.Contains(f.stderr.String(), "go-build: a live gate run owns this checkout") {
		t.Fatalf("stderr does not name the holder and the refusal:\n%s", f.stderr.String())
	}
	if data, _ := os.ReadFile(engine); string(data) != "previous engine" {
		t.Fatalf("refused build replaced bin/metasystem")
	}

	allowed := newBuildFixture(t)
	allowed.root = f.root
	allowed.env["METASYSTEM_ALLOW_CONCURRENT_GATE"] = "1"
	if code := allowed.run(); code != 0 || allowed.fenceHits != 0 {
		t.Fatalf("override exit %d fence hits %d; stderr:\n%s", code, allowed.fenceHits, allowed.stderr.String())
	}
}

func TestBuildFenceExemptsItsOwnAncestryAndPrunesAStaleMarker(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	f.installEngine()
	// The gate that invoked the build: an ancestor of this process, as the
	// `go` process and its caller are ancestors of a `go run` build.
	if _, err := gaterun.Register(f.root, int64(os.Getppid()), "go-gate"); err != nil {
		t.Fatal(err)
	}
	// A gate that has exited since it registered.
	stale := exec.Command("sleep", "60")
	if err := stale.Start(); err != nil {
		t.Fatal(err)
	}
	marker, err := gaterun.Register(f.root, int64(stale.Process.Pid), "go-gate")
	if err != nil {
		t.Fatal(err)
	}
	_ = stale.Process.Kill()
	_ = stale.Wait()
	if code := f.run(); code != 0 {
		t.Fatalf("exit %d; stderr:\n%s", code, f.stderr.String())
	}
	if f.fenceHits != 1 || len(f.goCalls) != 1 {
		t.Fatalf("fence hits %d, go calls %d; want the fence consulted and the build run", f.fenceHits, len(f.goCalls))
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("stale marker survived the fence: %v", err)
	}
}

func TestBuildRefusalsBeforeAnyEffect(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name   string
		args   []string
		setup  func(*buildFixture)
		code   int
		stderr string
	}{
		{name: "out without path", args: []string{"--out"}, code: 2, stderr: "go-build: --out needs a path"},
		{name: "unknown argument", args: []string{"--fast"}, code: 2, stderr: "go-build: unknown argument: --fast"},
		{name: "invalid workers", setup: func(f *buildFixture) { f.env["METASYSTEM_TEST_WORKERS"] = "nope" }, code: 1, stderr: "METASYSTEM_TEST_WORKERS must be a positive integer"},
		{name: "zero workers", setup: func(f *buildFixture) { f.env["METASYSTEM_TEST_WORKERS"] = "0" }, code: 1, stderr: "METASYSTEM_TEST_WORKERS must be a positive integer"},
		{name: "no toolchain", setup: func(f *buildFixture) { f.noGo = true }, code: 1, stderr: "no go toolchain on PATH"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			f := newBuildFixture(t)
			if test.setup != nil {
				test.setup(f)
			}
			if code := f.run(test.args...); code != test.code || !strings.Contains(f.stderr.String(), test.stderr) {
				t.Fatalf("exit %d stderr %q, want %d containing %q", code, f.stderr.String(), test.code, test.stderr)
			}
			if len(f.goCalls) != 0 || len(f.gitCalls) != 0 {
				t.Fatalf("refusal reached git %q or go %+v", f.gitCalls, f.goCalls)
			}
		})
	}
}

func TestBuildFailureLeavesTheInstalledEngine(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	engine := f.installEngine()
	f.goFails = true
	if code := f.run(); code != 1 || !strings.Contains(f.stderr.String(), "go-build: build failed") {
		t.Fatalf("exit %d stderr %q", code, f.stderr.String())
	}
	if data, _ := os.ReadFile(engine); string(data) != "previous engine" {
		t.Fatalf("failed build replaced bin/metasystem")
	}
}

func TestUnknownActionIsAUsageError(t *testing.T) {
	t.Parallel()
	f := newBuildFixture(t)
	if code := run(context.Background(), []string{"gate"}, f.root, f.deps()); code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func itoa(value int64) string { return strconv.FormatInt(value, 10) }
