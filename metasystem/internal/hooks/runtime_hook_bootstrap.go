package hooks

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
)

// The engine rebuild a hook starts never runs inside the hook event: the
// hook claims the bootstrap fence, starts the build detached, points the
// fence at the builder and answers the event at once. The fence is a
// symbolic link whose target is the pid of its holder. A holder that is no
// longer alive leaves a stale fence the next claimant takes over, so a hook
// killed at any point, or a builder that ended, never blocks a later
// rebuild. The plumbing stub keeps the same fence in shell.

// BootstrapFencePath and BootstrapLogPath are installation-relative.
const (
	BootstrapFencePath = "artifacts/agents/hook-bootstrap.fence"
	BootstrapLogPath   = "artifacts/agents/hook-bootstrap.log"
)

// ClaimBootstrapFence claims the installation's bootstrap fence for owner.
// It reports false, without error, while a live holder has it.
func ClaimBootstrapFence(installation string, owner int, alive func(int) bool) (bool, error) {
	fence := filepath.Join(installation, filepath.FromSlash(BootstrapFencePath))
	if err := os.MkdirAll(filepath.Dir(fence), 0o755); err != nil {
		return false, err
	}
	err := os.Symlink(strconv.Itoa(owner), fence)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, fs.ErrExist) {
		return false, err
	}
	if BootstrapFenceHeld(installation, alive) {
		return false, nil
	}
	if info, err := os.Lstat(fence); err == nil && info.Mode()&fs.ModeSymlink == 0 {
		return false, errors.New("the bootstrap fence " + fence + " is not a fence link")
	}
	return true, PointBootstrapFence(installation, owner)
}

// BootstrapFenceHeld reports whether a live holder has the fence.
func BootstrapFenceHeld(installation string, alive func(int) bool) bool {
	target, err := os.Readlink(filepath.Join(installation, filepath.FromSlash(BootstrapFencePath)))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(target)
	return err == nil && pid > 0 && alive(pid)
}

// PointBootstrapFence names pid as the fence holder, replacing the link in
// one rename.
func PointBootstrapFence(installation string, pid int) error {
	fence := filepath.Join(installation, filepath.FromSlash(BootstrapFencePath))
	staged := fence + "." + strconv.Itoa(os.Getpid())
	_ = os.Remove(staged)
	if err := os.Symlink(strconv.Itoa(pid), staged); err != nil {
		return err
	}
	if err := os.Rename(staged, fence); err != nil {
		_ = os.Remove(staged)
		return err
	}
	return nil
}
