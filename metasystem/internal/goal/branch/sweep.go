package branch

import (
	"fmt"
	"path/filepath"
	"strings"
)

const SweepUnlandedCode = "GOAL_SWEEP_UNLANDED"

type SweepHooks struct {
	AfterRemoteRead func() error
}

type SweepRequest struct {
	Repo, Remote, Transport, EndpointTip, GoalID, Dropped string
	Abandoned                                             bool
	CheckClaim                                            func() error
	PushTransport                                         PushTransport
	Hooks                                                 SweepHooks
}

type SweepResult struct {
	GoalID, Tip string
	Deleted     bool
}

type DeleteLandingRequest struct {
	Repo, Remote, GoalID string
	CheckClaim           func() error
	PushTransport        PushTransport
}

func DeleteLanding(req DeleteLandingRequest) error {
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{}
	}
	if !validName(req.GoalID) || req.Repo == "" || req.Remote == "" {
		return fmt.Errorf("landing branch delete needs a repository, remote, and goal")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return err
	}
	ref := landingBranchRef(req.GoalID)
	tip, present, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, ref)
	if err != nil || !present {
		return err
	}
	return deleteRemoteRef(req.PushTransport, req.Repo, req.Remote, ref, tip, LandBranchMovedCode)
}

func sourceCommits(repo, base, endpointTip string) (map[string]bool, error) {
	out, err := gitOutput(repo, "log", "--format=%(trailers:key=Goal-Source,valueonly)", base+".."+endpointTip)
	if err != nil {
		return nil, err
	}
	sources := map[string]bool{}
	for _, value := range strings.Fields(string(out)) {
		if hex40(value) {
			sources[value] = true
		}
	}
	return sources, nil
}

func droppedCommit(repo, commit, dropped string) bool {
	if !strings.Contains(dropped, commit) {
		return false
	}
	digest, err := UnitDigest(repo, commit)
	return err == nil && strings.Contains(dropped, digest)
}

func deleteRemoteRef(transport PushTransport, repo, remote, ref, expected, code string) error {
	outcome, pushErr := transport.Push(repo, remote, ref, expected, "")
	if outcome != CASLanded {
		return operationRefusal(code, "%s moved before leased deletion: %v", ref, pushErr)
	}
	return nil
}

func cleanupGoalWorktrees(repo, goalID, endpointTip string) error {
	out, err := gitOutput(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return err
	}
	wanted := goalBranchRef(goalID)
	for _, block := range strings.Split(strings.TrimSpace(string(out)), "\n\n") {
		path, branchRef := "", ""
		for _, line := range strings.Split(block, "\n") {
			if value, ok := strings.CutPrefix(line, "worktree "); ok {
				path = value
			}
			if value, ok := strings.CutPrefix(line, "branch "); ok {
				branchRef = value
			}
		}
		if branchRef != wanted || path == "" {
			continue
		}
		same, _ := filepath.Abs(path)
		root, _ := filepath.Abs(repo)
		if resolved, resolveErr := filepath.EvalSymlinks(same); resolveErr == nil {
			same = resolved
		}
		if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
			root = resolved
		}
		if same == root {
			if _, err := gitOutput(repo, "switch", "--quiet", "--detach", endpointTip); err != nil {
				return err
			}
			continue
		}
		if _, err := gitOutput(repo, "worktree", "remove", "--force", path); err != nil {
			return err
		}
	}
	_, err = gitOutput(repo, "update-ref", "-d", wanted)
	return err
}

func Sweep(req SweepRequest) (SweepResult, error) {
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{}
	}
	if req.Repo == "" || req.Remote == "" || !hex40(req.EndpointTip) || !validName(req.GoalID) {
		return SweepResult{}, fmt.Errorf("goal branch sweep needs a repository, remote, endpoint, and goal")
	}
	if err := checkClaim(req.CheckClaim); err != nil {
		return SweepResult{}, err
	}
	ref := goalBranchRef(req.GoalID)
	tip, present, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, ref)
	if err != nil || !present {
		return SweepResult{GoalID: req.GoalID}, err
	}
	if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, "sweep-"+req.GoalID, tip, req.PushTransport); err != nil {
		return SweepResult{}, err
	}
	if !req.Abandoned {
		baseOut, err := gitOutput(req.Repo, "merge-base", req.EndpointTip, tip)
		if err != nil {
			return SweepResult{}, err
		}
		base := strings.TrimSpace(string(baseOut))
		sources, err := sourceCommits(req.Repo, base, req.EndpointTip)
		if err != nil {
			return SweepResult{}, err
		}
		commits, err := ValidateRange(req.Repo, req.EndpointTip, tip, req.GoalID)
		if err != nil {
			return SweepResult{}, err
		}
		for _, commit := range commits {
			if !sources[commit.ID] && !droppedCommit(req.Repo, commit.ID, req.Dropped) {
				return SweepResult{}, operationRefusal(SweepUnlandedCode, "commit %s (%s %s) is neither a landed Goal-Source nor a declared dropped commit", commit.ID, commit.Kind, commit.Unit)
			}
		}
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return SweepResult{}, err
		}
	}
	if err := deleteRemoteRef(req.PushTransport, req.Repo, req.Remote, ref, tip, LeaseMovedCode); err != nil {
		return SweepResult{}, err
	}
	if req.Transport != "" {
		transportTip, exists, err := req.PushTransport.RemoteTip(req.Repo, req.Transport, ref)
		if err != nil {
			return SweepResult{}, err
		}
		if exists {
			if err := deleteRemoteRef(req.PushTransport, req.Repo, req.Transport, ref, transportTip, LeaseMovedCode); err != nil {
				return SweepResult{}, err
			}
		}
	}
	if err := cleanupGoalWorktrees(req.Repo, req.GoalID, req.EndpointTip); err != nil {
		return SweepResult{}, err
	}
	return SweepResult{GoalID: req.GoalID, Tip: tip, Deleted: true}, nil
}
