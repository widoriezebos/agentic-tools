package steward

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

var ErrRearmDeferred = errors.New("re-arm deferred")

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
func SeatAtUnitBoundary(root, home string, now time.Time) (bool, error) {
	work, err := goal.ReadClaimableBudgetedWork(root, now)
	if err != nil {
		return false, err
	}
	busy, _, skipped := SeatBusyAt(root, filepath.Join(home, "unit"), work, SeatBusyOptions{Now: now, AtBoundary: true})
	return !busy && skipped == 0, nil
}
