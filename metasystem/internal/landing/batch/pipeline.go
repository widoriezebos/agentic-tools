package batch

import (
	"time"

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

// PipelineSource reads the host board afresh at every decision.
type PipelineSource interface {
	Board(now time.Time) BoardPicture
}

// Underway is one unit on a seat of this host in the pre-join order, with
// the time it is expected to join.
type Underway struct {
	Goal          string
	Seat          string
	Stage         board.Stage
	Round         *board.Round
	Proof         *board.Proof
	Since         time.Time
	LastProgress  time.Time
	ExpectedReady time.Time
	// Basis says whether the current stage's estimate is measured by the
	// lane (measured, n=K) or the compiled default.
	Basis string
}

// NotNear is a unit the lane never waits for, named with why.
type NotNear struct {
	Goal   string
	Seat   string
	Reason string
	// At is the stamp the reason refers to: a stalled unit's last real
	// progress.
	At time.Time
}

// PipelineFacts are everything the start decision reads.
type PipelineFacts struct {
	Underway  []Underway
	Unknown   []NotNear
	ProofCost time.Duration
	// Basis names where the proof cost came from: measured, n=K, or default.
	Basis    string
	Readable bool
	Reason   string
}
