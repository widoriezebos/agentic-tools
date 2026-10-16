package plain

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
	landingstate "github.com/widoriezebos/agentic-tools/metasystem/internal/landing/state"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

type clockWake struct {
	At        time.Time `json:"at"`
	RetryAt   time.Time `json:"retry-at,omitempty"`
	Credit    string    `json:"credit"`
	Attempted bool      `json:"attempted"`
	Batch     string    `json:"batch,omitempty"`
	Gate      string    `json:"gate,omitempty"`
	Push      string    `json:"push,omitempty"`
	Reason    string    `json:"reason"`
}

func clockPath(install string) string { return filepath.Join(Dir(install), "clock.json") }
func readClock(install string) (clockWake, error) {
	var state clockWake
	data, err := os.ReadFile(clockPath(install))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &state)
	}
	return state, err
}

// ClockReason is the keeper's recorded explanation for the latest clock wake.
func ClockReason(install string) string {
	state, _ := readClock(install)
	if state.Reason == "" {
		return ""
	}
	return state.Reason + " (at " + state.At.Local().Format("15:04") + ")"
}

// KeeperClock gives one batch attempt priority over a due main check. It is
// called by the keeper before deciding whether this wake launches an agent.
func KeeperClock(record lane.Record, prepare func(lane.Record) error, seams ProveSeams) (hold string, err error) {
	install, checkout := record.Install, record.Root
	if seams.TimerHeld != nil && seams.TimerHeld() {
		return "", prepare(record)
	}
	drain, err := ReadDrain(install)
	if err != nil || drain != nil {
		return "", errors.Join(err, prepare(record))
	}
	if err = os.MkdirAll(Dir(install), 0755); err != nil {
		return "", err
	}
	guard, err := lock.File(filepath.Join(Dir(install), "clock-step.lock"), 0600, lock.Exclusive)
	if err != nil {
		return "", err
	}
	defer guard.Release()
	state, err := readClock(install)
	if err != nil {
		return "", err
	}
	reason := state.Reason
	defer func() {
		if state.At.IsZero() || state.Reason != reason {
			state.At = seams.now()
		}
		data, marshalErr := json.Marshal(state)
		if marshalErr == nil {
			_, marshalErr = atomicfile.WriteFile(clockPath(install), append(data, '\n'), 0600, install)
		}
		err = errors.Join(err, marshalErr)
	}()
	last, every, err := fullProofClock(install)
	if err != nil {
		return "", err
	}
	debt, err := scopedProofDue(install, seams.now())
	if err != nil {
		return "", err
	}
	if !debt && !last.IsZero() && seams.now().Sub(last) < every {
		state = clockWake{At: state.At, Reason: "clock check skipped; main's full check is current"}
		return "", prepare(record)
	}
	if state.Credit != last.String() || !state.RetryAt.IsZero() && !seams.now().Before(state.RetryAt) {
		state = clockWake{Credit: last.String()}
	}
	if !state.RetryAt.IsZero() {
		return "", prepare(record)
	}
	running, recorded, alive, err := ReadRunning(install, seams)
	if err != nil {
		return "", err
	}
	if alive {
		state.Reason = "clock due; waiting for the current gate or proof to finish"
		return state.Reason, prepare(record)
	}
	repair, err := candidateRepairPending(install, running, recorded)
	if err != nil {
		return "", err
	}
	if repair != nil {
		state.Attempted, state.Batch = true, running.BatchID
		state.Reason = "clock due; waiting for the candidate's repair to finish"
		return "", prepare(record)
	}
	prepared := false
	if !state.Attempted {
		state.Attempted = true
		selectionErr := prepare(record)
		// A refused fallback reuses this wake's preparation result.
		prepare = func(lane.Record) error { return selectionErr }
		batch, readErr := ReadBatch(install)
		if readErr != nil {
			return "", readErr
		}
		if selectionErr == nil && batch != nil && batch.State != BatchClosed {
			state.Batch = batch.ID
			if err := withLock(install, func() error {
				current, err := ReadBatch(install)
				if err != nil {
					return err
				}
				if current == nil || current.ID != batch.ID {
					return errors.New("the clock's selected batch changed")
				}
				current.ClockFull = true
				return writeBatch(install, current, seams)
			}); err != nil {
				return "", err
			}
			state.Reason = "clock due; batch " + batch.ID + " gets its full check first"
			prepared = true
		}
	}
	if state.Batch != "" {
		results, err := Results(install)
		if err != nil {
			return "", err
		}
		var candidate Result
		for _, result := range results {
			if !result.Trunk && result.BatchID == state.Batch {
				candidate = result
			}
		}
		if candidate.Result == Green && state.Push != candidate.Attempt+candidate.At {
			state.Push = candidate.Attempt + candidate.At
			state.Reason = "clock due; batch " + state.Batch + " gets one wake to push its green full check"
			return "", nil
		}
		gate, ok, err := LastGate(install)
		if err != nil {
			return "", err
		}
		if candidate.Result == "" && ok && gate.BatchID == state.Batch && gate.Result == Green && state.Gate != gate.Attempt+gate.At {
			state.Gate = gate.Attempt + gate.At
			state.Reason = "clock due; the batch's gate passed, its full check goes next"
			return "", nil
		}
		if prepared && candidate.Result == "" && (!ok || gate.BatchID != state.Batch) {
			return "", nil
		}
	}
	seams.Trunk, seams.Gate, seams.Impact = true, false, false
	seams.Person, seams.Fresh = nil, true
	state.Reason = "clock due; no batch completed within its wake; " + landingstate.Phrase("proving-main")
	if debt {
		results, _ := Results(install)
		for i := len(results) - 1; i >= 0; i-- {
			if results[i].Scope == "scoped" {
				at := results[i].OwedSince
				if at.IsZero() {
					at, _ = time.Parse(time.RFC3339, results[i].FullAt)
				}
				state.Reason = "the hourly debt of the " + at.Local().Format("15:04") + " check is due"
				break
			}
		}
	}
	_, _, err = Start(install, checkout, seams)
	if err != nil {
		state.Reason = "clock due; main's check could not start: " + err.Error()
		state.RetryAt = seams.now().Add(every)
		return "", prepare(record)
	}
	return state.Reason, err
}
