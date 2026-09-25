package proofrun

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// One test run owns exactly one scratch root inside the control root's
// scratch store. The record is written before the root receives bytes; the
// writer lock is one open file description that every root-writing child
// inherits, so the lock lives while any writer lives. The launcher removes
// the root only after it closed its own copy (never LOCK_UN: that would
// release every inheritor too) and took the lock again through a distinct
// description. A crashed run's root is recovered only after every recorded
// writer reads Dead and the lock is taken exclusively.
const (
	scratchSchema     = "metasystem.scratch.v1"
	scratchMarkerName = ".metasystem-scratch"
	scratchLockName   = ".writer-lock"

	ScratchWorktreeReserved = "reserved"
	ScratchWorktreeClosed   = "closed"

	ReconcileScratchRemoved = "scratch-removed"
	ReconcileScratchRefused = "scratch-refused"
	ReconcileScratchPending = "scratch-pending"

	// ScratchIncomplete prefixes every normal-cleanup failure the launcher
	// reports; the record and root stay for recovery.
	ScratchIncomplete = "cleanup.incomplete"
)

// ScratchSubdirectories are created with every root.
var ScratchSubdirectories = []string{"engine", "groups", "gocache", "staticcheck", "goenv", "freeze", "no-hooks"}

// ScratchWorktree is one recorded worktree tuple, reserved before git
// worktree add runs and closed after its removal.
type ScratchWorktree struct {
	Group   string `json:"group"`
	Parent  string `json:"parent"`
	Top     string `json:"top"`
	Control string `json:"control"`
	Common  string `json:"common"`
	State   string `json:"state"`
}

func (w ScratchWorktree) tuple() gittree.WorktreeTuple {
	return gittree.WorktreeTuple{Parent: w.Parent, Top: w.Top, Control: w.Control, Common: w.Common}
}

// ScratchRecord is `<store>/<run>.json`.
type ScratchRecord struct {
	Schema     string             `json:"schema"`
	Run        string             `json:"run"`
	Root       string             `json:"root"`
	Launcher   string             `json:"launcher"`
	Attempt    string             `json:"attempt,omitempty"`
	Custodians []ScratchCustodian `json:"custodians,omitempty"`
	Worktrees  []ScratchWorktree  `json:"worktrees,omitempty"`
}

// ScratchCustodian is one resource custodian that inherited the writer lock.
// Every custodian is kept: concurrent preparation commands each add one.
type ScratchCustodian struct {
	Ref   string `json:"ref"`
	Group int64  `json:"group"`
}

// ScratchLocator is the serialized binding a worker receives in its packet:
// the run, its root and the descriptor number of the inherited writer lock.
// The worker authenticates all three before it writes (OpenScratchRun).
type ScratchLocator struct {
	Run  string `json:"run"`
	Root string `json:"root"`
	FD   int    `json:"fd"`
}

// ScratchWriterFD is the descriptor number a child sees for the writer lock
// when it follows hostFiles in ExtraFiles (LaunchSuite's order).
func ScratchWriterFD(hostFiles []*os.File) int { return 3 + len(hostFiles) }

type scratchMarker struct {
	Schema string `json:"schema"`
	Run    string `json:"run"`
	Root   string `json:"root"`
}

// ScratchStore is the control root's scratch store, beside candidate-engines
// so that engine publication from a run root is a same-filesystem rename.
func ScratchStore(control string) string {
	return filepath.Join(control, "artifacts", "agents", "proof-runs", "scratch")
}

// ScratchRun is a process's handle on one run root. The launcher owns it;
// a worker borrows it through OpenScratchRun and never removes it. Record
// mutations from either process serialize on `<store>/<run>.record-lock`, a
// description of its own that no child inherits, and always reload the
// durable record first.
type ScratchRun struct {
	mu         sync.Mutex
	recordPath string
	record     ScratchRecord
	writer     *os.File
	removed    bool
	borrowed   bool
	// run and root never change after construction; getters read them
	// without the mutex while mutate replaces record under it.
	run, root string
}

