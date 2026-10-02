package testenv

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

// toolchainSlot is this test binary's one Go toolchain slot. Every go
// command a test runs (a build, a list, an env read) and every fixture that
// drives a real toolchain through the engine holds it, so the parallel tests
// of one package never run two toolchains at once: a full suite already runs
// -p packages times -parallel tests, and each toolchain adds hundreds of MB
// and CPU-seconds. Across package binaries the suite's -p bounds them.
var toolchainSlot = make(chan struct{}, 1)

// HoldToolchain takes this binary's toolchain slot for a fixture that drives
// a real toolchain itself (the engine building a candidate, a go gate running
// staticcheck) and returns its release. Nothing that takes the slot may run
// while it is held: the slot is not reentrant, so build engines first.
func HoldToolchain() (release func()) {
	toolchainSlot <- struct{}{}
	var once sync.Once
	return func() { once.Do(func() { <-toolchainSlot }) }
}

// GoCmd is one go toolchain command for a test. Run, Output and
// CombinedOutput hold the binary's toolchain slot for the command's life.
type GoCmd struct{ *exec.Cmd }

// Go is exec.Command("go", args...) under the toolchain slot.
func Go(args ...string) *GoCmd { return &GoCmd{exec.Command("go", args...)} }

// GoContext is exec.CommandContext(ctx, "go", args...) under the toolchain
// slot.
func GoContext(ctx context.Context, args ...string) *GoCmd {
	return &GoCmd{exec.CommandContext(ctx, "go", args...)}
}

// Run runs the command while it holds the toolchain slot.
func (command *GoCmd) Run() error {
	defer HoldToolchain()()
	return command.Cmd.Run()
}

// Output runs the command while it holds the toolchain slot.
func (command *GoCmd) Output() ([]byte, error) {
	defer HoldToolchain()()
	return command.Cmd.Output()
}

// CombinedOutput runs the command while it holds the toolchain slot.
func (command *GoCmd) CombinedOutput() ([]byte, error) {
	defer HoldToolchain()()
	return command.Cmd.CombinedOutput()
}

type sharedBuild struct {
	once sync.Once
	path string
	err  error
}

var sharedBuilds struct {
	sync.Mutex
	byPackage map[string]*sharedBuild
}

// Engine is this module's cmd/metasystem built once per test binary, for
// tests that need the real engine as a separate executable. See Built.
func Engine(t testing.TB) string {
	t.Helper()
	return Built(t, "./cmd/metasystem")
}

// Built is the module package pkg (a module-relative path such as
// ./cmd/devgate) built once per test binary. Every caller shares the one
// file: a caller that needs it at a path of its own links it there (Link),
// and none changes it in place. The build is `go build -buildvcs=false` from
// the module root under the toolchain slot: the engine reads no VCS stamp,
// and a fixture that needs a stamped build, other flags or other sources
// builds its own through Go.
func Built(t testing.TB, pkg string) string {
	t.Helper()
	sharedBuilds.Lock()
	if sharedBuilds.byPackage == nil {
		sharedBuilds.byPackage = map[string]*sharedBuild{}
	}
	build := sharedBuilds.byPackage[pkg]
	if build == nil {
		build = &sharedBuild{}
		sharedBuilds.byPackage[pkg] = build
	}
	sharedBuilds.Unlock()
	build.once.Do(func() { build.path, build.err = buildShared(pkg) })
	if build.err != nil {
		t.Fatalf("build the shared test executable %s: %v", pkg, build.err)
	}
	return build.path
}

// Link places a shared build at destination as a hard link: no bytes are
// copied and no writable descriptor exists for a concurrent fork to inherit.
// The destination's directory must exist and share the build's file system
// (any directory under the test binary's temporary namespace does).
func Link(t testing.TB, built, destination string) string {
	t.Helper()
	if err := os.Link(built, destination); err != nil {
		t.Fatalf("link the shared test executable to %s: %v", destination, err)
	}
	return destination
}

func buildShared(pkg string) (string, error) {
	module, err := moduleRoot()
	if err != nil {
		return "", err
	}
	directory, err := MkdirTemp("shared-build-")
	if err != nil {
		return "", err
	}
	output := filepath.Join(directory, filepath.Base(filepath.FromSlash(pkg)))
	build := Go("build", "-buildvcs=false", "-o", output, pkg)
	build.Dir = module
	if combined, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("%w\n%s", err, combined)
	}
	return output, nil
}

// moduleRoot is the nearest directory at or above the test's working
// directory that holds go.mod and cmd/metasystem: the engine's module.
func moduleRoot() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
			if info, err := os.Stat(filepath.Join(directory, "cmd", "metasystem")); err == nil && info.IsDir() {
				return directory, nil
			}
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("no engine module above the test's working directory")
		}
		directory = parent
	}
}
