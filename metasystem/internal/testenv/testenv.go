// Package testenv owns the process environment boundary for tests that run
// metasystem engines, guards, hooks, or agent scripts.
package testenv

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const supervisionRegistryHome = "METASYSTEM_SUPERVISION_REGISTRY_HOME"

const registryOwnerNonce = "METASYSTEM_TESTENV_REGISTRY_NONCE"

const registryHomePrefix = "metasystem-test-registry-"

const registryOwnerFile = ".owner"

const registryNonceSize = 32

var inheritedControlNames = []string{
	"METASYSTEM_ALLOW_NEW_PLAN",
	"METASYSTEM_BIN",
	"METASYSTEM_CHECKOUT_EXECUTION_GUARD_ROOT",
	"METASYSTEM_DELEGATE_ROOT",
	"METASYSTEM_GATE_WITNESS_ROOT",
	"METASYSTEM_GUARD_PROBE",
	"METASYSTEM_HARNESS_ROOT",
	"METASYSTEM_HOOK_DELEGATE_INSTALLATION_ROOT",
	"METASYSTEM_HOOK_DELEGATE_STATE_ROOT",
	"METASYSTEM_PROOF_AUTH_BIN",
	"METASYSTEM_PROOF_CONTROL_ROOT",
	"METASYSTEM_PROOF_RUN_ROOT",
	"METASYSTEM_SUITE_PROGRESS_ROOT",
}

var inheritedControlPrefixes = []string{
	"METASYSTEM_CHECKOUT_EXECUTION_GUARD_",
	identity.FixtureCustodianEnv,
	"METASYSTEM_GATE_WITNESS",
	"METASYSTEM_HOOK_DELEGATE_",
	"METASYSTEM_PROOF_",
	"METASYSTEM_SUITE_PROGRESS_",
}

var fixtureKeys struct {
	sync.Mutex
	keys []identity.FixtureKey
}

var fixtureCustodian identity.Ref
var fixtureCustodianRecords string

// FixtureCustodian returns the exact custodian started for this test binary.
func FixtureCustodian() (identity.Ref, bool) { return fixtureCustodian, fixtureCustodian.Pid != 0 }

// FixtureCustodianRecords returns the record file read by this test binary's custodian.
func FixtureCustodianRecords() (string, bool) {
	return fixtureCustodianRecords, fixtureCustodianRecords != ""
}

// RegisterFixtureKey records a key minted by this test binary for its exit scan.
func RegisterFixtureKey(key identity.FixtureKey) {
	fixtureKeys.Lock()
	defer fixtureKeys.Unlock()
	fixtureKeys.keys = append(fixtureKeys.keys, key)
}

func registeredFixtureKeys() []identity.FixtureKey {
	fixtureKeys.Lock()
	defer fixtureKeys.Unlock()
	return append([]identity.FixtureKey(nil), fixtureKeys.keys...)
}

// Declaration captures a selector that a test launcher deliberately supplies
// to a package. Main restores only declared selectors after clearing the
// caller's ambient controls.
type Declaration struct {
	name    string
	value   string
	present bool
}

// Declare captures a deliberate package-level input before Main clears it.
func Declare(name string) Declaration {
	value, present := os.LookupEnv(name)
	return Declaration{name: name, value: value, present: present}
}

// DeclareInheritedControls captures every currently present selector. Package
// TestMains use it only for an explicit subprocess-helper mode whose parent
// deliberately constructed the child's environment.
func DeclareInheritedControls() []Declaration {
	declarations := make([]Declaration, 0)
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if rejectsInheritedControl(name) {
			declarations = append(declarations, Declare(name))
		}
	}
	return declarations
}