// CreateScratchRun records, creates and locks one run root.
func CreateScratchRun(control string) (_ *ScratchRun, err error) {
	store, err := containedScratchStore(control, true)
	if err != nil {
		return nil, err
	}
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("scratch launcher identity unavailable: %v", err)
	}
	launcher, err := identity.EncodeRef(self.Ref())
	if err != nil {
		return nil, err
	}
	id, err := newAttemptID(time.Now())
	if err != nil {
		return nil, err
	}
	id = "scratch-" + strings.TrimPrefix(id, "proof-")
	run := &ScratchRun{
		recordPath: filepath.Join(store, id+".json"),
		record:     ScratchRecord{Schema: scratchSchema, Run: id, Root: filepath.Join(store, id), Launcher: launcher},
		run:        id, root: filepath.Join(store, id),
	}
	if err := writeScratchRecordFile(run.recordPath, run.record); err != nil {
		return nil, err
	}
	// From here a failure leaves the record for the next reconcile pass
	// whenever bytes may already exist; only an untouched record is undone.
	if err := os.Mkdir(run.record.Root, 0o700); err != nil {
		return nil, errors.Join(fmt.Errorf("scratch root: %w", err), os.Remove(run.recordPath))
	}
	lock, err := os.OpenFile(filepath.Join(run.record.Root, scratchLockName), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("scratch writer lock: %w", err)
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("scratch writer lock: %w", err)
	}
	run.writer = lock
	defer func() {
		if err != nil {
			err = errors.Join(err, run.Cleanup(nil))
		}
	}()
	marker, err := json.Marshal(scratchMarker{Schema: scratchSchema, Run: id, Root: run.record.Root})
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(run.record.Root, scratchMarkerName), marker, 0o600); err != nil {
		return nil, fmt.Errorf("scratch marker: %w", err)
	}
	// A stub module keeps a Go command run inside scratch, and the outer
	// module's ./..., from reaching each other.
	if err := os.WriteFile(filepath.Join(run.record.Root, "go.mod"), []byte("module metasystem.scratch\n"), 0o600); err != nil {
		return nil, fmt.Errorf("scratch module stub: %w", err)
	}
	for _, name := range ScratchSubdirectories {
		if err := os.Mkdir(filepath.Join(run.record.Root, name), 0o700); err != nil {
			return nil, fmt.Errorf("scratch %s: %w", name, err)
		}
	}
	return run, nil
}

// Root is the run root; Dir names one of its subdirectories.
func (r *ScratchRun) Root() string           { return r.root }
func (r *ScratchRun) ID() string             { return r.run }
func (r *ScratchRun) Dir(name string) string { return filepath.Join(r.root, name) }

// Writer is the launcher's copy of the writer lock. Every root-writing child
// inherits it through ExtraFiles; the caller never unlocks or closes it.
func (r *ScratchRun) Writer() *os.File {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writer
}

// Materialization is the gittree plumbing for a Git child writing into this
// root: the writer lock inherited, repository hooks off, temp inside.
func (r *ScratchRun) Materialization() *gittree.Materialization {
	return &gittree.Materialization{InheritFiles: []*os.File{r.Writer()}, HooksPath: r.Dir("no-hooks"), TempDir: r.Dir("engine")}
}

// Locator is the worker's packet binding for this run, fd being the number
// the worker sees for the inherited writer lock.
func (r *ScratchRun) Locator(fd int) *ScratchLocator {
	return &ScratchLocator{Run: r.run, Root: r.root, FD: fd}
}

// RecordAttempt binds the admitted attempt whose process records name the
// run's suite writers.
func (r *ScratchRun) RecordAttempt(attempt string) error {
	return r.mutate(func(record *ScratchRecord) error {
		record.Attempt = attempt
		return nil
	})
}

// RecordCustodian adds a resource custodian before its workload may write.
func (r *ScratchRun) RecordCustodian(custodian identity.Ref, group int64) error {
	encoded, err := identity.EncodeRef(custodian)
	if err != nil {
		return err
	}
	return r.mutate(func(record *ScratchRecord) error {
		record.Custodians = append(record.Custodians, ScratchCustodian{Ref: encoded, Group: group})
		return nil
	})
}

