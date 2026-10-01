package lane

// Why the landing agent would run now (landing-lane-runtime-redesign §3,
// Wake): work is queued, a batch is unfinished, validation is due, or a
// finalization is pending. landing status --json carries these as "wake",
// and the keeper wakes the agent on the very same read, in process: no
// process reads another's text. An idle lane runs no model.

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gaterun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
)

// The wake reasons, in the order they are listed.
const (
	WakeQueued              = "queued"
	WakeUnfinishedBatch     = "unfinished-batch"
	WakeValidationDue       = "validation-due"
	WakeFinalizationPending = "finalization-pending"
)

// Wake is why the landing agent would run now. Reasons empty is an idle
// lane. Unread names each source that could not be read and why: an unread
// source is never a reason, and it is shown, never hidden.
type Wake struct {
	Reasons []string `json:"reasons"`
	Unread  []string `json:"unread"`
}

// WakeSources are the reads a wake is built from. Records nil reads the
// lane's batch records from its checkout. Validation says whether the
// standing validation is due in the lane's installation (the recorded
// Layout's Install, K-a); Finalization whether
// a validation run awaits its finalization. nil reads nothing for that
// reason.
type WakeSources struct {
	Records      func(root string) ([]batch.Record, error)
	Validation   func(install string, now time.Time) (bool, error)
	Finalization func(root string) (bool, error)
}

// ReadWake reads why the landing agent of the registered lane would run
// now.
func ReadWake(record Record, now time.Time, sources WakeSources) Wake {
	records, err := readRecords(ViewSources{Records: sources.Records}, record.Root)
	return wakeOf(record, now, records, err, sources)
}

func wakeOf(record Record, now time.Time, records []batch.Record, recordsErr error, sources WakeSources) Wake {
	root := record.Root
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
	if sources.Validation != nil {
		due, err := validationOf(record, now, sources.Validation)
		switch {
		case err != nil:
			wake.Unread = append(wake.Unread, "validation: "+err.Error())
		case due:
			wake.Reasons = append(wake.Reasons, WakeValidationDue)
		}
	}
	if sources.Finalization != nil {
		pending, err := sources.Finalization(root)
		switch {
		case err != nil:
			wake.Unread = append(wake.Unread, "finalization: "+err.Error())
		case pending:
			wake.Reasons = append(wake.Reasons, WakeFinalizationPending)
		}
	}
	return wake
}

// batchReasons reads the batch records: queued is a batch collecting with a
// member joined; unfinished is a batch past collecting that has neither
// landed nor dissolved (proving, landing, diagnosing, or held).
func batchReasons(records []batch.Record) (queued, unfinished bool) {
	for _, record := range records {
		switch record.State {
		case batch.StateLanded, batch.StateDissolved:
		case batch.StateOpen, batch.StateSealed:
			if len(members(record)) > 0 {
				queued = true
			}
		default:
			unfinished = true
		}
	}
	return queued, unfinished
}

// validationOf asks the due read about the lane's recorded installation,
// never a root guessed from the checkout (K-a).
func validationOf(record Record, now time.Time, due func(string, time.Time) (bool, error)) (bool, error) {
	layout, err := record.Layout()
	if err != nil {
		return false, err
	}
	return due(string(layout.Install), now)
}

// ValidationDue is the production due read of the lane installation at
// install: by its goal ledger (the latest cadence status) and validation
// weight, the cadence's own clock and weight rules (gaterun.CadenceDueByClock).
// A ledger with no cadence status yet is due, by the cadence's rule. It
// writes nothing and takes no lock. Whether the trunk's deep-only groups
// changed identity needs a fetch and a revalidation, which validate judges
// when it runs.
func ValidationDue(install string, now time.Time) (bool, error) {
	return validationDueWith(install, now, goal.ResolveEndpoint)
}

func validationDueWith(module string, now time.Time, resolve func(string) (goal.Endpoint, error)) (bool, error) {
	endpoint, err := resolve(module)
	if err != nil {
		return false, fmt.Errorf("the goal ledger of %s cannot be read: %w", module, err)
	}
	projection, err := goal.Project(endpoint, false, now)
	if err != nil {
		return false, fmt.Errorf("the goal ledger of %s cannot be read: %w", module, err)
	}
	if projection.Tree == nil {
		return false, fmt.Errorf("the goal ledger of %s holds no goals tree", module)
	}
	weightDue, err := gaterun.WeightDueRead(module, gaterun.WeightThreshold(module), now)
	if err != nil {
		return false, fmt.Errorf("the validation weight of %s cannot be read: %w", module, err)
	}
	return gaterun.CadenceDueByClock(now, projection.Tree.Cadence, weightDue)
}
