package dispatch

import (
	"encoding/json"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// JobRecord is the typed READ LENS over a job record's wire document.
// It interprets the fields
// decisions dereference and nothing more; the document itself stays the
// permissive map the CAS path patches under its own rules, so nothing the
// wire accepts today is refused by going typed. An ill-typed field reads as
// its zero value plus ok=false — exactly what the asString/jsonInt casts
// give their callers, in one place per field.
type JobRecord struct {
	doc *wiredoc.Doc
}

// JobRecordOf wraps an already-decoded record map.
func JobRecordOf(record map[string]any) JobRecord {
	return JobRecord{doc: wiredoc.FromRaw(record)}
}

func (r JobRecord) text(key string) string {
	value, _ := r.doc.Get(key)
	text, _ := value.(string)
	return text
}

// Status is the record's lifecycle status ("" when absent or ill-typed).
func (r JobRecord) Status() string { return r.text("status") }

// JobID is the record's job identity.
func (r JobRecord) JobID() string { return r.text("jobId") }

// OperationID is the immutable reservation identity used for attempt
// accounting. Replaying the same operation does not mint another attempt.
func (r JobRecord) OperationID() string { return r.text("operationId") }

// Role is the dispatched role.
func (r JobRecord) Role() string { return r.text("role") }

// ParentJob is the chain parent ("" for a root).
func (r JobRecord) ParentJob() string { return r.text("parentJob") }

// GoalID is the optional goal this job was reserved for.
func (r JobRecord) GoalID() string { return r.text("goalId") }

// MachineID is the claim machine copied into a goal-bound reservation.
func (r JobRecord) MachineID() string { return r.text("machineId") }

// ApprovedRef is the immutable human word authorizing an oversized slice.
func (r JobRecord) ApprovedRef() string { return r.text("approvedRef") }

func (r JobRecord) uint64Field(key string) (uint64, bool) {
	value, present := r.doc.Get(key)
	if !present {
		return 0, false
	}
	switch number := value.(type) {
	case json.Number:
		parsed, err := strconv.ParseUint(number.String(), 10, 64)
		return parsed, err == nil
	case float64:
		if number < 0 || number != float64(uint64(number)) {
			return 0, false
		}
		return uint64(number), true
	case uint64:
		return number, true
	case int64:
		if number < 0 {
			return 0, false
		}
		return uint64(number), true
	}
	return 0, false
}

// GoalRevision is the exact claimed-goal revision this reservation spends.
func (r JobRecord) GoalRevision() (uint64, bool) { return r.uint64Field("goalRevision") }

// ClaimEpoch is the checkout generation that bound the reservation.
func (r JobRecord) ClaimEpoch() (int64, bool) {
	value, present := r.doc.Get("claimEpoch")
	if !present {
		return 0, false
	}
	return numInt(value)
}

// CapMinutes is the immutable reserved runtime cap.
func (r JobRecord) CapMinutes() (uint64, bool) { return r.uint64Field("capMin") }

// EndedAt is the terminal timestamp ("" while the job runs).
func (r JobRecord) EndedAt() string { return r.text("endedAt") }

// ErrorText is the record's error field ("" when null, absent, or clean).
func (r JobRecord) ErrorText() string { return r.text("error") }

// Round is the critique round number (0, false when absent or ill-typed).
func (r JobRecord) Round() (int64, bool) {
	value, _ := r.doc.Get("round")
	return numInt(value)
}

// Raw is the underlying document for the permissive paths.
func (r JobRecord) Raw() map[string]any { return r.doc.Raw() }
