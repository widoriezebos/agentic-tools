package goal

import (
	"fmt"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
	"slices"
)

type ScopeExclusion struct {
	Unit, Operation, Requirements, Result, Proof, Actor, Authority, Reason, Impact, At string
	Designs, Obligations                                                               []string
	RestoredAt, RestoredBy                                                             string
}

func (f *GoalFile) ExcludesScope(unit, requirement string) bool {
	return f != nil && slices.ContainsFunc(f.ScopeExclusions, func(exclusion ScopeExclusion) bool {
		return exclusion.Unit == unit && exclusion.RestoredAt == "" && (requirement == "" || requirement == "result:"+exclusion.Result || slices.Contains(exclusion.Designs, requirement) || slices.Contains(exclusion.Obligations, requirement)) && f.validateScope(exclusion) == nil
	})
}

func (f *GoalFile) validateScope(exclusion ScopeExclusion) error {
	if exclusion.Actor == "" || exclusion.Authority == "" || exclusion.Reason == "" || exclusion.Impact == "" || !validStamp(exclusion.At) || len(exclusion.Designs)+len(exclusion.Obligations) == 0 || (exclusion.RestoredAt == "") != (exclusion.RestoredBy == "") || exclusion.RestoredAt != "" && !validStamp(exclusion.RestoredAt) {
		return fmt.Errorf("scope exclusion needs a person's authority, impact, exact requirements and time")
	}
	for _, drop := range f.UnitDrops {
		if drop.Unit == exclusion.Unit && drop.Operation == exclusion.Operation && drop.Commit == exclusion.Result && drop.Proof == exclusion.Proof && drop.Requirements == exclusion.Requirements && drop.Actor == exclusion.Actor && drop.At == exclusion.At {
			return nil
		}
	}
	return fmt.Errorf("scope exclusion has no matching successful drop")
}

func RestoreScope(r VerbRequest, id, unit string) (PublishResult, error) {
	if err := r.requireHuman(humanAuthorityRow{Verb: "scope restore", Name: "scope restoration", Missing: "only a person restores scope: metasystem goal scope restore " + id + " " + unit + " --by NAME"}, humanauthority.GradeEnrolled); err != nil {
		return PublishResult{}, err
	}
	return Publish(r.Endpoint, PublishRequest{Opid: r.opid(), Machine: r.Actor.Machine, Lineage: r.Actor.Lineage, Intent: Intent{Verb: "scope-restore", Targets: []string{id}}, Message: "restore required scope for " + id + " unit " + unit,
		Mutate: func(tip string) ([]Change, error) {
			t, err := loadTreeFor(r.Endpoint, tip)
			if err != nil {
				return nil, err
			}
			f := t.Live[id]
			if f == nil || !slices.ContainsFunc(f.ScopeExclusions, func(e ScopeExclusion) bool { return e.Unit == unit }) {
				return nil, fmt.Errorf("goal %s has no scope exclusion for unit %s", id, unit)
			}
			for i := range f.ScopeExclusions {
				e := &f.ScopeExclusions[i]
				if e.Unit == unit && e.RestoredAt == "" {
					e.RestoredAt, e.RestoredBy = r.stamp(), r.Actor.Human
				}
			}
			touch(f, r, "scope-restore", []string{id})
			return []Change{{Path: livePath(id), Content: RenderFile(f)}}, nil
		}, Validate: func(commit string) error { return validateCommitFor(r.Endpoint, commit) }})
}
