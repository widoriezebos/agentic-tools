package steward

// Progress evidence as durable high-water marks: the checkout HEAD
// object id and the digest of this machine's claim-History opid set.
// Only either progress mark resets age and the dry-revival count. Sample
// times measure elapsed age after excluding evidenced provider waits; writes
// to the steward stores never count as progress.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Marks are the two durable identities progress is judged by.
type Marks struct {
	HeadOid    string `json:"headOid"`
	OpidDigest string `json:"opidDigest"`
}

// Evidence is the persisted state between ticks.
type Evidence struct {
	Marks               Marks              `json:"marks"`
	TicksSinceAdvance   int                `json:"ticksSinceAdvance"`
	DryRevivals         int                `json:"dryRevivals"`
	Abnormal            [2]AbnormalRestart `json:"abnormal,omitempty"`
	CurrentSeat         string             `json:"currentSeat,omitempty"`
	CurrentContinuation string             `json:"currentContinuation,omitempty"`
	AbnormalCount       int                `json:"abnormalCount,omitempty"`

	SampledAt string        `json:"sampledAt,omitempty"`
	Age       time.Duration `json:"age,omitempty"`
	// Degraded counts the ticks in a row whose verdict was degraded.
	Degraded int `json:"degraded,omitempty"`
}

// Observe folds one tick's current marks into the state: an advance
// of either mark resets both the age and the dry-revival count;
// identical marks age by one tick.
func Observe(prev Evidence, cur Marks) Evidence {
	if cur != prev.Marks {
		prev.Marks, prev.TicksSinceAdvance, prev.DryRevivals, prev.Degraded = cur, 0, 0, 0
		prev.SampledAt, prev.Age = "", 0
		return prev
	}
	prev.TicksSinceAdvance++
	return prev
}

// RecordRevival counts a dispatched continuation against the dry cap.
// It deliberately cannot touch the marks: only real progress resets.
func RecordRevival(e Evidence) Evidence {
	e.DryRevivals++
	return e
}

// EvidencePath is the store's location under the steward's own
// artifact directory — excluded from evidence by construction.
func EvidencePath(repoRoot string) string {
	return filepath.Join(repoRoot, "artifacts", "agents", "steward", "highwater.json")
}

// LoadEvidence reads the persisted state; a missing file is the zero
// state (first tick), an unreadable one is an error the tick reports
// as degraded rather than guessing.
func LoadEvidence(path string) (Evidence, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Evidence{}, nil
	}
	if err != nil {
		return Evidence{}, fmt.Errorf("evidence store unreadable: %w", err)
	}
	var e Evidence
	if err := json.Unmarshal(data, &e); err != nil {
		return Evidence{}, fmt.Errorf("evidence store malformed: %w", err)
	}
	return e, nil
}

// SaveEvidence publishes through the shared durability owner, so a tick does
// not claim that its high-water result survived before the filesystem agrees.
func SaveEvidence(repoRoot, path string, e Evidence) error {
	data, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return err
	}
	durable, err := atomicfile.WriteText(path, string(append(data, '\n')), repoRoot)
	if err != nil {
		return err
	}
	if !durable {
		return fmt.Errorf("steward evidence was published with durability unknown")
	}
	return nil
}
