package lane

// The proof hold (moving main cancels a running lane proof, 2026-10-01): a
// restart of the lane checkout's engine runs the stop transition, which
// stops every proof the checkout runs and records it cancelled. While a
// proof charged to the lane runs, the lane checkout's engine is therefore
// not restarted; it is restarted once the proof has ended. Seats are never
// held by it.
//
// RunningProof is the one place that says whether a lane proof runs, so a
// later signal (a detached landing prove's own running record) replaces
// only its body.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

// ProofHold is the lane proof that runs: its attempt and when it started.
type ProofHold struct {
	Attempt   string
	StartedAt string
}

// RunningProof says whether a proof charged to the registered lane runs
// while self is that lane's checkout or installation. The signal is the
// proof attempt record in the lane's installation: no terminal, no
// cancellation asked, and a launcher that is not gone. A checkout that is
// not the lane's, no lane, or a lane record that cannot be read holds
// nothing, so seats are never held. An attempt record of the lane that
// cannot be read is an error: unknown is never a go.
func RunningProof(home, self string, prober identity.Prober) (ProofHold, bool, error) {
	record, ok, err := Read(home)
	if err != nil || !ok || !ownsLane(self, record) {
		return ProofHold{}, false, nil
	}
	layout, err := record.Layout()
	if err != nil {
		return ProofHold{}, false, nil
	}
	account := AccountID(string(layout.Checkout))
	probe, err := proofrun.AttemptPath(string(layout.Install), "probe")
	if err != nil {
		return ProofHold{}, false, err
	}
	paths, err := filepath.Glob(filepath.Join(filepath.Dir(probe), "*.json"))
	if err != nil {
		return ProofHold{}, false, err
	}
	sort.Strings(paths)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return ProofHold{}, false, fmt.Errorf("the landing lane's test run record %s can't be read: %w", path, err)
		}
		var attempt proofrun.Attempt
		if err := json.Unmarshal(data, &attempt); err != nil {
			return ProofHold{}, false, fmt.Errorf("the landing lane's test run record %s can't be read: %w", path, err)
		}
		if attempt.Terminal != nil || attempt.CancellationIntent != "" || attempt.AccountedGoal() != account {
			continue
		}
		if identity.AliveRef(prober, attempt.Launcher.Ref()) == identity.Dead {
			continue
		}
		id := attempt.AttemptID
		if id == "" {
			id = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		return ProofHold{Attempt: id, StartedAt: attempt.StartedAt}, true, nil
	}
	return ProofHold{}, false, nil
}
