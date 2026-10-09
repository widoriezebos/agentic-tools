package goal

import (
	"encoding/hex"
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"reflect"
	"strings"
	"time"
)

// UnitDrop records a proved, published optional-unit inverse. Original
// subjects and reads remain evidence; this record never grants a clean read.
type UnitDrop struct {
	Unit, Operation, Loop, Subject, Commit, Tree, Proof string
	Decisions, Requirements, Actor, Reason, Impact, At  string
	PatchDigest, CommitTree                             string
	Revision                                            uint64
	Attempt                                             int
	Covered, Findings                                   []string
}

func (d UnitDrop) validate() error {
	if !bareReviewID(d.Unit) || !bareReviewID(d.Operation) || !bareReviewID(d.Loop) || !bareReviewID(d.Subject) || d.Attempt < 1 || d.Revision == 0 || (len(d.Covered) == 0 && d.PatchDigest == "") || len(d.Findings) == 0 || d.Proof == "" || d.Actor == "" || strings.TrimSpace(d.Reason) == "" {
		return fmt.Errorf("a drop needs its exact unit, stopped read, covered commits, successful checks and reason")
	}
	if _, err := time.Parse(time.RFC3339Nano, d.At); err != nil {
		return err
	}
	identities := append([]string{d.Tree}, d.Covered...)
	if d.Commit != "" {
		identities = append(identities, d.Commit)
	}
	if d.CommitTree != "" {
		identities = append(identities, d.CommitTree)
	}
	if (len(d.Covered) > 0) != (d.Commit != "") {
		return fmt.Errorf("covered commits need their inverse commit")
	}
	for _, value := range identities {
		if raw, err := hex.DecodeString(value); err != nil || len(raw) != 20 {
			return fmt.Errorf("a drop needs full commit and tree identities")
		}
	}
	digests := []string{d.Decisions, d.Requirements}
	if d.PatchDigest != "" {
		digests = append(digests, d.PatchDigest)
	}
	for _, value := range digests {
		if raw, err := hex.DecodeString(value); err != nil || len(raw) != 32 {
			return fmt.Errorf("a drop needs its bound decisions and requirements")
		}
	}
	return nil
}

// RecordUnitDrop publishes the outcome only for the current claim and goal
// revision. Repeating the operation joins the identical durable outcome.
func RecordUnitDrop(r VerbRequest, id string, drop UnitDrop, scope []ScopeExclusion, previous ...UnitDrop) (PublishResult, error) {
	if len(scope) > 0 {
		if err := r.requireHuman(humanAuthorityRow{Verb: "work-drop", Name: "required scope exclusion", Missing: "only a person excludes required scope: metasystem work review " + id + " --work " + drop.Unit + " --dispositions FILE --reason TEXT --by NAME"}, humanauthority.GradeEnrolled); err != nil {
			return PublishResult{}, err
		}
		scope[0].Unit, scope[0].Operation, scope[0].Requirements, scope[0].Result, scope[0].Proof = drop.Unit, drop.Operation, drop.Requirements, drop.Commit, drop.Proof
		scope[0].Actor, scope[0].Authority, scope[0].Reason, scope[0].Impact, scope[0].At = r.Actor.Human, r.Authority.Outcome, drop.Reason, drop.Impact, drop.At
	}
	if err := drop.validate(); err != nil {
		return PublishResult{}, err
	}
	if len(previous) > 1 {
		return PublishResult{}, fmt.Errorf("a rebased drop replaces one published outcome")
	}
	if len(previous) == 1 {
		prior := previous[0]
		prior.Commit, prior.Tree, prior.Proof, prior.Covered, prior.Revision = drop.Commit, drop.Tree, drop.Proof, drop.Covered, drop.Revision
		if !reflect.DeepEqual(prior, drop) {
			return PublishResult{}, fmt.Errorf("a rebase preserves the drop's decision and authority")
		}
	}
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage,
		Intent: Intent{Verb: "work-drop", Targets: []string{id}}, Message: "goal drop " + id + " unit " + drop.Unit,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil || f.State != StateClaimed || f.Claimed == nil || len(scope) == 0 && !ownPair(f.Claimed, r.Actor) {
				return nil, fmt.Errorf("goal %s drop requires its current claim holder", id)
			}
			for index, prior := range f.UnitDrops {
				if prior.Operation == drop.Operation {
					if len(previous) == 1 && reflect.DeepEqual(prior, previous[0]) && f.Revision == drop.Revision {
						f.UnitDrops[index] = drop
						for i := range f.ScopeExclusions {
							exclusion := &f.ScopeExclusions[i]
							if exclusion.Unit == prior.Unit && exclusion.Operation == prior.Operation {
								exclusion.Result, exclusion.Proof = drop.Commit, drop.Proof
							}
						}
						touch(f, r, "work-drop", []string{id})
						return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
					}
					// The revision guards publication, not the outcome's identity.
					prior.Revision = drop.Revision
					if !reflect.DeepEqual(prior, drop) {
						return nil, fmt.Errorf("the drop operation already records another outcome")
					}
					return nil, AlreadyApplied{}
				}
			}
			if len(previous) == 1 {
				return nil, fmt.Errorf("the published drop changed before its rebase was recorded")
			}
			if f.Revision != drop.Revision {
				return nil, fmt.Errorf("the goal changed before its drop outcome was recorded")
			}
			f.UnitDrops = append(f.UnitDrops, drop)
			if len(scope) > 0 {
				// Goal-file validation checks this exclusion before publication.
				f.ScopeExclusions = append(f.ScopeExclusions, scope[0])
			}
			displaced := ""
			if !ownPair(f.Claimed, r.Actor) {
				displaced = pairMarker(f.Claimed)
			}
			touchDisplaced(f, r, "work-drop", []string{id}, displaced)
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) }})
}