// PlanWorktree reserves one exact tuple in the durable record before any git
// worktree add runs, under <root>/<sub>; the plan flips the tuple to closed
// only after its own removal (registration included) succeeded.
func (r *ScratchRun) PlanWorktree(workspace gittree.Workspace, sub string) (*gittree.WorktreePlan, error) {
	dir := r.Dir(sub)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	workspace.Materialize = r.Materialization()
	plan, err := workspace.PlanDetachedWorktreeIn(dir)
	if err != nil {
		return nil, err
	}
	tuple := ScratchWorktree{Group: sub, Parent: plan.Parent, Top: plan.Top, Control: plan.Control, Common: plan.Common, State: ScratchWorktreeReserved}
	if err := r.mutate(func(record *ScratchRecord) error {
		record.Worktrees = append(record.Worktrees, tuple)
		return nil
	}); err != nil {
		return nil, errors.Join(err, os.RemoveAll(plan.Parent))
	}
	plan.AfterClose = func(closeErr error) error {
		if closeErr != nil {
			return nil
		}
		return r.mutate(func(record *ScratchRecord) error {
			for index := range record.Worktrees {
				if record.Worktrees[index].Top == tuple.Top {
					record.Worktrees[index].State = ScratchWorktreeClosed
				}
			}
			return nil
		})
	}
	return plan, nil
}

func scratchRecordLockPath(recordPath string) string {
	return strings.TrimSuffix(recordPath, ".json") + ".record-lock"
}

// lockScratchRecord takes the cross-process record mutation lock.
func lockScratchRecord(recordPath string) (*os.File, error) {
	lock, err := os.OpenFile(scratchRecordLockPath(recordPath), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("scratch record lock: %w", err)
	}
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX); err != nil {
		_ = lock.Close()
		return nil, fmt.Errorf("scratch record lock: %w", err)
	}
	return lock, nil
}

// mutate reloads the durable record under the record lock, applies change
// and republishes it; a record that no longer names this run fails closed.
func (r *ScratchRun) mutate(change func(*ScratchRecord) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	lock, err := lockScratchRecord(r.recordPath)
	if err != nil {
		return err
	}
	defer lock.Close()
	current, err := r.reloadLocked()
	if err != nil {
		return err
	}
	if err := change(&current); err != nil {
		return err
	}
	if err := writeScratchRecordFile(r.recordPath, current); err != nil {
		return err
	}
	r.record = current
	return nil
}

// reloadLocked reads the durable record; the caller holds the record lock.
func (r *ScratchRun) reloadLocked() (ScratchRecord, error) {
	current, err := readScratchRecordFile(r.recordPath)
	if err != nil {
		return ScratchRecord{}, err
	}
	if current.Run != r.run || current.Root != r.root {
		return ScratchRecord{}, fmt.Errorf("scratch record %s no longer names run %s", r.recordPath, r.run)
	}
	return current, nil
}

func readScratchRecordFile(path string) (ScratchRecord, error) {
	var record ScratchRecord
	encoded, err := os.ReadFile(path)
	if err != nil {
		return record, err
	}
	if err := json.Unmarshal(encoded, &record); err != nil || record.Schema != scratchSchema {
		return ScratchRecord{}, fmt.Errorf("scratch record %s is unreadable: %v", path, err)
	}
	return record, nil
}

// writeScratchRecordFile publishes through a unique temporary file and an
// atomic rename, so concurrent writers never share a temporary name.
func writeScratchRecordFile(path string, record ScratchRecord) error {
	encoded, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	_, writeErr := temporary.Write(append(encoded, '\n'))
	syncErr := temporary.Sync()
	closeErr := temporary.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	if err := os.Rename(temporary.Name(), path); err != nil {
		_ = os.Remove(temporary.Name())
		return err
	}
	return nil
}

// removeScratchRecord deletes a run's record, its record lock and any
// temporary a crashed writer of this exact run left behind.
// The JSON record goes before the lock file only after every temporary is
// gone, so a failed temporary deletion keeps the record discoverable; a lock
// file whose record is gone is recovered by its exact run name in
// ReconcileScratch. The caller holds the record lock.
func removeScratchRecord(recordPath string, remove func(string) error) error {
	if remove == nil {
		remove = os.Remove
	}
	gone := func(path string) error {
		if err := remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	stale, err := filepath.Glob(recordPath + ".tmp-*")
	if err != nil {
		return err
	}
	var errs []error
	for _, path := range stale {
		errs = append(errs, gone(path))
	}
	if err := errors.Join(errs...); err != nil {
		return err
	}
	if err := gone(recordPath); err != nil {
		return err
	}
	return gone(scratchRecordLockPath(recordPath))
}

// scratchRunName is the exact run name CreateScratchRun issues.
var scratchRunName = regexp.MustCompile(`^scratch-[0-9a-z]+-[0-9a-f]{16}$`)

// containedScratchStore resolves the control root's store and refuses one
// whose resolution leaves the canonical control root (an external symlink),
// checking the existing prefix before create makes anything.
func containedScratchStore(control string, create bool) (string, error) {
	canonical, err := filepath.EvalSymlinks(control)
	if err != nil {
		return "", fmt.Errorf("scratch store: control root: %w", err)
	}
	store := ScratchStore(control)
	within := func(path string) error {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("scratch store: %w", err)
		}
		if rel, err := filepath.Rel(canonical, resolved); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			return fmt.Errorf("scratch store %s resolves to %s outside the control root %s", store, resolved, canonical)
		}
		return nil
	}
	existing := store
	for {
		if _, err := os.Lstat(existing); err == nil {
			break
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("scratch store: %w", err)
		}
		existing = filepath.Dir(existing)
	}
	if err := within(existing); err != nil {
		return "", err
	}
	if existing != store {
		if !create {
			return "", fmt.Errorf("scratch store: %w", os.ErrNotExist)
		}
		if err := os.MkdirAll(store, 0o700); err != nil {
			return "", fmt.Errorf("scratch store: %w", err)
		}
	}
	if err := within(store); err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(store)
}

