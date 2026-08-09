// Package lock implements the acquisition discipline REG-4 defines for
// the registry lock and D-1 adopts verbatim for the checkout lock: a
// directory lock that is BORN OWNING, acquired by one atomic rename of
// a pre-populated private directory, so an ownerless lock can never
// result from acquisition (SLC-R8-001, SLC-R9-002).
//
// Takeover is death-only and three-way (SLC-R11-002): a waiter that
// exhausts its bounded wait may take the lock over ONLY after a
// SUCCESSFUL read proves the recorded holder dead by exact identity.
// Uninspectable is alive: an unreadable owner file never authorizes
// takeover. A lock directory with no owner file is garbage by
// construction — no live acquirer can be mid-publication — and is
// takeable after the same bounded window.
package lock

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// Identity names a lock holder by kernel facts, never by claims.
type Identity struct {
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	Tag          string `json:"instanceTag,omitempty"`
}

// Liveness is the three-way answer the takeover rule requires: only a
// definitive negative kills (SLC-R1-003, SLC-R2-006, SLC-R11-002).
type Liveness int

const (
	// Alive: the exact identity (pid + start time) was proven live.
	Alive Liveness = iota
	// Dead: a SUCCESSFUL read proved the identity absent.
	Dead
	// Unknown: the read itself failed. Unknown never authorizes anything.
	Unknown
)

// Probe answers liveness for a recorded identity. Consumers supply it
// (accept interfaces, return structs): the real one reads the kernel,
// tests inject schedules.
type Probe func(Identity) Liveness

// Options bound an acquisition attempt.
type Options struct {
	// Wait is the total bounded wait before giving up (or taking over
	// a provably dead or ownerless lock).
	Wait time.Duration
	// Poll is the re-inspection interval while waiting.
	Poll time.Duration
	// Probe proves holder liveness. Required.
	Probe Probe
}

// Lock is a held lock. Release it exactly once.
type Lock struct {
	path string
	self Identity
}

// HolderError reports the live (or unproven) holder that kept the lock.
type HolderError struct {
	Path   string
	Holder Identity
	State  Liveness
}

func (e *HolderError) Error() string {
	state := "alive"
	if e.State == Unknown {
		state = "of unproven liveness (uninspectable is alive)"
	}
	return fmt.Sprintf("lock %s is held by pid %d (started %d) %s", e.Path, e.Holder.Pid, e.Holder.PidStartedAt, state)
}

const ownerFile = "owner.json"

// Acquire takes the lock at path for self, waiting at most opts.Wait.
//
// The private directory is populated with owner.json BEFORE the rename,
// so every observable lock names its holder from birth. On contention
// the recorded holder is probed each poll: Dead (definitively) permits
// takeover; Unknown and Alive both keep waiting — and if the wait
// exhausts, the error names the holder and its state.
func Acquire(path string, self Identity, opts Options) (*Lock, error) {
	if opts.Probe == nil {
		return nil, errors.New("lock: acquisition requires a liveness probe")
	}
	if opts.Poll <= 0 {
		opts.Poll = 25 * time.Millisecond
	}
	private, err := populatePrivate(path, self)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(private)

	deadline := time.Now().Add(opts.Wait)
	var sawOwnerlessAt time.Time
	for {
		if err := os.Rename(private, path); err == nil {
			return &Lock{path: path, self: self}, nil
		} else if !isContention(err) {
			return nil, fmt.Errorf("lock: rename acquisition: %w", err)
		}

		holder, readErr := readOwner(path)
		switch {
		case readErr == nil:
			sawOwnerlessAt = time.Time{}
			switch opts.Probe(holder) {
			case Dead:
				// Death-only takeover. Removal is fenced by a kernel
				// flock and the death re-verified INSIDE it: without
				// the fence, two provers can both remove — the second
				// removing the FIRST winner's freshly renamed lock
				// (the two-winners race the unit test replays). The
				// kernel releases the fence if a remover dies holding
				// it.
				if err := removeIfHolder(path, holder, opts.Probe); err != nil {
					return nil, fmt.Errorf("lock: takeover removal: %w", err)
				}
				continue
			case Alive, Unknown:
				if time.Now().After(deadline) {
					state := Alive
					if opts.Probe(holder) == Unknown {
						state = Unknown
					}
					return nil, &HolderError{Path: path, Holder: holder, State: state}
				}
			}
		case os.IsNotExist(readErr) && lockDirExists(path):
			// Ownerless directory: garbage by construction, takeable
			// after the bounded window (REG-4). The window starts at
			// first observation, not at Acquire entry.
			if sawOwnerlessAt.IsZero() {
				sawOwnerlessAt = time.Now()
			}
			if time.Since(sawOwnerlessAt) >= opts.Wait {
				if err := removeIfOwnerless(path); err != nil {
					return nil, fmt.Errorf("lock: garbage removal: %w", err)
				}
				continue
			}
		case os.IsNotExist(readErr):
			// The lock vanished between rename failure and inspection:
			// retry immediately.
			sawOwnerlessAt = time.Time{}
			continue
		default:
			// Unreadable owner file: uninspectable is alive.
			sawOwnerlessAt = time.Time{}
			if time.Now().After(deadline) {
				return nil, &HolderError{Path: path, Holder: Identity{}, State: Unknown}
			}
		}
		time.Sleep(opts.Poll)
	}
}

