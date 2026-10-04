package plain

// landing push: HEAD goes on main only when results.jsonl says green for
// exactly HEAD's tree and origin's main is an ancestor of HEAD, leased at
// that main. Nothing else: a hand-in main then contains reads landed.

import (
	"errors"
	"fmt"
	"os/exec"
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
	Old    string `json:"old"`
	Commit string `json:"commit"`
	Tree   string `json:"tree"`
	At     string `json:"at"`
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
func PushChecked(install, checkout string, now time.Time, before func(old, head string) error) (PushOutcome, error) {
	head, tree, err := Head(checkout)
	if err != nil {
		return PushOutcome{}, err
	}
	outcome := PushOutcome{Commit: head, Tree: tree}
	if _, err := Git(checkout, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
		return outcome, fmt.Errorf("fetch origin's main: %w", err)
	}
	old, err := Git(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	if err != nil {
		return outcome, fmt.Errorf("read origin's main: %w", err)
	}
	outcome.Old = old
	onMain, err := IsAncestor(checkout, head, old)
	if err != nil {
		return outcome, err
	}
	if onMain {
		outcome.Commit = old
		return outcome, nil
	}
	if refusal := provenGreen(install, tree); refusal != nil {
		return outcome, refusal
	}
	forward, err := IsAncestor(checkout, old, head)
	if err != nil {
		return outcome, err
	}
	if !forward {
		return outcome, &Refusal{Code: CodeNotFastForward,
			Reason: "HEAD does not contain origin's main " + Short(old) + ", so pushing it would rewrite main; nothing was pushed",
			Next:   "merge origin/main into the lane checkout, prove it and push again"}
	}
	if before != nil {
		if err := before(old, head); err != nil {
			return outcome, err
		}
	}
	if _, err := Git(checkout, "push", "--quiet", "--force-with-lease=refs/heads/main:"+old, "origin", head+":refs/heads/main"); err != nil {
		return outcome, fmt.Errorf("push %s to main: %w", Short(head), err)
	}
	outcome.Changed = true
	return outcome, withLock(install, func() error {
		return appendLine(pushesPath(install), Pushed{Old: old, Commit: head, Tree: tree, At: now.UTC().Format(time.RFC3339)})
	})
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
		return &Refusal{Code: CodeRed, Reason: "HEAD's tree " + Short(tree) + " was proven " + result.Result + ", not green, so nothing was pushed", Next: "landing return"}
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
