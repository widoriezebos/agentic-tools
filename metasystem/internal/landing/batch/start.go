package batch

import (
	"fmt"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/board"
)

// Waited is one unit a batch waits for.
type Waited struct {
	Goal       string       `json:"goal"`
	Seat       string       `json:"seat"`
	Stage      board.Stage  `json:"stage"`
	Round      *board.Round `json:"round,omitempty"`
	Proof      *board.Proof `json:"proof,omitempty"`
	ExpectedAt time.Time    `json:"expectedAt"`
}

// WaitState is why an open batch has not started: the units it waits for,
// or the fallback's max wait when the board cannot be read. Reason is the
// identity of the wait (who, where, at what stage); a decision with the
// same reason writes nothing.
type WaitState struct {
	Reason    string        `json:"reason"`
	For       []Waited      `json:"for,omitempty"`
	ProofCost time.Duration `json:"proofCost"`
	Since     time.Time     `json:"since"`
	Basis     string        `json:"basis"`
	// Fallback names the unreadable board's cause when the timer decides.
	Fallback string    `json:"fallback,omitempty"`
	Until    time.Time `json:"until,omitempty"`
}

// Decision is one start decision: start now with a reason, or wait.
type Decision struct {
	Start  bool
	Window string
	Reason string
	Wait   *WaitState
}

func minutes(d time.Duration) string {
	return fmt.Sprintf("%d min", max(int(d.Round(time.Minute)/time.Minute), 0))
}

func localClock(at time.Time, location *time.Location) string {
	if location == nil {
		location = time.Local
	}
	return at.In(location).Format("15:04")
}
