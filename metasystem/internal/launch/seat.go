package launch

import (
	"errors"
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/stopfence"
)

// SeatEnvironment is what a seat launch sets over the launcher's environment:
// the seat's owner lineage, so up adopts it for the announcement and the
// lease and every command acts under it, and an empty delegate root and
// session id, so the child is a session of the checkout and not a delegate
// or the launcher's own session. Every other variable is inherited.
func SeatEnvironment() []string {
	return []string{"METASYSTEM_OWNER_LINEAGE=" + SeatOwnerLineage, "METASYSTEM_DELEGATE_ROOT=", "METASYSTEM_SESSION_ID="}
}

// seatNoFenceRoot refuses a seat that names no state root to bind its
// process-creation fence to.
const seatNoFenceRoot = "this seat launch names no state root, so it has no stop fence to bind to"

// seatFenceClosed reads the checkout's process-creation fence once and
// returns the fence's own description when it is closed, or why it could not
// be read; empty means a seat may be created.
func seatFenceClosed(checkout string) string {
	closed, record, err := stopfence.Closed(checkout)
	if err != nil {
		return "the process-creation fence of " + checkout + " cannot be read: " + err.Error()
	}
	if !closed {
		return ""
	}
	description, err := stopfence.ClosedDescription(record, checkout)
	if err != nil {
		return err.Error()
	}
	return description
}

// seatFenceMoved is the re-read after the child is recorded: a fence closed
// since, or reopened at another generation, ends the seat.
func seatFenceMoved(checkout string, generation int64) string {
	if reason := seatFenceClosed(checkout); reason != "" {
		return reason
	}
	record, err := stopfence.Read(checkout)
	if err != nil {
		return "the process-creation fence of " + checkout + " cannot be read: " + err.Error()
	}
	if record.Generation != generation {
		return fmt.Sprintf("%s was restarted while the seat started, so this seat was ended", checkout)
	}
	return ""
}

// endFencedSeat ends a seat child the fence refused after it started: the
// group is ended, the child reaped, and the record fails with the reason.
func (m *Manager) endFencedSeat(id, reason string, child Child, childRef identity.Ref) (Record, error) {
	cleanupErr := m.endGroup(childRef.Pid)
	if cleanupErr == nil {
		_, _ = child.Wait()
	}
	record, err := m.update(id, func(current *Record) error {
		if current.State.Terminal() {
			return nil
		}
		current.State, current.Reason = Failed, reason
		if cleanupErr != nil {
			current.Reason += "; process-group-unproven: " + cleanupErr.Error()
		}
		current.FinishedAt = m.Now().UTC().Format(time.RFC3339Nano)
		return nil
	})
	if err != nil {
		return Record{}, errors.Join(errors.New(reason), cleanupErr, err)
	}
	return record, errors.Join(errors.New(reason), cleanupErr)
}
