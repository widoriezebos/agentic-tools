package batch

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const defaultTestRunLock, defaultTestRunQueue, batchOwnerSeat = "/tmp/metasystem-testrun-lock", "/tmp/metasystem-testrun-queue", "landing-batch-owner"

// ProofLockOwner reports the owner record for the configured proof lock.
// An empty directory selects the same package default used by newProofLock.
func ProofLockOwner(lockDir string) string {
	data, err := os.ReadFile(filepath.Join(cmp.Or(lockDir, defaultTestRunLock), "owner"))
	if err != nil {
		return "free"
	}
	return strings.TrimSpace(string(data))
}

type lockPoll uint8

const lockAcquired, lockQueued, lockStaleRemoved lockPoll = 0, 1, 2

type proofLock struct {
	lockDir, queueDir, purpose, entry string
	pid                               int64
	now                               func() time.Time
	prober                            identity.Prober
	held                              bool
}

func newProofLock(store Store, lockDir, queueDir, purpose string, pid int64, now func() time.Time) *proofLock {
	return &proofLock{lockDir: cmp.Or(lockDir, defaultTestRunLock), queueDir: cmp.Or(queueDir, defaultTestRunQueue), purpose: purpose, pid: pid, now: now, prober: store.seams.prober}
}
func (lock *proofLock) pidDead(pid int64) bool {
	_, state, _ := lock.prober.Probe(pid)
	return state == identity.Dead
}
func queuePaths(dir string) ([]string, error) { return filepath.Glob(filepath.Join(dir, "[0-9]*")) }
func (lock *proofLock) cleanStale(now time.Time) (bool, error) {
	removed := false
	paths, err := queuePaths(lock.queueDir)
	if err != nil {
		return false, err
	}
	for _, path := range paths {
		name := filepath.Base(path)
		pid, parseErr := strconv.ParseInt(name[strings.LastIndex(name, "-")+1:], 10, 64)
		if parseErr == nil && lock.pidDead(pid) {
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return false, err
			}
			removed = true
		}
	}
	owner, ownerErr := os.ReadFile(filepath.Join(lock.lockDir, "owner"))
	if fields := strings.Fields(string(owner)); ownerErr == nil && len(fields) >= 2 {
		pid, parseErr := strconv.ParseInt(fields[1], 10, 64)
		if parseErr == nil && lock.pidDead(pid) {
			return true, os.RemoveAll(lock.lockDir)
		}
		return removed, nil
	}
	// An owner file without a pid counts as no owner, as testrun-lock.sh's awk reads it.
	info, statErr := os.Stat(lock.lockDir)
	if statErr == nil && (ownerErr == nil || os.IsNotExist(ownerErr)) && now.Sub(info.ModTime()) > 2*time.Minute {
		return true, os.RemoveAll(lock.lockDir)
	}
	return removed, nil
}
func (lock *proofLock) poll() (lockPoll, error) {
	now := lock.now().UTC()
	if lock.entry == "" {
		if err := os.MkdirAll(lock.queueDir, 0o755); err != nil {
			return lockQueued, err
		}
		lock.entry = filepath.Join(lock.queueDir, fmt.Sprintf("%d-%s-%d", now.Unix(), batchOwnerSeat, lock.pid))
		line := fmt.Sprintf("%s %d %s %s\n", batchOwnerSeat, lock.pid, now.Format(time.RFC3339), lock.purpose)
		if err := os.WriteFile(lock.entry, []byte(line), 0o644); err != nil {
			lock.entry = ""
			return lockQueued, err
		}
	}
	if removed, err := lock.cleanStale(now); err != nil || removed {
		return lockStaleRemoved, err
	}
	paths, err := queuePaths(lock.queueDir)
	if err != nil {
		return lockQueued, err
	}
	if len(paths) == 0 || paths[0] != lock.entry {
		return lockQueued, nil
	}
	if err := os.Mkdir(lock.lockDir, 0o755); err != nil {
		if os.IsExist(err) {
			return lockQueued, nil
		}
		return lockQueued, err
	}
	owner := fmt.Sprintf("%s %d %s %s\n", batchOwnerSeat, lock.pid, now.Format(time.RFC3339), lock.purpose)
	if err := os.WriteFile(filepath.Join(lock.lockDir, "owner"), []byte(owner), 0o644); err != nil {
		return lockQueued, errors.Join(err, os.RemoveAll(lock.lockDir))
	}
	lock.held = true
	if err := os.Remove(lock.entry); err != nil {
		return lockQueued, errors.Join(err, lock.release())
	}
	lock.entry = ""
	return lockAcquired, nil
}
func (lock *proofLock) release() error {
	var entryErr, lockErr error
	if lock.entry != "" {
		entryErr = os.RemoveAll(lock.entry)
		lock.entry = ""
	}
	if lock.held {
		owner, err := os.ReadFile(filepath.Join(lock.lockDir, "owner"))
		fields := strings.Fields(string(owner))
		if err == nil && len(fields) >= 2 && fields[1] == strconv.FormatInt(lock.pid, 10) {
			lockErr = os.RemoveAll(lock.lockDir)
		}
		lock.held = false
	}
	return errors.Join(entryErr, lockErr)
}
func (lock *proofLock) whileHeld(run func() error) (err error) {
	defer func() { err = errors.Join(err, lock.release()) }()
	return run()
}