// OpenScratchRun is the worker's authenticated reopen of the launcher's run.
// The locator must name this control root's store, the record must name the
// authenticated attempt, the marker must name the same run and root, and the
// descriptor must be the writer lock file itself holding the launcher's
// exclusive lock through the inherited description. The returned run is
// borrowed: its mutations reach the durable record, it never removes.
func OpenScratchRun(control, attempt string, locator ScratchLocator) (*ScratchRun, error) {
	fail := func(format string, args ...any) (*ScratchRun, error) {
		return nil, fmt.Errorf("scratch run authentication: "+format, args...)
	}
	store, err := containedScratchStore(control, false)
	if err != nil {
		return fail("store: %v", err)
	}
	if locator.Run == "" || filepath.Base(locator.Run) != locator.Run || !strings.HasPrefix(locator.Run, "scratch-") ||
		locator.Root != filepath.Join(store, locator.Run) || locator.FD < 3 {
		return fail("locator %+v does not name this control root's store", locator)
	}
	recordPath := filepath.Join(store, locator.Run+".json")
	record, err := readScratchRecordFile(recordPath)
	if err != nil {
		return fail("%v", err)
	}
	if record.Run != locator.Run || record.Root != locator.Root || attempt == "" || record.Attempt != attempt {
		return fail("record does not bind attempt %s", attempt)
	}
	encoded, err := os.ReadFile(filepath.Join(locator.Root, scratchMarkerName))
	var marker scratchMarker
	if err != nil || json.Unmarshal(encoded, &marker) != nil || marker.Schema != scratchSchema || marker.Run != record.Run || marker.Root != record.Root {
		return fail("marker does not name run %s", record.Run)
	}
	lockInfo, err := os.Lstat(filepath.Join(locator.Root, scratchLockName))
	if err != nil || !lockInfo.Mode().IsRegular() {
		return fail("writer lock file: %v", err)
	}
	var stat unix.Stat_t
	lockStat, ok := lockInfo.Sys().(*syscall.Stat_t)
	if err := unix.Fstat(locator.FD, &stat); err != nil || !ok ||
		uint64(stat.Dev) != uint64(lockStat.Dev) || uint64(stat.Ino) != uint64(lockStat.Ino) {
		return fail("descriptor %d is not the writer lock: %v", locator.FD, err)
	}
	// Only the launcher's own description can take the lock the launcher
	// holds; an independently opened description is refused here.
	if err := unix.Flock(locator.FD, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return fail("descriptor %d does not share the launcher's lock: %v", locator.FD, err)
	}
	writer := os.NewFile(uintptr(locator.FD), filepath.Join(locator.Root, scratchLockName))
	return &ScratchRun{recordPath: recordPath, record: record, writer: writer, borrowed: true, run: record.Run, root: record.Root}, nil
}

