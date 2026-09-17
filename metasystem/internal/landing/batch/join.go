package batch

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
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
				return nil, refuseBatch("BATCH_JOIN_CONFLICT", "unit "+unit.GoalID+" does not apply: "+strings.TrimSpace(string(output)))
			}
			conflicts := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
			return nil, refuseBatch("BATCH_JOIN_CONFLICT", "unit "+unit.GoalID+" paths "+strings.Join(conflicts, ", "))
		}
		next, snapshotErr := workspace.Snapshot("HEAD")
		if snapshotErr != nil {
			return nil, snapshotErr
		}
		prefixes = append(prefixes, next)
	}
	return prefixes, nil
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
