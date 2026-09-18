package run

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const (
	DefaultLocalWaitTimeout  = 4 * time.Hour
	MaxRegisteredWaitTimeout = 24 * time.Hour
)

// RegisterWaitRequest describes work that another process or a person must
// finish. These registrations remain pending without a blocking wait process.
type RegisterWaitRequest struct {
	Kind              string
	Pid               int64
	Label             string
	Question          string
	JobID             string
	Owner             Caller
	RuntimeSession    string
	Runtime           string
	Timeout           time.Duration
	OpenWorkSignature string
}

func exactRegisteredWaitMode(mode identity.ComparisonMode) bool {
	return mode == identity.CompareDarwinMicroseconds || mode == identity.CompareLinuxTicksBootID
}

func validateRegisterWaitRequest(request RegisterWaitRequest) error {
	if request.Kind != "local" && request.Kind != "human" {
		return fmt.Errorf("registered wait kind must be local or human")
	}
	if request.Owner.MainId == "" || request.Owner.OwnerLineage == "" || request.Owner.SessionId == "" ||
		request.Owner.ClaimEpoch == nil || request.RuntimeSession == "" {
		return fmt.Errorf("wait registration requires a classified owner, lineage, claim epoch, session, and runtime session")
	}
	if request.Owner.SessionId != request.RuntimeSession {
		return fmt.Errorf("wait registration session and runtime session must match")
	}
	if request.Timeout <= 0 || request.Timeout > MaxRegisteredWaitTimeout {
		return fmt.Errorf("wait timeout must be positive and no longer than 24 hours")
	}
	if request.Kind == "local" {
		if request.Pid <= 0 || strings.TrimSpace(request.Label) == "" {
			return fmt.Errorf("local wait requires a positive pid and a label")
		}
		if request.Question != "" {
			return fmt.Errorf("local wait cannot carry a human question")
		}
	} else {
		if request.Pid != 0 || strings.TrimSpace(request.Question) == "" {
			return fmt.Errorf("human wait requires a question and no pid")
		}
		if request.Label != "" || request.JobID != "" {
			return fmt.Errorf("human wait cannot carry a label or job")
		}
	}
	return nil
}

func registeredWaitRef(row Waiter) identity.Ref {
	return identity.Ref{Pid: row.Pid, StartedAtSec: row.PidStartedAt, StartedAtUnixMicro: row.PidStartedAtMicro,
		StartTicks: row.PidStartTicks, BootID: row.BootID}
}

func registeredWaitOwnedBy(row Waiter, owner Caller, runtimeSession string) bool {
	return registeredWaitSameOwner(row, owner) &&
		row.Session == owner.SessionId && row.RuntimeSession == runtimeSession &&
		row.ClaimEpoch != nil && owner.ClaimEpoch != nil && *row.ClaimEpoch == *owner.ClaimEpoch
}

func registeredWaitSameOwner(row Waiter, owner Caller) bool {
	return row.MainId == owner.MainId && row.OwnerDigest == OwnerDigest(owner.MainId) &&
		(row.OwnerLineage == owner.OwnerLineage || row.OwnerLineage == owner.MainId)
}

func endRegisteredWaitRow(row *Waiter, by string, now time.Time) {
	result := waitResult(*row, ExitInterrupted, WaiterStateInterrupted,
		"registered wait ended by "+by, "interrupted", "", row.LastCheckedTip, "", now)
	row.State = WaiterStateInterrupted
	row.RemainingNanos = 0
	row.InterruptedBy = by
	row.Result = &result
}

func (s *Store) sweepRegisteredWaits(owner Caller, runtimeSession, by string, options WaitOptions) error {
	paths, err := filepath.Glob(filepath.Join(WaitersDir(s.Root), "*-"+OwnerDigest(owner.MainId)+".json"))
	if err != nil {
		return err
	}
	now := options.Now().UTC()
	for _, path := range paths {
		row, readErr := readV2Waiter(path)
		if readErr != nil || !registeredWaitSameOwner(row, owner) || row.State != WaiterStatePending || row.Result != nil {
			continue
		}
		end := false
		switch row.Kind {
		case "local":
			liveness, mode := identity.AliveRefComparison(s.prober(), registeredWaitRef(row))
			end = liveness == identity.Dead && exactRegisteredWaitMode(mode)
		case "human":
			deadline, parseErr := time.Parse(time.RFC3339Nano, row.Deadline)
			end = parseErr != nil || !now.Before(deadline)
		}
		if end {
			endRegisteredWaitRow(&row, by, now)
			if err := writeV2Waiter(path, row); err != nil {
				return err
			}
		}
	}
	return nil
}