// Main prepares the package environment, runs the tests, releases its registry
// ownership, and returns the package exit code.
func Main(m *testing.M, declarations ...Declaration) (code int) {
	if os.Getenv(identity.FixtureCustodianEnv) == "1" {
		owner, err := identity.ParseRef(os.Getenv(identity.FixtureCustodianOwnerEnv))
		if err != nil {
			fmt.Fprintf(os.Stderr, "run fixture custodian: owner=%q: %v\n", os.Getenv(identity.FixtureCustodianOwnerEnv), err)
			return 2
		}
		var watchStat unix.Stat_t
		if err := unix.Fstat(3, &watchStat); err != nil {
			fmt.Fprintf(os.Stderr, "run fixture custodian: watch descriptor 3 is unavailable; it must be a pipe: %v\n", err)
			return 2
		}
		if watchStat.Mode&unix.S_IFMT != unix.S_IFIFO {
			fmt.Fprintf(os.Stderr, "run fixture custodian: watch descriptor 3 is mode %#o, not a pipe\n", watchStat.Mode&unix.S_IFMT)
			return 2
		}
		watchFlags, err := unix.FcntlInt(uintptr(3), unix.F_GETFL, 0)
		if err != nil || watchFlags&unix.O_ACCMODE != unix.O_RDONLY {
			fmt.Fprintf(os.Stderr, "run fixture custodian: watch descriptor 3 is not a pipe read end: flags=%#x err=%v\n", watchFlags, err)
			return 2
		}
		var readyStat unix.Stat_t
		statErr := unix.Fstat(4, &readyStat)
		readyFlags, err := unix.FcntlInt(uintptr(4), unix.F_GETFL, 0)
		if statErr != nil || readyStat.Mode&unix.S_IFMT != unix.S_IFIFO || err != nil || readyFlags&unix.O_ACCMODE != unix.O_WRONLY {
			fmt.Fprintf(os.Stderr, "run fixture custodian: ready descriptor 4 is not a pipe opened for writing: mode=%#o flags=%#x stat=%v flags-error=%v\n", readyStat.Mode&unix.S_IFMT, readyFlags, statErr, err)
			return 2
		}
		watch := os.NewFile(3, "fixture-owner-watch")
		ready := os.NewFile(4, "fixture-custodian-ready")
		err = identity.RunCustodian(owner, watch, ready, os.Stderr)
		_ = watch.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "run fixture custodian:", err)
			return 2
		}
		if logPath := os.Getenv(identity.FixtureCustodianLogEnv); logPath != "" {
			removeQuietFixtureCustodianLog(logPath, owner, os.Stderr)
		}
		return 0
	}
	cleanup, err := prepare(declarations)
	if err != nil {
		fmt.Fprintf(os.Stderr, "prepare test environment: %v\n", err)
		return 2
	}
	defer func() {
		if err := cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "clean test environment: %v\n", err)
			if code == 0 {
				code = 2
			}
		}
	}()
	fixtureCustodian, fixtureCustodianRecords, err = startFixtureCustodian(func() *exec.Cmd { return exec.Command(os.Args[0]) }, os.Getenv(supervisionRegistryHome))
	if err != nil {
		fmt.Fprintf(os.Stderr, "start fixture custodian: %v\n", err)
		return 2
	}
	code = exitScan(m.Run(), registeredFixtureKeys(), identity.FixtureSurvivors, identity.KernelProber{}, syscall.Kill, os.Stderr)
	return code
}

type fixtureScanFunc func(identity.FixtureKey) ([]identity.FixtureSurvivor, error)

func exitScan(code int, keys []identity.FixtureKey, scan fixtureScanFunc, prober identity.Prober, signal identity.SignalFunc, output io.Writer) int {
	failed := false
	for _, key := range keys {
		survivors, err := scan(key)
		if err != nil {
			fmt.Fprintf(output, "fixture exit scan: test=%q error=%v\n", key.Test, err)
			failed = true
			continue
		}
		for _, survivor := range survivors {
			exact, state, probeErr := prober.Probe(survivor.Ref.Pid)
			if probeErr == nil && state == identity.Alive && identity.SameIdentity(exact, survivor.Ref) && exact.Zombie {
				continue
			}
			fmt.Fprintf(output, "fixture child outlived test: test=%q pid=%d exe=%q argv=%q\n", key.Test, survivor.Ref.Pid, survivor.Exe, survivor.Argv)
			failed = true
			if survivor.Class == identity.FixtureSurvivorCertain {
				_ = identity.SignalExact(prober, survivor.Ref, syscall.SIGKILL, signal)
				waitForFixtureExit(prober, survivor.Ref)
			}
		}
	}
	if failed && code == 0 {
		return 1
	}
	return code
}

