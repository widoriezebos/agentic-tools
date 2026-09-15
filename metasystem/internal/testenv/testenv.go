// Package testenv owns the process environment boundary for tests that run
// metasystem engines, guards, hooks, or agent scripts.
package testenv

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
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
	"METASYSTEM_GATE_WITNESS",
	"METASYSTEM_HOOK_DELEGATE_",
	"METASYSTEM_PROOF_",
	"METASYSTEM_SUITE_PROGRESS_",
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
	code = m.Run()
	return code
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
	for _, entry := range entries {
		if !entry.IsDir() || entry.Type()&os.ModeSymlink != 0 || !strings.HasPrefix(entry.Name(), registryHomePrefix) {
			continue
		}
		home := filepath.Join(root, entry.Name())
		owner, err := openRegistryOwner(home)
		if err != nil {
			continue
		}
		if err := unix.Flock(int(owner.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			_ = owner.Close()
			continue
		}
		_ = os.RemoveAll(home)
		_ = owner.Close()
	}
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
