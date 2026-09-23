package partner

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/backlog"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/ui/snapshot"
)

// What the conversation can point at.
//
// An answer that names g1-s26 should link it, and an answer that names a
// record by its title should link that too — but only when the title names one
// record and not three. Deciding either needs the ledger's goals and the
// checkout's records, and the page has no reader for them: the drawer stands
// over every section, including the ones that read neither.
//
// So the index rides the conversation's own snapshot, which the page already
// reads on load and on every reconnect. It is not a second request and not a
// poll; it is the same answer, carrying the two lists that make a reference
// resolvable. Resolution is the page's, because the ring and the navigation
// are the page's.

// Index is the goals and records this workspace carries, as ids.
type Index struct {
	Goals   []string        `json:"goals"`
	Records []IndexedRecord `json:"records"`
}

// IndexedRecord is one record, by every name an answer could call it.
type IndexedRecord struct {
	// ID is the record's declared id, where it declares one.
	ID string `json:"id,omitempty"`
	// Path is the checkout-relative path, which every record has.
	Path string `json:"path"`
	// Title is the record's own heading, which links only when it is unique.
	Title string `json:"title,omitempty"`
	Kind  string `json:"kind,omitempty"`
}

// The index's bounds. A workspace larger than these links what it can and
// nothing it cannot: an unlinked id is text, which is what it was before.
const (
	maxIndexedGoals   = 2000
	maxIndexedRecords = 2000
)

// IndexOf reads the two lists from the same readers the pages use. A reader
// this build does not have contributes nothing rather than a refusal: the
// index is what makes a link possible, never what makes an answer possible.
func IndexOf(facts Facts) Index {
	index := Index{Goals: []string{}, Records: []IndexedRecord{}}
	if facts.Observe != nil {
		observed := facts.Observe()
		if observed.State == snapshot.StateRead && observed.Tree != nil {
			board := backlog.Project(observed.Tree, observed.Horizon, observed.Admission)
			for _, row := range append(append([]backlog.Row{}, board.Rows...), board.Closed...) {
				if len(index.Goals) >= maxIndexedGoals {
					break
				}
				index.Goals = append(index.Goals, row.ID)
			}
		}
	}
	if facts.Project != nil {
		if pane, err := facts.Project(); err == nil {
			for _, record := range pane.Records {
				if len(index.Records) >= maxIndexedRecords {
					break
				}
				index.Records = append(index.Records, IndexedRecord{
					ID: record.ID, Path: record.Path, Title: record.Title, Kind: record.Kind,
				})
			}
		}
	}
	return index
}
