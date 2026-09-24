package dispatch

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// FollowUpRebasePlan is the job-domain decision consumed by the follow-up
// dispatcher. Git mutation remains shell plumbing; this result says whether
// trunk changes overlap the chain or supply a path the brief cites.
type FollowUpRebasePlan struct {
	Rebase           bool     `json:"rebase"`
	Reason           string   `json:"reason"`
	BehindCount      int64    `json:"behindCount"`
	RebasedFrom      string   `json:"rebasedFrom"`
	RebasedTo        string   `json:"rebasedTo"`
	OverlappingPaths []string `json:"overlappingPaths"`
	CitedTrunkPaths  []string `json:"citedTrunkPaths"`
	UnmergedPaths    []string `json:"unmergedPaths"`
}

// worktreePosture supplies the facts shared by follow-up planning and cap text.
type worktreePosture interface {
	Head(worktree string) (string, error)
	TrackedPaths(worktree string) (map[string]struct{}, error)
	UntrackedPaths(worktree string) (map[string]struct{}, error)
}

type followUpRebasePosture interface {
	worktreePosture
	ResolveRef(worktree, ref string) (string, error)
	BehindCount(worktree, from, to string) (int64, error)
	UnmergedPaths(worktree string) (map[string]struct{}, error)
	TrunkTouchedPaths(worktree, from, to string) (map[string]struct{}, error)
	InstallPrefix(repoRoot string) (string, error)
	TreeDirectories(worktree, treeish string) (map[string]bool, error)
	PathPresent(worktree, treeish, path string) (bool, error)
}

type gitWorktreePosture struct{}

func (gitWorktreePosture) Head(worktree string) (string, error) {
	return gitOutput(worktree, "rev-parse", "HEAD")
}
func (gitWorktreePosture) ResolveRef(worktree, ref string) (string, error) {
	return gitOutput(worktree, "rev-parse", ref)
}

type invalidBehindCountError struct{ text string }

func (e invalidBehindCountError) Error() string {
	return fmt.Sprintf("follow-up rebase planning received an invalid behind count %q", e.text)
}
func (gitWorktreePosture) BehindCount(worktree, from, to string) (int64, error) {
	text, err := gitOutput(worktree, "rev-list", "--count", from+".."+to)
	if err != nil {
		return 0, err
	}
	count, err := strconv.ParseInt(text, 10, 64)
	if err != nil || count < 0 {
		return 0, invalidBehindCountError{text}
	}
	return count, nil
}
func (gitWorktreePosture) UnmergedPaths(worktree string) (map[string]struct{}, error) {
	return gitPathSet(worktree, "diff", "--name-only", "-z", "--diff-filter=U")
}
func (gitWorktreePosture) TrackedPaths(worktree string) (map[string]struct{}, error) {
	return gitPathSet(worktree, "diff", "--name-only", "-z", "--no-renames", "HEAD", "--")
}
func (gitWorktreePosture) UntrackedPaths(worktree string) (map[string]struct{}, error) {
	return gitPathSet(worktree, "ls-files", "--others", "--exclude-standard", "-z", "--")
}
func (gitWorktreePosture) TrunkTouchedPaths(worktree, from, to string) (map[string]struct{}, error) {
	return gitPathSet(worktree, "log", "--format=", "--name-only", "-z", "--no-renames", from+".."+to, "--")
}
func (gitWorktreePosture) InstallPrefix(repoRoot string) (string, error) {
	return projectInstallPrefix(repoRoot)
}
func (gitWorktreePosture) TreeDirectories(worktree, treeish string) (map[string]bool, error) {
	return treeDirectories(worktree, treeish)
}
func (gitWorktreePosture) PathPresent(worktree, treeish, path string) (bool, error) {
	_, err := gitOutput(worktree, "cat-file", "-e", treeish+":"+path)
	return err == nil, nil
}

