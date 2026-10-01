package batch

import (
	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// The facts a batch's start is decided from (batch-lane design D14, R22):
// what every seat of this host is working on and how close it is, read from
// the host board's cards, and what a separate proof costs, from the lane's
// own retained records.

// BoardPicture is what a pipeline source reads from the host: the cards it
// believes and the ones it cannot, classified by board.Classify and by the
// ledger checks only the source can make. Readable is false, with Reason,
// when the host registry cannot be read: the only case in which the lane
// falls back to its max wait.
type BoardPicture struct {
	Cards    []board.Card
	Unknown  []board.Unknown
	Readable bool
	Reason   string
}
