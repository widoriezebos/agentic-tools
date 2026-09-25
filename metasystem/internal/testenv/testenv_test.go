package testenv

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

func TestPrepareClearsAmbientControlsPinsRegistryAndRestoresDeclarations(t *testing.T) {
	hostileRegistry := filepath.Join(t.TempDir(), "metasystem-test-registry-hostile")
	if err := os.Mkdir(hostileRegistry, 0o700); err != nil {
		t.Fatal(err)
	}
	hostileOwner, err := os.OpenFile(filepath.Join(hostileRegistry, registryOwnerFile), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = hostileOwner.Close() })
	if _, err := hostileOwner.WriteString(strings.Repeat("b", hex.EncodedLen(registryNonceSize))); err != nil {
		t.Fatal(err)
	}
	if err := unix.Flock(int(hostileOwner.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	observedTemporaryRoot := t.TempDir()
	t.Setenv("TMPDIR", observedTemporaryRoot)
	t.Setenv("METASYSTEM_BIN", "/deliberate/candidate-engine")
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "inherited-job")
	t.Setenv("METASYSTEM_GUARD_PROBE", "inherited-probe")
	t.Setenv("METASYSTEM_ALLOW_NEW_PLAN", "1")
	t.Setenv("METASYSTEM_TESTING_WORKERS", "9")
	t.Setenv(supervisionRegistryHome, hostileRegistry)
	t.Setenv(registryOwnerNonce, strings.Repeat("a", hex.EncodedLen(registryNonceSize)))
	declaration := Declare("METASYSTEM_BIN")

	cleanup, err := prepare([]Declaration{declaration})
	if err != nil {
		t.Fatal(err)
	}
	cleaned := false
	t.Cleanup(func() {
		if !cleaned {
			_ = cleanup()
		}
	})
	registryHome := os.Getenv(supervisionRegistryHome)
	if got := os.Getenv("METASYSTEM_BIN"); got != "/deliberate/candidate-engine" {
		t.Fatalf("declared candidate engine = %q", got)
	}
	for _, name := range []string{"METASYSTEM_HOOK_DELEGATE_JOB", "METASYSTEM_GUARD_PROBE", "METASYSTEM_ALLOW_NEW_PLAN", "METASYSTEM_TESTING_WORKERS"} {
		if got := os.Getenv(name); got != "" {
			t.Errorf("%s survived as %q", name, got)
		}
	}
	if registryHome == "" || registryHome == hostileRegistry || !filepath.IsAbs(registryHome) {
		t.Fatalf("registry home = %q, want a new absolute directory", registryHome)
	}
	parent := filepath.Dir(registryHome)
	if parent != "/tmp" && parent != observedTemporaryRoot {
		t.Fatalf("registry home = %q, want a child of /tmp or fallback TMPDIR %q", registryHome, observedTemporaryRoot)
	}
	if info, err := os.Stat(registryHome); err != nil || !info.IsDir() {
		t.Fatalf("registry home is not a directory: %v", err)
	}
	if err := cleanup(); err != nil {
		t.Fatal(err)
	}
	cleaned = true
	if _, err := os.Stat(registryHome); !os.IsNotExist(err) {
		t.Fatalf("registry home survived cleanup: %v", err)
	}
}

func TestProcessNamespaceOwnsHomeAndRuntimeButKeepsGoCaches(t *testing.T) {
	t.Parallel()
	const helperMode = "TESTENV_PROCESS_NAMESPACE_HELPER_MODE"
	if mode := os.Getenv(helperMode); mode != "" {
		verifyProcessNamespaceGoConsumer(t, mode)
		return
	}
	for _, mode := range []string{"unset", "empty", "nonempty"} {
		mode := mode
		t.Run(mode, func(t *testing.T) {
			base := t.TempDir()
			environment := processNamespaceCacheCaseEnvironment(os.Environ(), mode, base)
			disableGoTelemetryForFixture(t, environment)
			expected := readGoCacheEnvironment(t, environment)
			command := exec.Command(os.Args[0], "-test.run=^TestProcessNamespaceOwnsHomeAndRuntimeButKeepsGoCaches$", "-test.count=1")
			command.Env = append(environment,
				helperMode+"="+mode,
				"TESTENV_EXPECT_GOCACHE="+expected["GOCACHE"],
				"TESTENV_EXPECT_GOMODCACHE="+expected["GOMODCACHE"],
				"TESTENV_EXPECT_GOPATH="+expected["GOPATH"],
				"TESTENV_PREPARE_HOME="+base,
			)
			if output, err := command.CombinedOutput(); err != nil {
				t.Fatalf("namespace helper %s: %v\n%s", mode, err, output)
			}
		})
	}
}

