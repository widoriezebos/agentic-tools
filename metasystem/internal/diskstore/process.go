package diskstore

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"
)

// Process scratch (3.2 "Process", R1, R2): every disposable temp need of an
// engine process lands in one lazily created, registered root,
// $TMPDIR/metasystem/<ulid>, owned by the process. Its writer lock is one
// open file description the process holds and every child started through
// PrepareChild inherits, so the lock lives while any writer lives. The
// owner's release runs on the way to the exit code: it closes only its own
// copy of the lock (never LOCK_UN, which would free every inheritor too),
// takes the lock again through a fresh description, and removes the root
// only if it gets it; otherwise the record stays for the sweeper's process
// proof.
const (
	// ProcessScratchClass is the class of a process's scratch root.
	ProcessScratchClass = "process-scratch"
	// WriterLockName is the writer lock file inside a process scratch root.
	WriterLockName = ".writer-lock"
	// processScratchParent is the one entry process scratch makes in TMPDIR.
	processScratchParent = "metasystem"
)

// processScratch is one process's root. users counts the directories and
// files handed out and not yet done; the owner's release leaves a root with
// users for the sweeper, so a verb ending on one goroutine never removes
// what another is still using.
type processScratch struct {
	registry Registry
	record   Record
	writer   *os.File
	users    int
	released bool
}

var (
	scratchMu      sync.Mutex
	currentScratch *processScratch
)

// ensureScratch returns this process's root, creating it on first use.
// scratchMu is held.
func ensureScratch() (*processScratch, error) {
	if currentScratch != nil {
		return currentScratch, nil
	}
	registry, err := machineRegistry()
	if err != nil {
		return nil, fmt.Errorf("process scratch: %w", err)
	}
	created, err := newProcessScratch(os.TempDir(), registry, rand.Reader)
	if err != nil {
		return nil, err
	}
	currentScratch = created
	return created, nil
}

// ProcessScratch is this process's scratch root, created and registered on
// first use.
func ProcessScratch() (string, error) {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	scratch, err := ensureScratch()
	if err != nil {
		return "", err
	}
	return scratch.record.Path, nil
}

// ScratchDir makes a new directory in the process's scratch root, as
// os.MkdirTemp does in TMPDIR. done removes it and ends its use; it is safe
// to call more than once.
func ScratchDir(pattern string) (string, func(), error) {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	scratch, err := ensureScratch()
	if err != nil {
		return "", nil, err
	}
	return scratch.mkdirLocked(pattern)
}

func (s *processScratch) mkdir(pattern string) (string, func(), error) {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	return s.mkdirLocked(pattern)
}

// mkdirLocked is ScratchDir in s; scratchMu is held.
func (s *processScratch) mkdirLocked(pattern string) (string, func(), error) {
	if s.released {
		return "", nil, fmt.Errorf("process scratch %s is released", s.record.Path)
	}
	dir, err := os.MkdirTemp(s.record.Path, pattern)
	if err != nil {
		return "", nil, fmt.Errorf("process scratch: %w", err)
	}
	return dir, s.use(func() { _ = os.RemoveAll(dir) }), nil
}

// ScratchFile creates a new file in the process's scratch root, as
// os.CreateTemp does in TMPDIR. done removes it and ends its use; closing
// the file stays the caller's.
func ScratchFile(pattern string) (*os.File, func(), error) {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	scratch, err := ensureScratch()
	if err != nil {
		return nil, nil, err
	}
	file, err := os.CreateTemp(scratch.record.Path, pattern)
	if err != nil {
		return nil, nil, fmt.Errorf("process scratch: %w", err)
	}
	return file, scratch.use(func() { _ = os.Remove(file.Name()) }), nil
}

// use counts one user and returns its done. scratchMu is held.
func (s *processScratch) use(remove func()) func() {
	s.users++
	var once sync.Once
	return func() {
		once.Do(func() {
			remove()
			scratchMu.Lock()
			s.users--
			scratchMu.Unlock()
		})
	}
}