// PlanFollowUpRebase computes the cumulative chain boundary and compares it
// with every path touched by commits present on trunk but absent from the
// worktree. Worktrees never commit, so their dirty path set carries every
// round's live changes; readable boundaries additionally preserve paths that
// a later round restored to HEAD.
func PlanFollowUpRebase(repoRoot, rootJob, worktree, trunk, briefPath string) (FollowUpRebasePlan, error) {
	return planFollowUpRebase(repoRoot, rootJob, worktree, trunk, briefPath, gitWorktreePosture{})
}

func planFollowUpRebase(repoRoot, rootJob, worktree, trunk, briefPath string, posture followUpRebasePosture) (FollowUpRebasePlan, error) {
	if repoRoot == "" || worktree == "" || trunk == "" || !validJobID.MatchString(rootJob) {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning requires a repository root, chain root job, worktree, and trunk ref")
	}
	from, err := posture.Head(worktree)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot resolve the worktree head: %w", err)
	}
	to, err := posture.ResolveRef(worktree, trunk)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot resolve trunk ref %q: %w", trunk, err)
	}
	behind, err := posture.BehindCount(worktree, from, to)
	if err != nil {
		var invalid invalidBehindCountError
		if errors.As(err, &invalid) {
			return FollowUpRebasePlan{}, err
		}
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot count trunk commits: %w", err)
	}
	plan := FollowUpRebasePlan{
		Reason: "the worktree already contains the trunk commit", BehindCount: behind,
		RebasedFrom: from, RebasedTo: to, OverlappingPaths: []string{}, CitedTrunkPaths: []string{}, UnmergedPaths: []string{},
	}
	unmerged, err := posture.UnmergedPaths(worktree)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot read unmerged worktree paths: %w", err)
	}
	for path := range unmerged {
		plan.UnmergedPaths = append(plan.UnmergedPaths, path)
	}
	sort.Strings(plan.UnmergedPaths)
	if behind == 0 {
		return plan, nil
	}

	chainPaths, err := followUpChainPaths(repoRoot, rootJob)
	if err != nil {
		return FollowUpRebasePlan{}, err
	}
	dirtyPaths, err := followUpDirtyPaths(worktree, posture)
	if err != nil {
		return FollowUpRebasePlan{}, err
	}
	for path := range dirtyPaths {
		chainPaths[path] = struct{}{}
	}
	touched, err := posture.TrunkTouchedPaths(worktree, from, to)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot read trunk paths: %w", err)
	}
	for path := range touched {
		if _, overlap := chainPaths[path]; overlap {
			plan.OverlappingPaths = append(plan.OverlappingPaths, path)
		}
	}
	sort.Strings(plan.OverlappingPaths)
	plan.CitedTrunkPaths, err = followUpCitedTrunkPaths(repoRoot, worktree, from, to, briefPath, posture)
	if err != nil {
		return FollowUpRebasePlan{}, err
	}
	if len(plan.OverlappingPaths) > 0 {
		plan.Rebase = true
		plan.Reason = "trunk commits touched this chain's files"
	} else if len(plan.CitedTrunkPaths) > 0 {
		plan.Rebase = true
		plan.Reason = "the brief cites paths the trunk gained"
	} else {
		plan.Reason = "the worktree is behind, but no trunk commit touched this chain's files"
	}
	return plan, nil
}