// RegisterDetachedWait records local process work or a pending human answer.
// The row itself is the durable watch; no process heartbeats it.
func (s *Store) RegisterDetachedWait(request RegisterWaitRequest, options WaitOptions) (Waiter, error) {
	options = normalizeWaitOptions(options)
	if err := validateRegisterWaitRequest(request); err != nil {
		return Waiter{}, &waiterError{ExitInvalidWait, err.Error()}
	}
	if err := ValidateWaitSelector(WaitSelector{Kind: request.Kind, TargetID: strings.Repeat("a", 32), Question: request.Question}); err != nil {
		return Waiter{}, &waiterError{ExitInvalidWait, err.Error()}
	}

	var processRef identity.Ref
	var registeredBootID string
	var registeredBootNanos int64
	if request.Kind == "local" {
		exact, state, err := s.prober().Probe(request.Pid)
		if state == identity.Dead {
			return Waiter{}, &waiterError{ExitNoRecord, fmt.Sprintf("process %d is not alive", request.Pid)}
		}
		if err != nil || state != identity.Alive {
			return Waiter{}, &waiterError{ExitWaiterUnknown, fmt.Sprintf("process %d identity is uncertain", request.Pid)}
		}
		processRef = exact.Ref()
		if !exactRegisteredWaitMode(processRef.Mode()) {
			return Waiter{}, &waiterError{ExitWaiterUnknown, fmt.Sprintf("process %d identity is not exact enough to detect pid reuse", request.Pid)}
		}
		bootID, elapsed, err := options.BootClock()
		if err != nil || bootID == "" {
			return Waiter{}, &waiterError{ExitWaiterUnknown, "the boot identity is unavailable"}
		}
		registeredBootID, registeredBootNanos = bootID, elapsed.Nanoseconds()
	}

	waitID, err := mintNonce()
	if err != nil {
		return Waiter{}, &waiterError{ExitWaiterIO, err.Error()}
	}
	nonce, err := mintNonce()
	if err != nil {
		return Waiter{}, &waiterError{ExitWaiterIO, err.Error()}
	}
	now := options.Now().UTC()
	deadline := now.Add(request.Timeout)
	selector := WaitSelector{Kind: request.Kind, TargetID: waitID, Question: request.Question}
	row := Waiter{
		SchemaVersion: 2, WaitID: waitID, Nonce: nonce, Kind: request.Kind, TargetID: waitID,
		OwnerDigest: OwnerDigest(request.Owner.MainId), Pid: processRef.Pid, PidStartedAt: processRef.StartedAtSec,
		PidStartedAtMicro: processRef.StartedAtUnixMicro, PidStartTicks: processRef.StartTicks, BootID: processRef.BootID,
		Session: request.Owner.SessionId, MainId: request.Owner.MainId, OwnerLineage: request.Owner.OwnerLineage,
		ClaimEpoch: request.Owner.ClaimEpoch, RuntimeSession: request.RuntimeSession, Runtime: request.Runtime,
		Label: strings.TrimSpace(request.Label), Question: strings.TrimSpace(request.Question), JobID: request.JobID,
		Selector: selector, RegisteredAt: now.Format(time.RFC3339Nano), Deadline: deadline.Format(time.RFC3339Nano),
		RemainingNanos: request.Timeout.Nanoseconds(), RegisteredBootNanos: registeredBootNanos,
		RegisteredBootID: registeredBootID, OpenWorkSignature: request.OpenWorkSignature,
		State: WaiterStatePending, Delivery: map[string]string{"local": "harness", "human": "human"}[request.Kind],
	}
	rowPath := WaiterPath(s.Root, row.Kind, row.TargetID, row.OwnerDigest)
	err = withWaiterLock(s.Root, func() error {
		if err := s.checkEpoch(request.Owner); err != nil {
			return err
		}
		if err := s.sweepRegisteredWaits(request.Owner, request.RuntimeSession, request.Owner.MainId, options); err != nil {
			return err
		}
		if _, err := os.Lstat(rowPath); err == nil || !os.IsNotExist(err) {
			return fmt.Errorf("wait identifier collision")
		}
		if err := writeV2Waiter(rowPath, row); err != nil {
			return err
		}
		if err := writeV2Pointer(s.Root, row, rowPath); err != nil {
			_ = os.Remove(rowPath)
			return err
		}
		return nil
	})
	if err != nil {
		return Waiter{}, &waiterError{ExitWaiterIO, err.Error()}
	}
	return row, nil
}

// EndDetachedWait ends the caller's registration. Repeating the operation is
// harmless, while a different owner cannot change the row.
func (s *Store) EndDetachedWait(waitID string, owner Caller, runtimeSession string, options WaitOptions) (Waiter, error) {
	options = normalizeWaitOptions(options)
	if !ValidWaitID(waitID) {
		return Waiter{}, &waiterError{ExitInvalidWait, "wait identifier is invalid"}
	}
	row, rowPath, err := FindWaiterByID(s.Root, waitID)
	if err != nil {
		return Waiter{}, &waiterError{ExitNoRecord, err.Error()}
	}
	if !registeredWaitOwnedBy(row, owner, runtimeSession) {
		return Waiter{}, &waiterError{ExitWaiterBusy, "wait belongs to another owner or session"}
	}
	if row.Kind != "local" && row.Kind != "human" {
		return Waiter{}, &waiterError{ExitInvalidWait, "wait end applies only to local and human registrations"}
	}
	err = withWaiterLock(s.Root, func() error {
		current, err := readV2Waiter(rowPath)
		if err != nil {
			return err
		}
		if !registeredWaitOwnedBy(current, owner, runtimeSession) {
			return &waiterError{ExitWaiterBusy, "wait belongs to another owner or session"}
		}
		for _, state := range WaiterStates {
			if state.Name == current.State && state.Class == WaiterStateEnded {
				row = current
				return nil
			}
		}
		endRegisteredWaitRow(&current, owner.MainId, options.Now().UTC())
		if err := writeV2Waiter(rowPath, current); err != nil {
			return err
		}
		row = current
		return nil
	})
	if err != nil {
		if coded, ok := err.(*waiterError); ok {
			return Waiter{}, coded
		}
		return Waiter{}, &waiterError{ExitWaiterIO, err.Error()}
	}
	return row, nil
}