// PrepareChild is the launcher seam: the child inherits the writer lock
// through ExtraFiles, and its TMPDIR, TMP, TEMP and GOTMPDIR are the root,
// so an engine child makes its own root nested under this one and nothing
// a child writes lands outside a registered store.
func PrepareChild(cmd *exec.Cmd) error {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	scratch, err := ensureScratch()
	if err != nil {
		return err
	}
	cmd.ExtraFiles = append(cmd.ExtraFiles, scratch.writer)
	env := cmd.Env
	if env == nil {
		env = os.Environ()
	}
	kept := make([]string, 0, len(env)+4)
	for _, entry := range env {
		switch name, _, _ := strings.Cut(entry, "="); name {
		case "TMPDIR", "TMP", "TEMP", "GOTMPDIR":
			continue
		}
		kept = append(kept, entry)
	}
	root := scratch.record.Path
	cmd.Env = append(kept, "TMPDIR="+root, "TMP="+root, "TEMP="+root, "GOTMPDIR="+root)
	return nil
}

// ReleaseProcessScratch is the owner's release at the end of dispatch. A
// root still in use in this process, or held by a child, stays for the
// sweeper; any error leaves the record for it too.
func ReleaseProcessScratch(ctx context.Context) error {
	scratchMu.Lock()
	scratch := currentScratch
	scratchMu.Unlock()
	if scratch == nil {
		return nil
	}
	_, err := scratch.releaseIfIdle(ctx)
	return err
}

func (s *processScratch) releaseIfIdle(ctx context.Context) (bool, error) {
	scratchMu.Lock()
	defer scratchMu.Unlock()
	if s.released {
		return true, nil
	}
	if s.users > 0 {
		return false, nil
	}
	s.released = true
	if currentScratch == s {
		currentScratch = nil
	}
	// Close only this process's copy: a child that inherited the
	// description keeps the lock, and with it the root, alive.
	_ = s.writer.Close()
	critical, err := s.registry.TryCritical(s.record.ID)
	if err != nil {
		return false, fmt.Errorf("process scratch %s is kept for the sweeper: %w", s.record.Path, err)
	}
	defer critical.Release()
	if verdict := releaseScratchRoot(ctx, critical, "owner"); verdict.Decision != Release {
		return false, fmt.Errorf("process scratch %s is kept for the sweeper: %s", s.record.Path, verdict.Reason)
	}
	// A normal end leaves nothing. Fail-closed rule 3 keeps the records
	// others count; nothing counts a process-scratch record, so its owner
	// removes its own record and record lock, under that lock.
	return true, errors.Join(os.Remove(s.registry.RecordPath(s.record.ID)), os.Remove(s.registry.LockPath(s.record.ID)))
}