func processNamespaceCacheCaseEnvironment(environment []string, mode, base string) []string {
	filtered := make([]string, 0, len(environment)+8)
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		switch name {
		case "GOCACHE", "GOMODCACHE", "GOPATH", "GOTOOLCHAIN", "GOPROXY", "HOME", "XDG_CACHE_HOME", "TESTENV_PROCESS_NAMESPACE_HELPER_MODE", "TEST_TELEMETRY_DIR":
			continue
		}
		filtered = append(filtered, entry)
	}
	filtered = append(filtered,
		"HOME="+base,
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"TEST_TELEMETRY_DIR="+filepath.Join(base, "go-telemetry"),
	)
	switch mode {
	case "empty":
		filtered = append(filtered, "GOCACHE=", "GOMODCACHE=", "GOPATH=")
	case "nonempty":
		filtered = append(filtered,
			"GOCACHE="+filepath.Join(base, "shared-go-build"),
			"GOMODCACHE="+filepath.Join(base, "shared-go-modules"),
			"GOPATH="+filepath.Join(base, "shared-go-path"),
		)
	}
	return filtered
}

func disableGoTelemetryForFixture(t *testing.T, environment []string) {
	t.Helper()
	command := exec.Command("go", "telemetry", "off")
	command.Env = environment
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("disable fixture Go telemetry: %v\n%s", err, output)
	}
}

func readGoCacheEnvironment(t *testing.T, environment []string) map[string]string {
	t.Helper()
	command := exec.Command("go", "env", "-json", "GOCACHE", "GOMODCACHE", "GOPATH")
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("read effective Go cache environment: %v\n%s", err, output)
	}
	values := make(map[string]string)
	if err := json.Unmarshal(output, &values); err != nil {
		t.Fatalf("decode effective Go cache environment: %v\n%s", err, output)
	}
	return values
}

func verifyProcessNamespaceGoConsumer(t *testing.T, mode string) {
	t.Helper()
	home, temporary := os.Getenv("HOME"), os.Getenv("TMPDIR")
	if mode == "" || home == "" || home == os.Getenv("TESTENV_PREPARE_HOME") || filepath.Dir(home) != filepath.Dir(temporary) {
		t.Fatalf("helper HOME %q was not isolated from %q", os.Getenv("HOME"), os.Getenv("TESTENV_PREPARE_HOME"))
	}
	actual := readGoCacheEnvironment(t, os.Environ())
	for _, name := range []string{"GOCACHE", "GOMODCACHE", "GOPATH"} {
		want := os.Getenv("TESTENV_EXPECT_" + name)
		if os.Getenv(name) != want || actual[name] != want {
			t.Errorf("%s after namespace = environment %q go-consumer %q, want pre-isolation effective value %q", name, os.Getenv(name), actual[name], want)
		}
	}
}

func TestInheritedRegistryHomeRejectsMatchingNonceWithoutLiveOwner(t *testing.T) {
	nonce := strings.Repeat("c", hex.EncodedLen(registryNonceSize))
	deadHome := filepath.Join(t.TempDir(), registryHomePrefix+"dead-owner")
	if err := os.Mkdir(deadHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deadHome, registryOwnerFile), []byte(nonce), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(supervisionRegistryHome, deadHome)
	t.Setenv(registryOwnerNonce, nonce)
	freshRoot := t.TempDir()

	registry, err := registryHomeForProcess(func(_ string, pattern string) (string, error) {
		return os.MkdirTemp(freshRoot, pattern)
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.cleanup() })
	if registry.path == deadHome || !registry.owner {
		t.Fatalf("dead inherited registry was accepted: %+v", registry)
	}
}

