package kernel

import (
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batch"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/batchowner"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/lane"
)

// The refusals of landing push.
const (
	// CodePushUnproven is a HEAD whose exact tree landing prove did not
	// record green.
	CodePushUnproven = "LANE_PUSH_UNPROVEN"
	// CodePushNotFastForward is a HEAD that does not contain origin's main.
	CodePushNotFastForward = "LANE_PUSH_NOT_FAST_FORWARD"
)

// PushRequest is one landing push.
type PushRequest struct {
	Home   string
	Layout lane.Layout
	Actor  string
}

// PushSeams are push's effects; ProductionPushSeams is the zero case.
type PushSeams struct {
	Now func() time.Time
	// Publish pushes the tuple through the lane's publication boundary.
	Publish func(home string, tuple lane.Tuple) error
	// Settle records every queued member the pushed commit contains as
	// landed and concludes it.
	Settle func(batchowner.PushedSettlement) ([]string, error)
}

// ProductionPushSeams are the production effects.
func ProductionPushSeams() PushSeams {
	return PushSeams{Now: func() time.Time { return time.Now().UTC() },
		Publish: func(home string, tuple lane.Tuple) error {
			return lane.Publish(home, tuple, lane.OpPublish, lane.AuthorityAgent)
		},
		Settle: batchowner.SettlePushed}
}

// PushOutcome is what landing push did: main moved from Old to Commit
// (Changed), or already was Commit; Landed are the members it settled.
type PushOutcome struct {
	Old     string   `json:"old"`
	Commit  string   `json:"commit"`
	Tree    string   `json:"tree"`
	Attempt string   `json:"attempt"`
	Changed bool     `json:"changed"`
	Landed  []string `json:"landed"`
}

// Push is rail 1: it puts the lane checkout's HEAD on main only when landing
// prove recorded a green result for HEAD's exact tree, and only as a
// fast-forward: origin's main, fetched now, must be an ancestor of HEAD, and
// the push leases main at that commit. A HEAD already on main pushes
// nothing. Either way every queued member whose head main now contains is
// recorded landed.
func Push(request PushRequest, seams PushSeams) (PushOutcome, error) {
	checkout := string(request.Layout.Checkout)
	head, err := gitIn(checkout, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return PushOutcome{}, fmt.Errorf("read the lane checkout's HEAD: %w", err)
	}
	tree, err := (gittree.Workspace{Dir: checkout}).TreeOf(head)
	if err != nil {
		return PushOutcome{}, err
	}
	outcome := PushOutcome{Commit: head, Tree: tree, Landed: []string{}}
	proof, proven, err := ReadTreeProof(request.Layout, tree)
	switch {
	case err != nil:
		return outcome, &Refusal{Code: CodePushUnproven, Reason: "the test result of HEAD's tree " + short(tree) + " can't be read (" + err.Error() + "), so nothing was pushed",
			Next: "run landing prove"}
	case !proven:
		return outcome, &Refusal{Code: CodePushUnproven, Reason: "HEAD's tree " + short(tree) + " was never proven, so nothing was pushed", Next: "run landing prove"}
	case proof.Status != batch.AttemptGreen:
		return outcome, &Refusal{Code: CodePushUnproven, Reason: "HEAD's tree " + short(tree) + " was proven " + proof.Status + ", not green, so nothing was pushed",
			Next: "run landing status to see the last proof"}
	}
	outcome.Attempt = proof.Attempt
	remote, err := gitIn(checkout, "remote", "get-url", "origin")
	if err != nil {
		return outcome, fmt.Errorf("read the lane checkout's origin: %w", err)
	}
	if _, err := gitIn(checkout, "fetch", "--quiet", "origin", "+"+lane.MainRef+":refs/remotes/origin/main"); err != nil {
		return outcome, fmt.Errorf("fetch origin's main: %w", err)
	}
	old, err := gitIn(checkout, "rev-parse", "--verify", "refs/remotes/origin/main^{commit}")
	if err != nil {
		return outcome, fmt.Errorf("read origin's main: %w", err)
	}
	outcome.Old = old
	if old != head {
		forward, err := isAncestor(checkout, old, head)
		if err != nil {
			return outcome, err
		}
		if !forward {
			return outcome, &Refusal{Code: CodePushNotFastForward,
				Reason: "HEAD does not contain origin's main " + short(old) + ", so pushing it would rewrite main; nothing was pushed",
				Next:   "merge origin/main into the lane checkout, prove it, and push again"}
		}
		tuple := lane.Tuple{Repo: checkout, RemoteURL: remote, Ref: lane.MainRef, Old: old, New: head, Tree: tree,
			Kind: lane.KindLanding, Op: proof.Attempt, ProofAttempt: proof.Attempt}
		if err := seams.Publish(request.Home, tuple); err != nil {
			return outcome, err
		}
		outcome.Changed = true
	}
	landed, err := seams.Settle(batchowner.PushedSettlement{Home: request.Home, Checkout: checkout, Install: string(request.Layout.Install),
		Commit: head, Actor: request.Actor, Now: seams.Now()})
	outcome.Landed = append(outcome.Landed, landed...)
	if err != nil {
		return outcome, fmt.Errorf("main is %s, and the members it holds could not all be recorded landed: %w", short(head), err)
	}
	return outcome, nil
}

// isAncestor reports whether ancestor is an ancestor of (or is) commit.
func isAncestor(root, ancestor, commit string) (bool, error) {
	command := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", ancestor, commit)
	command.Env = gittree.ScrubbedEnviron()
	err := command.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return true, nil
	case errors.As(err, &exit) && exit.ExitCode() == 1:
		return false, nil
	}
	return false, fmt.Errorf("whether %s contains %s can't be read: %w", short(commit), short(ancestor), err)
}

func short(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