// newProcessScratch registers and creates a root under tempRoot/metasystem.
// The root is made empty under a staging name first: the kernel stamps the
// instant its ULID and record carry (this package reads no clock), and its
// device and inode are in the record before it receives a byte.
func newProcessScratch(tempRoot string, registry Registry, entropy io.Reader) (*processScratch, error) {
	self, state, err := (identity.KernelProber{}).Probe(int64(os.Getpid()))
	if err != nil || state != identity.Alive {
		return nil, fmt.Errorf("process scratch: this process's identity is unreadable: %v", err)
	}
	ref, err := identity.EncodeRef(self.Ref())
	if err != nil {
		return nil, fmt.Errorf("process scratch: %w", err)
	}
	parent := filepath.Join(tempRoot, processScratchParent)
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return nil, fmt.Errorf("process scratch: %w", err)
	}
	if parent, err = filepath.EvalSymlinks(parent); err != nil {
		return nil, fmt.Errorf("process scratch: %w", err)
	}
	staging, err := os.MkdirTemp(parent, ".new-")
	if err != nil {
		return nil, fmt.Errorf("process scratch: %w", err)
	}
	info, err := os.Lstat(staging)
	if err != nil {
		return nil, errors.Join(fmt.Errorf("process scratch: %w", err), os.Remove(staging))
	}
	now := info.ModTime()
	name, err := NewID(now, entropy)
	if err != nil {
		return nil, errors.Join(err, os.Remove(staging))
	}
	root := filepath.Join(parent, name)
	if err := os.Rename(staging, root); err != nil {
		return nil, errors.Join(fmt.Errorf("process scratch: %w", err), os.Remove(staging))
	}
	device, inode, _ := fileID(info)
	record, err := registry.Register(Registration{Path: root, Class: ProcessScratchClass, Owner: Owner{Kind: OwnerProcess, Ref: ref},
		Lifetime: LifetimeOwner, CapKind: CapTarget, CapBytes: Settings{}.Bytes(config.DiskProcessScratchKey),
		RootDevice: device, RootInode: inode, RootGeneration: pathGeneration(root)}, now, entropy)
	if err != nil {
		// Unregistered and still empty: os.Remove removes only an empty
		// directory.
		return nil, errors.Join(fmt.Errorf("process scratch: %w", err), os.Remove(root))
	}
	// From here a failure leaves the record for the sweeper's process
	// proof; the writer lock is closed so that proof can take it once this
	// process has ended.
	writer, err := os.OpenFile(filepath.Join(root, WriterLockName), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("process scratch writer lock: %w", err)
	}
	if err := unix.Flock(int(writer.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		return nil, errors.Join(fmt.Errorf("process scratch writer lock: %w", err), writer.Close())
	}
	if err := WriteMarker(record); err != nil {
		return nil, errors.Join(err, writer.Close())
	}
	if record, err = registry.Accept(record.ID); err != nil {
		return nil, errors.Join(err, writer.Close())
	}
	return &processScratch{registry: registry, record: record, writer: writer}, nil
}

// ProbeWriterLock reports whether a process scratch root's writer lock is
// free, through a fresh description, holding nothing afterwards. A root
// that is gone holds nothing; a lock file that is missing from an accepted
// root is an error (fail-closed rule 1), and from a releasing one is free:
// the releaser that unlinked it held it.
func ProbeWriterLock(record Record) (bool, error) {
	file, err := openWriterLock(record)
	if err != nil || file == nil {
		return err == nil, err
	}
	defer file.Close()
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	return true, unix.Flock(int(file.Fd()), unix.LOCK_UN)
}

// openWriterLock opens the writer lock without following a link; nil, nil
// is a lock that provably holds nothing (root gone, or unlinked by a
// releaser).
func openWriterLock(record Record) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(record.Path, WriterLockName), os.O_RDONLY|unix.O_NOFOLLOW, 0)
	if !errors.Is(err, os.ErrNotExist) {
		return file, err
	}
	if _, statErr := os.Lstat(record.Path); errors.Is(statErr, os.ErrNotExist) || statErr == nil && record.State == StateReleasing {
		return nil, nil
	}
	return nil, fmt.Errorf("process scratch %s has no writer lock", record.Path)
}

// ProcessProof is the process owner kind's proof (3.1): the owner's
// reference reads Dead and the writer lock can be taken through a fresh
// description. It releases through its own sequence.
type ProcessProof struct {
	Prober identity.Prober
}

func (ProcessProof) Kind() OwnerKind { return OwnerProcess }

// Observe judges one process store; every unreadable input holds it
// (fail-closed rule 1).
func (p ProcessProof) Observe(_ context.Context, record Record) Verdict {
	pending := func(reason string) Verdict {
		return Verdict{Decision: Pending, Reason: reason, Command: "metasystem disk show"}
	}
	if record.Class != ProcessScratchClass || !record.Identity.Marker {
		return pending(fmt.Sprintf("a process store of class %s has no proof in this engine", record.Class))
	}
	ref, err := identity.ParseRef(record.Owner.Ref)
	if err != nil {
		return pending("its owner reference is unreadable: " + err.Error())
	}
	switch identity.AliveRef(p.Prober, ref) {
	case identity.Alive:
		return Verdict{Decision: Keep, Reason: "its process runs", Command: "metasystem disk clean, once that process has ended"}
	case identity.Unknown:
		return pending("its process's liveness cannot be read")
	}
	free, err := ProbeWriterLock(record)
	switch {
	case err != nil:
		return pending("its writer lock is unreadable: " + err.Error())
	case !free:
		return Verdict{Decision: Keep, Reason: "a process it started still holds its writer lock", Command: "metasystem disk clean, once that process has ended"}
	}
	return Verdict{Decision: Release, Reason: "its process ended and nothing holds its writer lock"}
}