func followUpCitedTrunkPaths(repoRoot, worktree, from, to, briefPath string, posture followUpRebasePosture) ([]string, error) {
	paths := []string{}
	if briefPath == "" {
		return paths, nil
	}
	data, err := os.ReadFile(briefPath)
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot read brief: %w", err)
	}
	var bounds BriefBounds
	_, err = admitBriefBytes(data, func() (string, error) { return posture.InstallPrefix(repoRoot) }, false,
		func(_ []byte, admitted BriefBounds) error { bounds = admitted; return nil })
	if err != nil {
		return paths, nil
	}
	topDirectories, err := posture.TreeDirectories(worktree, to)
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot inspect the trunk tree: %w", err)
	}
	nestedDirectories := map[string]bool{}
	if topDirectories["metasystem"] {
		nestedDirectories, err = posture.TreeDirectories(worktree, to+":metasystem")
		if err != nil {
			return nil, fmt.Errorf("follow-up rebase planning cannot inspect the trunk metasystem tree: %w", err)
		}
	}
	for _, candidate := range extractBriefAuthorityPaths(string(data), bounds, topDirectories, nestedDirectories) {
		if artifactAuthorityPath(candidate) {
			continue
		}
		inFrom, fromErr := posture.PathPresent(worktree, from, candidate)
		if fromErr != nil {
			return nil, fromErr
		}
		if inFrom {
			continue
		}
		inTo, toErr := posture.PathPresent(worktree, to, candidate)
		if toErr != nil {
			return nil, toErr
		}
		if inTo {
			paths = append(paths, candidate)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func followUpChainPaths(repoRoot, rootJob string) (map[string]struct{}, error) {
	roundsRoot := filepath.Join(repoRoot, "artifacts", "agents", rootJob, "rounds")
	entries, err := os.ReadDir(roundsRoot)
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot read chain rounds for %s: %w", rootJob, err)
	}
	rounds := make([]int, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		round, parseErr := strconv.Atoi(entry.Name())
		if parseErr == nil && round > 0 && strconv.Itoa(round) == entry.Name() {
			rounds = append(rounds, round)
		}
	}
	if len(rounds) == 0 {
		return nil, fmt.Errorf("follow-up rebase planning found no readable rounds for chain %s", rootJob)
	}
	sort.Ints(rounds)
	for index, round := range rounds {
		if round != index+1 {
			return nil, fmt.Errorf("follow-up rebase planning cannot read chain %s round %d", rootJob, index+1)
		}
	}
	paths := map[string]struct{}{}
	for _, round := range rounds {
		returnPath := filepath.Join(roundsRoot, strconv.Itoa(round), "return.json")
		content, readErr := os.ReadFile(returnPath)
		if readErr != nil {
			if os.IsNotExist(readErr) {
				continue
			}
			return nil, fmt.Errorf("follow-up rebase planning cannot read chain %s round %d return: %w", rootJob, round, readErr)
		}
		var raw map[string]json.RawMessage
		if decodeErr := json.Unmarshal(content, &raw); decodeErr != nil {
			return nil, fmt.Errorf("follow-up rebase planning cannot decode chain %s round %d return: %w", rootJob, round, decodeErr)
		}
		boundaryJSON, present := raw["diffBoundary"]
		if !present || bytes.Equal(bytes.TrimSpace(boundaryJSON), []byte("null")) {
			continue
		}
		var boundary []string
		if decodeErr := json.Unmarshal(boundaryJSON, &boundary); decodeErr != nil {
			return nil, fmt.Errorf("follow-up rebase planning cannot decode chain %s round %d diffBoundary: %w", rootJob, round, decodeErr)
		}
		for _, path := range boundary {
			if !validFollowUpRebasePath(path) {
				return nil, fmt.Errorf("follow-up rebase planning found invalid path %q in chain %s round %d", path, rootJob, round)
			}
			paths[path] = struct{}{}
		}
	}
	return paths, nil
}

func validFollowUpRebasePath(path string) bool {
	return path != "" && !filepath.IsAbs(path) && path != ".." &&
		!bytes.HasPrefix([]byte(path), []byte("../")) && filepath.Clean(path) == filepath.FromSlash(path)
}

func followUpDirtyPaths(worktree string, posture worktreePosture) (map[string]struct{}, error) {
	paths, err := posture.TrackedPaths(worktree)
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot read tracked worktree paths: %w", err)
	}
	untracked, err := posture.UntrackedPaths(worktree)
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot read untracked worktree paths: %w", err)
	}
	for path := range untracked {
		paths[path] = struct{}{}
	}
	return paths, nil
}

func gitPathSet(dir string, args ...string) (map[string]struct{}, error) {
	output, err := gitRawOutput(dir, args...)
	if err != nil {
		return nil, err
	}
	paths := map[string]struct{}{}
	for _, raw := range bytes.Split(output, []byte{0}) {
		if len(raw) > 0 {
			paths[string(raw)] = struct{}{}
		}
	}
	return paths, nil
}
