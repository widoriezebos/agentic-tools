package goal

import "fmt"

// Restamp moves a claimed goal's stop capability to the authenticated lease
// epoch without changing the claim binding or its accounting episode.
func Restamp(r VerbRequest, id string) (PublishResult, error) {
	return Publish(r.Endpoint, restampRequest(r, id))
}

func restampRequest(r VerbRequest, id string) PublishRequest {
	args := claimIntentArgs(r, map[string]string{"callerClass": r.CallerClass})
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "restamp", Targets: []string{id}, Args: args}, Message: "goal restamp " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTree(r.Endpoint.Root, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil || f.StopCapability == nil {
				return nil, fmt.Errorf("goal %s is not claimed with a stop capability", id)
			}
			if !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s is claimed by %s+%s; caller pair %s+%s does not match", id,
					f.Claimed.Machine, f.Claimed.Lineage, r.Actor.Machine, r.Actor.Lineage)
			}
			rebindEpoch, err := ClaimEpochForRebind(f, r)
			if err != nil {
				return nil, err
			}
			if r.EpochAuthority != EpochAuthorityHolder {
				return nil, fmt.Errorf("goal restamp requires the live lease holder of class MAIN")
			}
			current := f.StopCapability.ClaimEpoch
			if rebindEpoch < current {
				return nil, fmt.Errorf("goal %s stop capability epoch %d cannot move down to lease epoch %d", id, current, rebindEpoch)
			}
			if rebindEpoch == current {
				return nil, NothingToDo{Reason: fmt.Sprintf("stop capability already carries lease epoch %d", current)}
			}
			capability := *f.StopCapability
			capability.ClaimEpoch = rebindEpoch
			f.StopCapability = &capability
			touch(f, r, "restamp", []string{id})
			f.History[len(f.History)-1].Reason = fmt.Sprintf("stop capability claimEpoch %d->%d", current, rebindEpoch)
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		},
		Validate: func(commit string) error { return ValidateCommit(r.Endpoint.Root, commit) },
	}
}
