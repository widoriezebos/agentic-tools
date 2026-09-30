// Command devgate is the Go bootstrap of the development gate: the build,
// static and gate actions that must run from the tree they build without an
// installed engine. It is run as `go run ./cmd/devgate ACTION` from the
// metasystem module directory, so it never recurses into the test scheduler
// and never needs bin/metasystem to exist.
//
// Actions:
//
//   - build: the one fenced, pinned, stamped engine build (go-production-grade
//     Phase 0a); the go-build.sh stub that named it is gone (verbs-object-action
//     U7c). The gate and adoption both build through here, so no second unfenced path
//     can swap bin/metasystem under a live gate run.
//   - static: the fast static/build leaf the fast-static-build group runs
//     (dependency and parallel ratchets, gofmt, the bash -n shell parse, vet,
//     staticcheck, the refusal
//     register, the SessionStart exit audit, the Stop decision surface audit,
//     the project check, and the build). It replaced `go-gate.sh --fast`.
//   - gate: the full Go gate (cross-builds, govulncheck, the native race and
//     coverage selection, the coverage ratchet, the boundary-scoped witness).
//     It replaced `go-gate.sh` and, with --arm, the sourced witness-gate.sh.
//
// Every action is a leaf: none of them schedules `metasystem test run`
// (VOA-09), so a testing group that runs one cannot recurse.
package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/cachedomain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/diskstore"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

func main() {
	// `go run` places $GOROOT/bin at the beginning of its child's PATH. The
	// gate resolves gofmt, go and every other tool from its caller's PATH, so
	// a caller that puts a tool first (the fail-open tripwire's broken gofmt)
	// is heard.
	_ = os.Setenv("PATH", callerPath(os.Getenv("PATH")))
	root, err := os.Getwd()
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "devgate: cannot resolve the module directory: %v\n", err)
		os.Exit(1)
	}
	code := run(context.Background(), os.Args[1:], root, nativeDeps())
	// The gate's own scratch root, if it made one, is released before the
	// exit code is returned (Part B 3.2 "Process"); one still in use stays
	// for the sweeper.
	_ = diskstore.ReleaseProcessScratch(context.Background())
	os.Exit(code)
}

func run(ctx context.Context, args []string, root string, deps deps) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.stderr, usage)
		return 2
	}
	switch args[0] {
	case "build":
		return runBuild(ctx, args[1:], root, deps)
	case "static":
		return runStatic(ctx, args[1:], root, deps)
	case "gate":
		return runGateAction(ctx, args[1:], root, deps)
	default:
		fmt.Fprintf(deps.stderr, "devgate: unknown action %q; %s\n", args[0], usage)
		return 2
	}
}

const usage = "usage: go run ./cmd/devgate build [--out PATH] [--trimpath] | static [--proof-out PATH] | gate [--goal G] [--cap-min N] [--witness-check-only] [--arm plain|none --controller-pid PID --state-out FILE [--delivery]]"

// deps is every effect the build has on the world, so a test drives the
// whole action against stubbed Git, a stubbed Go toolchain and a chosen fence.
type deps struct {
	// git runs `git -C root ARGS...` and returns its standard output.
	git func(ctx context.Context, root string, args ...string) ([]byte, error)
	// lookGo reports whether a go toolchain is on PATH.
	lookGo func() error
	// goTool runs `go ARGS...` in root with env, streaming to stdout/stderr.
	goTool func(ctx context.Context, root string, env, args []string, stdout, stderr io.Writer) error
	// fence answers the live gate runs foreign to selfPid's process chain.
	fence func(root string, selfPid int64) []gaterun.Holder
	// selfPid is this process: its ancestry includes the `go run` process and
	// whatever invoked it, so a gate that runs the build never fences itself.
	selfPid int64
	getenv  func(string) string
	environ func() []string
	stdout  io.Writer
	stderr  io.Writer

	// tool runs one child program (gofmt, the trusted authentication
	// engine, the temporary proof launcher) and returns its exit error.
	tool func(ctx context.Context, call toolCall) error
	// now is the gate's clock for evidence names and witness stamps.
	now func() time.Time
	// owners are the in-process owners the static and full gates consult.
	owners owners
	// cacheDomain decides the build's cache pair and the context its
	// children inherit (cachedomain.Resolve, disk-lifetimes A8).
	cacheDomain func(environ []string, installationRoot string) (gocache.Resolution, error)
}

// toolCall is one child program run from dir with an explicit environment.
type toolCall struct {
	dir    string
	env    []string
	name   string
	args   []string
	stdout io.Writer
	stderr io.Writer
}

func nativeDeps() deps {
	return deps{
		git: func(ctx context.Context, root string, args ...string) ([]byte, error) {
			output, err := exec.CommandContext(ctx, "git", append([]string{"-C", root}, args...)...).Output()
			if exit, ok := err.(*exec.ExitError); ok && len(exit.Stderr) > 0 {
				err = fmt.Errorf("%w: %s", err, bytes.TrimSpace(exit.Stderr))
			}
			return output, err
		},
		lookGo: func() error {
			_, err := exec.LookPath("go")
			return err
		},
		goTool: func(ctx context.Context, root string, env, args []string, stdout, stderr io.Writer) error {
			command := exec.CommandContext(ctx, "go", args...)
			command.Dir = root
			command.Env = env
			command.Stdout, command.Stderr = stdout, stderr
			return command.Run()
		},
		fence:   gaterun.Fence,
		selfPid: int64(os.Getpid()),
		getenv:  os.Getenv,
		environ: os.Environ,
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		tool: func(ctx context.Context, call toolCall) error {
			command := exec.CommandContext(ctx, call.name, call.args...)
			command.Dir = call.dir
			command.Env = call.env
			command.Stdout, command.Stderr = call.stdout, call.stderr
			return command.Run()
		},
		now:         time.Now,
		owners:      nativeOwners(),
		cacheDomain: cachedomain.Resolve,
	}
}

// callerPath is PATH without the leading $GOROOT/bin entry `go run` adds.
func callerPath(path string) string {
	first, rest, found := strings.Cut(path, string(os.PathListSeparator))
	if !found || !goRootBin(first) {
		return path
	}
	return rest
}

// goRootBin reports a Go installation's bin directory: go and gofmt beside
// a sibling pkg/tool.
func goRootBin(dir string) bool {
	if filepath.Base(dir) != "bin" || !executableFile(filepath.Join(dir, "go")) || !executableFile(filepath.Join(dir, "gofmt")) {
		return false
	}
	info, err := os.Stat(filepath.Join(filepath.Dir(dir), "pkg", "tool"))
	return err == nil && info.IsDir()
}

// withEnvironment is d with its environment reads bound to env, so the build
// and child tools a gate stage runs see that stage's environment rather than
// the process's.
func (d deps) withEnvironment(env *environment) deps {
	d.getenv = env.get
	d.environ = env.list
	return d
}

// exitStatus is a child's exit status; a child that could not start is 127.
func exitStatus(err error) int {
	if err == nil {
		return 0
	}
	var coded interface{ ExitCode() int }
	if errors.As(err, &coded) {
		if coded.ExitCode() > 0 {
			return coded.ExitCode()
		}
		return 1
	}
	return 127
}
