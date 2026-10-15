package testenv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gocache"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testenv/testdata/buildlock"
	"golang.org/x/sys/unix"
)

func lockMarker(t *testing.T, root, name string) {
	t.Helper()
	checkTestenv(t, os.WriteFile(filepath.Join(root, name), nil, 0o600))
}
func awaitLockMarker(t *testing.T, root, name string) {
	t.Helper()
	Await(t, name, func() bool {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			return true
		}
		if status, err := os.ReadFile(filepath.Join(root, "finished")); err == nil {
			log, _ := os.ReadFile(filepath.Join(root, "log"))
			t.Fatalf("child exited before %s: %s\n%s", name, status, log)
		}
		return false
	})
}
func noLockEntry(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(root, "entered")); !os.IsNotExist(err) {
		t.Fatalf("command entered before lock release: %v", err)
	}
}

// Every child gets its own HOME/TMPDIR through Main and its own output path.
func lockChild(t *testing.T, cache, mode string) (string, func()) {
	t.Helper()
	root := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestBuildLockChild$", "-test.count=1", "-test.timeout=30m")
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, gocache.ContextEnv+"=") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GOCACHE="+cache, "TEST_BUILD_LOCK_MODE="+mode, buildlock.Root+"="+root)
	log, err := os.Create(filepath.Join(root, "log"))
	checkTestenv(t, err)
	cmd.Stdout, cmd.Stderr = log, log
	checkTestenv(t, cmd.Start())
	done := make(chan error, 1)
	go func() {
		err := cmd.Wait()
		_ = os.WriteFile(filepath.Join(root, "finished"), []byte(fmt.Sprint(err)), 0o600)
		done <- err
	}()
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = log.Close() })
	return root, func() {
		t.Helper()
		if err := <-done; err != nil {
			data, _ := os.ReadFile(log.Name())
			t.Fatalf("build child: %v\n%s", err, data)
		}
	}
}

func fakeBuild(t *testing.T, ctx context.Context, root string, flags ...string) *GoCmd {
	t.Helper()
	traced := &buildlock.TraceContext{Context: ctx, Root: root}
	args := append([]string{"build", "-buildvcs=false"}, flags...)
	args = append(args, "-o", filepath.Join(os.TempDir(), "own-engine"), "./cmd/metasystem")
	cmd := GoContext(traced, args...)
	traced.Arm()
	cmd.Cmd = exec.CommandContext(ctx, os.Args[0], args...)
	module, err := moduleRoot()
	checkTestenv(t, err)
	cmd.Dir = module
	cmd.Env = append(os.Environ(), buildlock.Root+"="+root)
	return cmd
}

func lockPath(t *testing.T, cmd *GoCmd) string {
	t.Helper()
	caches, err := gocache.Resolve(cmd.Environ())
	checkTestenv(t, err)
	paths, err := filepath.Glob(filepath.Join(caches.GoCache, "*.lock"))
	checkTestenv(t, err)
	if len(paths) == 0 {
		release, err := buildLock(t.Context(), cmd)
		checkTestenv(t, err)
		release()
		paths, err = filepath.Glob(filepath.Join(caches.GoCache, "*.lock"))
		checkTestenv(t, err)
	}
	if len(paths) != 1 {
		t.Fatalf("lock paths = %v", paths)
	}
	return paths[0]
}

func TestBuildLockChild(t *testing.T) {
	t.Parallel()
	mode, root := os.Getenv("TEST_BUILD_LOCK_MODE"), os.Getenv(buildlock.Root)
	if mode == "" {
		return
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var flags []string
	if mode == "different" {
		flags = []string{"-trimpath"}
	}
	buildContext := context.Background()
	if mode == "cancel" {
		buildContext = ctx
	}
	cmd := fakeBuild(t, buildContext, root, flags...)
	if mode == "cancel" {
		path := lockPath(t, cmd)
		before := lockDescriptors(t, path)
		done := make(chan error, 1)
		go func() { _, err := cmd.CombinedOutput(); done <- err }()
		awaitLockMarker(t, root, "waiting")
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation = %v", err)
		}
		noLockEntry(t, root)
		if after := lockDescriptors(t, path); after != before {
			t.Fatalf("lock descriptor leak: before %d after %d", before, after)
		}
		lockMarker(t, root, "cancelled")
		awaitLockMarker(t, root, "retry")
		cmd = fakeBuild(t, context.Background(), root)
	}
	_, err := cmd.CombinedOutput()
	if mode == "fail" {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 73 {
			t.Fatalf("build failure = %v", err)
		}
	} else {
		checkTestenv(t, err)
	}
}