func TestCreateRegistryHomeFallsBackToTMPDIR(t *testing.T) {
	fallbackRoot := t.TempDir()
	t.Setenv("TMPDIR", fallbackRoot)
	var directories []string
	mkdirTemp := func(directory, pattern string) (string, error) {
		directories = append(directories, directory)
		if directory == "/tmp" {
			return "", errors.New("sandbox denied /tmp")
		}
		return os.MkdirTemp(directory, pattern)
	}

	registry, err := createRegistryHome(mkdirTemp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = registry.cleanup() })
	if want := []string{"/tmp", ""}; !reflect.DeepEqual(directories, want) {
		t.Fatalf("MkdirTemp directories = %v, want %v", directories, want)
	}
	if filepath.Dir(registry.path) != fallbackRoot {
		t.Fatalf("fallback registry home = %q, want a child of %q", registry.path, fallbackRoot)
	}
}

func TestRemoveDeadRegistryHomesKeepsLiveAndUnrelatedDirectories(t *testing.T) {
	root := t.TempDir()
	live, err := createRegistryHomeUnder(os.MkdirTemp, root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = live.cleanupReporting(io.Discard) })
	dead, err := createRegistryHomeUnder(os.MkdirTemp, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dead.lock.Close(); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(root, "unrelated-old-directory")
	if err := os.Mkdir(unrelated, 0o700); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(root, registryHomePrefix+"symlink")
	if err := os.Symlink(unrelated, symlink); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-8 * 24 * time.Hour)
	for _, path := range []string{live.path, unrelated} {
		if err := os.Chtimes(path, old, old); err != nil {
			t.Fatal(err)
		}
	}
	file := func(name string, age bool) string {
		path := filepath.Join(root, name)
		checkTestenv(t, os.WriteFile(path, []byte("log"), 0o600))
		if age {
			checkTestenv(t, os.Chtimes(path, old, old))
		}
		return path
	}
	// Sidecars of an authenticated dead registry go once their log is unlocked.
	removed := []string{file(filepath.Base(dead.path)+".custodian-456.log", false)}
	// Neither age nor an absent home proves who owns a sidecar.
	kept := []string{
		file(registryHomePrefix+"old.custodian.log", true),
		file(registryHomePrefix+"old.custodian-123.log", true),
		file(registryHomePrefix+"young.custodian-123.log", false),
		file(registryHomePrefix+"young.custodian.log", false),
		file(filepath.Base(live.path)+".custodian-123.log", false),
		file(registryHomePrefix+"old.custodian-x.log", true),
		file(registryHomePrefix+"old.txt", true),
		file("other.custodian-1.log", true),
	}
	oldDirectory := filepath.Join(root, registryHomePrefix+"old.custodian-789.log")
	checkTestenv(t, os.Mkdir(oldDirectory, 0o700))
	checkTestenv(t, os.Chtimes(oldDirectory, old, old))
	kept = append(kept, oldDirectory)
	linkTarget := file("old-log-link-target", true)
	logSymlink := filepath.Join(root, registryHomePrefix+"old-link.custodian-789.log")
	checkTestenv(t, os.Symlink(linkTarget, logSymlink))
	kept = append(kept, logSymlink, linkTarget)

	removeDeadRegistryHomes(root, io.Discard)
	removeDeadRegistryHomes(root, io.Discard)
	if _, err := os.Stat(dead.path); !os.IsNotExist(err) {
		t.Fatalf("dead registry survived cleanup: %v", err)
	}
	for _, path := range []string{live.path, unrelated} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("cleanup removed %s: %v", path, err)
		}
	}
	if info, err := os.Lstat(symlink); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("cleanup followed or removed registry-shaped symlink: %v", err)
	}
	for _, path := range removed {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Errorf("old custodian log survived cleanup: %s: %v", filepath.Base(path), err)
		}
	}
	for _, path := range kept {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("cleanup removed kept path %s: %v", filepath.Base(path), err)
		}
	}
}

