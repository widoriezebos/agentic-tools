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
