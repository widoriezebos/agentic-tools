package diskstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// hostSharedNames are the host paths that must not follow a process's
// TMPDIR: locks that a parent with a process-scratch TMPDIR and its child
// with a nested one must contend for (DL2-15).
var hostSharedNames = map[string]bool{
	"metasystem-seat-launch.lock": true,
}

var (
	hostTempOnce  sync.Once
	hostTempValue string
	hostTempErr   error
)

// HostTempRoot is the host's temporary root, resolved once per process and
// independent of TMPDIR: on darwin the per-user confstr directory
// (_CS_DARWIN_USER_TEMP_DIR, /var/folders/…/T), on Linux /tmp.
func HostTempRoot() (string, error) {
	hostTempOnce.Do(func() {
		root, err := readHostTempRoot()
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
	if !hostSharedNames[name] {
		return "", fmt.Errorf("%q is not a shared host path; disposable files go in the process's scratch", name)
	}
	root, err := HostTempRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name), nil
}

// ProcessTempRoot is the process's own temporary root, os.TempDir(), for
// the validation readers whose meaning is "under this process's temp root"
// (R1's third class). It is the one sanctioned reader outside testenv.
func ProcessTempRoot() string { return os.TempDir() }