func TestSweepKeepsSidecarsWithoutAuthenticatedDeadHome(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	live, err := createRegistryHomeUnder(os.MkdirTemp, root)
	checkTestenv(t, err)
	t.Cleanup(func() { _ = live.cleanupReporting(io.Discard) })
	old := time.Now().Add(-8 * 24 * time.Hour)
	makeFile := func(name string) string {
		path := filepath.Join(root, name)
		checkTestenv(t, os.WriteFile(path, []byte("diagnostic\n"), 0o600))
		checkTestenv(t, os.Chtimes(path, old, old))
		return path
	}
	kept := []string{
		// An aged pair of a live registry, its log unlocked.
		makeFile(filepath.Base(live.path) + ".fixture-refs-123"),
		makeFile(filepath.Base(live.path) + ".custodian-123.log"),
		// Aged records of a live registry without a log.
		makeFile(filepath.Base(live.path) + ".fixture-refs-124"),
		// An aged pair whose registry home is absent and so unknown.
		makeFile(registryHomePrefix + "old.fixture-refs-123"),
		makeFile(registryHomePrefix + "old.custodian-123.log"),
		makeFile(registryHomePrefix + "old.fixture-refs-not-pid"),
	}
	directory := filepath.Join(root, registryHomePrefix+"old.fixture-refs-456")
	checkTestenv(t, os.Mkdir(directory, 0o700))
	checkTestenv(t, os.Chtimes(directory, old, old))
	target := makeFile("record-link-target")
	link := filepath.Join(root, registryHomePrefix+"old.fixture-refs-789")
	checkTestenv(t, os.Symlink(target, link))
	kept = append(kept, live.path, directory, target, link)
	var report bytes.Buffer
	removeDeadRegistryHomes(root, &report)
	for _, path := range kept {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("cleanup removed kept path %s: %v", filepath.Base(path), err)
		}
	}
	if report.Len() != 0 {
		t.Fatalf("sweep reported sidecars it does not own: %q", report.String())
	}
}

func TestQuietFixtureCustodianLogDecision(t *testing.T) {
	owner := liveTestOwner(t)
	watch := identity.FixtureCustodianWatchLine(owner)
	completion := identity.FixtureCustodianCompletionLine(owner)
	other := owner
	other.Pid++
	for _, test := range []struct {
		name, content string
		remove        bool
	}{
		{"watch and completion", watch + completion, true},
		{"descendant observation", watch + "fixture-custodian action=observe pid=41 carrier=descendant identity=exact\n" + completion, true},
		{"unavailable observation", watch + "fixture-custodian observation=unavailable error=not enough memory\n" + completion, true},
		{"completion alone", completion, false},
		{"kill", watch + completion + "fixture-custodian action=kill pid=1\n", false},
		{"kill owner", watch + completion + "fixture-custodian action=kill-owner\n", false},
		{"race report", watch + completion + "WARNING: DATA RACE\n", false},
		{"panic", "panic: runtime failure\n" + watch + completion, false},
		{"twice", watch + completion + completion, false},
		{"no newline", watch + strings.TrimSuffix(completion, "\n"), false},
		{"another owner", watch + identity.FixtureCustodianCompletionLine(other), false},
		{"empty", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			log, err := os.Create(filepath.Join(t.TempDir(), "custodian.log"))
			checkTestenv(t, err)
			t.Cleanup(func() { _ = log.Close() })
			_, err = log.WriteString(test.content)
			checkTestenv(t, err)
			removeQuietFixtureCustodianLog(log.Name(), owner, log)
			_, err = os.Lstat(log.Name())
			if test.remove != os.IsNotExist(err) {
				t.Fatalf("removed=%t, want %t: %v", os.IsNotExist(err), test.remove, err)
			}
		})
	}

	target, err := os.Create(filepath.Join(t.TempDir(), "target.log"))
	checkTestenv(t, err)
	defer target.Close()
	_, err = target.WriteString(watch + completion)
	checkTestenv(t, err)
	link := target.Name() + ".link"
	checkTestenv(t, os.Symlink(target.Name(), link))
	removeQuietFixtureCustodianLog(link, owner, target)
	for _, path := range []string{link, target.Name()} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatalf("symlink case removed %s: %v", path, err)
		}
	}

	different, err := os.Create(filepath.Join(t.TempDir(), "different.log"))
	checkTestenv(t, err)
	defer different.Close()
	_, err = different.WriteString(watch + completion)
	checkTestenv(t, err)
	removeQuietFixtureCustodianLog(different.Name(), owner, target)
	if _, err := os.Lstat(different.Name()); err != nil {
		t.Fatalf("different regular file was removed: %v", err)
	}
}

