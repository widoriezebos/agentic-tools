package run

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"golang.org/x/sys/unix"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

// The waiter layer: the blocking watch verb and its owner-keyed records.
// A waiter is the wake path — a live process any runtime's background
// facility can hold — and its record is the fact the turn verdict reads.
// Records are exclusive PER (kind, id, owner) by liveness: a foreign
// waiter neither satisfies your unwatched rule nor blocks your own
// watching; a live same-owner waiter from a DEAD lifecycle is replaceable
// (FIX-R6-02), because its own watch is about to exit on the terminal
// record it is actually reading.

// Watch exit codes: outcomes 0-4, operational failures 64-66, disjoint.
const (
	ExitGreen         = 0
	ExitRed           = 1
	ExitEndedUnknown  = 2
	ExitLaunchFailed  = 3
	ExitNoRecord      = 4
	ExitWaiterBusy    = 64
	ExitWaiterIO      = 65
	ExitWaiterUnknown = 66
)

// WaiterTarget pins the waiter to one lifecycle.
type WaiterTarget struct {
	StartedAt   string `json:"startedAt,omitempty"`   // jobs
	Generation  int    `json:"generation,omitempty"`  // runs
	LaunchNonce string `json:"launchNonce,omitempty"` // runs
}

// Waiter is one registered waiter record.
type Waiter struct {
	Kind          string       `json:"kind"`
	Pid           int64        `json:"pid"`
	PidStartedAt  int64        `json:"pidStartedAt"`
	PidStartTicks int64        `json:"pidStartTicks,omitempty"`
	BootID        string       `json:"bootId,omitempty"`
	Session       string       `json:"session"`
	MainId        string       `json:"mainId"`
	Target        WaiterTarget `json:"target"`
}

// WaitersDir is the one namespace for job and run waiters alike.
func WaitersDir(root string) string { return filepath.Join(root, "artifacts", "agents", "waiters") }

// WaiterPath keys a record by kind, public id, and owner digest.
func WaiterPath(root, kind, id, ownerDigest string) string {
	return filepath.Join(WaitersDir(root), fmt.Sprintf("%s-%s-%s.json", kind, id, ownerDigest))
}

// withWaiterLock bounds every waiter-record mutation.
func withWaiterLock(root string, fn func() error) error {
	dir := WaitersDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("waiter lock is busy")
		}
		time.Sleep(20 * time.Millisecond)
	}
	defer unix.Flock(int(f.Fd()), unix.LOCK_UN)
	return fn()
}

// waiterError carries the pinned operational exit code.
type waiterError struct {
	code    int
	message string
}

func (e *waiterError) Error() string { return e.message }

// WaiterExitCode maps an error from this layer to its pinned code.
func WaiterExitCode(err error) int {
	if we, ok := err.(*waiterError); ok {
		return we.code
	}
	return ExitWaiterIO
}

