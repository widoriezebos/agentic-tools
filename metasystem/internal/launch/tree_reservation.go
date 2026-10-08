package launch

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/atomicfile"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/lock"
)

// TreeWaitingError names the writer whose custody must end before another starts.
type TreeWaitingError struct {
	Run       string
	CanCancel bool
}

func (e *TreeWaitingError) Error() string {
	message := "the worktree belongs to run " + e.Run + "; wait with metasystem work wait run:" + e.Run
	if e.CanCancel {
		message += "; no child is live: a person can release it with metasystem work stop run:" + e.Run
	}
	return message
}

type treeReservation struct {
	Worktree string       `json:"worktree"`
	Run      string       `json:"run"`
	Round    int          `json:"round"`
	Phase    string       `json:"phase"`
	Subject  *UnitSubject `json:"subject,omitempty"`
	Children []string     `json:"children"`
}

// treeLocked serializes only ownership changes, never child or remote waits.
func (runner *UnitRunner) treeLocked(worktree string, act func(string, *treeReservation) error) error {
	git := runner.Git
	if git == nil {
		git = OSGitRunner{}
	}
	root, err := git.Run(worktree, nil, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	real, err := filepath.EvalSymlinks(strings.TrimSpace(string(root)))
	if err != nil {
		return err
	}
	path := filepath.Join(runner.root(), ".trees", digestHex([]byte(real))+".json")
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	held, err := lock.File(path+".lock", 0600, lock.TryExclusive)
	if err != nil {
		return err
	}
	defer held.Release()
	var owner treeReservation
	err = readJSONFile(path, &owner)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err == nil && (owner.Worktree != real || !idPattern.MatchString(owner.Run)) {
		return fmt.Errorf("the worktree ownership record is damaged: %s", path)
	}
	owner.Worktree = real
	return act(path, &owner)
}

// GateTree checks custody before a writer enters the worktree. The optional
// transition runs while ownership is locked.
func (runner *UnitRunner) GateTree(worktree, run string, act func(string, *treeReservation) error) error {
	return runner.treeLocked(worktree, func(path string, owner *treeReservation) error {
		if owner.Run != "" && owner.Run != run {
			if released, ended := runner.treeQuiescent(*owner); !released {
				return &TreeWaitingError{Run: owner.Run, CanCancel: ended}
			}
			if err := os.Remove(path); err != nil {
				return err
			}
			*owner = treeReservation{Worktree: owner.Worktree}
		}
		if act != nil {
			return act(path, owner)
		}
		return nil
	})
}

func writeUnitJSON(path string, record any, root string) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	_, err = atomicfile.WriteText(path, string(data)+"\n", root)
	return err
}

func (runner *UnitRunner) reserveTree(record UnitRunRecord) error {
	return runner.GateTree(record.Worktree, record.ID, func(path string, owner *treeReservation) error {
		owner.Run, owner.Round, owner.Phase = record.ID, len(record.Rounds), record.State
		owner.Children = unitChildren(record, owner.Children)
		for _, subject := range record.Subjects {
			if subject.Round == owner.Round {
				copy := subject
				owner.Subject = &copy
			}
		}
		return writeUnitJSON(path, *owner, runner.root())
	})
}

func unitChildren(record UnitRunRecord, children []string) []string {
	for _, round := range record.Rounds {
		for _, step := range round.Steps {
			for _, id := range append(append([]string{}, step.LaunchIDs...), step.LaunchID) {
				if id != "" && !slices.Contains(children, id) {
					children = append(children, id)
				}
			}
		}
	}
	return children
}

// A missing launch record under the run lock means the child never started;
// every launch is created under that same lock.
func (runner *UnitRunner) treeChildrenEnded(children []string) bool {
	for _, id := range children {
		if runner.Manager == nil {
			return false
		}
		child, err := runner.Manager.Store.Read(id)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !child.State.Terminal() || !runner.Manager.provenDead(child) {
			return false
		}
	}
	return true
}

func (runner *UnitRunner) treeQuiescent(owner treeReservation) (released, ended bool) {
	held, err := runner.lock(owner.Run)
	if err != nil {
		return false, false
	}
	defer releaseUnitLock(held)
	record, err := runner.read(owner.Run)
	if err != nil {
		return false, false
	}
	closed := record.State == "cancelled"
	if len(record.Rounds) > 0 {
		round := record.Rounds[len(record.Rounds)-1]
		closed = closed || round.Transferred
		for _, subject := range record.Subjects {
			closed = closed || subject.Round == round.Number && subject.Published != "" && round.Stop != nil && round.Stop.Decision == "close"
		}
	}
	ended = runner.treeChildrenEnded(owner.Children)
	return closed && ended, ended
}

// CancelRun records the person's stop before signalling, so no new step can
// start. Ownership remains until every retained child's exact custody ends.
func (runner *UnitRunner) CancelRun(id string) (UnitRunRecord, error) {
	record, err := runner.read(id)
	if err != nil {
		return record, err
	}
	var children []string
	err = runner.treeLocked(record.Worktree, func(_ string, owner *treeReservation) error {
		held, err := runner.lock(id)
		if err != nil {
			return err
		}
		defer releaseUnitLock(held)
		record, err = runner.read(id)
		if err != nil {
			return err
		}
		if owner.Run != "" && owner.Run != id && record.State != "cancelled" {
			return errors.New("another run owns the worktree; this run cannot release it")
		}
		if owner.Run == id {
			children = append([]string(nil), owner.Children...)
		}
		children = unitChildren(record, children)
		record.State = "cancelled"
		if err := writeUnitJSON(filepath.Join(runner.runDir(id), "run.json"), record, runner.root()); err != nil {
			return err
		}
		children = slices.DeleteFunc(children, func(child string) bool {
			if runner.Manager == nil {
				return false
			}
			_, err := runner.Manager.Store.Read(child)
			return errors.Is(err, os.ErrNotExist)
		})
		return nil
	})
	if err != nil {
		return record, err
	}
	// Cancellation may wait for processes; no ownership or run lock is held.
	for _, child := range children {
		if runner.Manager == nil {
			return record, errors.New("unit launch manager is unavailable")
		}
		if _, err := runner.Manager.Cancel(child); err != nil {
			return record, err
		}
	}
	if !runner.treeChildrenEnded(children) {
		return record, errors.New("the run's children are not proven dead; its worktree remains reserved")
	}
	err = runner.treeLocked(record.Worktree, func(path string, owner *treeReservation) error {
		if owner.Run != id {
			return nil
		}
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	})
	return record, err
}

func (runner *UnitRunner) treeMoved(record *UnitRunRecord, round *UnitRound) (UnitResult, error) {
	round.Cause = "environment"
	for i := range round.Steps {
		step := &round.Steps[i]
		if step.State == StepRunning || i == len(round.Steps)-1 {
			step.State, step.Cause, step.Reason = StepFailed, "environment", "the worktree bytes changed across this step"
			break
		}
	}
	return runner.finish(record, round, "proof-wrote")
}
