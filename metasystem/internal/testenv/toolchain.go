package testenv

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// toolchainSlot is this test binary's one Go toolchain slot. Every go
// command a test runs (a build, a list, an env read) and every fixture that
// drives a real toolchain through the engine holds it, so the parallel tests
// of one package never run two toolchains at once: a full suite already runs
// -p packages times -parallel tests, and each toolchain adds hundreds of MB
// and CPU-seconds. Across package binaries the suite's -p bounds them.
var toolchainSlot = make(chan struct{}, 1)
var toolchainTempRoot = filepath.Clean(os.TempDir())

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
type GoCmd struct {
	*exec.Cmd
	ctx context.Context
}

// Go is exec.Command("go", args...) under the toolchain slot.
func Go(args ...string) *GoCmd { return GoContext(context.Background(), args...) }

// GoContext is exec.CommandContext(ctx, "go", args...) under the toolchain
// slot.
func GoContext(ctx context.Context, args ...string) *GoCmd {
	return &GoCmd{Cmd: exec.CommandContext(ctx, "go", args...), ctx: ctx}
}

// Run runs the command while it holds the toolchain slot.
func (command *GoCmd) Run() error {
	_, err := command.execute(func() ([]byte, error) { return nil, command.Cmd.Run() })
	return err
}

// Output runs the command while it holds the toolchain slot.
func (command *GoCmd) Output() ([]byte, error) { return command.execute(command.Cmd.Output) }

// CombinedOutput runs the command while it holds the toolchain slot.
func (command *GoCmd) CombinedOutput() ([]byte, error) {
	return command.execute(command.Cmd.CombinedOutput)
}

func (command *GoCmd) execute(run func() ([]byte, error)) ([]byte, error) {
	select {
	case toolchainSlot <- struct{}{}:
	case <-command.ctx.Done():
		return nil, command.ctx.Err()
	}
	defer func() { <-toolchainSlot }()
	if len(command.Args) > 1 && command.Args[1] == "build" {
		release, err := buildLock(command.ctx, command)
		if err != nil {
			return nil, fmt.Errorf("build lock: %w", err)
		}
		defer release()
	}
	return run()
}

// buildLock's record describes the holder; only flock establishes exclusion.
func buildLock(ctx context.Context, variant *GoCmd) (release func(), err error) {
	caches, err := gocache.Resolve(variant.Environ())
	if err != nil {
		return nil, err
	}
	module, err := moduleRoot(variant.Dir)
	if err != nil {
		return nil, err
	}
	directory, _ := filepath.Abs(variant.Dir) // moduleRoot resolved the same directory above.
	args := []string{module}
	for i, arg := range variant.Args[1:] {
		if arg == "-o" || variant.Args[i] == "-o" || strings.HasPrefix(arg, "-o=") {
			continue
		}
		if strings.HasPrefix(arg, ".") {
			path, _ := filepath.Rel(module, filepath.Join(directory, arg))
			arg = "./" + filepath.ToSlash(path)
		}
		for _, field := range strings.Fields(arg) {
			value := field[strings.Index(field, "=")+1:]
			if strings.HasPrefix(value, toolchainTempRoot+string(filepath.Separator)) || strings.HasPrefix(value, os.TempDir()+string(filepath.Separator)) || strings.Contains(value, string(filepath.Separator)+processNamespacePrefix) {
				arg = strings.ReplaceAll(arg, value, "<temporary>")
			}
		}
		args = append(args, arg)
	}
	key, _ := json.Marshal(args)
	path := filepath.Join(caches.GoCache, fmt.Sprintf("metasystem-build-%x.lock", sha256.Sum256(key)))
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = unlockAndClose(file)
		}
	}()
	for err = ctx.Err(); err == nil; err = ctx.Err() {
		err = unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		if err != unix.EWOULDBLOCK && err != unix.EINTR {
			break
		}
		select {
		case <-ctx.Done():
		case <-time.After(10 * time.Millisecond):
		}
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	self, _, err := (identity.KernelProber{}).ReadStart(int64(os.Getpid()))
	if err == nil {
		record, _ := json.Marshal(map[string]any{"pid": os.Getpid(), "startTicks": self.StartTicks, "startMicro": self.StartedAt.UnixMicro(), "variant": fmt.Sprintf("%x", sha256.Sum256(key)), "since": time.Now().UTC()})
		err = os.WriteFile(path, record, 0o600)
	}
	return func() { _ = unlockAndClose(file) }, err
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
func moduleRoot(at ...string) (string, error) {
	directory, err := os.Getwd()
	if len(at) > 0 && at[0] != "" {
		directory, err = filepath.Abs(at[0])
	}
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
