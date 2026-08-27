package gaterun

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type GuardResult int

const (
	GuardAcquired GuardResult = iota
	GuardJoined
)

type executionGuardMember struct {
	Pid          int64  `json:"pid"`
	PidStartedAt int64  `json:"pidStartedAt"`
	Owner        string `json:"owner"`
}

type executionGuardRecord struct {
	Pid          int64                  `json:"pid"`
	PidStartedAt int64                  `json:"pidStartedAt"`
	Owner        string                 `json:"owner"`
	Generation   int64                  `json:"generation"`
	AcquiredAt   string                 `json:"acquiredAt"`
	Members      []executionGuardMember `json:"members"`
}

func executionGuardPath(root string) string {
	return filepath.Join(markerDir(root), "checkout-execution.lock.d")
}

type executionGuardCodec struct{ record executionGuardRecord }

func (codec executionGuardCodec) Encode(holder lock.Identity) ([]byte, error) {
	record := codec.record
	record.Pid = holder.Pid
	record.PidStartedAt = holder.PidStartedAt
	record.Owner = holder.Label
	generation, err := strconv.ParseInt(holder.Tag, 10, 64)
	if err != nil || generation < 1 {
		return nil, errors.New("checkout execution owner has an invalid generation")
	}
	record.Generation = generation
	if record.AcquiredAt == "" {
		record.AcquiredAt = clock().UTC().Format("2006-01-02T15:04:05Z")
	}
	data, err := json.Marshal(record)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func (executionGuardCodec) Decode(data []byte) (lock.Identity, error) {
	record, err := decodeExecutionGuardRecord(data)
	if err != nil {
		return lock.Identity{}, err
	}
	return record.identity(), nil
}

func decodeExecutionGuardRecord(data []byte) (executionGuardRecord, error) {
	var record executionGuardRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return record, fmt.Errorf("checkout execution owner is unparseable: %w", err)
	}
	if record.Pid <= 0 || record.PidStartedAt <= 0 || record.Owner == "" ||
		record.Generation < 1 || record.AcquiredAt == "" || len(record.Members) == 0 {
		return record, errors.New("checkout execution owner is incomplete")
	}
	for _, member := range record.Members {
		if member.Pid <= 0 || member.PidStartedAt <= 0 || member.Owner == "" {
			return record, errors.New("checkout execution owner has an incomplete member")
		}
	}
	return record, nil
}

func (record executionGuardRecord) identity() lock.Identity {
	return lock.Identity{Pid: record.Pid, PidStartedAt: record.PidStartedAt, Tag: strconv.FormatInt(record.Generation, 10), Label: record.Owner}
}

func readExecutionGuardRecord(root string) (executionGuardRecord, error) {
	data, err := os.ReadFile(filepath.Join(executionGuardPath(root), "owner.json"))
	if err != nil {
		return executionGuardRecord{}, err
	}
	return decodeExecutionGuardRecord(data)
}

func executionGuardProbe(root string) lock.Probe {
	return func(lock.Identity) lock.Liveness {
		record, err := readExecutionGuardRecord(root)
		if err != nil {
			return lock.Unknown
		}
		sawUnknown := false
		for _, member := range record.Members {
			switch livenessOf(member.Pid, member.PidStartedAt) {
			case identity.Alive:
				return lock.Alive
			case identity.Unknown:
				sawUnknown = true
			}
		}
		if sawUnknown {
			return lock.Unknown
		}
		return lock.Dead
	}
}

func executionGuardMemberFor(pid int64, owner string) (executionGuardMember, error) {
	if owner == "" {
		return executionGuardMember{}, errors.New("checkout execution guard requires a named member")
	}
	started, ok := processStart(pid)
	if !ok {
		return executionGuardMember{}, fmt.Errorf("checkout execution guard cannot verify process identity for pid %d", pid)
	}
	return executionGuardMember{Pid: pid, PidStartedAt: started, Owner: owner}, nil
}

func memberAlive(member executionGuardMember) identity.Liveness {
	return livenessOf(member.Pid, member.PidStartedAt)
}

// callerDescendsFromMember mirrors the wrapper-token proof: a registered
// exact live process must occur in the caller's native ancestry. Inherited
// strings and recycled process identifiers prove nothing.
func callerDescendsFromMember(caller int64, members []executionGuardMember) bool {
	byPid := make(map[int64]executionGuardMember, len(members))
	for _, member := range members {
		byPid[member.Pid] = member
	}
	seen := map[int64]bool{}
	for current := caller; current > 0 && !seen[current]; {
		seen[current] = true
		if member, ok := byPid[current]; ok && memberAlive(member) == identity.Alive {
			return true
		}
		parent, ok := identity.ParentPid(current)
		if !ok {
			break
		}
		current = parent
	}
	return false
}

func liveMembersExcept(record executionGuardRecord, departing *executionGuardMember) []executionGuardMember {
	members := make([]executionGuardMember, 0, len(record.Members))
	for _, member := range record.Members {
		if departing != nil && member.Pid == departing.Pid && member.PidStartedAt == departing.PidStartedAt {
			continue
		}
		if memberAlive(member) != identity.Dead {
			members = append(members, member)
		}
	}
	return members
}

func updateExecutionGuard(root string, current executionGuardRecord, members []executionGuardMember) error {
	successor := current
	successor.Generation++
	successor.Members = members
	// The public holder name follows the first surviving member, so progress
	// and expiry notes never keep naming an entry process that already exited.
	successor.Pid = members[0].Pid
	successor.PidStartedAt = members[0].PidStartedAt
	successor.Owner = members[0].Owner
	return lock.TransferNamed(executionGuardPath(root), current.identity(), successor.identity(), executionGuardCodec{record: successor})
}

