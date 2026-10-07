package goal

import (
	"fmt"
	"strings"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/humanauthority"
)

// personalClaim checks the directing person, never a stored name or grant.
func (r VerbRequest) personalClaim() (bool, error) {
	if r.Actor.Human == "" {
		return false, nil
	}
	if r.Authority == nil || !r.Authority.ValidFor(r.Endpoint.Root) || r.Authority.Helm != nil {
		return false, fmt.Errorf("only a person may reserve a goal\n%s", humanauthority.PersonActRemedy("metasystem goal claim GOAL"))
	}
	return true, nil
}

// PersonalReservation verifies the origin of this ownership episode. History
// proves storage shape; it never authorizes a new directing act.
func (f *GoalFile) PersonalReservation() bool {
	if f == nil || f.Claimed == nil {
		return false
	}
	c := f.Claimed
	if !strings.HasPrefix(c.By, "human:") || len(c.By) == len("human:") || c.Machine == "" || c.Lineage == "" || c.Revision == 0 || c.EpisodeRevision == 0 || c.EpisodeRevision > uint64(len(f.History)) {
		return false
	}
	machine, lineage := c.Machine, c.Lineage
	for i := len(f.History) - 1; i >= 0; i-- {
		event := f.History[i]
		switch event.Verb {
		case "claim", "steal":
			return uint64(i+1) <= c.Revision && event.Actor == c.By && event.AuthorityOutcome == AuthorityOutcomeHumanAuthorityProven && len(event.Opid) >= 26 && event.Opid == Opid(event.Opid[:26], machine, lineage)
		case "handover":
			// A handover records the source holder. Follow custody back
			// to the pair whose person-origin claim began the reservation.
			sourceMachine, err := OpidMachine(event.Opid)
			sourceLineage, ok := strings.CutPrefix(event.Actor, sourceMachine+"+")
			if err != nil || !ok || sourceLineage == "" || event.Opid != Opid(event.Opid[:26], sourceMachine, sourceLineage) {
				return false
			}
			machine, lineage = sourceMachine, sourceLineage
		case "release", "park":
			return false
		}
	}
	return false
}

func reservationWarnings(t *TreeGoals, r VerbRequest, f *GoalFile, tip string) []string {
	warnings := append([]string(nil), r.ClaimWarnings...)
	if f.Pinned != "" && f.Pinned != r.Actor.Machine {
		warnings = append(warnings, "goal "+f.Id+" is pinned to machine "+f.Pinned)
	}
	for _, dep := range f.Blocked {
		if depState(t, dep) != StateDone {
			warnings = append(warnings, "goal "+f.Id+" is blocked by unfinished goal "+dep)
		}
	}
	if _, err := claimQuotaRefusal(t, r, f.Id, tip, false); err != nil {
		warnings = append(warnings, err.Error())
	}
	return warnings
}

func bindReservation(f *GoalFile, r VerbRequest) error {
	person, err := r.personalClaim()
	if err != nil {
		return err
	}
	if err := bindClaim(f, r.Actor.Machine, r.Actor.Lineage, r.stamp(), f.Revision, r.ClaimEpoch, r); err != nil {
		return err
	}
	if person {
		f.Claimed.By = r.Actor.historyActor()
		h := &f.History[len(f.History)-1]
		h.AuthorityOutcome = AuthorityOutcomeHumanAuthorityProven
		h.AuthorityGeneration = r.Authority.TerminalGeneration
	}
	return nil
}

// ClaimEligibility is computed from the same accepted records as ownership.
// An absent result grants no launch authority.
type ClaimEligibility struct {
	Ready bool
	Wait  string
}

func claimExecutionEligibility(context *claimAdmissionContext, tree *TreeGoals, f *GoalFile, now time.Time) ClaimEligibility {
	if eligibility := ClaimApprovalEligibility(tree, f, now); !eligibility.Ready {
		return eligibility
	}
	if _, err := requireApprovedForClaimWithContext(context, tree, f, now, "work"); err != nil {
		return ClaimEligibility{Wait: err.Error()}
	}
	return ClaimEligibility{Ready: true}
}

// ClaimApprovalEligibility admits spending on an existing claim under its
// current approval. Claim and steward selection also require tier norm coverage.
func ClaimApprovalEligibility(tree *TreeGoals, f *GoalFile, now time.Time) ClaimEligibility {
	if tree == nil || f == nil || f.State != StateClaimed || f.Claimed == nil {
		return ClaimEligibility{Wait: "the owned claim is unreadable"}
	}
	if f.StopCapability == nil {
		return ClaimEligibility{Wait: "the claim has no stop capability; repair its claim authority"}
	}
	if f.StopFence != nil {
		return ClaimEligibility{Wait: "the claim is stopped by " + f.StopFence.StopID}
	}
	if f.Approved == nil || f.Budget == nil {
		return ClaimEligibility{Wait: fmt.Sprintf("awaits a person's approval; run: metasystem goal approve %s (as a person)", f.Id)}
	}
	if _, err := requireCurrentApproval(tree, f, now, "work"); err != nil {
		return ClaimEligibility{Wait: err.Error()}
	}
	return ClaimEligibility{Ready: true}
}