// RegisterWaiter writes the caller's waiter record. Exclusive per key by
// liveness; a live same-key waiter on the SAME lifecycle refuses (64); a
// live waiter on a dead lifecycle, a dead owner, or a mismatched target
// is replaced by identity-checked compare-and-delete; an owner whose
// liveness is unknowable refuses (66) — unknown never authorizes.
func (s *Store) RegisterWaiter(kind, id string, owner Caller, target WaiterTarget) error {
	digest := OwnerDigest(owner.MainId)
	path := WaiterPath(s.Root, kind, id, digest)
	return withWaiterLock(s.Root, func() error {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			var existing Waiter
			if json.Unmarshal(data, &existing) != nil {
				// A malformed record cannot be identity-checked; refuse
				// rather than clobber (66).
				return &waiterError{ExitWaiterUnknown, "existing waiter record is unreadable; refusing to replace what cannot be identity-checked"}
			}
			switch identity.AliveRef(s.prober(), identity.Ref{Pid: existing.Pid, StartedAtSec: existing.PidStartedAt,
				StartTicks: existing.PidStartTicks, BootID: existing.BootID}) {
			case identity.Alive:
				if existing.Target == target {
					return &waiterError{ExitWaiterBusy, "a live waiter already watches this lifecycle for this owner"}
				}
				// Live but watching a DEAD lifecycle: replaceable
				// (FIX-R6-02); fall through to the write.
			case identity.Unknown:
				return &waiterError{ExitWaiterUnknown, "existing waiter liveness unknown; refusing"}
			}
		case !os.IsNotExist(err):
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		self := int64(os.Getpid())
		exact, state, _ := s.prober().Probe(self)
		if state != identity.Alive {
			return &waiterError{ExitWaiterUnknown, "own identity unreadable"}
		}
		record := Waiter{
			Kind: kind, Pid: self, PidStartedAt: exact.StartedAt.Unix(),
			PidStartTicks: exact.StartTicks, BootID: exact.BootID,
			Session: owner.SessionId, MainId: owner.MainId, Target: target,
		}
		encoded, err := json.MarshalIndent(record, "", " ")
		if err != nil {
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		if err := atomicWrite(path, append(encoded, '\n')); err != nil {
			return &waiterError{ExitWaiterIO, err.Error()}
		}
		return nil
	})
}

// RemoveWaiter deletes the caller's own record — compare-and-delete on
// identity, so one waiter can never delete another's registration.
func (s *Store) RemoveWaiter(kind, id string, owner Caller) {
	path := WaiterPath(s.Root, kind, id, OwnerDigest(owner.MainId))
	_ = withWaiterLock(s.Root, func() error {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var existing Waiter
		if json.Unmarshal(data, &existing) != nil {
			return nil
		}
		self := int64(os.Getpid())
		exact, state, _ := s.prober().Probe(self)
		same := false
		if existing.Pid == self && state == identity.Alive {
			// The pair decides when both sides carry it (issue #1):
			// cleanup must not orphan its own record after clock drift.
			if existing.PidStartTicks > 0 && existing.BootID != "" &&
				exact.StartTicks > 0 && exact.BootID != "" {
				same = existing.PidStartTicks == exact.StartTicks && existing.BootID == exact.BootID
			} else {
				same = existing.PidStartedAt == exact.StartedAt.Unix()
			}
		}
		if same {
			os.Remove(path)
		}
		return nil
	})
}

// LiveWaiter reports whether a live identity-verified waiter of the given
// owner watches the given CURRENT lifecycle — the owner-correlated fact
// the unwatched rule consumes.
func LiveWaiter(root string, prober identity.Prober, kind, id, mainId string, target WaiterTarget) bool {
	data, err := os.ReadFile(WaiterPath(root, kind, id, OwnerDigest(mainId)))
	if err != nil {
		return false
	}
	var waiter Waiter
	if json.Unmarshal(data, &waiter) != nil {
		return false
	}
	if waiter.Target != target {
		return false
	}
	return identity.AliveRef(prober, identity.Ref{Pid: waiter.Pid, StartedAtSec: waiter.PidStartedAt,
		StartTicks: waiter.PidStartTicks, BootID: waiter.BootID}) == identity.Alive
}

// Watch blocks until the run record is terminal and returns the pinned
// exit code. It registers its waiter record on entry and removes it on
// every exit path. Watch is OPEN — it polls the record; conclusions
// belong to the watcher and the record-writer path.
func (s *Store) Watch(id string, owner Caller, poll time.Duration) int {
	record, err := s.Read(id)
	if err != nil || record == nil {
		return ExitNoRecord
	}
	target := WaiterTarget{Generation: record.Generation, LaunchNonce: record.LaunchNonce}
	if err := s.RegisterWaiter("run", id, owner, target); err != nil {
		return WaiterExitCode(err)
	}
	defer s.RemoveWaiter("run", id, owner)
	if poll <= 0 {
		poll = 2 * time.Second
	}
	for {
		record, err := s.Read(id)
		if err != nil || record == nil {
			return ExitNoRecord
		}
		if record.Generation != target.Generation || record.LaunchNonce != target.LaunchNonce {
			// The public id now names a DIFFERENT lifecycle (reuse or
			// adoption): this waiter's target is gone.
			return ExitNoRecord
		}
		switch record.Status {
		case StatusGreen:
			return ExitGreen
		case StatusRed:
			return ExitRed
		case StatusEndedUnknown:
			return ExitEndedUnknown
		case StatusLaunchFailed:
			return ExitLaunchFailed
		}
		time.Sleep(poll)
	}
}

// List returns every parsed record plus the unreadable paths — readers
// surface failures, never skip them.
func (s *Store) List() (records []Record, unreadable []string) {
	for _, path := range RecordFiles(s.Root) {
		data, err := os.ReadFile(path)
		if err != nil {
			unreadable = append(unreadable, path+": "+err.Error())
			continue
		}
		var record Record
		if err := json.Unmarshal(data, &record); err != nil {
			unreadable = append(unreadable, path+": unparsable run record")
			continue
		}
		if problems := Validate(&record); len(problems) > 0 {
			unreadable = append(unreadable, path+": "+problems[0])
			continue
		}
		records = append(records, record)
	}
	sort.Slice(records, func(i, j int) bool { return records[i].StartedAt < records[j].StartedAt })
	return records, unreadable
}
