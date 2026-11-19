package steward

import (
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/config"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// runnerRepairs counts starts for dead or failing runners until a later
// runner completes a tick. All writes share the arm lock.
type runnerRepairs struct {
	SinceSuccess int       `json:"sinceSuccess"`
	Since        time.Time `json:"since"`
}

func runnerRepairsPath(root string) string {
	return filepath.Join(runnerDir(root), "runner-repairs.json")
}

func readRunnerRepairs(root string) (runnerRepairs, error) {
	var count runnerRepairs
	err := readJSON(runnerRepairsPath(root), &count)
	if errors.Is(err, os.ErrNotExist) {
		return runnerRepairs{}, nil
	}
	if err == nil && (count.SinceSuccess < 0 || (count.SinceSuccess > 0 && count.Since.IsZero())) {
		err = errors.New("invalid runner repair count")
	}
	return count, err
}

// RunnerRepairLimit is the number of unsuccessful starts before healing ends.
func RunnerRepairLimit(root string) (int, error) {
	limit := config.MustIntDefault("steward.runner-repair-limit")
	if _, err := os.Stat(filepath.Join(root, "metasystem.conf")); os.IsNotExist(err) {
		return limit, nil
	}
	return boundedConfig(root, "steward.runner-repair-limit", limit, 1)
}

func runnerRepairsState(root string) (count runnerRepairs, limit int, succeeded bool, err error) {
	if count, err = readRunnerRepairs(root); err != nil {
		return
	}
	if limit, err = RunnerRepairLimit(root); err != nil {
		return
	}
	var progress RunnerProgress
	succeeded = count.SinceSuccess > 0 && readJSON(runnerProgressPath(root), &progress) == nil &&
		progress.OKTicks > 0 && progress.PidStartedAt >= count.Since.Unix()
	return
}

func chargeRunnerRepair(root string, at time.Time) error {
	count, err := readRunnerRepairs(root)
	if err != nil {
		return err
	}
	if count.SinceSuccess == 0 {
		count.Since = at
	}
	count.SinceSuccess++
	return writeJSONAtomic(runnerRepairsPath(root), count)
}

// ErrRunnerRepairBusy means a repair or arm must finish before a restart
// can clear its records.
var ErrRunnerRepairBusy = errors.New("a runner repair holds its lock")

// ClearRunnerRepairs starts a person's restart with no pending repair or count.
func ClearRunnerRepairs(root string) error {
	if err := os.MkdirAll(runnerDir(root), 0o755); err != nil {
		return err
	}
	held, err := lock.File(filepath.Join(runnerDir(root), "arm.flock"), 0o644, lock.TryExclusive)
	if lock.Busy(err) {
		return ErrRunnerRepairBusy
	}
	if err != nil {
		return err
	}
	defer held.Release()
	paths := []string{runnerReplacementPath(root), runnerRepairsPath(root)}
	contents := make([][]byte, len(paths))
	// Read both first so an unreadable file leaves both records untouched.
	for i, path := range paths {
		contents[i], err = os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for i, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			if i > 0 && contents[0] != nil {
				_, restoreErr := atomicfile.WriteFile(paths[0], contents[0], 0o644, "")
				return errors.Join(err, restoreErr)
			}
			return err
		}
	}
	return nil
}
