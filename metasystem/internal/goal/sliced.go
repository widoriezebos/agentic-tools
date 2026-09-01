package goal

import "fmt"

// MarkSliced publishes the first-slicing fact before dispatch may create a
// reservation. There is deliberately no command-line mount.
func MarkSliced(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, sliceStartRequest(r, id))
}

func sliceStartRequest(r VerbRequest, id string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "slice-start", Targets: []string{id}, Args: intentArgs(r, nil)},
		Message: "goal slice-start " + id,
		Mutate: func(tip string) ([]Change, error) {
			tree, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			goal := tree.Live[id]
			if goal == nil {
				return nil, fmt.Errorf("goal %s is not live; no reservation may start", id)
			}
			if goal.Sliced != nil {
				return nil, NothingToDo{Reason: "first-slicing fact already recorded"}
			}
			if goal.State != StateClaimed || goal.Claimed == nil {
				return nil, fmt.Errorf("goal %s is %s, not claimed; no reservation may start", id, goal.State)
			}
			if !ownPair(goal.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s, not the dispatch actor", id, goal.Claimed.Machine, goal.Claimed.Lineage)
			}
			goal.Sliced = &SlicedRecord{Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Revision: goal.Claimed.Revision, At: r.stamp()}
			touch(goal, r, "slice-start", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(goal)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}
