package plain

// landing push: HEAD goes on main only when results.jsonl says green for
// exactly HEAD's tree and origin's main is an ancestor of HEAD, leased at
// that main. Then every waiting hand-in main contains is settled. The
// settlement runs on every push, also when there is nothing new to push or
// the push is refused, so a crash between a push and its settlement is
// finished by the next push.

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
// (Changed), or already held HEAD; Settled are the hand-ins it settled.
type PushOutcome struct {
	Old     string  `json:"old"`
	Commit  string  `json:"commit"`
	Tree    string  `json:"tree"`
	Changed bool    `json:"changed"`
	Settled []Entry `json:"settled"`
}

// PushSeams are a push's effects.
type PushSeams struct {
	Now func() time.Time
	// Done concludes a settled hand-in's goal; it returns a note kept with
	// the landed line.
	Done func(entry Entry, main string) (string, error)
}

// Push runs landing push in the lane checkout.
func Push(install, checkout string, seams PushSeams) (PushOutcome, error) {
	now := func() time.Time { return time.Now().UTC() }
	if seams.Now != nil {
		now = seams.Now
	}
	head, tree, err := Head(checkout)
	if err != nil {
		return PushOutcome{}, err
	}
	outcome := PushOutcome{Commit: head, Tree: tree, Settled: []Entry{}}
	if _, err := Git(checkout, "fetch", "--quiet", "origin", "+refs/heads/main:refs/remotes/origin/main"); err != nil {
		return outcome, fmt.Errorf("fetch origin's main: %w", err)
	}
	old, err := Git(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	if err != nil {
		return outcome, fmt.Errorf("read origin's main: %w", err)
	}
	outcome.Old = old
	settle := func(main string) error {
		settled, err := Settle(install, main, func(sha, main string) (bool, error) { return contains(checkout, sha, main) }, seams.Done, now())
		outcome.Settled = append(outcome.Settled, settled...)
		return err
	}
	// What main already holds is settled first: a push that crashed before
	// its settlement is finished here.
	settleErr := settle(old)
	onMain, err := IsAncestor(checkout, head, old)
	if err != nil {
		return outcome, errors.Join(err, settleErr)
	}
	if onMain {
		outcome.Commit = old
		return outcome, settleErr
	}
	if refusal := provenGreen(install, tree); refusal != nil {
		return outcome, errors.Join(refusal, settleErr)
	}
	forward, err := IsAncestor(checkout, old, head)
	if err != nil {
		return outcome, errors.Join(err, settleErr)
	}
	if !forward {
		return outcome, errors.Join(&Refusal{Code: CodeNotFastForward,
			Reason: "HEAD does not contain origin's main " + Short(old) + ", so pushing it would rewrite main; nothing was pushed",
			Next:   "merge origin/main into the lane checkout, prove it and push again"}, settleErr)
	}
	if _, err := Git(checkout, "push", "--quiet", "--force-with-lease=refs/heads/main:"+old, "origin", head+":refs/heads/main"); err != nil {
		return outcome, errors.Join(fmt.Errorf("push %s to main: %w", Short(head), err), settleErr)
	}
	outcome.Changed = true
	recordErr := withLock(install, func() error {
		return appendLine(pushesPath(install), Pushed{Old: old, Commit: head, Tree: tree, At: now().Format(time.RFC3339)})
	})
	return outcome, errors.Join(settleErr, recordErr, settle(head))
}

// provenGreen refuses a tree results.jsonl does not hold green.
func provenGreen(install, tree string) *Refusal {
	result, ok, err := ResultFor(install, tree)
	switch {
	case err != nil:
		return &Refusal{Code: CodeUnproven, Reason: "the proof results can't be read (" + err.Error() + "), so nothing was pushed", Next: "landing prove"}
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

// contains says whether main contains sha; a commit the checkout does not
// have is not in main, which the checkout holds whole.
func contains(dir, sha, main string) (bool, error) {
	if _, err := Git(dir, "cat-file", "-e", sha+"^{commit}"); err != nil {
		return false, nil
	}
	return IsAncestor(dir, sha, main)
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
