package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
)

// Fix is one bounded repair of a batch, retained beside its brief.
type Fix struct {
	Attempt string   `json:"attempt"`
	Round   int      `json:"round"`
	Parent  string   `json:"parent"`
	Verdict string   `json:"verdict,omitempty"`
	Goal    string   `json:"goal"`
	Units   []string `json:"units"`
	Job     string   `json:"job"`
	Read    string   `json:"read"`
	Commit  string   `json:"commit"`
	State   string   `json:"state"`
}

func readFix(install string, goals ...string) (*Fix, error) {
	return readFixRecords(install, false, goals...)
}

// readFixRecords may close stale attempts only while the caller holds the lane lock.
func readFixRecords(install string, closeStale bool, goals ...string) (*Fix, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	running, recorded, _, readErr := ReadRunning(install, ProveSeams{})
	recorded = recorded && readErr == nil
	var active *Fix
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fix Fix
		if err := json.Unmarshal(data, &fix); err != nil {
			if len(goals) > 0 && fix.Goal != goals[0] {
				continue
			}
			return nil, err
		}
		if len(goals) > 0 && goals[0] != "" && fix.Goal != goals[0] {
			continue
		}
		if fix.State != "running" && fix.State != "reviewing" && fix.State != "building" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.Job == "" || fix.State != "building" && fix.Commit == "" {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if fix.Attempt == "" {
			fix.Attempt = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if recorded && running.Attempt != fix.Attempt {
			if closeStale {
				fix.State = "closed"
				if err := WriteFix(install, fix); err != nil {
					return nil, err
				}
			}
			continue
		}
		if active != nil {
			return nil, errors.New("more than one lane fix is running")
		}
		active = &fix
	}
	return active, nil
}

func fixHeadline(fix *Fix) string {
	commit := fix.Commit
	if commit == "" {
		commit = fix.Parent
	}
	round := max(1, fix.Round)
	return fmt.Sprintf("Fixing %s of %s on %s (fix round %d)", strings.Join(fix.Units, ", "), fix.Goal, Short(commit), round)
}

// WriteFix retains a fix's lifecycle beside its attempt's brief.
func WriteFix(install string, fix Fix) error {
	data, err := json.Marshal(fix)
	if err != nil {
		return err
	}
	dir := filepath.Join(Dir(install), "fixes")
	_, err = atomicfile.WriteText(filepath.Join(dir, fix.Attempt+".json"), string(data)+"\n", dir)
	return err
}

func ActiveFix(install string) (*Fix, error) { return readFix(install) }

func FixForAttempt(install, attempt string) (*Fix, error) {
	data, err := os.ReadFile(filepath.Join(Dir(install), "fixes", attempt+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var fix Fix
	return &fix, json.Unmarshal(data, &fix)
}
func currentFix(install string, seams ProveSeams) (*Fix, error) {
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return nil, err
	}
	if !recorded || running.Checkpoint {
		result, present, err := LastResult(install)
		if running.Gate {
			result, present, err = LastGate(install)
		}
		if err != nil || !present {
			return nil, err
		}
		if recorded && (result.Result != Red || result.Attempt != running.Attempt || result.Commit != running.Commit) || !recorded && result.Result != Green {
			return nil, nil
		}
		running.Attempt, running.Commit = result.Attempt, result.Commit
	}
	fix, err := FixForAttempt(install, running.Attempt)
	if err != nil || fix == nil || fix.State != "reviewing" && fix.State != "running" || fix.Job == "" || len(fix.Units) == 0 || fix.Commit == "" || fix.Round != 1 || fix.Parent == "" || fix.Attempt != running.Attempt {
		return nil, err
	}
	if running.Commit != fix.Parent && running.Commit != fix.Commit {
		batch, err := ReadBatch(install)
		if err != nil || batch == nil || batch.Lane.Root == "" {
			return nil, err
		}
		contained, err := fixInFirstParent(batch.Lane.Root, running.Commit, fix.Commit, seams)
		if err != nil || !contained {
			return nil, err
		}
	}
	return fix, nil
}

// A main refresh keeps the repair on the lane's first-parent chain.
func fixInFirstParent(checkout, commit, fix string, seams ProveSeams) (bool, error) {
	if commit == fix {
		return true, nil
	}
	chain, err := seams.git(checkout, "rev-list", "--first-parent", commit)
	return slices.Contains(strings.Fields(chain), fix), err
}

func advanceFix(install, checkout string, running Running, seams ProveSeams) error {
	previous, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil {
		return err
	}
	if !recorded {
		result, present, err := LastResult(install)
		if err != nil || !present || result.Result != Green {
			return err
		}
		previous = Running{Attempt: result.Attempt, Commit: result.Commit, BatchID: result.BatchID, BatchMembers: result.BatchMembers}
	}
	if previous.Attempt == running.Attempt {
		return err
	}
	fix, err := FixForAttempt(install, previous.Attempt)
	if err != nil || fix == nil || fix.State == "closed" {
		return err
	}
	follows := false
	if !running.Trunk && fix.Commit != "" && previous.BatchID == running.BatchID && slices.Equal(previous.BatchMembers, running.BatchMembers) {
		follows, err = fixInFirstParent(checkout, running.Commit, fix.Commit, seams)
		if err != nil {
			return err
		}
	}
	fix.State = "closed"
	err = WriteFix(install, *fix)
	if err == nil && follows {
		fix.Attempt, fix.State = running.Attempt, "reviewing"
		err = WriteFix(install, *fix)
	}
	return err
}

func closeFix(install, goal string) error {
	fix, err := readFixRecords(install, true, goal)
	if err != nil {
		return err
	}
	if fix != nil {
		fix.State = "closed"
		if err := WriteFix(install, *fix); err != nil {
			return err
		}
	}
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil || !recorded || !running.Checkpoint || goal != "" && !slices.ContainsFunc(running.BatchMembers, func(m GoalSHA) bool { return m.Goal == goal }) {
		return err
	}
	return os.Remove(runningPath(install))
}
