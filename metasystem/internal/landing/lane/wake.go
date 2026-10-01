package lane

// Why the landing agent would run now (simple lane §1): work is queued or a
// batch is unfinished. landing status --json carries these as "wake",
// and the keeper wakes the agent on the very same read, in process: no
// process reads another's text. An idle lane runs no model.

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The wake reasons, in the order they are listed.
const (
	WakeQueued          = "queued"
	WakeUnfinishedBatch = "unfinished-batch"
)

// Wake is why the landing agent would run now. Reasons empty is an idle
// lane. Unread names each source that could not be read and why: an unread
// source is never a reason, and it is shown, never hidden.
type Wake struct {
	Reasons []string `json:"reasons"`
	Unread  []string `json:"unread"`
}

// WakeSources are the reads a wake is built from. Records nil reads the
// lane's batch records from its checkout.
type WakeSources struct {
	Records func(root string) ([]batch.Record, error)
}

// ReadWake reads why the landing agent of the registered lane would run
// now.
func ReadWake(record Record, now time.Time, sources WakeSources) Wake {
	records, err := readRecords(ViewSources{Records: sources.Records}, record.Root)
	return wakeOf(records, err)
}

func wakeOf(records []batch.Record, recordsErr error) Wake {
	wake := Wake{Reasons: []string{}, Unread: []string{}}
	if recordsErr != nil {
		wake.Unread = append(wake.Unread, "batches: "+recordsErr.Error())
	}
	queued, unfinished := batchReasons(records)
	if queued {
		wake.Reasons = append(wake.Reasons, WakeQueued)
	}
	if unfinished {
		wake.Reasons = append(wake.Reasons, WakeUnfinishedBatch)
	}
	return wake
}

// batchReasons reads the batch records: queued is a batch collecting with a
// member joined; unfinished is a batch past collecting that has neither
// landed nor dissolved (proving, landing, diagnosing, or held) and still
// has a member: a memberless one is the keeper's to settle, not the
// agent's.
func batchReasons(records []batch.Record) (queued, unfinished bool) {
	for _, record := range records {
		switch record.State {
		case batch.StateLanded, batch.StateDissolved:
		case batch.StateOpen, batch.StateSealed:
			if len(members(record)) > 0 {
				queued = true
			}
		default:
			if !batch.Memberless(record) {
				unfinished = true
			}
		}
	}
	return queued, unfinished
}

// keeperActor is the actor a keeper's settlement records in a batch's
// history.
const keeperActor = "landing-keeper"

// SettleMemberless settles as dissolved each batch of the lane at root that
// is past collecting and has no member left (batch.Memberless): nothing in
// it can be proven, pushed or returned, so no agent is woken for it. It
// lists the batches it settled; a batch it could not settle is in the error,
// and the rest are still settled.
func SettleMemberless(root string, now time.Time) ([]string, error) {
	records, err := readRecords(ViewSources{}, root)
	store := batch.NewStore(root, nil)
	settled := []string{}
	var failed []error
	for _, record := range records {
		if !batch.Memberless(record) {
			continue
		}
		done, settleErr := batch.DissolveMemberless(store, record.BatchID, keeperActor, now)
		if settleErr != nil {
			failed = append(failed, fmt.Errorf("batch %s: %w", record.BatchID, settleErr))
			continue
		}
		if done {
			settled = append(settled, record.BatchID)
		}
	}
	return settled, errors.Join(append([]error{err}, failed...)...)
}
