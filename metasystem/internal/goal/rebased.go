package goal

import (
	"fmt"
	"strings"
)

// RecordRebase records the branch's rebase after it has been published.
func RecordRebase(r VerbRequest, id, reason string) (PublishResult, error) {
	if r.Actor.Human != "" {
		return PublishResult{}, fmt.Errorf("the rebase line is the seat's that holds the goal; it takes no --by")
	}
	if strings.TrimSpace(reason) == "" || strings.ContainsAny(reason, "\r\n") {
		return PublishResult{}, fmt.Errorf("the rebase line names, on one line, what the rebase did")
	}
	return Publish(r.Endpoint, rebasedRequest(r, id, reason))
}

func rebasedRequest(r VerbRequest, id, reason string) PublishRequest {
	return PublishRequest{
		Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent:  Intent{Verb: "rebase", Targets: []string{id}, Args: intentArgs(r, map[string]string{"reason": reason})},
		Message: "goal rebase " + id,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil {
				return nil, fmt.Errorf("goal %s is not live; its rebase is recorded while it is claimed", id)
			}
			if opidLanded(f, r) {
				return nil, AlreadyApplied{}
			}
			if f.State != StateClaimed || f.Claimed == nil || !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s is not this seat's claim; its rebase is recorded by the seat that holds it", id)
			}
			touch(f, r, "rebase", []string{id})
			f.History[len(f.History)-1].Reason = reason
			return ackDisplacements(t, r, []Change{{Path: livePath(id), Content: RenderFile(f)}}), nil
		},
		Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) },
	}
}
