package proofrun

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"golang.org/x/sys/unix"
)

// A lease is one run-invariant slot directory for a group's managed
// environment and candidate worktree: <store>/leases/<policy>/<key>/<slot>.
// Go's test cache hashes the strings of every path and variable a test reads,
// so sequential runs of the same group must see the same strings: they take
// slot 0 again; only a run overlapping a live owner takes a higher slot.
//
// Ownership is the file <slot>.lease, changed only under its exclusive flock
// and holding the owning run's root. A slot is free when its owner is empty
// or the owner's scratch record no longer exists. A crashed owner keeps its
// record, so its slot stays taken until ReconcileScratch proves every writer
// dead, removes the recorded worktrees and releases the lease. Liveness is
// never probed by taking another run's writer lock, which would make that
// run's own nonblocking cleanup fail.
//
// What stays on disk per (control root, policy, group environment identity)
// after every run has ended: the directory <key>/ holding one zero-byte
// <slot>.lease lock file per slot ever used concurrently (slot 0 for
// sequential runs); release removes the slot's tree. The lock files are kept
// because unlinking a file others may flock would let two claimants lock
// different inodes of one name. A crashed run's slot tree stays until
// ReconcileScratch proves it dead, like its run root.
const (
	scratchLeaseDir      = "leases"
	scratchLeaseMaxSlots = 64
	scratchLeaseWorktree = "wt"
)

// scratchLeaseKey names one group environment identity under one policy.
func scratchLeaseKey(policy, groupID, environmentDigest string) string {
	digest := sha256.Sum256([]byte(policy + "\x00" + groupID + "\x00" + environmentDigest))
	return hex.EncodeToString(digest[:12])
}

func (r *ScratchRun) leaseRoot(policy string) string {
	return filepath.Join(filepath.Dir(r.root), scratchLeaseDir, strings.ReplaceAll(policy, "/", "-"))
}

// ClaimLease takes the lowest free slot for key and records it in the run's
// durable record before returning; a slot this run already owns is returned
// again. The returned directory exists and is empty of any earlier tree.
func (r *ScratchRun) ClaimLease(policy, key string) (string, error) {
	if key == "" || filepath.Base(key) != key {
		return "", fmt.Errorf("scratch lease: key %q is not one path element", key)
	}
	parent := filepath.Join(r.leaseRoot(policy), key)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", fmt.Errorf("scratch lease: %w", err)
	}
	for slot := 0; slot < scratchLeaseMaxSlots; slot++ {
		dir := filepath.Join(parent, strconv.Itoa(slot))
		claimed, err := r.claimLeaseSlot(dir)
		if err != nil {
			return "", err
		}
		if claimed {
			return dir, nil
		}
	}
	return "", fmt.Errorf("scratch lease: all %d slots of %s are taken", scratchLeaseMaxSlots, parent)
}

func (r *ScratchRun) claimLeaseSlot(dir string) (bool, error) {
	file, owner, err := lockLease(dir)
	if err != nil {
		return false, err
	}
	defer releaseScratchLock(file)
	if owner == r.root {
		return true, nil
	}
	if owner != "" {
		if _, err := os.Lstat(owner + ".json"); !errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
	}
	// A tree left by an owner whose record is gone is emptied unless it
	// still holds a checkout, which only its recorded tuple may remove.
	if _, err := os.Lstat(filepath.Join(dir, scratchLeaseWorktree)); err == nil {
		if entries, _ := filepath.Glob(filepath.Join(dir, scratchLeaseWorktree, "*", ".git")); len(entries) != 0 {
			return false, nil
		}
	}
	if err := os.RemoveAll(dir); err != nil {
		return false, fmt.Errorf("scratch lease: empty %s: %w", dir, err)
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return false, fmt.Errorf("scratch lease: %w", err)
	}
	if err := r.mutate(func(record *ScratchRecord) error {
		if !slices.Contains(record.Leases, dir) {
			record.Leases = append(record.Leases, dir)
		}
		return nil
	}); err != nil {
		return false, err
	}
	return true, writeLeaseOwner(file, r.root)
}

// lockLease opens and exclusively locks <dir>.lease and reads its owner.
func lockLease(dir string) (*os.File, string, error) {
	file, err := os.OpenFile(dir+".lease", os.O_RDWR|os.O_CREATE|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("scratch lease: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		_ = file.Close()
		return nil, "", fmt.Errorf("scratch lease lock: %w", err)
	}
	owner, err := io.ReadAll(io.LimitReader(file, 4096))
	if err != nil {
		releaseScratchLock(file)
		return nil, "", fmt.Errorf("scratch lease owner: %w", err)
	}
	return file, strings.TrimSpace(string(owner)), nil
}

func writeLeaseOwner(file *os.File, owner string) error {
	if err := file.Truncate(0); err != nil {
		return fmt.Errorf("scratch lease owner: %w", err)
	}
	if _, err := file.WriteAt([]byte(owner), 0); err != nil {
		return fmt.Errorf("scratch lease owner: %w", err)
	}
	return file.Sync()
}

// leaseOwnedBy reports whether record's run still owns the slot at dir.
func leaseOwnedBy(dir string, record ScratchRecord) bool {
	file, owner, err := lockLease(dir)
	if err != nil {
		return false
	}
	defer releaseScratchLock(file)
	return owner == record.Root
}

// leaseOfWorktree names the recorded lease holding tuple, if any.
func leaseOfWorktree(record ScratchRecord, tuple ScratchWorktree) (string, bool) {
	for _, dir := range record.Leases {
		if filepath.Dir(tuple.Parent) == dir {
			return dir, true
		}
	}
	return "", false
}

// releaseScratchLeases removes the tree of and frees every slot the run still
// owns. The caller holds the run's writer lock through a fresh description
// and has removed the run's recorded worktrees, so no writer uses the tree.
func releaseScratchLeases(record ScratchRecord) (string, error) {
	for _, dir := range record.Leases {
		file, owner, err := lockLease(dir)
		if err != nil {
			return "lease " + dir, errors.Join(errScratchPending, err)
		}
		if owner == record.Root {
			// The slot's tree goes; only the path's name must be stable
			// (Go hashes strings), and the next claim recreates it.
			err = os.RemoveAll(dir)
			if err == nil {
				err = writeLeaseOwner(file, "")
			}
		}
		releaseScratchLock(file)
		if err != nil {
			return "lease " + dir, errors.Join(errScratchPending, err)
		}
	}
	return "", nil
}

// PlanLeasedWorktree reserves the candidate worktree of a slot this run
// owns at its run-invariant path <slot>/wt/<name>, the name unique per slot
// because Git names the worktree's admin entry after it.
func (r *ScratchRun) PlanLeasedWorktree(workspace gittree.Workspace, lease string) (*gittree.WorktreePlan, error) {
	r.mu.Lock()
	leases := append([]string(nil), r.record.Leases...)
	r.mu.Unlock()
	if !slices.Contains(leases, lease) {
		if err := r.mutate(func(record *ScratchRecord) error { leases = record.Leases; return nil }); err != nil {
			return nil, err
		}
		if !slices.Contains(leases, lease) {
			return nil, fmt.Errorf("scratch lease %s is not held by run %s", lease, r.run)
		}
	}
	name := "wt-" + filepath.Base(filepath.Dir(lease)) + "-" + filepath.Base(lease)
	workspace.Materialize = r.Materialization()
	plan, err := workspace.PlanDetachedWorktreeAt(filepath.Join(lease, scratchLeaseWorktree), name)
	if err != nil {
		return nil, err
	}
	return r.recordPlan(plan, "groups")
}