func waitForFixtureExit(prober identity.Prober, ref identity.Ref) {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.After(5 * time.Second)
	for {
		exact, state, err := prober.Probe(ref.Pid)
		if err == nil && (state == identity.Dead || state == identity.Alive && (!identity.SameIdentity(exact, ref) || exact.Zombie)) {
			return
		}
		select {
		case <-ticker.C:
		case <-deadline:
			return
		}
	}
}

func startFixtureCustodian(build func() *exec.Cmd, registry string) (identity.Ref, string, error) {
	exact, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return identity.Ref{}, "", fmt.Errorf("prove owner identity: state=%s err=%v", state, err)
	}
	owner, err := identity.EncodeRef(exact.Ref())
	if err != nil {
		return identity.Ref{}, "", err
	}
	runOwner, runOwnerSet := os.LookupEnv(identity.RunOwnerEnv)
	chain, err := identity.ResolveRunOwner(exact.Ref(), runOwner, runOwnerSet)
	if err != nil {
		return identity.Ref{}, "", fmt.Errorf("run owner %q: %w", runOwner, err)
	}
	chainValues := make([]string, len(chain))
	for index, ref := range chain {
		chainValues[index], err = identity.EncodeRef(ref)
		if err != nil {
			return identity.Ref{}, "", err
		}
	}
	recordsPath := fmt.Sprintf("%s.fixture-refs-%d", registry, exact.Pid)
	records, err := os.OpenFile(recordsPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return identity.Ref{}, "", err
	}
	if err := records.Close(); err != nil {
		return identity.Ref{}, "", err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		return identity.Ref{}, "", err
	}
	defer func() {
		reader.Close()
		writer.Close()
	}()
	readyReader, readyWriter, err := os.Pipe()
	if err != nil {
		return identity.Ref{}, "", err
	}
	defer readyReader.Close()
	defer readyWriter.Close()
	logPath := fmt.Sprintf("%s.custodian-%d.log", registry, exact.Pid)
	logFile, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return identity.Ref{}, "", err
	}
	defer logFile.Close()
	if err := unix.Flock(int(logFile.Fd()), unix.LOCK_EX); err != nil {
		return identity.Ref{}, "", err
	}
	command := build()
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if name != identity.FixtureOwnerEnv && !strings.HasPrefix(name, identity.FixtureCustodianEnv) {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, identity.FixtureCustodianEnv+"=1", identity.FixtureCustodianOwnerEnv+"="+owner,
		identity.FixtureCustodianLogEnv+"="+logPath, identity.FixtureCustodianChainEnv+"="+strings.Join(chainValues, "|"),
		identity.FixtureCustodianRecordsEnv+"="+recordsPath)
	for _, name := range []string{identity.FixtureCustodianPollEnv, identity.FixtureCustodianBoundEnv} {
		if value, present := os.LookupEnv(name); present {
			command.Env = append(command.Env, name+"="+value)
		}
	}
	command.ExtraFiles = []*os.File{reader, readyWriter}
	command.Stderr = logFile
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := command.Start(); err != nil {
		return identity.Ref{}, "", err
	}
	_ = readyWriter.Close()
	ready := bufio.NewScanner(readyReader)
	for ready.Scan() && ready.Text() != "ready" {
	}
	if ready.Text() != "ready" {
		_ = command.Wait()
		return identity.Ref{}, "", fmt.Errorf("custodian exited before ready: %s; log tail: %q", command.ProcessState, fixtureCustodianLogTail(logPath))
	}
	custodian, state, probeErr := (identity.KernelProber{}).Probe(int64(command.Process.Pid))
	if probeErr != nil || state != identity.Alive || !custodian.Ref().NativeExact() || custodian.Zombie {
		_ = command.Process.Kill()
		_ = command.Wait()
		return identity.Ref{}, "", fmt.Errorf("probe custodian: state=%s zombie=%t err=%v", state, custodian.Zombie, probeErr)
	}
	// The duplicate is never closed; the kernel closes it at process exit, which tells the custodian the owner died.
	if _, err := unix.FcntlInt(writer.Fd(), unix.F_DUPFD_CLOEXEC, 0); err != nil {
		_ = identity.SignalExact(identity.KernelProber{}, custodian.Ref(), syscall.SIGKILL)
		_ = command.Wait()
		return identity.Ref{}, "", err
	}
	return custodian.Ref(), recordsPath, nil
}

