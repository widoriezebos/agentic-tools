package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func criticRecordParams(t *testing.T, root, role string) BuildRecordParams {
	t.Helper()
	tmp := t.TempDir()
	capResolution := filepath.Join(tmp, "cap.json")
	if err := WriteCapResolution(capResolution, 45, "built-in", "default"); err != nil {
		t.Fatal(err)
	}
	workspace, _ := filepath.Abs(filepath.Join("..", "..", ".."))
	p := BuildRecordParams{Output: filepath.Join(tmp, "record.json"), Job: role, Role: role, Root: root, Runtime: "fake", Workspace: workspace, CapResolution: capResolution, Model: "fake-model", Snapshot: "snapshot", InputBytes: 1, InputHash: "hash", Permissions: writeJSONFile(t, tmp, "permissions.json", map[string]any{}), Fallbacks: "[]", Signal: true, HandshakeBudget: 20, MainID: "main", ClaimEpoch: "1", DestructiveReach: HazardMechanical, ReasoningEffort: "medium", LaunchMode: LaunchModeSharedCheckout, OutputStream: filepath.Join(tmp, "stream.jsonl")}
	if role == "design-critic" {
		p.DeclaredOutputs = filepath.Join(tmp, "outputs.txt")
		if err := os.WriteFile(p.DeclaredOutputs, []byte("metasystem/internal/dispatch/build.go\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p.Design = "metasystem/plans/critique-closes-on-folded-proof-design.md"
	}
	return p
}
func TestDesignCriticDispatchCapsGoalRoundLimitAtFive(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	p := criticRecordParams(t, bed.root, "design-critic")
	p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
	if limit := buildCriticLimitWithReads(t, p, bed.reads); limit != 5 {
		t.Fatalf("design-critic dispatch limit = %d, want 5", limit)
	}
}
func TestGoalFreeDesignCriticDispatchAndFallbackCapAtFive(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "metasystem.conf"), []byte("metasystem.budget.review-round-max=3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	reads := goalFreeCriticReads(t)
	for role, want := range map[string]int64{"design-critic": 5, "code-critic": 3} {
		if got := buildCriticLimitWithReads(t, criticRecordParams(t, root, role), reads); got != want {
			t.Fatalf("goal-free %s dispatch limit = %d, want %d", role, got, want)
		}
	}
	account, err := critiqueRoundAccountingWithReads(root, critiqueState{}, "critic", map[string]any{"role": "design-critic", "goalId": nil, criticRoundsConsumedField: 0}, reads)
	if err != nil || account.limit != 5 {
		t.Fatalf("goal-free design-critic fallback = %+v, %v; want limit 5", account, err)
	}
	path := filepath.Join(root, "artifacts", "agents", "jobs", "critic.json")
	writeJSONFile(t, filepath.Dir(path), filepath.Base(path), map[string]any{"jobId": "critic", "role": "design-critic", "goalId": nil, reviewRoundLimitField: 3, criticRoundsConsumedField: 0, "critiqueBudgetBinding": map[string]any{"opid": "critique-budget-rebind-critic-r0"}})
	if outcome, err := critiqueBudgetRebindWithReads(root, "critic", reads); err != nil || outcome != "rebound" {
		t.Fatalf("goal-free rebind = %q, %v", outcome, err)
	}
	record := readJSONFile(t, path)
	if limit, _ := numInt(record[reviewRoundLimitField]); limit != 5 {
		t.Fatalf("goal-free rebound limit = %v, want 5", record[reviewRoundLimitField])
	}
}

// Design critique exists at tiers 2 and 3 (Wido 2026-10-01); tier 1 is refused.
func TestDesignCriticDispatchRequiresTierTwoOrThree(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	p := criticRecordParams(t, bed.root, "design-critic")
	p.Workspace = t.TempDir()
	p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 1, "full", "bed-m1"
	err := buildRecordWithReads(p, recordFacts(t, p.Workspace, 1, ""), bed.reads)
	if err == nil || !strings.Contains(err.Error(), "design-critic at goal tier 1") || !strings.Contains(err.Error(), "design critique exists at tiers 2 and 3") {
		t.Fatalf("tier-1 refusal = %v", err)
	}
	if err := validateReviewRoundTier("design-critic", true, 2); err != nil {
		t.Fatalf("tier 2 design critique refused: %v", err)
	}
	if _, err := os.Stat(p.Output); !os.IsNotExist(err) {
		t.Fatalf("tier-1 refusal wrote record: %v", err)
	}
}
func TestDesignCriticBudgetRebindRepairsAboveCap(t *testing.T) {
	bed := newGoalAdmissionBed(t, 2)
	goalRoot := bed.root
	path := filepath.Join(goalRoot, "artifacts", "agents", "jobs", "critic.json")
	writeJSONFile(t, filepath.Dir(path), filepath.Base(path), map[string]any{"jobId": "critic", "role": "design-critic", "goalId": "bounded", "goalRevision": 2, "goalTier": 3, reviewRoundLimitField: 6, criticRoundsConsumedField: 1})
	if _, err := critiqueRoundAccounting(goalRoot, critiqueState{}, "critic", readJSONFile(t, path)); err == nil || !strings.Contains(err.Error(), "design-critic review-round limit 6 exceeds cap 5") {
		t.Fatalf("above-cap stored limit accepted: %v", err)
	}
	if outcome, err := critiqueBudgetRebindWithReads(goalRoot, "critic", bed.reads); err != nil || outcome != "rebound" {
		t.Fatalf("goal-bound repair = %q, %v; want rebound", outcome, err)
	}
	record := readJSONFile(t, path)
	// The goal's stored member (3, an older tier-3 box) is below the cap, so
	// the repair rebinds to it; stored goal records are left alone.
	if limit, _ := numInt(record[reviewRoundLimitField]); limit != 3 {
		t.Fatalf("goal-bound repaired limit = %v, want 3", record[reviewRoundLimitField])
	}
	account, err := critiqueRoundAccounting(goalRoot, critiqueState{}, "critic", record)
	if err != nil || account.limit != 3 {
		t.Fatalf("repaired accounting = %+v, %v; want limit 3", account, err)
	}
	delete(record, reviewRoundLimitField)
	delete(record, "goalTier")
	if account, err = critiqueRoundAccountingWithReads(goalRoot, critiqueState{}, "critic", record, bed.reads); err != nil || account.limit != 5 {
		t.Fatalf("tier-free fallback accounting = %+v, %v; want limit 5", account, err)
	}
	if limit := reviewRoundLimitForRole("design-critic", true, 1).rebindLimit(); limit != 1 {
		t.Fatalf("goal-bound below-cap rebind = %d, want 1", limit)
	}
}
func TestNonDesignCriticsKeepGoalRoundLimit(t *testing.T) {
	for _, role := range []string{"code-critic", "warden"} {
		bed := newGoalAdmissionBed(t, 2)
		p := criticRecordParams(t, bed.root, role)
		p.GoalID, p.GoalRevision, p.GoalTier, p.GateWidth, p.MachineID = "bounded", 2, 3, "full", "bed-m1"
		if got := buildCriticLimitWithReads(t, p, bed.reads); got != 3 {
			t.Errorf("%s limit = %d, want 3", role, got)
		}
	}
}
