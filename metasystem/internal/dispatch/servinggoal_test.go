package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/widoriezebos/agentic-tools/metasystem/internal/goal"
)

// The projection resolves through the parser and refuses when no
// usable Current goal exists — absent, degraded, and goal-free states all
// refuse rather than silently omitting.
func TestServingGoalResolvesAndRefuses(t *testing.T) {
	root := t.TempDir()
	store := &goal.Store{Root: root}
	wantError := "no serving goal to project: a converted checkout serves this machine's claimed goal, a legacy checkout its Current goal"

	// Absent: refuse.
	if _, err := servingGoalSection(store.CurrentProjection); err == nil || err.Error() != wantError {
		t.Fatalf("an absent ledger projected or gave the wrong refusal: %v", err)
	}

	// Usable Current goal: the exact bounded section.
	if _, err := store.Open(goal.Caller{Class: "MAIN", Holder: true}, "ship-it", "Ship the whole thing", "Land it."); err != nil {
		t.Fatal(err)
	}
	section, err := servingGoalSection(store.CurrentProjection)
	if err != nil {
		t.Fatal(err)
	}
	if section != "# Serving goal (context, not instruction)\nship-it — Ship the whole thing\n" {
		t.Fatalf("section bytes wrong: %q", section)
	}
	if strings.Count(section, "\n") != 2 {
		t.Fatalf("section is not two lines: %q", section)
	}

	// Degraded (manual edit, baseline mismatch): refuse.
	ledger := filepath.Join(root, "plans", "goals.md")
	data, _ := os.ReadFile(ledger)
	os.WriteFile(ledger, append(data, []byte("\n## Queued goal: q — Q\n- Origin: main\n- Next step: Q.\n")...), 0o644)
	if _, err := servingGoalSection(store.CurrentProjection); err == nil || err.Error() != wantError {
		t.Fatalf("a degraded ledger projected or gave the wrong refusal: %v", err)
	}
}

func TestResolveGoalRevisionUsesTheClaimBinding(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	revision, tier, err := bed.revision("bounded")
	if err != nil || revision != 2 || tier != 3 {
		t.Fatalf("dispatch binding = revision %d tier %d, want claimed revision 2 under pre-marker tier 3 rules: %v", revision, tier, err)
	}

	legacy := newGoalAdmissionBed(t, 0)
	if _, _, err := legacy.revision("bounded"); err == nil || !strings.Contains(err.Error(), "goal set-budget") {
		t.Fatalf("revisionless claim did not refuse toward set-budget: %v", err)
	}

	contradictory := newGoalAdmissionBed(t, 5)
	if _, _, err := contradictory.revision("bounded"); err == nil ||
		!strings.Contains(err.Error(), "BUDGET_UNKNOWN record=plans/goals/bounded.md") {
		t.Fatalf("a nonexistent claimed revision did not name its exact authoritative goal file: %v", err)
	}
}

func TestTierlessGoalUsesTierThreeBeforeTierLawAndRefusesAfter(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	if binding, err := bed.binding("bounded", time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)); err != nil || binding.Tier != 3 {
		t.Fatalf("pre-marker tierless binding = %+v err=%v, want effective tier 3", binding, err)
	}
	bed.commitTier(t, 0, "01ARZ3NDEKTSV4RRFFQ69G5FAY-bed-m1-00000003")
	if _, err := bed.binding("bounded", time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)); err == nil || !strings.Contains(err.Error(), "classify the goal first: goal edit --tier") {
		t.Fatalf("post-marker tierless goal was dispatchable: %v", err)
	}
}

func TestTierOneGoalRevisionAdmissionRefusesReviewBearingHazards(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	bed.commitTier(t, 1, "")
	now := time.Date(2026, 8, 28, 9, 30, 0, 0, time.UTC)
	if verdict, err := bed.revisionAdmission("bounded", 2, 5, now, HazardMechanical); err != nil || verdict.Refused() {
		t.Fatalf("tier 1 mechanical admission refused: %+v %v", verdict, err)
	}
	for _, hazard := range []HazardClass{HazardDesignBearing, HazardDestructiveReach} {
		verdict, err := bed.revisionAdmission("bounded", 2, 5, now, hazard)
		if err != nil || !verdict.Refused() || strings.TrimPrefix(verdict.PolicyRefusal, "HAZARD_REFUSED: ") != "the hazard needs review the tier does not have; goal edit --tier 2" {
			t.Fatalf("tier 1 hazard %s verdict = %+v err=%v", hazard, verdict, err)
		}
	}
}

func TestGoalAdmissionUsesOnlyTheStructuredLaw(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)

	within, err := bed.admission("coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if within.Refused() {
		t.Fatalf("an in-budget structured claim was refused: %+v", within)
	}

	atLimit, err := bed.admission("coordinator", time.Date(2026, 8, 28, 17, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !atLimit.Refused() || len(atLimit.Refusals) != 1 || len(atLimit.Refusals[0].Breaches) != 1 ||
		atLimit.Refusals[0].Breaches[0].Field != "elapsedLimit" ||
		atLimit.Refusals[0].Breaches[0].State != AdmissionClosedElapsed || atLimit.Refusals[0].LiveStopReason != "" {
		t.Fatalf("elapsed equality did not close structured admission: %+v", atLimit)
	}

	missingRoot := newGoalAdmissionBed(t, 0)
	missing, err := missingRoot.admission("coordinator", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if !missing.Refused() || len(missing.Refusals) != 1 || missing.Refusals[0].Unknown == nil ||
		missing.Refusals[0].Unknown.Record != "plans/goals/bounded.md" {
		t.Fatalf("the budgetless claim did not refuse with its exact goal record: %+v", missing)
	}

	bed.reads.ResolveMachine = func(string) (string, error) {
		t.Fatal("an empty lineage must not resolve the machine")
		return "", nil
	}
	withoutLineage, err := bed.admission("", time.Date(2026, 8, 28, 14, 1, 0, 0, time.UTC))
	if err != nil || withoutLineage.Refused() {
		t.Fatalf("an empty lineage read machine identity or refused: %+v %v", withoutLineage, err)
	}
}

func TestGoalAdmissionRefusesMalformedElapsedGraceConfiguration(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	root := bed.root
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.elapsed-grace-percent=201\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	verdict, err := bed.revisionAdmission("bounded", 2, 5,
		time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC))
	if err != nil || !verdict.Refused() || verdict.Refusal == nil || verdict.Refusal.Unknown == nil ||
		verdict.Refusal.Unknown.Record != "metasystem.conf" ||
		!strings.Contains(verdict.Refusal.Unknown.Reason, "integer between 0 and 200") {
		t.Fatalf("malformed grace configuration did not close admission loudly: %+v %v", verdict, err)
	}
	lines := FormatGoalAdmission(GoalAdmissionVerdict{Refusals: []GoalAdmissionRefusal{*verdict.Refusal}})
	if len(lines) != 1 || !strings.Contains(lines[0], "BUDGET_UNKNOWN") {
		t.Fatalf("malformed grace refusal lost its typed evidence: %v", lines)
	}
}