func fixtureCustodianLogTail(path string) string {
	contents, _ := os.ReadFile(path)
	lines := strings.Split(strings.TrimRight(string(contents), "\n"), "\n")
	return strings.Join(lines[max(0, len(lines)-8):], "\n")
}

func removeQuietFixtureCustodianLog(path string, owner identity.Ref, stderr *os.File) {
	if quietFixtureCustodianLog(path, owner, stderr) {
		_ = os.Remove(path)
	}
}

func quietFixtureCustodianLog(path string, owner identity.Ref, stderr *os.File) bool {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return false
	}
	log := os.NewFile(uintptr(fd), path)
	defer log.Close()
	logInfo, logErr := log.Stat()
	stderrInfo, stderrErr := stderr.Stat()
	if logErr != nil || stderrErr != nil || !logInfo.Mode().IsRegular() || !stderrInfo.Mode().IsRegular() || !os.SameFile(logInfo, stderrInfo) {
		return false
	}
	contents, err := io.ReadAll(log)
	if err != nil || !quietFixtureCustodianContents(string(contents), owner) {
		return false
	}
	pathInfo, err := os.Lstat(path)
	return err == nil && pathInfo.Mode().IsRegular() && os.SameFile(logInfo, pathInfo)
}

func quietFixtureCustodianContents(contents string, owner identity.Ref) bool {
	if !strings.HasSuffix(contents, "\n") {
		return false
	}
	lines := strings.Split(strings.TrimSuffix(contents, "\n"), "\n")
	watch := strings.TrimSuffix(identity.FixtureCustodianWatchLine(owner), "\n")
	complete := strings.TrimSuffix(identity.FixtureCustodianCompletionLine(owner), "\n")
	if len(lines) < 2 || lines[0] != watch || lines[len(lines)-1] != complete {
		return false
	}
	for _, line := range lines[1 : len(lines)-1] {
		if !strings.HasPrefix(line, "fixture-custodian action=observe ") &&
			!strings.HasPrefix(line, "fixture-custodian observation=unavailable ") {
			return false
		}
	}
	return true
}

func prepare(declarations []Declaration) (func() error, error) {
	registry, err := registryHomeForProcess(os.MkdirTemp)
	if err != nil {
		return nil, fmt.Errorf("create registry home: %w", err)
	}
	fail := func(err error) (func() error, error) {
		_ = registry.cleanup()
		return nil, err
	}

	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if rejectsInheritedControl(name) {
			if err := os.Unsetenv(name); err != nil {
				return fail(fmt.Errorf("clear inherited control %s: %w", name, err))
			}
		}
	}
	for _, declaration := range declarations {
		if !rejectsInheritedControl(declaration.name) {
			return fail(fmt.Errorf("declare input %s: not an inherited control", declaration.name))
		}
		if declaration.present {
			if err := os.Setenv(declaration.name, declaration.value); err != nil {
				return fail(fmt.Errorf("restore declared input %s: %w", declaration.name, err))
			}
		}
	}
	if err := os.Setenv(registryOwnerNonce, registry.nonce); err != nil {
		return fail(fmt.Errorf("pin registry owner nonce: %w", err))
	}
	if err := os.Setenv(supervisionRegistryHome, registry.path); err != nil {
		return fail(fmt.Errorf("pin registry home: %w", err))
	}
	return registry.cleanup, nil
}

