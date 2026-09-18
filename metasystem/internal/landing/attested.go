package landing

import (
	"fmt"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/gittree"
)

type attestationCodedError interface {
	LandingAttestationCode() string
}

func attestedBindingCode(err error) string {
	if coded, ok := err.(attestationCodedError); ok {
		switch coded.LandingAttestationCode() {
		case "unreadable":
			return "attested-unreadable"
		case "goal-mismatch":
			return "attested-goal-mismatch"
		case "change-mismatch":
			return "attested-change-mismatch"
		}
	}
	return "attested-invalid"
}

func observeAttested(params ObserveParams, change string) Observation {
	provenance := "attested=" + params.Attested + " change=" + change
	if !hexCommit(params.Attested) {
		return refuse("attested-malformed-id", provenance)
	}
	workspace := gittree.Workspace{Dir: params.RepoRoot}
	baseTree, err := workspace.HeadTree()
	if err != nil {
		return refuse("attested-unreadable", provenance)
	}
	if params.BindAttested == nil || params.AttestedSnapshot == "" || params.AttestedBase == "" {
		return refuse("attested-unreadable", provenance)
	}
	bound, err := params.BindAttested(params.Attested, params.AttestedSnapshot, params.AttestedBase, params.Goal, baseTree, params.CandidateTree)
	if err != nil {
		result := refuse(attestedBindingCode(err), provenance)
		result.Detail = err.Error()
		return result
	}
	if bound.Goal != params.Goal {
		return refuse("attested-goal-mismatch", provenance)
	}
	if bound.CriticRoot == "" {
		return refuse("attested-not-critic", provenance)
	}
	if bound.Destructive && !bound.HasPlan {
		return refuse("attested-not-design-bearing", provenance)
	}
	provenance = fmt.Sprintf("attested=%s goal=%s unit=%s critic=%s/%d change=%s",
		params.Attested, bound.Goal, bound.Unit, bound.CriticRoot, bound.Round, bound.Digest)
	if _, err := readTestReceipt(params); err != nil {
		result := refuse("attested-invalid", provenance)
		result.Detail = err.Error()
		return result
	}
	classes, err := loadPathClasses(workspace, baseTree)
	if err != nil {
		return wouldRefuse(carriageRefusalCode(err), provenance)
	}
	changedPaths, err := workspace.ChangedPaths(baseTree, params.CandidateTree)
	if err != nil {
		return wouldRefuse("candidate-tree-unreadable", provenance)
	}
	resolved, err := resolvePathClasses(workspace, classes, changedPaths)
	if err != nil {
		return wouldRefuse("register-carriage-policy-unreadable", provenance)
	}
	if _, err := heldGoal(workspace, baseTree, params.Goal, params.Actor, bound.GoalRevision); err != nil {
		return wouldRefuseFromCarriage(err, provenance)
	}
	if err := chainClassError(resolved, changedPaths); err != nil {
		return wouldRefuseFromCarriage(err, provenance)
	}
	boundPaths := make(map[string]bool, len(bound.ChangedPaths)+len(bound.FoldPaths))
	for _, path := range bound.ChangedPaths {
		boundPaths[path] = true
	}
	for _, path := range bound.FoldPaths {
		boundPaths[path] = true
	}
	var extras []string
	for _, path := range changedPaths {
		if !boundPaths[path] {
			extras = append(extras, path)
		}
	}
	extras = carriedReceiptLedger(workspace, baseTree, params.CandidateTree, extras)
	if len(extras) != 0 {
		if _, err := registerCarriage(params.RepoRoot, params.CandidateTree, extras, params.Goal, params.Actor, bound.GoalRevision); err != nil {
			return wouldRefuseFromCarriage(err, provenance)
		}
	}
	result := pass(BarAttested, "attested-branch", provenance)
	result.GoalRevision = bound.GoalRevision
	return result
}

func hexCommit(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, character := range value {
		if character < '0' || character > '9' {
			if character < 'a' || character > 'f' {
				return false
			}
		}
	}
	return true
}
