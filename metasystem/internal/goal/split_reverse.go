package goal

import (
	"fmt"
	"strings"
)

// ReverseSplit compensates an unstarted split without deleting its lineage.
func ReverseSplit(r VerbRequest, id, transaction, reason string) (PublishResult, error) {
	return Publish(r.Endpoint, PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "split-reverse", Targets: []string{id}, Args: intentArgs(r, map[string]string{"transaction": transaction, "reason": reason, "impact": "restore the parent and park its children without execution approval"})},
		Message: "goal split " + id + " --reverse",
		Mutate: func(tip string) ([]Change, error) {
			if r.Actor.Human == "" || r.Authority == nil || !r.Authority.ValidFor(r.Endpoint.Root) || r.Authority.Helm != nil {
				return nil, fmt.Errorf("reversing a split is a person's act at their terminal\nrun: metasystem goal split %s --reverse --reason TEXT --by <your name>", id)
			}
			if strings.TrimSpace(reason) == "" {
				return nil, fmt.Errorf("reversing goal %s needs a reason", id)
			}
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			parent := t.Live[id]
			if parent == nil || parent.Split == nil {
				return nil, fmt.Errorf("goal %s has no split to reverse", id)
			}
			if parent.Split.Transaction != transaction {
				return nil, fmt.Errorf("goal %s has a different split; inspect metasystem goal show %s before reversing it", id, id)
			}
			if opidLanded(parent, r) {
				return nil, AlreadyApplied{}
			}
			if parent.State != StateSplit {
				for i := len(parent.History) - 1; i >= 0; i-- {
					if parent.History[i].Verb == "split" {
						break
					}
					if parent.History[i].Verb == "split-reverse" {
						return nil, AlreadyHolds{Reason: "goal " + id + " is already reversed"}
					}
				}
				return nil, fmt.Errorf("goal %s is not an active split", id)
			}
			targets := append([]string{id}, parent.Split.Children...)
			changes := make([]Change, 0, len(targets)+1)
			for _, childID := range parent.Split.Children {
				child := t.Live[childID]
				if child == nil {
					return nil, fmt.Errorf("child %s is no longer live; reconcile its work before reversing %s", childID, id)
				}
				started := child.FirstClaimAt != "" || child.Claimed != nil || child.Sliced != nil || child.Episode != nil
				// Older goal records carry claim evidence only in their history.
				for _, h := range child.History {
					if h.Verb == "claim" || h.Verb == "steal" || h.Verb == "slice-start" || h.Verb == "release" || h.Displaced != "" && !h.Ack {
						started = true
					}
				}
				if started {
					return nil, fmt.Errorf("child %s has started work; reconcile that work before reversing %s", childID, id)
				}
				if r.SplitCheck == nil {
					return nil, fmt.Errorf("child %s's work evidence is unavailable; inspect metasystem goal show %s", childID, childID)
				}
				if err := r.SplitCheck(child); err != nil {
					return nil, err
				}
				child.State, child.Claimed = StateParked, nil
				child.Parked = &ParkRecord{By: r.Actor.historyActor(), At: r.stamp(), Because: "split reversed: " + reason}
				child.Approved, child.Budget, child.NormApproval = nil, nil, nil
				child.StopCapability, child.Landing = nil, nil
				touch(child, r, "split-reverse", targets)
				child.History[len(child.History)-1].Reason = reason
				changes = append(changes, Change{Path: livePath(childID), Content: RenderFile(child)})
			}
			parent.State, parent.Parked = parent.Split.PriorState, parent.Split.PriorParked
			if parent.State == StateClaimed {
				parent.State = StateApproved
				if _, err := requireCurrentApproval(t, parent, r.Now, "reverse"); err != nil {
					parent.State, parent.Approved = StateQueued, nil
				}
			}
			touch(parent, r, "split-reverse", targets)
			parent.History[len(parent.History)-1].Reason = reason
			changes = append(changes, Change{Path: livePath(id), Content: RenderFile(parent)})
			t.Root.Free = nil
			t.Root.Revision++
			t.Root.History = append(t.Root.History, HistoryLine{At: r.stamp(), Opid: r.opid(), Verb: "split-reverse", Actor: r.Actor.historyActor(), Targets: targets, Keep: -1, Reason: reason})
			changes = append(changes, Change{Path: goalsPrefix + "backlog.md", Content: RenderRoot(t.Root)})
			return changes, nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	})
}