func guardChanged(root string, former lock.Identity) bool {
	current, err := readExecutionGuardRecord(root)
	return err == nil && current.identity() != former
}

func registerExecutionGuardMember(root string, member executionGuardMember) (bool, error) {
	for {
		record, err := readExecutionGuardRecord(root)
		if os.IsNotExist(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !callerDescendsFromMember(member.Pid, record.Members) {
			return false, nil
		}
		members := liveMembersExcept(record, nil)
		for _, existing := range members {
			if existing.Pid == member.Pid && existing.PidStartedAt == member.PidStartedAt {
				return true, nil
			}
		}
		members = append(members, member)
		if err := updateExecutionGuard(root, record, members); err == nil {
			return true, nil
		} else if guardChanged(root, record.identity()) {
			continue
		} else {
			return false, err
		}
	}
}

// RegisterSpawnedExecutionGuardMember registers a detached process while its
// launcher remains in the native ancestry, before detachment can erase proof.
func RegisterSpawnedExecutionGuardMember(root string, pid int64, owner string) error {
	member, err := executionGuardMemberFor(pid, owner)
	if err != nil {
		return err
	}
	joined, err := registerExecutionGuardMember(root, member)
	if err != nil {
		return err
	}
	if !joined {
		return errors.New("checkout execution guard spawn registration refused: the process does not descend from a live registered member")
	}
	return nil
}

// AcquireExecutionGuard waits for exclusive checkout execution. Callers in a
// registered member's exact ancestry join the member list instead of waiting.
func AcquireExecutionGuard(root string, pid int64, owner string, wait, progress time.Duration, notes io.Writer) (GuardResult, error) {
	if wait <= 0 || progress <= 0 {
		return GuardAcquired, errors.New("checkout execution guard wait and progress intervals must be positive")
	}
	if notes == nil {
		notes = io.Discard
	}
	self, err := executionGuardMemberFor(pid, owner)
	if err != nil {
		return GuardAcquired, err
	}
	started := time.Now()
	deadline := started.Add(wait)
	lastProgress := started
	probe := executionGuardProbe(root)
	for {
		if joined, joinErr := registerExecutionGuardMember(root, self); joinErr != nil {
			return GuardAcquired, joinErr
		} else if joined {
			return GuardJoined, nil
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			remaining = time.Millisecond
		}
		slice := 100 * time.Millisecond
		if remaining < slice {
			slice = remaining
		}
		record := executionGuardRecord{
			Pid: self.Pid, PidStartedAt: self.PidStartedAt, Owner: self.Owner,
			Generation: 1, AcquiredAt: clock().UTC().Format("2006-01-02T15:04:05Z"),
			Members: []executionGuardMember{self},
		}
		_, acquireErr := lock.Acquire(executionGuardPath(root), record.identity(), lock.Options{
			Wait: slice, Poll: 25 * time.Millisecond, Probe: probe, Codec: executionGuardCodec{record: record},
			OnStale: func(holder lock.Identity) {
				fmt.Fprintf(notes, "checkout execution guard: removed stale holder %s (pid %d, started %d)\n", holder.Label, holder.Pid, holder.PidStartedAt)
			},
		})
		if acquireErr == nil {
			return GuardAcquired, nil
		}
		var holderErr *lock.HolderError
		if !errors.As(acquireErr, &holderErr) {
			return GuardAcquired, acquireErr
		}
		now := time.Now()
		if now.Sub(lastProgress) >= progress {
			fmt.Fprintf(notes, "checkout execution guard: waiting for %s (pid %d); elapsed %s\n", holderErr.Holder.Label, holderErr.Holder.Pid, now.Sub(started).Round(time.Second))
			lastProgress = now
		}
		if !now.Before(deadline) {
			name := holderErr.Holder.Label
			if name == "" {
				name = "an uninspectable holder"
			}
			return GuardAcquired, fmt.Errorf("checkout execution guard expired after %s waiting for %s (pid %d, started %d)", wait.Round(time.Second), name, holderErr.Holder.Pid, holderErr.Holder.PidStartedAt)
		}
	}
}

// ReleaseExecutionGuard removes the caller's exact membership, sweeps dead
// members, and releases the directory lock only after the last member exits.
func ReleaseExecutionGuard(root string, pid int64) error {
	departing, err := executionGuardMemberFor(pid, "departing member")
	if err != nil {
		return err
	}
	for {
		record, err := readExecutionGuardRecord(root)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("checkout execution guard release could not read its owner: %w", err)
		}
		registered := false
		for _, member := range record.Members {
			if member.Pid == departing.Pid && member.PidStartedAt == departing.PidStartedAt {
				registered = true
				break
			}
		}
		if !registered {
			return fmt.Errorf("checkout execution guard release refused: pid %d is not an exact registered member", pid)
		}
		members := liveMembersExcept(record, &departing)
		if len(members) == 0 {
			if err := lock.ReleaseNamed(executionGuardPath(root), record.identity(), executionGuardCodec{}); err == nil {
				return nil
			} else if guardChanged(root, record.identity()) {
				continue
			} else {
				return err
			}
		}
		if err := updateExecutionGuard(root, record, members); err == nil {
			return nil
		} else if guardChanged(root, record.identity()) {
			continue
		} else {
			return err
		}
	}
}
