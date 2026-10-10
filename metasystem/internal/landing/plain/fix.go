package plain

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/conflict"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/testpolicy"
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

	Paths     []conflict.Path        `json:"paths,omitempty"`
	Tip       string                 `json:"tip,omitempty"`
	Reason    string                 `json:"reason,omitempty"`
	Brief     string                 `json:"brief,omitempty"`
	Generated []testpolicy.Generated `json:"generated,omitempty"`
}

func ReadFix(install string) (*Fix, error) { return readFix(install, false) }

func readFix(install string, includeClosed bool, goals ...string) (*Fix, error) {
	return readFixRecords(install, false, includeClosed, false, goals...)
}

// readFixRecords may close stale attempts only while the caller holds the lane lock.
func readFixRecords(install string, closeStale, includeClosed, mergeOnly bool, goals ...string) (*Fix, error) {
	paths, err := filepath.Glob(filepath.Join(Dir(install), "fixes", "*.json"))
	if err != nil || len(paths) == 0 {
		return nil, err
	}
	running, recorded, _, readErr := ReadRunning(install, ProveSeams{})
	recorded = recorded && readErr == nil
	var active, latest *Fix
	var newest time.Time
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
		if mergeOnly && fix.Tip == "" && fix.State != "resolving" && fix.State != "resolved" {
			continue
		}
		if includeClosed {
			info, err := os.Stat(path)
			if err != nil {
				return nil, err
			}
			if latest == nil || info.ModTime().After(newest) {
				latest, newest = &fix, info.ModTime()
			}
		}
		if fix.State != "running" && fix.State != "reviewing" && fix.State != "building" && fix.State != "resolving" && fix.State != "resolved" {
			continue
		}
		if fix.Goal == "" || len(fix.Units) == 0 || fix.State != "resolving" && fix.Job == "" || fix.State != "building" && fix.Commit == "" || fix.State == "resolving" && (fix.Tip == "" || len(fix.Paths) == 0) {
			return nil, fmt.Errorf("incomplete lane fix in %s", path)
		}
		if fix.Attempt == "" {
			fix.Attempt = strings.TrimSuffix(filepath.Base(path), ".json")
		}
		if recorded && fix.Tip == "" && running.Attempt != fix.Attempt {
			if closeStale {
				fix.State = "closed"
				if err := WriteFix(install, &fix); err != nil {
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
	if active == nil && latest != nil && latest.State == "abandoned" {
		return latest, nil
	}
	return active, nil
}

func fixHeadline(fix *Fix) string {
	if fix.State == "resolved" {
		return fmt.Sprintf("Resolved merge of %s, read pending: %s", fix.Goal, fix.Reason)
	}
	if fix.State == "resolving" {
		var paths []string
		for _, path := range fix.Paths {
			paths = append(paths, path.Path)
		}
		return fmt.Sprintf("Resolving %d conflicts of %s (%s)", len(paths), fix.Goal, strings.Join(paths, ", "))
	}
	commit := fix.Commit
	if commit == "" {
		commit = fix.Parent
	}
	round := max(1, fix.Round)
	return fmt.Sprintf("Fixing %s of %s on %s (fix round %d)", strings.Join(fix.Units, ", "), fix.Goal, Short(commit), round)
}

// WriteFix retains a fix's lifecycle beside its attempt's brief.
func WriteFix(install string, fix *Fix) error {
	data, err := json.Marshal(fix)
	if err != nil {
		return err
	}
	dir := filepath.Join(Dir(install), "fixes")
	_, err = atomicfile.WriteText(filepath.Join(dir, fix.Attempt+".json"), string(data)+"\n", dir)
	return err
}

func ActiveFix(install string) (*Fix, error) { return ReadFix(install) }

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
	err = WriteFix(install, fix)
	if err == nil && follows {
		fix.Attempt, fix.State = running.Attempt, "reviewing"
		err = WriteFix(install, fix)
	}
	return err
}

func closeFix(install, goal string) error {
	fix, err := readFixRecords(install, true, false, false, goal)
	if err != nil {
		return err
	}
	if fix != nil {
		fix.State = "closed"
		if err := WriteFix(install, fix); err != nil {
			return err
		}
	}
	running, recorded, _, err := ReadRunning(install, ProveSeams{})
	if err != nil || !recorded || !running.Checkpoint || goal != "" && !slices.ContainsFunc(running.BatchMembers, func(m GoalSHA) bool { return m.Goal == goal }) {
		return err
	}
	return os.Remove(runningPath(install))
}

// RefreshMerge closes a resolution whose pending merge no longer exists.
func RefreshMerge(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	return withLock(install, func() error { return refreshMergeLocked(install, checkout, fix, seams) })
}

func refreshMergeLocked(install, checkout string, fix *Fix, seams ProveSeams) error {
	if fix == nil || fix.State != "resolving" {
		return nil
	}
	// Status can wait behind a commit; the current record owns the transition.
	data, err := os.ReadFile(filepath.Join(Dir(install), "fixes", fix.Attempt+".json"))
	if err != nil {
		return err
	}
	var current Fix
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	*fix = current
	if fix.State != "resolving" {
		return nil
	}
	tip, err := seams.git(checkout, "rev-parse", "--verify", "MERGE_HEAD^{commit}")
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != 128 {
			return err
		}
		fix.Reason = "the pending merge is gone; its resolution was abandoned"
	} else if tip != fix.Tip {
		fix.Reason = "the pending merge changed; its previous resolution was abandoned"
	} else {
		return nil
	}
	fix.State = "abandoned"
	return WriteFix(install, fix)
}

// RecordedFixRed binds an ordinary repair to the failed proof's checkpoint.
func RecordedFixRed(install string, running Running) bool {
	mode := ProveSeams{Gate: running.Gate}
	results, err := readLines[Result](mode.resultsPath(install))
	if err != nil {
		return false
	}
	for _, result := range slices.Backward(results) {
		if result.Commit == running.Commit && result.Tree == running.Tree && result.Attempt == running.Attempt && result.BatchID == running.BatchID && slices.Equal(result.BatchMembers, running.BatchMembers) {
			return result.Result == Red
		}
	}
	return false
}

func proofFixReady(install string, seams ProveSeams) error {
	if seams.Trunk {
		return nil
	}
	fix, err := readFixRecords(install, false, false, true)
	if err != nil {
		return err
	}
	if fix != nil && (fix.State == "resolving" || fix.State == "resolved" || fix.State == "reviewing" && fix.Tip != "") {
		message := fmt.Sprintf("finish the resolution and its read for %s before landing prove (repair is %s)", fix.Goal, fix.State)
		if fix.Reason != "" {
			message += ": " + fix.Reason
		}
		return errors.New(message)
	}
	return nil
}
