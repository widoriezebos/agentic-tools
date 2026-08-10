package lease

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"golang.org/x/sys/unix"
)

// lockWaitSeconds bounds how long a claim/renew waits for the lease lock. A
// blocking acquire would deadlock the legitimate nested case — an arming that
// runs inside an operation already holding the lease — into an unexplained
// hang, so acquisition is bounded and turns that into a plain refusal.
func lockWaitSeconds() float64 {
	if v := os.Getenv("METASYSTEM_LEASE_LOCK_WAIT_SEC"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
			return f
		}
	}
	return 10
}

type fileLock struct{ f *os.File }

// acquireBounded takes the exclusive lease lock, creating the lock file on
// first use, and never blocks forever. what names the operation for the
// refusal message.
func acquireBounded(path, what string) (*fileLock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, fmt.Errorf("checkout lease lock cannot be opened: %w", err)
	}
	wait := lockWaitSeconds()
	deadline := time.Now().Add(time.Duration(wait * float64(time.Second)))
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			return &fileLock{f: f}, nil
		}
		if time.Now().After(deadline) {
			f.Close()
			return nil, fmt.Errorf("checkout lease lock is busy for %s after %gs; "+
				"another lease-gated operation holds it. If this call is nested inside "+
				"one, pass --lease-held; otherwise retry in a moment.", what, wait)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func (l *fileLock) release() {
	if l == nil || l.f == nil {
		return
	}
	_ = unix.Flock(int(l.f.Fd()), unix.LOCK_UN)
	_ = l.f.Close()
}
