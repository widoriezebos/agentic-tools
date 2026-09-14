package steward

import (
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

const seatHandoffReason = "seatHandoff"

// HandoffBinding ties one launch authorization to an immutable state capture
// and to the exact predecessor whose death may later permit that launch.
type HandoffBinding struct {
	StatePath      string       `json:"statePath"`
	StateDigest    string       `json:"stateDigest"`
	Runtime        string       `json:"runtime"`
	Session        string       `json:"session"`
	MainId         string       `json:"mainId,omitempty"`
	Predecessor    identity.Ref `json:"predecessor"`
	PredecessorTag string       `json:"predecessorTag,omitempty"`
	PredecessorJob string       `json:"predecessorJob,omitempty"`
	RecordedAt     time.Time    `json:"recordedAt"`
}

// HandoffDir is the immutable directory owned by one handoff nonce.
func HandoffDir(stateRoot, nonce string) string {
	root, err := filepath.Abs(stateRoot)
	if err == nil {
		if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
			root = resolved
		}
	}
	return filepath.Join(root, "artifacts", "agents", "context", "handoffs", nonce)
}
