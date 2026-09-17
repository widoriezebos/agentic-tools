package batch

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type batchRecordFields struct {
	BaseTree       string           `json:"baseTree,omitempty"`
	ClosedReason   string           `json:"closedReason,omitempty"`
	PrefixTrees    []string         `json:"prefixTrees,omitempty"`
	SelectedGroups []string         `json:"selectedGroups,omitempty"`
	Seal           map[string]Claim `json:"seal,omitempty"`
}

type assemblyConflict struct {
	GoalID string
	Cause  error
}

func (conflict *assemblyConflict) Error() string { return conflict.Cause.Error() }
func (conflict *assemblyConflict) Unwrap() error { return conflict.Cause }

func joinRefusal(record Record) error {
	if record.ClosedReason != "" {
		return refuseBatch("BATCH_CLOSED", "batch "+record.BatchID+" closed at "+record.ClosedReason)
	}
	if record.State != StateOpen {
		return refuseBatch("BATCH_SEALED", "batch "+record.BatchID+" no longer accepts joins")
	}
	return nil
}

func assembleUnits(root, base string, units []Unit) (prefixes []string, err error) {
	if slices.ContainsFunc(units, func(unit Unit) bool { return len(unit.Builds) != 0 }) {
		current := base
		for _, unit := range units {
			member, ok := branchMemberOf(unit)
			if !ok {
				return nil, refuseBatch("BATCH_JOIN_UNREAD", "goal "+unit.GoalID+" has no branch member identity")
			}
			memberPrefixes, memberErr := AssembleBranchMembers(root, current, []BranchMember{member})
			if memberErr != nil {
				return nil, memberErr
			}
			current = memberPrefixes[0]
			prefixes = append(prefixes, current)
		}
		return prefixes, nil
	}
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(base)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	workspace := detached.Workspace()
	for _, unit := range units {
		patch, err := os.ReadFile(filepath.Join(root, "artifacts", "agents", "landing-batches", "chains", unit.Chain, "diff.patch"))
		if err != nil {
			return nil, err
		}
		command := exec.Command("git", "-C", workspace.Dir, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-")
		command.Env, command.Stdin = gittree.ScrubbedEnviron(), bytes.NewReader(patch)
		if output, applyErr := command.CombinedOutput(); applyErr != nil {
			paths := exec.Command("git", "-C", workspace.Dir, "diff", "--name-only", "--diff-filter=U", "-z")
			paths.Env = gittree.ScrubbedEnviron()
			raw, _ := paths.Output()
			if len(raw) == 0 {
				return nil, &assemblyConflict{GoalID: unit.GoalID, Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit "+unit.GoalID+" does not apply: "+strings.TrimSpace(string(output)))}
			}
			conflicts := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
			return nil, &assemblyConflict{GoalID: unit.GoalID, Cause: refuseBatch("BATCH_JOIN_CONFLICT", "unit "+unit.GoalID+" paths "+strings.Join(conflicts, ", "))}
		}
		next, snapshotErr := workspace.Snapshot("HEAD")
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		prefixes = append(prefixes, next)
	}
	return prefixes, nil
}

func branchPatch(repo, commit string) ([]byte, error) {
	command := exec.Command("git", "-C", repo, "-c", "core.useReplaceRefs=false", "diff", "--binary", "--full-index", commit+"^", commit)
	command.Env = gittree.ScrubbedEnviron()
	output, err := command.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("read branch patch %s: %s: %w", commit, strings.TrimSpace(string(output)), err)
	}
	return output, nil
}

func branchMemberPatch(repo string, member BranchMember) ([]byte, error) {
	var patch bytes.Buffer
	for _, build := range member.Builds {
		commits := make([]string, 0, len(build.Folds)+1)
		for _, fold := range build.Folds {
			commits = append(commits, fold.ID)
		}
		commits = append(commits, build.Commit)
		for _, commit := range commits {
			transition, err := branchPatch(repo, commit)
			if err != nil {
				return nil, err
			}
			patch.Write(transition)
			if len(transition) != 0 && transition[len(transition)-1] != '\n' {
				patch.WriteByte('\n')
			}
		}
	}
	return patch.Bytes(), nil
}

// BranchMemberPatch returns the commit transitions applied for one branch contribution.
func BranchMemberPatch(repo string, member BranchMember) ([]byte, error) {
	return branchMemberPatch(repo, member)
}

func applyBranchCommit(repo, worktree, commit string) error {
	patch, err := branchPatch(repo, commit)
	if err != nil {
		return err
	}
	command := exec.Command("git", "-C", worktree, "-c", "core.useReplaceRefs=false", "-c", "core.hooksPath=/dev/null", "apply", "--index", "--3way", "--binary", "--whitespace=nowarn", "-")
	command.Env, command.Stdin = gittree.ScrubbedEnviron(), bytes.NewReader(patch)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("apply branch commit %s: %s: %w", commit, strings.TrimSpace(string(output)), err)
	}
	return nil
}

