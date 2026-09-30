package branch

import (
	"context"
	"errors"
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
	// Context bounds every git and network call of the sweep; nil is
	// context.Background(). The disk sweeper passes its pass context.
	Context context.Context
}

func (req SweepRequest) context() context.Context {
	if req.Context == nil {
		return context.Background()
	}
	return req.Context
}

type SweepResult struct {
	GoalID, Tip string
	Deleted     bool
}

type sweepDependencies struct {
	localBranchTip func(repo, ref string) (string, bool, error)
	gitOutput      func(repo string, args ...string) ([]byte, error)
	validateRange  func(repo, endpointTip, tip, goalID string) ([]Commit, error)
	ancestor       func(repo, older, newer string) (bool, error)
	clearRef       func(repo, ref string) error
	// objectPresent reports whether a commit is in the local object store,
	// so a plan can validate a remote tip without fetching it.
	objectPresent func(repo, sha string) bool
}

func defaultSweepDependencies() sweepDependencies {
	return sweepDependenciesFor(context.Background())
}

// sweepDependenciesFor binds every git call of a sweep to ctx.
func sweepDependenciesFor(ctx context.Context) sweepDependencies {
	read := func(repo string, args ...string) ([]byte, error) { return gitOutputContext(ctx, repo, args...) }
	return sweepDependencies{
		localBranchTip: func(repo, ref string) (string, bool, error) {
			out, err := read(repo, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
			if err != nil {
				if ctx.Err() != nil {
					return "", false, ctx.Err()
				}
				return "", false, nil
			}
			return strings.TrimSpace(string(out)), true, nil
		},
		gitOutput: read,
		validateRange: func(repo, endpointTip, tip, goalID string) ([]Commit, error) {
			return ValidateRangeWithGit(repo, endpointTip, tip, goalID, read)
		},
		ancestor: func(repo, older, newer string) (bool, error) {
			_, err := read(repo, "merge-base", "--is-ancestor", older, newer)
			if err == nil {
				return true, nil
			}
			if ctx.Err() != nil {
				return false, ctx.Err()
			}
			return ancestor(repo, older, newer)
		},
		clearRef: clearPushTxn,
		objectPresent: func(repo, sha string) bool {
			_, err := read(repo, "cat-file", "-e", sha+"^{commit}")
			return err == nil
		},
	}
}

// IsOperationRefusal reports whether err is a goal-branch refusal with code.
func IsOperationRefusal(err error, code string) bool {
	var refusal *OpError
	return errors.As(err, &refusal) && refusal.Code == code
}

func sourceCommitsWith(repo, base, endpointTip string, read func(string, ...string) ([]byte, error)) (map[string]bool, error) {
	out, err := read(repo, "log", "--format=%(trailers:key=Goal-Source,valueonly)", base+".."+endpointTip)
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

func concludedGoalAtWith(repo, endpointTip, goalID string, read func(string, ...string) ([]byte, error)) bool {
	data, err := read(repo, "show", endpointTip+":metasystem/records/goals/"+goalID+".md")
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

func droppedCommitWith(repo, endpointTip, goalID, commit, dropped string, read func(string, ...string) ([]byte, error)) bool {
	if !concludedGoalAtWith(repo, endpointTip, goalID, read) {
		return false
	}
	digest, err := unitDigestWithGit(repo, commit, read)
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

func goalWorktreePathsWith(repo, goalID string, read func(string, ...string) ([]byte, error)) ([]string, error) {
	out, err := read(repo, "worktree", "list", "--porcelain")
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

func cleanGoalWorktreesWith(repo, goalID string, read func(string, ...string) ([]byte, error)) ([]string, error) {
	return cleanGoalWorktreesReading(repo, goalID, read, false)
}

// cleanGoalWorktreesReading is the dirty-worktree check; observeOnly runs
// status with --no-optional-locks, so the check never refreshes an index.
func cleanGoalWorktreesReading(repo, goalID string, read func(string, ...string) ([]byte, error), observeOnly bool) ([]string, error) {
	paths, err := goalWorktreePathsWith(repo, goalID, read)
	if err != nil {
		return nil, err
	}
	statusArgs := []string{"status", "--porcelain=v1", "--untracked-files=all"}
	if observeOnly {
		statusArgs = append([]string{"--no-optional-locks"}, statusArgs...)
	}
	for _, path := range paths {
		status, err := read(path, statusArgs...)
		if err != nil {
			return nil, err
		}
		if len(status) != 0 {
			entries := strings.Split(strings.TrimSpace(string(status)), "\n")
			for i := range entries {
				entries[i] = strings.TrimSpace(entries[i])
			}
			if len(entries) > 8 {
				entries = append(entries[:8], fmt.Sprintf("and %d more", len(entries)-8))
			}
			return nil, operationRefusal(StaleCode, "goal/%s worktree %s has uncommitted work: %s", goalID, path, strings.Join(entries, ", "))
		}
	}
	return paths, nil
}

func cleanupGoalWorktrees(repo, goalID, endpointTip, localTip string, paths []string, deps sweepDependencies) error {
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
			if _, err := deps.gitOutput(repo, "switch", "--quiet", "--detach", endpointTip); err != nil {
				return err
			}
			continue
		}
		if _, err := deps.gitOutput(repo, "worktree", "remove", path); err != nil {
			return err
		}
	}
	if localTip == "" {
		return nil
	}
	ref := goalBranchRef(goalID)
	if _, err := deps.gitOutput(repo, "update-ref", "-d", ref, localTip); err != nil {
		observed, present, readErr := deps.localBranchTip(repo, ref)
		if readErr != nil {
			return readErr
		}
		if !present {
			observed = "absent"
		}
		return operationRefusal(LeaseMovedCode, "%s moved from expected tip %s to %s", ref, localTip, observed)
	}
	return nil
}

func checkSweepTipWith(req SweepRequest, place, tip string, deps sweepDependencies) error {
	if req.Abandoned {
		return nil
	}
	baseOut, err := deps.gitOutput(req.Repo, "merge-base", req.EndpointTip, tip)
	if err != nil {
		return err
	}
	base := strings.TrimSpace(string(baseOut))
	sources, err := sourceCommitsWith(req.Repo, base, req.EndpointTip, deps.gitOutput)
	if err != nil {
		return err
	}
	commits, err := deps.validateRange(req.Repo, req.EndpointTip, tip, req.GoalID)
	if err != nil {
		return err
	}
	var unlanded []string
	for _, commit := range commits {
		if !sources[commit.ID] && !droppedCommitWith(req.Repo, req.EndpointTip, req.GoalID, commit.ID, req.Dropped, deps.gitOutput) {
			unlanded = append(unlanded, fmt.Sprintf("%s (%s %s)", commit.ID, commit.Kind, commit.Unit))
		}
	}
	if len(unlanded) > 0 {
		if len(unlanded) > 8 {
			unlanded = append(unlanded[:8], fmt.Sprintf("and %d more", len(unlanded)-8))
		}
		return operationRefusal(SweepUnlandedCode, "goal/%s at %s tip %s has unlanded commits: %s", req.GoalID, place, tip, strings.Join(unlanded, ", "))
	}
	return nil
}

func sweepTipCoveredWith(repo, tip string, checked []string, isAncestor func(string, string, string) (bool, error)) (bool, error) {
	for _, checkedTip := range checked {
		if tip == checkedTip {
			return true, nil
		}
		covered, err := isAncestor(repo, tip, checkedTip)
		if err != nil {
			return false, err
		}
		if covered {
			return true, nil
		}
	}
	return false, nil
}

// SweepPlan is Sweep's observation: the refusal Sweep would return and what
// it would remove, read without mutation and without a fetch (a remote tip
// whose commits are not local is left for Sweep to verify). The disk
// sweeper reads it before it enters a goal worktree's critical section.
func SweepPlan(req SweepRequest) (SweepPlanResult, error) {
	return sweepPlanWithDependencies(req, sweepDependenciesFor(req.context()))
}

func Sweep(req SweepRequest) (SweepResult, error) {
	return sweepWithDependencies(req, sweepDependenciesFor(req.context()))
}

// SweepPlanResult is SweepPlan's observation of one goal's branch and
// worktrees.
type SweepPlanResult struct {
	GoalID     string
	Worktrees  []string
	LocalTip   string
	RemoteTips map[string]string
	// Unverified names remote tips whose commits are not in the local store:
	// the plan never fetches, so Sweep checks them when it applies.
	Unverified []string
	// Refusal is the refusal Sweep would return (StaleCode for uncommitted
	// work, SweepUnlandedCode for unlanded commits), or nil.
	Refusal error
}

// Nothing reports a goal with no branch and no worktree left to sweep.
func (p SweepPlanResult) Nothing() bool {
	return p.Refusal == nil && p.LocalTip == "" && len(p.RemoteTips) == 0 && len(p.Worktrees) == 0
}

func sweepPlanWithDependencies(req SweepRequest, deps sweepDependencies) (SweepPlanResult, error) {
	ctx := req.context()
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{Context: ctx}
	}
	if req.Repo == "" || req.Remote == "" || !hex40(req.EndpointTip) || !validName(req.GoalID) {
		return SweepPlanResult{}, fmt.Errorf("goal branch sweep plan needs a repository, remote, endpoint, and goal")
	}
	if err := ctx.Err(); err != nil {
		return SweepPlanResult{}, err
	}
	ref := goalBranchRef(req.GoalID)
	plan := SweepPlanResult{GoalID: req.GoalID, RemoteTips: map[string]string{}}
	for _, remote := range []string{req.Remote, req.Transport} {
		if remote == "" {
			continue
		}
		tip, present, err := req.PushTransport.RemoteTip(req.Repo, remote, ref)
		if err != nil {
			return SweepPlanResult{}, err
		}
		if present {
			plan.RemoteTips[remote] = tip
		}
		if err := ctx.Err(); err != nil {
			return SweepPlanResult{}, err
		}
	}
	localTip, _, err := deps.localBranchTip(req.Repo, ref)
	if err != nil {
		return SweepPlanResult{}, err
	}
	plan.LocalTip = localTip
	worktrees, err := cleanGoalWorktreesReading(req.Repo, req.GoalID, deps.gitOutput, true)
	if err != nil {
		var refusal *OpError
		if errors.As(err, &refusal) {
			plan.Refusal = err
			return plan, nil
		}
		return SweepPlanResult{}, err
	}
	plan.Worktrees = worktrees
	var checked []string
	tips := []struct{ place, tip string }{{req.Remote, plan.RemoteTips[req.Remote]}, {req.Transport, plan.RemoteTips[req.Transport]}, {"local", localTip}}
	for _, candidate := range tips {
		if candidate.tip == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return SweepPlanResult{}, err
		}
		if candidate.place != "local" && candidate.tip != localTip && !deps.objectPresent(req.Repo, candidate.tip) {
			plan.Unverified = append(plan.Unverified, candidate.place+" "+candidate.tip)
			continue
		}
		covered, err := sweepTipCoveredWith(req.Repo, candidate.tip, checked, deps.ancestor)
		if err != nil {
			return SweepPlanResult{}, err
		}
		if covered {
			continue
		}
		if err := checkSweepTipWith(req, candidate.place, candidate.tip, deps); err != nil {
			var refusal *OpError
			if errors.As(err, &refusal) {
				plan.Refusal = err
				return plan, nil
			}
			return SweepPlanResult{}, err
		}
		checked = append(checked, candidate.tip)
	}
	return plan, nil
}

func sweepWithDependencies(req SweepRequest, deps sweepDependencies) (SweepResult, error) {
	ctx := req.context()
	if req.PushTransport == nil {
		req.PushTransport = GitPushTransport{Context: ctx}
	}
	if req.Repo == "" || req.Remote == "" || !hex40(req.EndpointTip) || !validName(req.GoalID) {
		return SweepResult{}, fmt.Errorf("goal branch sweep needs a repository, remote, endpoint, and goal")
	}
	if err := ctx.Err(); err != nil {
		return SweepResult{}, err
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
	localTip, localPresent, err := deps.localBranchTip(req.Repo, ref)
	if err != nil {
		return SweepResult{}, err
	}
	if err := ctx.Err(); err != nil {
		return SweepResult{}, err
	}
	worktrees, err := cleanGoalWorktreesWith(req.Repo, req.GoalID, deps.gitOutput)
	if err != nil {
		return SweepResult{}, err
	}
	if !originPresent && !transportPresent && !localPresent && len(worktrees) == 0 {
		return SweepResult{GoalID: req.GoalID}, nil
	}
	resultTip := localTip
	if transportPresent {
		resultTip = transportTip
	}
	if originPresent {
		resultTip = originTip
	}
	tips := []struct {
		place   string
		tip     string
		present bool
		remote  bool
		opid    string
	}{
		{place: req.Remote, tip: originTip, present: originPresent, remote: true, opid: "sweep-origin-" + req.GoalID},
		{place: req.Transport, tip: transportTip, present: transportPresent, remote: true, opid: "sweep-transport-" + req.GoalID},
		{place: "local", tip: localTip, present: localPresent},
	}
	var checked []string
	for _, candidate := range tips {
		if !candidate.present {
			continue
		}
		if err := ctx.Err(); err != nil {
			return SweepResult{}, err
		}
		if candidate.remote {
			if err := fetchAndValidateWith(req.Repo, candidate.place, req.EndpointTip, req.GoalID, candidate.opid, candidate.tip, req.PushTransport,
				fetchValidationDependencies{validateRange: deps.validateRange, clearRef: deps.clearRef}); err != nil {
				return SweepResult{}, err
			}
		}
		covered, err := sweepTipCoveredWith(req.Repo, candidate.tip, checked, deps.ancestor)
		if err != nil {
			return SweepResult{}, err
		}
		if covered {
			continue
		}
		if err := checkSweepTipWith(req, candidate.place, candidate.tip, deps); err != nil {
			return SweepResult{}, err
		}
		checked = append(checked, candidate.tip)
	}
	if req.Hooks.AfterRemoteRead != nil {
		if err := req.Hooks.AfterRemoteRead(); err != nil {
			return SweepResult{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return SweepResult{}, err
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
	if err := cleanupGoalWorktrees(req.Repo, req.GoalID, req.EndpointTip, localTip, worktrees, deps); err != nil {
		return SweepResult{}, err
	}
	return SweepResult{GoalID: req.GoalID, Tip: resultTip, Deleted: true}, nil
}