func TestCustodianStartRefusesACustodianThatExitsAtOnce(t *testing.T) {
	for script, want := range map[string]string{"exit 2": "exit status 2", "printf 'notready\\n' >&4": "custodian exited before ready"} {
		_, _, err := startFixtureCustodian(func() *exec.Cmd { return exec.Command("/bin/sh", "-c", script) }, filepath.Join(t.TempDir(), "registry"))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("script %q: start error = %v, want text %q", script, err, want)
		}
	}
	binaryRef, _ := FixtureCustodian()
	const testPoll, testBound = 50 * time.Millisecond, 500 * time.Millisecond
	t.Setenv(identity.FixtureCustodianPollEnv, testPoll.String())
	t.Setenv(identity.FixtureCustodianBoundEnv, testBound.String())
	var command *exec.Cmd
	registry := filepath.Join(t.TempDir(), "registry")
	ref, records, err := startFixtureCustodian(func() *exec.Cmd { command = exec.Command(os.Args[0]); return command }, registry)
	checkTestenv(t, err)
	reapCustodianAfterTest(t, command, ref, testPoll, testBound)
	if info, statErr := os.Stat(records); statErr != nil || !info.Mode().IsRegular() || records != fmt.Sprintf("%s.fixture-refs-%d", registry, os.Getpid()) {
		t.Fatalf("custodian records = %q, info=%v err=%v", records, info, statErr)
	}
	if exact, state, probeErr := (identity.KernelProber{}).Probe(ref.Pid); probeErr != nil || state != identity.Alive || !identity.SameIdentity(exact, ref) || exact.Zombie {
		t.Fatalf("started custodian: state=%s exact=%+v err=%v", state, exact, probeErr)
	}
	if current, present := FixtureCustodian(); !present || binaryRef != current {
		t.Fatalf("binary custodian changed: before=%+v after=%+v/%t", binaryRef, current, present)
	}
	t.Run("passes explicit timing", func(t *testing.T) {
		t.Setenv(identity.FixtureCustodianPollEnv, "17ms")
		t.Setenv(identity.FixtureCustodianBoundEnv, "500ms")
		var timedCommand *exec.Cmd
		timedRegistry := filepath.Join(t.TempDir(), "registry")
		timedRef, _, err := startFixtureCustodian(func() *exec.Cmd {
			timedCommand = exec.Command(os.Args[0])
			return timedCommand
		}, timedRegistry)
		checkTestenv(t, err)
		reapCustodianAfterTest(t, timedCommand, timedRef, 17*time.Millisecond, 500*time.Millisecond)
		for _, want := range []string{
			identity.FixtureCustodianPollEnv + "=17ms",
			identity.FixtureCustodianBoundEnv + "=500ms",
		} {
			found := false
			for _, entry := range timedCommand.Env {
				found = found || entry == want
			}
			if !found {
				t.Fatalf("custodian environment omitted %q", want)
			}
		}
	})
	t.Run("cleanup wait is bounded", func(t *testing.T) {
		expired := make(chan time.Time, 1)
		expired <- time.Unix(1, 0)
		if custodianReaped(make(chan error), expired) {
			t.Fatal("expired custodian cleanup reported a reap")
		}
	})
}

