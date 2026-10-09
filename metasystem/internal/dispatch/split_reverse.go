package dispatch

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/obligationstate"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/run"
)

// SplitChildWorkStarted reads all retained starts, independent of budget episodes.
func SplitChildWorkStarted(root, id string) (bool, error) {
	for _, dir := range []string{filepath.Join(root, "artifacts", "agents", "jobs"), run.Dir(root), filepath.Join(root, "artifacts", "agents", "proof-runs", "attempts")} {
		entries, err := os.ReadDir(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		for _, entry := range entries {
			if strings.HasSuffix(entry.Name(), ".exit.json") || !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			record, err := readObject(filepath.Join(dir, entry.Name()))
			if err != nil {
				return false, err
			}
			value, present := record["goalId"]
			if !present {
				return false, fmt.Errorf("work record %s has no goal identity", entry.Name())
			}
			if value == nil {
				continue
			}
			named, ok := value.(string)
			if !ok || named == "" {
				return false, fmt.Errorf("work record %s has an unreadable goal identity", entry.Name())
			}
			if named == id {
				return true, nil
			}
		}
	}
	attempts, err := proofrun.ReadAttempts(root)
	if err != nil {
		return false, err
	}
	for _, attempt := range attempts {
		if attempt.AccountedGoal() == id {
			return true, nil
		}
	}
	states, err := obligationstate.LoadGoal(root, id)
	if err != nil {
		return false, err
	}
	for _, state := range states {
		if len(state.Attempts) != 0 {
			return true, nil
		}
	}
	return false, nil
}
