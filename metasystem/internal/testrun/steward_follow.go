package testrun

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/identity"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/landing/plain"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/steward"
)

// DeployedFollow supplies the observations and mutations of one boundary pass.
type DeployedFollow struct {
	Now            func() time.Time
	Deployed       func() (steward.DeployedEngine, error)
	EnrolledDigest func() (string, error)
	Landed         func(string) error
	Ready          func() (bool, string)
	Lock           func() (func(), error)
	Forward        func(string) error
	Install        func(steward.DeployedEngine) error
	Start          func() error
}

const followWaitingForBoundary = "the seat is not at its unit boundary"

func FollowDeployedEngineAtBoundary(projectRoot string, follow DeployedFollow) (replaced bool, refusal *steward.FollowRefusal, err error) {
	engine, err := follow.Deployed()
	waiting := false
	defer func() {
		if err != nil {
			refusal = &steward.FollowRefusal{Commit: engine.Commit, Reason: err.Error(), Waiting: waiting,
				Command: fmt.Sprintf("cd %s && cp -p %s bin/.metasystem.hand && mv bin/.metasystem.hand bin/metasystem && bin/metasystem system restart", projectRoot, engine.Artifact)}
			if engine.Commit == "" || engine.Artifact == "" {
				refusal.Commit = steward.FollowDeploymentUnavailable
				refusal.Command = "cd " + projectRoot + " && bin/metasystem deploy status"
			}
			_ = steward.NoteFollowFailure(projectRoot, engine.Commit, err.Error(), refusal.Command, follow.Now())
			replaced, err = false, nil
		}
	}()
	if err != nil {
		return false, nil, err
	}
	digest, err := follow.EnrolledDigest()
	if err != nil || digest == engine.Digest {
		return false, nil, err
	}
	if err := follow.Landed(engine.Commit); err != nil {
		return false, nil, err
	}
	ready := func() error {
		if ok, reason := follow.Ready(); !ok {
			waiting = reason == followWaitingForBoundary
			return fmt.Errorf("%s", reason)
		}
		return nil
	}
	if err := ready(); err != nil {
		return false, nil, err
	}
	unlock, err := follow.Lock()
	if err != nil {
		return false, nil, err
	}
	defer unlock()
	for _, step := range []func() error{ready, func() error { return follow.Forward(engine.Commit) }, func() error { return follow.Install(engine) }, follow.Start} {
		if err := step(); err != nil {
			return false, nil, err
		}
	}
	return true, nil, nil
}

// DeployedFollowDependencies binds the boundary path's bounded Git operations without rebuilding.
func DeployedFollowDependencies(ctx context.Context, root string, now func() time.Time, boundary func() (bool, error), deployed func() (steward.DeployedEngine, error), install func(steward.DeployedEngine) error, operations ...steward.RearmOperations) DeployedFollow {
	ops := steward.RearmOperations{Clock: landedRearmClock}
	if len(operations) > 0 {
		ops = operations[0]
	}
	if ops.Clock == nil {
		ops.Clock = landedRearmClock
	}
	if ops.Git != nil {
		ctx = context.WithValue(ctx, landedRearmGitOutputContextKey{}, ops.Git)
	}
	clock, seconds := ops.Clock, ops.ResolveSeconds(root)
	lane := plain.ProveSeams{}
	if ops.Prober != nil {
		lane.Alive = func(run plain.Running) bool {
			ref, err := identity.ParseRef(run.Process)
			return err == nil && identity.LiveRef(ops.Prober, ref) != identity.Dead
		}
	}
	read := func(args ...string) (string, error) {
		return landedRearmGitStep(ctx, clock, seconds, "engine-follow", root, args...)
	}
	ancestor := func(a, b string) (bool, error) { return landedRearmAncestry(ctx, clock, seconds, root, a, b) }
	return DeployedFollow{
		Now: now, Deployed: deployed, Install: install,
		EnrolledDigest: func() (string, error) {
			enrolled, err := steward.VerifyIdentity(steward.RepoIdentityPath(root), root)
			return enrolled.InstallDigest, err
		},
		Landed: func(commit string) error {
			ref, remote, branch, err := landingRefParts(ctx, clock, seconds, root)
			if err != nil {
				return err
			}
			if err := landedRearmFetch(ctx, clock, seconds, root, remote, branch); err != nil {
				return err
			}
			if _, err := read("rev-parse", "--verify", commit+"^{commit}"); err != nil {
				return err
			}
			ok, err := ancestor(commit, ref)
			if err == nil && !ok {
				err = fmt.Errorf("the deployed commit %s is not on %s", commit, ref)
			}
			return err
		},
		Ready: func() (bool, string) {
			if ok, err := boundary(); err != nil {
				return false, "the unit boundary cannot be read: " + err.Error()
			} else if !ok {
				return false, followWaitingForBoundary
			}
			if job, open := steward.OpenJob(root); open {
				return false, "job " + job + " is still open"
			}
			attempts, err := proofrun.ReadAttempts(root)
			if err != nil {
				return false, "proof attempts cannot be read: " + err.Error()
			}
			for _, attempt := range attempts {
				if attempt.Terminal == nil {
					return false, "proof attempt " + attempt.AttemptID + " is still open"
				}
			}
			if _, _, alive, err := plain.ReadRunning(root, lane); err != nil {
				return false, "the lane proof's running file cannot be read: " + err.Error()
			} else if alive {
				return false, "the lane proof is running"
			}
			return true, ""
		},
		Lock: func() (func(), error) { return landedRearmMutationLock(root) },
		Forward: func(commit string) error {
			prefix, err := read("rev-parse", "--show-prefix")
			if err != nil {
				return err
			}
			status, err := landedRearmGitOutputStep(ctx, clock, seconds, "follow-checkout-status", root, "status", "--porcelain=v1", "-z", "--untracked-files=no")
			if err != nil {
				return err
			}
			for _, entry := range strings.Split(status, "\x00") {
				if entry != "" && (len(entry) < 4 || !slices.ContainsFunc(landing.AppendOnlyRegisters(), func(register string) bool { return entry[3:] == prefix+register })) {
					return fmt.Errorf("the checkout has tracked changes: %s", entry)
				}
			}
			if contains, err := ancestor(commit, "HEAD"); err != nil || contains {
				return err
			}
			if behind, err := ancestor("HEAD", commit); err != nil || !behind {
				if err != nil {
					return err
				}
				return fmt.Errorf("HEAD has commits the deployed commit lacks")
			}
			return landedRearmFastForward(ctx, root, commit)
		},
		Start: stewardBoundaryDependencies(ctx, root, "", nil, steward.RearmOperations{Clock: landedRearmClock}).start,
	}
}