// Apply is never reached: the store releases through Release.
func (ProcessProof) Apply(context.Context, *Critical) error {
	return errors.New("a process scratch root is released only through its own sequence")
}

// Release is the sweeper's removal inside the store's critical section.
func (ProcessProof) Release(ctx context.Context, critical *Critical, _ *UseCensus) Verdict {
	return releaseScratchRoot(ctx, critical, "sweeper")
}

// releaseScratchRoot removes a process scratch root inside its critical
// section, for the owner and the sweeper alike. The fail-closed rules for
// every removal bind it:
//  1. any read error or unreadable input holds the root (pending);
//  2. the root is the recorded one by device and inode, never by its path
//     string: a replaced directory or a symlink is held;
//  3. only the root's payload goes; the record stays as history (the
//     owner's normal end alone removes its own, uncounted record);
//  4. every attempt, a retry of a releasing root included, re-takes the
//     writer lock through a fresh description before anything is removed,
//     and holds it through the last unlink;
//  5. the root is the process's, never a person's.
func releaseScratchRoot(ctx context.Context, critical *Critical, by string) Verdict {
	record := critical.Record()
	pending := func(reason string) Verdict {
		return Verdict{Decision: Pending, Reason: reason, Command: "metasystem disk show"}
	}
	if record.Class != ProcessScratchClass || !record.Identity.Marker {
		return pending("not a process scratch root")
	}
	info, err := os.Lstat(record.Path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		return markReleased(critical, by, "its root is already gone")
	case err != nil:
		return pending("its root is unreadable: " + err.Error())
	}
	if err := sameRoot(record, info); err != nil {
		return pending(err.Error())
	}
	if record.State == StateReleasing {
		err = revalidateReleasing(record)
	} else {
		err = Revalidate(record)
	}
	if err != nil {
		return pending("the root is not the recorded one: " + err.Error())
	}
	writer, err := openWriterLock(record)
	if err != nil {
		return pending("its writer lock is unreadable: " + err.Error())
	}
	if writer != nil {
		defer writer.Close()
		if err := unix.Flock(int(writer.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
				return Verdict{Decision: Keep, Reason: "a process it started still holds its writer lock", Command: "metasystem disk clean, once that process has ended"}
			}
			return pending("its writer lock is unreadable: " + err.Error())
		}
	}
	if record.State != StateReleasing {
		record.State = StateReleasing
		if err := critical.Write(record); err != nil {
			return pending("could not record releasing: " + err.Error())
		}
	}
	if err := RemoveStore(ctx, record); err != nil {
		return Verdict{Decision: Pending, Reason: "removal cut short (" + err.Error() + "); the root is releasing and the next pass finishes it", Command: "metasystem disk clean"}
	}
	return markReleased(critical, by, "released")
}

func markReleased(critical *Critical, by, reason string) Verdict {
	record := critical.Record()
	record.State, record.ReleasedBy = StateReleased, by
	if err := critical.Write(record); err != nil {
		return Verdict{Decision: Pending, Reason: "removed, but released could not be recorded: " + err.Error(), Command: "metasystem disk clean"}
	}
	return Verdict{Decision: Release, Reason: reason}
}

// sameRoot checks a marker store's root against the device, inode and
// generation its record carries, when it carries them.
func sameRoot(record Record, info os.FileInfo) error {
	if record.Identity.RootInode == 0 {
		return nil
	}
	device, inode, ok := fileID(info)
	if !ok || info.Mode()&os.ModeSymlink != 0 || device != record.Identity.RootDevice || inode != record.Identity.RootInode ||
		record.Identity.RootGeneration != 0 && pathGeneration(record.Path) != record.Identity.RootGeneration {
		return fmt.Errorf("store %s is not the recorded directory (device and inode differ)", record.Path)
	}
	return nil
}