type registryHomeLease struct {
	path  string
	nonce string
	lock  *os.File
	owner bool
}

func registryHomeForProcess(mkdirTemp func(string, string) (string, error)) (*registryHomeLease, error) {
	if inherited := inheritedRegistryHome(); inherited != nil {
		return inherited, nil
	}
	return createRegistryHome(mkdirTemp)
}

func createRegistryHome(mkdirTemp func(string, string) (string, error)) (*registryHomeLease, error) {
	removeDeadRegistryHomes("/tmp")
	registry, primaryErr := createRegistryHomeUnder(mkdirTemp, "/tmp")
	if primaryErr == nil {
		return registry, nil
	}

	fallbackRoot := os.TempDir()
	if fallbackRoot != "/tmp" {
		removeDeadRegistryHomes(fallbackRoot)
	}
	registry, fallbackErr := createRegistryHomeUnder(mkdirTemp, "")
	if fallbackErr == nil {
		return registry, nil
	}
	return nil, fmt.Errorf("in /tmp: %v; in TMPDIR: %w", primaryErr, fallbackErr)
}

func createRegistryHomeUnder(mkdirTemp func(string, string) (string, error), root string) (*registryHomeLease, error) {
	staging, err := mkdirTemp(root, "."+registryHomePrefix)
	if err != nil {
		return nil, err
	}
	removeStaging := true
	defer func() {
		if removeStaging {
			_ = os.RemoveAll(staging)
		}
	}()

	owner, err := os.OpenFile(filepath.Join(staging, registryOwnerFile), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, fmt.Errorf("create owner file: %w", err)
	}
	closeOwner := true
	defer func() {
		if closeOwner {
			_ = owner.Close()
		}
	}()
	if err := unix.Flock(int(owner.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		return nil, fmt.Errorf("lock owner file: %w", err)
	}
	nonceBytes := make([]byte, registryNonceSize)
	if _, err := rand.Read(nonceBytes); err != nil {
		return nil, fmt.Errorf("create owner nonce: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	if _, err := io.WriteString(owner, nonce); err != nil {
		return nil, fmt.Errorf("write owner nonce: %w", err)
	}

	base := filepath.Base(staging)
	if !strings.HasPrefix(base, "."+registryHomePrefix) {
		return nil, fmt.Errorf("temporary registry home has unexpected name %q", base)
	}
	home := filepath.Join(filepath.Dir(staging), strings.TrimPrefix(base, "."))
	if err := os.Rename(staging, home); err != nil {
		return nil, fmt.Errorf("publish registry home: %w", err)
	}
	removeStaging = false
	closeOwner = false
	return &registryHomeLease{path: home, nonce: nonce, lock: owner, owner: true}, nil
}

func inheritedRegistryHome() *registryHomeLease {
	home, homePresent := os.LookupEnv(supervisionRegistryHome)
	nonce, noncePresent := os.LookupEnv(registryOwnerNonce)
	if !homePresent || !noncePresent || len(nonce) != hex.EncodedLen(registryNonceSize) || !filepath.IsAbs(home) || filepath.Clean(home) != home ||
		!strings.HasPrefix(filepath.Base(home), registryHomePrefix) {
		return nil
	}
	info, err := os.Lstat(home)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil
	}
	owner, err := openRegistryOwner(home)
	if err != nil {
		return nil
	}
	if ownerNonce(owner) != nonce {
		_ = owner.Close()
		return nil
	}
	// A live owner holds a shared lock. An exclusive nonblocking lock succeeds
	// only after every owning process is gone; a child joins the shared lock so
	// the home remains protected if its parent exits first.
	if err := unix.Flock(int(owner.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
		_ = unix.Flock(int(owner.Fd()), unix.LOCK_UN)
		_ = owner.Close()
		return nil
	} else if !lockWouldBlock(err) {
		_ = owner.Close()
		return nil
	}
	if err := unix.Flock(int(owner.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		_ = owner.Close()
		return nil
	}
	info, err = os.Lstat(home)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		_ = owner.Close()
		return nil
	}
	return &registryHomeLease{path: home, nonce: nonce, lock: owner}
}

func ownerNonce(owner *os.File) string {
	info, err := owner.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() != int64(hex.EncodedLen(registryNonceSize)) {
		return ""
	}
	if _, err := owner.Seek(0, io.SeekStart); err != nil {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(owner, int64(hex.EncodedLen(registryNonceSize)+1)))
	if err != nil || len(data) != hex.EncodedLen(registryNonceSize) {
		return ""
	}
	return string(data)
}

func openRegistryOwner(home string) (*os.File, error) {
	path := filepath.Join(home, registryOwnerFile)
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, err
	}
	owner := os.NewFile(uintptr(fd), path)
	if owner == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open owner file")
	}
	info, err := owner.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = owner.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("owner file is not regular")
	}
	return owner, nil
}