// Holder returns the identity a live lock currently names, for
// callers that must re-verify ownership (D-1's fenced publication).
func Holder(path string) (Identity, error) { return readOwner(path) }

// Release frees the lock by renaming it aside and removing it, so no
// observer ever sees the lock path ownerless. Releasing a lock whose
// owner file no longer names self fails: something took it over, and
// removing a successor's lock would be exactly the trap-kill shape
// SLC-F-001 forbids.
func (l *Lock) Release() error {
	holder, err := readOwner(l.path)
	if err != nil {
		return fmt.Errorf("lock: release verification: %w", err)
	}
	if holder != l.self {
		return fmt.Errorf("lock %s no longer names this holder (found pid %d started %d)", l.path, holder.Pid, holder.PidStartedAt)
	}
	return removeLock(l.path)
}

func populatePrivate(path string, self Identity) (string, error) {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return "", fmt.Errorf("lock: random suffix: %w", err)
	}
	private := fmt.Sprintf("%s.acquire-%d-%s", path, self.Pid, hex.EncodeToString(suffix))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("lock: parent directory: %w", err)
	}
	if err := os.Mkdir(private, 0o755); err != nil {
		return "", fmt.Errorf("lock: private directory: %w", err)
	}
	owner, err := json.Marshal(self)
	if err != nil {
		os.RemoveAll(private)
		return "", fmt.Errorf("lock: owner encoding: %w", err)
	}
	if err := os.WriteFile(filepath.Join(private, ownerFile), append(owner, '\n'), 0o644); err != nil {
		os.RemoveAll(private)
		return "", fmt.Errorf("lock: owner publication: %w", err)
	}
	return private, nil
}

func readOwner(path string) (Identity, error) {
	content, err := os.ReadFile(filepath.Join(path, ownerFile))
	if err != nil {
		return Identity{}, err
	}
	var holder Identity
	if err := json.Unmarshal(content, &holder); err != nil {
		return Identity{}, fmt.Errorf("owner file unparseable: %w", err)
	}
	return holder, nil
}

func lockDirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// withMutationFence runs action while holding an exclusive kernel
// flock beside the lock. Destructive mutations re-verify their
// condition inside the fence; the kernel releases it if the mutator
// dies, so a crash never wedges the lock's neighbors.
func withMutationFence(path string, action func() error) error {
	fence, err := os.OpenFile(path+".mutations.flock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("mutation fence: %w", err)
	}
	defer fence.Close()
	if err := syscall.Flock(int(fence.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("mutation fence flock: %w", err)
	}
	defer syscall.Flock(int(fence.Fd()), syscall.LOCK_UN)
	return action()
}

// removeIfHolder removes the lock only if, inside the mutation fence,
// it still names the identity proven dead. A lock that vanished or
// changed hands is left alone — the caller loops and reassesses.
func removeIfHolder(path string, deadHolder Identity, probe Probe) error {
	return withMutationFence(path, func() error {
		current, err := readOwner(path)
		if os.IsNotExist(err) {
			return nil // already removed or replaced-in-flight
		}
		if err != nil {
			return nil // uninspectable is alive; reassess outside
		}
		if current != deadHolder || probe(current) != Dead {
			return nil // someone else's lock now; never touch it
		}
		return removeLock(path)
	})
}

// removeIfOwnerless removes an ownerless directory only if it is
// still ownerless inside the fence.
func removeIfOwnerless(path string) error {
	return withMutationFence(path, func() error {
		if _, err := readOwner(path); err == nil || !os.IsNotExist(err) {
			return nil
		}
		if !lockDirExists(path) {
			return nil
		}
		return removeLock(path)
	})
}

// removeLock renames the lock aside before deleting, so the lock path
// never exists ownerless mid-removal.
func removeLock(path string) error {
	suffix := make([]byte, 4)
	if _, err := rand.Read(suffix); err != nil {
		return err
	}
	trash := fmt.Sprintf("%s.release-%s", path, hex.EncodeToString(suffix))
	if err := os.Rename(path, trash); err != nil {
		return err
	}
	return os.RemoveAll(trash)
}

// isContention reports the rename failures that mean "the lock path is
// occupied": EEXIST and ENOTEMPTY (renaming a directory onto an
// existing, populated directory differs by platform).
func isContention(err error) bool {
	return errors.Is(err, os.ErrExist) || errors.Is(err, syscall.EEXIST) || errors.Is(err, syscall.ENOTEMPTY)
}
