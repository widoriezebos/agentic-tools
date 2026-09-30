package goal

import (
	"strings"
	"testing"
)

func TestStandingAuthorityGapNamesWhyTheCadenceCannotRun(t *testing.T) {
	t.Parallel()
	lane := Actor{Machine: "landing", Lineage: "landing-owner"}
	obligation := &GovernedObligation{}
	cases := []struct {
		name    string
		file    *GoalFile
		actor   Actor
		reason  string
		command string
	}{
		{"missing", nil, lane, "is not open", "metasystem goal show standing-validation"},
		{"unapproved", &GoalFile{State: StateQueued, Obligation: obligation}, lane, "is not approved", "metasystem goal approve standing-validation"},
		{"no obligation", &GoalFile{State: StateApproved}, lane, "no governed obligation", "metasystem goal edit standing-validation --obligation ENFORCED"},
		{"held elsewhere", &GoalFile{State: StateClaimed, Obligation: obligation, Claimed: &ClaimRecord{Machine: "m1e", Lineage: "seat"}}, lane, "is claimed by m1e+seat", "metasystem goal show standing-validation"},
		{"pinned elsewhere", &GoalFile{State: StateApproved, Obligation: obligation, Pinned: "m1e"}, lane, "is pinned to machine m1e", "metasystem goal show standing-validation"},
	}
	for _, tc := range cases {
		tree := &TreeGoals{Live: map[string]*GoalFile{}}
		if tc.file != nil {
			tree.Live["standing-validation"] = tc.file
		}
		gap := StandingAuthorityGap(tree, "standing-validation", tc.actor)
		if gap == nil || !strings.Contains(gap.Reason, tc.reason) || gap.Command != tc.command || !strings.Contains(gap.Error(), tc.reason) {
			t.Fatalf("%s: gap=%+v, want reason %q command %q", tc.name, gap, tc.reason, tc.command)
		}
	}
	ready := []*GoalFile{
		{State: StateApproved, Obligation: obligation},
		{State: StateApproved, Obligation: obligation, Pinned: "landing"},
		{State: StateClaimed, Obligation: obligation, Claimed: &ClaimRecord{Machine: "landing", Lineage: "landing-owner"}},
	}
	for index, file := range ready {
		tree := &TreeGoals{Live: map[string]*GoalFile{"standing-validation": file}}
		if gap := StandingAuthorityGap(tree, "standing-validation", lane); gap != nil {
			t.Fatalf("ready case %d reported a gap: %+v", index, gap)
		}
	}
	// A reader without a lane pair (health) treats any holder as the lane.
	held := &TreeGoals{Live: map[string]*GoalFile{"standing-validation": {State: StateClaimed, Obligation: obligation, Claimed: &ClaimRecord{Machine: "m1e", Lineage: "seat"}}}}
	if gap := StandingAuthorityGap(held, "standing-validation", Actor{}); gap != nil {
		t.Fatalf("a held authority read without a pair reported a gap: %+v", gap)
	}
}
