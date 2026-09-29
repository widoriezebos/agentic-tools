package diskstore

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// ErrStoreGone is an entrant finding its store releasing, released or
// unregistered: the store is not there to enter.
var ErrStoreGone = errors.New("the store is being released or is gone")

// HeldError is a nonblocking acquisition that found the record lock held;
// the item is pending, never waited on.
type HeldError struct{ Path string }

func (e *HeldError) Error() string { return "store record lock is held: " + e.Path }

// recordLockAcquired runs each time a record lock is taken; tests stand a
// fork's duplicate descriptor in at that moment.
var recordLockAcquired = func(*os.File) {}

// unlockAndClose ends a record-lock hold: the flock is released on the open
// file description first, so a duplicate a concurrent fork made before its
// exec cannot keep the lock after the holder has ended.
func unlockAndClose(file *os.File) error {
	unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
	return errors.Join(unlockErr, file.Close())
}

// Entrant is an engine verb inside a store: it holds the record lock shared
// from before it resolves the path until it leaves (3.1, "Entrants").
type Entrant struct {
	file   *os.File
	Record Record
}

// Enter takes the record lock shared, blocking (a verb waits at most one
// store's critical section), then re-reads the record. A record that is
// releasing, released or absent answers ErrStoreGone and holds nothing.
func (r Registry) Enter(id string) (*Entrant, error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrStoreGone
	}
	if err != nil {
		return nil, fmt.Errorf("store record lock %s: %w", id, err)
	}
	if err := flockRetry(file, unix.LOCK_SH); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("store record lock %s: %w", id, err)
	}
	recordLockAcquired(file)
	record, err := r.Load(id)
	if errors.Is(err, ErrNotFound) || err == nil && (record.State == StateReleasing || record.State == StateReleased) {
		_ = unlockAndClose(file)
		return nil, ErrStoreGone
	}
	if err != nil {
		_ = unlockAndClose(file)
		return nil, err
	}
	return &Entrant{file: file, Record: record}, nil
}

// Leave drops the shared hold.
func (e *Entrant) Leave() error {
	if e == nil || e.file == nil {
		return nil
	}
	err := unlockAndClose(e.file)
	e.file = nil
	return err
}

// Critical is one store's critical section (3.1): the record lock held
// exclusively, taken without waiting, with the record reloaded from disk
// after acquisition. Nothing between TryCritical and Release gives the lock
// up, and no removal of the store happens outside one.
type Critical struct {
	registry Registry
	file     *os.File
	record   Record
}

// TryCritical takes the record lock LOCK_EX|LOCK_NB. A held lock is a
// *HeldError at once; an absent lock file is ErrNotFound (the record was
// never completed, or is gone). The record is reloaded after acquisition,
// so a change made between a caller's lookup and this call is what the
// caller sees.
func (r Registry) TryCritical(id string) (*Critical, error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("store record lock %s: %w", id, err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, &HeldError{Path: r.LockPath(id)}
		}
		return nil, fmt.Errorf("store record lock %s: %w", id, err)
	}
	recordLockAcquired(file)
	record, err := r.Load(id)
	if err != nil {
		_ = unlockAndClose(file)
		return nil, err
	}
	return &Critical{registry: r, file: file, record: record}, nil
}

// Record is the record as reloaded under the lock, updated by Write.
func (c *Critical) Record() Record { return c.record }

// Write publishes a changed record under the lock. The id and schema are
// fixed; a record cannot leave released.
func (c *Critical) Write(record Record) error {
	if c.file == nil {
		return errors.New("store critical section already released")
	}
	if record.ID != c.record.ID {
		return fmt.Errorf("store critical section for %s cannot write record %s", c.record.ID, record.ID)
	}
	if c.record.State == StateReleased && record.State != StateReleased {
		return fmt.Errorf("store %s is released; its record is history", record.ID)
	}
	record.Schema = Schema
	if err := c.registry.write(record); err != nil {
		return err
	}
	c.record = record
	return nil
}

// Release ends the critical section.
func (c *Critical) Release() error {
	if c == nil || c.file == nil {
		return nil
	}
	err := unlockAndClose(c.file)
	c.file = nil
	return err
}

// Transition moves a record the owner itself holds from one of from to to,
// waiting for the lock: an owner's own act (Accept, a release at its end)
// is bounded by one sweeper critical section. A record already in to is
// success and writes nothing (R-129).
func (r Registry) Transition(id string, from []State, to State, mutate func(*Record)) (Record, error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDWR, 0)
	if errors.Is(err, os.ErrNotExist) {
		return Record{}, ErrNotFound
	}
	if err != nil {
		return Record{}, err
	}
	if err := flockRetry(file, unix.LOCK_EX); err != nil {
		_ = file.Close()
		return Record{}, err
	}
	defer unlockAndClose(file)
	recordLockAcquired(file)
	record, err := r.Load(id)
	if err != nil {
		return Record{}, err
	}
	if record.State == to {
		return record, nil
	}
	if record.State == StateReleased {
		return record, fmt.Errorf("store %s is released; its record is history", id)
	}
	allowed := false
	for _, state := range from {
		allowed = allowed || record.State == state
	}
	if !allowed {
		return record, fmt.Errorf("store %s is %s; it cannot become %s", id, record.State, to)
	}
	record.State = to
	if mutate != nil {
		mutate(&record)
	}
	if err := r.write(record); err != nil {
		return Record{}, err
	}
	return record, nil
}

// Accept marks a completely created store accepted.
func (r Registry) Accept(id string) (Record, error) {
	return r.Transition(id, []State{StateReserved}, StateAccepted, nil)
}

// ProbeRecordLock reports whether the record lock is free, without creating
// the lock file and without holding anything afterwards: the plan phase's
// probe (R15). An absent lock file reads as free.
func (r Registry) ProbeRecordLock(id string) (free bool, err error) {
	file, err := os.OpenFile(r.LockPath(id), os.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return false, nil
		}
		return false, err
	}
	recordLockAcquired(file)
	if err := unlockAndClose(file); err != nil {
		return false, err
	}
	return true, nil
}

func flockRetry(file *os.File, operation int) error {
	for {
		err := unix.Flock(int(file.Fd()), operation)
		if !errors.Is(err, unix.EINTR) {
			return err
		}
	}
}
