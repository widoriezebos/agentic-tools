package launch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"golang.org/x/sys/unix"

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

// treeLocked serializes tree transitions and command mutations.
func (runner *UnitRunner) treeLocked(worktree string, act func(string, *treeReservation) error) error {
	if runner.tree != nil && runner.tree.path != "" {
		return act(runner.tree.path, runner.tree.owner)
	}
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
		if lock.Busy(err) {
			return coded("UNIT_RUN_BUSY", "tree="+real, errors.New("another command is changing this worktree; repeat the same command when it finishes"))
		}
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
	if runner.tree != nil {
		runner.tree.path, runner.tree.owner = path, &owner
		runner.tree.files = append(runner.tree.files, held.File())
	}
	return act(path, &owner)
}

// GateTree checks custody before a writer enters the worktree. The optional
// transition runs while ownership is locked.
func (runner *UnitRunner) GateTree(worktree, run string, act func(string, *treeReservation) error) error {
	return runner.treeLocked(worktree, func(path string, owner *treeReservation) error {
		if owner.Run != "" && owner.Run != run && (run == "" || owner.Run != runner.mutation) {
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
		if owner.Run != runner.mutation || runner.mutation == "" {
			owner.Run, owner.Round, owner.Phase = record.ID, len(record.Rounds), record.State
		}
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
	if record.Mutation != nil {
		ended = identity.LiveRef(runner.Manager.Prober, *record.Mutation) == identity.Dead
		childrenEnded := runner.treeChildrenEnded(owner.Children)
		return (record.State != "running" || ended) && childrenEnded, ended && childrenEnded
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
	if record.Mutation != nil && record.State == "running" && identity.LiveRef(runner.Manager.Prober, *record.Mutation) != identity.Dead {
		return record, &TreeWaitingError{Run: id}
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

// A command takes the tree before its named unit and run locks. Saves reuse
// that boundary; waits for child results release the locks and validate the run.
type treeCommand struct {
	path  string
	owner *treeReservation
	files []*os.File
}

func treeCall[T any](runner *UnitRunner, worktree string, act func(*UnitRunner) (T, error)) (T, error) {
	bound := *runner
	bound.tree = &treeCommand{}
	var result T
	err := bound.treeLocked(worktree, func(string, *treeReservation) error {
		var err error
		result, err = act(&bound)
		return err
	})
	return result, err
}

// CommandWait releases command locks while another actor works, then validates
// the retained run before this command may write again.
func (runner *UnitRunner) CommandWait(act func() error) error {
	if runner.tree == nil {
		return act()
	}
	files := []*os.File{}
	retained := map[string][]byte{}
	for _, file := range runner.tree.files {
		if _, err := file.Stat(); err != nil {
			continue
		}
		files = append(files, file)
		if idPattern.MatchString(filepath.Base(filepath.Dir(file.Name()))) {
			path := filepath.Join(filepath.Dir(file.Name()), "run.json")
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			retained[path] = data
		}
	}
	for i := len(files) - 1; i >= 0; i-- {
		_ = unix.Flock(int(files[i].Fd()), unix.LOCK_UN)
	}
	ownerRun := runner.tree.owner.Run
	waitErr := act()
	for i, file := range files {
		mode := unix.LOCK_EX | unix.LOCK_NB
		if i == 0 {
			mode = unix.LOCK_EX
		}
		if err := unix.Flock(int(file.Fd()), mode); err != nil {
			return coded("UNIT_WAIT_RETRY", "", fmt.Errorf("the command locks could not be reacquired; repeat the same command to follow the current run: %w", err))
		}
	}
	if err := readJSONFile(runner.tree.path, runner.tree.owner); err != nil {
		return coded("UNIT_WAIT_RETRY", "", fmt.Errorf("the worktree ownership changed during the wait; repeat the same command to follow the current run: %w", err))
	}
	if runner.tree.owner.Run != ownerRun {
		return coded("UNIT_WAIT_RETRY", "", errors.New("the worktree owner changed during the wait; repeat the same command to follow its current state"))
	}
	for path, data := range retained {
		current, err := os.ReadFile(path)
		if err != nil {
			return coded("UNIT_WAIT_RETRY", "", fmt.Errorf("the run cannot be reread after the wait; repeat the same command: %w", err))
		}
		if !bytes.Equal(data, current) {
			return coded("UNIT_WAIT_RETRY", "", errors.New("the run changed during the wait; repeat the command to follow its current state"))
		}
	}
	return waitErr
}

func (runner *UnitRunner) waitLaunch(id string, timeout time.Duration) (record Record, terminal bool, err error) {
	err = runner.CommandWait(func() error {
		var waitErr error
		record, terminal, waitErr = runner.Manager.Wait(id, timeout)
		return waitErr
	})
	return
}

// ReserveMutation gives a branch operation real run custody across remote
// calls. Its wait observes that command; it never starts another mutation.
func (runner *UnitRunner) ReserveMutation(worktree, goal, operation string) (func() error, error) {
	self, err := runner.Manager.Processes.SelfRef()
	if err != nil {
		return nil, err
	}
	id, err := newID(runner.Manager.Now())
	if err != nil {
		return nil, err
	}
	record := UnitRunRecord{ID: id, Worktree: worktree, Goal: goal, Unit: operation, State: "running", Mutation: &self}
	err = runner.GateTree(worktree, "", func(path string, owner *treeReservation) error {
		if err := writeUnitJSON(filepath.Join(runner.runDir(id), "run.json"), record, runner.root()); err != nil {
			return err
		}
		owner.Run, owner.Phase = id, operation
		return writeUnitJSON(path, *owner, runner.root())
	})
	if err != nil {
		return nil, err
	}
	runner.mutation = id
	return func() error {
		err := runner.treeLocked(worktree, func(_ string, owner *treeReservation) error {
			if owner.Run != id {
				return errors.New("the branch operation no longer owns the worktree")
			}
			record.State = "completed"
			return writeUnitJSON(filepath.Join(runner.runDir(id), "run.json"), record, runner.root())
		})
		return err
	}, nil
}

// MutationFinished observes custody without advancing the branch operation.
func (runner *UnitRunner) MutationFinished(record UnitRunRecord) (bool, error) {
	finished := false
	err := runner.treeLocked(record.Worktree, func(_ string, owner *treeReservation) error {
		if owner.Run == record.ID {
			finished, _ = runner.treeQuiescent(*owner)
		} else {
			finished = record.State != "running" || identity.LiveRef(runner.Manager.Prober, *record.Mutation) == identity.Dead
		}
		return nil
	})
	return finished, err
}