func (registry *registryHomeLease) cleanup() error {
	if registry == nil || registry.lock == nil {
		return nil
	}
	if !registry.owner {
		return registry.lock.Close()
	}
	if err := unix.Flock(int(registry.lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		closeErr := registry.lock.Close()
		if lockWouldBlock(err) {
			return closeErr
		}
		return errors.Join(fmt.Errorf("lock registry home for cleanup: %w", err), closeErr)
	}
	removeErr := os.RemoveAll(registry.path)
	return errors.Join(removeErr, registry.lock.Close())
}

func removeDeadRegistryHomes(root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	oldestKept := time.Now().Add(-7 * 24 * time.Hour)
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		if isFixtureCustodianLog(entry.Name()) || isFixtureCustodianRecords(entry.Name()) {
			info, err := entry.Info()
			if err == nil && info.Mode().IsRegular() && info.ModTime().Before(oldestKept) {
				_ = os.Remove(path)
			}
			continue
		}
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), registryHomePrefix) {
			continue
		}
		owner, err := openRegistryOwner(path)
		if err != nil {
			continue
		}
		if err := unix.Flock(int(owner.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			_ = owner.Close()
			continue
		}
		_ = os.RemoveAll(path)
		_ = owner.Close()
	}
}

func isFixtureCustodianRecords(name string) bool {
	if !strings.HasPrefix(name, registryHomePrefix) {
		return false
	}
	marker := strings.LastIndex(name, ".fixture-refs-")
	if marker < 0 {
		return false
	}
	digits := name[marker+len(".fixture-refs-"):]
	return digits != "" && strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

func isFixtureCustodianLog(name string) bool {
	if !strings.HasPrefix(name, registryHomePrefix) {
		return false
	}
	if strings.HasSuffix(name, ".custodian.log") {
		return true
	}
	stem, found := strings.CutSuffix(name, ".log")
	marker := strings.LastIndex(stem, ".custodian-")
	if !found || marker < 0 {
		return false
	}
	digits := stem[marker+len(".custodian-"):]
	return digits != "" && strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' }) < 0
}

func lockWouldBlock(err error) bool {
	return errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)
}

// WithoutInheritedControls removes engine and installation selectors from a
// child environment while preserving unrelated values and their order.
func WithoutInheritedControls(environment []string) []string {
	clean := make([]string, 0, len(environment))
	for _, entry := range environment {
		name, _, _ := strings.Cut(entry, "=")
		if !rejectsInheritedControl(name) {
			clean = append(clean, entry)
		}
	}
	return clean
}

func rejectsInheritedControl(name string) bool {
	for _, inherited := range inheritedControlNames {
		if name == inherited {
			return true
		}
	}
	for _, prefix := range inheritedControlPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}
