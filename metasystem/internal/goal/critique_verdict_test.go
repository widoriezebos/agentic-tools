package goal

import (
	"strings"
	"testing"
)

func critiqueVerdict(t *testing.T, fact JobFact) (string, bool) {
	t.Helper()
	verdict, err := legacyTurnVerdict(legacyVerdictStore(t, true), ScanResult{Busy: []Item{{Kind: "job", Id: "busy", Detail: "busy"}}, Jobs: []JobFact{fact}}, t.Name(), "", "seat")
	if err != nil {
		t.Fatal(err)
	}
	return verdict.Display, verdict.ShouldBlock
}

func TestCritiqueVerdictShowsZeroMaterialFold(t *testing.T) {
	t.Parallel()
	display, blocked := critiqueVerdict(t, JobFact{Id: "critic", MainId: "other", Status: "completed", Role: "design-critic", ReviewRoundLimit: 2, CritiqueRound: 2})
	want := "design critique round 2 of 2 folded: 0 material"
	if strings.Count("\n"+display+"\n", "\n"+want+"\n") != 1 || blocked {
		t.Fatalf("fold display = %q, blocked=%t; want one %q, false", display, blocked, want)
	}
}

func TestCritiqueVerdictShowsMechanicalFallingFold(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		falling bool
		want    string
	}{{"falling", true, "design critique round 2 of 2 folded: 3 mechanical, falling"}, {"not_falling", false, "design critique round 2 of 2 folded: 3 material"}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			display, blocked := critiqueVerdict(t, JobFact{Id: "critic", MainId: "other", Status: "completed", Role: "design-critic", ReviewRoundLimit: 2, CritiqueRound: 2, CritiqueMaterial: 3, CritiqueMechanical: true, CritiqueFalling: tc.falling})
			if strings.Count("\n"+display+"\n", "\n"+tc.want+"\n") != 1 || blocked {
				t.Fatalf("fold display = %q, blocked=%t; want one %q, false", display, blocked, tc.want)
			}
		})
	}
}

func TestCritiqueVerdictOmitsCycleWithoutDesignCritic(t *testing.T) {
	t.Parallel()
	withFields := JobFact{Id: "critic", MainId: "other", Status: "completed", Role: "code-critic", ReviewRoundLimit: 2, CritiqueRound: 2, CritiqueMaterial: 1, CritiqueMechanical: true, CritiqueFalling: true}
	display, blocked := critiqueVerdict(t, withFields)
	withoutFields := withFields
	withoutFields.ReviewRoundLimit, withoutFields.CritiqueRound, withoutFields.CritiqueMaterial = 0, 0, 0
	baseline, baselineBlocked := critiqueVerdict(t, withoutFields)
	if strings.Contains(display, "design critique round") || blocked != baselineBlocked || strings.Contains(baseline, "design critique round") {
		t.Fatalf("non-design critic gained cycle or changed block: display=%q blocked=%t baseline=%q/%t", display, blocked, baseline, baselineBlocked)
	}
	zeroLimit, zeroLimitBlocked := critiqueVerdict(t, JobFact{Id: "critic", MainId: "other", Status: "completed", Role: "design-critic"})
	if strings.Contains(zeroLimit, "design critique round") || zeroLimitBlocked != baselineBlocked {
		t.Fatalf("zero-limit design critic gained cycle or changed block: display=%q blocked=%t baseline=%t", zeroLimit, zeroLimitBlocked, baselineBlocked)
	}
}
