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

func concludedGoalAt(repo, endpointTip, goalID string) bool {
	data, err := gitOutput(repo, "show", endpointTip+":metasystem/records/goals/"+goalID+".md")
	if err != nil {
		return false
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "# "+goalID {
		return false
	}
	done, concluded := false, false
	for _, line := range lines {
		done = done || strings.TrimSpace(line) == "- State: done"
		concluded = concluded || strings.HasPrefix(strings.TrimSpace(line), "- Concluded: ") && strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "- Concluded: ")) != ""
	}
	return done && concluded
}

func droppedCommit(repo, endpointTip, goalID, commit, dropped string) bool {
	if !concludedGoalAt(repo, endpointTip, goalID) {
		return false
	}
	digest, err := UnitDigest(repo, commit)
	if err != nil {
		return false
	}
	want := "dropped:" + commit + ":" + digest
	matches := 0
	for _, field := range strings.Fields(dropped) {
		if field == want {
			matches++
		}
	}
	return matches == 1
}

func deleteRemoteRef(transport PushTransport, repo, remote, ref, expected, code string) error {
	outcome, pushErr := transport.Push(repo, remote, ref, expected, "")
	if outcome == CASRefused {
		return operationRefusal(code, "%s moved before leased deletion: %v", ref, pushErr)
	}
	if outcome == CASUnknown {
		observed, present, err := transport.RemoteTip(repo, remote, ref)
		if err != nil || present {
			return operationRefusal(PushUnknownCode, "%s delete outcome is unknown; it now holds %s: %v", ref, observed, pushErr)
		}
	}
	return nil
}

func goalWorktreePaths(repo, goalID string) ([]string, error) {
	out, err := gitOutput(repo, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	wanted := goalBranchRef(goalID)
	var paths []string
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
		if branchRef == wanted && path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func cleanGoalWorktrees(repo, goalID string) ([]string, error) {
	paths, err := goalWorktreePaths(repo, goalID)
	if err != nil {
		return nil, err
	}
	for _, path := range paths {
		status, err := gitOutput(path, "status", "--porcelain=v1", "--untracked-files=all")
		if err != nil {
			return nil, err
		}
		if len(status) != 0 {
			return nil, operationRefusal(StaleCode, "goal/%s worktree %s has uncommitted work", goalID, path)
		}
	}
	return paths, nil
}

func cleanupGoalWorktrees(repo, goalID, endpointTip string, paths []string) error {
	for _, path := range paths {
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
		if _, err := gitOutput(repo, "worktree", "remove", path); err != nil {
			return err
		}
	}
	_, err := gitOutput(repo, "update-ref", "-d", goalBranchRef(goalID))
	return err
}

func checkSweepTip(req SweepRequest, tip string) error {
	if req.Abandoned {
		return nil
	}
	baseOut, err := gitOutput(req.Repo, "merge-base", req.EndpointTip, tip)
	if err != nil {
		return err
	}
	base := strings.TrimSpace(string(baseOut))
	sources, err := sourceCommits(req.Repo, base, req.EndpointTip)
	if err != nil {
		return err
	}
	commits, err := ValidateRange(req.Repo, req.EndpointTip, tip, req.GoalID)
	if err != nil {
		return err
	}
	for _, commit := range commits {
		if !sources[commit.ID] && !droppedCommit(req.Repo, req.EndpointTip, req.GoalID, commit.ID, req.Dropped) {
			return operationRefusal(SweepUnlandedCode, "commit %s (%s %s) is neither a landed Goal-Source nor a declared dropped commit", commit.ID, commit.Kind, commit.Unit)
		}
	}
	return nil
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
	originTip, originPresent, err := req.PushTransport.RemoteTip(req.Repo, req.Remote, ref)
	if err != nil {
		return SweepResult{}, err
	}
	transportTip, transportPresent := "", false
	if req.Transport != "" {
		transportTip, transportPresent, err = req.PushTransport.RemoteTip(req.Repo, req.Transport, ref)
		if err != nil {
			return SweepResult{}, err
		}
	}
	localTip, localPresent, err := localBranchTip(req.Repo, ref)
	if err != nil {
		return SweepResult{}, err
	}
	worktrees, err := cleanGoalWorktrees(req.Repo, req.GoalID)
	if err != nil {
		return SweepResult{}, err
	}
	if !originPresent && !transportPresent && !localPresent && len(worktrees) == 0 {
		return SweepResult{GoalID: req.GoalID}, nil
	}
	if originPresent {
		if err := fetchAndValidate(req.Repo, req.Remote, req.EndpointTip, req.GoalID, "sweep-origin-"+req.GoalID, originTip, req.PushTransport); err != nil {
			return SweepResult{}, err
		}
	}
	if transportPresent {
		if err := fetchAndValidate(req.Repo, req.Transport, req.EndpointTip, req.GoalID, "sweep-transport-"+req.GoalID, transportTip, req.PushTransport); err != nil {
			return SweepResult{}, err
		}
	}
	checkedTip := originTip
	if !originPresent {
		checkedTip = transportTip
		if !transportPresent {
			checkedTip = localTip
		}
	}
	if err := checkSweepTip(req, checkedTip); err != nil {
		return SweepResult{}, err
	}
	if transportPresent && transportTip != checkedTip {
		covered, err := ancestor(req.Repo, transportTip, checkedTip)
		if err != nil {
			return SweepResult{}, err
		}
		if !covered {
			if err := checkSweepTip(req, transportTip); err != nil {
				return SweepResult{}, err
			}
		}
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return SweepResult{}, err
		}
	}
	if originPresent {
		if err := deleteRemoteRef(req.PushTransport, req.Repo, req.Remote, ref, originTip, LeaseMovedCode); err != nil {
			return SweepResult{}, err
		}
	}
	if transportPresent {
		if err := deleteRemoteRef(req.PushTransport, req.Repo, req.Transport, ref, transportTip, LeaseMovedCode); err != nil {
			return SweepResult{}, err
		}
	}
	if err := cleanupGoalWorktrees(req.Repo, req.GoalID, req.EndpointTip, worktrees); err != nil {
		return SweepResult{}, err
	}
	return SweepResult{GoalID: req.GoalID, Tip: checkedTip, Deleted: true}, nil
}