// Cleanup is the normal removal. Every open tuple is removed through its
// recorded proof first; then the launcher's lock copy is closed without
// unlocking, a distinct description takes the lock exclusively, and only
// after the root preconditions hold is the root removed and the record
// deleted. Any failure leaves the record (and the root) for recovery and
// returns an error that names ScratchIncomplete.
func (r *ScratchRun) Cleanup(remove func(gittree.WorktreeTuple, gittree.Workspace) (string, error)) error {
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.borrowed {
		return fmt.Errorf("scratch run %s is borrowed; only its launcher removes it", r.record.Run)
	}
	if r.removed {
		return nil
	}
	if r.writer != nil {
		// Close only this process's copy: an inheriting child keeps the
		// description, and with it the lock, alive.
		_ = r.writer.Close()
		r.writer = nil
	}
	// The worktrees a worker recorded live only in the durable record: it
	// is reloaded after the writers drained, under the record lock.
	lock, err := lockScratchRecord(r.recordPath)
	if err != nil {
		return fmt.Errorf("%s: scratch %s retained: %w", ScratchIncomplete, r.record.Root, err)
	}
	defer lock.Close()
	reason, err := removeScratchRoot(r.record, func() (string, error) {
		current, err := r.reloadLocked()
		if err != nil {
			return "record reload", errors.Join(errScratchPending, err)
		}
		r.record = current
		return removeRecordedWorktrees(current, ScratchOptions{RemoveWorktree: remove})
	})
	if err != nil {
		return fmt.Errorf("%s: scratch %s retained: %s: %w", ScratchIncomplete, r.record.Root, reason, err)
	}
	if err := removeScratchRecord(r.recordPath, nil); err != nil {
		return fmt.Errorf("%s: scratch record %s: %w", ScratchIncomplete, r.recordPath, err)
	}
	r.removed = true
	return nil
}

var errScratchPending = errors.New("pending")

// removeScratchRoot takes the writer lock through a fresh description and
// removes the root after its identity checks. The returned reason is the
// operator word; errScratchPending marks a live writer.
func removeScratchRoot(record ScratchRecord, whileLocked func() (string, error)) (string, error) {
	info, err := os.Lstat(record.Root)
	if errors.Is(err, os.ErrNotExist) {
		if reason, err := whileLocked(); err != nil {
			return reason, err
		}
		return "root absent", nil
	}
	if err != nil {
		return "root unreadable", errors.Join(errScratchPending, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return "root is not a directory", errors.New("refused")
	}
	if resolved, err := filepath.EvalSymlinks(record.Root); err != nil || resolved != record.Root {
		return "root does not resolve to the record", errors.New("refused")
	}
	markerPath := filepath.Join(record.Root, scratchMarkerName)
	lockPath := filepath.Join(record.Root, scratchLockName)
	encoded, markerErr := os.ReadFile(markerPath)
	if errors.Is(markerErr, os.ErrNotExist) {
		// An unmarked root is removed only while it is empty; creation order
		// puts the lock before the marker, so nothing else is provably ours.
		if len(record.Worktrees) != 0 {
			return "unmarked root with recorded worktrees", errors.New("refused")
		}
		if err := os.Remove(record.Root); err != nil {
			return "unmarked root is not empty", errors.New("refused")
		}
		return "unmarked empty root", nil
	}
	var marker scratchMarker
	if markerErr != nil || json.Unmarshal(encoded, &marker) != nil || marker.Schema != scratchSchema ||
		marker.Run != record.Run || marker.Root != record.Root {
		return "marker is foreign or unreadable", errors.New("refused")
	}
	if lockInfo, err := os.Lstat(lockPath); err != nil || !lockInfo.Mode().IsRegular() {
		return "marker without writer lock", errors.New("refused")
	}
	lock, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		return "writer lock unreadable", errors.Join(errScratchPending, err)
	}
	defer lock.Close()
	if err := unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return "writer-lock-held", errScratchPending
		}
		return "writer lock", errors.Join(errScratchPending, err)
	}
	if reason, err := whileLocked(); err != nil {
		return reason, err
	}
	if err := os.RemoveAll(record.Root); err != nil {
		return "remove root", err
	}
	return "removed", nil
}

// ScratchOptions injects the process prover, group reader and worktree
// remover used by recovery; zero values read the real kernel and Git.
type ScratchOptions struct {
	Prober         identity.Prober
	GroupMembers   func(pgid int64) ([]int64, error)
	RemoveWorktree func(gittree.WorktreeTuple, gittree.Workspace) (string, error)
	// RemoveFile deletes one record file; nil is os.Remove.
	RemoveFile func(string) error
	// Self is the calling run's own record, never recovered by itself.
	Self string
}

