package dispatch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

// FollowUpRebasePlan is the job-domain decision consumed by the follow-up
// dispatcher. Git mutation remains shell plumbing; this result says whether
// the trunk commits overlap the complete chain path boundary.
type FollowUpRebasePlan struct {
	Rebase           bool     `json:"rebase"`
	Reason           string   `json:"reason"`
	BehindCount      int64    `json:"behindCount"`
	RebasedFrom      string   `json:"rebasedFrom"`
	RebasedTo        string   `json:"rebasedTo"`
	OverlappingPaths []string `json:"overlappingPaths"`
	UnmergedPaths    []string `json:"unmergedPaths"`
}

// PlanFollowUpRebase computes the cumulative chain boundary and compares it
// with every path touched by commits present on trunk but absent from the
// worktree. Worktrees never commit, so their dirty path set carries every
// round's live changes; readable boundaries additionally preserve paths that
// a later round restored to HEAD.
func PlanFollowUpRebase(repoRoot, rootJob, worktree, trunk string) (FollowUpRebasePlan, error) {
	if repoRoot == "" || worktree == "" || trunk == "" || !validJobID.MatchString(rootJob) {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning requires a repository root, chain root job, worktree, and trunk ref")
	}
	from, err := gitOutput(worktree, "rev-parse", "HEAD")
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot resolve the worktree head: %w", err)
	}
	to, err := gitOutput(worktree, "rev-parse", trunk)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot resolve trunk ref %q: %w", trunk, err)
	}
	behindText, err := gitOutput(worktree, "rev-list", "--count", from+".."+to)
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot count trunk commits: %w", err)
	}
	behind, err := strconv.ParseInt(behindText, 10, 64)
	if err != nil || behind < 0 {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning received an invalid behind count %q", behindText)
	}
	plan := FollowUpRebasePlan{
		Reason: "the worktree already contains the trunk commit", BehindCount: behind,
		RebasedFrom: from, RebasedTo: to, OverlappingPaths: []string{}, UnmergedPaths: []string{},
	}
	unmerged, err := gitPathSet(worktree, "diff", "--name-only", "-z", "--diff-filter=U")
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
	dirtyPaths, err := followUpDirtyPaths(worktree)
	if err != nil {
		return FollowUpRebasePlan{}, err
	}
	for path := range dirtyPaths {
		chainPaths[path] = struct{}{}
	}
	touched, err := gitPathSet(worktree, "log", "--format=", "--name-only", "-z", "--no-renames", from+".."+to, "--")
	if err != nil {
		return FollowUpRebasePlan{}, fmt.Errorf("follow-up rebase planning cannot read trunk paths: %w", err)
	}
	for path := range touched {
		if _, overlap := chainPaths[path]; overlap {
			plan.OverlappingPaths = append(plan.OverlappingPaths, path)
		}
	}
	sort.Strings(plan.OverlappingPaths)
	if len(plan.OverlappingPaths) == 0 {
		plan.Reason = "the worktree is behind, but no trunk commit touched this chain's files"
		return plan, nil
	}
	plan.Rebase = true
	plan.Reason = "trunk commits touched this chain's files"
	return plan, nil
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

func followUpDirtyPaths(worktree string) (map[string]struct{}, error) {
	paths, err := gitPathSet(worktree, "diff", "--name-only", "-z", "--no-renames", "HEAD", "--")
	if err != nil {
		return nil, fmt.Errorf("follow-up rebase planning cannot read tracked worktree paths: %w", err)
	}
	untracked, err := gitPathSet(worktree, "ls-files", "--others", "--exclude-standard", "-z", "--")
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
