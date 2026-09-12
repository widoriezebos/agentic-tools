package goal

import "testing"

func TestTierProbeCountsRecordedAndDerivedTiers(t *testing.T) {
	risk := func(s, n, e, a uint8) *RiskRecord {
		return &RiskRecord{Severity: s, Novelty: n, Exposure: e, Accumulation: a, Basis: "probe"}
	}
	tree := &TreeGoals{Live: map[string]*GoalFile{
		"exposed":  {Id: "exposed", State: StateApproved, Tier: 3, Risk: risk(1, 1, 3, 1)},
		"severe":   {Id: "severe", State: StateQueued, Tier: 3, Risk: risk(3, 1, 1, 1)},
		"novel":    {Id: "novel", State: StateClaimed, Tier: 2, Risk: risk(1, 2, 1, 2)},
		"routine":  {Id: "routine", State: StateQueued, Tier: 1, Risk: risk(1, 1, 1, 1)},
		"tierless": {Id: "tierless", State: StateQueued},
		"overrated": {Id: "overrated", State: StateQueued, Tier: 3, Risk: risk(2, 1, 3, 2),
			History: []HistoryLine{{Verb: "open", Reason: "TierOverride: derived=2 set=3 why=review wanted"}}},
	}}
	probe := ProbeTiers(tree)
	if probe.Open != 5 || probe.Recorded[3] != 3 || probe.Recorded[2] != 1 || probe.Recorded[1] != 1 {
		t.Fatalf("recorded spread: %+v", probe)
	}
	if probe.Derived[3] != 1 || probe.Derived[2] != 2 || probe.Derived[1] != 2 {
		t.Fatalf("derived spread counts severity and novelty only: %+v", probe)
	}
	recorded, derived := probe.Tier3Share()
	if recorded != 60 || derived != 20 {
		t.Fatalf("tier-3 share recorded %d derived %d", recorded, derived)
	}
	if len(probe.Lowerable) != 2 || probe.Lowerable[0].ID != "exposed" || probe.Lowerable[0].Derived != 1 || probe.Lowerable[0].Override ||
		probe.Lowerable[1].ID != "overrated" || probe.Lowerable[1].Derived != 2 || !probe.Lowerable[1].Override {
		t.Fatalf("the goals a person may lower, overrides marked: %+v", probe.Lowerable)
	}
	if empty := ProbeTiers(nil); empty.Open != 0 || empty.Lowerable == nil {
		t.Fatalf("a nil tree probes empty with an empty list: %+v", empty)
	}
}