// ReconcileScratch recovers the scratch roots whose every recorded writer is
// proven dead. It never sweeps by prefix: only `<store>/<run>.json` records
// and the exact paths they name are considered.
func ReconcileScratch(control string, options ScratchOptions) []ReconcileOutcome {
	if options.Prober == nil {
		options.Prober = identity.KernelProber{}
	}
	if options.GroupMembers == nil {
		options.GroupMembers = liveGroupMembers
	}
	if options.RemoveWorktree == nil {
		options.RemoveWorktree = gittree.RemoveRecordedWorktree
	}
	store, err := containedScratchStore(control, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return []ReconcileOutcome{{AttemptID: "scratch", Action: ReconcileScratchRefused, Reason: err.Error()}}
	}
	entries, err := os.ReadDir(store)
	if err != nil {
		return []ReconcileOutcome{{AttemptID: "scratch", Action: ReconcileScratchPending, Reason: "scratch store unreadable: " + err.Error()}}
	}
	var names []string
	var outcomes []ReconcileOutcome
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, ".json") && entry.Type().IsRegular() && strings.TrimSuffix(name, ".json") != options.Self {
			names = append(names, name)
		}
		// A record lock whose exact run has no record left is the tail of
		// an interrupted terminal deletion.
		if run := strings.TrimSuffix(name, ".record-lock"); run != name && scratchRunName.MatchString(run) && run != options.Self && entry.Type().IsRegular() {
			if _, err := os.Lstat(filepath.Join(store, run+".json")); errors.Is(err, os.ErrNotExist) {
				outcomes = append(outcomes, reconcileScratchSidecars(store, run, options))
			}
		}
	}
	sort.Strings(names)
	for _, name := range names {
		run := strings.TrimSuffix(name, ".json")
		action, reason := reconcileScratchOne(control, store, run, options)
		outcomes = append(outcomes, ReconcileOutcome{AttemptID: "scratch:" + run, Action: action, Reason: reason})
	}
	return outcomes
}

func reconcileScratchSidecars(store, run string, options ScratchOptions) ReconcileOutcome {
	outcome := ReconcileOutcome{AttemptID: "scratch:" + run, Action: ReconcileScratchRemoved, Reason: "record sidecars removed"}
	recordPath := filepath.Join(store, run+".json")
	lock, err := lockScratchRecord(recordPath)
	if err == nil {
		defer lock.Close()
		if _, statErr := os.Lstat(recordPath); !errors.Is(statErr, os.ErrNotExist) {
			return ReconcileOutcome{AttemptID: outcome.AttemptID, Action: ReconcileScratchPending, Reason: "record reappeared"}
		}
		err = removeScratchRecord(recordPath, options.RemoveFile)
	}
	if err != nil {
		outcome.Action, outcome.Reason = ReconcileScratchPending, "record sidecars: "+err.Error()
	}
	return outcome
}

func reconcileScratchOne(control, store, run string, options ScratchOptions) (string, string) {
	recordPath := filepath.Join(store, run+".json")
	lock, err := lockScratchRecord(recordPath)
	if err != nil {
		return ReconcileScratchPending, err.Error()
	}
	defer lock.Close()
	encoded, err := os.ReadFile(recordPath)
	if errors.Is(err, os.ErrNotExist) {
		// Another cleanup removed the record first; its sidecars are ours
		// to finish, and a failure to do so is reported, not hidden.
		if err := removeScratchRecord(recordPath, options.RemoveFile); err != nil {
			return ReconcileScratchPending, "record sidecars: " + err.Error()
		}
		return ReconcileScratchRemoved, "record already removed"
	}
	if err != nil {
		return ReconcileScratchPending, "record unreadable: " + err.Error()
	}
	var record ScratchRecord
	if err := json.Unmarshal(encoded, &record); err != nil || record.Schema != scratchSchema || record.Run != run ||
		record.Root != filepath.Join(store, run) {
		return ReconcileScratchRefused, "record is not this store's scratch record"
	}
	if reason, dead := scratchWritersDead(control, record, options); !dead {
		return ReconcileScratchPending, reason
	}
	reason, err := removeScratchRootWithWorktrees(record, options)
	switch {
	case err == nil:
	case errors.Is(err, errScratchPending):
		return ReconcileScratchPending, reason
	default:
		return ReconcileScratchRefused, reason + ": " + err.Error()
	}
	if err := removeScratchRecord(recordPath, options.RemoveFile); err != nil {
		return ReconcileScratchPending, "record removal: " + err.Error()
	}
	return ReconcileScratchRemoved, reason
}