func reapCustodianAfterTest(t *testing.T, command *exec.Cmd, ref identity.Ref, poll, bound time.Duration) {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	t.Cleanup(func() {
		started := time.Now()
		_ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL)
		timer := time.NewTimer(10 * (bound + poll))
		defer timer.Stop()
		if custodianReaped(done, timer.C) {
			return
		}
		exact, state, err := (identity.KernelProber{}).Probe(ref.Pid)
		t.Errorf("custodian %d did not report exit after %s (bound %s): state=%s same=%t zombie=%t probe=%v",
			ref.Pid, time.Since(started), 10*(bound+poll), state, identity.SameIdentity(exact, ref), exact.Zombie, err)
	})
}

func custodianReaped(done <-chan error, bound <-chan time.Time) bool {
	select {
	case <-done:
		return true
	case <-bound:
		return false
	}
}

func liveTestOwner(t *testing.T) identity.Ref {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		t.Fatalf("probe test owner: state=%s err=%v", state, err)
	}
	return exact.Ref()
}

func checkTestenv(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func TestDeclareInheritedControlsCapturesOnlyScrubbedSelectors(t *testing.T) {
	t.Setenv("METASYSTEM_BIN", "/deliberate/candidate-engine")
	t.Setenv("METASYSTEM_HOOK_DELEGATE_JOB", "deliberate-job")
	t.Setenv("UNRELATED_TEST_INPUT", "unrelated")

	declarations := DeclareInheritedControls()
	seen := make(map[string]string, len(declarations))
	for _, declaration := range declarations {
		seen[declaration.name] = declaration.value
	}
	if seen["METASYSTEM_BIN"] != "/deliberate/candidate-engine" || seen["METASYSTEM_HOOK_DELEGATE_JOB"] != "deliberate-job" {
		t.Fatalf("declared controls = %#v", seen)
	}
	if _, exists := seen["UNRELATED_TEST_INPUT"]; exists {
		t.Fatalf("unrelated input was declared: %#v", seen)
	}
}

func TestPrepareRejectsDeclarationsOutsideTheScrubbedControls(t *testing.T) {
	t.Setenv("UNRELATED_TEST_INPUT", "value")
	cleanup, err := prepare([]Declaration{Declare("UNRELATED_TEST_INPUT")})
	if err == nil {
		t.Fatal("unowned declaration was accepted")
	}
	if cleanup != nil {
		t.Fatal("unowned declaration returned a cleanup function")
	}
}

func TestHelperProcessReusesAuthenticatedParentRegistryHome(t *testing.T) {
	parentHome := os.Getenv(supervisionRegistryHome)
	if parentHome == "" {
		t.Fatal("parent TestMain did not pin a registry home")
	}
	ownerPath := filepath.Join(parentHome, registryOwnerFile)
	if err := os.Chmod(ownerPath, 0o400); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(ownerPath, 0o600) })
	scratch := t.TempDir()
	command := exec.Command(os.Args[0], "-test.run=^TestRegistryOwnerProcess$", "-test.count=1")
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != "TMPDIR" && name != "TESTENV_REGISTRY_HELPER_MODE" {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "TMPDIR="+scratch, "TESTENV_REGISTRY_HELPER_MODE=exit")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("run abrupt helper: %v\n%s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) != 3 || lines[0] != parentHome {
		t.Fatalf("helper namespace report = %q, want registry %q plus HOME and TMPDIR", lines, parentHome)
	}
	if lines[1] == os.Getenv("HOME") || !strings.HasPrefix(lines[1], parentHome+string(filepath.Separator)) {
		t.Fatalf("helper HOME = %q, want its own directory below %q", lines[1], parentHome)
	}
	if !strings.HasPrefix(lines[2], parentHome+string(filepath.Separator)) || filepath.Dir(lines[1]) != filepath.Dir(lines[2]) {
		t.Fatalf("helper HOME/TMPDIR = %q/%q, want one private process namespace", lines[1], lines[2])
	}
	if entries, err := os.ReadDir(scratch); err != nil || len(entries) != 0 {
		t.Fatalf("helper left entries in its TMPDIR: entries=%v err=%v", entries, err)
	}
}

