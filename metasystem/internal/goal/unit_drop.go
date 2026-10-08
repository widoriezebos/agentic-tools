package goal

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// UnitDrop records a proved, published optional-unit inverse. Original
// subjects and reads remain evidence; this record never grants a clean read.
type UnitDrop struct {
	Unit, Operation, Loop, Subject, Commit, Tree, Proof string
	Decisions, Requirements, Actor, Reason, Impact, At  string
	Revision                                            uint64
	Attempt                                             int
	Covered, Findings                                   []string
}

func (d UnitDrop) validate() error {
	if !bareReviewID(d.Unit) || !bareReviewID(d.Operation) || !bareReviewID(d.Loop) || !bareReviewID(d.Subject) || d.Attempt < 1 || d.Revision == 0 || len(d.Covered) == 0 || len(d.Findings) == 0 || d.Proof == "" || d.Actor == "" || strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("a drop needs its exact unit, stopped read, covered commits, successful checks and reason")
	}
	if _, err := time.Parse(time.RFC3339Nano, d.At); err != nil {
		return err
	}
	for _, value := range append([]string{d.Commit, d.Tree}, d.Covered...) {
		if raw, err := hex.DecodeString(value); err != nil || len(raw) != 20 {
			return fmt.Errorf("a drop needs full commit and tree identities")
		}
	}
	for _, value := range []string{d.Decisions, d.Requirements} {
		if raw, err := hex.DecodeString(value); err != nil || len(raw) != 32 {
			return fmt.Errorf("a drop needs its bound decisions and requirements")
		}
	}
	return nil
}

// RecordUnitDrop publishes the outcome only for the current claim and goal
// revision. Repeating the operation joins the identical durable outcome.
func RecordUnitDrop(r VerbRequest, id string, drop UnitDrop) (PublishResult, error) {
	if err := drop.validate(); err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "work-drop", Targets: []string{id}}, Message: "goal drop " + id + " unit " + drop.Unit,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil || f.State != StateClaimed || !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s drop requires its current claim holder", id)
			}
			for _, prior := range f.UnitDrops {
				if prior.Operation == drop.Operation {
					// The revision guards publication, not the outcome's identity.
					prior.Revision = drop.Revision
					if !reflect.DeepEqual(prior, drop) {
						return nil, fmt.Errorf("the drop operation already records another outcome")
					}
					return nil, AlreadyApplied{}
				}
			}
			if f.Revision != drop.Revision {
				return nil, fmt.Errorf("the goal changed before its drop outcome was recorded")
			}
			f.UnitDrops = append(f.UnitDrops, drop)
			touch(f, r, "work-drop", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) }})
}
