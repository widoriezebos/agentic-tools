package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/dispatch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
)

var ErrRearmDeferred = errors.New("re-arm deferred")

// OpenJob holds the engine at any nonterminal or unreadable job, regardless of its goal or process.
func OpenJob(root string) (job string, open bool) {
	paths, _ := filepath.Glob(filepath.Join(root, "artifacts", "agents", "jobs", "*.json"))
	for _, path := range paths {
		value, err := readHealthObject(path)
		status, _ := value["status"].(string)
		if err != nil || !dispatch.TerminalStatus(status) {
			return strings.TrimSuffix(filepath.Base(path), ".json"), true
		}
	}
	return "", false
}

// NoteFollowFailure retains one line for each deployed commit and refusal reason.
func NoteFollowFailure(root, commit, reason, command string, now time.Time) error {
	message := "engine follow: " + strings.ReplaceAll(reason, "\n", "; ") + "; to follow by hand: " + command
	return logOrphanSeatRun(root, message+" ["+commit+"]", now)
}

// NoteRearmFailure logs each refresh reason once without holding the runner's pass.
func NoteRearmFailure(root string, reason error, now time.Time) error {
	return logOrphanSeatRun(root, "engine refresh: "+strings.ReplaceAll(reason.Error(), "\n", "; "), now)
}

func deferRearm(root, engine, checkout string, now time.Time) error {
	if err := NoteDeferredRearm(root, engine, checkout, now); err != nil {
		return err
	}
	return fmt.Errorf("%w: checkout at %s, main at %s", ErrRearmDeferred, checkout, engine)
}

// NoteDeferredRearm records the pending engine and logs once for that push.
func NoteDeferredRearm(root, engine, checkout string, now time.Time) error {
	line := fmt.Sprintf("engine %s, checkout %s, re-arm deferred", engine, checkout)
	if _, err := atomicfile.WriteText(filepath.Join(runnerDir(root), "rearm-deferred.txt"), line, root); err != nil {
		return err
	}
	message := fmt.Sprintf("re-arm deferred: checkout at %s, main at %s", checkout, engine)
	if err := logOrphanSeatRun(root, message, now, ", main at "+engine); err != nil {
		return err
	}
	return nil
}

// RearmDeferredLine is the last deferred re-arm observation, until checkout
// and engine match and the arm decision clears it.
func RearmDeferredLine(root string) string {
	data, _ := os.ReadFile(filepath.Join(runnerDir(root), "rearm-deferred.txt"))
	return strings.TrimSpace(string(data))
}

// ClearDeferredRearm clears an observation once checkout and engine match.
func ClearDeferredRearm(root string) error {
	err := os.Remove(filepath.Join(runnerDir(root), "rearm-deferred.txt"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// SeatAtUnitBoundary excludes running steps, critics and unreadable work.
func SeatAtUnitBoundary(root, home string, now time.Time, prober identity.Prober, observed ...goal.ClaimableBudgetedWork) (bool, error) {
	var work goal.ClaimableBudgetedWork
	var err error
	if len(observed) > 0 {
		work = observed[0]
	} else {
		work, err = goal.ReadClaimableBudgetedWorkWith(root, now, (TickConfig{Prober: prober}).withDefaults().Prober)
	}
	if err != nil {
		return false, err
	}
	busy, _, skipped := SeatBusyAt(root, filepath.Join(home, "unit"), work, SeatBusyOptions{Now: now, AtBoundary: true, Probe: func(ref identity.Ref) identity.Liveness {
		return identity.AliveRef((TickConfig{Prober: prober}).withDefaults().Prober, ref)
	}})
	return !busy && skipped == 0, nil
}