// removeScratchRootWithWorktrees removes every open recorded worktree while
// the writer lock is held through a fresh description (or the root is
// gone), then the root. An unresolved Git registration keeps both root and
// record.
func removeScratchRootWithWorktrees(record ScratchRecord, options ScratchOptions) (string, error) {
	return removeScratchRoot(record, func() (string, error) { return removeRecordedWorktrees(record, options) })
}

func removeRecordedWorktrees(record ScratchRecord, options ScratchOptions) (string, error) {
	if options.RemoveWorktree == nil {
		options.RemoveWorktree = gittree.RemoveRecordedWorktree
	}
	for _, tuple := range record.Worktrees {
		if tuple.State == ScratchWorktreeClosed {
			continue
		}
		action, err := options.RemoveWorktree(tuple.tuple(), gittree.Workspace{Dir: tuple.Control})
		switch {
		case err == nil && action == gittree.WorktreeRemoved:
		case action == gittree.WorktreeRefused:
			return fmt.Sprintf("worktree %s refused", tuple.Top), fmt.Errorf("%v", err)
		default:
			return fmt.Sprintf("worktree %s unresolved", tuple.Top), errors.Join(errScratchPending, err)
		}
	}
	return "", nil
}

// scratchWritersDead is the recovery predicate. Unlike allRecordedEnded it
// reads StatusDone records too and the launcher and custodian: any Unknown,
// missing or unreadable fact is pending.
func scratchWritersDead(control string, record ScratchRecord, options ScratchOptions) (string, bool) {
	dead := func(label, encoded string) (string, bool) {
		ref, err := identity.ParseRef(encoded)
		if err != nil {
			return fmt.Sprintf("%s identity unreadable: %v", label, err), false
		}
		if state := identity.AliveRef(options.Prober, ref); state != identity.Dead {
			return fmt.Sprintf("%s pid %d is %s", label, ref.Pid, state), false
		}
		return "", true
	}
	emptyGroup := func(label string, pgid int64) (string, bool) {
		if pgid <= 0 {
			return fmt.Sprintf("%s group is unrecorded", label), false
		}
		members, err := options.GroupMembers(pgid)
		if err != nil {
			return fmt.Sprintf("%s group %d is uninspectable: %v", label, pgid, err), false
		}
		if len(members) != 0 {
			return fmt.Sprintf("%s group %d still has live members %v", label, pgid, members), false
		}
		return "", true
	}
	if reason, ok := dead("launcher", record.Launcher); !ok {
		return reason, false
	}
	if record.Attempt != "" {
		attempt, err := ReadAttempt(control, record.Attempt)
		if err != nil {
			return "attempt unreadable: " + err.Error(), false
		}
		if len(attempt.ProcessKeys) == 0 {
			return "attempt " + record.Attempt + " has no process records", false
		}
		for _, key := range attempt.ProcessKeys {
			process, err := ReadProcessRecord(control, key)
			if err != nil {
				return fmt.Sprintf("process record %s unreadable: %v", key, err), false
			}
			for label, ref := range map[string]identity.Ref{"suite": process.SuiteProcess.Ref(),
				"watchdog": process.Watchdog.Ref(), "suite launcher": process.Launcher.Ref()} {
				if ref.Pid <= 0 {
					continue
				}
				if state := identity.AliveRef(options.Prober, ref); state != identity.Dead {
					return fmt.Sprintf("%s pid %d of %s is %s", label, ref.Pid, key, state), false
				}
			}
			if reason, ok := emptyGroup("suite", process.SuiteProcess.Pgid); !ok {
				return reason, false
			}
		}
	}
	for _, custodian := range record.Custodians {
		if reason, ok := dead("custodian", custodian.Ref); !ok {
			return reason, false
		}
		if reason, ok := emptyGroup("custodian", custodian.Group); !ok {
			return reason, false
		}
	}
	return "", true
}

type scratchContextKey struct{}

// WithScratchRun binds the run's scratch root to every resource command
// started under ctx: its custodian inherits the writer lock and is recorded
// before the workload may write.
func WithScratchRun(ctx context.Context, run *ScratchRun) context.Context {
	return context.WithValue(ctx, scratchContextKey{}, run)
}

// ScratchRunFromContext is the run bound by WithScratchRun, or nil.
func ScratchRunFromContext(ctx context.Context) *ScratchRun {
	run, _ := ctx.Value(scratchContextKey{}).(*ScratchRun)
	return run
}
