package steward

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// HandoffClock is the injected time seam: a test clock advances its own Now
// from Sleep, so no witness waits on wall time.
type HandoffClock struct {
	Now   func() time.Time
	Sleep func(time.Duration)
}

func handoffWaiterClass(name string) (run.WaiterStateClass, bool) {
	for _, state := range run.WaiterStates {
		if state.Name == name {
			return state.Class, true
		}
	}
	return run.WaiterStateEnded, false
}

func readHandoffWaiter(root, relative string) (run.Waiter, error) {
	source, err := readStableSource(root, relative, "waiter record", true)
	if err != nil {
		return run.Waiter{}, err
	}
	var row run.Waiter
	if err := json.Unmarshal(source.data, &row); err != nil {
		return row, fmt.Errorf("handoff waiter record %s is malformed: %w", source.source, err)
	}
	return row, nil
}

func graceHandoffWaiter(root, relative, mainID string, row run.Waiter, callStarted time.Time, clock HandoffClock) (run.Waiter, bool, error) {
	if row.State != run.WaiterStateRegistering {
		return row, true, nil
	}
	now := clock.Now()
	registered, parseErr := time.Parse(time.RFC3339Nano, row.RegisteredAt)
	graceEnd := callStarted.Add(5 * time.Second)
	if parseErr == nil {
		if !now.Before(registered.Add(5 * time.Second)) {
			return row, true, nil
		}
		if registered.Add(5 * time.Second).Before(graceEnd) {
			graceEnd = registered.Add(5 * time.Second)
		}
	} else {
		graceEnd = now.Add(5 * time.Second)
	}
	for polls := 0; polls < 20 && clock.Now().Before(graceEnd); polls++ {
		clock.Sleep(250 * time.Millisecond)
		current, err := readHandoffWaiter(root, relative)
		if err != nil || current.MainId != mainID || current.SchemaVersion != 2 {
			return current, false, fmt.Errorf("waiter changed during handoff grace")
		}
		class, known := handoffWaiterClass(current.State)
		if !known {
			return current, false, fmt.Errorf("handoff waiter record %s has unknown state %q", relative, current.State)
		}
		if class != run.WaiterStateInFlight {
			return current, false, nil
		}
		row = current
		if row.State != run.WaiterStateRegistering {
			break
		}
	}
	return row, true, nil
}

// EndInFlightWaits interrupts the caller's in-flight waiter rows and returns
// the durable work needed by a successor.
func EndInFlightWaits(stateRoot string, caller HandoffCaller, nonce string, clock HandoffClock) ([]HandoffOpenWork, error) {
	if !handoffNoncePattern.MatchString(nonce) {
		return nil, fmt.Errorf("handoff nonce must be 16 lowercase hexadecimal characters")
	}
	if clock.Now == nil {
		clock.Now = time.Now
	}
	if clock.Sleep == nil {
		clock.Sleep = time.Sleep
	}
	callStarted := clock.Now()
	entries, err := os.ReadDir(run.WaitersDir(stateRoot))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var result []HandoffOpenWork
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		relative := filepath.ToSlash(filepath.Join("artifacts", "agents", "waiters", entry.Name()))
		candidate, err := os.ReadFile(filepath.Join(stateRoot, filepath.FromSlash(relative)))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		handoffCandidateRead(relative)
		var row run.Waiter
		if err := json.Unmarshal(candidate, &row); err != nil {
			return nil, fmt.Errorf("handoff waiter record %s is malformed: %w", relative, err)
		}
		if row.SchemaVersion == 0 || row.MainId != caller.MainId {
			continue
		}
		row, err = readHandoffWaiter(stateRoot, relative)
		if err != nil {
			return nil, err
		}
		if row.MainId != caller.MainId {
			continue
		}
		if row.SchemaVersion != 2 {
			return nil, fmt.Errorf("handoff waiter record %s has unsupported schema %d", relative, row.SchemaVersion)
		}
		class, known := handoffWaiterClass(row.State)
		if !known {
			return nil, fmt.Errorf("handoff waiter record %s has unknown state %q", relative, row.State)
		}
		if class != run.WaiterStateInFlight {
			continue
		}
		rowID := strings.TrimSuffix(entry.Name(), ".json")
		row, selected, graceErr := graceHandoffWaiter(stateRoot, relative, caller.MainId, row, callStarted, clock)
		if graceErr != nil {
			return nil, refusal("HANDOFF_WAIT_UNENDABLE", "row="+rowID)
		}
		if !selected {
			continue
		}
		path := filepath.Join(stateRoot, filepath.FromSlash(relative))
		ended, err := (&run.Store{Root: stateRoot}).InterruptWaiterRow(path, "handoff "+nonce, run.WaitOptions{
			Now: clock.Now, Sleep: func(_ context.Context, duration time.Duration) error { clock.Sleep(duration); return nil },
		})
		if err != nil {
			return nil, refusal("HANDOFF_WAIT_UNENDABLE", "row="+rowID)
		}
		left := int64(0)
		if deadline, parseErr := time.Parse(time.RFC3339Nano, ended.Deadline); parseErr == nil {
			left = int64(deadline.Sub(clock.Now()) / time.Second)
			if left < 0 {
				left = 0
			}
		}
		work := HandoffOpenWork{RowID: rowID, WaitID: ended.WaitID, Kind: ended.Kind, Selector: ended.Selector, Target: ended.Target, DeadlineLeftSeconds: left}
		if ended.Pid > 0 && ended.PidStartedAt > 0 {
			work.Pid, work.PidStartedAt = ended.Pid, ended.PidStartedAt
		}
		result = append(result, work)
	}
	return result, nil
}
