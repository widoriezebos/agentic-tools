package dispatch

import (
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

// Role is the dispatched role.
func (r JobRecord) Role() string { return r.text("role") }

// ParentJob is the chain parent ("" for a root).
func (r JobRecord) ParentJob() string { return r.text("parentJob") }

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
