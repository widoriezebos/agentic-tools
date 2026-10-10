package plain

// landing push: HEAD goes on main only when results.jsonl says green for
// exactly HEAD's tree and origin's main is an ancestor of HEAD, leased at
// that main. Nothing else: a hand-in main then contains reads landed.

import (
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

// The refusals of landing push.
const (
	CodeUnproven       = "LANE_PUSH_UNPROVEN"
	CodeRed            = "LANE_PUSH_RED"
	CodeNotFastForward = "LANE_PUSH_NOT_FAST_FORWARD"
)

// Refusal is a push that put nothing on main, with why and what to do.
type Refusal struct {
	Code   string
	Reason string
	Next   string
}

func (r *Refusal) Error() string { return r.Code + ": " + r.Reason }

// Pushed is one line of pushes.jsonl.
type Pushed struct {
	Clock        *Clock    `json:"clock,omitempty"`
	BatchID      string    `json:"batch-id,omitempty"`
	BatchMembers []GoalSHA `json:"batch-members,omitempty"`
	Old          string    `json:"old"`
	Commit       string    `json:"commit"`
	Tree         string    `json:"tree"`
	At           string    `json:"at"`
}

// PushOutcome is what a push did: main moved from Old to Commit
// (Changed), or already held HEAD.
type PushOutcome struct {
	Old     string `json:"old"`
	Commit  string `json:"commit"`
	Tree    string `json:"tree"`
	Changed bool   `json:"changed"`
}

// Push runs landing push in the lane checkout.
func Push(install, checkout string, now time.Time) (PushOutcome, error) {
	return PushChecked(install, checkout, now, nil)
}

// PushChecked runs before with fetched main and HEAD after the push's own
// checks pass. An error from before prevents publication and its push record.
func PushChecked(install, checkout string, now time.Time, before func(old, head string) error, effects ...ProveSeams) (PushOutcome, error) {
	seams := ProveSeams{}
	if len(effects) > 0 {
		seams = effects[0]
	}
	head, tree, err := head(seams.git, checkout)
	if err != nil {
		return PushOutcome{}, err
	}
	outcome := PushOutcome{Commit: head, Tree: tree}
	if err := seams.fetchMain(checkout); err != nil {
		return outcome, err
	}
	old, err := seams.git(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	if err != nil {
		return outcome, fmt.Errorf("read origin's main: %w", err)
	}
	outcome.Old = old
	contains := func(ancestor, commit string) (bool, error) {
		if seams.Git == nil {
			return IsAncestor(checkout, ancestor, commit)
		}
		return checkoutGit(checkout, seams).contains(commit, ancestor)
	}
	onMain, err := contains(head, old)
	if err != nil {
		return outcome, err
	}
	if onMain {
		outcome.Commit = old
		return outcome, withLock(install, func() error {
			batch, err := ReadBatch(install)
			if err != nil {
				return err
			}
			if batch != nil {
				if err := seams.batchLane(batch); err != nil {
					return err
				}
				if batch.ClosureReason == "confirmed push accounts for the selected members" {
					return closeFix(install, "")
				}
				for _, member := range batch.Members {
					onMain, err := batchContains(checkout, old, member.SHA, seams)
					if err != nil || !onMain {
						return err
					}
				}
				old = batch.Base
			} else if push, ok, err := LastPush(install); err != nil {
				return err
			} else if ok {
				old = push.Commit
			}
			operation := seams
			operation.AgentRunning = nil
			idle, err := batchIdle(install, operation)
			if err != nil || !idle || provenGreen(install, tree) != nil {
				return err
			}
			return completePush(install, checkout, batch, old, head, tree, now, seams)
		})
	}
	if refusal := provenGreen(install, tree); refusal != nil {
		return outcome, refusal
	}
	forward, err := contains(old, head)
	if err != nil {
		return outcome, err
	}
	if !forward {
		return outcome, &Refusal{Code: CodeNotFastForward,
			Reason: "HEAD does not contain origin's main " + Short(old) + ", so pushing it would rewrite main; nothing was pushed",
			Next:   "merge origin/main into the lane checkout, prove it and push again"}
	}
	if batch, err := CheckBatch(install, checkout, head, old, false, seams); err != nil {
		return outcome, err
	} else if batch != nil && batch.State != BatchRunning {
		return outcome, batchRefusal("the recorded selection has not been admitted for execution")
	}
	if before != nil {
		if err := before(old, head); err != nil {
			return outcome, err
		}
	}
	err = withLock(install, func() error {
		batch, err := checkBatchLocked(install, checkout, head, old, false, false, seams)
		if err != nil {
			return err
		}
		if batch != nil && batch.State != BatchRunning {
			return batchRefusal("the recorded selection has not been admitted for execution")
		}
		operation := seams
		operation.AgentRunning = nil
		idle, err := batchIdle(install, operation)
		if err != nil {
			return err
		}
		// The landing agent owns this push; only another proof or regeneration
		// is a conflicting operation. Its own launch is expected to be alive.
		if !idle {
			return batchRefusal("a test run or conflict regeneration still owns the batch")
		}
		if refusal := provenGreen(install, tree); refusal != nil {
			return refusal
		}
		if _, err := seams.git(checkout, "push", "--quiet", "--force-with-lease=refs/heads/main:"+old, "origin", head+":refs/heads/main"); err != nil {
			return fmt.Errorf("push %s to main: %w", Short(head), err)
		}
		outcome.Changed = true
		return completePush(install, checkout, batch, old, head, tree, now, seams)
	})
	return outcome, err
}

// completePush finishes a confirmed publication; its commit identifies one record.
func completePush(install, checkout string, batch *Batch, old, head, tree string, now time.Time, seams ProveSeams) error {
	pushes, err := readLines[Pushed](pushesPath(install))
	if err != nil {
		return err
	}
	pushed := Pushed{Old: old, Commit: head, Tree: tree, At: now.UTC().Format(time.RFC3339)}
	clockBatch := batch
	if batch != nil {
		pushed.BatchID = batch.ID
		for _, member := range batch.Members {
			landed, err := batchContains(checkout, head, member.SHA, seams)
			if err != nil {
				return err
			}
			if landed {
				pushed.BatchMembers = append(pushed.BatchMembers, member)
			}
		}
		copy := *batch
		copy.Members = pushed.BatchMembers
		clockBatch = &copy
	}
	pushed.Clock = landingClock(install, checkout, clockBatch, now, seams)
	if !slices.ContainsFunc(pushes, func(p Pushed) bool { return p.Commit == head }) {
		if err := appendLine(pushesPath(install), pushed); err != nil {
			return err
		}
	}
	if batch != nil && batch.ClosureReason != "confirmed push accounts for the selected members" {
		batch.State, batch.ClosureReason = BatchClosed, "confirmed push accounts for the selected members"
		if err := writeBatch(install, batch); err != nil {
			return err
		}
	}
	if err := closeFix(install, ""); err != nil {
		return err
	}
	if err := closeProofLoop(install); err != nil {
		return err
	}
	for _, member := range pushed.BatchMembers {
		if err := closeGoalStopsLocked(install, member.Goal, "push", now, member.SHA); err != nil {
			return err
		}
	}
	return nil
}

func (s ProveSeams) fetchMain(checkout string) error {
	args := []string{"fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"}
	var err error
	if s.FetchCommand != nil || s.Git == nil {
		_, err = boundedFetch(checkout, s.FetchTimeout, s.FetchCommand, s.FetchDeadline, args...)
	} else {
		_, err = s.git(checkout, args...)
	}
	if err != nil {
		return fmt.Errorf("fetch origin's main: %w", err)
	}
	return nil
}

// provenGreen refuses a tree results.jsonl does not hold green.
func provenGreen(install, tree string) *Refusal {
	result, ok, err := ResultFor(install, tree)
	switch {
	case err != nil:
		return &Refusal{Code: CodeUnproven, Reason: "the results of landing prove can't be read (" + err.Error() + "), so nothing was pushed", Next: "landing prove"}
	case !ok:
		return &Refusal{Code: CodeUnproven, Reason: "HEAD's tree " + Short(tree) + " was never proven, so nothing was pushed", Next: "landing prove"}
	case result.Result != Green:
		return &Refusal{Code: CodeRed, Reason: "HEAD's tree " + Short(tree) + " was proven " + result.Result + ", not green, so nothing was pushed", Next: "landing status"}
	}
	return nil
}

// LastPush is the newest push.
func LastPush(install string) (Pushed, bool, error) {
	pushes, err := readLines[Pushed](pushesPath(install))
	if err != nil || len(pushes) == 0 {
		return Pushed{}, false, err
	}
	return pushes[len(pushes)-1], true, nil
}

// ContainedIn says, in the checkout at dir, whether main contains a sha. A
// commit the checkout does not have is not in main, which the checkout
// holds whole; a main it does not have contains nothing it can tell.
func ContainedIn(dir, main string) func(sha string) (bool, error) {
	return func(sha string) (bool, error) {
		for _, commit := range []string{main, sha} {
			if _, err := Git(dir, "cat-file", "-e", commit+"^{commit}"); err != nil {
				return false, nil
			}
		}
		return IsAncestor(dir, sha, main)
	}
}

// IsAncestor says whether ancestor is commit or one of its ancestors.
func IsAncestor(dir, ancestor, commit string) (bool, error) {
	command := exec.Command("git", "-C", dir, "merge-base", "--is-ancestor", ancestor, commit)
	command.Env = gittree.ScrubbedEnviron()
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, fmt.Errorf("whether %s contains %s can't be read: %w", Short(commit), Short(ancestor), err)
}