// Native descriptor observations distinguish closing from merely unlocking.
func lockDescriptors(t *testing.T, path string) int {
	t.Helper()
	var want unix.Stat_t
	checkTestenv(t, unix.Stat(path, &want))
	entries, err := os.ReadDir("/dev/fd")
	checkTestenv(t, err)
	count := 0
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		var got unix.Stat_t
		if unix.Fstat(fd, &got) == nil && got.Dev == want.Dev && got.Ino == want.Ino {
			count++
		}
	}
	return count
}

func TestBuildLockExcludesSameVariantAcrossNamespaces(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	first, joinFirst := lockChild(t, cache, "fail")
	lockMarker(t, first, "fail")
	awaitLockMarker(t, first, "entered")
	paths, err := filepath.Glob(filepath.Join(cache, "*.lock"))
	checkTestenv(t, err)
	if len(paths) != 1 {
		t.Fatalf("locks = %v", paths)
	}
	data, err := os.ReadFile(paths[0])
	checkTestenv(t, err)
	var holder struct {
		PID        int64  `json:"pid"`
		StartTicks int64  `json:"startTicks"`
		StartMicro int64  `json:"startMicro"`
		Variant    string `json:"variant"`
		Since      string `json:"since"`
	}
	checkTestenv(t, json.Unmarshal(data, &holder))
	exact, state, err := (identity.KernelProber{}).ReadStart(holder.PID)
	checkTestenv(t, err)
	if state != identity.Alive || holder.StartTicks != exact.StartTicks || holder.StartMicro != exact.StartedAt.UnixMicro() || holder.Since == "" || !strings.Contains(paths[0], holder.Variant) {
		t.Fatalf("holder = %s", data)
	}
	second, joinSecond := lockChild(t, cache, "pass")
	Await(t, "second build blocked or entered", func() bool {
		_, waiting := os.Stat(filepath.Join(second, "waiting"))
		_, entered := os.Stat(filepath.Join(second, "entered"))
		return waiting == nil || entered == nil
	})
	noLockEntry(t, second)
	different, joinDifferent := lockChild(t, cache, "different")
	awaitLockMarker(t, different, "entered")
	lockMarker(t, different, "release")
	joinDifferent()
	lockMarker(t, first, "release")
	joinFirst()
	awaitLockMarker(t, second, "entered")
	var one, two map[string]any
	data, err = os.ReadFile(filepath.Join(first, "entered"))
	checkTestenv(t, err)
	checkTestenv(t, json.Unmarshal(data, &one))
	data, err = os.ReadFile(filepath.Join(second, "entered"))
	checkTestenv(t, err)
	checkTestenv(t, json.Unmarshal(data, &two))
	if one["home"] == two["home"] || one["tmp"] == two["tmp"] {
		t.Fatalf("namespaces not distinct: %v %v", one, two)
	}
	lockMarker(t, second, "release")
	joinSecond()
}

func TestBuildLockCancellationClosesDescriptorAndReleasesSlot(t *testing.T) {
	t.Parallel()
	cache := t.TempDir()
	holder, joinHolder := lockChild(t, cache, "pass")
	awaitLockMarker(t, holder, "entered")
	waiter, joinWaiter := lockChild(t, cache, "cancel")
	awaitLockMarker(t, waiter, "cancelled")
	noLockEntry(t, waiter)
	lockMarker(t, holder, "release")
	joinHolder()
	lockMarker(t, waiter, "retry")
	awaitLockMarker(t, waiter, "entered")
	lockMarker(t, waiter, "release")
	joinWaiter()
}

