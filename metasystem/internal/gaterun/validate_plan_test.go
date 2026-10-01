package gaterun

import (
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
	"github.com/widoriezebos/agentic-tools/metasystem/internal/proofrun"
)

var planValidationStart = time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC)

func planProbe(id, identity, status string) proofrun.GroupResult {
	group := proofrun.GroupResult{ID: id, Kind: "unit", InputManifest: []string{"source"}, ExecutionIdentity: identity,
		Status: status, CollectionComplete: status == "passed" || status == "reused"}
	if status == "reused" {
		group.ReuseAttempt = "retained-attempt"
	}
	return group
}

func planLatest(trunk CadenceTrunk, identity, status string) *goal.CadenceStatus {
	return &goal.CadenceStatus{TrunkCommit: trunk.Commit, TrunkTree: trunk.Tree, Trigger: goal.CadenceTriggerForcedWindow,
		RunID: "run-old", AttemptID: "attempt-old", StartedAt: planValidationStart.Format(time.RFC3339), EndedAt: planValidationStart.Format(time.RFC3339),
		ForcedWindowStart: planValidationStart.Format(time.RFC3339), Groups: []goal.CadenceGroupStatus{{Group: "section/deep", ExecutionIdentity: identity,
			Status: status, EvidenceDigest: strings.Repeat("e", 64)}}}
}

// landing validate's due decision (the cadence trigger it kept from the
// deleted tick): a deep-only group whose identity changed, that is missing
// or whose newest result is not green is due by identity; weight over its
// threshold is due by weight; nothing else is due inside the forced window.
func TestPlanValidationTriggers(t *testing.T) {
	t.Parallel()
	trunk := CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	identity := strings.Repeat("1", 64)
	now := planValidationStart.Add(time.Hour)
	for _, test := range []struct {
		name, status, latestIdentity, latestStatus string
		weightDue, due                             bool
		trigger                                    goal.CadenceTrigger
	}{
		{name: "unchanged", status: "reused", latestIdentity: identity, latestStatus: "passed"},
		{name: "changed", status: "reused", latestIdentity: strings.Repeat("2", 64), latestStatus: "passed", due: true, trigger: goal.CadenceTriggerIdentityChanged},
		{name: "missing", status: "not-run", latestIdentity: identity, latestStatus: "passed", due: true, trigger: goal.CadenceTriggerIdentityChanged},
		{name: "newest non-green", status: "reused", latestIdentity: identity, latestStatus: "failed", due: true, trigger: goal.CadenceTriggerIdentityChanged},
		{name: "weight due", status: "reused", latestIdentity: identity, latestStatus: "passed", weightDue: true, due: true, trigger: goal.CadenceTriggerWeightDue},
	} {
		t.Run(test.name, func(t *testing.T) {
			plan, err := PlanValidation(now, trunk, planLatest(trunk, test.latestIdentity, test.latestStatus), WeightState{Generation: 4}, test.weightDue,
				[]string{"section/deep"}, CadenceRevalidation{Groups: []proofrun.GroupResult{planProbe("section/deep", identity, test.status)}})
			if err != nil || plan.Due != test.due || plan.Trigger != test.trigger || plan.ForceGroups ||
				plan.Key.TrunkTree != trunk.Tree || plan.Key.WeightGeneration != 4 || plan.Key.ForcedWindowStart != planValidationStart.Format(time.RFC3339) ||
				len(plan.Probes) != 1 || plan.Probes[0].ExecutionIdentity != identity {
				t.Fatalf("plan = %+v, %v", plan, err)
			}
		})
	}
}

// The forced window: no status yet, or a window that ran its interval,
// forces every deep-only group in a window that starts now.
func TestPlanValidationForcesEverySixHours(t *testing.T) {
	t.Parallel()
	trunk := CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	identity := strings.Repeat("1", 64)
	revalidation := CadenceRevalidation{Groups: []proofrun.GroupResult{planProbe("section/deep", identity, "reused")}}
	latest := planLatest(trunk, identity, "passed")
	early := planValidationStart.Add(CadenceForcedInterval - time.Second)
	if plan, err := PlanValidation(early, trunk, latest, WeightState{}, false, []string{"section/deep"}, revalidation); err != nil || plan.Due {
		t.Fatalf("inside the window = %+v, %v", plan, err)
	}
	due := planValidationStart.Add(CadenceForcedInterval)
	for _, prior := range []*goal.CadenceStatus{latest, nil} {
		plan, err := PlanValidation(due, trunk, prior, WeightState{}, false, []string{"section/deep"}, revalidation)
		if err != nil || !plan.Due || !plan.ForceGroups || plan.Trigger != goal.CadenceTriggerForcedWindow || plan.Key.ForcedWindowStart != due.Format(time.RFC3339) {
			t.Fatalf("forced window (prior %v) = %+v, %v", prior != nil, plan, err)
		}
	}
}

// An inventory or a revalidation that cannot be judged is refused, never
// taken as due or as clean.
func TestPlanValidationRefusesWhatItCannotJudge(t *testing.T) {
	t.Parallel()
	trunk := CadenceTrunk{Commit: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40)}
	identity := strings.Repeat("1", 64)
	good := CadenceRevalidation{Groups: []proofrun.GroupResult{planProbe("section/deep", identity, "reused")}}
	now := planValidationStart.Add(time.Hour)
	for name, call := range map[string]func() error{
		"short trunk": func() error {
			_, err := PlanValidation(now, CadenceTrunk{Commit: "a", Tree: trunk.Tree}, nil, WeightState{}, false, []string{"section/deep"}, good)
			return err
		},
		"empty inventory": func() error {
			_, err := PlanValidation(now, trunk, nil, WeightState{}, false, nil, good)
			return err
		},
		"duplicate inventory": func() error {
			_, err := PlanValidation(now, trunk, nil, WeightState{}, false, []string{"section/deep", "section/deep"}, good)
			return err
		},
		"omitted group": func() error {
			_, err := PlanValidation(now, trunk, nil, WeightState{}, false, []string{"section/deep", "section/other"}, good)
			return err
		},
		"bad identity": func() error {
			_, err := PlanValidation(now, trunk, nil, WeightState{}, false, []string{"section/deep"},
				CadenceRevalidation{Groups: []proofrun.GroupResult{planProbe("section/deep", strings.Repeat("z", 64), "reused")}})
			return err
		},
		"unreadable window": func() error {
			latest := planLatest(trunk, identity, "passed")
			latest.ForcedWindowStart = "not a time"
			_, err := PlanValidation(now, trunk, latest, WeightState{}, false, []string{"section/deep"}, good)
			return err
		},
	} {
		if call() == nil {
			t.Errorf("%s was judged", name)
		}
	}
}
