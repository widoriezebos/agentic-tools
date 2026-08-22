package missionrunner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/wiredoc"
)

// TurnRecord is the typed read lens over a turn record's wire document.
// Same contract as
// dispatch's JobRecord: it interprets the fields decisions dereference,
// the document stays the permissive map, and an ill-typed field reads as
// its zero value — one tolerance for every caller, in one place.
type TurnRecord struct {
	doc *wiredoc.Doc
}

// TurnRecordOf wraps an already-decoded turn document.
func TurnRecordOf(turn map[string]any) TurnRecord {
	return TurnRecord{doc: wiredoc.FromRaw(turn)}
}

func (r TurnRecord) text(key string) string {
	value, _ := r.doc.Get(key)
	text, _ := value.(string)
	return text
}

// Runtime is the host runtime the turn launches under.
func (r TurnRecord) Runtime() string { return r.text("runtime") }

// Status is the turn's lifecycle status.
func (r TurnRecord) Status() string { return r.text("status") }

// Outcome is the turn's terminal outcome ("" while running).
func (r TurnRecord) Outcome() string { return r.text("outcome") }

// HostSession is the announced resume session ("" when none).
func (r TurnRecord) HostSession() string { return r.text("hostSession") }

// InstanceTag is the minted host tag ("" before launch).
func (r TurnRecord) InstanceTag() string { return r.text("instanceTag") }

// TurnCapMin is the wall-clock allowance (0, false when absent/ill-typed).
func (r TurnRecord) TurnCapMin() (int64, bool) {
	value, _ := r.doc.Get("turnCapMin")
	return jsonInt(value)
}

// Raw is the underlying document for the permissive paths.
func (r TurnRecord) Raw() map[string]any { return r.doc.Raw() }
