package batch

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// DefaultProofLockDir is the root of the per-batch proof locks: each batch's
// mutex is its own directory <root>/batch-<id>, so batches prove side by side
// and the host admission cap, not a lock, bounds how many prove at once.
const DefaultProofLockDir = "/tmp/metasystem-batch-locks"

const defaultTestRunLock, defaultTestRunQueue, batchOwnerSeat = DefaultProofLockDir, "/tmp/metasystem-batch-queues", "landing-batch-owner"

// ProofLockOwner reports the owner record of every batch lock held under the
// configured lock root, or "free". An empty root selects the package default.
func ProofLockOwner(lockDir string) string {
	held, _ := filepath.Glob(filepath.Join(cmp.Or(lockDir, defaultTestRunLock), "batch-*", "owner"))
	var owners []string
	for _, path := range held {
		if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
			owners = append(owners, strings.TrimSpace(string(data)))
		}
	}
	if len(owners) == 0 {
		return "free"
	}
	return strings.Join(owners, "; ")
}

// newBatchProofLock builds one batch's own lock: its mutex directory is
// <lockRoot>/batch-<id> and its queue registrations live in <queueRoot>/batch-<id>.
func newBatchProofLock(store Store, lockRoot, queueRoot, id string, pid int64, now func() time.Time) *proofLock {
	return newProofLock(store, filepath.Join(cmp.Or(lockRoot, defaultTestRunLock), "batch-"+id), filepath.Join(cmp.Or(queueRoot, defaultTestRunQueue), "batch-"+id), id, pid, now)
}

type lockPoll uint8

const lockAcquired, lockQueued lockPoll = 0, 1

type proofLock struct {
	lockDir, queueDir, purpose, entry string
	key                               string // the batch id: each batch has its own queue registration
	refused                           error
	pid                               int64
	now                               func() time.Time
	prober                            identity.Prober
	held                              bool
}

// newProofLock builds the lock for one key, the batch id, over the directories
// newBatchProofLock derives from it. The key names the queue registration, so
// it may hold no "-": the last "-" segment is the pid.
func newProofLock(store Store, lockDir, queueDir, key string, pid int64, now func() time.Time) *proofLock {
	lock := &proofLock{lockDir: lockDir, queueDir: queueDir, purpose: "batch:" + key, key: key, pid: pid, now: now, prober: store.seams.prober}
	if key == "" || strings.Contains(key, "-") {
		lock.refused = fmt.Errorf("proof lock key %q must be non-empty and hold no \"-\"", key)
	}
	return lock
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
	if lock.refused != nil {
		return lockQueued, lock.refused
	}
	now := lock.now().UTC()
	if lock.entry == "" {
		if err := os.MkdirAll(lock.queueDir, 0o755); err != nil {
			return lockQueued, err
		}
		lock.entry = filepath.Join(lock.queueDir, fmt.Sprintf("%d-%s-%s-%d", now.Unix(), batchOwnerSeat, lock.key, lock.pid))
		line := fmt.Sprintf("%s %d %s %s\n", batchOwnerSeat, lock.pid, now.Format(time.RFC3339), lock.purpose)
		if err := os.WriteFile(lock.entry, []byte(line), 0o644); err != nil {
			lock.entry = ""
			return lockQueued, err
		}
	}
	if _, err := lock.cleanStale(now); err != nil {
		return lockQueued, err
	}
	paths, err := queuePaths(lock.queueDir)
	if err != nil {
		return lockQueued, err
	}
	if len(paths) == 0 || paths[0] != lock.entry {
		return lockQueued, nil
	}
	if err := os.MkdirAll(filepath.Dir(lock.lockDir), 0o755); err != nil {
		return lockQueued, err
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

// retire releases the lock and removes the batch's own queue directory once
// it is empty. The directory is <queueRoot>/batch-<id> for a key the
// constructor validated; only an empty directory is removed, never a tree, so
// a registration another process wrote in the meantime survives.
func (lock *proofLock) retire() error {
	err := lock.release()
	if lock.refused != nil || filepath.Base(lock.queueDir) != "batch-"+lock.key {
		return err
	}
	if removeErr := os.Remove(lock.queueDir); removeErr != nil && !os.IsNotExist(removeErr) && !isDirectoryNotEmpty(removeErr) {
		err = errors.Join(err, removeErr)
	}
	return err
}

func isDirectoryNotEmpty(err error) bool {
	var pathErr *os.PathError
	return errors.As(err, &pathErr) && (errors.Is(pathErr.Err, syscall.ENOTEMPTY) || errors.Is(pathErr.Err, syscall.EEXIST))
}

func (lock *proofLock) whileHeld(run func() error) (err error) {
	defer func() { err = errors.Join(err, lock.release()) }()
	return run()
}
