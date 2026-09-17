package testenv

import (
	"bufio"
	"bytes"
	"encoding/hex"
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
	for _, name := range []string{"METASYSTEM_HOOK_DELEGATE_JOB", "METASYSTEM_GUARD_PROBE", "METASYSTEM_ALLOW_NEW_PLAN"} {
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
	t.Cleanup(func() { _ = live.cleanup() })
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
	removed := []string{
		file(registryHomePrefix+"old.custodian.log", true),
		file(registryHomePrefix+"old.custodian-123.log", true),
	}
	kept := []string{
		file(registryHomePrefix+"young.custodian.log", false),
		file(registryHomePrefix+"young.custodian-123.log", false),
		file(registryHomePrefix+"old.custodian-x.log", true),
		file(registryHomePrefix+"old.txt", true),
		file("other.custodian-1.log", true),
		file(filepath.Base(dead.path)+".custodian-456.log", false),
	}
	oldDirectory := filepath.Join(root, registryHomePrefix+"old.custodian-789.log")
	checkTestenv(t, os.Mkdir(oldDirectory, 0o700))
	checkTestenv(t, os.Chtimes(oldDirectory, old, old))
	kept = append(kept, oldDirectory)
	linkTarget := file("old-log-link-target", true)
	logSymlink := filepath.Join(root, registryHomePrefix+"old-link.custodian-789.log")
	checkTestenv(t, os.Symlink(linkTarget, logSymlink))
	kept = append(kept, logSymlink, linkTarget)

	removeDeadRegistryHomes(root)
	removeDeadRegistryHomes(root)
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

func TestAgedSweepRemovesFixtureRecordFiles(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-8 * 24 * time.Hour)
	makeFile := func(name string, age bool) string {
		path := filepath.Join(root, name)
		checkTestenv(t, os.WriteFile(path, nil, 0o600))
		if age {
			checkTestenv(t, os.Chtimes(path, old, old))
		}
		return path
	}
	removed := makeFile(registryHomePrefix+"old.fixture-refs-123", true)
	kept := []string{
		makeFile(registryHomePrefix+"young.fixture-refs-123", false),
		makeFile(registryHomePrefix+"old.fixture-refs-not-pid", true),
	}
	directory := filepath.Join(root, registryHomePrefix+"old.fixture-refs-456")
	checkTestenv(t, os.Mkdir(directory, 0o700))
	checkTestenv(t, os.Chtimes(directory, old, old))
	target := makeFile("record-link-target", true)
	link := filepath.Join(root, registryHomePrefix+"old.fixture-refs-789")
	checkTestenv(t, os.Symlink(target, link))
	kept = append(kept, directory, target, link)
	removeDeadRegistryHomes(root)
	if _, err := os.Lstat(removed); !os.IsNotExist(err) {
		t.Fatalf("old fixture record survived cleanup: %v", err)
	}
	for _, path := range kept {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("cleanup removed kept path %s: %v", filepath.Base(path), err)
		}
	}
}

func TestQuietFixtureCustodianLogDecision(t *testing.T) {
	owner := liveTestOwner(t)
	completion := identity.FixtureCustodianCompletionLine(owner)
	other := owner
	other.Pid++
	for _, test := range []struct {
		name, content string
		remove        bool
	}{
		{"completion alone", completion, true},
		{"kill", completion + "fixture-custodian action=kill pid=1\n", false},
		{"kill owner", completion + "fixture-custodian action=kill-owner\n", false},
		{"race report", completion + "WARNING: DATA RACE\n", false},
		{"panic", "panic: runtime failure\n" + completion, false},
		{"twice", completion + completion, false},
		{"no newline", strings.TrimSuffix(completion, "\n"), false},
		{"another owner", identity.FixtureCustodianCompletionLine(other), false},
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
	_, err = target.WriteString(completion)
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
	_, err = different.WriteString(completion)
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
	var command *exec.Cmd
	registry := filepath.Join(t.TempDir(), "registry")
	ref, records, err := startFixtureCustodian(func() *exec.Cmd { command = exec.Command(os.Args[0]); return command }, registry)
	checkTestenv(t, err)
	t.Cleanup(func() { _ = identity.SignalExact(identity.KernelProber{}, ref, syscall.SIGKILL); _ = command.Wait() })
	if info, statErr := os.Stat(records); statErr != nil || !info.Mode().IsRegular() || records != fmt.Sprintf("%s.fixture-refs-%d", registry, os.Getpid()) {
		t.Fatalf("custodian records = %q, info=%v err=%v", records, info, statErr)
	}
	if exact, state, probeErr := (identity.KernelProber{}).Probe(ref.Pid); probeErr != nil || state != identity.Alive || !identity.SameIdentity(exact, ref) || exact.Zombie {
		t.Fatalf("started custodian: state=%s exact=%+v err=%v", state, exact, probeErr)
	}
	if current, present := FixtureCustodian(); !present || binaryRef != current {
		t.Fatalf("binary custodian changed: before=%+v after=%+v/%t", binaryRef, current, present)
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
	if got := strings.TrimSpace(string(output)); got != parentHome {
		t.Fatalf("helper registry home = %q, want parent home %q", got, parentHome)
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