func TestBuildLockTakesSlotBeforeHostLock(t *testing.T) {
	t.Parallel()
	cache, root := t.TempDir(), t.TempDir()
	cmd := fakeBuild(t, context.Background(), root)
	cmd.Env = append(cmd.Env, "GOCACHE="+cache)
	releaseSlot := HoldToolchain()
	released := false
	defer func() {
		if !released {
			releaseSlot()
		}
	}()
	done := make(chan error, 1)
	go func() { _, err := cmd.CombinedOutput(); done <- err }()
	awaitLockMarker(t, root, "slot")
	other, joinOther := lockChild(t, cache, "pass")
	Await(t, "other process enters while the slot is held", func() bool {
		_, waiting := os.Stat(filepath.Join(other, "waiting"))
		_, entered := os.Stat(filepath.Join(other, "entered"))
		return waiting == nil || entered == nil
	})
	if _, err := os.Stat(filepath.Join(other, "waiting")); err == nil {
		t.Fatal("host lock taken before the binary's slot")
	}
	releaseSlot()
	released = true
	awaitLockMarker(t, root, "waiting")
	noLockEntry(t, root)
	lockMarker(t, other, "release")
	joinOther()
	awaitLockMarker(t, root, "entered")
	lockMarker(t, root, "release")
	checkTestenv(t, <-done)
}

func TestBuildLockOpenErrorPreventsCommandEntry(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	cmd := fakeBuild(t, context.Background(), root)
	cmd.Env = append(cmd.Env, "GOCACHE="+cache)
	path := lockPath(t, cmd)
	checkTestenv(t, os.Remove(path))
	checkTestenv(t, os.Mkdir(path, 0o700))
	lockMarker(t, root, "release")
	_, err := cmd.Output()
	if err == nil || !strings.Contains(err.Error(), "build lock:") || !strings.Contains(err.Error(), path) {
		t.Fatalf("lock error = %v", err)
	}
	noLockEntry(t, root)
}

func TestBuildLockIdentityKeepsFlagsAndSourceButOmitsTemporaryPaths(t *testing.T) {
	t.Parallel()
	cache, root := t.TempDir(), t.TempDir()
	source := func() string {
		module := t.TempDir()
		checkTestenv(t, os.WriteFile(filepath.Join(module, "go.mod"), []byte("module fixture\n"), 0o600))
		checkTestenv(t, os.MkdirAll(filepath.Join(module, "cmd", "metasystem"), 0o700))
		return module
	}
	module := source()
	take := func(dir string, args ...string) {
		t.Helper()
		cmd := Go(args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GOCACHE="+cache)
		release, err := buildLock(t.Context(), cmd)
		checkTestenv(t, err)
		release()
	}
	take(module, "build", "-trimpath", "-ldflags", "-X fixture.Path="+filepath.Join(root, "one"), "-o", filepath.Join(root, "output-one"), "./cmd/metasystem")
	take(filepath.Join(module, "cmd", "metasystem"), "build", "-trimpath", "-ldflags", "-X fixture.Path="+filepath.Join(root, "two"), "-o="+filepath.Join(root, "output-two"), ".")
	take(module, "build", "-trimpath", "-ldflags", "-X fixture.Path="+filepath.Join(toolchainTempRoot, "synthetic-build-input"), "./cmd/metasystem")
	paths, err := filepath.Glob(filepath.Join(cache, "*.lock"))
	checkTestenv(t, err)
	if len(paths) != 1 {
		t.Fatalf("output or temporary path split one variant: %v", paths)
	}

	take(module, "build", "-ldflags", "-X fixture.Path="+filepath.Join(root, "two"), "-trimpath", "./cmd/metasystem")
	take(source(), "build", "-trimpath", "-ldflags", "-X fixture.Path="+filepath.Join(root, "two"), "./cmd/metasystem")
	take(module, "build", "-trimpath", "-ldflags", "-X fixture.Path="+filepath.Join(root, "two"), "./cmd/devgate")
	paths, err = filepath.Glob(filepath.Join(cache, "*.lock"))
	checkTestenv(t, err)
	if len(paths) != 4 {
		t.Fatalf("flag order, package or module identity collapsed variants: %v", paths)
	}
}
