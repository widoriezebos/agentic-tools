package diskstore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// hostSharedNames are the host paths that must not follow a process's
// TMPDIR: locks that a parent with a process-scratch TMPDIR and its child
// with a nested one must contend for (DL2-15).
var hostSharedNames = map[string]bool{
	"metasystem-seat-launch.lock": true,
}

// hostSharedPrefixes are the shared host directories named per owner: a
// landing lane's retained-verification worktrees, which a later owner of the
// same lane must find whatever its TMPDIR (the name ends in the lane's
// control-root digest, lower-case hex).
var hostSharedPrefixes = []string{"metasystem-batch-sources-"}

func hostSharedName(name string) bool {
	if hostSharedNames[name] {
		return true
	}
	for _, prefix := range hostSharedPrefixes {
		suffix, ok := strings.CutPrefix(name, prefix)
		if ok && suffix != "" && strings.Trim(suffix, "0123456789abcdef") == "" {
			return true
		}
	}
	return false
}

// HostTempRootEnv, set to an absolute directory, is the host temporary root
// in place of the system's: a test binary's namespace sets it (testenv) so
// no test touches the host's real temporary directory, and every process it
// starts inherits it, whatever TMPDIR that process is given. Production sets
// nothing.
const HostTempRootEnv = "METASYSTEM_HOST_TEMP_ROOT"

var (
	hostTempOnce  sync.Once
	hostTempValue string
	hostTempErr   error
)

// HostTempRoot is the host's temporary root, resolved once per process and
// independent of TMPDIR: on darwin the per-user confstr directory
// (_CS_DARWIN_USER_TEMP_DIR, /var/folders/…/T), on Linux /tmp; or the
// directory HostTempRootEnv names.
func HostTempRoot() (string, error) {
	hostTempOnce.Do(func() {
		root, err := os.Getenv(HostTempRootEnv), error(nil)
		if root == "" {
			root, err = readHostTempRoot()
		}
		if err == nil && !filepath.IsAbs(root) {
			err = fmt.Errorf("the host temporary root %q is not absolute", root)
		}
		hostTempValue, hostTempErr = filepath.Clean(root), err
	})
	return hostTempValue, hostTempErr
}

// HostShared is an allowlisted shared host path under HostTempRoot. An
// unknown name is refused: a disposable allocation belongs in process
// scratch, never in a shared host path.
func HostShared(name string) (string, error) {
	if !hostSharedName(name) {
		return "", fmt.Errorf("%q is not a shared host path; disposable files go in the process's scratch", name)
	}
	root, err := HostTempRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

// HostTempRoots are the temporary roots of the host that no process's
// TMPDIR moves: HostTempRoot and /tmp, each with its links resolved, for a
// check that a path is a host temporary path whatever TMPDIR the checking
// process was given (the proof-admission test directory). A root that
// cannot be resolved is left out.
func HostTempRoots() []string {
	var roots []string
	candidates := []string{"/tmp"}
	if root, err := HostTempRoot(); err == nil {
		candidates = append([]string{root}, candidates...)
	}
	for _, candidate := range candidates {
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		seen := false
		for _, root := range roots {
			seen = seen || root == resolved
		}
		if !seen {
			roots = append(roots, resolved)
		}
	}
	return roots
}

// ProcessTempRoot is the process's own temporary root, os.TempDir(), for
// the validation readers whose meaning is "under this process's temp root"
// (R1's third class). It is the one sanctioned reader outside testenv.
func ProcessTempRoot() string { return os.TempDir() }
