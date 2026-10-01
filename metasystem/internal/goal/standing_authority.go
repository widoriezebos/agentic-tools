package goal

// StandingAuthorityGap is carried from the redloop branch (79c9dd340, not
// landed): the read-only check landing validate makes before it claims the
// standing validation authority, so that an unapproved goal is reported in
// plain words and nothing is written.

import "fmt"

// StandingValidationGoal is the standing goal a landing validation runs
// under.
const StandingValidationGoal = "standing-validation"

// AuthorityGap is why a standing goal cannot carry a governed run: the
// plain reason and the one command that closes it.
type AuthorityGap struct{ Reason, Command string }

func (gap *AuthorityGap) Error() string { return gap.Reason }

// StandingAuthorityGap reads, without writing, whether the standing goal id
// can carry a governed run for actor: it is open, approved or already held,
// binds a governed obligation, and is neither held by nor pinned to another
// pair. A zero actor (a health reader) accepts any holder. Nil means a claim
// may succeed; the claim itself stays the authority.
func StandingAuthorityGap(tree *TreeGoals, id string, actor Actor) *AuthorityGap {
	show := "metasystem goal show " + id
	var file *GoalFile
	if tree != nil {
		file = tree.Live[id]
	}
	switch {
	case file == nil:
		return &AuthorityGap{Reason: fmt.Sprintf("goal %s is not open", id), Command: show}
	case file.State == StateClaimed && file.Claimed != nil:
		if actor.Machine != "" && !ownPair(file.Claimed, actor) {
			return &AuthorityGap{Reason: fmt.Sprintf("goal %s is claimed by %s+%s", id, file.Claimed.Machine, file.Claimed.Lineage), Command: show}
		}
	case file.State != StateApproved:
		return &AuthorityGap{Reason: fmt.Sprintf("goal %s is not approved", id), Command: "metasystem goal approve " + id}
	case actor.Machine != "" && file.Pinned != "" && file.Pinned != actor.Machine:
		return &AuthorityGap{Reason: fmt.Sprintf("goal %s is pinned to machine %s", id, file.Pinned), Command: show}
	}
	if file.Obligation == nil {
		// Binding one takes every obligation field (owner, recurrence,
		// platform, toolchain, surface digest, effects, review
		// assumptions), which only the person knows: the command shows
		// them all.
		return &AuthorityGap{Reason: fmt.Sprintf("goal %s binds no governed obligation", id), Command: "metasystem goal edit --help"}
	}
	return nil
}
