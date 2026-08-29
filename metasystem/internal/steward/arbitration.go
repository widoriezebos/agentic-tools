package steward

// The shared arbitration lock: worker enrollment and steward
// reservation serialize on the same file, so a worker enrolling and
// a steward reserving can never interleave between check and launch.
// The steward holds it from the final predicate re-run through the
// dispatch return — one critical section, one contender wins.

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// ArbitrationLockPath is the one file both sides lock.
func ArbitrationLockPath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "arbitration.flock")
}

// ArbitrationLock is a held exclusive lock.
type ArbitrationLock struct{ f *os.File }

var beforeArbitrationWait = func() {}

// AcquireArbitration blocks until the critical section is ours.
func AcquireArbitration(repoRoot string) (*ArbitrationLock, error) {
	path := ArbitrationLockPath(repoRoot)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	beforeArbitrationWait()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &ArbitrationLock{f: f}, nil
}

// Release ends the critical section.
func (l *ArbitrationLock) Release() {
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
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
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(fmt.Sprintf("%d\n", n+1)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
