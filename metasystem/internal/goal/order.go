package goal

import (
	"fmt"
	"sort"
	"strconv"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

type goalRank struct {
	Priority uint8
	Sequence uint64
}

type priorityChange struct {
	File   *GoalFile
	Before goalRank
	After  goalRank
}

type priorityCompaction struct {
	Priority uint8
	Departed []string
	Changed  []priorityChange
	Targets  []string
}

// OrderedOpenGoalIDs returns the scheduling order without changing the
// alphabetical order used by serialization and unrelated projections.
func OrderedOpenGoalIDs(live map[string]*GoalFile) []string {
	ids := sortedGoalIds(live)
	sort.SliceStable(ids, func(i, j int) bool {
		left, right := live[ids[i]], live[ids[j]]
		leftRanked := left.Priority != 0 && left.Sequence != 0
		rightRanked := right.Priority != 0 && right.Sequence != 0
		if leftRanked != rightRanked {
			return leftRanked
		}
		if !leftRanked {
			return ids[i] < ids[j]
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		if left.Sequence != right.Sequence {
			return left.Sequence < right.Sequence
		}
		// A valid accepted tree cannot reach this tie. Do not give an
		// invalid duplicate pair an ordering rule of its own; tree
		// validation owns that refusal.
		return false
	})
	return ids
}

// SetPriority is an enrolled-terminal human act. The proof is checked before
// publication even when the requested pair already holds.
func SetPriority(r VerbRequest, id string, priority uint8, sequence *uint64, proof *humanauthority.Proof) (PublishResult, error) {
	if r.Actor.Human == "" {
		return PublishResult{}, fmt.Errorf("set-priority is a human act and names its human (--by)")
	}
	if proof == nil || !proof.ValidFor(r.Endpoint.Root) {
		return PublishResult{}, fmt.Errorf("set-priority requires freshly observed enrolled-terminal human authority")
	}
	if priority < 1 || priority > 3 {
		return PublishResult{}, fmt.Errorf("set-priority priority %d is not 1, 2, or 3", priority)
	}
	if sequence != nil && *sequence == 0 {
		return PublishResult{}, fmt.Errorf("set-priority sequence is one-based and must be positive")
	}
	return Publish(r.Endpoint, setPriorityRequest(r, id, priority, sequence))
}

func setPriorityRequest(r VerbRequest, id string, priority uint8, sequence *uint64) PublishRequest {
	args := map[string]string{"priority": strconv.FormatUint(uint64(priority), 10)}
	if sequence != nil {
		args["sequence"] = strconv.FormatUint(*sequence, 10)
	}
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "set-priority", Targets: []string{id}, Args: intentArgs(r, args)},
		Message: fmt.Sprintf("goal set-priority %s -> %d", id, priority),
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			target := tree.Live[id]
			if target == nil {
				return nil, fmt.Errorf("goal %s is not open; set-priority changes live backlog goals only", id)
			}
			if opidLanded(target, r) {
				return nil, AlreadyApplied{}
			}

			before := make(map[string]goalRank, len(tree.Live))
			for goalID, file := range tree.Live {
				before[goalID] = rankOf(file)
			}

			destination := priorityIDs(tree.Live, priority, id)
			position := uint64(len(destination) + 1)
			requestWord := "append"
			if sequence != nil {
				position = *sequence
				requestWord = strconv.FormatUint(*sequence, 10)
			} else if target.Priority == priority {
				position = target.Sequence
				requestWord = "keep"
			}
			maximum := uint64(len(destination) + 1)
			if position < 1 || position > maximum {
				return nil, fmt.Errorf("goal %s sequence %d is outside the current destination range 1..%d for priority %d", id, position, maximum, priority)
			}

			if target.Priority != 0 && target.Priority != priority {
				numberPriority(tree.Live, target.Priority, priorityIDs(tree.Live, target.Priority, id))
			}
			insertAt := int(position - 1)
			destination = append(destination, "")
			copy(destination[insertAt+1:], destination[insertAt:])
			destination[insertAt] = id
			numberPriority(tree.Live, priority, destination)

			var changed []priorityChange
			for _, goalID := range sortedGoalIds(tree.Live) {
				after := rankOf(tree.Live[goalID])
				if before[goalID] != after {
					changed = append(changed, priorityChange{File: tree.Live[goalID], Before: before[goalID], After: after})
				}
			}
			if len(changed) == 0 {
				return nil, NothingToDo{Reason: "the requested priority and sequence already hold"}
			}

			targets := make([]string, 0, len(changed))
			for _, change := range changed {
				targets = append(targets, change.File.Id)
			}
			sort.Strings(targets)
			changes := make([]Change, 0, len(changed))
			for _, change := range changed {
				touch(change.File, r, "set-priority", targets)
				change.File.History[len(change.File.History)-1].Reason = fmt.Sprintf(
					"priority-order subject=%s from=%s to=%s requested-sequence=%s",
					id, renderRank(change.Before), renderRank(change.After), requestWord,
				)
				changes = append(changes, Change{Path: livePath(change.File.Id), Content: RenderFile(change.File)})
			}
			return ackDisplacements(tree, r, changes), nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}

func rankOf(file *GoalFile) goalRank {
	return goalRank{Priority: file.Priority, Sequence: file.Sequence}
}

func renderRank(rank goalRank) string {
	if rank.Priority == 0 && rank.Sequence == 0 {
		return "unranked"
	}
	return fmt.Sprintf("%d:%d", rank.Priority, rank.Sequence)
}

func priorityIDs(live map[string]*GoalFile, priority uint8, omit string) []string {
	ids := make([]string, 0)
	for id, file := range live {
		if id != omit && file.Priority == priority {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool {
		return live[ids[i]].Sequence < live[ids[j]].Sequence
	})
	return ids
}

func numberPriority(live map[string]*GoalFile, priority uint8, ids []string) {
	for index, id := range ids {
		live[id].Priority = priority
		live[id].Sequence = uint64(index + 1)
	}
}

func compactDepartedPriorities(live map[string]*GoalFile, departed []*GoalFile) []priorityCompaction {
	departedByPriority := map[uint8][]string{}
	for _, file := range departed {
		if file != nil && file.Priority != 0 {
			departedByPriority[file.Priority] = append(departedByPriority[file.Priority], file.Id)
		}
	}
	var compacted []priorityCompaction
	for priority := uint8(1); priority <= 3; priority++ {
		departedIDs := sortedUnique(departedByPriority[priority])
		if len(departedIDs) == 0 {
			continue
		}
		ids := priorityIDs(live, priority, "")
		changes := make([]priorityChange, 0)
		for index, id := range ids {
			file := live[id]
			before := rankOf(file)
			file.Sequence = uint64(index + 1)
			after := rankOf(file)
			if before != after {
				changes = append(changes, priorityChange{File: file, Before: before, After: after})
			}
		}
		targets := append([]string{}, departedIDs...)
		for _, change := range changes {
			targets = append(targets, change.File.Id)
		}
		compacted = append(compacted, priorityCompaction{
			Priority: priority,
			Departed: departedIDs,
			Changed:  changes,
			Targets:  sortedUnique(targets),
		})
	}
	return compacted
}

func mergePriorityEvent(file *GoalFile, r VerbRequest, verb string, targets []string, before, after goalRank) {
	index := len(file.History) - 1
	if index < 0 || file.History[index].Opid != r.opid() {
		touch(file, r, verb, targets)
		index = len(file.History) - 1
	} else {
		file.History[index].Targets = sortedUnique(append(file.History[index].Targets, targets...))
	}
	reason := fmt.Sprintf("priority-order from=%s to=%s", renderRank(before), renderRank(after))
	if file.History[index].Reason == "" {
		file.History[index].Reason = reason
	} else {
		file.History[index].Reason += "; " + reason
	}
}
