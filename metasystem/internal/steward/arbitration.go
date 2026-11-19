package steward

// The shared arbitration lock: worker enrollment and steward
// reservation serialize on the same file, so a worker enrolling and
// a steward reserving can never interleave between check and launch.
// The steward holds it from the final predicate re-run through the
// dispatch return — one critical section, one contender wins.

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"golang.org/x/sys/unix"
)

// ArbitrationLockPath is the one file both sides lock.
func ArbitrationLockPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "arbitration.flock")
}

// ArbitrationLock is a held exclusive lock.
type ArbitrationLock struct{ f *os.File }

// ErrCheckoutGone means the root supplied for arbitration no longer exists.
var ErrCheckoutGone = errors.New("checkout is gone")

func arbitrationCheckoutExists(root string) error {
	info, err := os.Stat(root)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrCheckoutGone, root)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("arbitration checkout %s is not a directory", root)
	}
	return nil
}

func arbitrationPathError(root string, err error) error {
	if errors.Is(err, os.ErrNotExist) {
		if rootErr := arbitrationCheckoutExists(root); rootErr != nil {
			return rootErr
		}
	}
	return err
}

func prepareArbitrationDirectory(root string) error {
	if err := arbitrationCheckoutExists(root); err != nil {
		return err
	}
	// Create only descendants, one level at a time. Recursive parent creation
	// could recreate the checkout if it disappeared after the existence check.
	dir := root
	for _, part := range []string{"artifacts", "agents", "steward"} {
		dir = filepath.Join(dir, part)
		if err := os.Mkdir(dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
			return arbitrationPathError(root, err)
		}
	}
	return nil
}

var beforeArbitrationWait = func() {}

// arbitrationWantPath is where a blocking acquirer queues: it holds this
// file shared while it waits, and the disk sweeper, which only ever takes
// arbitration without waiting, yields to any queued waiter. So a hook that
// waits for arbitration during a sweep waits at most one nonce's critical
// section (Part B 3.3).
func arbitrationWantPath(repoRoot string) string { return ArbitrationLockPath(repoRoot) + ".want" }

// AcquireArbitration blocks until the critical section is ours. An
// uncontended acquisition takes the lock at once and writes nothing more; a
// contended one queues on the want file first, so the disk sweeper yields.
func AcquireArbitration(repoRoot string) (*ArbitrationLock, error) {
	path := ArbitrationLockPath(repoRoot)
	if err := prepareArbitrationDirectory(repoRoot); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, arbitrationPathError(repoRoot, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
		return &ArbitrationLock{f: f}, nil
	} else if !errors.Is(err, unix.EWOULDBLOCK) && !errors.Is(err, unix.EAGAIN) {
		f.Close()
		return nil, err
	}
	want, err := os.OpenFile(arbitrationWantPath(repoRoot), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		unlockAndClose(f)
		return nil, arbitrationPathError(repoRoot, err)
	}
	// The queued hold ends with an unlock: a sibling's fork copy of the
	// want description would otherwise keep the sweeper yielding.
	defer unlockAndClose(want)
	if err := flockWaiting(want, unix.LOCK_SH); err != nil {
		unlockAndClose(f)
		return nil, err
	}
	beforeArbitrationWait()
	if err := flockWaiting(f, unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &ArbitrationLock{f: f}, nil
}

func flockWaiting(f *os.File, operation int) error {
	for {
		err := unix.Flock(int(f.Fd()), operation)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}

// ErrArbitrationHeld is a nonblocking acquisition finding the lock held.
var ErrArbitrationHeld = errors.New("steward arbitration is held")

// TryAcquireArbitration takes the lock LOCK_EX|LOCK_NB: the disk sweeper's
// acquisition, which never waits inside a pass (Part B R14) and yields to a
// queued blocking acquirer: a held lock or a queued waiter is
// ErrArbitrationHeld.
func TryAcquireArbitration(repoRoot string) (*ArbitrationLock, error) {
	path := ArbitrationLockPath(repoRoot)
	if err := prepareArbitrationDirectory(repoRoot); err != nil {
		return nil, err
	}
	// A queued waiter holds the want file shared; the sweeper yields to it.
	// No want file means no waiter has ever queued, and none is created here.
	if want, err := os.OpenFile(arbitrationWantPath(repoRoot), os.O_RDONLY, 0); err == nil {
		queued := unix.Flock(int(want.Fd()), unix.LOCK_EX|unix.LOCK_NB)
		unlockAndClose(want)
		if queued != nil {
			if errors.Is(queued, unix.EWOULDBLOCK) || errors.Is(queued, unix.EAGAIN) {
				return nil, ErrArbitrationHeld
			}
			return nil, queued
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, arbitrationPathError(repoRoot, err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, ErrArbitrationHeld
		}
		return nil, err
	}
	return &ArbitrationLock{f: f}, nil
}

// ProbeArbitration reports whether the lock is free without creating the
// lock file or its directory and without holding anything afterwards: the
// plan phase's probe (Part B R15). An absent lock file in an existing
// checkout reads as free; an absent checkout returns ErrCheckoutGone.
func ProbeArbitration(repoRoot string) (free bool, err error) {
	if err := arbitrationCheckoutExists(repoRoot); err != nil {
		return false, err
	}
	f, err := os.OpenFile(ArbitrationLockPath(repoRoot), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		if rootErr := arbitrationCheckoutExists(repoRoot); rootErr != nil {
			return false, rootErr
		}
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer unlockAndClose(f)
	if err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Release ends the critical section.
func (l *ArbitrationLock) Release() {
	unlockAndClose(l.f)
}

// unlockAndClose ends a hold on file: the flock is released on the open file
// description first, so a duplicate a concurrent fork made before its exec
// cannot keep the lock after the holder has ended. Unlocking a description
// that holds nothing is a no-op.
func unlockAndClose(file *os.File) {
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	_ = file.Close()
}

// The enrollment fence: a counter every enrollment bumps under the
// lock. The steward records it at reservation and re-checks it
// before launch — a bump in between cancels the reservation.
func fencePath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "enrollment-fence")
}

// ReadEnrollmentFence returns the current generation; absent = 0.
func ReadEnrollmentFence(repoRoot string) (int64, error) {
	data, err := os.ReadFile(fencePath(repoRoot))
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	var n int64
	if _, err := fmt.Sscanf(string(data), "%d", &n); err != nil {
		return 0, fmt.Errorf("enrollment fence malformed: %w", err)
	}
	return n, nil
}

// BumpEnrollmentFence advances the generation; callers hold the
// arbitration lock.
func BumpEnrollmentFence(repoRoot string) error {
	n, err := ReadEnrollmentFence(repoRoot)
	if err != nil {
		return err
	}
	path := fencePath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err = atomicfile.WriteFile(path, []byte(fmt.Sprintf("%d\n", n+1)), 0o644, "")
	return err
}