func treeTransitionDigest(repo, before, after string) (string, error) {
	command := exec.Command("git", "-C", repo, "-c", "core.useReplaceRefs=false", "diff-tree", "-r", "-z", "--no-renames", "--full-index", before, after)
	command.Env = gittree.ScrubbedEnviron("LC_ALL=C")
	raw, err := command.Output()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// AssembleBranchMembers applies each member as a contiguous sequence and
// checks every change against its own transition before exposing cumulative
// member prefix trees.
func AssembleBranchMembers(root, base string, members []BranchMember) (prefixes []string, err error) {
	detached, err := (gittree.Workspace{Dir: root}).NewDetachedWorktree(base)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, detached.Close()) }()
	workspace := detached.Workspace()
	for _, member := range members {
		if len(member.Builds) == 0 {
			return nil, refuseBatch("BATCH_JOIN_UNREAD", "goal "+member.GoalID+" contributes no certified build")
		}
		for _, build := range member.Builds {
			for _, fold := range build.Folds {
				if err := applyBranchCommit(root, workspace.Dir, fold.ID); err != nil {
					return nil, refuseBatch("BATCH_JOIN_REREAD", "goal "+member.GoalID+" fold no longer applies: "+err.Error())
				}
			}
			before, err := workspace.Snapshot("HEAD")
			if err != nil {
				return nil, err
			}
			if err := applyBranchCommit(root, workspace.Dir, build.Commit); err != nil {
				return nil, refuseBatch("BATCH_JOIN_REREAD", "goal "+member.GoalID+" build no longer applies: "+err.Error())
			}
			after, err := workspace.Snapshot("HEAD")
			if err != nil {
				return nil, err
			}
			digest, err := treeTransitionDigest(root, before, after)
			if err != nil {
				return nil, err
			}
			if digest != build.Digest {
				return nil, refuseBatch("BATCH_JOIN_REREAD", fmt.Sprintf("goal %s build %s applies as %s, not %s", member.GoalID, strings.Join(build.Units, "+"), digest, build.Digest))
			}
		}
		prefix, err := workspace.Snapshot("HEAD")
		if err != nil {
			return nil, err
		}
		prefixes = append(prefixes, prefix)
	}
	return prefixes, nil
}

func AssembleUnits(root, base string, units []Unit) ([]string, error) {
	return assembleUnits(root, base, units)
}

func checkMembership(store Store, batchID, goalID, chainID string) error {
	paths, err := filepath.Glob(filepath.Join(store.root, "artifacts", "agents", "landing-batches", "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		id := strings.TrimSuffix(filepath.Base(path), ".json")
		record, err := store.Load(id)
		if err != nil {
			return err
		}
		if id == batchID {
			if err := joinRefusal(record); err != nil {
				return err
			}
		}
		if record.State == StateLanded || record.State == StateDissolved {
			continue
		}
		for _, unit := range record.Units {
			if unit.State != UnitJoining && unit.State != UnitJoined {
				continue
			}
			if id == batchID {
				if unit.GoalID == goalID && unit.Chain != chainID {
					return refuseBatch("BATCH_GOAL_ELSEWHERE", "goal "+goalID+" already belongs to batch "+id)
				}
				continue
			}
			if unit.Chain == chainID {
				return refuseBatch("BATCH_UNIT_ELSEWHERE", "chain "+chainID+" already belongs to batch "+id)
			}
			if unit.GoalID == goalID {
				return refuseBatch("BATCH_GOAL_ELSEWHERE", "goal "+goalID+" already belongs to batch "+id)
			}
		}
	}
	return nil
}
