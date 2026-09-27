// Command devgate is the Go bootstrap of the development gate: the build,
// static and gate actions that must run from the tree they build without an
// installed engine. It is run as `go run ./cmd/devgate ACTION` from the
// metasystem module directory, so it never recurses into the test scheduler
// and never needs bin/metasystem to exist.
//
// Its one action today is `build`, the one fenced, pinned, stamped engine
// build (go-production-grade Phase 0a), which replaced the body of
// scripts/agents/go-build.sh: the gate and adoption both build through here,
// so no second unfenced path can swap bin/metasystem under a live gate run.
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
)

func main() {
	root, err := os.Getwd()
	if err == nil {
		root, err = filepath.EvalSymlinks(root)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "devgate: cannot resolve the module directory: %v\n", err)
		os.Exit(1)
	}
	os.Exit(run(context.Background(), os.Args[1:], root, nativeDeps()))
}

func run(ctx context.Context, args []string, root string, deps deps) int {
	if len(args) == 0 {
		fmt.Fprintln(deps.stderr, "usage: go run ./cmd/devgate build [--out PATH] [--trimpath]")
		return 2
	}
	switch args[0] {
	case "build":
		return runBuild(ctx, args[1:], root, deps)
	default:
		fmt.Fprintf(deps.stderr, "devgate: unknown action %q; usage: go run ./cmd/devgate build [--out PATH] [--trimpath]\n", args[0])
		return 2
	}
}

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
	}
}