func TestLiveRegistryHomeSurvivesAnotherProcessMainWhenBackdated(t *testing.T) {
	command, home := startRegistryOwnerProcess(t)
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(home, old, old); err != nil {
		t.Fatal(err)
	}
	runRegistryScavenger(t)
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		t.Fatalf("live registry home vanished: %v", err)
	}
	stopRegistryOwnerProcess(t, command)
	runRegistryScavenger(t)
}

func TestDeadRegistryHomeIsRemovedByAnotherProcessMain(t *testing.T) {
	command, home := startRegistryOwnerProcess(t)
	if info, err := os.Stat(home); err != nil || !info.IsDir() {
		t.Fatalf("live owner's registry home is missing before the kill: %v", err)
	}
	stopRegistryOwnerProcess(t, command)
	// The killed owner's custodian keeps the home until it has settled.
	waitRegistryCustodiansExited(t, home)
	runRegistryScavenger(t)
	if _, err := os.Stat(home); !os.IsNotExist(err) {
		t.Fatalf("dead registry home survived another process's Main: %v", err)
	}
}

func TestRegistryOwnerProcess(t *testing.T) {
	mode := os.Getenv("TESTENV_REGISTRY_HELPER_MODE")
	if mode == "" {
		return
	}
	fmt.Fprintln(os.Stdout, os.Getenv(supervisionRegistryHome))
	switch mode {
	case "exit":
		fmt.Fprintln(os.Stdout, os.Getenv("HOME"))
		fmt.Fprintln(os.Stdout, os.Getenv("TMPDIR"))
		os.Exit(0)
	case "wait":
		_, _ = io.Copy(io.Discard, os.Stdin)
		os.Exit(0)
	default:
		t.Fatalf("unknown registry helper mode %q", mode)
	}
}

func startRegistryOwnerProcess(t *testing.T) (*exec.Cmd, string) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestRegistryOwnerProcess$", "-test.count=1")
	command.Env = append(withoutRegistryOwnership(os.Environ()), "TESTENV_REGISTRY_HELPER_MODE=wait")
	if _, err := command.StdinPipe(); err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	scanner := bufio.NewScanner(stdout)
	if !scanner.Scan() {
		_ = command.Process.Kill()
		_ = command.Wait()
		t.Fatalf("registry owner did not report its home: scan=%v stderr=%s", scanner.Err(), stderr.String())
	}
	home := strings.TrimSpace(scanner.Text())
	if home == "" {
		t.Fatal("registry owner reported an empty home")
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
		_ = os.RemoveAll(home)
	})
	return command, home
}

func stopRegistryOwnerProcess(t *testing.T, command *exec.Cmd) {
	t.Helper()
	if err := command.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err == nil {
		t.Fatal("killed registry owner exited successfully")
	}
}

// waitRegistryCustodiansExited waits until no custodian log beside home is
// still locked by its running custodian.
func waitRegistryCustodiansExited(t *testing.T, home string) {
	t.Helper()
	bound, err := FixtureExitWaitBound()
	checkTestenv(t, err)
	deadline := time.Now().Add(bound)
	for {
		logs, err := filepath.Glob(home + ".custodian-*.log")
		checkTestenv(t, err)
		held := 0
		for _, path := range logs {
			log, err := os.Open(path)
			if err != nil {
				continue
			}
			if lockWouldBlock(unix.Flock(int(log.Fd()), unix.LOCK_EX|unix.LOCK_NB)) {
				held++
			}
			_ = log.Close()
		}
		if held == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d custodian logs beside %s stayed locked for %s", held, home, bound)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func runRegistryScavenger(t *testing.T) {
	t.Helper()
	command := exec.Command(os.Args[0], "-test.run=^TestRegistryOwnerProcess$", "-test.count=1")
	command.Env = withoutRegistryOwnership(os.Environ())
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("run registry scavenger: %v\n%s", err, output)
	}
}

func withoutRegistryOwnership(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if name != supervisionRegistryHome && name != "METASYSTEM_TESTENV_REGISTRY_NONCE" && name != "TESTENV_REGISTRY_HELPER_MODE" {
			clean = append(clean, entry)
		}
	}
	return clean
}
